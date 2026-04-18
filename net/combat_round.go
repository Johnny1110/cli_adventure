// net/combat_round.go — Host-side state machine for team-play combat.
//
// THEORY — Simultaneous-decision rounds:
// Every round has two phases:
//
//   COLLECT (10 seconds): all players choose an action independently
//   — Attack / Defend / Flee / Skill. A player who doesn't submit in
//   time auto-Defends (see `defaultTimeoutAction`).
//
//   RESOLVE: the host plays out the round in SPD order. Each participant
//   acts once, then the monster picks its target (80% single, 20% AOE)
//   with a 10% chance that its chosen attack lands as a critical.
//
// A "round log" is emitted step-by-step via the shared session state — the
// broadcast loop forwards each log line to every peer and their MP combat
// screen animates the snapshot. Because each resolve step updates the
// authoritative HP and pushes a log line, clients always see the fight
// unfold exactly as the host computed it, without any client-side math.
//
// THEORY — Scope:
// We only mirror the most common combat verbs (Attack, Defend, Flee, and
// the class's signature skill). The rich skill tree and MP-cost
// bookkeeping in combat/engine.go stay single-player for now; a future
// pass can mirror more of it via MsgCombatAction.SkillID.
package net

import (
	"math/rand"
	"strconv"
	"time"
)

// Timing constants. The host broadcast loop ticks at 10 Hz, so each tick
// is 100 ms. A second is therefore 10 ticks.
const (
	hostTicksPerSecond    = 10
	roundCollectSeconds   = 10 // decision window per round
	roundResolveStepTicks = 7  // ~700ms per action step (readable cadence)
	roundEndHoldTicks     = 15 // short pause after last action
)

// defaultTimeoutAction is applied to any player who didn't pick in time.
var defaultTimeoutAction = CombatDefend

// roundMachine drives the MP fight on the host. Methods are called from
// the Host's broadcast loop (single goroutine) so internal state doesn't
// need its own mutex; it goes through the Host's mutex when touching
// peer state.
type roundMachine struct {
	host *Host

	phase       string // "collect"|"resolve"|"ended"
	roundNum    int
	tickCount   uint64 // ticks since this phase started
	collectEnd  uint64 // tickCount value at which collect deadline expires
	roundSeed   int64  // RNG seed for this round (reserved for replay)

	// Resolve-phase script: a queued list of actions to play out, including
	// the monster turn as the last entry.
	queue   []roundEvent
	queueAt int

	// Tick at which the current resolve step finishes.
	stepDeadline uint64

	// True after an end was detected this tick (victory/defeat) so the
	// broadcast loop knows to stop the machine.
	done bool
}

// roundEvent is a single atomic action to play at resolve time.
type roundEvent struct {
	actorID   string // "host" or peer ID ("" for monster)
	actorName string // display name of the actor
	kind      string // "attack"|"defend"|"flee"|"skill"|"monster_single"|"monster_aoe"|"monster_crit_single"|"monster_crit_aoe"
	targetID  string // for monster_single / monster_crit_single
}

func newRoundMachine(h *Host) *roundMachine {
	m := &roundMachine{
		host:      h,
		phase:     RoundPhaseCollect,
		roundNum:  1,
		tickCount: 0,
		roundSeed: time.Now().UnixNano(),
	}
	m.collectEnd = uint64(roundCollectSeconds * hostTicksPerSecond)
	m.pushCombatSnapshot("") // seed with current state
	return m
}

// tick advances the state machine by one host broadcast tick.
func (m *roundMachine) tick(_ uint64) {
	if m.done {
		return
	}
	m.tickCount++

	switch m.phase {
	case RoundPhaseCollect:
		m.tickCollect()
	case RoundPhaseResolve:
		m.tickResolve()
	}
}

// ---------------- Collect phase ----------------

