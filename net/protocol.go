// Package net implements the peer-to-peer / host-authoritative networking
// layer used by the wormhole multiplayer feature.
//
// THEORY — Why JSON lines over TCP:
// For a Game-Boy-scale 2D RPG with LAN-only co-op, we are not pushing millions
// of bytes per frame — a player's position, facing, HP and an occasional
// combat action. TCP guarantees in-order delivery, which simplifies the state
// model enormously (no packet reordering, no retransmission logic to write).
// JSON is more verbose than a binary protocol, but it's trivially debuggable
// (`nc` to the host and you can read the stream) and the payloads are tiny.
// Each message is a single JSON object terminated by '\n' — a classic
// line-delimited framing that plays nicely with bufio.Scanner.
//
// THEORY — Host-authoritative model:
// One peer creates the room and becomes the HOST. Every other peer is a
// CLIENT. Clients send their INPUT intent ("I pressed right") to the host;
// the host updates its own world and broadcasts the authoritative STATE
// (player positions, combat turn, HPs) back to every client. Clients render
// whatever the host says. This means:
//   - No desync: everyone sees the host's world.
//   - No cheating: the client cannot claim "I did 9999 damage" — only host math counts.
//   - Simpler code: the combat engine only runs on the host.
// The tradeoff is latency — a client input takes ~1 RTT to be visible. On a
// LAN that's a few milliseconds, imperceptible.
package net

import (
	"encoding/json"
	"errors"
	"io"
)

// MsgType identifies a message kind. Using a small string set (rather than an
// int enum) makes captured traffic self-describing.
type MsgType string

const (
	// MsgHello is the very first client→host message after TCP connect.
	// It announces the joining player's name and class. The host replies
	// with MsgWelcome carrying an assigned PeerID.
	MsgHello MsgType = "hello"

	// MsgWelcome is host→new-client, assigning them a PeerID and telling
	// them the current room state (host's name, current area, peer list).
	MsgWelcome MsgType = "welcome"

	// MsgPeerJoin is host→everyone when a new peer joins (so existing
	// clients can add the new player's sprite).
	MsgPeerJoin MsgType = "peer_join"

	// MsgPeerLeave is host→everyone when a peer disconnects.
	MsgPeerLeave MsgType = "peer_leave"

	// MsgInput is client→host — the player's current movement intent and
	// any action button press this tick. Host applies it to the peer's
	// avatar and rebroadcasts authoritative state.
	MsgInput MsgType = "input"

	// MsgState is host→everyone — the authoritative snapshot of all
	// remote-visible players (position, facing, HP) plus the current area.
	// Sent ~10x/second, not every tick (60 TPS would flood the wire).
	MsgState MsgType = "state"

	// MsgAreaChange is host→everyone when the host walks between areas
	// (town → forest → cave). All clients snap into the same area.
	MsgAreaChange MsgType = "area_change"

	// MsgCombatStart is host→everyone — "we are now in combat with this
	// monster". Every peer switches to the combat screen.
	MsgCombatStart MsgType = "combat_start"

	// MsgCombatAction is client→host — "I chose Attack / Skill-N / Flee
	// for my turn". Host applies the action at turn-resolution time.
	MsgCombatAction MsgType = "combat_action"

	// MsgCombatState is host→everyone — authoritative combat snapshot:
	// whose turn, monster HP, log line for this turn. Clients just render it.
	MsgCombatState MsgType = "combat_state"

	// MsgCombatEnd is host→everyone — the fight is over (victory/defeat).
	// Carries XP/coin rewards so every peer can apply them to their player.
	MsgCombatEnd MsgType = "combat_end"

	// MsgChat is peer→everyone (relayed via host). Purely cosmetic, but
	// having chat is what makes a multiplayer session feel alive.
	MsgChat MsgType = "chat"

	// MsgPing is either direction — heartbeat to detect dead peers. If we
	// don't see a ping for ~5 seconds we consider the peer gone.
	MsgPing MsgType = "ping"
)

// Envelope wraps every wire message. The host/client reads the Type first,
// then decodes the matching payload struct from Raw.
type Envelope struct {
	Type MsgType         `json:"t"`
	Raw  json.RawMessage `json:"d"`
}

// ---------- Payload types ----------

// HelloMsg — client's initial handshake.
type HelloMsg struct {
	Name    string `json:"name"`
	Class   int    `json:"class"` // entity.Class value (Knight=0, Mage=1, Archer=2)
	Version int    `json:"version"`
}

