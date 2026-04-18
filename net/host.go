// net/host.go — TCP listener accepting peers into the wormhole room.
//
// THEORY — Goroutine-per-peer model:
// Each accepted TCP connection gets two goroutines: a reader
// (scanner.Scan → envelope → route by type) and a writer (pull from a
// per-peer send channel → serialise → write). This separation keeps the
// reader from ever blocking on a slow writer and vice-versa. The main
// "state tick" goroutine iterates at 10 Hz, builds the authoritative
// snapshot, and fan-outs to every peer's send channel.
package net

import (
	"bufio"
	"encoding/json"
	"fmt"
	stdnet "net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// MaxPeers caps room size. 4 is the classic "party of four" JRPG vibe.
const MaxPeers = 4

// Host is the server side of a multiplayer session.
type Host struct {
	listener stdnet.Listener
	beacon   *Beacon
	session  *Session
	tickHz   int

	mu        sync.Mutex
	peers     map[string]*hostPeer // keyed by PeerID
	nextID    uint64

	// monster / combat authoritative state
	monsterID   string
	monsterName string
	monsterHP   int
	monsterMax  int
	combatOpen  bool

	// action queue: PeerID → their committed turn action
	pendingActions map[string]CombatActionMsg

	stopCh   chan struct{}
	stopOnce sync.Once

	// Last area announced (used when a new peer arrives mid-run).
	hostArea  string
	hostTileX int
	hostTileY int
}

type hostPeer struct {
	id     string
	name   string
	class  int
	conn   stdnet.Conn
	sendCh chan []byte
	alive  atomic.Bool

	tileX  int
	tileY  int
	facing int
	hp     int
	maxHP  int
	area   string

	// per-peer combat state (HP/MP mirrored from the local player struct
	// in a fuller implementation; we keep enough here to show in HUD)
	mp     int
	maxMP  int
	ready  bool
}

// StartHost binds a TCP listener on an ephemeral port and begins the LAN
// beacon. Returns a Session the game can plug into its screens.
func StartHost(roomName, hostName string, hostClass int) (*Session, error) {
	// Port 0 → OS-assigned ephemeral port.
	ln, err := stdnet.Listen("tcp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("net: host listen: %w", err)
	}
	_, portStr, _ := stdnet.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	beacon, err := StartBeacon(roomName, hostName, port, MaxPeers)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	sess := newSession(RoleHost, "host", hostName)
	// Seed the session with the host's own presence so the town screen can
	// show the host on the map straight away.
	sess.setRemotePeer(RemotePlayer{
		PeerID: "host", Name: hostName, Class: hostClass,
		TileX: 9, TileY: 10, HP: 30, MaxHP: 30, Area: "town",
	})

	h := &Host{
		listener:       ln,
		beacon:         beacon,
		session:        sess,
		tickHz:         10,
		peers:          map[string]*hostPeer{},
		pendingActions: map[string]CombatActionMsg{},
		stopCh:         make(chan struct{}),
		hostArea:       "town",
		hostTileX:      9,
		hostTileY:      10,
	}
	sess.host = h

	go h.acceptLoop()
	go h.broadcastLoop()

	return sess, nil
}

// Close tears down the listener, beacon, and all peer connections.
func (h *Host) Close() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
		_ = h.listener.Close()
		if h.beacon != nil {
			h.beacon.Close()
		}
		h.mu.Lock()
		for _, p := range h.peers {
			_ = p.conn.Close()
		}
		h.peers = map[string]*hostPeer{}
		h.mu.Unlock()
		h.session.Close()
	})
}

// acceptLoop pulls new connections and handshakes them.
func (h *Host) acceptLoop() {
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			select {
			case <-h.stopCh:
				return
			default:
			}
			return
		}
		go h.handleNewPeer(conn)
	}
}

