# CLI-Adventure Development Plan

> Phased checkpoint plan for the cute 8-bit RPG MVP.
> Each phase is a self-contained milestone — the game should **compile and run** after every phase.
>
> **Engine:** Ebitengine (`hajimehoshi/ebiten/v2`)
> **Resolution:** 160x144 (Game Boy) scaled up to a desktop window
> **Art style:** Hand-crafted pixel-art sprite sheets (PNG), no emojis
> **Font:** Bitmap pixel font for retro text

---

## Phase 0 — Project Scaffold & Rendering Pipeline

**Goal:** Establish the project skeleton, dependencies, and a working Ebitengine window that renders at Game Boy resolution.

**Theory — Why Ebitengine and why 160x144:**
Ebitengine implements a classic game loop via the `ebiten.Game` interface: `Update()` runs game logic at 60 TPS (ticks per second), `Draw()` renders to the screen, and `Layout()` returns the logical resolution. By returning `(160, 144)` from `Layout()`, we tell Ebitengine to render onto a tiny canvas that it then scales up to the OS window using nearest-neighbor interpolation — preserving those crisp pixel edges. This is exactly how Game Boy emulators work: small logical resolution, scaled up for display.

The `Update/Draw` split is important. `Update()` is called at a fixed rate regardless of display refresh — this means game logic (movement, combat math, timers) runs deterministically. `Draw()` can be called at a different rate and should be purely visual — no state changes. This separation prevents the classic bug where game speed depends on frame rate.

**Tasks:**
- Initialize project with `go mod init cli_adventure`.
- `go get` dependencies: `github.com/hajimehoshi/ebiten/v2`.
- Create `main.go`: boot Ebitengine with a 160x144 logical resolution, window title "CLI Adventure", scaled to ~4x (640x576 window).
- Create `engine/game.go`: the root `Game` struct implementing `ebiten.Game`.
- Create `assets/` directory structure for sprite sheets and fonts.
- Load and render a test sprite (a colored rectangle or placeholder PNG) to confirm the pipeline works.
- Verify: `go run .` opens a small window showing a colored background at the right resolution.

**Folder structure after this phase:**
```
cli_adventure/
├── main.go                 # entrypoint — ebiten.RunGame()
├── go.mod / go.sum
├── engine/
│   └── game.go             # root Game struct (Update/Draw/Layout)
├── screen/                 # one file per game screen
│   └── screen.go           # Screen interface definition
├── entity/                 # domain types (player, monster, npc, item)
├── combat/                 # combat engine logic
├── data/                   # static game data (monster tables, shop items, areas)
├── asset/
│   ├── loader.go           # sprite sheet loader + cache
│   ├── sprite/             # PNG sprite sheets
│   │   └── placeholder.png
│   └── font/               # bitmap font files
├── render/
│   ├── sprite.go           # sprite drawing helpers (sub-image, animation frames)
│   └── text.go             # bitmap text renderer
└── requirement.md / plan.md
```

**Checkpoint:** App compiles, opens a Game Boy-sized window, renders a test sprite, exits on window close.

---

## Phase 1 — Sprite System & State Machine

**Goal:** Build the sprite sheet loader, the bitmap text renderer, the game state machine, and the Main Menu screen with class selection — each class shown as a pixel-art sprite.

**Theory — Sprite sheets and sub-images:**
In 2D game dev, you don't load individual images per frame per entity. Instead, all frames for a character (idle, walk, attack) are packed into a single PNG called a **sprite sheet**. At load time, you slice it into sub-images using `img.SubImage(image.Rect(x, y, x+w, y+h))`. Ebitengine caches textures on the GPU, so sub-imaging is essentially free — it's just a pointer to a region of the same texture.

For animation, you store a list of sub-image frames and advance through them on a timer. At 60 TPS with Game Boy aesthetics, 2–4 frame animations at ~150ms per frame look great (that authentic choppy retro feel).

**Theory — State machine:**
The root `Game` struct holds a `currentScreen` field implementing a `Screen` interface (`Update()`, `Draw()`, `OnEnter()`, `OnExit()`). Switching screens means setting `currentScreen` to a new instance. The root `Update()` and `Draw()` just delegate to the current screen. This is the same "parent delegates to child" pattern from bubbletea, but with Ebitengine's draw surface instead of a string return.

**Tasks:**
- Implement `asset/loader.go`:
  - Load PNG sprite sheets via `ebitenutil.NewImageFromFile` or embedded FS.
  - `SpriteSheet` struct: source image + cell width/height + methods to get frame N.
  - Cache loaded images to avoid reloading.
- Implement `render/sprite.go`:
  - `DrawSprite(screen, sheet, frame, x, y)` — draw a specific frame at a position.
  - `Animation` struct: list of frames + timing + current frame index.