func (m *roundMachine) tickCollect() {
	// How many seconds remain in the current collection window?
	var secsLeft int
	if m.collectEnd > m.tickCount {
		remainingTicks := int(m.collectEnd - m.tickCount)
		secsLeft = (remainingTicks + hostTicksPerSecond - 1) / hostTicksPerSecond
	}

	// If the host or any peer hasn't picked yet, the window stays open
	// until either (a) everyone alive has submitted OR (b) the deadline
	// expires. If the deadline expires, auto-fill Defend for anyone
	// still un-ready.
	if m.tickCount >= m.collectEnd {
		m.finalizeTimeouts()
		m.beginResolve()
		return
	}

	if m.allAliveReady() {
		m.beginResolve()
		return
	}

	// Keep pushing the ticking snapshot so clients can render the counter.
	m.pushCombatSnapshot("")
	m.host.session.setCombat(m.withPhase(RoundPhaseCollect, secsLeft))
}

// allAliveReady returns true when every non-fled, non-dead participant
// has an action locked in.
func (m *roundMachine) allAliveReady() bool {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hostSelf.HP > 0 && !h.hostFled && !h.hostReady {
		return false
	}
	for _, p := range h.peers {
		if p.hp <= 0 || p.fled {
			continue
		}
		if !p.ready {
			return false
		}
	}
	return true
}

func (m *roundMachine) finalizeTimeouts() {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hostSelf.HP > 0 && !h.hostFled && !h.hostReady {
		h.hostAction = defaultTimeoutAction
		h.hostReady = true
	}
	for _, p := range h.peers {
		if p.hp <= 0 || p.fled {
			continue
		}
		if !p.ready {
			p.action = defaultTimeoutAction
			p.hasAction = true
			p.ready = true
		}
	}
}

// ---------------- Resolve phase ----------------

type actorSlot struct {
	id     string
	name   string
	spd    int
	action CombatActionKind
	alive  bool
}

func (m *roundMachine) beginResolve() {
	h := m.host
	h.mu.Lock()
	// Snapshot who acts this round (exclude fled/dead).
	actors := make([]actorSlot, 0, len(h.peers)+1)
	if h.hostSelf.HP > 0 && !h.hostFled {
		actors = append(actors, actorSlot{
			id:     "host",
			name:   m.actorName("host", h.hostSelf.Name),
			spd:    h.hostSelf.SPD,
			action: h.hostAction,
			alive:  true,
		})
	}
	for _, p := range h.peers {
		if p.hp <= 0 || p.fled {
			continue
		}
		actors = append(actors, actorSlot{
			id:     p.id,
			name:   m.actorName(p.id, p.name),
			spd:    p.spd,
			action: p.action,
			alive:  true,
		})
	}
	h.mu.Unlock()

	// Sort actors by SPD descending; stable so equal SPD keeps host-first ordering.
	sortSlots(actors)

	events := make([]roundEvent, 0, len(actors)+1)
	for _, s := range actors {
		events = append(events, roundEvent{
			actorID:   s.id,
			actorName: s.name,
			kind:      resolveKind(s.action),
		})
	}
	// Monster turn: roll AOE vs single + critical
	if h.monsterHP > 0 {
		monK, tgt := m.rollMonsterTurn()
		events = append(events, roundEvent{
			actorID:   "",
			actorName: h.monsterName,
			kind:      monK,
			targetID:  tgt,
		})
	}

	m.queue = events
	m.queueAt = 0
	m.phase = RoundPhaseResolve
	m.stepDeadline = m.tickCount + uint64(roundResolveStepTicks)
	m.host.session.setCombat(m.withPhase(RoundPhaseResolve, 0))
}

// actorName returns the display name for a given participant.
func (m *roundMachine) actorName(id, fallback string) string {
	if fallback != "" {
		return fallback
	}
	return id
}