// handleNewPeer performs the Hello/Welcome handshake then starts reader/writer.
func (h *Host) handleNewPeer(conn stdnet.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 8*1024), 256*1024)

	if !scanner.Scan() {
		_ = conn.Close()
		return
	}
	var env Envelope
	if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
		_ = conn.Close()
		return
	}
	var hello HelloMsg
	if err := DecodePayload(&env, MsgHello, &hello); err != nil {
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	h.mu.Lock()
	if len(h.peers) >= MaxPeers-1 { // -1 because host counts too
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	h.nextID++
	peerID := fmt.Sprintf("p%d", h.nextID)
	p := &hostPeer{
		id:     peerID,
		name:   hello.Name,
		class:  hello.Class,
		conn:   conn,
		sendCh: make(chan []byte, 64),
		tileX:  h.hostTileX,
		tileY:  h.hostTileY,
		hp:     30, maxHP: 30,
		area: h.hostArea,
	}
	p.alive.Store(true)
	h.peers[peerID] = p

	// Build welcome with the peer list as of right now.
	peers := h.snapshotPeers()
	h.mu.Unlock()

	// Publish the joiner to the session so host's screens see them.
	h.session.setRemotePeer(RemotePlayer{
		PeerID: peerID, Name: p.name, Class: p.class,
		TileX: p.tileX, TileY: p.tileY, HP: p.hp, MaxHP: p.maxHP, Area: p.area,
	})
	h.session.pushEvent(SessionEvent{Kind: "peer_join", PeerID: peerID, Text: p.name})
	if h.beacon != nil {
		h.beacon.UpdatePlayers(len(h.peers) + 1)
	}

	// Start writer first so we can queue the welcome.
	go h.peerWriter(p)

	welcome := WelcomeMsg{
		YourPeerID: peerID,
		HostName:   h.session.MyName(),
		Area:       h.hostArea,
		Peers:      peers,
	}
	h.enqueueTo(p, MsgWelcome, welcome)

	// Tell the rest of the room.
	h.broadcastExcept(peerID, MsgPeerJoin, PeerInfo{
		PeerID: peerID, Name: p.name, Class: p.class,
		TileX: p.tileX, TileY: p.tileY, HP: p.hp, MaxHP: p.maxHP, Area: p.area,
	})

	// Reader loop.
	h.peerReader(p, scanner)
}

// peerReader consumes incoming messages from one peer until disconnect.
func (h *Host) peerReader(p *hostPeer, scanner *bufio.Scanner) {
	defer h.dropPeer(p, "disconnect")
	for scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		switch env.Type {
		case MsgInput:
			var in InputMsg
			if err := DecodePayload(&env, MsgInput, &in); err == nil {
				h.applyPeerInput(p, in)
			}
		case MsgCombatAction:
			var ca CombatActionMsg
			if err := DecodePayload(&env, MsgCombatAction, &ca); err == nil {
				h.applyCombatAction(p.id, ca)
			}
		case MsgChat:
			var c ChatMsg
			if err := DecodePayload(&env, MsgChat, &c); err == nil {
				c.From = p.name
				h.broadcast(MsgChat, c)
				h.session.pushEvent(SessionEvent{Kind: "chat", PeerID: p.id, Text: c.Text})
			}
		case MsgPing:
			// Passive heartbeat — nothing to do besides "we saw activity".
		}
	}
}

// peerWriter drains the per-peer send channel.
func (h *Host) peerWriter(p *hostPeer) {
	defer p.conn.Close()
	for {
		select {
		case <-h.stopCh:
			return
		case b, ok := <-p.sendCh:
			if !ok {
				return
			}
			if _, err := p.conn.Write(b); err != nil {
				p.alive.Store(false)
				return
			}
		}
	}
}

// dropPeer handles disconnects.
func (h *Host) dropPeer(p *hostPeer, reason string) {
	h.mu.Lock()
	if _, ok := h.peers[p.id]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.peers, p.id)
	p.alive.Store(false)
	close(p.sendCh)
	h.mu.Unlock()

	h.session.removeRemotePeer(p.id)
	h.session.pushEvent(SessionEvent{Kind: "peer_leave", PeerID: p.id, Text: p.name})
	if h.beacon != nil {
		h.beacon.UpdatePlayers(len(h.peers) + 1)
	}
	h.broadcast(MsgPeerLeave, PeerLeaveMsg{PeerID: p.id, Reason: reason})
}

