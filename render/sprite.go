package render

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// SpriteSheet wraps a loaded image and provides frame extraction.
//
// THEORY: A sprite sheet packs multiple frames into a single PNG. We slice it
// into sub-images by cell size. On the GPU, the entire sheet is one texture,
// and SubImage just creates a view into a region — no pixel copying. This is
// why sprite sheets are fast: one texture bind, many draws.
type SpriteSheet struct {
	Image     *ebiten.Image
	CellW     int // width of each frame in pixels
	CellH     int // height of each frame in pixels
	Cols      int // number of columns in the sheet
	FrameCount int // total frames (may be less than cols * rows)
}

// NewSpriteSheet creates a SpriteSheet from a loaded image.
func NewSpriteSheet(img *ebiten.Image, cellW, cellH int) *SpriteSheet {
	bounds := img.Bounds()
	cols := bounds.Dx() / cellW
	rows := bounds.Dy() / cellH
	return &SpriteSheet{
		Image:     img,
		CellW:     cellW,
		CellH:     cellH,
		Cols:      cols,
		FrameCount: cols * rows,
	}
}

// Frame returns the sub-image for frame index n (0-based, row-major order).
func (s *SpriteSheet) Frame(n int) *ebiten.Image {
	if n < 0 || n >= s.FrameCount {
		n = 0
	}
	x := (n % s.Cols) * s.CellW
	y := (n / s.Cols) * s.CellH
	return s.Image.SubImage(image.Rect(x, y, x+s.CellW, y+s.CellH)).(*ebiten.Image)
}

// DrawFrame draws a specific frame at (x, y) on dst.
func (s *SpriteSheet) DrawFrame(dst *ebiten.Image, frame int, x, y float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	dst.DrawImage(s.Frame(frame), op)
}

// DrawFrameScaled draws a frame at (x, y) scaled by (sx, sy).
func (s *SpriteSheet) DrawFrameScaled(dst *ebiten.Image, frame int, x, y, sx, sy float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(x, y)
	dst.DrawImage(s.Frame(frame), op)
}

// DrawFrameTinted draws a frame with RGB color scaling (for golden/tinted monsters).
// r, g, b are multipliers (1.0 = normal, 0.5 = half brightness for that channel).
func (s *SpriteSheet) DrawFrameTinted(dst *ebiten.Image, frame int, x, y, r, g, b float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(r), float32(g), float32(b), 1)
	dst.DrawImage(s.Frame(frame), op)
}

// Animation manages frame cycling for animated sprites.
//
// THEORY: At 60 TPS, playing through animation frames every tick would be way
// too fast. Instead, we use a tick counter and advance the frame every N ticks.
// For Game Boy aesthetics, 10-15 ticks per frame (6-4 FPS animation) gives that
// authentic choppy retro feel.
type Animation struct {
	Frames       []int // frame indices into the sprite sheet
	TicksPerFrame int   // how many Update() ticks before advancing
	ticker       int   // current tick counter
	current      int   // index into Frames
	Loop         bool  // whether to loop or stop at last frame
}

// NewAnimation creates a looping animation.
func NewAnimation(frames []int, ticksPerFrame int) *Animation {
	return &Animation{
		Frames:       frames,
		TicksPerFrame: ticksPerFrame,
		Loop:         true,
	}
}

// Update advances the animation timer. Call once per Update() tick.
func (a *Animation) Update() {
	a.ticker++
	if a.ticker >= a.TicksPerFrame {
		a.ticker = 0
		a.current++
		if a.current >= len(a.Frames) {
			if a.Loop {
				a.current = 0
			} else {
				a.current = len(a.Frames) - 1
			}
		}
	}
}

// CurrentFrame returns the current animation frame index.
func (a *Animation) CurrentFrame() int {
	if len(a.Frames) == 0 {
		return 0
	}
	return a.Frames[a.current]
}

// Reset restarts the animation from the beginning.
func (a *Animation) Reset() {
	a.current = 0
	a.ticker = 0
}