func (m *roundMachine) tickResolve() {
	if m.tickCount < m.stepDeadline {
		return
	}
	// Advance one step.
	if m.queueAt >= len(m.queue) {
		// Round over — check end conditions.
		if m.checkEnd() {
			return
		}
		m.beginNextRound()
		return
	}
	ev := m.queue[m.queueAt]
	m.queueAt++
	m.applyEvent(ev)
	m.stepDeadline = m.tickCount + uint64(roundResolveStepTicks)

	// If someone died mid-resolve, we let the rest of the queued events
	// still play (missing targets are skipped). After the last step we
	// re-check outcomes.
}

// beginNextRound wipes ready flags and reopens the collect window.
func (m *roundMachine) beginNextRound() {
	h := m.host
	h.mu.Lock()
	h.hostReady = false
	h.hostAction = ""
	for _, p := range h.peers {
		p.ready = false
		p.hasAction = false
		p.action = ""
	}
	h.mu.Unlock()

	m.roundNum++
	m.phase = RoundPhaseCollect
	m.tickCount = 0
	m.collectEnd = uint64(roundCollectSeconds * hostTicksPerSecond)
	m.pushCombatSnapshot("Round " + strconv.Itoa(m.roundNum) + "!")
	m.host.session.setCombat(m.withPhase(RoundPhaseCollect, roundCollectSeconds))
}

// applyEvent mutates authoritative state and pushes a log line.
func (m *roundMachine) applyEvent(ev roundEvent) {
	h := m.host
	var log string

	switch ev.kind {
	case "attack":
		log = m.doPlayerAttack(ev.actorID, ev.actorName)
	case "skill":
		log = m.doPlayerSkill(ev.actorID, ev.actorName)
	case "defend":
		h.mu.Lock()
		// Mark defending by bumping DEF temporarily via a side flag on
		// hostSelf/peer: we don't keep a full buff system here, instead
		// the monster turn halves damage to defenders (see monsterHit).
		if ev.actorID == "host" {
			// nothing special — the monster turn checks h.hostAction
		}
		h.mu.Unlock()
		log = ev.actorName + " braces."
	case "flee":
		log = m.doPlayerFlee(ev.actorID, ev.actorName)
	case "monster_single":
		if m.host.monsterHP <= 0 {
			log = m.host.monsterName + " collapses!"
		} else {
			log = m.doMonsterSingle(ev.targetID, false)
		}
	case "monster_crit_single":
		if m.host.monsterHP <= 0 {
			log = m.host.monsterName + " collapses!"
		} else {
			log = m.doMonsterSingle(ev.targetID, true)
		}
	case "monster_aoe":
		if m.host.monsterHP <= 0 {
			log = m.host.monsterName + " collapses!"
		} else {
			log = m.doMonsterAOE(false)
		}
	case "monster_crit_aoe":
		if m.host.monsterHP <= 0 {
			log = m.host.monsterName + " collapses!"
		} else {
			log = m.doMonsterAOE(true)
		}
	}
	m.pushCombatSnapshot(log)
}

// ---------------- Attack / skill / flee ----------------

func (m *roundMachine) doPlayerAttack(actorID, actorName string) string {
	h := m.host
	if h.monsterHP <= 0 {
		return actorName + " swings at thin air."
	}
	atk := m.actorATK(actorID)
	def := h.monsterDEF
	base := float64(atk) - float64(def)/2.0
	if base < 1 {
		base = 1
	}
	dmg := int(base * (0.8 + rand.Float64()*0.4))
	if dmg < 1 {
		dmg = 1
	}
	crit := rand.Intn(100) < 10
	if crit {
		dmg = dmg * 3 / 2
	}
	h.mu.Lock()
	h.monsterHP -= dmg
	if h.monsterHP < 0 {
		h.monsterHP = 0
	}
	h.mu.Unlock()
	if crit {
		return actorName + " crit! -" + strconv.Itoa(dmg)
	}
	return actorName + " hits for " + strconv.Itoa(dmg) + "!"
}

