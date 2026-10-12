package ui

// Coming back: a room run reconnects with backoff, the startup check finds a
// round still going, and an any-time result can be refreshed.
//
// The backoff is the plan's whole of this PR: an interrupted round is the one
// online state the player cannot fix from anywhere else, so the game walks it
// back on its own and only gives up when the room itself is gone.

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// roomRetryMsg is one scheduled reconnect. The gen it carries is the
// connection it was scheduled for: a later generation's retry is stale.
type roomRetryMsg struct{ gen int }

// roomBackoff is the wait before reconnect attempt n (1-based): 1, 2, 4, 8,
// 16 seconds, then 30 seconds for every later attempt.
func roomBackoff(n int) time.Duration {
	switch n {
	case 1:
		return 1 * time.Second
	case 2:
		return 2 * time.Second
	case 3:
		return 4 * time.Second
	case 4:
		return 8 * time.Second
	case 5:
		return 16 * time.Second
	default:
		// From attempt 6 on, the longest step: a room that has been dark half
		// a minute is one that has either been left or lost.
		return 30 * time.Second
	}
}

// roomLost handles a closed or failed connection of the current session. It
// is the error branch of both the dial and the listen loop, and it is the one
// place the reconnection rules live: gone is gone, leaving stays gone, and
// anything else walks back in its own second.
func (m *Model) roomLost(err error) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	// 1. The server turned us away: nothing online runs again this run, and
	// the room stops with us (markGone closes it).
	if m.markGone(err) {
		return nil
	}
	// 2. The connection is over either way.
	r.conn = nil
	// 3. Still unwelcomed: the dial never produced a room to come back to,
	// and a room the server never answered is not one to retry with.
	if !r.welcomed {
		m.roomSay("couldn't reach the server — try again", true)
		m.closeRoom()
		return nil
	}
	// 4. The player's own leave wins: never reconnect.
	if r.leaving {
		m.closeRoom()
		return nil
	}
	// 5. Anything else walks back on its own, with the wait doubling.
	gen := r.gen
	r.retries++
	at := roomBackoff(r.retries)
	return tea.Tick(at, func(time.Time) tea.Msg { return roomRetryMsg{gen} })
}

// roomRetryNext discards the wait and finds a connection again. Old waits are
// dropped: they belong to a connection that never produced a session.
func (m *Model) roomRetryNext(msg roomRetryMsg) tea.Cmd {
	if m.room == nil || msg.gen != m.room.gen || m.room.conn != nil {
		// A retry for a session that has moved on, or after one whose socket
		// is already open: neither does anything.
		return nil
	}
	m.roomGen++
	m.room.gen = m.roomGen
	return dialRoomCmd(m.online, m.room.code,
		online.Hello(m.clientName, online.RoomName(m.settingsOf().DisplayName), m.room.key),
		m.room.gen)
}

// ---- the startup check ----

// roomProbeMsg is the startup check's answer: the round still running (with
// its connection to keep), or nothing but what the search learned.
type roomProbeMsg struct {
	entry   store.RoomEntry
	conn    online.Conn      // the connection to keep, or nil
	welcome online.ServerMsg // its welcome
	dropped []string         // codes the server no longer knows
	gone    string           // a gone message, if the server said so
}

// probeCmd asks, once at startup with consent and rooms to ask about, whether
// one of your rounds is running. The command runs alone in the background and
// answers with a roomProbeMsg.
func (m *Model) probeCmd() tea.Cmd {
	if !m.roomsOnline() {
		return nil
	}
	entries := m.activeRooms()
	if len(entries) == 0 {
		return nil
	}
	// The hello's name is cleaned first, here, so the command closes over
	// text rather than reaching for the model while the probe runs.
	name := online.RoomName(m.settingsOf().DisplayName)
	clientName := m.clientName
	return probeRoomsCmd(m.online, entries, func(key string) online.ClientMsg {
		return online.Hello(clientName, name, key)
	})
}

