package screen

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/asset"
	"cli_adventure/entity"
	"cli_adventure/render"
)

// CombatStubScreen is a placeholder for Phase 4's full combat implementation.
// It shows the monster sprite and basic info, then lets you "win" for testing.
type CombatStubScreen struct {
	switcher       ScreenSwitcher
	player         *entity.Player
	monster        *entity.Monster
	areaKey        string // which area to return to after combat
	monsterSprites map[string]*render.SpriteSheet
	monsterAnim    *render.Animation
	tick           int
}

func NewCombatStubScreen(switcher ScreenSwitcher, player *entity.Player, monster *entity.Monster, areaKey string) *CombatStubScreen {
	return &CombatStubScreen{
		switcher:       switcher,
		player:         player,
		monster:        monster,
		areaKey:        areaKey,
		monsterSprites: asset.GenerateMonsterSprites(),
		monsterAnim:    render.NewAnimation([]int{0, 1}, 20),
	}
}

func (c *CombatStubScreen) OnEnter() {}
func (c *CombatStubScreen) OnExit()  {}

func (c *CombatStubScreen) Update() error {
	c.tick++
	c.monsterAnim.Update()

	// Z to "win" (stub: auto-victory for testing the flow)
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		// Grant rewards
		c.player.GainXP(c.monster.XPReward)
		c.player.Coins += c.monster.CoinReward

		// Update quest progress
		q := c.player.ActiveQuest()
		if q != nil && q.Target == c.monster.Name && !q.Done {
			q.Progress++
		}

		// Return to wild area
		c.switcher.SwitchScreen(NewWildScreen(c.switcher, c.player, c.areaKey))
	}

	// X to flee
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		if !c.monster.IsBoss {
			c.switcher.SwitchScreen(NewWildScreen(c.switcher, c.player, c.areaKey))
		}
	}

	return nil
}

func (c *CombatStubScreen) Draw(screen *ebiten.Image) {
	// Dark combat background
	screen.Fill(render.ColorBG)

	// Monster name and stats
	render.DrawText(screen, c.monster.Name, 60, 4, render.ColorPink)
	render.DrawText(screen, "HP: "+intToStr(c.monster.HP)+"/"+intToStr(c.monster.MaxHP), 50, 14, render.ColorRed)

	// Monster sprite (centered)
	sprite, ok := c.monsterSprites[c.monster.SpriteID]
	if ok {
		frame := c.monsterAnim.CurrentFrame()
		if c.monster.IsBoss {
			// Boss is 32x32, center it
			sprite.DrawFrame(screen, frame, 64, 28)
		} else {
			// Normal 16x16, center it
			sprite.DrawFrame(screen, frame, 72, 32)
		}
	}

	// Divider
	render.DrawText(screen, "----------------", 16, 60, render.ColorDarkGray)

	// Player stats
	info := entity.ClassTable[c.player.Class]
	render.DrawText(screen, info.Name+" Lv."+intToStr(c.player.Level), 8, 68, render.ColorSky)

	s := c.player.Stats
	render.DrawText(screen, "HP:", 8, 80, render.ColorGreen)
	render.DrawBar(screen, 30, 81, 50, 5, float64(s.HP)/float64(s.MaxHP), render.ColorGreen, render.ColorDarkGray)
	render.DrawText(screen, intToStr(s.HP)+"/"+intToStr(s.MaxHP), 84, 80, render.ColorWhite)

	// Combat coming soon message
	render.DrawBox(screen, 4, 100, 152, 40, render.ColorBoxBG, render.ColorBoxBorder)
	render.DrawText(screen, "Combat in Phase 4!", 20, 104, render.ColorGold)
	render.DrawText(screen, "Z: Win  X: Flee", 24, 116, render.ColorWhite)
	if c.monster.IsBoss {
		render.DrawText(screen, "Cannot flee boss!", 20, 128, render.ColorRed)
	}
}
