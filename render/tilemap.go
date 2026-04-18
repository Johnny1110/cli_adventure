// render/tilemap.go — Tile map rendering with camera offset.
//
// THEORY — Tile-based worlds:
// Classic RPGs (Pokemon, Final Fantasy) render the world as a grid of tiles.
// Each tile is a small square image (here 16x16 pixels). The world map is a
// 2D array of tile IDs, where each ID indexes into a tileset sprite sheet.
//
// At 160x144 resolution with 16x16 tiles, we see a 10x9 tile viewport.
// But the actual map is larger — we only render the tiles visible through
// the "camera". The camera position is typically centered on the player,
// clamped so it doesn't scroll past map edges.
//
// The rendering loop is: for each visible tile position, look up its tile ID
// in the map array, find that tile's position in the tileset, and draw the
// sub-image. Ebitengine batches these draws efficiently on the GPU.
//
// We use TWO layers:
//   - Ground layer: terrain (grass, path, water) drawn first
//   - Overlay layer: objects drawn on top (trees, signs, decorations)
// This lets us have trees that the player can walk behind (partial overlap).
package render

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	TileSize = 16 // pixels per tile edge
)

// TileMap represents a 2D tile-based map with collision data.
type TileMap struct {
	Width    int          // map width in tiles
	Height   int          // map height in tiles
	Ground   [][]int      // ground layer tile IDs (row-major)
	Overlay  [][]int      // overlay layer tile IDs (-1 = empty)
	Solid    [][]bool     // collision layer: true = impassable
	Tileset  *ebiten.Image // the tileset sprite sheet
	TilesetCols int       // number of tile columns in the tileset
}

// NewTileMap creates a tile map from layer data and a tileset.
func NewTileMap(w, h int, ground, overlay [][]int, solid [][]bool, tileset *ebiten.Image) *TileMap {
	cols := tileset.Bounds().Dx() / TileSize
	return &TileMap{
		Width:       w,
		Height:      h,
		Ground:      ground,
		Overlay:     overlay,
		Solid:       solid,
		Tileset:     tileset,
		TilesetCols: cols,
	}
}

// IsSolid returns whether the tile at (tileX, tileY) blocks movement.
func (m *TileMap) IsSolid(tileX, tileY int) bool {
	if tileX < 0 || tileY < 0 || tileX >= m.Width || tileY >= m.Height {
		return true // out of bounds = solid
	}
	return m.Solid[tileY][tileX]
}

// SetGround updates a ground tile at runtime (e.g., opening a chest).
func (m *TileMap) SetGround(tileX, tileY, tileID int) {
	if tileX >= 0 && tileX < m.Width && tileY >= 0 && tileY < m.Height {
		m.Ground[tileY][tileX] = tileID
	}
}

// Draw renders the visible portion of the map given a camera offset (in pixels).
// camX, camY is the top-left corner of the viewport in world-pixel coordinates.
func (m *TileMap) Draw(dst *ebiten.Image, camX, camY int) {
	m.drawLayer(dst, m.Ground, camX, camY)
}

// DrawOverlay renders the overlay layer (trees, decorations on top of entities).
func (m *TileMap) DrawOverlay(dst *ebiten.Image, camX, camY int) {
	m.drawLayer(dst, m.Overlay, camX, camY)
}

func (m *TileMap) drawLayer(dst *ebiten.Image, layer [][]int, camX, camY int) {
	// Calculate the range of visible tiles
	startTX := camX / TileSize
	startTY := camY / TileSize
	// +2 to handle partial tiles at edges
	endTX := startTX + (160/TileSize) + 2
	endTY := startTY + (144/TileSize) + 2

	for ty := startTY; ty < endTY; ty++ {
		for tx := startTX; tx < endTX; tx++ {
			if ty < 0 || ty >= m.Height || tx < 0 || tx >= m.Width {
				continue
			}
			tileID := layer[ty][tx]
			if tileID < 0 {
				continue // empty tile in overlay
			}

			// Find the tile's position in the tileset
			srcX := (tileID % m.TilesetCols) * TileSize
			srcY := (tileID / m.TilesetCols) * TileSize
			srcRect := image.Rect(srcX, srcY, srcX+TileSize, srcY+TileSize)

			// Draw position on screen (world position minus camera offset)
			screenX := float64(tx*TileSize - camX)
			screenY := float64(ty*TileSize - camY)

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(screenX, screenY)
			dst.DrawImage(m.Tileset.SubImage(srcRect).(*ebiten.Image), op)
		}
	}
}

// Camera tracks the viewport position, following a target smoothly.
//
// THEORY: The camera centers on the player but clamps to map edges so we
// never show void outside the map. The "target" position is where the camera
// wants to be (player center), and we lerp toward it for smooth scrolling.
// For a Game Boy feel, you can set Smooth=false for instant snapping.
type Camera struct {
	X, Y     int // current camera position (top-left of viewport, in pixels)
	MapW     int // map width in pixels (tiles * TileSize)
	MapH     int // map height in pixels
	Smooth   bool
	targetX  int
	targetY  int
}

// NewCamera creates a camera for the given map dimensions.
func NewCamera(mapW, mapH int) *Camera {
	return &Camera{
		MapW:   mapW * TileSize,
		MapH:   mapH * TileSize,
		Smooth: false, // instant snap for authentic GB feel
	}
}

// Follow updates the camera to center on (worldX, worldY) — typically the
// player's center pixel position.
func (c *Camera) Follow(worldX, worldY int) {
	// Center the viewport on the target
	c.targetX = worldX - 160/2
	c.targetY = worldY - 144/2

	// Clamp to map boundaries
	if c.targetX < 0 {
		c.targetX = 0
	}
	if c.targetY < 0 {
		c.targetY = 0
	}
	maxX := c.MapW - 160
	maxY := c.MapH - 144
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if c.targetX > maxX {
		c.targetX = maxX
	}
	if c.targetY > maxY {
		c.targetY = maxY
	}

	if c.Smooth {
		// Lerp toward target (smooth scroll)
		c.X += (c.targetX - c.X) / 4
		c.Y += (c.targetY - c.Y) / 4
		// Snap if very close
		if abs(c.X-c.targetX) <= 1 {
			c.X = c.targetX
		}
		if abs(c.Y-c.targetY) <= 1 {
			c.Y = c.targetY
		}
	} else {
		c.X = c.targetX
		c.Y = c.targetY
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
