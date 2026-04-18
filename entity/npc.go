package entity

// NPC represents a non-player character in the game world.
type NPC struct {
	Name     string
	TileX    int // position on tile map
	TileY    int
	Role     string   // "merchant", "elder", etc.
	Dialogue []string // lines of dialogue spoken in sequence
}
