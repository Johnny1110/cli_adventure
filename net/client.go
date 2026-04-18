// net/client.go — TCP client that joins an existing host's room.
//
// THEORY — Client is a thin skin over the host's snapshots:
// The client sends Hello, waits for Welcome (which gives it its PeerID and
// the current peer list), then enters its main loop: send an Input message
// whenever something meaningful changed locally, and apply inbound State /
// CombatState / AreaChange messages by updating the shared Session struct
// that screens read from. No game logic runs on the client — it just
// renders whatever the host tells it.
package net

import (
	"bufio"
	"encoding/json"
	"fmt"
	stdnet "net"
	"sync"
	"time"
)

// Client is the joiner side of a multiplayer session.
type Client struct {
	conn    stdnet.Conn
	session *Session

	sendCh   chan []byte
	stopCh   chan struct{}
	stopOnce sync.Once
}

// Dial connects to the given "ip:port" host advertised over the LAN beacon
// and performs the Hello/Welcome handshake. Returns a Session the game
// can plug in directly.
func Dial(addr, playerName string, class int) (*Session, error) {
	return DialWithStats(addr, CombatPlayerStats{
		Name:  playerName,
		Class: class,
		HP:    30, MaxHP: 30,
		ATK: 6, DEF: 4, SPD: 4, Level: 1,
	})
}

// DialWithStats is the full-fidelity variant — Hello carries the player's
// current stats so the host can resolve MP fights against real numbers.
func DialWithStats(addr string, stats CombatPlayerStats) (*Session, error) {
	// 3 s dial timeout — plenty for a LAN.
	conn, err := stdnet.DialTimeout("tcp4", addr, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("net: dial %s: %w", addr, err)
	}
	// Send Hello with the player's stats baked in.
	hello := HelloMsg{
		Name:    stats.Name,
		Class:   stats.Class,
		Version: 1,
		Level:   stats.Level,
		HP:      stats.HP,
		MaxHP:   stats.MaxHP,
		MP:      stats.MP,
		MaxMP:   stats.MaxMP,
		ATK:     stats.ATK,
		DEF:     stats.DEF,
		SPD:     stats.SPD,
	}
	if err := WriteMsg(conn, MsgHello, hello); err != nil {
		_ = conn.Close()
		return nil, err
	}
	// Wait for Welcome.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 8*1024), 256*1024)
	if !scanner.Scan() {
		_ = conn.Close()
		return nil, fmt.Errorf("net: host closed before Welcome")
	}
	var env Envelope
	if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
		_ = conn.Close()
		return nil, err
	}
	var welcome WelcomeMsg
	if err := DecodePayload(&env, MsgWelcome, &welcome); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Time{})

	sess := newSession(RoleClient, welcome.YourPeerID, stats.Name)
	sess.setArea(welcome.Area)
	for _, p := range welcome.Peers {
		sess.setRemotePeer(RemotePlayer{
			PeerID: p.PeerID, Name: p.Name, Class: p.Class,
			TileX: p.TileX, TileY: p.TileY, Facing: p.Facing,
			HP: p.HP, MaxHP: p.MaxHP, Area: p.Area,
		})
	}

	c := &Client{
		conn:    conn,
		session: sess,
		sendCh:  make(chan []byte, 64),
		stopCh:  make(chan struct{}),
	}
	sess.client = c

	go c.readerLoop(scanner)
	go c.writerLoop()
	go c.inputPump()

	return sess, nil
}

// Close shuts the client down.
func (c *Client) Close() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		_ = c.conn.Close()
		c.session.Close()
	})
}