// WelcomeMsg — host's reply to Hello.
type WelcomeMsg struct {
	YourPeerID string     `json:"your_id"`
	HostName   string     `json:"host_name"`
	Area       string     `json:"area"` // "town" or a wild-area id
	Peers      []PeerInfo `json:"peers"`
}

// PeerInfo — summary of a connected player (used in Welcome + PeerJoin).
type PeerInfo struct {
	PeerID string `json:"id"`
	Name   string `json:"name"`
	Class  int    `json:"class"`
	TileX  int    `json:"tx"`
	TileY  int    `json:"ty"`
	Facing int    `json:"f"`
	HP     int    `json:"hp"`
	MaxHP  int    `json:"max_hp"`
	Area   string `json:"area"`
}

// PeerLeaveMsg — a peer disconnected.
type PeerLeaveMsg struct {
	PeerID string `json:"id"`
	Reason string `json:"reason"`
}

// InputMsg — client's per-tick input. We only send inputs when something
// changes (direction press, action press) to keep traffic low.
type InputMsg struct {
	// DX / DY are the movement intent: -1, 0, +1 along each axis.
	DX int `json:"dx"`
	DY int `json:"dy"`
	// Interact = true if Z was just pressed (NPC / wormhole interaction).
	Interact bool `json:"z"`
}

// StateMsg — host's authoritative snapshot of the overworld.
type StateMsg struct {
	Tick  uint64     `json:"tick"`
	Area  string     `json:"area"`
	Peers []PeerInfo `json:"peers"`
}

// AreaChangeMsg — host moved between maps; everyone follows.
type AreaChangeMsg struct {
	Area  string `json:"area"`
	TileX int    `json:"tx"`
	TileY int    `json:"ty"`
}

// CombatStartMsg — all peers enter combat against this monster.
type CombatStartMsg struct {
	MonsterID   string `json:"mon_id"`
	MonsterHP   int    `json:"mon_hp"`
	MonsterMax  int    `json:"mon_max"`
	MonsterName string `json:"mon_name"`
}

// CombatActionKind identifies a battle command.
type CombatActionKind string

const (
	CombatAttack  CombatActionKind = "attack"
	CombatSkill   CombatActionKind = "skill"
	CombatDefend  CombatActionKind = "defend"
	CombatFlee    CombatActionKind = "flee"
	CombatPotion  CombatActionKind = "potion"
)

// CombatActionMsg — client chose an action for their turn.
type CombatActionMsg struct {
	Kind    CombatActionKind `json:"k"`
	SkillID int              `json:"sid,omitempty"`
}

// CombatSnapshot — per-peer combat status for host broadcasts.
type CombatSnapshot struct {
	PeerID string `json:"id"`
	HP     int    `json:"hp"`
	MaxHP  int    `json:"max"`
	MP     int    `json:"mp"`
	MaxMP  int    `json:"mpmax"`
	Ready  bool   `json:"ready"` // submitted their action?
}

// CombatStateMsg — authoritative combat snapshot broadcast every ~10 Hz.
type CombatStateMsg struct {
	Tick        uint64           `json:"tick"`
	MonsterHP   int              `json:"mon_hp"`
	MonsterMax  int              `json:"mon_max"`
	MonsterName string           `json:"mon_name"`
	Players     []CombatSnapshot `json:"players"`
	LogLine     string           `json:"log,omitempty"` // latest text
}

// CombatEndMsg — fight is over.
type CombatEndMsg struct {
	Victory bool `json:"win"`
	XP      int  `json:"xp"`
	Coins   int  `json:"coin"`
}

// ChatMsg — a line of player chat.
type ChatMsg struct {
	From string `json:"from"`
	Text string `json:"text"`
}

// ---------- Framing helpers ----------

// WriteMsg encodes payload into an Envelope and writes it as a '\n'-terminated
// JSON line. Safe for concurrent use only if the caller serialises writes
// (each peer has a single writer goroutine in our design).
func WriteMsg(w io.Writer, kind MsgType, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := Envelope{Type: kind, Raw: raw}
	line, err := json.Marshal(env)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	_, err = w.Write(line)
	return err
}

// DecodePayload decodes env.Raw into out. Returns an error if the type
// doesn't match the expected MsgType, so callers can assert safely.
func DecodePayload(env *Envelope, expected MsgType, out any) error {
	if env.Type != expected {
		return errors.New("net: envelope type mismatch")
	}
	return json.Unmarshal(env.Raw, out)
}
