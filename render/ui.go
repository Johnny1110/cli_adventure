package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Pastel color palette — used consistently across all UI and sprites.
var (
	ColorBG        = color.RGBA{R: 16, G: 16, B: 24, A: 255}   // dark background
	ColorWhite     = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	ColorPink      = color.RGBA{R: 255, G: 150, B: 180, A: 255}
	ColorMint      = color.RGBA{R: 130, G: 230, B: 180, A: 255}
	ColorSky       = color.RGBA{R: 130, G: 180, B: 255, A: 255}
	ColorLavender  = color.RGBA{R: 180, G: 150, B: 255, A: 255}
	ColorPeach     = color.RGBA{R: 255, G: 200, B: 150, A: 255}
	ColorGold      = color.RGBA{R: 255, G: 220, B: 100, A: 255}
	ColorRed       = color.RGBA{R: 255, G: 100, B: 100, A: 255}
	ColorGreen     = color.RGBA{R: 100, G: 220, B: 100, A: 255}
	ColorGray      = color.RGBA{R: 120, G: 120, B: 140, A: 255}
	ColorDarkGray  = color.RGBA{R: 60, G: 60, B: 80, A: 255}
	ColorBoxBG     = color.RGBA{R: 24, G: 24, B: 40, A: 230}
	ColorBoxBorder = color.RGBA{R: 140, G: 160, B: 200, A: 255}
)

// DrawBox draws a filled rectangle with a 1px border — the standard UI frame.
func DrawBox(dst *ebiten.Image, x, y, w, h int, bgColor, borderColor color.Color) {
	// Background fill
	bg := ebiten.NewImage(w, h)
	bg.Fill(bgColor)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(bg, op)

	// Border (draw 4 edges as thin rectangles)
	drawRect(dst, x, y, w, 1, borderColor)         // top
	drawRect(dst, x, y+h-1, w, 1, borderColor)     // bottom
	drawRect(dst, x, y, 1, h, borderColor)           // left
	drawRect(dst, x+w-1, y, 1, h, borderColor)       // right
}

// DrawBar draws a horizontal bar (for HP/MP display).
// fraction should be 0.0 to 1.0.
func DrawBar(dst *ebiten.Image, x, y, w, h int, fraction float64, fillColor, emptyColor color.Color) {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	// Empty background
	drawRect(dst, x, y, w, h, emptyColor)

	// Filled portion
	fillW := int(float64(w) * fraction)
	if fillW > 0 {
		drawRect(dst, x, y, fillW, h, fillColor)
	}
}

// DrawCursor draws a small right-pointing arrow cursor at (x, y).
func DrawCursor(dst *ebiten.Image, x, y int, clr color.Color) {
	DrawText(dst, ">", x, y, clr)
}

func drawRect(dst *ebiten.Image, x, y, w, h int, clr color.Color) {
	img := ebiten.NewImage(w, h)
	img.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}
