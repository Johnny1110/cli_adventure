// screen/skill.go — Skill tree screen for learning and upgrading skills.
//
// THEORY — Skill tree as investment decision:
// The skill screen presents the player's class skill tree as a vertical list.
// Each skill shows its current level, max level, SP cost to upgrade, and a
// brief description. Skills are locked until the previous one is learned
// (linear progression, like a simplified talent tree).
//
// The visual design uses level "pips" (filled/empty circles) to show progress
// at a glance — a technique borrowed from tabletop RPG character sheets.
// Gold pips = learned, gray pips = available, dark gray = locked.
package screen

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	"cli_adventure/render"
)

// SkillScreen is the skill tree management UI.
type SkillScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	returnTo Screen

	tree    []entity.SkillDef
	cursor  int
	tick    int
	msgText string
	msgTick int
}

// NewSkillScreen creates the skill tree screen.
func NewSkillScreen(switcher ScreenSwitcher, player *entity.Player, returnTo Screen) *SkillScreen {
	return &SkillScreen{
		switcher: switcher,
		player:   player,
		returnTo: returnTo,
		tree:     entity.ClassSkillTree(player.Class),
	}
}

func (s *SkillScreen) OnEnter() {}
func (s *SkillScreen) OnExit()  {}

func (s *SkillScreen) Update() error {
	s.tick++
	if s.msgTick > 0 {
		s.msgTick--
	}

	// Navigate
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		s.cursor--
		if s.cursor < 0 {
			s.cursor = len(s.tree) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		s.cursor++
		if s.cursor >= len(s.tree) {
			s.cursor = 0
		}
	}

	// Learn/upgrade skill
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		s.tryLearn()
	}

	// Close
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.switcher.SwitchScreen(s.returnTo)
	}

	return nil
}

func (s *SkillScreen) tryLearn() {
	if s.cursor >= len(s.tree) {
		return
	}
	def := s.tree[s.cursor]
	currentLvl := s.player.SkillLevel(def.ID)

	// Check prerequisite: must learn previous skill first
	if s.cursor > 0 {
		prevDef := s.tree[s.cursor-1]
		if s.player.SkillLevel(prevDef.ID) == 0 {
			s.msgText = "Learn " + s.tree[s.cursor-1].Name + " first!"
			s.msgTick = 60
			return
		}
	}

	if currentLvl >= def.MaxLevel {
		s.msgText = "Already maxed!"
		s.msgTick = 40
		return
	}

	cost := def.SPCost[currentLvl]
	if s.player.SkillPoints < cost {
		s.msgText = "Need " + intToStr(cost) + " SP!"
		s.msgTick = 40
		return
	}

	if s.player.LearnSkill(def) {
		newLvl := s.player.SkillLevel(def.ID)
		s.msgText = def.Name + " Lv." + intToStr(newLvl) + "!"
		s.msgTick = 60
	}
}

func (s *SkillScreen) Draw(screen *ebiten.Image) {
	screen.Fill(render.ColorBG)

	// Title
	render.DrawText(screen, "Skills", 56, 2, render.ColorLavender)

	// SP display
	render.DrawText(screen, "SP: "+intToStr(s.player.SkillPoints), 112, 2, render.ColorGold)

	// Class name
	info := entity.ClassTable[s.player.Class]
	render.DrawText(screen, info.Name+" Tree", 4, 14, render.ColorPink)

	// Skill list
	y := 26
	for i, def := range s.tree {
		currentLvl := s.player.SkillLevel(def.ID)

		// Check if this skill is locked (need previous skill first)
		locked := false
		if i > 0 {
			prevDef := s.tree[i-1]
			if s.player.SkillLevel(prevDef.ID) == 0 {
				locked = true
			}
		}

		// Name color
		nameClr := color.Color(render.ColorWhite)
		if i == s.cursor {
			nameClr = render.ColorGold
			render.DrawCursor(screen, 2, y, render.ColorGold)
		}
		if locked {
			nameClr = render.ColorDarkGray
		} else if currentLvl >= def.MaxLevel {
			nameClr = render.ColorMint // maxed = green
		}

		render.DrawText(screen, def.Name, 10, y, nameClr)

		// Level pips: [***] for Lv.3/3, [**-] for Lv.2/3, etc.
		pipStr := "["
		for lv := 0; lv < def.MaxLevel; lv++ {
			if lv < currentLvl {
				pipStr += "*"
			} else {
				pipStr += "-"
			}
		}
		pipStr += "]"
		pipClr := color.Color(render.ColorGray)
		if currentLvl > 0 {
			pipClr = render.ColorGold
		}
		if locked {
			pipClr = render.ColorDarkGray
		}
		render.DrawText(screen, pipStr, 88, y, pipClr)

		// SP cost for next level
		if currentLvl < def.MaxLevel && !locked {
			cost := def.SPCost[currentLvl]
			render.DrawText(screen, intToStr(cost)+"SP", 120, y, render.ColorPeach)
		} else if currentLvl >= def.MaxLevel {
			render.DrawText(screen, "MAX", 120, y, render.ColorMint)
		}

		y += 12
	}

	// Selected skill details
	if s.cursor < len(s.tree) {
		def := s.tree[s.cursor]
		currentLvl := s.player.SkillLevel(def.ID)

		detailY := 80
		// Divider
		for x := 4; x < 156; x += 2 {
			screen.Set(x, detailY-2, render.ColorDarkGray)
		}

		render.DrawText(screen, def.Name, 4, detailY, render.ColorGold)
		if currentLvl > 0 {
			render.DrawText(screen, "Lv."+intToStr(currentLvl), 120, detailY, render.ColorMint)
		}

		render.DrawText(screen, def.Desc, 4, detailY+10, render.ColorWhite)

		// Effect info
		infoY := detailY + 34
		render.DrawText(screen, def.SpecialDesc, 4, infoY, render.ColorSky)

		if currentLvl > 0 && currentLvl <= 3 {
			mpCost := def.MPCost[currentLvl-1]
			render.DrawText(screen, "MP: "+intToStr(mpCost), 100, infoY, render.ColorLavender)
		} else if currentLvl == 0 {
			// Show Lv.1 stats as preview
			render.DrawText(screen, "MP: "+intToStr(def.MPCost[0]), 100, infoY, render.ColorGray)
		}
	}

	// Message overlay
	if s.msgTick > 0 {
		render.DrawBox(screen, 20, 56, 120, 20, render.ColorBoxBG, render.ColorGold)
		render.DrawText(screen, s.msgText, 26, 60, render.ColorGold)
	}

	// Controls
	render.DrawText(screen, "Z:Learn X:Close", 32, 136, render.ColorGray)
}
