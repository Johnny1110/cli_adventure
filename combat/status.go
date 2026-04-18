// combat/status.go — Unified status effect system for combat buffs, debuffs,
// DoTs, and status conditions.
//
// THEORY — Why a unified system:
// The original combat code used scattered ad-hoc fields: ATKBuff, ATKBuffTurns,
// AegisActive, PoisonDmg, PoisonTurns, EnemyStunned. Each was special-cased
// with its own tick-down logic, its own reset, its own query. This works for
// 3-4 effects but doesn't scale — adding "Burn" would mean yet another pair
// of fields, another tick-down check, another reset line.
//
// The unified system uses the Entity-Component pattern: one []StatusEffect
// slice per combatant, where each entry carries its own type, magnitude, and
// duration. Adding a new effect = appending one struct. Ticking = one loop.
// Querying = filter + sum. This is the same approach used in Diablo, WoW,
// and most modern RPGs with rich buff/debuff systems.
//
// THEORY — Why effects live on Engine, not Player:
// Status effects are combat-scoped. When combat ends, the Engine is garbage
// collected and all effects vanish naturally. If effects lived on Player, we'd
// need explicit cleanup — and forgetting to clean up would cause bugs where
// combat buffs leak into the overworld. The Engine is the natural owner.
//
// THEORY — Three effect categories:
//   1. StatMod: modifies a stat (ATK/DEF/SPD) by a flat amount. Positive = buff,
//      negative = debuff. Stacks additively. Duration in turns.
//   2. DoT (Damage over Time): deals damage at the start of each turn. Poison,
//      Burn, Bleed. Applied to the target (enemy DoTs damage the enemy, player
//      DoTs damage the player).
//   3. StatusCondition: binary states like Stun (skip turn), Blind (miss chance),
//      Regen (heal each turn), Aegis (halve next hit). Duration in turns.
package combat

// EffectType categorizes what an effect does.
type EffectType int

const (
	EffStatMod EffectType = iota // modifies ATK/DEF/SPD
	EffDoT                       // damage over time
	EffStatus                    // status condition (stun, blind, regen, aegis)
)

// StatTarget identifies which stat a StatMod effect modifies.
type StatTarget int

const (
	StatATK StatTarget = iota
	StatDEF
	StatSPD
)

// StatusCond identifies a specific status condition.
type StatusCond int

const (
	CondNone  StatusCond = iota
	CondStun             // skip turn
	CondBlind            // 50% miss chance on attacks
	CondRegen            // heal each turn
	CondAegis            // halve next incoming hit, consumed on trigger
)

// DoTKind identifies a specific DoT flavor (for display and stacking rules).
type DoTKind int

const (
	DoTPoison DoTKind = iota
	DoTBurn
	DoTBleed
)

// StatusEffect represents a single active effect on a combatant.
//
// THEORY — Flat struct, no inheritance:
// We could model this as an interface with StatModEffect, DoTEffect, etc.
// But Go favors flat structs with type switches, and for a small set of
// categories (3), a single struct with a discriminator is simpler, faster,
// and easier to serialize if we ever need to save mid-combat state.
// The unused fields for a given type are zero-valued and ignored.
type StatusEffect struct {
	Type EffectType
	Name string // display name: "ATK Up", "Poison", "Stun"

	// StatMod fields
	Stat  StatTarget // which stat (only for EffStatMod)
	Value int        // magnitude: +5 = buff, -3 = debuff (StatMod); damage per turn (DoT); heal per turn (Regen)

	// DoT fields
	DotKind DoTKind // flavor of DoT (only for EffDoT)

	// Status condition fields
	Condition StatusCond // which condition (only for EffStatus)

	// Duration
	Duration int // turns remaining (0 = expired, -1 = permanent until consumed)

	// Source tracking (for UI and stacking decisions)
	FromPlayer bool // true if applied by the player, false if by the enemy
}

// IsExpired returns true if this effect has run out.
func (e *StatusEffect) IsExpired() bool {
	return e.Duration == 0
}

