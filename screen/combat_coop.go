// screen/combat_coop.go — Client-side multiplayer combat screen.
//
// THEORY — Thin client over authoritative state:
// In host-authoritative co-op, the client doesn't simulate combat. It
// reads the shared CombatSharedState (fed from host broadcasts) and
// displays it: monster sprite, monster HP bar, party list with per-peer
// HPs, the latest log line the host sent. It lets the player submit a
// turn action (Attack / Defend / Flee) via net.MsgCombatAction, then
// waits for the next snapshot.
//
// We intentionally reuse the host's combat engine rather than try to
// render a parallel battle on every peer — this avoids divergence and
// keeps the client trivially small.
package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

// CombatCoopScreen displays the shared co-op battle for a client.
type CombatCoopScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	session  *netpkg.Session
	returnTo Screen

	menuIdx    int // 0=Attack,1=Defend,2=Flee
	submitted  bool
	lastLog    string
	logTick    int
}

// NewCombatCoopScreen creates the joining-peer's battle view.
// returnTo is the screen to fall back to when combat ends.
func NewCombatCoopScreen(switcher ScreenSwitcher, player *entity.Player, sess *netpkg.Session, returnTo Screen) *CombatCoopScreen {
	return &CombatCoopScreen{
		switcher: switcher,
		player:   player,
		session:  sess,
		returnTo: returnTo,
	}
}

func (c *CombatCoopScreen) OnEnter() {}
func (c *CombatCoopScreen) OnExit()  {}

func (c *CombatCoopScreen) Update() error {
	// If the host signalled the fight is over, apply rewards and exit.
	if cs, ended := c.session.ConsumeCombatEnd(); ended {
		if cs.EndVictory {
			c.player.GainXP(cs.EndXP)
			c.player.Coins += cs.EndCoins
		}
		if c.returnTo != nil {
			c.switcher.SwitchScreen(c.returnTo)
		} else {
			c.switcher.SwitchScreen(NewTownScreenMP(c.switcher, c.player, c.session))
		}
		return nil
	}

	// Session died (host quit).
	select {
	case <-c.session.Done:
		c.switcher.SwitchScreen(NewTownScreen(c.switcher, c.player))
		return nil
	default:
	}

	// Track latest log line — prevents overwriting with stale empties.
	cs := c.session.CombatState()
	if cs.LastLog != "" && cs.LastLog != c.lastLog {
		c.lastLog = cs.LastLog
		c.logTick = 60 // show for 1 s
	}
	if c.logTick > 0 {
		c.logTick--
	}

	if c.submitted {
		return nil // waiting for host to resolve
	}

	// Menu navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		c.menuIdx = (c.menuIdx + 2) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		c.menuIdx = (c.menuIdx + 1) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		var k netpkg.CombatActionKind
		switch c.menuIdx {
		case 0:
			k = netpkg.CombatAttack
		case 1:
			k = netpkg.CombatDefend
		case 2:
			k = netpkg.CombatFlee
		}
		c.session.SubmitCombatAction(netpkg.CombatActionMsg{Kind: k})
		c.submitted = true
	}
	return nil
}

func (c *CombatCoopScreen) Draw(screen *ebiten.Image) {
	cs := c.session.CombatState()

	// --- Monster area (top) ---
	render.DrawText(screen, cs.MonsterName, 8, 6, render.ColorRed)
	if cs.MonsterMax > 0 {
		frac := float64(cs.MonsterHP) / float64(cs.MonsterMax)
		render.DrawBar(screen, 8, 18, 144, 6, frac, render.ColorRed, render.ColorDarkGray)
	}
	render.DrawText(screen, "HP "+intToStr(cs.MonsterHP)+"/"+intToStr(cs.MonsterMax),
		8, 28, render.ColorWhite)

	// --- Party list ---
	render.DrawText(screen, "Party", 8, 44, render.ColorMint)
	y := 56
	// Show the local player first (from authoritative combat snapshot
	// if present, else from the player struct).
	myHP := c.player.Stats.HP
	myMax := c.player.Stats.MaxHP
	for _, p := range cs.Players {
		if p.PeerID == c.session.MyID() {
			myHP = p.HP
			myMax = p.MaxHP
		}
	}
	render.DrawText(screen, c.session.MyName()+" (you)", 8, y, render.ColorGold)
	if myMax > 0 {
		render.DrawBar(screen, 80, y+1, 72, 5, float64(myHP)/float64(myMax),
			render.ColorGreen, render.ColorDarkGray)
	}
	y += 10
	// Other party members
	for _, p := range cs.Players {
		if p.PeerID == c.session.MyID() {
			continue
		}
		// Look up display name from session's peer list
		name := p.PeerID
		for _, r := range c.session.RemotePlayers("") {
			if r.PeerID == p.PeerID {
				name = r.Name
				break
			}
		}
		render.DrawText(screen, name, 8, y, render.ColorWhite)
		if p.MaxHP > 0 {
			render.DrawBar(screen, 80, y+1, 72, 5, float64(p.HP)/float64(p.MaxHP),
				render.ColorGreen, render.ColorDarkGray)
		}
		y += 10
		if y > 94 {
			break
		}
	}

	// --- Log line / prompt ---
	if c.logTick > 0 && c.lastLog != "" {
		render.DrawBox(screen, 4, 100, 152, 14, render.ColorBoxBG, render.ColorSky)
		render.DrawText(screen, c.lastLog, 8, 103, render.ColorWhite)
	}

	// --- Action menu ---
	menuY := 118
	if c.submitted {
		render.DrawText(screen, "Waiting for host...", 24, menuY+4, render.ColorGray)
		return
	}
	items := []string{"Attack", "Defend", "Flee"}
	for i, it := range items {
		x := 16 + i*48
		clr := render.ColorWhite
		if i == c.menuIdx {
			render.DrawCursor(screen, x-6, menuY, render.ColorGold)
			clr = render.ColorGold
		}
		render.DrawText(screen, it, x, menuY, clr)
	}
	render.DrawText(screen, "Z:OK", 70, 134, render.ColorGray)
}
