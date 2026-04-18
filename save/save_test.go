package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cli_adventure/entity"
)

// TestSaveLoadRoundTrip verifies that saving and loading a player produces
// identical state. This is the most critical test for the save system —
// if any field is lost during serialization, the player loses progress.
func TestSaveLoadRoundTrip(t *testing.T) {
	// Use a temp directory so we don't pollute real saves
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "save1.json")

	// Create a player with non-default state
	player := entity.NewPlayer(entity.ClassMage)
	player.Level = 7
	player.XP = 42
	player.Coins = 350
	player.SkillPoints = 3
	player.Stats.HP = 15
	player.Stats.MP = 8
	player.FairyBlessing = true

	// Add items
	player.AddItem(entity.Item{Name: "Hi Potion", Type: entity.ItemConsumable, StatBoost: 40, Price: 80, ClassRestrict: -1})
	player.AddItem(entity.Item{Name: "Rune Staff", Type: entity.ItemWeapon, StatBoost: 6, Price: 200, ClassRestrict: 1})

	// Equip weapon and armor
	player.Equip(entity.Item{Name: "Oak Staff", Type: entity.ItemWeapon, StatBoost: 3, Price: 80, ClassRestrict: 1})
	player.Equip(entity.Item{Name: "Linen Robe", Type: entity.ItemArmor, StatBoost: 2, Price: 60, ClassRestrict: 1})

	// Add a quest
	player.Quests = append(player.Quests, entity.Quest{
		Name: "Slime Trouble", Target: "Slime", Required: 3, Progress: 2,
		RewardXP: 30, RewardCoin: 50,
	})

	// Learn a skill
	player.Skills = append(player.Skills, entity.PlayerSkill{ID: entity.SkillFireball, Level: 2})

	// Set day/night state
	player.DayNight.Phase = entity.PhaseDusk
	player.DayNight.StepCount = 15

	// Mark some chests opened
	player.OpenedChests["forest:3:5"] = true
	player.OpenedChests["cave:7:2"] = true

	// Snapshot
	sd := SnapshotPlayer(player, "Dark Cave")

	// Write to temp file
	b, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read back
	b2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded SaveData
	if err := json.Unmarshal(b2, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Restore player from loaded data
	restored := RestorePlayer(loaded)

	// ---- Verify all fields ----
	if loaded.CurrentArea != "Dark Cave" {
		t.Errorf("CurrentArea: got %q, want %q", loaded.CurrentArea, "Dark Cave")
	}
	if restored.Class != entity.ClassMage {
		t.Errorf("Class: got %d, want %d", restored.Class, entity.ClassMage)
	}
	if restored.Level != 7 {
		t.Errorf("Level: got %d, want 7", restored.Level)
	}
	if restored.XP != 42 {
		t.Errorf("XP: got %d, want 42", restored.XP)
	}
	if restored.Coins != 350 {
		t.Errorf("Coins: got %d, want 350", restored.Coins)
	}
	if restored.SkillPoints != 3 {
		t.Errorf("SkillPoints: got %d, want 3", restored.SkillPoints)
	}
	if restored.Stats.HP != 15 {
		t.Errorf("HP: got %d, want 15", restored.Stats.HP)
	}
	if restored.Stats.MP != 8 {
		t.Errorf("MP: got %d, want 8", restored.Stats.MP)
	}
	if !restored.FairyBlessing {
		t.Error("FairyBlessing: got false, want true")
	}

	// Items
	if len(restored.Items) != 2 {
		t.Fatalf("Items count: got %d, want 2", len(restored.Items))
	}
	if restored.Items[0].Name != "Hi Potion" {
		t.Errorf("Item[0]: got %q, want %q", restored.Items[0].Name, "Hi Potion")
	}

	// Equipment
	if restored.Weapon == nil || restored.Weapon.Name != "Oak Staff" {
		t.Errorf("Weapon: got %v, want Oak Staff", restored.Weapon)
	}
	if restored.Armor == nil || restored.Armor.Name != "Linen Robe" {
		t.Errorf("Armor: got %v, want Linen Robe", restored.Armor)
	}

	// Quest
	if len(restored.Quests) != 1 {
		t.Fatalf("Quests count: got %d, want 1", len(restored.Quests))
	}
	if restored.Quests[0].Progress != 2 {
		t.Errorf("Quest progress: got %d, want 2", restored.Quests[0].Progress)
	}

	// Skills
	if len(restored.Skills) != 1 {
		t.Fatalf("Skills count: got %d, want 1", len(restored.Skills))
	}
	if restored.Skills[0].ID != entity.SkillFireball || restored.Skills[0].Level != 2 {
		t.Errorf("Skill: got %+v, want Fireball Lv.2", restored.Skills[0])
	}

	// Day/night
	if restored.DayNight == nil {
		t.Fatal("DayNight is nil")
	}
	if restored.DayNight.Phase != entity.PhaseDusk {
		t.Errorf("DayPhase: got %d, want %d", restored.DayNight.Phase, entity.PhaseDusk)
	}
	if restored.DayNight.StepCount != 15 {
		t.Errorf("DaySteps: got %d, want 15", restored.DayNight.StepCount)
	}

	// Opened chests
	if !restored.OpenedChests["forest:3:5"] {
		t.Error("OpenedChests missing forest:3:5")
	}
	if !restored.OpenedChests["cave:7:2"] {
		t.Error("OpenedChests missing cave:7:2")
	}
}
