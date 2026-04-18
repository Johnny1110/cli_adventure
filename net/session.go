// net/session.go — A Session is the game's handle to an active multiplayer
// room. Screens (town, wild, combat) read remote player info from it and
// submit local intents to it, without caring whether the local machine is
// the host or a client.
//
// THEORY — One abstraction, two implementations:
// A host's job (tick the world and broadcast state) is very different from
// a client's job (forward inputs and apply snapshots). But the game code —
// "who else is in my town? where are they standing? what's the monster's
// HP?" — is the same. We capture that shared read/write surface in the
// Session struct. Both the Host and the Client update the same Session;
// screens don't know the difference.
package net

import (
	"sync"
	"sync/atomic"
)

// Role identifies this peer's role in the current session.
type Role int

const (
	RoleNone Role = iota
	RoleHost
	RoleClient
)

// RemotePlayer is a remote peer as rendered by the local game.
// Position is in tile coordinates; the renderer interpolates visually.
type RemotePlayer struct {
	PeerID string
	Name   string
	Class  int // entity.Class
	TileX  int
	TileY  int
	Facing int
	HP     int
	MaxHP  int
	Area   string
}

// CombatPlayer is the per-peer combat state (for shared HP display).
type CombatPlayer struct {
	PeerID string
	Name   string
	Class  int
	HP     int
	MaxHP  int
	MP     int
	MaxMP  int
	Ready  bool   // has submitted an action for the current round
	Action string // locked-in action label ("attack"|"defend"|"flee"|"skill")
	Fled   bool   // this player has left the fight via a successful flee
}

// CombatSharedState is the host-authoritative combat picture.
type CombatSharedState struct {
	Active          bool // true between CombatStart and CombatEnd
	MonsterID       string
	MonsterName     string
	MonsterSpriteID string
	MonsterHP       int
	MonsterMax      int
	MonsterATK      int
	MonsterDEF      int
	MonsterSPD      int
	MonsterIsBoss   bool
	Players         []CombatPlayer
	LastLog         string

	// Team-play round machine (set by host, mirrored by clients).
	Phase       string // "collect"|"resolve"|"ended"
	SecondsLeft int
	RoundNum    int

	EndVictory bool
	EndXP      int
	EndCoins   int
	EndPending bool // set when CombatEnd arrives; screens consume and clear
}

// Session is a thread-safe handle into the live multiplayer state.
type Session struct {
	role     Role
	myID     string // assigned by host (host's own ID is "host")
	myName   string
	area     atomic.Value // string
	mu       sync.RWMutex
	peers    map[string]*RemotePlayer // keyed by PeerID
	combat   CombatSharedState

	// Input buffer: set by the local screen each tick; consumed by the
	// outbound network writer (client side) or applied directly (host side).
	pendingInput   InputMsg
	hasPendingInput bool

	// Buffered incoming messages (chat, combat-end notices) consumed by screens.
	events []SessionEvent

	// Back-references let screens signal "host me an area change" or
	// "I pressed the attack button in combat".
	host   *Host
	client *Client

	// Channel closed when the session ends (peer hung up / host stopped).
	Done chan struct{}
	once sync.Once
}

// SessionEvent is a one-shot notification consumed by screens
// (e.g. "a peer said hi", "combat started").
type SessionEvent struct {
	Kind   string // "chat", "peer_join", "peer_leave", "combat_start", "combat_end"
	PeerID string
	Text   string
	Area   string
}

func newSession(role Role, myID, myName string) *Session {
	s := &Session{
		role:   role,
		myID:   myID,
		myName: myName,
		peers:  map[string]*RemotePlayer{},
		Done:   make(chan struct{}),
	}
	s.area.Store("town")
	return s
}

// Role returns the local role (Host or Client).
func (s *Session) Role() Role { return s.role }

// MyID returns the local peer ID.
func (s *Session) MyID() string { return s.myID }

// MyName returns the local display name.
func (s *Session) MyName() string { return s.myName }

// Area returns the current shared area name.
func (s *Session) Area() string {
	if v, ok := s.area.Load().(string); ok {
		return v
	}
	return ""
}

// RemotePlayers returns a snapshot of all peers other than the local player
// that are currently in the given area.
func (s *Session) RemotePlayers(area string) []RemotePlayer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RemotePlayer, 0, len(s.peers))
	for id, p := range s.peers {
		if id == s.myID {
			continue
		}
		if area != "" && p.Area != area {
			continue
		}
		out = append(out, *p)
	}
	return out
}

// SetMyPosition records the local player's tile position (used by the
// host to include itself in broadcasts, and by clients to send inputs).
func (s *Session) SetMyPosition(area string, tx, ty, facing, hp, maxHP int) {
	s.area.Store(area)
	s.mu.Lock()
	p, ok := s.peers[s.myID]
	if !ok {
		p = &RemotePlayer{PeerID: s.myID, Name: s.myName}
		s.peers[s.myID] = p
	}
	p.Area = area
	p.TileX = tx
	p.TileY = ty
	p.Facing = facing
	p.HP = hp
	p.MaxHP = maxHP
	s.mu.Unlock()
}

