// combat/engine.go — Pure turn-based combat logic, no rendering.
//
// THEORY — Separation of concerns:
// The combat engine is a pure state machine that takes actions as input and
// returns results as output. It knows nothing about sprites, screens, or pixels.
// This means we can test damage formulas, turn order, and flee mechanics with
// simple Go unit tests — no Ebitengine needed.
//
// THEORY — Turn order:
// Each round, we compare SPD stats to determine who acts first. If speeds are
// equal, the player gets priority (player-friendly design). This is simpler
// than Final Fantasy's ATB gauge but still rewards investing in SPD.
//
// THEORY — Damage formula:
// We use: base = ATK * multiplier - DEF / 2, then apply ±20% randomness.
// The DEF/2 means defense is strong but never makes you invincible.
// Floor at 1 ensures every hit does *something* — no zero-damage stalemates.
// This is close to Dragon Quest's formula, known for being easy to balance.
//
// THEORY — State machine:
// Combat flows: Start → PlayerTurn → (action resolves) → EnemyTurn → (action
// resolves) → check for victory/defeat → next round. The engine tracks this
// as a CombatPhase enum so the screen knows what to animate.
package combat

import (
	"math/rand"

	"cli_adventure/entity"
)

// Phase represents the current phase of combat.
type Phase int

const (
	PhaseStart       Phase = iota
	PhasePlayerTurn        // waiting for player to pick an action
	PhasePlayerAct         // player's action is resolving
	PhaseEnemyTurn         // enemy is about to act
	PhaseEnemyAct          // enemy's action is resolving
	PhaseVictory           // monster HP <= 0
	PhaseDefeat            // player HP <= 0
	PhaseFlee              // player successfully fled
	PhaseFleeFaild         // flee failed, enemy gets a free turn
)

// Action represents a combat action the player can take.
type Action int

const (
	ActionAttack Action = iota
	ActionMagic
	ActionDefend
	ActionFlee
	ActionItem  // use potion from bag
	ActionSkill // use a specific learned skill
)

// Result holds the outcome of a single action for the screen to animate.
type Result struct {
	Action    Action
	Damage    int
	IsCrit    bool
	Healed    int
	Message   string
	TargetIsPlayer bool // true if the player took damage
}

// Engine manages the state of a single combat encounter.
type Engine struct {
	Player  *entity.Player
	Monster *entity.Monster

	Phase    Phase
	Round    int
	Results  []Result // results to animate this phase

	// State flags
	PlayerDefending bool
	EnemyGoesFirst  bool
	EnemyStunned    bool // Shield Bash stun — skip next enemy turn

	// Boss special attack tracking
	bossAttackCounter int

	// Skill system: which skill the player selected (set before calling PlayerAction)
	SelectedSkillIdx int // index into player's learned skills
}

// NewEngine creates a combat engine for a player vs monster encounter.
func NewEngine(player *entity.Player, monster *entity.Monster) *Engine {
	e := &Engine{
		Player:  player,
		Monster: monster,
		Phase:   PhaseStart,
		Round:   1,
	}
	// Determine initial turn order based on SPD
	e.EnemyGoesFirst = monster.SPD > player.Stats.SPD
	return e
}

// Start transitions from PhaseStart to the first turn.
func (e *Engine) Start() {
	if e.EnemyGoesFirst {
		e.Phase = PhaseEnemyTurn
	} else {
		e.Phase = PhasePlayerTurn
	}
}

// PlayerAction processes the player's chosen action.
// Returns the result for animation. Transitions phase afterward.
func (e *Engine) PlayerAction(action Action) Result {
	e.Results = nil
	var r Result

	// Tick down buffs at start of player's turn
	if e.Player.ATKBuffTurns > 0 {
		e.Player.ATKBuffTurns--
		if e.Player.ATKBuffTurns <= 0 {
			e.Player.ATKBuff = 0
		}
	}

	switch action {
	case ActionAttack:
		r = e.calcPlayerAttack(1.0)
	case ActionMagic:
		r = e.calcPlayerMagic()
	case ActionSkill:
		r = e.calcUseSkill()
	case ActionDefend:
		r = e.calcPlayerDefend()
	case ActionFlee:
		r = e.calcFlee()
		if r.Action == ActionFlee {
			// Successful flee
			e.Phase = PhaseFlee
			e.Results = append(e.Results, r)
			return r
		}
		// Failed flee — enemy gets a free hit
		e.Phase = PhaseFleeFaild
		e.Results = append(e.Results, r)
		return r
	case ActionItem:
		r = e.calcUsePotion()
	}

	e.Results = append(e.Results, r)
	e.Phase = PhasePlayerAct

	// Check if monster is dead
	if e.Monster.HP <= 0 {
		e.Monster.HP = 0
		e.Phase = PhaseVictory
	}

	return r
}

