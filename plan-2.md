# CLI-Adventure Phase 2 Development Plan

> Expansion plan that picks up after Phase 6 of the original plan.
> Each phase is a self-contained milestone — the game should **compile and run** after every phase.
>
> **Prerequisite:** All features from plan.md (Phases 0–6) are complete, including:
> equipment panel, skill tree, home facility, day/night cycle, and class-specific gear.

---

## Core Rules

These rules apply globally across all phases and should be respected by every system that touches leveling, equipment, or combat.

**Max Level: 99.** The XP curve formula `level^2 * 10` scales naturally to 99. At level 99, the player has reached godlike power — no further leveling is possible. The XP bar shows "MAX" and all XP gains are discarded. This cap gives endgame a sense of finality and prevents overflow bugs in stat calculations.

**Equipment Level Requirements.** Better weapons and armor require a minimum player level to equip. This prevents a low-level player from trivializing content by buying or finding overpowered gear early. Every `Item` has a `ReqLevel int` field — the `Equip()` method checks `player.Level >= item.ReqLevel` before allowing equip. If the check fails, the UI shows "Requires Lv.X" in red. This is the same system used in Diablo, World of Warcraft, and most loot-driven RPGs. It creates a progression incentive: "I found an amazing sword, now I need 3 more levels to use it."

**Equipment Rarity Classes.** Every item has a `Rarity` from lowest to highest: White → Blue → Pink → Orange → Gold → Rainbow. Rarity determines the item's name color in the UI, its stat power, and how it's obtained. White items are shop-bought commons. Blue and above come from drops and chests, with higher rarities having exponentially lower drop rates. This is the loot-color system pioneered by Diablo II and adopted by virtually every RPG since — it works because color-coding lets players instantly assess loot value at a glance.

---

## Phase 7 — Save/Load System

**Goal:** Persist game state to disk so the player can quit and resume later. Save at any safe location (town), load from the main menu.

**Theory — Serialization strategy:**
Games serialize state into a snapshot file. The simplest approach is JSON — it's human-readable (great for debugging), Go's `encoding/json` handles it natively, and the data is small (a few KB). We serialize the entire `Player` struct plus world state (opened chests, completed quests, current area, day/night phase) into a single JSON blob.

The tricky part is **what to save vs. what to reconstruct**. Static data (monster tables, shop inventories, map layouts) never changes — it lives in `data/` and doesn't need saving. Only *mutable player state* gets serialized. This is the Memento pattern: capture the object's internal state without exposing its structure, so you can restore it later.

**Theory — Save slots:**
Classic RPGs offer 3 save slots. Each slot stores a separate JSON file (`save1.json`, `save2.json`, `save3.json`) under a platform-appropriate directory (`~/.cli_adventure/` on Linux/Mac, `%APPDATA%/cli_adventure/` on Windows). The main menu shows each slot's summary (class, level, play time) so the player can pick which save to load.

**Theory — Save points vs. save anywhere:**
We use "save at town" — the player must return to a safe zone to save. This is a deliberate design choice from classic RPGs (Final Fantasy, Dragon Quest). It adds weight to exploration: venturing deep into a dungeon without saving creates tension. If you die, you lose progress back to your last save. This risk/reward loop is core to the survival feel the game is building.

**Tasks:**
- Create `save/save.go`:
  - `SaveData` struct: Player snapshot, current area ID, opened chests, quest states, day/night state, play time.
  - `Save(slot int, data SaveData) error` — marshal to JSON, write to file.
  - `Load(slot int) (SaveData, error)` — read file, unmarshal.
  - `ListSlots() []SlotSummary` — scan save directory, return summaries.
  - Platform-aware save directory via `os.UserConfigDir()`.
- Add save prompt to town:
  - Interact with a save crystal/monument NPC in town center.
  - Dialogue confirms "Save your progress?" with Yes/No.
- Add load option to main menu:
  - "New Game / Continue" on the title screen.
  - "Continue" opens a slot picker showing class icon, level, and play time.
- Add auto-save on area transition (entering town) as a safety net.
- Handle missing/corrupt save files gracefully (warn + offer new game).

**Checkpoint:** Player can save in town, quit the game, relaunch, and continue from exactly where they left off — same level, inventory, quests, day/night phase, and opened chests.

---

## Phase 8 — Post-Boss Continuation & Expanded World Structure

**Goal:** Remove the hard ending after the Dragon boss. Instead, defeating the boss unlocks new regions. Build the framework for a multi-area world with directional exits from town.

**Theory — Open-world structure:**
The original plan has a linear path: Town → Forest → Cave → Dragon's Lair → End. For a richer game, the town becomes a **hub** with exits in all four cardinal directions, each leading to a different biome chain. This is the classic JRPG world map pattern (think Final Fantasy I's four fiends or Pokemon's routes):

```
                  [Snow Mountains] ← North
                        ↑
[Desert Ruins] ← West — TOWN — East → [Enchanted Forest → Dark Cave → Dragon's Lair]
                        ↓
                  [Swamp → Volcano] ← South
```

Each direction has 2–3 connected areas of increasing difficulty. The existing south path (Forest → Cave → Dragon) becomes the "East" chain. New chains are stubs for now — placeholder maps with correct connections wired up.

**Theory — Level gating:**
Instead of hard-locking areas behind story flags, we use **soft level gates**: a guard NPC at the entrance to dangerous zones warns the player ("These mountains are treacherous — only seasoned warriors survive"). The player *can* enter, but monsters are scaled such that underleveled players will get destroyed. This respects player agency while naturally guiding progression. The guard's dialogue changes once you're strong enough ("You look ready — be careful in there").

**Tasks:**
- Modify the Dragon boss victory to unlock new areas rather than ending the game:
  - Victory dialogue: "The dragon falls... but dark energy stirs in distant lands."
  - Set a `BossDefeated` flag on the player. Post-boss, new content activates.
- Add north, west, and south exits to the town map:
  - Modify `data/maps.go` to add exit tiles on all four edges.
  - Each exit leads to a new area chain.
