// screen/wormhole.go — The multiplayer "wormhole" menu.
//
// THEORY — Three sub-states in one screen:
// The wormhole is a small UX funnel:
//   1. Main menu — Host | Find Room | Back
//   2. Room list — scanning the LAN, showing live rooms, press Z to join
//   3. Lobby    — connected (as host or guest), waiting to start, press
//                  Z to enter the world together
// All three share the same sprite background (the portal tile blown up
// as decoration) and a dialog-style panel. Keeping them in one screen
// file lets us navigate back and forth without going through the
// ScreenSwitcher each time — a tiny internal state machine instead.
package screen

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"cli_adventure/entity"
	netpkg "cli_adventure/net"
	"cli_adventure/render"
)

type wormholeState int

const (
	wormholeMenu wormholeState = iota
	wormholeRoomList
	wormholeLobby
	wormholeError
)

// WormholeScreen is the multiplayer entrypoint menu.
type WormholeScreen struct {
	switcher ScreenSwitcher
	player   *entity.Player
	returnTo Screen // the town screen to resume after we're done

	state wormholeState

	// Main menu
	menuCursor int // 0=Host, 1=Find, 2=Back

	// Room list
	scanner    *netpkg.Scanner
	rooms      []netpkg.Room
	listCursor int
	scanStart  time.Time
	scanErr    string

	// Lobby / live session
	session *netpkg.Session
	role    netpkg.Role

	errMsg string

	// Cosmetic
	tick int
}

// NewWormholeScreen creates the menu.
// returnTo is the town screen to re-enter when the player cancels, so we
// preserve position/state instead of rebuilding the whole town.
func NewWormholeScreen(switcher ScreenSwitcher, player *entity.Player, returnTo Screen) *WormholeScreen {
	return &WormholeScreen{
		switcher: switcher,
		player:   player,
		returnTo: returnTo,
		state:    wormholeMenu,
	}
}

func (w *WormholeScreen) OnEnter() {}

func (w *WormholeScreen) OnExit() {
	// If we never actually joined/hosted a session, clean up the scanner.
	if w.scanner != nil && w.session == nil {
		w.scanner.Close()
	}
}

func (w *WormholeScreen) Update() error {
	w.tick++

	switch w.state {
	case wormholeMenu:
		w.updateMenu()
	case wormholeRoomList:
		w.updateRoomList()
	case wormholeLobby:
		w.updateLobby()
	case wormholeError:
		// Any key returns to menu.
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) ||
			inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
			inpututil.IsKeyJustPressed(ebiten.KeyX) ||
			inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			w.errMsg = ""
			w.state = wormholeMenu
		}
	}
	return nil
}

// --- Main menu ---

func (w *WormholeScreen) updateMenu() {
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		w.menuCursor--
		if w.menuCursor < 0 {
			w.menuCursor = 2
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		w.menuCursor = (w.menuCursor + 1) % 3
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		w.backToTown()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		switch w.menuCursor {
		case 0:
			w.startHost()
		case 1:
			w.startRoomList()
		case 2:
			w.backToTown()
		}
	}
}

func (w *WormholeScreen) backToTown() {
	if w.returnTo != nil {
		w.switcher.SwitchScreen(w.returnTo)
		return
	}
	w.switcher.SwitchScreen(NewTownScreen(w.switcher, w.player))
}

// --- Start host ---

func (w *WormholeScreen) startHost() {
	name := entity.ClassTable[w.player.Class].Name
	room := name + "'s Wormhole"
	sess, err := netpkg.StartHostWithStats(room, playerCombatStats(w.player))
	if err != nil {
		w.errMsg = "Host failed: " + err.Error()
		w.state = wormholeError
		return
	}
	w.session = sess
	w.role = netpkg.RoleHost
	w.state = wormholeLobby
}

// playerCombatStats distils the Player struct into the stats bundle that
// the net layer needs for MP combat. Kept in this file so the net package
// never has to import entity.
func playerCombatStats(p *entity.Player) netpkg.CombatPlayerStats {
	name := entity.ClassTable[p.Class].Name
	return netpkg.CombatPlayerStats{
		Name:  name,
		Class: int(p.Class),
		HP:    p.Stats.HP, MaxHP: p.Stats.MaxHP,
		MP:  p.Stats.MP, MaxMP: p.Stats.MaxMP,
		ATK: p.EffectiveATK(),
		DEF: p.EffectiveDEF(),
		SPD: p.Stats.SPD,
		Level: p.Level,
	}
}