// Tick decrements the duration by 1 (unless permanent/-1).
// Returns true if the effect just expired.
func (e *StatusEffect) Tick() bool {
	if e.Duration < 0 {
		return false // permanent until consumed
	}
	e.Duration--
	return e.Duration == 0
}

// --- Effect constructors (readable helpers) ---

// NewStatMod creates a stat modification effect.
func NewStatMod(name string, stat StatTarget, value, duration int) StatusEffect {
	return StatusEffect{
		Type:     EffStatMod,
		Name:     name,
		Stat:     stat,
		Value:    value,
		Duration: duration,
	}
}

// NewDoT creates a damage-over-time effect.
func NewDoT(name string, kind DoTKind, damage, duration int) StatusEffect {
	return StatusEffect{
		Type:     EffDoT,
		Name:     name,
		DotKind:  kind,
		Value:    damage,
		Duration: duration,
	}
}

// NewStatus creates a status condition effect.
func NewStatus(name string, cond StatusCond, duration int) StatusEffect {
	return StatusEffect{
		Type:      EffStatus,
		Name:      name,
		Condition: cond,
		Duration:  duration,
	}
}

// NewRegen creates a regeneration effect (heals Value HP per turn).
func NewRegen(name string, healPerTurn, duration int) StatusEffect {
	return StatusEffect{
		Type:      EffStatus,
		Name:      name,
		Condition: CondRegen,
		Value:     healPerTurn,
		Duration:  duration,
	}
}

// --- Effect list operations ---

// TickEffects processes all effects: applies DoT/Regen damage/healing,
// decrements durations, and removes expired effects. Returns a list of
// event messages for the UI to display.
//
// THEORY — Tick ordering:
// We apply effects BEFORE decrementing. This means a 1-turn Poison deals
// damage on the turn it's applied AND the next turn... no, actually we
// tick at the START of the affected entity's turn. So a 3-turn poison
// deals damage 3 times: once at the start of each of the next 3 turns.
// The turn it's applied, no tick happens (the skill already dealt direct
// damage). This matches Pokemon/FF behavior.
func TickEffects(effects *[]StatusEffect, currentHP, maxHP *int) []string {
	var messages []string
	alive := (*effects)[:0] // reuse backing array

	for i := range *effects {
		eff := &(*effects)[i]

		// Apply per-turn effects
		switch eff.Type {
		case EffDoT:
			*currentHP -= eff.Value
			if *currentHP < 0 {
				*currentHP = 0
			}
			messages = append(messages, eff.Name+": "+itoa(eff.Value)+" dmg!")
		case EffStatus:
			if eff.Condition == CondRegen && eff.Value > 0 {
				*currentHP += eff.Value
				if *currentHP > *maxHP {
					*currentHP = *maxHP
				}
				messages = append(messages, eff.Name+": +"+itoa(eff.Value)+" HP!")
			}
		}

		// Tick duration
		eff.Tick()

		// Keep if not expired
		if !eff.IsExpired() {
			alive = append(alive, *eff)
		} else {
			messages = append(messages, eff.Name+" wore off.")
		}
	}

	*effects = alive
	return messages
}

// SumStatMod returns the total modifier for a given stat from active effects.
// Positive = buff, negative = debuff.
func SumStatMod(effects []StatusEffect, stat StatTarget) int {
	total := 0
	for _, eff := range effects {
		if eff.Type == EffStatMod && eff.Stat == stat {
			total += eff.Value
		}
	}
	return total
}

// HasCondition checks if a specific status condition is active.
func HasCondition(effects []StatusEffect, cond StatusCond) bool {
	for _, eff := range effects {
		if eff.Type == EffStatus && eff.Condition == cond {
			return true
		}
	}
	return false
}

// ConsumeCondition removes the first instance of a condition (e.g., Aegis
// is consumed when it blocks a hit). Returns true if found and removed.
func ConsumeCondition(effects *[]StatusEffect, cond StatusCond) bool {
	for i, eff := range *effects {
		if eff.Type == EffStatus && eff.Condition == cond {
			*effects = append((*effects)[:i], (*effects)[i+1:]...)
			return true
		}
	}
	return false
}

