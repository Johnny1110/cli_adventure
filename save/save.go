// Package save handles game state persistence — save slots, JSON serialization,
// and platform-aware file paths.
//
// THEORY — The Memento Pattern:
// We capture a snapshot of all *mutable* game state into a SaveData struct,
// serialize it to JSON, and write it to disk. Static data (monster tables,
// shop inventories, map layouts) lives in the `data` package and is never
// saved — it's reconstructed at load time. This keeps save files tiny (~2KB)
// and means we never have versioning issues with static content.
//
// THEORY — Why JSON:
// JSON is human-readable (great for debugging saves during development),
// Go's encoding/json handles it natively with struct tags, and the data is
// small enough that performance doesn't matter. Binary formats (gob, protobuf)
// would be faster but add complexity for zero practical benefit at this scale.
//
// THEORY — Save slots:
// Classic RPGs offer 3 save slots. Each slot is a separate file under
// ~/.cli_adventure/ (or the platform equivalent via os.UserConfigDir).
// The main menu reads slot summaries (class, level, area) so the player
// can pick which save to load. An empty/missing file means an unused slot.
package save

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cli_adventure/entity"
)

// MaxSlots is the number of save slots available.
const MaxSlots = 3

// ---------- Serializable snapshot types ----------

// SaveData is the top-level structure written to each save file.
// It captures everything needed to reconstruct the game state:
// the player's full state plus world-level context like current area.
type SaveData struct {
	// Where the player is right now — "town" or an area name like "Enchanted Forest".
	CurrentArea string `json:"current_area"`

	// Player state
	Class       int         `json:"class"`
	Level       int         `json:"level"`
	XP          int         `json:"xp"`
	Coins       int         `json:"coins"`
	Stats       StatSnap    `json:"stats"`
	Items       []ItemSnap  `json:"items"`
	Weapon      *ItemSnap   `json:"weapon,omitempty"`
	Armor       *ItemSnap   `json:"armor,omitempty"`
	Helmet      *ItemSnap   `json:"helmet,omitempty"`
	Boots       *ItemSnap   `json:"boots,omitempty"`
	Shield      *ItemSnap   `json:"shield,omitempty"`
	Accessory   *ItemSnap   `json:"accessory,omitempty"`
	Quests      []QuestSnap `json:"quests"`
	Skills      []SkillSnap `json:"skills"`
	SkillPoints int         `json:"skill_points"`

	// Day/night cycle
	DayPhase    int  `json:"day_phase"`
	DaySteps    int  `json:"day_steps"`
	DayLocked   bool `json:"day_locked"`

	// Exploration state
	OpenedChests  map[string]bool `json:"opened_chests"`
	FairyBlessing bool            `json:"fairy_blessing"`

	// World progression
	BossDefeated map[string]bool `json:"boss_defeated,omitempty"`
}

// StatSnap captures the player's Stats struct.
type StatSnap struct {
	MaxHP int `json:"max_hp"`
	HP    int `json:"hp"`
	MaxMP int `json:"max_mp"`
	MP    int `json:"mp"`
	ATK   int `json:"atk"`
	DEF   int `json:"def"`
	SPD   int `json:"spd"`
}

// ItemSnap captures a single item.
//
// THEORY — Forward-compatible save fields:
// We added EnhanceLevel and Consumable with `omitempty` so that old save files
// (which lack these fields) deserialize cleanly: Go's JSON decoder fills missing
// fields with zero values, which is exactly the correct default (no enhancement,
// HP consumable type). New saves include the fields. This is a lightweight form
// of schema migration — no version numbers needed because every new field has a
// meaningful zero value.
type ItemSnap struct {
	Name          string `json:"name"`
	Type          int    `json:"type"`
	StatBoost     int    `json:"stat_boost"`
	Price         int    `json:"price"`
	ClassRestrict int    `json:"class_restrict"`
	EnhanceLevel  int    `json:"enhance_level,omitempty"`
	Consumable    int    `json:"consumable,omitempty"`
	LevelReq      int    `json:"level_req,omitempty"`
	BonusStat     int    `json:"bonus_stat,omitempty"`
	BonusType     int    `json:"bonus_type,omitempty"`
	Rarity        int    `json:"rarity,omitempty"`
}

// QuestSnap captures a quest's state.
type QuestSnap struct {
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Target     string `json:"target"`
	Required   int    `json:"required"`
	Progress   int    `json:"progress"`
	RewardXP   int    `json:"reward_xp"`
	RewardCoin int    `json:"reward_coin"`
	Done       bool   `json:"done"`
}

// SkillSnap captures a learned skill.
type SkillSnap struct {
	ID    int `json:"id"`
	Level int `json:"level"`
}