// doPlayerSkill — class-flavoured special attack. Mirrors the single-player
// signature skills at a high level (no full skill tree yet).
func (m *roundMachine) doPlayerSkill(actorID, actorName string) string {
	h := m.host
	if h.monsterHP <= 0 {
		return actorName + " holds back."
	}
	cls := m.actorClass(actorID)
	atk := m.actorATK(actorID)
	def := h.monsterDEF

	var mult float64
	label := "Skill!"
	switch cls {
	case 0: // Knight — Shield Bash (stun ignored in MP for simplicity)
		mult = 1.2
		label = "Shield Bash!"
	case 1: // Mage — Fireball
		mult = 2.0
		label = "Fireball!"
	case 2: // Archer — Snipe (guaranteed double crit)
		mult = 1.0
		label = "Snipe!"
	default:
		mult = 1.3
	}
	base := float64(atk)*mult - float64(def)/2.0
	if base < 2 {
		base = 2
	}
	dmg := int(base * (0.9 + rand.Float64()*0.2))
	if cls == 2 { // Snipe
		dmg *= 2
	}
	if dmg < 1 {
		dmg = 1
	}
	h.mu.Lock()
	h.monsterHP -= dmg
	if h.monsterHP < 0 {
		h.monsterHP = 0
	}
	h.mu.Unlock()
	return actorName + " " + label + " -" + strconv.Itoa(dmg)
}

func (m *roundMachine) doPlayerFlee(actorID, actorName string) string {
	h := m.host
	if h.monsterIsBoss {
		return actorName + " can't flee!"
	}
	// Flee chance based on SPD ratio.
	pspd := m.actorSPD(actorID)
	chance := 50 + (pspd-h.monsterSPD)*5
	if chance < 20 {
		chance = 20
	}
	if chance > 90 {
		chance = 90
	}
	if rand.Intn(100) >= chance {
		return actorName + " can't escape!"
	}
	// Only this player leaves.
	h.mu.Lock()
	if actorID == "host" {
		h.hostFled = true
	} else if p, ok := h.peers[actorID]; ok {
		p.fled = true
	}
	h.mu.Unlock()
	return actorName + " escaped!"
}

// ---------------- Monster turn ----------------

// rollMonsterTurn picks the monster's action and target.
//
// Probabilities: 80% single-target, 20% AOE.
// On top of that, an extra 10% roll upgrades the attack to a critical.
func (m *roundMachine) rollMonsterTurn() (kind, target string) {
	h := m.host
	aoe := rand.Intn(100) < 20
	crit := rand.Intn(100) < 10
	if aoe {
		if crit {
			return "monster_crit_aoe", ""
		}
		return "monster_aoe", ""
	}
	// Single target — pick a random living, non-fled participant.
	h.mu.Lock()
	var pool []string
	if h.hostSelf.HP > 0 && !h.hostFled {
		pool = append(pool, "host")
	}
	for _, p := range h.peers {
		if p.hp > 0 && !p.fled {
			pool = append(pool, p.id)
		}
	}
	h.mu.Unlock()
	if len(pool) == 0 {
		// No valid targets — treat as AOE that hits nothing.
		return "monster_aoe", ""
	}
	target = pool[rand.Intn(len(pool))]
	if crit {
		return "monster_crit_single", target
	}
	return "monster_single", target
}

func (m *roundMachine) doMonsterSingle(targetID string, crit bool) string {
	h := m.host
	if targetID == "" {
		return h.monsterName + " swipes at air."
	}
	baseATK := h.monsterATK
	if crit {
		baseATK = baseATK * 3 / 2
	}
	dmg, tgtName, tgtAlive := m.applyDamageTo(targetID, baseATK)
	if !tgtAlive {
		return tgtName + " is down!"
	}
	if crit {
		return h.monsterName + " critical!\n" + tgtName + " -" + strconv.Itoa(dmg)
	}
	return h.monsterName + " hits " + tgtName + " -" + strconv.Itoa(dmg)
}

