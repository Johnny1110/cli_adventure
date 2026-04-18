// screen/combat_mp.go — Team-play multiplayer combat screen.
//
// THEORY — A single screen for everyone:
// Unlike the single-player CombatScreen, this screen contains no combat
// engine. It's a read-only view of the host-authoritative
// net.CombatSharedState, plus a small input layer for locking in the
// local player's per-round action. The host's round machine
// (net/combat_round.go) does the math, picks the turn order, rolls the
// monster's attack, and pushes a ticking snapshot back to everyone.
//
// Round flow:
//
//   COLLECT (phase="collect"):
//     * Every peer sees a shared 10-second countdown.
//     * Each peer independently chooses Attack / Skill / Defend / Flee.
//     * Choosing locks the action (a "ready" indicator lights up in the
//       party panel). You cannot change your mind mid-round.
//     * When the timer hits zero — or everyone has locked in — the host
//       flips the shared phase to "resolve".
//
//   RESOLVE (phase="resolve"):
//     * Host plays out the round step-by-step at ~700 ms per step,
//       updating HP and log line as it goes.
//     * Screen just renders the current snapshot. All visual "animation"
//       is the natural one-log-line-per-tick cadence.
//
//   ENDED:
//     * Monster dead → apply XP/coins (from CombatEnd) and return to wild.
//     * All local players down → return to town with a heal (mercy).
//     * Fled → return to wild at pre-combat tile.
//
// THEORY — Why a separate file:
// The single-player combat screen has a rich animation system (sprite
// shake, floating damage numbers, multi-stage intro). Reusing it under
// a net-driven model would have forced every snapshot to re-animate
// every frame, and combat.go's state machine assumed exclusive ownership
// of player/monster HP. A dedicated MP screen is simpler and keeps the
// single-player path uncontaminated.
package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

// CombatMPScreen is the shared team-play combat view.
type CombatMPScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	session  *netpkg.Session

	// Return-to destination on combat end.
	areaKey      string
	returnX, returnY int

	// Menu cursor for the local decision.
	menuIdx   int // 0=Attack, 1=Skill, 2=Defend, 3=Flee
	submitted bool

	// Sprites (just for flair — monster + party icons).
	monsterSprites map[string]*render.SpriteSheet
	charSprites    map[int]*render.SpriteSheet

	// Latest log we've displayed; used to keep the dialogue box from
	// redrawing the same line forever while resolution steps play.
	lastSeenLog string
	lastRound   int
}

// NewCombatMPScreen creates a team-play combat view. `returnTo` is used
// when we can fall back to a stored wild-area tile position; if it's
// unset we send the party to the town.
func NewCombatMPScreen(switcher ScreenSwitcher, player *entity.Player, sess *netpkg.Session, areaKey string, retX, retY int) *CombatMPScreen {
	return &CombatMPScreen{
		switcher:       switcher,
		player:         player,
		session:        sess,
		areaKey:        areaKey,
		returnX:        retX,
		returnY:        retY,
		monsterSprites: asset.GenerateMonsterSprites(),
		charSprites:    asset.GenerateCharSprites(),
	}
}

func (c *CombatMPScreen) OnEnter() {}
func (c *CombatMPScreen) OnExit()  {}

func (c *CombatMPScreen) Update() error {
	// Did the fight end? Apply results and exit.
	if cs, ended := c.session.ConsumeCombatEnd(); ended {
		return c.finishCombat(cs)
	}

	// Session torn down under us.
	select {
	case <-c.session.Done:
		c.switcher.SwitchScreen(NewTownScreen(c.switcher, c.player))
		return nil
	default:
	}

	cs := c.session.CombatState()

	// When the round advances, reset our local menu / submission state
	// so we can pick again. `lastRound` trips to 0 on fresh start so we
	// initialise immediately.
	if cs.RoundNum != c.lastRound {
		c.lastRound = cs.RoundNum
		c.submitted = false
		c.menuIdx = 0
	}

	// Sync local HP from the authoritative snapshot so the overworld sees
	// the right numbers when we return.
	if me := c.localPlayer(cs); me != nil {
		c.player.Stats.HP = me.HP
		if me.Fled {
			// Fled: immediately transition back to wild at saved tile.
			c.returnToOverworld()
			return nil
		}
	}

	// Input only during collect phase, and only if the player is alive
	// and hasn't already locked in.
	if cs.Phase == netpkg.RoundPhaseCollect && !c.submitted && c.player.Stats.HP > 0 {
		c.updateMenu()
	}

	return nil
}