// SlotSummary provides a quick overview of a save slot for the menu UI.
type SlotSummary struct {
	Used  bool
	Class int
	Level int
	Area  string
}

// ---------- Snapshot conversion: entity ↔ save ----------

// SnapshotItem converts an entity.Item to an ItemSnap.
func SnapshotItem(it entity.Item) ItemSnap {
	return ItemSnap{
		Name:          it.Name,
		Type:          int(it.Type),
		StatBoost:     it.StatBoost,
		Price:         it.Price,
		ClassRestrict: it.ClassRestrict,
		EnhanceLevel:  it.EnhanceLevel,
		Consumable:    int(it.Consumable),
		LevelReq:      it.LevelReq,
		BonusStat:     it.BonusStat,
		BonusType:     int(it.BonusType),
		Rarity:        int(it.Rarity),
	}
}

// RestoreItem converts an ItemSnap back to an entity.Item.
func RestoreItem(s ItemSnap) entity.Item {
	return entity.Item{
		Name:          s.Name,
		Type:          entity.ItemType(s.Type),
		StatBoost:     s.StatBoost,
		Price:         s.Price,
		ClassRestrict: s.ClassRestrict,
		EnhanceLevel:  s.EnhanceLevel,
		Consumable:    entity.ConsumableType(s.Consumable),
		LevelReq:      s.LevelReq,
		BonusStat:     s.BonusStat,
		BonusType:     entity.BonusStatType(s.BonusType),
		Rarity:        entity.Rarity(s.Rarity),
	}
}

// SnapshotPlayer captures the full player state into a SaveData.
// currentArea is "town" or the wild area name the player is in.
func SnapshotPlayer(p *entity.Player, currentArea string) SaveData {
	sd := SaveData{
		CurrentArea: currentArea,
		Class:       int(p.Class),
		Level:       p.Level,
		XP:          p.XP,
		Coins:       p.Coins,
		Stats: StatSnap{
			MaxHP: p.Stats.MaxHP, HP: p.Stats.HP,
			MaxMP: p.Stats.MaxMP, MP: p.Stats.MP,
			ATK: p.Stats.ATK, DEF: p.Stats.DEF, SPD: p.Stats.SPD,
		},
		SkillPoints:   p.SkillPoints,
		FairyBlessing: p.FairyBlessing,
	}

	// Items
	for _, it := range p.Items {
		sd.Items = append(sd.Items, SnapshotItem(it))
	}

	// Equipment (all 6 slots)
	if p.Weapon != nil {
		snap := SnapshotItem(*p.Weapon)
		sd.Weapon = &snap
	}
	if p.Armor != nil {
		snap := SnapshotItem(*p.Armor)
		sd.Armor = &snap
	}
	if p.Helmet != nil {
		snap := SnapshotItem(*p.Helmet)
		sd.Helmet = &snap
	}
	if p.Boots != nil {
		snap := SnapshotItem(*p.Boots)
		sd.Boots = &snap
	}
	if p.Shield != nil {
		snap := SnapshotItem(*p.Shield)
		sd.Shield = &snap
	}
	if p.Accessory != nil {
		snap := SnapshotItem(*p.Accessory)
		sd.Accessory = &snap
	}

	// Quests
	for _, q := range p.Quests {
		sd.Quests = append(sd.Quests, QuestSnap{
			Name: q.Name, Desc: q.Desc, Target: q.Target,
			Required: q.Required, Progress: q.Progress,
			RewardXP: q.RewardXP, RewardCoin: q.RewardCoin,
			Done: q.Done,
		})
	}

	// Skills
	for _, s := range p.Skills {
		sd.Skills = append(sd.Skills, SkillSnap{ID: int(s.ID), Level: s.Level})
	}

	// Day/night
	if p.DayNight != nil {
		sd.DayPhase = int(p.DayNight.Phase)
		sd.DaySteps = p.DayNight.StepCount
		sd.DayLocked = p.DayNight.Locked
	}

	// Opened chests
	if p.OpenedChests != nil {
		sd.OpenedChests = p.OpenedChests
	}

	// Boss defeats
	if p.BossDefeated != nil {
		sd.BossDefeated = p.BossDefeated
	}

	return sd
}

