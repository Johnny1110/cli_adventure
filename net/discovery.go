// net/discovery.go — UDP broadcast-based LAN room discovery.
//
// THEORY — Why UDP broadcast:
// On a local network we don't have a central server to ask "what rooms are
// available?". The classic solution is: every host periodically blasts a
// small UDP packet to the broadcast address (255.255.255.255). Every machine
// on the same subnet receives it. Clients who are "looking for games" open
// a UDP socket on the same port and collect packets they hear, building a
// live list of rooms. This is how Minecraft LAN, old LAN FPS games, and
// service-discovery tools (mDNS/Bonjour in spirit) all work.
//
// Packet format: a tiny JSON blob with a magic string. We include the magic
// so we don't accidentally treat unrelated UDP traffic on this port as a room.
//
//   {"magic":"CLIADVENTURE/1","room":"Kai's Room","host":"Kai","tcp":6789,
//    "players":2,"max":4}
//
// Beacons fire at 1 Hz. Rooms that haven't been heard from in 5 s are
// evicted — that's how we detect a host who quit without telling us.
package net

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// DiscoveryPort is the UDP port used for room announcements.
// Chosen to be well above the well-known range and unlikely to collide.
const DiscoveryPort = 47281

// discoveryMagic tags our packets so unrelated UDP traffic is ignored.
const discoveryMagic = "CLIADVENTURE/1"

// beaconPacket is the on-the-wire format for a room announcement.
type beaconPacket struct {
	Magic    string `json:"magic"`
	Room     string `json:"room"`
	Host     string `json:"host"`
	TCPPort  int    `json:"tcp"`
	Players  int    `json:"players"`
	MaxPeers int    `json:"max"`
}

// Room is a discovered room, as seen by a would-be joiner.
type Room struct {
	Name     string    // host-supplied room name
	HostName string    // display name of the host
	Addr     string    // "ip:port" to dial over TCP
	Players  int       // currently connected peers (including host)
	MaxPeers int       // capacity
	LastSeen time.Time // for expiry
}

// ---------- Beacon (host side) ----------

// Beacon periodically broadcasts a room-announcement packet on the LAN.
// Stop when the host is torn down.
type Beacon struct {
	conn     *net.UDPConn
	packet   beaconPacket
	mu       sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

// StartBeacon begins broadcasting a room announcement every second.
// roomName / hostName are display strings. tcpPort is the port the host's
// TCP listener accepted on. Returns a running Beacon whose Update() /
// Close() methods the caller drives.
func StartBeacon(roomName, hostName string, tcpPort, maxPeers int) (*Beacon, error) {
	// A UDP socket without a fixed local port — OS assigns an ephemeral one.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("net: beacon listen: %w", err)
	}
	b := &Beacon{
		conn: conn,
		packet: beaconPacket{
			Magic:    discoveryMagic,
			Room:     roomName,
			Host:     hostName,
			TCPPort:  tcpPort,
			Players:  1,
			MaxPeers: maxPeers,
		},
		stopCh: make(chan struct{}),
	}
	go b.loop()
	return b, nil
}

// UpdatePlayers refreshes the player count that future beacons will announce.
func (b *Beacon) UpdatePlayers(n int) {
	b.mu.Lock()
	b.packet.Players = n
	b.mu.Unlock()
}

// Close stops the beacon goroutine.
func (b *Beacon) Close() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
		_ = b.conn.Close()
	})
}

func (b *Beacon) loop() {
	// Broadcast destination — every host on the /24 hears it.
	dst := &net.UDPAddr{IP: net.IPv4bcast, Port: DiscoveryPort}

	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	// Send once immediately so clients see the room quickly.
	b.send(dst)

	for {
		select {
		case <-b.stopCh:
			return
		case <-tick.C:
			b.send(dst)
		}
	}
}

func (b *Beacon) send(dst *net.UDPAddr) {
	b.mu.Lock()
	payload, _ := json.Marshal(b.packet)
	b.mu.Unlock()
	// Best-effort — if the write fails (interface down, etc.) we'll retry
	// next tick.
	_, _ = b.conn.WriteToUDP(payload, dst)
}

// ---------- Scanner (client side) ----------

// Scanner listens for broadcast packets and maintains a live room list.
// Call Rooms() to get the current snapshot.
type Scanner struct {
	conn     *net.UDPConn
	mu       sync.Mutex
	rooms    map[string]*Room // keyed by Addr
	stopCh   chan struct{}
	stopOnce sync.Once
}

// StartScanner binds to the discovery UDP port and starts listening for
// room announcements. Returns an error if the port is in use (e.g. because
// another copy of the game is already hosting on this machine — in which
// case the host can still see rooms through the localhost path below).
func StartScanner() (*Scanner, error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: DiscoveryPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("net: scanner listen: %w", err)
	}
	s := &Scanner{
		conn:   conn,
		rooms:  map[string]*Room{},
		stopCh: make(chan struct{}),
	}
	go s.loop()
	go s.expireLoop()
	return s, nil
}

// Rooms returns a copy of the currently-visible rooms.
// Stable order: by room name.
func (s *Scanner) Rooms() []Room {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		out = append(out, *r)
	}
	// Simple O(n^2) sort — list is tiny.
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Close shuts the scanner down.
func (s *Scanner) Close() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		_ = s.conn.Close()
	})
}

func (s *Scanner) loop() {
	buf := make([]byte, 2048)
	for {
		n, src, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			// Socket closed → exit.
			select {
			case <-s.stopCh:
				return
			default:
			}
			return
		}
		var pkt beaconPacket
		if err := json.Unmarshal(buf[:n], &pkt); err != nil {
			continue
		}
		if pkt.Magic != discoveryMagic {
			continue
		}
		// Sender IP + advertised TCP port = dial address.
		addr := fmt.Sprintf("%s:%d", src.IP.String(), pkt.TCPPort)
		s.mu.Lock()
		s.rooms[addr] = &Room{
			Name:     pkt.Room,
			HostName: pkt.Host,
			Addr:     addr,
			Players:  pkt.Players,
			MaxPeers: pkt.MaxPeers,
			LastSeen: time.Now(),
		}
		s.mu.Unlock()
	}
}

// expireLoop drops rooms we haven't heard from in 5 s.
func (s *Scanner) expireLoop() {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-tick.C:
			cutoff := time.Now().Add(-5 * time.Second)
			s.mu.Lock()
			for k, r := range s.rooms {
				if r.LastSeen.Before(cutoff) {
					delete(s.rooms, k)
				}
			}
			s.mu.Unlock()
		}
	}
}