// updateMenu handles action selection and submission.
func (c *CombatMPScreen) updateMenu() {
	// 2×2 grid — layout: 0=Attack (TL), 1=Skill (TR), 2=Defend (BL), 3=Flee (BR).
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if c.menuIdx%2 == 1 {
			c.menuIdx--
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		if c.menuIdx%2 == 0 {
			c.menuIdx++
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if c.menuIdx >= 2 {
			c.menuIdx -= 2
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if c.menuIdx < 2 {
			c.menuIdx += 2
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		var k netpkg.CombatActionKind
		switch c.menuIdx {
		case 0:
			k = netpkg.CombatAttack
		case 1:
			k = netpkg.CombatSkill
		case 2:
			k = netpkg.CombatDefend
		case 3:
			k = netpkg.CombatFlee
		}
		c.session.SubmitCombatAction(netpkg.CombatActionMsg{Kind: k})
		c.submitted = true
	}
}

// localPlayer returns the CombatPlayer matching this session's peer ID,
// or nil if not found.
func (c *CombatMPScreen) localPlayer(cs netpkg.CombatSharedState) *netpkg.CombatPlayer {
	for i := range cs.Players {
		if cs.Players[i].PeerID == c.session.MyID() {
			return &cs.Players[i]
		}
	}
	return nil
}

// finishCombat applies rewards / penalties and routes back to the overworld.
func (c *CombatMPScreen) finishCombat(cs netpkg.CombatSharedState) error {
	// Sync final HP from snapshot in case a late tick landed.
	if me := c.localPlayer(cs); me != nil {
		c.player.Stats.HP = me.HP
	}

	if cs.EndVictory {
		c.player.GainXP(cs.EndXP)
		c.player.Coins += cs.EndCoins
		c.returnToOverworld()
		return nil
	}

	// Defeat or everyone-fled. If we're actually dead (HP==0 or we
	// fell during the fight), apply the single-player defeat penalty.
	if c.player.Stats.HP <= 0 {
		c.player.Stats.HP = c.player.Stats.MaxHP
		c.player.Stats.MP = c.player.Stats.MaxMP
		c.player.Coins = c.player.Coins * 3 / 4
		c.switcher.SwitchScreen(NewTownScreenMP(c.switcher, c.player, c.session))
		return nil
	}
	// Otherwise we ran away — return to the world.
	c.returnToOverworld()
	return nil
}

func (c *CombatMPScreen) returnToOverworld() {
	if c.areaKey != "" && c.areaKey != "town" {
		c.switcher.SwitchScreen(NewWildScreenMPAt(c.switcher, c.player, c.areaKey, c.returnX, c.returnY, c.session))
		return
	}
	c.switcher.SwitchScreen(NewTownScreenMP(c.switcher, c.player, c.session))
}

// ---- Draw ----

func (c *CombatMPScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)
	cs := c.session.CombatState()

	// --- Header: monster name + HP ---
	nameClr := render.ColorPink
	if cs.MonsterIsBoss {
		nameClr = render.ColorGold
	}
	render.DrawText(screen, cs.MonsterName, 8, 4, nameClr)
	if cs.MonsterMax > 0 {
		frac := float64(cs.MonsterHP) / float64(cs.MonsterMax)
		render.DrawBar(screen, 8, 24, 200, 8, frac, render.ColorRed, render.ColorDarkGray)
	}
	render.DrawText(screen, intToStr(cs.MonsterHP)+"/"+intToStr(cs.MonsterMax),
		216, 20, render.ColorWhite)

	// Monster sprite (right side).
	if sheet, ok := c.monsterSprites[cs.MonsterSpriteID]; ok {
		sheet.DrawFrame(screen, 0, 240, 36)
	}

	// --- Round / phase indicator ---
	roundTag := "Round " + intToStr(cs.RoundNum)
	render.DrawText(screen, roundTag, 8, 44, render.ColorSky)
	if cs.Phase == netpkg.RoundPhaseCollect {
		timerStr := "Time: " + intToStr(cs.SecondsLeft) + "s"
		clr := render.ColorMint
		if cs.SecondsLeft <= 3 {
			clr = render.ColorRed
		}
		render.DrawText(screen, timerStr, 160, 44, clr)
	} else if cs.Phase == netpkg.RoundPhaseResolve {
		render.DrawText(screen, "Resolving...", 160, 44, render.ColorGold)
	}

	// --- Party panel ---
	y := 68
	render.DrawText(screen, "Party:", 8, y, render.ColorWhite)
	y += 16
	for _, p := range cs.Players {
		c.drawPartyRow(screen, p, 8, y)
		y += 16
		if y > 188 {
			break
		}
	}

	// --- Log box ---
	if cs.LastLog != "" {
		render.DrawBox(screen, 4, 196, 312, 36, render.ColorBoxBG, render.ColorSky)
		render.DrawText(screen, cs.LastLog, 12, 204, render.ColorWhite)
	}

	// --- Action menu (collect phase) or "waiting" message ---
	c.drawActionArea(screen, cs)
}

// drawPartyRow draws one peer's name, HP bar, and status chip.
func (c *CombatMPScreen) drawPartyRow(screen *ebiten.Image, p netpkg.CombatPlayer, x, y int) {
	name := p.Name
	if name == "" {
		name = p.PeerID
	}
	clr := render.ColorWhite
	if p.PeerID == c.session.MyID() {
		clr = render.ColorGold
		name = name + " (you)"
	}
	render.DrawText(screen, name, x, y, clr)

	// HP bar
	if p.MaxHP > 0 {
		frac := float64(p.HP) / float64(p.MaxHP)
		hpClr := hpBarColor(frac)
		render.DrawBar(screen, x+136, y+1, 100, 8, frac, hpClr, render.ColorDarkGray)
	}

	// Status chip on the far right
	chip := ""
	chipClr := render.ColorGray
	switch {
	case p.Fled:
		chip = "fled"
	case p.HP <= 0:
		chip = "down"
		chipClr = render.ColorRed
	case p.Ready:
		chip = actionLabel(p.Action)
		chipClr = render.ColorMint
	default:
		chip = "..."
	}
	render.DrawText(screen, chip, x+248, y, chipClr)
}

// actionLabel turns a locked-in action kind into a short visible tag.
func actionLabel(a string) string {
	switch a {
	case "attack":
		return "ATK"
	case "skill":
		return "SKL"
	case "defend":
		return "DEF"
	case "flee":
		return "RUN"
	}
	return "OK"
}

// drawActionArea draws the 2×2 menu during collect, or a status line
// during resolve / once we're locked in.
func (c *CombatMPScreen) drawActionArea(screen *ebiten.Image, cs netpkg.CombatSharedState) {
	menuY := 240
	if c.player.Stats.HP <= 0 {
		render.DrawText(screen, "You are down...", 96, menuY+10, render.ColorGray)
		return
	}
	if cs.Phase == netpkg.RoundPhaseResolve {
		render.DrawText(screen, "Watch the action!", 88, menuY+10, render.ColorGray)
		return
	}
	if c.submitted {
		render.DrawText(screen, "Locked in — waiting...", 60, menuY+10, render.ColorMint)
		return
	}
	// 2×2 action grid
	items := []string{"Attack", "Skill", "Defend", "Flee"}
	colWidth := 136
	for i, it := range items {
		col := i % 2
		row := i / 2
		x := 16 + col*colWidth
		y := menuY + row*18
		clr := render.ColorWhite
		if i == c.menuIdx {
			render.DrawCursor(screen, x-8, y, render.ColorGold)
			clr = render.ColorGold
		}
		render.DrawText(screen, it, x, y, clr)
	}
}
