// entity/daynight.go — Day/night cycle system.
//
// THEORY — Step-based time of day:
// Rather than real-time, RPGs on Game Boy used step counters for day/night.
// Every N steps, the time phase advances. This ties progression to exploration:
// the more you explore, the faster night falls. It creates natural urgency —
// you know night is coming and monsters will get stronger, so you plan your
// resource usage and decide when to head home.
//
// THEORY — Four phases:
//   Day (0)   → bright, shops open, normal monsters
//   Dusk (1)  → warm orange tint, transition warning
//   Night (2) → blue tint, shops closed, monsters +50% stats, must sleep to end
//   Dawn (3)  → pink tint, brief transition back to day
//
// The cycle is: Day → Dusk → Night → (sleep) → Dawn → Day
// Night doesn't end on its own — you MUST sleep at home. This is the "survival
// feel" the user asked for. It adds tension: if you're deep in the cave at night,
// monsters are brutal and you can't buy potions. Plan ahead!
//
// THEORY — Screen tint implementation:
// Each phase has an RGBA tint color. We draw a fullscreen rectangle with this
// color on top of everything (before HUD). Alpha controls intensity: day is
// fully transparent, night is a semi-opaque blue wash. This is the same
// technique used in Pokemon Gold/Silver for its day/night system.
package entity

import "image/color"

// TimePhase represents the current time of day.
type TimePhase int

const (
	PhaseDay  TimePhase = iota // bright, normal
	PhaseDusk                  // warm orange, warning
	PhaseNight                 // blue, dangerous
	PhaseDawn                  // pink, brief transition
)

// DayNight tracks the global time cycle.
type DayNight struct {
	Phase     TimePhase
	StepCount int // steps in current phase
	Locked    bool // when true (night), phase only advances via Sleep()
}

// Steps needed to advance each phase:
//   Day:  100 steps → Dusk
//   Dusk:  30 steps → Night
//   Night: locked, only Sleep() advances it
//   Dawn:  15 steps → Day
const (
	daySteps  = 100
	duskSteps = 30
	dawnSteps = 15
)

// NewDayNight creates a day/night cycle starting at day.
func NewDayNight() *DayNight {
	return &DayNight{Phase: PhaseDay}
}

// Step advances the day/night cycle by one step.
// Call this every time the player moves one tile in the overworld.
func (dn *DayNight) Step() {
	if dn.Locked {
		return // night is locked — must sleep
	}
	dn.StepCount++

	switch dn.Phase {
	case PhaseDay:
		if dn.StepCount >= daySteps {
			dn.Phase = PhaseDusk
			dn.StepCount = 0
		}
	case PhaseDusk:
		if dn.StepCount >= duskSteps {
			dn.Phase = PhaseNight
			dn.StepCount = 0
			dn.Locked = true // night is locked!
		}
	case PhaseDawn:
		if dn.StepCount >= dawnSteps {
			dn.Phase = PhaseDay
			dn.StepCount = 0
		}
	}
}

// Sleep advances night → dawn. Only works at night.
func (dn *DayNight) Sleep() bool {
	if dn.Phase != PhaseNight {
		return false
	}
	dn.Phase = PhaseDawn
	dn.StepCount = 0
	dn.Locked = false
	return true
}

// IsNight returns true if it's night time.
func (dn *DayNight) IsNight() bool {
	return dn.Phase == PhaseNight
}

// PhaseName returns a display string for the current phase.
func (dn *DayNight) PhaseName() string {
	switch dn.Phase {
	case PhaseDay:
		return "Day"
	case PhaseDusk:
		return "Dusk"
	case PhaseNight:
		return "Night"
	case PhaseDawn:
		return "Dawn"
	}
	return ""
}

// TintColor returns the screen overlay tint for the current phase.
// The alpha channel controls intensity.
func (dn *DayNight) TintColor() color.RGBA {
	switch dn.Phase {
	case PhaseDay:
		return color.RGBA{R: 0, G: 0, B: 0, A: 0} // no tint
	case PhaseDusk:
		return color.RGBA{R: 60, G: 30, B: 0, A: 40} // warm orange
	case PhaseNight:
		return color.RGBA{R: 0, G: 0, B: 60, A: 70} // blue wash
	case PhaseDawn:
		return color.RGBA{R: 50, G: 20, B: 40, A: 30} // pink/purple
	}
	return color.RGBA{}
}

// MonsterStatMultiplier returns the monster stat boost for the current phase.
// Night makes monsters 50% stronger — the survival tension.
func (dn *DayNight) MonsterStatMultiplier() float64 {
	switch dn.Phase {
	case PhaseNight:
		return 1.5
	case PhaseDusk:
		return 1.2
	default:
		return 1.0
	}
}