// applyPeerInput translates a client input into a position change.
//
// THEORY — Step-wise movement:
// We interpret dx/dy as "move one tile in this direction". The host isn't
// running the tilemap collision code of the peer's view; we just accept the
// update optimistically for MVP LAN. A fuller implementation would keep the
// tilemap here and reject illegal moves.
func (h *Host) applyPeerInput(p *hostPeer, in InputMsg) {
	h.mu.Lock()
	if in.DX != 0 {
		p.tileX += sign(in.DX)
		if in.DX < 0 {
			p.facing = 1 // left
		} else {
			p.facing = 2 // right
		}
	}
	if in.DY != 0 {
		p.tileY += sign(in.DY)
		if in.DY < 0 {
			p.facing = 3 // up
		} else {
			p.facing = 0 // down
		}
	}
	h.mu.Unlock()

	h.session.setRemotePeer(RemotePlayer{
		PeerID: p.id, Name: p.name, Class: p.class,
		TileX: p.tileX, TileY: p.tileY, Facing: p.facing,
		HP: p.hp, MaxHP: p.maxHP, Area: p.area,
	})
}

// applyCombatAction records a peer's committed turn action.
// Combat resolution itself lives in combat/engine.go and is driven by the
// host's combat screen tick — here we just stash the intent.
func (h *Host) applyCombatAction(peerID string, a CombatActionMsg) {
	h.mu.Lock()
	h.pendingActions[peerID] = a
	if p, ok := h.peers[peerID]; ok {
		p.ready = true
	}
	h.mu.Unlock()
}

// ConsumePendingActions returns & clears all queued combat actions. Called
// by the host's combat screen when resolving a turn.
func (h *Host) ConsumePendingActions() map[string]CombatActionMsg {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.pendingActions
	h.pendingActions = map[string]CombatActionMsg{}
	for _, p := range h.peers {
		p.ready = false
	}
	return out
}

// startCombat announces a new fight to everyone.
func (h *Host) startCombat(monID, monName string, hp, maxHP int) {
	h.mu.Lock()
	h.monsterID = monID
	h.monsterName = monName
	h.monsterHP = hp
	h.monsterMax = maxHP
	h.combatOpen = true
	h.pendingActions = map[string]CombatActionMsg{}
	h.mu.Unlock()

	h.session.setCombat(CombatSharedState{
		Active: true, MonsterID: monID, MonsterName: monName,
		MonsterHP: hp, MonsterMax: maxHP,
	})
	h.session.pushEvent(SessionEvent{Kind: "combat_start", Text: monName})
	h.broadcast(MsgCombatStart, CombatStartMsg{
		MonsterID: monID, MonsterHP: hp, MonsterMax: maxHP, MonsterName: monName,
	})
}

// endCombat closes the fight and broadcasts rewards.
func (h *Host) endCombat(victory bool, xp, coins int) {
	h.mu.Lock()
	h.combatOpen = false
	h.mu.Unlock()
	cs := h.session.CombatState()
	cs.Active = false
	cs.EndVictory = victory
	cs.EndXP = xp
	cs.EndCoins = coins
	cs.EndPending = true
	h.session.setCombat(cs)
	h.broadcast(MsgCombatEnd, CombatEndMsg{Victory: victory, XP: xp, Coins: coins})
	h.session.pushEvent(SessionEvent{Kind: "combat_end"})
}

// setMonsterHP mid-combat (used by the host combat screen).
func (h *Host) setMonsterHP(hp int) {
	h.mu.Lock()
	h.monsterHP = hp
	h.mu.Unlock()
	cs := h.session.CombatState()
	cs.MonsterHP = hp
	h.session.setCombat(cs)
}

// announceAreaChange sends everyone to a new area.
func (h *Host) announceAreaChange(area string, tx, ty int) {
	h.mu.Lock()
	h.hostArea = area
	h.hostTileX = tx
	h.hostTileY = ty
	for _, p := range h.peers {
		p.area = area
		p.tileX = tx
		p.tileY = ty
	}
	h.mu.Unlock()
	h.session.setArea(area)
	h.broadcast(MsgAreaChange, AreaChangeMsg{Area: area, TileX: tx, TileY: ty})
}