// EnemyAction processes the enemy's turn. Returns the result.
func (e *Engine) EnemyAction() Result {
	e.Results = nil
	var r Result

	// Boss special: fire breath every 3 rounds
	if e.Monster.IsBoss && e.bossAttackCounter >= 2 {
		r = e.calcBossSpecial()
		e.bossAttackCounter = 0
	} else {
		r = e.calcEnemyAttack()
		e.bossAttackCounter++
	}

	e.Results = append(e.Results, r)
	e.Phase = PhaseEnemyAct

	// Check if player is dead
	if e.Player.Stats.HP <= 0 {
		e.Player.Stats.HP = 0
		e.Phase = PhaseDefeat
	}

	return r
}

// AdvancePhase moves to the next logical phase after an action resolves.
// The screen calls this when it's done animating the current result.
func (e *Engine) AdvancePhase() {
	switch e.Phase {
	case PhasePlayerAct:
		if e.Monster.HP <= 0 {
			e.Phase = PhaseVictory
		} else if e.EnemyStunned {
			// Enemy is stunned — skip their turn
			e.EnemyStunned = false
			e.PlayerDefending = false
			e.Round++
			e.Phase = PhasePlayerTurn
		} else {
			e.Phase = PhaseEnemyTurn
		}
	case PhaseEnemyAct:
		if e.Player.Stats.HP <= 0 {
			e.Phase = PhaseDefeat
		} else {
			e.PlayerDefending = false
			e.Round++
			e.Phase = PhasePlayerTurn
		}
	case PhaseFleeFaild:
		// After failed flee, enemy attacks
		e.Phase = PhaseEnemyTurn
	}
}

// --- Damage calculations ---

func (e *Engine) calcPlayerAttack(multiplier float64) Result {
	atk := float64(e.Player.EffectiveATK()) * multiplier
	def := float64(e.Monster.DEF)

	// Base damage: ATK * mult - DEF/2, with ±20% randomness
	base := atk - def/2.0
	if base < 1 {
		base = 1
	}
	variance := 0.8 + rand.Float64()*0.4 // 0.8 to 1.2
	damage := int(base * variance)
	if damage < 1 {
		damage = 1
	}

	// Critical hit: 10% chance, 1.5x damage
	crit := rand.Intn(100) < 10
	if crit {
		damage = damage * 3 / 2
	}

	e.Monster.HP -= damage
	return Result{
		Action:  ActionAttack,
		Damage:  damage,
		IsCrit:  crit,
		Message: "You attack!",
	}
}

func (e *Engine) calcPlayerMagic() Result {
	// Class-specific special moves!
	switch e.Player.Class {
	case entity.ClassKnight:
		return e.calcShieldBash()
	case entity.ClassMage:
		return e.calcFireball()
	case entity.ClassArcher:
		return e.calcSnipe()
	default:
		return e.calcGenericMagic()
	}
}

// Shield Bash (Knight): damage + stun enemy for 1 turn. Costs 3 MP.
func (e *Engine) calcShieldBash() Result {
	mpCost := 3
	if e.Player.Stats.MP < mpCost {
		return Result{Action: ActionMagic, Message: "Not enough MP!"}
	}
	e.Player.Stats.MP -= mpCost

	atk := float64(e.Player.EffectiveATK()) * 0.8
	def := float64(e.Monster.DEF)
	base := atk - def/2.0
	if base < 1 { base = 1 }
	damage := int(base * (0.9 + rand.Float64()*0.2))
	if damage < 1 { damage = 1 }

	e.Monster.HP -= damage
	e.EnemyStunned = true

	return Result{
		Action:  ActionMagic,
		Damage:  damage,
		Message: "Shield Bash!\nEnemy is stunned!",
	}
}