func (m *roundMachine) doMonsterAOE(crit bool) string {
	h := m.host
	// AOE hits all living, non-fled for 60% damage each (80% on crit).
	mult := 0.6
	if crit {
		mult = 0.8
	}
	total := 0
	h.mu.Lock()
	targets := []string{}
	if h.hostSelf.HP > 0 && !h.hostFled {
		targets = append(targets, "host")
	}
	for _, p := range h.peers {
		if p.hp > 0 && !p.fled {
			targets = append(targets, p.id)
		}
	}
	h.mu.Unlock()

	baseATK := int(float64(h.monsterATK) * mult)
	for _, t := range targets {
		dmg, _, _ := m.applyDamageTo(t, baseATK)
		total += dmg
	}
	if crit {
		return h.monsterName + " critical blast!\nAOE -" + strconv.Itoa(total)
	}
	return h.monsterName + " AOE blast!\n-" + strconv.Itoa(total) + " total"
}

// applyDamageTo subtracts HP from a target and returns the damage dealt,
// the target's name, and whether they're still alive after the hit.
// Honours the target's defend flag (halves damage).
func (m *roundMachine) applyDamageTo(targetID string, baseATK int) (int, string, bool) {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()

	var def int
	var name string
	defendMult := 1.0
	if targetID == "host" {
		def = h.hostSelf.DEF
		name = h.hostSelf.Name
		if h.hostAction == CombatDefend {
			defendMult = 0.5
		}
	} else if p, ok := h.peers[targetID]; ok {
		def = p.def
		name = p.name
		if p.action == CombatDefend {
			defendMult = 0.5
		}
	} else {
		return 0, targetID, false
	}

	base := float64(baseATK) - float64(def)/2.0
	if base < 1 {
		base = 1
	}
	dmg := int(base * (0.8 + rand.Float64()*0.4) * defendMult)
	if dmg < 1 {
		dmg = 1
	}

	if targetID == "host" {
		h.hostSelf.HP -= dmg
		if h.hostSelf.HP < 0 {
			h.hostSelf.HP = 0
		}
		return dmg, name, h.hostSelf.HP > 0
	}
	p := h.peers[targetID]
	p.hp -= dmg
	if p.hp < 0 {
		p.hp = 0
	}
	return dmg, name, p.hp > 0
}

// ---------------- End-of-round checks ----------------

// checkEnd decides whether combat should end this tick.
// Returns true if the round machine closed out the fight.
func (m *roundMachine) checkEnd() bool {
	h := m.host

	h.mu.Lock()
	monDead := h.monsterHP <= 0
	anyAlive := false
	if h.hostSelf.HP > 0 && !h.hostFled {
		anyAlive = true
	}
	for _, p := range h.peers {
		if p.hp > 0 && !p.fled {
			anyAlive = true
			break
		}
	}
	// Everyone has left the fight via flee or death?
	anyRemaining := false
	if !h.hostFled && h.hostSelf.HP > 0 {
		anyRemaining = true
	}
	for _, p := range h.peers {
		if !p.fled && p.hp > 0 {
			anyRemaining = true
			break
		}
	}
	h.mu.Unlock()

	switch {
	case monDead:
		m.closeRound("Victory!")
		// XP / coins are set by the caller who provided MonsterInit. We
		// stash the rewards on the session so the host's combat screen
		// can splash them and the client screens can apply them.
		// (XP/coins propagation: the caller of StartTeamCombat passed
		// them in MonsterInit — we expose them via session.SetCombatEnd
		// in a follow-up; for this round machine we emit victory with
		// placeholder rewards that the host's screen can override if
		// needed.)
		h.endCombat(true, m.rewardXP(), m.rewardCoins())
		m.done = true
		return true
	case !anyAlive && !anyRemaining:
		// Everyone is dead or gone. If no one is left alive (all dead):
		// defeat. If at least one fled, we still treat as "not victory"
		// — caller can differentiate by the EndVictory flag.
		h.endCombat(false, 0, 0)
		m.done = true
		return true
	case !anyRemaining:
		// All living participants have fled — close quietly (no rewards).
		h.endCombat(false, 0, 0)
		m.done = true
		return true
	}
	return false
}