- Implement `render/text.go`:
  - Load a bitmap pixel font (e.g., a small 4x6 or 5x7 font sheet).
  - `DrawText(screen, text, x, y, color)` — render pixel text character by character.
- Define `screen/screen.go`:
  - `Screen` interface: `Update()`, `Draw(*ebiten.Image)`, `OnEnter()`, `OnExit()`.
  - `GameState` enum: `StateMenu | StateTown | StateWild | StateCombat`.
- Create placeholder pixel-art sprites for Knight, Mage, Archer (even rough 16x16 placeholders).
- Implement `screen/menu.go`:
  - Show game title in pixel font.
  - Display three class sprites side by side, cursor highlights the selected one.
  - Arrow keys to navigate, Enter/Z to confirm.
  - On selection, transition to Town screen (stub).
- Implement `entity/player.go`:
  - `Player` struct: Name, Class, HP, MaxHP, MP, ATK, DEF, SPD, Level, XP, Coins.
  - Class-specific base stats.
  - Reference to sprite sheet + current animation.

**Checkpoint:** Player can launch the game, see a pixel-art title menu, preview class sprites, pick a class, and the game transitions to a "Town" placeholder screen.

---

## Phase 2 — Town, NPCs & Shop

**Goal:** Build the Town screen — a tile-based safe zone where the player walks around, talks to NPCs, and shops for gear.

**Theory — Tile maps:**
Classic RPGs render the world as a grid of tiles (grass, stone, water, walls). Each tile is typically 16x16 pixels. At 160x144 resolution, that gives us a **10x9 tile viewport** — exactly the Game Boy's visible area. The actual map can be larger; we just render the visible portion offset by a camera following the player.

A tile map is stored as a 2D array of tile IDs. Each ID maps to a position on a **tileset** (another sprite sheet, but for terrain). Drawing the map means: for each visible tile, look up its tileset position and draw it. Ebitengine is extremely fast at this because each tile draw is just a sub-image blit.

**Theory — NPC interaction:**
NPCs are entities placed on the tile map at specific coordinates. When the player faces an NPC and presses the interact key, we trigger a dialogue state — a text box appears at the bottom of the screen (like Pokemon). The dialogue system is a small state machine itself: show text, wait for input, advance to next line, offer choices.

**Tasks:**
- Create `render/tilemap.go`:
  - `TileMap` struct: 2D grid of tile IDs + reference to tileset sprite sheet.
  - `Draw(screen, cameraX, cameraY)` — render visible tiles.
  - Simple collision layer (solid vs walkable tiles).
- Create a small town tileset (grass, path, walls, roofs, flowers — pastel palette).
- Create a town map (~20x18 tiles, 2 screens worth of scrolling).
- Implement player overworld movement:
  - Arrow keys to walk in 4 directions.
  - Grid-based movement with smooth pixel interpolation (4-frame walk animation).
  - Collision checking against solid tiles.
  - Camera follows player, clamped to map edges.
- Create NPC sprites: Merchant (cute shopkeeper), Elder (wise owl/sage character).
- Implement `entity/npc.go`:
  - NPC struct: Name, Position, Sprite, Dialogue, Role.
  - NPCs stand on the map, face the player when interacted with.
- Implement `render/dialogue.go`:
  - Dialogue box: semi-transparent box at screen bottom, pixel font text.
  - Typewriter text effect (characters appear one at a time).
  - "Press Z to continue" prompt.
  - Choice selection for branching dialogue (e.g., "Yes / No" for shop).
- Implement `screen/shop.go`:
  - Item list with pixel-art icons, names, prices.
  - Buy/sell interface using dialogue box system.
  - Deduct coins, add item to player inventory.
- Implement `entity/item.go`:
  - Item struct: Name, Type (weapon/armor/consumable), StatBoost, Price, SpriteFrame.
  - Player inventory + equip system.
- Implement basic quest system:
  - `entity/quest.go`: Quest struct — description, target, progress, reward.
  - Elder gives a quest via dialogue ("Defeat 3 Slimes").
  - Quest log accessible via pause menu.

**Checkpoint:** Player walks around a tile-based town, talks to NPCs with typewriter dialogue, accepts a quest, opens the shop, buys an item, and can see their inventory in a pause menu.

---

## Phase 3 — Wild Exploration & Encounters

**Goal:** Add overworld areas outside town (forest, cave) with random encounters that transition to combat.

**Theory — Area transitions and encounter tables:**
Each area is a separate tile map with its own tileset (forest tiles, cave tiles). Transition zones are special tiles at map edges — step on one and the game loads the next area (with a brief fade transition). Random encounters use a step counter: every N steps in a wild area, roll against an encounter table (weighted list of monsters for that area). This is how Pokemon does it — the encounter rate and monster table are per-area data, completely decoupled from the encounter/combat logic.