// AddEffect appends an effect, but first checks stacking rules:
//   - DoTs of the same kind: refresh duration and take max damage (don't stack count)
//   - StatMods: stack freely (multiple ATK buffs all apply)
//   - Status conditions: refresh duration (don't double-stun)
//
// THEORY — Stacking rules:
// Unlimited stacking of DoTs would make Poison Arrow absurdly OP (just spam it).
// Instead, re-applying the same DoT refreshes its duration and upgrades damage
// if the new one is stronger. This matches WoW's DoT refresh mechanic.
// StatMods DO stack because they come from different sources (War Cry + item buff)
// and stacking makes the player feel clever for combining buffs.
// Status conditions refresh because double-stun isn't meaningfully different
// from single-stun — you still skip one turn.
func AddEffect(effects *[]StatusEffect, newEff StatusEffect) {
	switch newEff.Type {
	case EffDoT:
		// Refresh existing DoT of same kind
		for i := range *effects {
			if (*effects)[i].Type == EffDoT && (*effects)[i].DotKind == newEff.DotKind {
				if newEff.Value > (*effects)[i].Value {
					(*effects)[i].Value = newEff.Value
				}
				(*effects)[i].Duration = newEff.Duration
				(*effects)[i].Name = newEff.Name
				return
			}
		}
	case EffStatus:
		// Refresh existing condition
		for i := range *effects {
			if (*effects)[i].Type == EffStatus && (*effects)[i].Condition == newEff.Condition {
				(*effects)[i].Duration = newEff.Duration
				if newEff.Value > (*effects)[i].Value {
					(*effects)[i].Value = newEff.Value
				}
				return
			}
		}
	}
	// StatMods always stack; new DoTs/Status that didn't match append too
	*effects = append(*effects, newEff)
}

// ClearEffects removes all effects (used at combat start).
func ClearEffects(effects *[]StatusEffect) {
	*effects = (*effects)[:0]
}

// EffectSummary returns a compact display string for active effects.
// Used by the combat UI to show "ATK+5 PSN BRN STN" etc.
func EffectSummary(effects []StatusEffect) []EffectIcon {
	var icons []EffectIcon
	for _, eff := range effects {
		icon := EffectIcon{Name: eff.Name, Turns: eff.Duration}
		switch eff.Type {
		case EffStatMod:
			if eff.Value > 0 {
				icon.Color = IconBuff // green
			} else {
				icon.Color = IconDebuff // red
			}
			icon.Short = statShort(eff.Stat, eff.Value)
		case EffDoT:
			icon.Color = IconDoT // purple
			icon.Short = dotShort(eff.DotKind)
		case EffStatus:
			icon.Color = IconStatus // yellow
			icon.Short = condShort(eff.Condition)
			if eff.Condition == CondRegen {
				icon.Color = IconBuff
			}
		}
		icons = append(icons, icon)
	}
	return icons
}

// IconColorType represents the color category for UI display.
type IconColorType int

const (
	IconBuff   IconColorType = iota // green — beneficial
	IconDebuff                      // red — harmful stat mod
	IconDoT                         // purple — damage over time
	IconStatus                      // yellow/orange — status condition
)

// EffectIcon is a compact representation for the combat UI.
type EffectIcon struct {
	Name  string
	Short string        // 3-char abbreviation: "ATK", "PSN", "STN"
	Turns int           // turns remaining
	Color IconColorType // color category
}

func statShort(stat StatTarget, val int) string {
	prefix := "+"
	if val < 0 {
		prefix = ""
	}
	switch stat {
	case StatATK:
		return "ATK" + prefix + itoa(val)
	case StatDEF:
		return "DEF" + prefix + itoa(val)
	case StatSPD:
		return "SPD" + prefix + itoa(val)
	}
	return "???"
}

func dotShort(kind DoTKind) string {
	switch kind {
	case DoTPoison:
		return "PSN"
	case DoTBurn:
		return "BRN"
	case DoTBleed:
		return "BLD"
	}
	return "DOT"
}

func condShort(cond StatusCond) string {
	switch cond {
	case CondStun:
		return "STN"
	case CondBlind:
		return "BLD"
	case CondRegen:
		return "RGN"
	case CondAegis:
		return "AGS"
	}
	return "???"
}