// closeRound tags the final log line.
func (m *roundMachine) closeRound(tag string) {
	m.pushCombatSnapshot(tag)
}

// rewardXP / rewardCoins — placeholders; StartTeamCombat stores them in
// MonsterInit fields that aren't surfaced here yet. The host's combat
// screen currently passes them via SetHostCombatHP; for the MVP we rely
// on the monster's name-based lookup at the wild.go call site.
func (m *roundMachine) rewardXP() int    { return m.host.rewardXP }
func (m *roundMachine) rewardCoins() int { return m.host.rewardCoins }

// ---------------- Helpers ----------------

// withPhase builds a CombatSharedState with current authoritative numbers
// plus the supplied phase/timer/round.
func (m *roundMachine) withPhase(phase string, secs int) CombatSharedState {
	h := m.host
	players := h.buildCombatPlayers()
	h.mu.Lock()
	monHP := h.monsterHP
	monMax := h.monsterMax
	monName := h.monsterName
	monSprite := h.monsterSpriteID
	monATK := h.monsterATK
	monDEF := h.monsterDEF
	monSPD := h.monsterSPD
	monBoss := h.monsterIsBoss
	monID := h.monsterID
	h.mu.Unlock()
	return CombatSharedState{
		Active:          true,
		MonsterID:       monID,
		MonsterName:     monName,
		MonsterSpriteID: monSprite,
		MonsterHP:       monHP,
		MonsterMax:      monMax,
		MonsterATK:      monATK,
		MonsterDEF:      monDEF,
		MonsterSPD:      monSPD,
		MonsterIsBoss:   monBoss,
		Players:         players,
		Phase:           phase,
		SecondsLeft:     secs,
		RoundNum:        m.roundNum,
		LastLog:         m.host.session.CombatState().LastLog,
	}
}

// pushCombatSnapshot stamps a log line onto the shared state without
// changing HP/MP (resolve helpers already mutated those). If `log`
// is empty the last log line is preserved.
func (m *roundMachine) pushCombatSnapshot(log string) {
	cs := m.withPhase(m.phase, m.secondsLeftForPhase())
	if log != "" {
		cs.LastLog = log
	}
	m.host.session.setCombat(cs)
}

func (m *roundMachine) secondsLeftForPhase() int {
	if m.phase != RoundPhaseCollect {
		return 0
	}
	if m.collectEnd <= m.tickCount {
		return 0
	}
	remaining := int(m.collectEnd - m.tickCount)
	return (remaining + hostTicksPerSecond - 1) / hostTicksPerSecond
}

func (m *roundMachine) actorATK(id string) int {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if id == "host" {
		return h.hostSelf.ATK
	}
	if p, ok := h.peers[id]; ok {
		return p.atk
	}
	return 1
}

func (m *roundMachine) actorSPD(id string) int {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if id == "host" {
		return h.hostSelf.SPD
	}
	if p, ok := h.peers[id]; ok {
		return p.spd
	}
	return 1
}

func (m *roundMachine) actorClass(id string) int {
	h := m.host
	h.mu.Lock()
	defer h.mu.Unlock()
	if id == "host" {
		return h.hostSelf.Class
	}
	if p, ok := h.peers[id]; ok {
		return p.class
	}
	return 0
}

// resolveKind normalises a CombatActionKind into the round-event kind string.
func resolveKind(a CombatActionKind) string {
	switch a {
	case CombatAttack:
		return "attack"
	case CombatSkill:
		return "skill"
	case CombatDefend:
		return "defend"
	case CombatFlee:
		return "flee"
	case CombatPotion:
		return "defend" // treat potion as defend for MP MVP
	}
	return "defend"
}

// sortSlots does a stable insertion sort by SPD descending. Length is
// small (<=MaxPeers+1) so any simple sort is fine.
func sortSlots(ss []actorSlot) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j-1].spd < ss[j].spd; j-- {
			ss[j-1], ss[j] = ss[j], ss[j-1]
		}
	}
}