// --- Room list (join flow) ---

func (w *WormholeScreen) startRoomList() {
	sc, err := netpkg.StartScanner()
	if err != nil {
		w.errMsg = "Scan failed: " + err.Error()
		w.state = wormholeError
		return
	}
	w.scanner = sc
	w.scanStart = time.Now()
	w.state = wormholeRoomList
	w.listCursor = 0
}

func (w *WormholeScreen) updateRoomList() {
	// Refresh rooms list each tick — Scanner keeps its own state, we just
	// pull a snapshot.
	w.rooms = w.scanner.Rooms()
	if w.listCursor >= len(w.rooms) {
		w.listCursor = 0
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if len(w.rooms) > 0 {
			w.listCursor--
			if w.listCursor < 0 {
				w.listCursor = len(w.rooms) - 1
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if len(w.rooms) > 0 {
			w.listCursor = (w.listCursor + 1) % len(w.rooms)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if w.scanner != nil {
			w.scanner.Close()
			w.scanner = nil
		}
		w.state = wormholeMenu
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if len(w.rooms) == 0 {
			return
		}
		room := w.rooms[w.listCursor]
		_ = entity.ClassTable[w.player.Class].Name
		sess, err := netpkg.DialWithStats(room.Addr, playerCombatStats(w.player))
		if err != nil {
			w.errMsg = "Join failed: " + err.Error()
			w.state = wormholeError
			return
		}
		// We're connected. Stop the scanner — we don't want to keep
		// occupying the discovery port.
		if w.scanner != nil {
			w.scanner.Close()
			w.scanner = nil
		}
		w.session = sess
		w.role = netpkg.RoleClient
		w.state = wormholeLobby
	}
}

// --- Lobby ---

func (w *WormholeScreen) updateLobby() {
	// Session ended unexpectedly?
	select {
	case <-w.session.Done:
		w.errMsg = "Session ended"
		w.state = wormholeError
		return
	default:
	}

	// Press Z to start the game. Either role can trigger — when the host
	// presses Z the session is already live, we just navigate into town.
	// When a client presses Z we also navigate into town; they'll see the
	// host's position via the State snapshots.
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		w.switcher.SwitchScreen(NewTownScreenMP(w.switcher, w.player, w.session))
		return
	}

	// X aborts: close session and go back to the menu.
	if inpututil.IsKeyJustPressed(ebiten.KeyX) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if w.session != nil {
			w.session.Close()
			w.session = nil
		}
		w.state = wormholeMenu
	}
}

// --- Draw ---

func (w *WormholeScreen) Draw(screen *ebiten.Image) {
	// Swirling background — concentric fading rings that rotate.
	w.drawBackdrop(screen)

	// Title
	title := "WORMHOLE"
	render.DrawText(screen, title, (160-render.TextWidth(title))/2, 8, render.ColorLavender)

	switch w.state {
	case wormholeMenu:
		w.drawMenu(screen)
	case wormholeRoomList:
		w.drawRoomList(screen)
	case wormholeLobby:
		w.drawLobby(screen)
	case wormholeError:
		w.drawError(screen)
	}
}

func (w *WormholeScreen) drawBackdrop(screen *ebiten.Image) {
	// A couple of slow-rotating rings to sell the "portal" vibe without
	// costing much to draw. Keep the area small (centered) so UI text
	// stays readable.
	t := float64(w.tick) / 30.0
	for i := 0; i < 8; i++ {
		radius := 10 + i*6
		phase := t + float64(i)
		for a := 0; a < 24; a++ {
			ang := float64(a)*0.2618 + phase
			cx := 80 + int(float64(radius)*cosApprox(ang))
			cy := 72 + int(float64(radius)*sinApprox(ang))
			if cx >= 0 && cx < 160 && cy >= 0 && cy < 144 {
				r := uint8(80 + i*15)
				g := uint8(40 + i*10)
				b := uint8(120 + i*10)
				screen.Set(cx, cy, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
}

func (w *WormholeScreen) drawMenu(screen *ebiten.Image) {
	items := []string{"Host Room", "Find Room", "Back"}
	y0 := 44
	render.DrawBox(screen, 24, 36, 112, 60, render.ColorBoxBG, render.ColorLavender)
	for i, it := range items {
		clr := render.ColorWhite
		if i == w.menuCursor {
			render.DrawCursor(screen, 32, y0+i*14, render.ColorGold)
			clr = render.ColorGold
		}
		render.DrawText(screen, it, 44, y0+i*14, clr)
	}
	render.DrawText(screen, "Z:OK  X:Back", 44, 128, render.ColorGray)
}

func (w *WormholeScreen) drawRoomList(screen *ebiten.Image) {
	render.DrawBox(screen, 6, 22, 148, 110, render.ColorBoxBG, render.ColorSky)
	render.DrawText(screen, "Finding rooms...", 12, 26, render.ColorSky)

	if len(w.rooms) == 0 {
		elapsed := int(time.Since(w.scanStart).Seconds())
		msg := "Scanning"
		for i := 0; i < elapsed%4; i++ {
			msg += "."
		}
		render.DrawText(screen, msg, 12, 44, render.ColorGray)
		render.DrawText(screen, "No rooms yet on LAN.", 12, 60, render.ColorGray)
		render.DrawText(screen, "Ask a friend to Host!", 12, 72, render.ColorGray)
	} else {
		for i, r := range w.rooms {
			y := 40 + i*12
			clr := render.ColorWhite
			if i == w.listCursor {
				render.DrawCursor(screen, 10, y, render.ColorGold)
				clr = render.ColorGold
			}
			line := r.Name
			if len(line) > 18 {
				line = line[:18]
			}
			render.DrawText(screen, line, 20, y, clr)
			count := intToStr(r.Players) + "/" + intToStr(r.MaxPeers)
			render.DrawText(screen, count, 128, y, render.ColorPeach)
		}
	}
	render.DrawText(screen, "Z:Join  X:Back", 38, 136-14, render.ColorGray)
}

func (w *WormholeScreen) drawLobby(screen *ebiten.Image) {
	render.DrawBox(screen, 8, 24, 144, 108, render.ColorBoxBG, render.ColorMint)

	if w.role == netpkg.RoleHost {
		render.DrawText(screen, "Hosting room:", 14, 28, render.ColorMint)
		render.DrawText(screen, w.session.MyName()+"'s", 14, 40, render.ColorWhite)
		render.DrawText(screen, "Waiting for friends...", 14, 56, render.ColorGray)
	} else {
		render.DrawText(screen, "Joined room!", 14, 28, render.ColorMint)
	}

	// Peer list
	peers := w.session.RemotePlayers("")
	render.DrawText(screen, "Players:", 14, 74, render.ColorSky)
	y := 86
	// Include self first
	render.DrawText(screen, "> "+w.session.MyName()+" (you)", 16, y, render.ColorGold)
	y += 10
	for _, p := range peers {
		render.DrawText(screen, "  "+p.Name, 16, y, render.ColorWhite)
		y += 10
		if y > 118 {
			break
		}
	}

	render.DrawText(screen, "Z:Start  X:Cancel", 32, 124, render.ColorGray)
}

func (w *WormholeScreen) drawError(screen *ebiten.Image) {
	render.DrawBox(screen, 8, 48, 144, 56, render.ColorBoxBG, render.ColorRed)
	render.DrawText(screen, "Problem", 62, 54, render.ColorRed)
	// Word wrap the error to ~24 chars/line.
	lines := wrapText(w.errMsg, 24)
	for i, ln := range lines {
		if i > 3 {
			break
		}
		render.DrawText(screen, ln, 14, 66+i*10, render.ColorWhite)
	}
	render.DrawText(screen, "Z:OK", 72, 94, render.ColorGray)
}

// ---- small helpers ----

// cosApprox / sinApprox — we don't need math precision for a backdrop,
// and staying out of the math import keeps the file light.
func cosApprox(x float64) float64 {
	// Taylor-lite: wrap x into [-π,π], then approximate with 1 - x²/2 + x⁴/24.
	const pi = 3.14159265
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	x2 := x * x
	return 1 - x2/2 + x2*x2/24
}

func sinApprox(x float64) float64 {
	const pi = 3.14159265
	return cosApprox(x - pi/2)
}

// wrapText splits s into lines no wider than width characters, on spaces.
func wrapText(s string, width int) []string {
	var out []string
	start := 0
	for start < len(s) {
		end := start + width
		if end >= len(s) {
			out = append(out, s[start:])
			break
		}
		// Back up to the last space within the window.
		br := end
		for br > start && s[br] != ' ' {
			br--
		}
		if br == start {
			br = end // no space — hard-split
		}
		out = append(out, s[start:br])
		start = br
		if start < len(s) && s[start] == ' ' {
			start++
		}
	}
	return out
}