// SubmitInput buffers an input for the outbound writer.
// No-op on host (the host applies its own input directly via SetMyPosition).
func (s *Session) SubmitInput(in InputMsg) {
	s.mu.Lock()
	s.pendingInput = in
	s.hasPendingInput = true
	s.mu.Unlock()
}

// drainPendingInput returns the last buffered input, clearing it.
func (s *Session) drainPendingInput() (InputMsg, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasPendingInput {
		return InputMsg{}, false
	}
	in := s.pendingInput
	s.hasPendingInput = false
	return in, true
}

// CombatState returns a copy of the current combat snapshot.
func (s *Session) CombatState() CombatSharedState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Copy Players slice
	cp := s.combat
	cp.Players = append([]CombatPlayer(nil), s.combat.Players...)
	return cp
}

// ConsumeCombatEnd returns true exactly once after MsgCombatEnd arrives,
// giving screens a signal to transition out of combat.
func (s *Session) ConsumeCombatEnd() (CombatSharedState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.combat.EndPending {
		return CombatSharedState{}, false
	}
	cp := s.combat
	s.combat.EndPending = false
	s.combat.Active = false
	return cp, true
}

// PopEvents returns and clears the buffered SessionEvent list.
func (s *Session) PopEvents() []SessionEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev := s.events
	s.events = nil
	return ev
}

// pushEvent is an internal helper.
func (s *Session) pushEvent(e SessionEvent) {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
}

// Close marks the session done. Idempotent.
func (s *Session) Close() {
	s.once.Do(func() {
		close(s.Done)
		if s.host != nil {
			s.host.Close()
		}
		if s.client != nil {
			s.client.Close()
		}
	})
}

// getPeer returns a copy of a peer's info, or (RemotePlayer{}, false) if unknown.
// Safe for concurrent use.
func (s *Session) getPeer(id string) (RemotePlayer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[id]
	if !ok {
		return RemotePlayer{}, false
	}
	return *p, true
}

// --- Host-driven state mutation (used only when role == RoleHost) ---

// setRemotePeer upserts a peer's public state.
func (s *Session) setRemotePeer(p RemotePlayer) {
	s.mu.Lock()
	s.peers[p.PeerID] = &p
	s.mu.Unlock()
}

// removeRemotePeer drops a peer.
func (s *Session) removeRemotePeer(peerID string) {
	s.mu.Lock()
	delete(s.peers, peerID)
	s.mu.Unlock()
}

// setCombat replaces the combat snapshot.
func (s *Session) setCombat(c CombatSharedState) {
	s.mu.Lock()
	s.combat = c
	s.mu.Unlock()
}

// setArea updates the shared area string.
func (s *Session) setArea(area string) {
	s.area.Store(area)
}

// BroadcastAreaChange is called by the host when it walks into a new area.
// No-op on a client (the client follows the host's area broadcasts).
func (s *Session) BroadcastAreaChange(area string, tx, ty int) {
	if s.role != RoleHost || s.host == nil {
		return
	}
	s.host.announceAreaChange(area, tx, ty)
}

// SubmitCombatAction — client's per-turn battle command. Forwarded to host.
func (s *Session) SubmitCombatAction(a CombatActionMsg) {
	if s.role == RoleClient && s.client != nil {
		s.client.sendCombatAction(a)
		return
	}
	if s.role == RoleHost && s.host != nil {
		s.host.applyCombatAction(s.myID, a)
	}
}

// StartCombat is called by the host to open a co-op battle. No-op on client.
func (s *Session) StartCombat(monID, monName string, hp, maxHP int) {
	if s.role != RoleHost || s.host == nil {
		return
	}
	s.host.startCombat(monID, monName, hp, maxHP)
}

// MonsterInit bundles the combat-start parameters the host needs.
type MonsterInit struct {
	ID         string
	Name       string
	SpriteID   string
	HP         int
	MaxHP      int
	ATK        int
	DEF        int
	SPD        int
	IsBoss     bool
	XPReward   int
	CoinReward int
}

// StartTeamCombat opens a full team-play battle. Only meaningful on host.
// hostHP/MaxHP/etc describe the host player's starting stats and are used
// by the round resolver to compute damage.
func (s *Session) StartTeamCombat(mon MonsterInit, host CombatPlayerStats) {
	if s.role != RoleHost || s.host == nil {
		return
	}
	s.host.startTeamCombat(mon, host)
}

// CombatPlayerStats bundles the host player's combat stats (and is also
// used to cache each peer's stats).
type CombatPlayerStats struct {
	Name  string
	Class int
	HP    int
	MaxHP int
	MP    int
	MaxMP int
	ATK   int
	DEF   int
	SPD   int
	Level int
}

// EndCombat is called by the host to close a battle with rewards.
func (s *Session) EndCombat(victory bool, xp, coins int) {
	if s.role != RoleHost || s.host == nil {
		return
	}
	s.host.endCombat(victory, xp, coins)
}

// SetMonsterHP lets the host update monster HP between broadcasts.
func (s *Session) SetMonsterHP(hp int) {
	if s.role != RoleHost || s.host == nil {
		return
	}
	s.host.setMonsterHP(hp)
}