- Create area framework:
  - **East (existing):** Enchanted Forest → Dark Cave → Dragon's Lair
  - **North:** Frozen Path → Snow Mountains → Ice Cavern (unlocked at level 8+)
  - **South:** Murky Swamp → Volcano Entrance → Volcano Core (unlocked at level 12+)
  - **West:** Arid Desert → Sand Ruins → Buried Temple (unlocked post-boss)
- Implement `data/world.go`:
  - `WorldArea` struct: ID, Name, TileMapRef, Connections (N/S/E/W → area ID), MinLevel, RequiresFlag.
  - `WorldGraph` linking all areas with their connections.
- Add level-gate guard NPCs at dangerous zone entrances.
- Create stub tile maps for new areas (can reuse/recolor existing tilesets initially).
- Set max level to 99:
  - Update `GainXP()` to stop granting XP at level 99.
  - Update `LevelUp()` to cap at 99.
  - Update XP bar rendering to show "MAX" at level 99.

**Checkpoint:** After beating the Dragon, the game continues. The player can explore exit paths in all four directions from town. New areas load with correct transitions and have placeholder content. Level cap is enforced at 99.

---

## Phase 9 — Expanded NPC & Shop System

**Goal:** Add specialized merchant NPCs across the world. Different sellers stock different categories of equipment, creating a reason to visit multiple towns and areas.

**Theory — Economic specialization:**
In real RPGs, different towns specialize in different goods. This serves two design purposes: it gives each location a distinct identity, and it creates a travel incentive — you *need* to visit the desert town for fire-resistant armor, even if you're grinding in the snow mountains. The economy becomes a web of reasons to explore.

Specialized shops also solve a UI problem: a single shop with 50 items is overwhelming. Five shops with 10 items each is manageable. The constraint forces the player to make interesting choices about what to buy *now* vs. what to save for.

**Theory — NPC roles as a type system:**
Each NPC has a `Role` string that drives their behavior: "weapon_seller", "armor_seller", "potion_seller", "mage_shop", "archer_shop", "blacksmith", "innkeeper". The interaction handler switches on this role. This is data-driven — adding a new shop type means adding a role string and a filtered inventory function, not rewriting interaction logic.

**Tasks:**
- Add new NPC roles and interaction handlers:
  - `potion_seller` — sells only consumables (potions, elixirs, antidotes, ethers).
  - `weapon_seller` — sells weapons for all classes.
  - `armor_seller` — sells armor for all classes.
  - `mage_shop` — sells mage-only equipment + MP potions.
  - `archer_shop` — sells archer-only equipment + special arrows.
  - `blacksmith` — upgrades existing equipment for a cost (stretch goal).
- Create item filter functions in `data/items.go`:
  - `PotionShopItems()`, `WeaponShopItems(class)`, `ArmorShopItems(class)`, etc.
- Place specialized NPCs across the world:
  - Town: general merchant (current), potion seller.
  - Snow area: armor specialist (cold-resistant gear).
  - Desert area: weapon specialist (fire-imbued weapons).
  - Swamp area: consumable specialist (antidotes, status cures).
- Add shop-closed-at-night behavior to all new merchants.
- Create distinct NPC sprites for each shop type (palette swaps of existing merchant are fine initially).

**Checkpoint:** Multiple specialized shops exist across the world. Each stocks a distinct category of goods. Players must visit different areas to access different equipment.

---

## Phase 10 — Extended Equipment Slots & Level Requirements

**Goal:** Expand from 2 equipment slots (weapon, armor) to 8 slots: weapon, armor, helmet, shoes, belt, ring, necklace, bracelet. Each slot provides distinct stat bonuses. All equipment now requires a minimum level to equip.

**Theory — Slot-based equipment systems:**
Classic RPGs use multiple equipment slots to create a rich character-building minigame. Each slot contributes different stat bonuses, and the player assembles a "build" from their equipped set. The key insight is that *each slot should serve a distinct mechanical purpose* — if two slots both just add ATK, there's no interesting choice. Here's the design:

| Slot      | Primary Stat | Secondary Effect |
|-----------|-------------|-----------------|
| Weapon    | ATK         | Class-specific damage type |
| Armor     | DEF         | HP bonus on heavy armors |
| Helmet    | DEF (small) | Status resistance |
| Shoes     | SPD         | Encounter rate modifier |
| Belt      | MaxHP       | Potion effectiveness bonus |
| Ring      | ATK or DEF  | Elemental affinity |
| Necklace  | MaxMP       | MP regen per turn |
| Bracelet  | Mixed       | Crit chance or dodge bonus |

This design means the player is always making trade-offs: do I want more speed (shoes) or more bulk (belt)? More MP for skills (necklace) or more crit damage (bracelet)? These decisions are the fun of equipment systems.

**Theory — Level-gated equipment:**
Every item now has a `ReqLevel` field. Tier 1 gear requires level 1 (starter), tier 2 requires ~level 5, tier 3 ~level 10, and so on up to endgame gear requiring level 40+. The `Equip()` method checks this before allowing equip. The shop UI shows items the player can't yet equip in a dimmed color with a "Lv.X" tag — this is a *forward motivation* technique. The player sees the powerful gear they'll soon be able to use, which drives them to keep leveling. It's the same dopamine loop as locked items in a skill tree.

**Theory — Stat aggregation:**
With 8 slots, we need a clean way to compute total stats. Instead of `EffectiveATK()` checking weapon + armor + ring + bracelet individually, we compute `EquipmentBonuses() Stats` — a single method that loops over all equipped items and sums their contributions into a `Stats` struct. Then `EffectiveATK() = base.ATK + equipBonus.ATK + tempBuffs`. This is the **composite pattern** — aggregate many small effects into one total.

**Tasks:**
- Extend `entity/item.go`:
  - Add `ReqLevel int` field to `Item`.
  - Add `CanEquipLevel(playerLevel int) bool` method.
  - Add new `ItemType` constants: `ItemHelmet`, `ItemShoes`, `ItemBelt`, `ItemRing`, `ItemNecklace`, `ItemBracelet`.
  - Add `StatBonuses Stats` field to `Item` (replaces single `StatBoost int` for new slots — allows multi-stat items).