// broadcastLoop emits an authoritative state snapshot every tick.
func (h *Host) broadcastLoop() {
	interval := time.Second / time.Duration(h.tickHz)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var tick uint64
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			tick++
			// Pick up host's own position from session (the game screen updates it).
			if self, ok := h.session.getPeer("host"); ok {
				h.mu.Lock()
				h.hostArea = self.Area
				h.hostTileX = self.TileX
				h.hostTileY = self.TileY
				h.mu.Unlock()
			}
			h.mu.Lock()
			peers := h.snapshotPeers()
			h.mu.Unlock()

			msg := StateMsg{Tick: tick, Area: h.hostArea, Peers: peers}
			h.broadcast(MsgState, msg)

			if h.combatOpen {
				h.broadcast(MsgCombatState, h.combatSnapshot(tick))
			}
		}
	}
}

// snapshotPeers builds the PeerInfo list under the host lock.
// NOTE: caller holds h.mu. We acquire session.mu separately below via
// getPeer, which is safe because the two mutexes never nest in the
// opposite order.
func (h *Host) snapshotPeers() []PeerInfo {
	// Include the host as a peer so clients see them.
	out := make([]PeerInfo, 0, len(h.peers)+1)
	if self, ok := h.session.getPeer("host"); ok {
		out = append(out, PeerInfo{
			PeerID: "host", Name: self.Name, Class: self.Class,
			TileX: self.TileX, TileY: self.TileY, Facing: self.Facing,
			HP: self.HP, MaxHP: self.MaxHP, Area: self.Area,
		})
	}
	for _, p := range h.peers {
		out = append(out, PeerInfo{
			PeerID: p.id, Name: p.name, Class: p.class,
			TileX: p.tileX, TileY: p.tileY, Facing: p.facing,
			HP: p.hp, MaxHP: p.maxHP, Area: p.area,
		})
	}
	return out
}

func (h *Host) combatSnapshot(tick uint64) CombatStateMsg {
	cs := h.session.CombatState()
	snaps := make([]CombatSnapshot, 0, len(cs.Players))
	for _, pl := range cs.Players {
		snaps = append(snaps, CombatSnapshot{
			PeerID: pl.PeerID, HP: pl.HP, MaxHP: pl.MaxHP,
			MP: pl.MP, MaxMP: pl.MaxMP, Ready: pl.Ready,
		})
	}
	return CombatStateMsg{
		Tick:        tick,
		MonsterHP:   cs.MonsterHP,
		MonsterMax:  cs.MonsterMax,
		MonsterName: cs.MonsterName,
		Players:     snaps,
		LogLine:     cs.LastLog,
	}
}

// broadcast sends to every connected peer.
func (h *Host) broadcast(kind MsgType, payload any) {
	raw, err := wireBytes(kind, payload)
	if err != nil {
		return
	}
	h.mu.Lock()
	for _, p := range h.peers {
		if !p.alive.Load() {
			continue
		}
		select {
		case p.sendCh <- raw:
		default:
			// Writer is backed up — drop this snapshot; next one is fine.
		}
	}
	h.mu.Unlock()
}

// broadcastExcept sends to every peer except one.
func (h *Host) broadcastExcept(skipID string, kind MsgType, payload any) {
	raw, err := wireBytes(kind, payload)
	if err != nil {
		return
	}
	h.mu.Lock()
	for id, p := range h.peers {
		if id == skipID || !p.alive.Load() {
			continue
		}
		select {
		case p.sendCh <- raw:
		default:
		}
	}
	h.mu.Unlock()
}

// enqueueTo sends to a specific peer.
func (h *Host) enqueueTo(p *hostPeer, kind MsgType, payload any) {
	raw, err := wireBytes(kind, payload)
	if err != nil {
		return
	}
	select {
	case p.sendCh <- raw:
	default:
	}
}

// wireBytes marshals an envelope to bytes terminated by newline.
func wireBytes(kind MsgType, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env := Envelope{Type: kind, Raw: raw}
	line, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return append(line, '\n'), nil
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}