// Fireball (Mage): high magic damage. Costs 5 MP.
func (e *Engine) calcFireball() Result {
	mpCost := 5
	if e.Player.Stats.MP < mpCost {
		return Result{Action: ActionMagic, Message: "Not enough MP!"}
	}
	e.Player.Stats.MP -= mpCost

	atk := float64(e.Player.EffectiveATK()) * 2.0
	def := float64(e.Monster.DEF)
	base := atk - def/4.0
	if base < 2 { base = 2 }
	damage := int(base * (0.9 + rand.Float64()*0.2))

	e.Monster.HP -= damage
	return Result{
		Action:  ActionMagic,
		Damage:  damage,
		Message: "Fireball!",
	}
}

// Snipe (Archer): guaranteed critical hit. Costs 4 MP.
func (e *Engine) calcSnipe() Result {
	mpCost := 4
	if e.Player.Stats.MP < mpCost {
		return Result{Action: ActionMagic, Message: "Not enough MP!"}
	}
	e.Player.Stats.MP -= mpCost

	atk := float64(e.Player.EffectiveATK())
	def := float64(e.Monster.DEF)
	base := atk - def/2.0
	if base < 1 { base = 1 }
	damage := int(base * (0.9 + rand.Float64()*0.2))
	damage = damage * 2 // guaranteed double damage crit

	e.Monster.HP -= damage
	return Result{
		Action:  ActionMagic,
		Damage:  damage,
		IsCrit:  true,
		Message: "Snipe! Critical!",
	}
}

// Generic magic (fallback)
func (e *Engine) calcGenericMagic() Result {
	mpCost := 4
	if e.Player.Stats.MP < mpCost {
		return Result{Action: ActionMagic, Message: "Not enough MP!"}
	}
	e.Player.Stats.MP -= mpCost

	atk := float64(e.Player.EffectiveATK()) * 1.5
	def := float64(e.Monster.DEF)
	base := atk - def/4.0
	if base < 1 { base = 1 }
	damage := int(base * (0.9 + rand.Float64()*0.2))
	if damage < 1 { damage = 1 }

	e.Monster.HP -= damage
	return Result{Action: ActionMagic, Damage: damage, Message: "Magic blast!"}
}

func (e *Engine) calcPlayerDefend() Result {
	e.PlayerDefending = true
	return Result{
		Action:  ActionDefend,
		Message: "You brace yourself!",
	}
}

func (e *Engine) calcFlee() Result {
	if e.Monster.IsBoss {
		return Result{
			Action:  ActionDefend, // not ActionFlee — signals failure
			Message: "Can't flee a boss!",
		}
	}

	// Flee chance based on SPD ratio: base 50%, +5% per SPD advantage
	chance := 50 + (e.Player.Stats.SPD-e.Monster.SPD)*5
	if chance < 20 {
		chance = 20
	}
	if chance > 90 {
		chance = 90
	}

	if rand.Intn(100) < chance {
		return Result{
			Action:  ActionFlee,
			Message: "Got away safely!",
		}
	}

	return Result{
		Action:  ActionDefend, // signals flee failed
		Message: "Can't escape!",
	}
}

func (e *Engine) calcUsePotion() Result {
	used := e.Player.UsePotion()
	if used == nil {
		return Result{
			Action:  ActionItem,
			Healed:  0,
			Message: "No potions!",
		}
	}
	return Result{
		Action:  ActionItem,
		Healed:  used.StatBoost,
		Message: "Used " + used.Name + "!",
	}
}