// RestorePlayer rebuilds a *entity.Player from a SaveData.
func RestorePlayer(sd SaveData) *entity.Player {
	p := &entity.Player{
		Class:       entity.Class(sd.Class),
		Level:       sd.Level,
		XP:          sd.XP,
		Coins:       sd.Coins,
		SkillPoints: sd.SkillPoints,
		Stats: entity.Stats{
			MaxHP: sd.Stats.MaxHP, HP: sd.Stats.HP,
			MaxMP: sd.Stats.MaxMP, MP: sd.Stats.MP,
			ATK: sd.Stats.ATK, DEF: sd.Stats.DEF, SPD: sd.Stats.SPD,
		},
		FairyBlessing: sd.FairyBlessing,
		BossDefeated:  sd.BossDefeated,
		DayNight: &entity.DayNight{
			Phase:     entity.TimePhase(sd.DayPhase),
			StepCount: sd.DaySteps,
			Locked:    sd.DayLocked,
		},
		OpenedChests: sd.OpenedChests,
	}

	if p.OpenedChests == nil {
		p.OpenedChests = map[string]bool{}
	}
	if p.BossDefeated == nil {
		p.BossDefeated = map[string]bool{}
	}

	// Items
	for _, snap := range sd.Items {
		p.Items = append(p.Items, RestoreItem(snap))
	}

	// Equipment (all 6 slots)
	if sd.Weapon != nil {
		it := RestoreItem(*sd.Weapon)
		p.Weapon = &it
	}
	if sd.Armor != nil {
		it := RestoreItem(*sd.Armor)
		p.Armor = &it
	}
	if sd.Helmet != nil {
		it := RestoreItem(*sd.Helmet)
		p.Helmet = &it
	}
	if sd.Boots != nil {
		it := RestoreItem(*sd.Boots)
		p.Boots = &it
	}
	if sd.Shield != nil {
		it := RestoreItem(*sd.Shield)
		p.Shield = &it
	}
	if sd.Accessory != nil {
		it := RestoreItem(*sd.Accessory)
		p.Accessory = &it
	}

	// Quests
	for _, qs := range sd.Quests {
		p.Quests = append(p.Quests, entity.Quest{
			Name: qs.Name, Desc: qs.Desc, Target: qs.Target,
			Required: qs.Required, Progress: qs.Progress,
			RewardXP: qs.RewardXP, RewardCoin: qs.RewardCoin,
			Done: qs.Done,
		})
	}

	// Skills
	for _, ss := range sd.Skills {
		p.Skills = append(p.Skills, entity.PlayerSkill{
			ID:    entity.SkillID(ss.ID),
			Level: ss.Level,
		})
	}

	return p
}

// ---------- File I/O ----------

// saveDir returns the path to the save directory, creating it if needed.
// Uses ~/.cli_adventure/ as the save location.
func saveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	dir := filepath.Join(home, ".cli_adventure")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create save directory: %w", err)
	}
	return dir, nil
}

// slotPath returns the file path for a given slot number (0-based).
func slotPath(slot int) (string, error) {
	dir, err := saveDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("save%d.json", slot+1)), nil
}

// Save writes a SaveData to the given slot (0-based: 0, 1, 2).
func Save(slot int, data SaveData) error {
	if slot < 0 || slot >= MaxSlots {
		return fmt.Errorf("invalid slot %d (must be 0-%d)", slot, MaxSlots-1)
	}
	path, err := slotPath(slot)
	if err != nil {
		return err
	}

	// Marshal with indentation for human-readable save files
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal save data: %w", err)
	}

	// Write atomically: write to temp file then rename.
	// This prevents corruption if the game crashes mid-write.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write save file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename save file: %w", err)
	}
	return nil
}

// Load reads a SaveData from the given slot. Returns an error if the
// slot is empty or the file is corrupt.
func Load(slot int) (SaveData, error) {
	if slot < 0 || slot >= MaxSlots {
		return SaveData{}, fmt.Errorf("invalid slot %d", slot)
	}
	path, err := slotPath(slot)
	if err != nil {
		return SaveData{}, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SaveData{}, fmt.Errorf("slot %d is empty", slot+1)
		}
		return SaveData{}, fmt.Errorf("read save file: %w", err)
	}

	var data SaveData
	if err := json.Unmarshal(b, &data); err != nil {
		return SaveData{}, fmt.Errorf("corrupt save file (slot %d): %w", slot+1, err)
	}
	return data, nil
}

// ListSlots returns a summary for each of the 3 save slots.
// Empty or corrupt slots have Used=false.
func ListSlots() [MaxSlots]SlotSummary {
	var slots [MaxSlots]SlotSummary
	for i := 0; i < MaxSlots; i++ {
		data, err := Load(i)
		if err != nil {
			continue // empty or corrupt
		}
		slots[i] = SlotSummary{
			Used:  true,
			Class: data.Class,
			Level: data.Level,
			Area:  data.CurrentArea,
		}
	}
	return slots
}

// DeleteSlot removes a save file for the given slot.
func DeleteSlot(slot int) error {
	path, err := slotPath(slot)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