// readerLoop consumes host messages and mutates the Session.
func (c *Client) readerLoop(scanner *bufio.Scanner) {
	defer c.Close()
	for scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		switch env.Type {
		case MsgState:
			var s StateMsg
			if err := DecodePayload(&env, MsgState, &s); err == nil {
				c.applyState(s)
			}
		case MsgPeerJoin:
			var p PeerInfo
			if err := DecodePayload(&env, MsgPeerJoin, &p); err == nil {
				c.session.setRemotePeer(RemotePlayer{
					PeerID: p.PeerID, Name: p.Name, Class: p.Class,
					TileX: p.TileX, TileY: p.TileY, Facing: p.Facing,
					HP: p.HP, MaxHP: p.MaxHP, Area: p.Area,
				})
				c.session.pushEvent(SessionEvent{Kind: "peer_join", PeerID: p.PeerID, Text: p.Name})
			}
		case MsgPeerLeave:
			var p PeerLeaveMsg
			if err := DecodePayload(&env, MsgPeerLeave, &p); err == nil {
				c.session.removeRemotePeer(p.PeerID)
				c.session.pushEvent(SessionEvent{Kind: "peer_leave", PeerID: p.PeerID})
			}
		case MsgAreaChange:
			var a AreaChangeMsg
			if err := DecodePayload(&env, MsgAreaChange, &a); err == nil {
				c.session.setArea(a.Area)
				c.session.pushEvent(SessionEvent{Kind: "area_change", Area: a.Area})
			}
		case MsgCombatStart:
			var cs CombatStartMsg
			if err := DecodePayload(&env, MsgCombatStart, &cs); err == nil {
				c.session.setCombat(CombatSharedState{
					Active:          true,
					MonsterID:       cs.MonsterID,
					MonsterName:     cs.MonsterName,
					MonsterSpriteID: cs.MonsterSpriteID,
					MonsterHP:       cs.MonsterHP,
					MonsterMax:      cs.MonsterMax,
					MonsterATK:      cs.MonsterATK,
					MonsterDEF:      cs.MonsterDEF,
					MonsterSPD:      cs.MonsterSPD,
					MonsterIsBoss:   cs.IsBoss,
					Phase:           RoundPhaseCollect,
					RoundNum:        1,
				})
				c.session.pushEvent(SessionEvent{Kind: "combat_start", Text: cs.MonsterName})
			}
		case MsgCombatState:
			var cs CombatStateMsg
			if err := DecodePayload(&env, MsgCombatState, &cs); err == nil {
				current := c.session.CombatState()
				current.Active = true
				current.MonsterHP = cs.MonsterHP
				current.MonsterMax = cs.MonsterMax
				current.MonsterName = cs.MonsterName
				if cs.LogLine != "" {
					current.LastLog = cs.LogLine
				}
				if cs.Phase != "" {
					current.Phase = cs.Phase
				}
				current.SecondsLeft = cs.SecondsLeft
				if cs.RoundNum > 0 {
					current.RoundNum = cs.RoundNum
				}
				players := make([]CombatPlayer, 0, len(cs.Players))
				for _, p := range cs.Players {
					players = append(players, CombatPlayer{
						PeerID: p.PeerID,
						Name:   p.Name,
						Class:  p.Class,
						HP:     p.HP, MaxHP: p.MaxHP,
						MP: p.MP, MaxMP: p.MaxMP,
						Ready:  p.Ready,
						Action: p.Action,
						Fled:   p.Fled,
					})
				}
				current.Players = players
				c.session.setCombat(current)
			}
		case MsgCombatEnd:
			var ce CombatEndMsg
			if err := DecodePayload(&env, MsgCombatEnd, &ce); err == nil {
				current := c.session.CombatState()
				current.EndVictory = ce.Victory
				current.EndXP = ce.XP
				current.EndCoins = ce.Coins
				current.EndPending = true
				c.session.setCombat(current)
				c.session.pushEvent(SessionEvent{Kind: "combat_end"})
			}
		case MsgChat:
			var ch ChatMsg
			if err := DecodePayload(&env, MsgChat, &ch); err == nil {
				c.session.pushEvent(SessionEvent{Kind: "chat", Text: fmt.Sprintf("%s: %s", ch.From, ch.Text)})
			}
		}
	}
}

// writerLoop pushes queued bytes to the socket.
func (c *Client) writerLoop() {
	defer c.Close()
	for {
		select {
		case <-c.stopCh:
			return
		case b, ok := <-c.sendCh:
			if !ok {
				return
			}
			if _, err := c.conn.Write(b); err != nil {
				return
			}
		}
	}
}

// inputPump sends buffered inputs to the host every 100 ms.
func (c *Client) inputPump() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			in, ok := c.session.drainPendingInput()
			if !ok {
				continue
			}
			c.enqueue(MsgInput, in)
		}
	}
}

// applyState overlays the host's snapshot onto the session's peer list.
//
// THEORY — Authoritative reconciliation:
// We could be clever and interpolate, but at 10 Hz LAN with tile-based
// movement the simpler path is: trust the host, snap positions to what it
// says. The town screen's own visual interpolation (pixel lerp between
// tiles) smooths the result.
func (c *Client) applyState(s StateMsg) {
	c.session.setArea(s.Area)
	// Build a set of peers the host now reports.
	seen := map[string]bool{}
	for _, p := range s.Peers {
		seen[p.PeerID] = true
		c.session.setRemotePeer(RemotePlayer{
			PeerID: p.PeerID, Name: p.Name, Class: p.Class,
			TileX: p.TileX, TileY: p.TileY, Facing: p.Facing,
			HP: p.HP, MaxHP: p.MaxHP, Area: p.Area,
		})
	}
	// Evict anyone not in the snapshot (we might have missed PeerLeave).
	c.session.mu.Lock()
	for id := range c.session.peers {
		if !seen[id] && id != c.session.myID {
			delete(c.session.peers, id)
		}
	}
	c.session.mu.Unlock()
}

// sendCombatAction queues a CombatAction envelope.
func (c *Client) sendCombatAction(a CombatActionMsg) {
	c.enqueue(MsgCombatAction, a)
}

// enqueue serialises and buffers a message.
func (c *Client) enqueue(kind MsgType, payload any) {
	raw, err := wireBytes(kind, payload)
	if err != nil {
		return
	}
	select {
	case c.sendCh <- raw:
	default:
		// Channel full — drop. Inputs are sent continuously so a miss
		// will be covered by the next tick.
	}
}
