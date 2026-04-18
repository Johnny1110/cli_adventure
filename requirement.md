## CLI-Adventure Requirement

Act as an expert Golang developer game designer specializing in 2D pixel-art games. I want to build a Minimum Viable Product (MVP) for a cute, 8-bit RPG game rendered in a small Game Boy-style window.

Please use `hajimehoshi/ebiten/v2` (Ebitengine) as the game engine. The game runs at **160x144 logical resolution** (Game Boy), scaled up to a desktop window. All art is hand-crafted pixel-art sprite sheets — no emojis.

Here are the core requirements for the MVP:

1. Aesthetic & UI (Crucial):
- Game Boy resolution (160x144) scaled up to a visible window.
- The style must be "cute" 8-bit pixel art — chibi characters, pastel-tinted palette.
- All entities (players, NPCs, monsters, items) are pixel-art sprites loaded from PNG sprite sheets.
- Use a bitmap pixel font for all in-game text to match the retro aesthetic.
- Scalable and maintainable: adding new sprites/entities should be data-driven, not code changes.
- art-style resources:
    - https://opengameart.org/content/pixel-art-8-bit-style-pack
    - https://opengameart.org/content/pixel-art-8-bit-style-pack-2

2. Main Character (Player):
- The player can choose from 3 classes at the start:
    - Knight (High HP/Defense) — armored chibi with sword & shield
    - Mage (High Magic Damage) — robed chibi with staff
    - Archer (High Speed/Evasion) — slim chibi with bow

3. NPCs & Economy (Town State):
- Safe zones where the player can interact with NPCs (e.g., Merchant, Elder).
- NPCs can give simple quests (e.g., "Defeat 3 Slimes").
- An equipment shop where the player can trade coins for weapon/armor upgrades.

4. Combat & Exploration (Wild State):
- Areas containing normal monsters (e.g., Slime, Bat) and Bosses (e.g., Dragon).
- Simple turn-based combat.
- Character can travel between areas (towns, dungeons, etc.).
- Defeating monsters grants XP (to level up basic stats), Coins, and updates Quest progress.

Tasks for you:
1. Suggest a clean Golang project folder structure for this MVP.
2. Provide the `go mod` dependencies required.
3. Write the core `main.go` boilerplate using Ebitengine. This boilerplate should implement a simple state machine (Switching between: Main Menu -> Town/NPC -> Combat) and demonstrate sprite rendering at Game Boy resolution.