- Extend `entity/player.go`:
  - Add equipment fields: `Helmet`, `Shoes`, `Belt`, `Ring`, `Necklace`, `Bracelet *Item`.
  - Add `EquipmentBonuses() Stats` — sums all slot contributions.
  - Refactor `EffectiveATK()` and `EffectiveDEF()` to use `EquipmentBonuses()`.
  - Add `EffectiveSPD()`, `EffectiveMaxHP()`, `EffectiveMaxMP()`.
  - Update `Equip()` to check `ReqLevel`.
- Extend `data/items.go` with accessory items and level requirements:
  - Tier 1 (Lv.1): Bronze Ring (+2 ATK), Leather Belt (+10 HP), etc.
  - Tier 2 (Lv.5): Silver Ring (+4 ATK), Chain Belt (+20 HP), etc.
  - Tier 3 (Lv.10): Gold Ring (+7 ATK), Mithril Belt (+35 HP), etc.
  - Add `ReqLevel` to all existing weapons and armor.
- Update `screen/equip.go`:
  - Show all 8 slots in a scrollable list.
  - Each slot shows: slot name, item name (or "—Empty—"), stat bonuses.
  - Show "Req Lv.X" in red if player level is too low.
- Update `screen/bag.go`:
  - Equip logic routes items to the correct slot based on `ItemType`.
  - Block equip with a message if level requirement not met.
- Update shop screens:
  - Dim unequippable items (too high level) but still allow purchase (buying ahead).
  - Show level requirement next to price.
- Update save/load to serialize all equipment slots.

**Checkpoint:** Player can equip items in all 8 slots. Each slot visibly affects stats. Equipment has level requirements enforced. The equipment panel shows the full loadout with level gates visible.

---

## Phase 11 — Equipment Rarity System

**Goal:** Add a color-coded rarity system to all equipment. Higher rarity means stronger stats, bonus attributes, and rarer drop rates. This transforms loot from a linear upgrade path into a treasure-hunting minigame.

**Theory — Why rarity works:**
The rarity system is one of the most psychologically effective mechanics in RPG design. It exploits the **variable-ratio reinforcement schedule** — the same principle behind slot machines. The player knows that *any* enemy could drop a rare item, so every encounter has a little thrill of anticipation. The color-coded tiers create an instant visual hierarchy that triggers excitement: seeing a gold-colored item name pop up after a boss fight feels incredible, even before the player reads the stats.

Diablo II invented the modern rarity color system (white/blue/yellow/green/gold), and it's been adopted by Path of Exile, Borderlands, Destiny, Genshin Impact, and hundreds of other games. The reason it persists is that it works on a fundamental level — it makes loot *exciting*.

**Theory — Rarity tiers and their mechanics:**