// calcUseSkill resolves a learned skill based on SelectedSkillIdx.
func (e *Engine) calcUseSkill() Result {
	learned := e.Player.LearnedSkills()
	if e.SelectedSkillIdx < 0 || e.SelectedSkillIdx >= len(learned) {
		return Result{Action: ActionSkill, Message: "No skill selected!"}
	}
	def := learned[e.SelectedSkillIdx]
	lvl := e.Player.SkillLevel(def.ID)
	if lvl <= 0 {
		return Result{Action: ActionSkill, Message: "Skill not learned!"}
	}

	mpCost := def.MPCost[lvl-1]
	if e.Player.Stats.MP < mpCost {
		return Result{Action: ActionSkill, Message: "Not enough MP!"}
	}
	e.Player.Stats.MP -= mpCost

	mult := def.Multiplier[lvl-1]

	// Dispatch based on skill ID for special effects
	switch def.ID {
	case entity.SkillShieldBash:
		return e.skillShieldBash(mult)
	case entity.SkillWarCry:
		return e.skillWarCry(mult)
	case entity.SkillHolyStrike:
		return e.skillHolyStrike(mult)
	case entity.SkillAegis:
		return e.skillAegis()
	case entity.SkillFireball:
		return e.skillMagicDamage("Fireball!", mult, 4.0)
	case entity.SkillIceShard:
		return e.skillIceShard(mult)
	case entity.SkillThunder:
		return e.skillThunder(mult)
	case entity.SkillMeteor:
		return e.skillMagicDamage("Meteor!", mult, 4.0)
	case entity.SkillSnipe:
		return e.skillSnipe(mult)
	case entity.SkillPoisonArrow:
		return e.skillPoisonArrow(mult)
	case entity.SkillMultiShot:
		return e.skillMultiShot(mult)
	case entity.SkillDeadeye:
		return e.skillDeadeye(mult)
	}

	return e.skillMagicDamage(def.Name+"!", mult, 2.0)
}

func (e *Engine) skillShieldBash(mult float64) Result {
	damage := e.calcDamage(mult, 2.0)
	e.Monster.HP -= damage
	e.EnemyStunned = true
	return Result{Action: ActionSkill, Damage: damage, Message: "Shield Bash!\nEnemy is stunned!"}
}

func (e *Engine) skillWarCry(mult float64) Result {
	// Buff ATK for 2 turns
	buff := int(mult * 3)
	e.Player.ATKBuff = buff
	e.Player.ATKBuffTurns = 2
	return Result{Action: ActionSkill, Message: "War Cry!\nATK +" + itoa(buff) + " for 2 turns!"}
}

func (e *Engine) skillHolyStrike(mult float64) Result {
	damage := e.calcDamage(mult, 2.0)
	e.Monster.HP -= damage
	// Lifesteal: heal 50% of damage dealt
	heal := damage / 2
	if heal < 1 { heal = 1 }
	e.Player.Stats.HP += heal
	if e.Player.Stats.HP > e.Player.Stats.MaxHP {
		e.Player.Stats.HP = e.Player.Stats.MaxHP
	}
	return Result{Action: ActionSkill, Damage: damage, Healed: heal, Message: "Holy Strike!\nHealed " + itoa(heal) + " HP!"}
}

func (e *Engine) skillAegis() Result {
	e.Player.AegisActive = true
	return Result{Action: ActionSkill, Message: "Aegis!\nNext hit halved!"}
}

func (e *Engine) skillMagicDamage(name string, mult, defDiv float64) Result {
	damage := e.calcMagicDamage(mult, defDiv)
	e.Monster.HP -= damage
	return Result{Action: ActionSkill, Damage: damage, Message: name}
}

func (e *Engine) skillIceShard(mult float64) Result {
	damage := e.calcMagicDamage(mult, 2.0)
	e.Monster.HP -= damage
	e.Monster.SPD -= 3
	if e.Monster.SPD < 1 { e.Monster.SPD = 1 }
	return Result{Action: ActionSkill, Damage: damage, Message: "Ice Shard!\nEnemy SPD -3!"}
}

func (e *Engine) skillThunder(mult float64) Result {
	// Ignores DEF entirely
	atk := float64(e.Player.EffectiveATK()) * mult
	damage := int(atk * (0.9 + rand.Float64()*0.2))
	if damage < 1 { damage = 1 }
	e.Monster.HP -= damage
	return Result{Action: ActionSkill, Damage: damage, Message: "Thunder!\nIgnores defense!"}
}

func (e *Engine) skillSnipe(mult float64) Result {
	damage := e.calcDamage(mult, 2.0)
	damage = damage * 2 // guaranteed crit
	e.Monster.HP -= damage
	return Result{Action: ActionSkill, Damage: damage, IsCrit: true, Message: "Snipe!\nCritical hit!"}
}