// probeRoomsCmd dials each room in turn: its questions are exactly "is a
// round of mine running", "is this room gone", and nothing else. The first
// running round answers for the run; every other room's finding is carried
// back to be dropped or announced in one place.
func probeRoomsCmd(client online.Client, entries []store.RoomEntry, hello func(key string) online.ClientMsg) tea.Cmd {
	return func() tea.Msg {
		msg := roomProbeMsg{}
		for _, entry := range entries {
			ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
			conn, err := client.Dial(ctx, entry.Code)
			if err != nil {
				cancel()
				continue // an error keeps the entry
			}
			if err := conn.Send(ctx, hello(entry.Key)); err != nil {
				cancel()
				_ = conn.Close()
				continue
			}
			readCtx, readCancel := context.WithTimeout(context.Background(), online.Timeout)
			served, err := conn.Receive(readCtx)
			cancel()
			readCancel()
			switch {
			case err != nil:
				_ = conn.Close()
				continue
			case served.T == "welcome" && roundRunning(served.Room):
				msg.entry, msg.conn, msg.welcome = entry, conn, served
				return msg
			case served.T == "welcome":
				// Not running: the room is not one to rejoin, but not gone.
				_ = conn.Close()
			case served.T == "refuse":
				switch served.Reason {
				case "no_room", "bad_key", "left":
					msg.dropped = append(msg.dropped, entry.Code)
					_ = conn.Close()
				default:
					_ = conn.Close()
				}
			case served.T == "gone":
				msg.gone = served.Message
				_ = conn.Close()
				return msg
			default:
				_ = conn.Close()
			}
		}
		return msg
	}
}

// roundRunning reports whether a welcome's room is one whose round a player
// would want to be put back in: the clock is running, the seat is in the
// round, and the seat has not yet finished it.
func roundRunning(room *online.RoomView) bool {
	if room == nil {
		return false
	}
	if room.Mode != "async" && room.Phase != "countdown" && room.Phase != "playing" {
		return false
	}
	return room.You.InRound && !room.You.Done
}

// settleProbe is what the startup check's answer does. One session can be
// made for it; the screens it moves are the probe's own rule, so an unplayed
// first board can give way but a puzzle in hand never does.
func (m *Model) settleProbe(msg roomProbeMsg) tea.Cmd {
	for _, code := range msg.dropped {
		m.dropRoom(code)
	}
	if len(msg.dropped) > 0 && msg.conn == nil {
		m.note = "a room you were in ended while you were away"
	}
	if msg.gone != "" {
		m.markGone(&online.GoneError{Message: msg.gone})
		return nil
	}
	if msg.conn == nil {
		return nil
	}
	if m.room != nil {
		// The player opened a room while the probe ran; its disc: the probe's
		// one, whose answer came late.
		_ = msg.conn.Close()
		return nil
	}

	entry := msg.entry
	m.roomGen++
	gen := m.roomGen
	m.room = &roomSession{
		code: entry.Code, key: entry.Key, mode: entry.Mode,
		gen: gen, conn: msg.conn, welcomed: true, probe: true,
	}
	// The welcome carries the truth: the room as the server now has it.
	if msg.welcome.Room != nil {
		m.room.view = *msg.welcome.Room
		m.room.seat = msg.welcome.Seat
		m.room.key = msg.welcome.Key
		m.room.mode = msg.welcome.Room.Mode
		m.room.welcomed = true
		if msg.welcome.Now != 0 {
			m.room.offset = unixMillis(msg.welcome.Now).Sub(timeNow())
		}
	}
	m.upsertRoom(store.RoomEntry{
		Code: m.room.view.Code, Key: m.room.key, Mode: m.room.mode,
		JoinedAt: timeNow().UTC(),
	})

	switch {
	case m.screen == screenSplash:
		m.showRoom()
		// The splash goes back on top: its dismissal still covers this
		// room's screen, the way it covered the board it was raised over.
		next := m.screen
		m.screen = screenSplash
		m.splash.next = next
	case m.screen == screenGame && m.game != nil && m.game.room == nil &&
		!m.game.persisted && m.game.typing == "":
		m.showRoom()
	default:
		// A board in play, a list, anything else: the player has something
		// open; what the probe found comes when they ask for it.
		m.note = fmt.Sprintf("your round in room %s is still running — open it from social play", m.room.code)
	}
	return m.listen(msg.conn, gen)
}

// unixMillis reads a server millisecond as a time.
func unixMillis(ms int64) time.Time { return time.UnixMilli(ms) }
