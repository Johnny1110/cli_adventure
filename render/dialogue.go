// render/dialogue.go — Pokemon-style dialogue box with typewriter text.
//
// THEORY — Typewriter text:
// In classic RPGs, text doesn't appear all at once — it types out character
// by character, creating a sense of a character "speaking". We implement this
// with a tick counter: each Update() tick increments the visible character count.
// The full string is stored, but Draw() only renders up to visibleChars.
//
// The dialogue box sits at the bottom of the screen (like Pokemon) and is
// 152x40 pixels (leaving 4px margin on each side). It can also show a list
// of choices for branching dialogue (shop buy/sell, quest accept).
package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// DialogueBox manages a text box at the bottom of the screen.
type DialogueBox struct {
	// Text content
	Lines       []string // queue of lines to show
	currentLine int
	visibleChars int
	fullText     string

	// Choices (optional)
	Choices       []string
	SelectedChoice int
	ShowChoices    bool
	ChoiceMade     bool

	// Timing
	CharsPerTick int // how many chars to reveal per tick (1 = slow, 3 = fast)
	tickCounter  int
	allRevealed  bool

	// State
	Active   bool
	Finished bool // true when all lines have been shown and dismissed
}

// NewDialogueBox creates a dialogue box with the given lines.
func NewDialogueBox(lines []string) *DialogueBox {
	d := &DialogueBox{
		Lines:        lines,
		CharsPerTick: 1,
		Active:       true,
	}
	if len(lines) > 0 {
		d.fullText = lines[0]
	}
	return d
}

// NewChoiceBox creates a dialogue box that ends with a choice selection.
func NewChoiceBox(prompt string, choices []string) *DialogueBox {
	return &DialogueBox{
		Lines:        []string{prompt},
		fullText:     prompt,
		Choices:      choices,
		ShowChoices:  false,
		CharsPerTick: 2,
		Active:       true,
	}
}

// Update advances the typewriter effect and handles input.
func (d *DialogueBox) Update() {
	if !d.Active || d.Finished {
		return
	}

	if d.ShowChoices {
		// Choice selection mode
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			d.SelectedChoice--
			if d.SelectedChoice < 0 {
				d.SelectedChoice = len(d.Choices) - 1
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			d.SelectedChoice++
			if d.SelectedChoice >= len(d.Choices) {
				d.SelectedChoice = 0
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			d.ChoiceMade = true
			d.Finished = true
			d.Active = false
		}
		return
	}

	// Typewriter effect
	if !d.allRevealed {
		d.tickCounter++
		if d.tickCounter >= 2 { // reveal a char every 2 ticks (~30 chars/sec)
			d.tickCounter = 0
			d.visibleChars += d.CharsPerTick
			if d.visibleChars >= len(d.fullText) {
				d.visibleChars = len(d.fullText)
				d.allRevealed = true
			}
		}
	}

	// Z/Enter to advance
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if !d.allRevealed {
			// Skip to full text
			d.visibleChars = len(d.fullText)
			d.allRevealed = true
		} else {
			// Advance to next line
			d.currentLine++
			if d.currentLine >= len(d.Lines) {
				// Check if we need to show choices
				if len(d.Choices) > 0 {
					d.ShowChoices = true
				} else {
					d.Finished = true
					d.Active = false
				}
			} else {
				d.fullText = d.Lines[d.currentLine]
				d.visibleChars = 0
				d.allRevealed = false
			}
		}
	}
}

// Draw renders the dialogue box at the bottom of the screen.
func (d *DialogueBox) Draw(dst *ebiten.Image) {
	if !d.Active {
		return
	}

	// Box dimensions — positioned at the bottom of the 320x288 screen
	bx, by := 8, 216
	bw, bh := 304, 64

	// Draw box background and border
	DrawBox(dst, bx, by, bw, bh, ColorBoxBG, ColorBoxBorder)

	// Draw visible text with typewriter effect
	if len(d.fullText) > 0 {
		visible := d.fullText
		if d.visibleChars < len(d.fullText) {
			visible = d.fullText[:d.visibleChars]
		}
		DrawText(dst, visible, bx+8, by+8, ColorWhite)
	}

	// "continue" indicator when text is fully revealed
	if d.allRevealed && !d.ShowChoices && !d.Finished {
		// Blinking arrow
		DrawText(dst, ">", bx+bw-10, by+bh-10, ColorGold)
	}

	// Draw choices if active
	if d.ShowChoices {
		choiceBoxW := 100
		choiceBoxH := len(d.Choices)*(GlyphH+6) + 10
		choiceBoxX := bx + bw - choiceBoxW - 2
		choiceBoxY := by - choiceBoxH - 2

		DrawBox(dst, choiceBoxX, choiceBoxY, choiceBoxW, choiceBoxH,
			ColorBoxBG, ColorBoxBorder)

		for i, choice := range d.Choices {
			cy := choiceBoxY + 5 + i*(GlyphH+6)
			clr := color.Color(ColorWhite)
			if i == d.SelectedChoice {
				DrawCursor(dst, choiceBoxX+3, cy, ColorGold)
				clr = ColorGold
			}
			DrawText(dst, choice, choiceBoxX+12, cy, clr)
		}
	}
}