func (e *Engine) skillPoisonArrow(mult float64) Result {
	damage := e.calcDamage(mult, 2.0)
	e.Monster.HP -= damage
	e.Player.PoisonDmg = 3 + e.Player.Level // poison scales with level
	e.Player.PoisonTurns = 3
	return Result{Action: ActionSkill, Damage: damage, Message: "Poison Arrow!\nEnemy is poisoned!"}
}

func (e *Engine) skillMultiShot(mult float64) Result {
	hits := 2 + rand.Intn(2) // 2 or 3 hits
	total := 0
	for i := 0; i < hits; i++ {
		dmg := e.calcDamage(mult, 2.0)
		total += dmg
	}
	e.Monster.HP -= total
	return Result{Action: ActionSkill, Damage: total, Message: "Multi Shot!\n" + itoa(hits) + " hits!"}
}

func (e *Engine) skillDeadeye(mult float64) Result {
	damage := e.calcDamage(mult, 2.0)
	damage = damage * 3 // triple crit
	e.Monster.HP -= damage
	return Result{Action: ActionSkill, Damage: damage, IsCrit: true, Message: "Deadeye!\nTriple critical!"}
}

// calcDamage is a helper: ATK*mult - DEF/defDiv with variance.
func (e *Engine) calcDamage(mult, defDiv float64) int {
	atk := float64(e.Player.EffectiveATK()) * mult
	def := float64(e.Monster.DEF)
	base := atk - def/defDiv
	if base < 1 { base = 1 }
	damage := int(base * (0.9 + rand.Float64()*0.2))
	if damage < 1 { damage = 1 }
	return damage
}

// calcMagicDamage uses ATK*mult - DEF/defDiv (magic uses same ATK but ignores more DEF).
func (e *Engine) calcMagicDamage(mult, defDiv float64) int {
	atk := float64(e.Player.EffectiveATK()) * mult
	def := float64(e.Monster.DEF)
	base := atk - def/defDiv
	if base < 2 { base = 2 }
	damage := int(base * (0.9 + rand.Float64()*0.2))
	if damage < 1 { damage = 1 }
	return damage
}

// itoa is a tiny int-to-string helper within the combat package.
func itoa(n int) string {
	if n == 0 { return "0" }
	neg := n < 0
	if neg { n = -n }
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg { digits = append(digits, '-') }
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func (e *Engine) calcEnemyAttack() Result {
	atk := float64(e.Monster.ATK)
	def := float64(e.Player.EffectiveDEF())

	if e.PlayerDefending {
		def *= 2 // defending doubles DEF
	}

	base := atk - def/2.0
	if base < 1 {
		base = 1
	}
	variance := 0.8 + rand.Float64()*0.4
	damage := int(base * variance)
	if damage < 1 {
		damage = 1
	}

	// Aegis: halves damage from next hit
	msg := e.Monster.Name + " attacks!"
	if e.Player.AegisActive {
		damage = damage / 2
		if damage < 1 { damage = 1 }
		e.Player.AegisActive = false
		msg = e.Monster.Name + " attacks!\nAegis absorbs half!"
	}

	// Apply poison to enemy at start of their turn
	if e.Player.PoisonTurns > 0 {
		e.Monster.HP -= e.Player.PoisonDmg
		e.Player.PoisonTurns--
		if e.Player.PoisonTurns <= 0 {
			e.Player.PoisonDmg = 0
		}
	}

	e.Player.Stats.HP -= damage
	return Result{
		Action:         ActionAttack,
		Damage:         damage,
		TargetIsPlayer: true,
		Message:        msg,
	}
}

func (e *Engine) calcBossSpecial() Result {
	// Fire breath: high damage, ignores half defense
	atk := float64(e.Monster.ATK) * 1.8
	def := float64(e.Player.EffectiveDEF())

	if e.PlayerDefending {
		def *= 2
	}

	base := atk - def/4.0
	if base < 2 {
		base = 2
	}
	variance := 0.9 + rand.Float64()*0.2
	damage := int(base * variance)

	e.Player.Stats.HP -= damage
	return Result{
		Action:         ActionMagic,
		Damage:         damage,
		TargetIsPlayer: true,
		Message:        "Dragon breathes fire!",
	}
}
