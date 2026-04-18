// data/world.go — Hub-and-spoke world graph.
//
// THEORY — World as a directed graph:
// The game world is modeled as a graph where each node is a WorldArea
// and edges are directional connections (N/S/E/W). The town sits at the
// hub, with four spoke chains radiating outward:
//
//                   [Snow Mountains] ← North
//                         ↑
//   [Desert Ruins] ← W — TOWN — E → [Forest → Cave → Dragon's Lair]
//                         ↓
//                   [Swamp → Volcano] ← South
//
// This is the classic JRPG pattern (Final Fantasy I's four fiends,
// Pokemon's branching routes). It lets the player choose exploration
// order while level-gating harder content.
//
// THEORY — Soft level gates:
// Instead of hard-locking areas with story flags, we use "soft gates":
// a guard NPC warns underleveled players, but doesn't block them.
// Monsters in higher-level zones are strong enough to naturally punish
// premature entry. This respects player agency while guiding progression.
// The guard's dialogue changes once you meet the level threshold,
// providing positive reinforcement ("You look ready!").
//
// THEORY — Why BossDefeated is a map:
// We track which bosses have been defeated as a map[string]bool rather
// than a single flag. This future-proofs the system: each biome chain
// will have its own boss, and we need to track them independently for
// the "Four Seals" quest chain (Phase 13). The Dragon is just the first.
package data

// Direction represents a cardinal direction for world connections.
type Direction int

const (
	DirNorth Direction = iota
	DirSouth
	DirEast
	DirWest
)

// WorldArea describes a node in the world graph. Each area has a name,
// a map key (matching the keys in AllAreas() and EncounterTable), and
// connections to adjacent areas in cardinal directions.
type WorldArea struct {
	ID       string    // unique key: "forest", "cave", "frozen_path", etc.
	Name     string    // display name: "Enchanted Forest"
	Chain    string    // which direction chain this belongs to: "east", "north", etc.
	MinLevel int       // recommended level (0 = any level, i.e. no gate)
	BossGate string    // if non-empty, requires this boss defeated to enter (e.g. "dragon")

	// Adjacent areas by direction. Empty string = no exit in that direction.
	North string
	South string
	East  string
	West  string
}

// WorldGraph is the complete world map — every area and its connections.
// Town is the hub; it connects to the first area of each directional chain.
//
// The graph is built once at startup and never mutated. Area transitions
// read from this graph to determine which area to load next.
var WorldGraph = map[string]*WorldArea{
	// ─── Hub ───
	"town": {
		ID: "town", Name: "Peaceful Village", Chain: "hub",
		North: "frozen_path",
		South: "swamp",
		East:  "forest",
		West:  "desert",
	},

	// ─── East Chain (existing): Forest → Cave → Dragon's Lair ───
	"forest": {
		ID: "forest", Name: "Enchanted Forest", Chain: "east",
		MinLevel: 0, // starting area, no gate
		North:    "",
		South:    "cave",
		East:     "",
		West:     "town",
	},
	"cave": {
		ID: "cave", Name: "Dark Cave", Chain: "east",
		MinLevel: 4,
		North:    "forest",
		South:    "lair",
	},
	"lair": {
		ID: "lair", Name: "Dragon's Lair", Chain: "east",
		MinLevel: 6,
		North:    "cave",
	},

	// ─── North Chain: Frozen Path → Snow Mountains → Ice Cavern ───
	"frozen_path": {
		ID: "frozen_path", Name: "Frozen Path", Chain: "north",
		MinLevel: 8,
		South:    "town",
		North:    "snow_mountains",
	},
	"snow_mountains": {
		ID: "snow_mountains", Name: "Snow Mountains", Chain: "north",
		MinLevel: 10,
		South:    "frozen_path",
		North:    "ice_cavern",
	},
	"ice_cavern": {
		ID: "ice_cavern", Name: "Ice Cavern", Chain: "north",
		MinLevel: 12,
		South:    "snow_mountains",
	},

	// ─── South Chain (via east town exit): Murky Swamp → Volcano ───
	// Accessed from the east exit of town. Map exits use west/east edges.
	"swamp": {
		ID: "swamp", Name: "Murky Swamp", Chain: "south",
		MinLevel: 12,
		West:  "town",
		East:  "volcano",
	},
	"volcano": {
		ID: "volcano", Name: "Volcano Core", Chain: "south",
		MinLevel: 15,
		West: "swamp",
	},

	// ─── West Chain: Arid Desert → Sand Ruins → Buried Temple ───
	// This chain requires the Dragon boss to be defeated (post-game content).
	"desert": {
		ID: "desert", Name: "Arid Desert", Chain: "west",
		MinLevel: 10, BossGate: "dragon",
		East:  "town",
		West:  "sand_ruins",
	},
	"sand_ruins": {
		ID: "sand_ruins", Name: "Sand Ruins", Chain: "west",
		MinLevel: 14, BossGate: "dragon",
		East: "desert",
		West: "buried_temple",
	},
	"buried_temple": {
		ID: "buried_temple", Name: "Buried Temple", Chain: "west",
		MinLevel: 18, BossGate: "dragon",
		East: "sand_ruins",
	},
}

// AreaChain returns the ordered list of area IDs for a directional chain.
// Useful for the map screen to draw chains in order.
func AreaChain(dir string) []string {
	switch dir {
	case "east":
		return []string{"forest", "cave", "lair"}
	case "north":
		return []string{"frozen_path", "snow_mountains", "ice_cavern"}
	case "south":
		return []string{"swamp", "volcano"}
	case "west":
		return []string{"desert", "sand_ruins", "buried_temple"}
	}
	return nil
}

// CanEnterArea checks whether the player meets the requirements to enter
// an area. Returns (allowed, reason). If not allowed, reason is a
// human-readable message for the guard NPC dialogue.
func CanEnterArea(areaID string, playerLevel int, bossDefeated map[string]bool) (bool, string) {
	wa, ok := WorldGraph[areaID]
	if !ok {
		return true, ""
	}

	// Check boss gate first (hard requirement)
	if wa.BossGate != "" {
		if bossDefeated == nil || !bossDefeated[wa.BossGate] {
			return false, "A dark barrier blocks\nthe way... Defeat the\n" + wa.BossGate + " first."
		}
	}

	// Level gate is soft — we warn but don't block
	// The caller decides whether to block or just show the warning
	if wa.MinLevel > 0 && playerLevel < wa.MinLevel {
		return true, "Warning: Lv." + itoa(wa.MinLevel) + "+ recommended.\nDangerous monsters ahead!"
	}

	return true, ""
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