| Rarity   | Color        | Source                  | Stat Bonus      | Special Attributes |
|----------|-------------|------------------------|-----------------|-------------------|
| White    | White (#FFF) | Shops, common drops     | Base stats only  | None |
| Blue     | Blue (#4488FF) | Uncommon drops, chests  | +10–20% bonus   | 1 minor buff |
| Pink     | Pink (#FF44CC) | Rare drops, boss chests | +25–40% bonus   | 1–2 buffs |
| Orange   | Orange (#FF8800) | Boss drops (low rate)   | +50–75% bonus   | 2 buffs, may include debuff immunity |
| Gold     | Gold (#FFD700) | Boss drops (very rare)  | +100% bonus     | 2–3 powerful buffs |
| Rainbow  | Cycling colors | Superboss/dungeon only  | +150% bonus     | 3 buffs, unique passive effect |

**Theory — Stat bonus scaling:**
A White Iron Sword might have ATK +3. A Blue Iron Sword has ATK +3 plus a 15% bonus = ATK +3 (rounded up to +4) plus a minor buff like "Crit Rate +2%". A Gold Iron Sword has ATK +3 doubled = ATK +6 plus two powerful buffs like "Crit Rate +8%, Lifesteal 5%". The base item defines the foundation; the rarity multiplies it and layers on bonus effects. This means every item drop is potentially exciting — even a low-tier weapon in Gold rarity might be worth using.

**Theory — Drop rate design:**
Drop rates follow an exponential decay: White (60%), Blue (25%), Pink (10%), Orange (4%), Gold (0.9%), Rainbow (0.1%). Boss monsters have a separate, more generous loot table: they always drop at least Blue quality, with a ~5% chance at Gold and ~0.5% at Rainbow. This means boss fights are the primary source of rare loot — giving the player strong motivation to seek out and defeat bosses.

**Tasks:**
- Create `entity/rarity.go`:
  - `Rarity int` enum: `RarityWhite`, `RarityBlue`, `RarityPink`, `RarityOrange`, `RarityGold`, `RarityRainbow`.
  - `RarityInfo` struct: Name, Color (for text rendering), StatMultiplier, MaxBuffSlots.
  - `RarityTable` mapping each rarity to its info.
  - `RarityColor(r Rarity) color.RGBA` — returns the display color.
  - `RollRarity(isBosDrop bool) Rarity` — weighted random roll using the drop rate tables.
- Extend `entity/item.go`:
  - Add `Rarity Rarity` field to `Item`.
  - Add `Buffs []ItemBuff` field — bonus effects granted by the item while equipped.
  - `ItemBuff` struct: `BuffID`, `Value`, `Description`.
  - `GenerateRarityItem(baseItem Item, rarity Rarity) Item` — takes a base item and rolls rarity-appropriate stat bonuses and buffs.
- Create buff definitions for equipment:
  - Minor buffs (Blue): Crit Rate +2%, Dodge +2%, HP Regen +1/turn, MP Regen +1/turn.
  - Major buffs (Orange/Gold): Crit Rate +8%, Lifesteal 5%, Poison Immunity, Stun Immunity.
  - Legendary buffs (Rainbow): unique passives like "Double Strike 10% chance", "Reflect 15% damage".
- Update rendering for rarity:
  - Item names drawn in their rarity color everywhere: inventory, shop, equip panel, loot drops.
  - Rainbow rarity cycles through hue over time (animated text color) — 1-pixel-font color shift per frame.
  - Add a rarity icon/border in the equipment panel (colored gem or star beside the name).
- Update loot drop system:
  - `combat/engine.go`: boss drops roll from `RollRarity(true)`, normal drops from `RollRarity(false)`.
  - Loot popup shows the item name in rarity color with a brief "glow" effect for Orange+ drops.
- Update shop system:
  - Shops sell only White rarity items. Better rarity must be found through drops and exploration.
  - This creates the core loop: buy White gear to survive → fight bosses → hope for rare drops → equip upgrades.
- Update save/load to serialize item rarity and buff data.

**Checkpoint:** All items have a rarity tier. Item names glow in their rarity color throughout the UI. Boss drops roll rarity with appropriate rates. Blue+ items have bonus attributes. Rainbow items are legendary chase targets.

---

## Phase 12 — Combat Buff & Debuff System

**Goal:** Implement a rich buff/debuff system where both players and monsters can have timed status effects that alter combat mechanics. Effects come from skills, equipment, and monster abilities.

**Theory — Status effects as combat depth:**
Without buffs and debuffs, combat is a DPS race: highest ATK wins. Status effects add a *strategic layer* — they change what the optimal move is on each turn. If the enemy poisons you, you need to decide: heal now (lose a damage turn) or push through and kill it before the poison kills you? If you're bleeding, should you use a turn to apply a healing buff or try to end the fight faster? Every debuff creates a **decision point**, and decisions are what make combat interesting.

The classic status effect systems (Final Fantasy, Pokemon, Chrono Trigger) all follow the same pattern: each effect has a type, a potency (how strong), and a duration (how many turns it lasts). Effects tick at the start or end of each turn, then decrement their duration counter. When duration hits 0, the effect expires. Multiple effects can stack (you can be poisoned AND bleeding), but the same effect type typically doesn't stack — a second poison application just refreshes the duration.

**Theory — Buff/debuff architecture:**
We model all effects as a single `StatusEffect` struct with a `Type` field. This is the **Strategy pattern**: the effect type determines the behavior, and the combat engine dispatches on it. Both `Player` and `Monster` hold a `[]StatusEffect` slice. At the start of each turn, the engine iterates over active effects, applies their per-turn logic, and decrements durations. This unified approach means the exact same code handles player buffs and monster buffs — no duplication.

**Debuffs (harmful):**

| Effect     | Per-Turn Logic                           | Typical Duration | Source Examples |
|------------|------------------------------------------|-----------------|-----------------|
| Bleed      | Lose 5% MaxHP per turn                   | 3 turns         | Monster claw attack, Scorpion sting |
| Poison     | Lose flat damage per turn (scales with source ATK) | 3–5 turns | Poison Arrow skill, Toxic Frog attack |
| Dizziness  | 30% chance to miss your action each turn | 2 turns         | Shield Bash skill, heavy hit stagger |
| Confusion  | 25% chance to hit yourself instead of enemy | 2–3 turns    | Mummy curse, Will-o-Wisp attack |
| Freeze     | Skip turn entirely (hard CC)             | 1 turn          | Ice Shard skill, Ice Wyrm breath |
| Burn       | Lose flat damage + ATK reduced by 10%    | 2–3 turns       | Fireball skill, Volcano monsters |
| Slow       | SPD halved (act last in turn order)      | 2 turns         | Swamp Bog Lurker, Frost Golem |

**Buffs (beneficial):**

| Effect       | Logic                                    | Typical Duration | Source Examples |
|-------------|------------------------------------------|-----------------|-----------------|
| Dodge Up     | +X% chance to completely avoid attacks   | 3 turns         | Archer Deadeye stance, Wind Cloak equipment |
| Crit Up      | +X% critical hit chance                  | 3 turns         | War Cry skill, Bracelet of Fury equipment |
| HP Regen     | Recover X HP per turn                    | 3–5 turns       | Fairy Blessing, Belt of Recovery equipment |
| MP Regen     | Recover X MP per turn                    | 3–5 turns       | Necklace of Wisdom equipment, Mage passive |
| Immunity     | Immune to all debuffs                    | 2 turns         | Aegis skill upgrade, Rainbow-rarity shields |
| ATK Up       | ATK increased by X%                      | 2–3 turns       | War Cry skill (replaces old flat buff) |
| DEF Up       | DEF increased by X%                      | 2–3 turns       | Defend action upgrade, Holy Armor passive |
| Reflect      | Reflect X% of damage back to attacker    | 1–2 turns       | Gold/Rainbow shields, Mage spell |

**Theory — Why monsters get buffs too:**
If only the player has status effects, combat becomes one-directional: debuff the enemy, then kill. When monsters can buff themselves and debuff the player, every encounter becomes a two-way tactical exchange. A boss that casts ATK Up on itself creates urgency — you need to kill it fast or apply a debuff to counter. A monster that applies Bleed forces you to bring healing. This **symmetry** is what separates deep combat from simple combat.

**Theory — Duration balancing:**
Short durations (1–2 turns) make effects feel urgent and tactical — "I need to act NOW before the buff wears off." Long durations (4–5 turns) create sustained pressure — "I'm bleeding and I need a plan." Hard CC (Freeze, Dizziness) should always be short (1–2 turns) because losing your turn entirely feels terrible if it lasts too long. Damage-over-time effects (Bleed, Poison, Burn) can last longer because they still let you act each turn.

**Tasks:**
- Create `entity/status.go`:
  - `StatusType int` enum: `StatusBleed`, `StatusPoison`, `StatusDizzy`, `StatusConfusion`, `StatusFreeze`, `StatusBurn`, `StatusSlow`, `StatusDodgeUp`, `StatusCritUp`, `StatusHPRegen`, `StatusMPRegen`, `StatusImmunity`, `StatusATKUp`, `StatusDEFUp`, `StatusReflect`.
  - `StatusEffect` struct: `Type StatusType`, `Potency int` (strength/percentage), `Duration int` (remaining turns), `SourceName string` (for UI: "Poisoned by Toxic Frog").
  - `IsDebuff(t StatusType) bool` — returns true for harmful effects.
  - `StatusName(t StatusType) string` — display name.
  - `StatusColor(t StatusType) color.RGBA` — icon/text color (red for debuffs, green/blue for buffs).
- Create `entity/statuslist.go`:
  - `StatusList []StatusEffect` type with helper methods:
  - `Add(effect StatusEffect)` — adds or refreshes duration if same type already active (no stacking same type).
  - `Tick()` — called each turn: decrement all durations, remove expired effects.
  - `Has(t StatusType) bool` — check if an effect is active.
  - `Get(t StatusType) *StatusEffect` — get the active effect of a type (for reading potency).
  - `Clear(t StatusType)` — remove a specific effect (for cure items).
  - `ClearDebuffs()` — remove all debuffs (for Immunity or cure-all items).
- Extend `entity/player.go` and `entity/monster.go`:
  - Add `Statuses StatusList` field to both.
  - Remove the old `ATKBuff`, `ATKBuffTurns`, `AegisActive`, `PoisonDmg`, `PoisonTurns` fields from Player — these are now handled by the unified StatusEffect system.
  - `ResetCombatBuffs()` → `ResetStatuses()` — clears all effects at combat start.
- Update `combat/engine.go`:
  - Add `applyStatusEffects(target)` called at the start of each turn:
    - Bleed: deal `potency`% of MaxHP as damage.
    - Poison: deal `potency` flat damage.
    - Burn: deal `potency` damage + reduce effective ATK by 10%.
    - HP Regen: heal `potency` HP.
    - MP Regen: heal `potency` MP.
    - Reflect: store reflected damage to apply after next hit.
  - Add status checks before actions:
    - Freeze: skip turn entirely, remove effect.
    - Dizziness: roll 30% miss chance, log "missed due to dizziness!"
    - Confusion: roll 25% self-hit chance, deal self-damage.
    - Slow: modify turn order calculation.
  - Add status checks during damage:
    - Dodge Up: roll dodge chance before applying damage.
    - Crit Up: add potency to crit roll.
    - DEF Up: multiply effective DEF by (1 + potency/100).
    - ATK Up: multiply effective ATK by (1 + potency/100).
  - Call `target.Statuses.Tick()` at end of each turn.
- Refactor existing skills to use StatusEffect:
  - War Cry → applies `StatusATKUp{Potency: 25, Duration: 2}` (25% ATK boost for 2 turns).
  - Aegis → applies `StatusDEFUp{Potency: 50, Duration: 1}` (50% DEF boost for 1 turn).
  - Poison Arrow → applies `StatusPoison{Potency: 5, Duration: 3}` on the enemy.
  - Shield Bash → applies `StatusDizzy{Potency: 30, Duration: 1}` (30% miss chance).
  - Ice Shard → applies `StatusSlow{Potency: 50, Duration: 2}` (50% SPD reduction).
- Add monster skills that inflict debuffs:
  - Toxic Frog: "Toxic Spit" → Poison (3 turns).
  - Ice Wolf: "Frost Bite" → Slow (2 turns).
  - Mummy: "Ancient Curse" → Confusion (2 turns).
  - Sand Scorpion: "Venomous Sting" → Bleed (3 turns).
  - Boss special attacks: Ice Wyrm "Blizzard" → Freeze (1 turn), Hydra "Acid Spray" → Burn (3 turns).
- Add monster self-buffs:
  - Dragon: "Dragon Rage" → ATK Up 50% for 2 turns.
  - Frost Golem: "Ice Armor" → DEF Up 30% for 3 turns.
  - Sphinx: "Riddle of Power" → Crit Up 25% for 2 turns.
- Wire equipment buffs:
  - Items with `Buffs []ItemBuff` from Phase 11 (rarity system) apply their effects as permanent status effects during combat.
  - Equipment buffs don't have duration — they last the entire combat, refreshed each turn.
  - Example: a Gold Necklace with "MP Regen +3" applies `StatusMPRegen{Potency: 3, Duration: 999}`.
- Add consumable items for status management:
  - "Antidote" — clears Poison.
  - "Smelling Salts" — clears Dizziness and Confusion.
  - "Panacea" — clears all debuffs.
  - "Warmth Potion" — clears Freeze and Burn.
- Update combat UI (`screen/combat.go`):
  - Show active status icons above/below HP bars for both player and enemy.
  - Each icon is a tiny 8x8 sprite (skull for debuffs, shield for buffs) colored by status type.
  - Show remaining duration as a small number beside the icon.
  - Status application/removal gets a brief text log: "You are poisoned! (3 turns)".
  - Show damage-over-time ticks in the battle log: "Poison deals 5 damage!"
- Update save/load to serialize active statuses (relevant for overworld persistent effects, if any).

**Checkpoint:** Both player and monsters can have multiple simultaneous buffs and debuffs. Status icons show in combat UI with durations. Skills and equipment apply appropriate effects. Cure items cleanse debuffs. The old hardcoded buff fields are fully replaced by the unified system.

---

## Phase 13 — Expanded Monsters, Quests & Story

**Goal:** Populate the world with diverse monsters, multi-step quest chains, and a narrative arc that gives the player purpose beyond grinding.

**Theory — Monster design philosophy:**
Good monster design follows a few principles. First, **visual theme matches biome**: forest has plant/insect monsters, caves have bats/golems, snow has wolves/ice elementals. Second, **mechanical variety**: some monsters hit hard but are slow (tank archetype), others are fast but fragile (glass cannon), some inflict status effects (debuffers). This forces the player to adapt strategy per encounter rather than spamming Attack. Third, **scaling within a biome**: each area has 3–4 monster types at slightly different power levels, creating a difficulty gradient as you explore deeper.

**Theory — Quest design:**
The original quest system is "kill N monsters." Richer quests add **fetch quests** (find item X in area Y), **escort quests** (guide NPC from A to B), **discovery quests** (find the hidden shrine), and **boss quests** (defeat the area boss). Multi-step quest chains create narrative momentum: "Find the ancient sword" → "Bring it to the blacksmith" → "Use it to defeat the Ice Dragon." Each step gives partial rewards, and the chain culminates in something significant (unique item, new area access, stat boost).

**Tasks:**
- Create monster tables for each biome (all monsters now have status effect abilities from Phase 12):
  - **Forest (existing):** Slime, Bat, Mushroom, Boss: Dragon.
  - **Snow:** Ice Wolf (fast, applies Slow), Frost Golem (high DEF, self-buffs DEF Up), Snow Harpy (debuffs SPD), Boss: Ice Wyrm (Blizzard → Freeze).
  - **Swamp:** Toxic Frog (Poison), Bog Lurker (high HP, applies Bleed), Will-o-Wisp (Confusion), Boss: Hydra (Acid Spray → Burn, multi-attack).
  - **Desert:** Sand Scorpion (Bleed + crit chance), Dust Devil (high dodge, applies Dizziness), Mummy (Confusion curse), Boss: Sphinx (self-buffs Crit Up, Riddle mechanic).
- Create monster sprites for new enemies (16x16, 2–4 frame idle animations).
- Implement quest chain system:
  - `Quest.ChainID` and `Quest.Step` fields for multi-step quests.
  - Completing step N auto-assigns step N+1.
  - NPCs recognize quest chain state in their dialogue.
- Add 3–4 quest chains:
  - **Main story:** "The Four Seals" — defeat each biome boss to break a seal. All four seals broken unlocks the final dungeon.
  - **Side quest:** "The Lost Heirloom" — find items across multiple biomes for a reward.
  - **Class quest:** Class-specific quest chain that grants a unique ultimate skill.
  - **Collector quest:** "Monster Compendium" — encounter every monster type.
- Add a quest log screen (Q key) showing active, completed, and available quests.

**Checkpoint:** Each biome has 3–4 unique monsters with distinct behaviors and status effects. Multiple quest chains drive the player across the world. A main story arc connects all biomes through the Four Seals narrative.

---

## Phase 14 — High-Level Dungeons

**Goal:** Add endgame dungeon areas with elevated difficulty and proportionally greater rewards. These are the "prestige content" that gives high-level players something to strive for.

**Theory — Risk/reward scaling:**
Dungeons are optional, dangerous areas with the best loot in the game. The design follows the **risk/reward curve**: the harder the content, the better the reward, but death means losing progress (back to last save). This creates a gambling-like tension — "do I push deeper for better loot, or leave now and save?" Classic examples: Lufia's Ancient Cave, Pokemon's Unknown Dungeon, Final Fantasy's bonus dungeons.

**Theory — Dungeon mechanics:**
Dungeons differ from regular wild areas in several ways: higher encounter rate, no save points inside, stronger monster variants (palette-swapped with boosted stats), environmental hazards (damage tiles, one-way doors, dark rooms requiring a light source), and treasure chests with rare/unique items. The dungeon itself is a resource-management puzzle — you have limited potions and MP, and every fight drains you. Debuffs carry between fights in dungeons (no auto-cleanse), making status cure items critical to bring.

**Tasks:**
- Create 2–3 endgame dungeons:
  - **Abyssal Cavern** (accessible post-boss, level 15+): 3 floors, each a 20x18 map connected by staircase tiles. Strongest base monsters, rare weapon drops.
  - **Tower of Trials** (level 20+): 5 floors of increasingly brutal encounters. Each floor has a mini-boss guarding the stairs. Top floor has a superboss with unique loot.
  - **Shadow Realm** (hidden, requires quest item to enter): Single floor labyrinth with palette-inverted monsters at 2x stats. Contains the best equipment in the game (Gold/Rainbow rarity guaranteed).
- Implement dungeon-specific mechanics:
  - **No save inside** — entering a dungeon warns the player to save first.
  - **Floor transitions** — staircase tiles move between dungeon floors.
  - **Damage tiles** — lava/poison floor tiles that deal 1 HP per step (applies Burn or Poison debuff).
  - **Treasure chests** — unique items only found in dungeons, guaranteed Blue+ rarity.
  - **Mini-bosses** — non-random encounters at fixed map positions, guaranteed Orange+ drops.
- Create palette-swap monster variants:
  - Recolor existing sprites with a tint shader or pre-made palette variants.
  - Stats multiplied by 1.5x–2.0x of their base-area counterparts.
  - Enhanced status effect abilities (longer durations, higher potency).
- Add dungeon-exclusive equipment:
  - Items that can't be bought, only found. These should be exciting and build-defining.
  - Examples: "Vampire Blade" (Gold weapon, Lifesteal 10% passive), "Aegis Shield" (Gold armor, auto-Immunity 1 turn per combat), "Speed Boots" (Gold shoes, permanent Dodge Up +15%).

**Checkpoint:** Endgame dungeons are explorable, dangerous, and rewarding. High-level players have meaningful content to pursue after the main story. Dungeon loot integrates with the rarity and buff systems.

---

## Phase 15 — Visual Equipment on Sprites

**Goal:** Make the player's equipped weapon and armor visually appear on the character sprite in the overworld and in combat. Equipment rarity affects the visual glow/tint.

**Theory — Layered sprite composition:**
Instead of having a unique sprite for every equipment combination (which would be hundreds of sprites), we use **layered rendering**. The character is drawn as a base body sprite, then the armor is drawn on top as a separate overlay sprite, then the weapon is drawn on top of that. Each layer lines up with the same animation frame. This is how games like Terraria and Stardew Valley handle visual equipment — it's modular and extensible.

The layering order matters: body → armor → weapon (→ helmet → accessories if visible). Each equipment item references an overlay sprite sheet with the same frame layout as the character (16 frames: 4 directions × 4 walk frames). When the player equips a new sword, only the weapon overlay sprite reference changes.

**Theory — Rarity visual distinction:**
Beyond palette-shifting for tiers, rarity adds a subtle glow effect: White items render normally, Blue items have a faint blue shimmer, Orange items pulse with a warm glow, Gold items have a golden particle effect, and Rainbow items cycle through a hue-shifting overlay. This means you can *see* another player's gear quality at a glance (important for future multiplayer). On a Game Boy resolution screen, the glow is just 1–2 extra pixels around the sprite in the rarity color, toggled on/off every 30 frames for a subtle sparkle.

**Tasks:**
- Create overlay sprite sheets:
  - Weapon overlays: sword (3 tiers), staff (3 tiers), bow (3 tiers) — 9 sheets total.
  - Armor overlays: heavy armor (3 tiers), robe (3 tiers), light armor (3 tiers) — 9 sheets total.
  - Each sheet follows the same 4×4 frame layout as the character base sprite.
- Extend `entity/item.go`:
  - Add `OverlaySpriteSheet` field to `Item` — references the overlay asset.
  - Add `OverlayPalette` for palette-swap tiers.
- Extend the rendering pipeline:
  - `render/sprite.go`: `DrawLayeredSprite(screen, baseSheet, overlays[], frame, x, y)`.
  - Overlay sprites drawn with the same `DrawImageOptions` (position, frame) as the base.
  - Add rarity glow rendering: `DrawRarityGlow(screen, rarity, x, y, frame)`.
- Update `screen/town.go` and `screen/wild.go`:
  - Player draw call uses layered rendering with current equipment overlays.
- Update `screen/combat.go`:
  - Player combat sprite shows equipped weapon during attack animation.
  - Weapon swing animation overlay during the attack action.
- Handle unequipped state gracefully (draw base sprite only, no overlay crash).

**Checkpoint:** The player's character visually changes when equipping different weapons and armor. Different equipment tiers and rarities are visually distinguishable in the overworld and combat.

---

## Phase 16 — Refined UI with Stat Bars

**Goal:** Replace numeric-only stat displays with visual bars for HP, MP, XP, and SP. Add a persistent HUD with level badge and status effect icons. Make all UI elements consistent and polished.

**Theory — Why bars matter:**
Numeric displays ("HP: 23/30") require mental math to assess urgency. A colored bar conveys health status *instantly* — glance at the bar, know you're in trouble. The bar color reinforces this: green (safe) → yellow (caution) → red (danger). This is fundamental UX: **reduce cognitive load for frequently-checked information**.

At Game Boy resolution (160x144), screen real estate is precious. The HUD should be minimal but information-dense. A thin bar takes less vertical space than a text line while communicating more. The classic layout: HP bar (green), MP bar (blue), and XP bar (yellow) stacked in 2–3 pixel-tall strips, with the level number beside them.

**Theory — Bar rendering:**
A stat bar is three rectangles drawn in sequence: background (dark, full width), fill (colored, proportional width), and border (1px frame). The fill width is `(currentValue / maxValue) * barWidth`. Color interpolation from green→yellow→red uses the ratio: if ratio > 0.5 use green-to-yellow lerp, else yellow-to-red lerp. This gives a smooth visual gradient as health drops.

**Tasks:**
- Create `render/bar.go`:
  - `DrawBar(screen, x, y, width, height, ratio, colorFull, colorEmpty)` — generic bar renderer.
  - `DrawHPBar`, `DrawMPBar`, `DrawXPBar` — preset wrappers with appropriate colors.
  - HP bar: green (#40C040) → yellow (#C0C040) → red (#C04040).
  - MP bar: solid blue (#4040C0) fill.
  - XP bar: solid yellow (#C0C040) fill.
  - SP display: numeric with pip indicators (since SP is small numbers).
- Create persistent HUD component:
  - Top-left: class icon (8x8) + level number (shows "Lv.99" at max).
  - Below icon: HP bar + numeric overlay ("23/30" drawn tiny inside the bar).
  - Below HP: MP bar + numeric overlay.
  - Bottom of HUD: XP bar showing progress to next level (or "MAX" at 99).
  - Right side: day/night phase indicator with icon.
  - Status effect row: small icons for active buffs/debuffs with turn counters (from Phase 12).
- Update `screen/town.go`, `screen/wild.go`:
  - Replace text-based stat display with bar-based HUD.
  - Ensure HUD doesn't overlap with game content (reserve top 16px or use transparency).
- Update `screen/combat.go`:
  - Player and enemy HP bars use the new bar renderer.
  - Add MP bar below HP bar for the player.
  - Status effect icons row below HP/MP bars for both player and enemy.
  - Animate bar changes (smooth decrease/increase over ~10 frames) for satisfying feedback.
- Update `screen/equip.go`:
  - Stat comparison bars: when hovering a new item, show current bar vs. projected bar side by side.
  - Green arrow for stat increase, red arrow for decrease.
  - Show item rarity color and buff list.
  - Show level requirement with color coding (green = can equip, red = too low).
- Add consistent UI frame/border style:
  - 9-slice border sprite for all dialog boxes, menus, and panels.
  - Consistent padding and margins.

**Checkpoint:** All screens use visual bars for HP/MP/XP. The HUD is clean, informative, and fits the Game Boy aesthetic. Status effects are visible. Bar animations provide satisfying feedback during combat.

---

## Phase 17 — Second Town & World Polish

**Goal:** Add a second town in a different biome as a waypoint and restock point. Polish the overall world flow.

**Theory — Multiple towns as progression markers:**
A second town serves three purposes: it gives the player a new save point deeper in the world, it provides access to better equipment (higher-tier shops), and it signals narrative progress ("you've reached a new civilization"). The second town should feel distinct from the first — different tileset palette, different NPC personalities, different shop specializations. If the first town is a green pastoral village, the second might be a snowy mountain outpost or a desert trading post.

**Tasks:**
- Create a second town tileset (palette variation of town tiles — e.g., snow theme or desert theme).
- Create a second town map (20x18 tiles) with:
  - Different layout from the first town (L-shaped paths, elevated terrain, etc.).
  - Inn (save point + rest), specialized shops, quest NPCs.
  - Exits connecting to surrounding wild areas.
- Add NPCs:
  - Higher-tier merchants with better equipment (higher ReqLevel items available).
  - New quest givers with area-specific quest chains.
  - Lore NPCs that flesh out the world's backstory.
  - Status cure specialist NPC (sells Antidotes, Smelling Salts, Panaceas).
- Wire up save/load to work in the new town.
- Add a world map screen (M key):
  - Simple overview showing discovered areas and connections.
  - Fast travel between visited towns (unlocked after discovering both).

**Checkpoint:** Two distinct towns exist in the world. The second town has its own shops, quests, and identity. Players can fast-travel between discovered towns.

---

## Phase 18 — Final Polish & Balance

**Goal:** Balance all systems, fix edge cases, and ensure the complete game is fun from start to endgame.

**Tasks:**
- **Balance pass:** playtest every biome at intended levels. Adjust monster stats, XP rewards, shop prices, and equipment power to ensure smooth progression from level 1 to 99.
- **Difficulty curve audit:** chart player power vs. monster power across levels 1–99. Smooth any spikes or plateaus. Ensure the level 99 cap feels earned, not grindy.
- **Equipment balance:** ensure each equipment slot contributes meaningfully. No slot should feel mandatory or useless. Verify level requirements create natural progression gates.
- **Rarity balance:** confirm drop rates feel rewarding but not trivial. A Gold drop should feel exciting, not routine. Verify Blue+ stat bonuses are meaningful but not game-breaking.
- **Status effect balance:** ensure no single debuff is oppressive (Confusion self-hits shouldn't one-shot). Verify cure items are available before debuff-heavy areas. Ensure boss status attacks are telegraphed ("Hydra is gathering acid...").
- **Skill balance:** verify each class's skill tree feels distinct and all 4 skills per class are worth investing in. Skills that apply status effects should be competitive with pure-damage skills.
- **Economy balance:** players should feel gold-constrained early (interesting choices) and wealthy late (power fantasy).
- **Quest flow audit:** ensure quest chains don't dead-end and the main story has clear breadcrumbs guiding the player.
- **Edge case sweep:**
  - Full inventory handling (can't pick up items when full).
  - Save corruption recovery.
  - Stat overflow guards (HP can't exceed MaxHP, etc.).
  - Equipment unequip flow (remove item from slot back to inventory).
  - Level 99 edge cases (XP display, skill points, stat growth).
  - Status effect edge cases (what if player has Immunity + someone tries to debuff? What if Confusion + Freeze overlap?).
- **Visual consistency pass:** all sprites, UI elements, rarity colors, and status icons are cohesive.
- **Performance check:** ensure 60 TPS is maintained even with layered sprites, multiple overlays, bar animations, and status effect processing.

**Checkpoint:** The game is balanced, polished, and playable from character creation through endgame dungeons without any dead ends, softlocks, or balance-breaking exploits.

---

## Architecture Additions

```
save/
  save.go              — JSON serialization, save slots, platform paths

data/
  world.go             — WorldArea graph, connections, level gates
  items.go             — expanded with accessories, level reqs, rarity variants
  monsters.go          — expanded with biome-specific monster tables + status abilities
  quests.go            — quest chain definitions

entity/
  player.go            — 8 equipment slots, EquipmentBonuses(), level cap 99
  item.go              — new ItemTypes, Rarity, ReqLevel, StatBonuses, ItemBuff
  rarity.go            — Rarity enum, color table, drop rate tables, stat multipliers
  status.go            — StatusType enum, StatusEffect struct, debuff/buff definitions
  statuslist.go        — StatusList with Add/Tick/Has/Get/Clear helpers
  monster.go           — extended with Statuses field and skill abilities
  quest.go             — ChainID, Step fields for multi-step quests

combat/
  engine.go            — status effect processing, monster buff/debuff abilities

render/
  bar.go               — HP/MP/XP stat bar renderer
  hud.go               — persistent HUD component + status icons
  sprite.go            — extended with DrawLayeredSprite() + rarity glow

screen/
  questlog.go          — quest log UI (Q key)
  worldmap.go          — world map overview (M key)
  combat.go            — status effect icons, rarity-colored loot popups
  equip.go             — 8 slots, level req display, rarity colors
  bag.go               — level req check on equip, rarity display
```

---

## Milestone Summary

| Phase | Name                          | Key Deliverable                              |
|-------|-------------------------------|----------------------------------------------|
| 7     | Save/Load                     | Persistent game state across sessions        |
| 8     | Post-Boss & World Structure   | Hub-and-spoke world, level cap 99            |
| 9     | NPC & Shop Expansion          | Specialized merchants across the world       |
| 10    | Equipment Slots & Level Reqs  | 8 slots with level-gated equipping           |
| 11    | Equipment Rarity              | White→Blue→Pink→Orange→Gold→Rainbow loot      |
| 12    | Combat Buff & Debuff System   | Timed status effects for player and monsters |
| 13    | Monsters, Quests & Story      | Rich content: 16+ monsters, quest chains     |
| 14    | High-Level Dungeons           | Endgame content with risk/reward tension      |
| 15    | Visual Equipment              | Layered sprite rendering + rarity glow       |
| 16    | Refined UI                    | Stat bars, animated HUD, status icons        |
| 17    | Second Town & World Polish    | Multiple towns, fast travel, world map        |
| 18    | Final Polish & Balance        | Balanced, polished, complete experience       |

---

*Each phase builds on the last. The game remains runnable at every checkpoint. Phases 7–8 are structural foundations; 9–12 are core systems (equipment, rarity, buffs); 13–14 are content expansion; 15–16 are visual/UX upgrades; 17–18 are world-building and polish.*