**Tasks:**
- Create tilesets for wild areas: Forest (trees, grass, mushrooms), Cave (stone, stalagmites, darkness).
- Create tile maps for 2-3 areas:
  - Enchanted Forest (~20x18 tiles)
  - Dark Cave (~20x18 tiles)
  - Dragon's Lair (small boss room, ~10x9 tiles)
- Implement area transitions:
  - Transition tiles at map edges trigger area loading.
  - Brief screen fade/wipe effect between areas.
  - Player spawns at the correct entry point in the new area.
- Implement random encounter system:
  - Step counter that increments with each player move in wild areas.
  - Encounter check: every 8-16 steps, roll against area's monster table.
  - On encounter, screen does the classic flash effect, then transitions to combat.
- Create monster sprites: Slime (blobby), Bat (winged), Mushroom (cap with eyes), Dragon (larger boss sprite).
- Implement `entity/monster.go`:
  - Monster struct: Name, Sprite, HP, ATK, DEF, SPD, XPReward, CoinReward, AnimationSet.
  - Monster templates in `data/monsters.go`.
- Implement `data/areas.go`:
  - Area struct: Name, TileMap, MonsterTable, Connections, EncounterRate.

**Checkpoint:** Player can leave town, walk through a pixel-art forest, trigger random encounters (screen flash), and transition to a "Combat (coming soon)" stub. Player can travel between areas and return to town.

---

## Phase 4 — Turn-Based Combat

**Goal:** Implement the core turn-based combat loop with animated sprites, HP bars, and a battle menu.

**Theory — Combat as a layered state machine:**
The combat screen is the most complex screen because it has its own internal state machine: `Intro -> PlayerTurn -> PlayerAction -> EnemyTurn -> EnemyAction -> CheckResult -> (Victory | Defeat | NextTurn)`. Each state controls what's drawn and what input is accepted. For example, during `PlayerTurn` the action menu is active; during `EnemyAction` the menu is hidden and the enemy attack animation plays.

The combat **engine** (damage formulas, turn order, stat checks) should be pure logic with no rendering — it takes inputs and returns results. The combat **screen** takes those results and turns them into animations and UI updates. This separation means the engine is trivially unit-testable and the screen is free to animate however it wants.

**Theory — Pokemon-style battle layout:**
The screen layout follows the classic formula: enemy sprite in the top-right, player sprite in the bottom-left (or just a stat panel), HP bars alongside each sprite, and a text box + action menu at the bottom. At 160x144, this fits tightly but it's exactly what Game Boy RPGs did.

**Tasks:**
- Implement `combat/engine.go` (pure logic, no rendering):
  - Turn order based on SPD stat.
  - Damage formula: `ATK * (1 + rand(0.0, 0.2)) - DEF / 2`, clamped to minimum 1.
  - Magic damage: `ATK * 1.5 - DEF / 4` (costs MP).
  - Defend: halve incoming damage next turn.
  - Flee: success chance based on SPD comparison.
  - Level-up: XP threshold formula, stat gains per class.
- Implement `screen/combat.go`:
  - Battle layout: enemy sprite (top area), player stats (bottom-left), action menu (bottom-right).
  - HP bars: colored pixel bars (green -> yellow -> red) with numeric display.
  - Action menu: Attack / Magic / Defend / Flee — cursor selection with Z to confirm.
  - Battle text box: "Slime attacks!" / "You deal 5 damage!" with typewriter effect.
  - Simple attack animation: sprite shakes, flash effect, damage number appears.
  - Victory sequence: XP/coin reward display, level-up notification if applicable.
  - Defeat sequence: "Game Over" screen with "Try Again?" option.
- Wire quest progress: defeating quest-target monsters increments progress counter.
- Wire equipment: equipped weapon/armor modifies ATK/DEF in combat calculations.

**Checkpoint:** Full combat loop — player fights monsters with animated sprites, sees damage numbers, can use all 4 actions, levels up on victory, and gets Game Over on defeat. Quest progress updates.

---

## Phase 5 — Polish, Boss Fight & Complete Game Loop

**Goal:** Close the gameplay loop, add the boss fight, and polish all screens.

**Theory — The complete loop:**
The MVP game loop is: **Menu -> Town (quest + gear) -> Wild (explore + fight) -> Town (turn in quest + upgrade) -> repeat -> Boss -> Ending**. This phase wires together every system built so far and adds the final content. The boss fight reuses the combat engine but with a beefier monster and a special attack pattern (e.g., fire breath every 3 turns). Polish means: transitions between all screens feel smooth, edge cases are handled, and the visual style is consistent.

**Tasks:**
- Wire quest completion flow:
  - Quest progress tracked across combats.
  - Return to Elder when quest complete -> dialogue acknowledges it -> reward (coins, item).
  - New quests available after completing previous ones.
- Implement boss fight:
  - Dragon boss: larger sprite (maybe 32x32), higher stats, special attack pattern.
  - Boss music change (stretch: Ebitengine audio support).
  - Cannot flee from boss fights.
- Victory ending screen:
  - Pixel-art celebration scene.
  - "Thanks for playing!" with credits scroll.
  - Return to main menu option.
- Screen transitions polish:
  - Fade to black between areas.
  - Battle encounter flash (screen blinks white).
  - Smooth menu transitions.
- UI consistency pass:
  - All text uses the bitmap font.
  - Consistent UI frame/border style across menus, shop, dialogue, combat.
  - Pastel color palette consistent across all sprite sheets.
- Edge cases:
  - Can't buy items without enough coins.
  - Can't use magic without enough MP.
  - Inventory capacity limit.
  - Heal at town (inn NPC or free heal on town entry).
- Save/Load system (stretch goal):
  - Serialize game state to JSON (`~/.cli_adventure/save.json`).
  - Save at town, load from main menu.

**Checkpoint:** Complete MVP playable from start to finish — pick class, accept quests, explore, fight, shop, defeat the Dragon, see the ending.

---

## Phase 6 — Testing & Code Quality

**Goal:** Ensure the codebase is robust, well-tested, and maintainable.

**Tasks:**
- Unit tests for `combat/engine.go`: damage formulas, turn order, level-up math, flee probability.
- Unit tests for `entity/`: player stats, inventory management, quest progress tracking, equipment modifiers.
- Unit tests for `asset/loader.go`: sprite sheet slicing, frame indexing, animation timing.
- Integration test: simulate a combat encounter programmatically (no rendering).
- Run `go vet`, `golangci-lint`, `gopls` — fix all warnings and type errors.
- Review data-driven design: confirm adding a new monster/item/area requires only:
  1. A new sprite in the sprite sheet PNG.
  2. A new data entry in `data/`.
  3. No logic changes.
- Document the sprite sheet format (cell size, layout convention, palette constraints).

**Checkpoint:** All tests pass, linter is clean, sprite/data pipeline is documented.

---

## Dependency Summary

```
require (
    github.com/hajimehoshi/ebiten/v2    v2.x    // game engine (rendering, input, audio)
)
```

Ebitengine is intentionally low-dependency. Additional Go stdlib packages (`image`, `image/png`, `encoding/json`, `math/rand`) cover the rest.

---

## Architecture Diagram

```
main.go
  └── engine/game.go (root Game struct — ebiten.Game interface)
        ├── screen/menu.go       (class selection, title screen)
        ├── screen/town.go       (overworld movement, NPC interaction)
        ├── screen/wild.go       (exploration, area transitions)
        ├── screen/combat.go     (turn-based battle UI + animation)
        │     └── combat/engine.go  (pure damage/turn logic, no rendering)
        └── screen/shop.go       (buy/sell equipment)

Rendering pipeline:
  asset/loader.go     — load & cache PNG sprite sheets
  render/sprite.go    — draw sprites, animate frame sequences
  render/tilemap.go   — draw tile-based maps with camera offset
  render/text.go      — bitmap pixel font renderer
  render/dialogue.go  — typewriter text box (Pokemon-style)

Domain:
  entity/player.go    — player stats, inventory, equipment, class
  entity/monster.go   — monster stats, sprite reference
  entity/npc.go       — NPC data, dialogue trees
  entity/item.go      — item definitions, stat boosts
  entity/quest.go     — quest tracking

Static data:
  data/monsters.go    — monster stat tables
  data/items.go       — shop inventory, item definitions
  data/areas.go       — area maps, encounter tables, connections

Assets:
  asset/sprite/*.png  — all sprite sheets (characters, monsters, tiles, items, UI)
  asset/font/*.png    — bitmap font sprite sheet
```

---

## Sprite Sheet Convention

All sprites follow a consistent format for maintainability:

- **Character sprites:** 16x16 per frame, 4 frames per direction (down, left, right, up), 4 directions = 16 frames per character. Sheet layout: 4 columns x 4 rows.
- **Monster sprites (normal):** 16x16 per frame, 2-4 animation frames (idle bob). Single row.
- **Monster sprites (boss):** 32x32 per frame, 2-4 animation frames. Single row.
- **Tile sets:** 16x16 per tile, packed in rows. Each area has its own tileset PNG.
- **Item icons:** 8x8 or 16x16, packed in a single sheet.
- **UI elements:** 8x8 tiles for window borders, cursors, arrows.

Palette: constrained to ~32 colors (pastel-leaning) shared across all sheets for visual cohesion.

---

*Each phase builds on the last. The game is runnable at every checkpoint, so you can playtest continuously as you build.*
