package ui

// The room session and its connection half. One room at a time: opening
// another closes the current connection without leaving it, and only an
// explicit leave sends one.
//
// Everything here is the plumbing; the rules of a room's rounds live in the
// screens and in applyRoomMsg. Screens read m.room.view when they draw and
// keep no copy, so a message that lands lands everywhere at once.

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// roomSession is the room this run is connected to.
type roomSession struct {
	code string
	key  string // our seat key; "" until the first welcome of a join
	mode string // "live" or "async"; "" until the first welcome of a join
	seat int

	// gen is the connection generation this session's messages carry. A
	// message with another is from a connection that has been closed and is
	// dropped without a reply.
	gen      int
	conn     online.Conn     // nil while not connected
	view     online.RoomView // the room as last known
	offset   time.Duration   // server clock minus ours, from the last message with "now"
	welcomed bool            // a welcome has arrived on some connection of this session

	// pending is the guess number sent and not yet answered; 0 = none.
	pending     int
	pendingWord string

	// retries counts reconnect attempts since the last welcome (PR 10).
	retries int
	// leaving is the player's own choice to leave: never reconnect (PR 10).
	leaving bool
	// probe marks a session the startup check made: its first welcome moves
	// the screen by the probe's own rule, not the ordinary one.
	probe bool
}

// local turns a server time into ours.
func (r *roomSession) local(ms int64) time.Time { return time.UnixMilli(ms).Add(-r.offset) }

// me is our own PlayerView, if the room lists us.
func (r *roomSession) me() (online.PlayerView, bool) {
	for _, p := range r.view.Players {
		if p.Seat == r.seat {
			return p, true
		}
	}
	return online.PlayerView{}, false
}

type roomDialedMsg struct {
	gen  int
	conn online.Conn
	err  error
}

type roomRecvMsg struct {
	gen int
	msg online.ServerMsg
	err error
}

type roomCreatedMsg struct {
	opts    online.RoomOptions
	created online.Created
	err     error
}

// dialRoomCmd opens the socket and sends the hello. The hello carries the
// client header's text (there are no headers on a WebSocket), the cleaned
// name, and the seat key we already hold if any.
func dialRoomCmd(client online.Client, code string, hello online.ClientMsg, gen int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
		defer cancel()
		conn, err := client.Dial(ctx, code)
		if err == nil {
			if sendErr := conn.Send(ctx, hello); sendErr != nil {
				// The hello is what carries the version, so there is no second
				// try on this socket: close it and report the send failure.
				_ = conn.Close()
				conn, err = nil, sendErr
			}
		}
		return roomDialedMsg{gen: gen, conn: conn, err: err}
	}
}

// listenRoom waits for the room's next message. It is the command the root
// returns after every message, so a session listens until the connection is
// over. Tests stub it, because a real Receive blocks.
func listenRoom(conn online.Conn, gen int) tea.Cmd {
	return func() tea.Msg {
		msg, err := conn.Receive(context.Background())
		return roomRecvMsg{gen: gen, msg: msg, err: err}
	}
}

// sendRoomCmd writes one message; a 5s context bounds a write that cannot
// finish. A failed send shows up as a closed connection in the listen loop,
// so none of its errors are reported here.
func sendRoomCmd(conn online.Conn, msg online.ClientMsg) tea.Cmd {
	if conn == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		_ = conn.Send(ctx, msg)
		return nil
	}
}

// createRoomCmd asks the server for a room. The command copies opts.
func createRoomCmd(client online.Client, opts online.RoomOptions) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		created, err := client.CreateRoom(ctx, opts)
		return roomCreatedMsg{opts: opts, created: created, err: err}
	}
}

// leaveRoomCmd is the best-effort leave for a room this run is not connected
// to: dial, say hello (which carries the seat key), say leave, close, and
// never mind what any of it answered.
func leaveRoomCmd(client online.Client, code string, hello online.ClientMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
		defer cancel()
		conn, err := client.Dial(ctx, code)
		if err != nil {
			return nil
		}
		if err := conn.Send(ctx, hello); err != nil {
			_ = conn.Close()
			return nil
		}
		_ = conn.Send(ctx, online.Leave())
		_ = conn.Close()
		return nil
	}
}

// --- the active-rooms list ---

// roomStore is the rooms half of a store (store.JSON and store.KVStore both
// have it). A store without it has no rooms; the social screen then offers
// none.
type roomStore interface {
	Rooms() []store.RoomEntry
	SaveRooms([]store.RoomEntry) error
}

// activeRooms is the list the social screen shows, newest first.
func (m *Model) activeRooms() []store.RoomEntry {
	rs, ok := m.store.(roomStore)
	if !ok {
		return nil
	}
	entries := rs.Rooms()
	// Newest first, so the row the player is most likely after is at the top.
	sorted := make([]store.RoomEntry, len(entries))
	copy(sorted, entries)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].JoinedAt.After(sorted[j-1].JoinedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

// upsertRoom records a room, keeping the entry's earlier joined time on an
// update. A Failed save is told about once here, which is the one method the
// callers share.
func (m *Model) upsertRoom(e store.RoomEntry) {
	rs, ok := m.store.(roomStore)
	if !ok {
		return
	}
	rooms := rs.Rooms()
	for i, old := range rooms {
		if old.Code == e.Code {
			e.JoinedAt = old.JoinedAt
			rooms[i] = e
			if err := rs.SaveRooms(rooms); err != nil {
				m.note = saveFailed(err)
			}
			return
		}
	}
	rooms = append(rooms, e)
	if err := rs.SaveRooms(rooms); err != nil {
		m.note = saveFailed(err)
	}
}

// dropRoom forgets one room by code.
func (m *Model) dropRoom(code string) {
	rs, ok := m.store.(roomStore)
	if !ok {
		return
	}
	rooms := rs.Rooms()
	kept := rooms[:0]
	for _, e := range rooms {
		if e.Code != code {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(rooms) {
		return
	}
	if err := rs.SaveRooms(kept); err != nil {
		m.note = saveFailed(err)
	}
}

// roomsOnline reports whether rooms may be used at all: consent, a store with
// the rooms half, and a server that has not switched the feature off.
func (m *Model) roomsOnline() bool {
	if _, ok := m.store.(roomStore); !ok {
		return false
	}
	return m.consent() && (m.status == nil || m.status.Features.Rooms)
}

// --- the session ---

// openRoom makes the session for a room, after closing any other one. The
// hello carries the seat key we hold and the cleaned name to show.
func (m *Model) openRoom(code, key, mode string) tea.Cmd {
	if m.room != nil && m.room.code == code && m.room.conn != nil {
		return m.showRoom()
	}
	m.closeRoom()
	m.roomGen++
	m.room = &roomSession{code: code, key: key, mode: mode, gen: m.roomGen}
	return dialRoomCmd(m.online, code, m.roomHello(key), m.roomGen)
}

// openRoomCode opens one without knowing its mode — a join, or a rejoin whose
// mode was stored. The first welcome's room.Mode fills it in either way.
func (m *Model) openRoomCode(code, key string) tea.Cmd {
	return m.openRoom(code, key, "")
}

// roomHello is the hello this run sends for a room. key is the seat key to
// rejoin with, "" when this is a fresh seat.
func (m *Model) roomHello(key string) online.ClientMsg {
	name := online.RoomName(m.settingsOf().DisplayName)
	return online.Hello(m.clientName, name, key)
}

// closeRoom drops the session without leaving: the connection is closed, the
// room keeps running on the server. It is what opening another room, losing
// the network or quitting does; only the player's own leave sends one.
func (m *Model) closeRoom() {
	if m.room != nil && m.room.conn != nil {
		_ = m.room.conn.Close()
	}
	m.room = nil
	// A room result the leaving screen owed the player is gone with it.
	m.pendingRoomResult = false
}

// leaveRoom is the player choosing to go: one leave over the wire however
// the connection stands, then back to social play without the room.
func (m *Model) leaveRoom() tea.Cmd {
	r := m.room
	var cmd tea.Cmd
	switch {
	case r == nil:
	case r.conn != nil:
		r.leaving = true
		conn := r.conn
		m.room.conn = nil
		cmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
			defer cancel()
			_ = conn.Send(ctx, online.Leave())
			_ = conn.Close()
			return nil
		}
	default:
		cmd = leaveRoomCmd(m.online, r.code, m.roomHello(r.key))
	}
	if r != nil {
		m.dropRoom(r.code)
	}
	m.room = nil
	m.pendingRoomResult = false
	m.openSocialScreen()
	return cmd
}

// roomSay puts one line where the player is looking, so the same sentence
// about a join that did not happen lands on whichever screen is showing.
func (m *Model) roomSay(text string, isErr bool) {
	switch m.screen {
	case screenRoomJoin:
		m.roomJoin.note, m.roomJoin.noteErr = text, isErr
		m.roomJoin.busy = false
	case screenRoomMake:
		m.roomMake.note, m.roomMake.noteErr = text, isErr
		m.roomMake.busy = false
	case screenSocial:
		m.social.note, m.social.noteErr = text, isErr
	case screenRoom:
		m.roomScreen.note, m.roomScreen.noteErr = text, isErr
	case screenRoomResult:
		m.roomResult.note, m.roomResult.noteErr = text, isErr
	default:
		m.note = text
	}
}

// applyRoomMsg updates the session's view from one message, checking every
// field the message can vary rather than trusting the wire. Each type below
// is the plan's Appendix A meaning; a message whose fields fail is ignored.
//
// A message that carries `now` also resets the clock offset, which is how
// countdowns and round clocks read server times as ours.
func (m *Model) applyRoomMsg(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	// A message that carries `now` reads the server's clock: every later
	// deadline the room names is translated through one offset, set here,
	// where the freshest reading has just arrived.
	if msg.Now != 0 {
		r.offset = time.UnixMilli(msg.Now).Sub(timeNow())
	}
	switch msg.T {
	case "welcome":
		if msg.Room == nil || msg.Seat < 1 {
			return nil
		}
		first := !r.welcomed
		r.seat = msg.Seat
		r.key = msg.Key
		r.mode = msg.Room.Mode
		r.view = *msg.Room
		r.welcomed = true
		r.retries = 0
		// The room is one we are in again, not one to show as making.
		m.upsertRoom(store.RoomEntry{
			Code:     r.view.Code,
			Key:      r.key,
			Mode:     r.mode,
			JoinedAt: timeNow().UTC(),
		})
		m.roomJoin.busy = false
		m.roomMake.busy = false
		if first {
			return m.showRoom()
		}
		// A re-welcome is the truth overtaking what we remember, and four
		// things are owed it, in this order:
		//  - a guess that was out either landed while we were away (the view)
		//    or answers for another send;
		//  - the board of this room, when it is showing, is rebuilt from the
		//    view rather than argued with;
		//  - an open finished round saves itself, and the screen of this room
		//    moves to wherever its state now says;
		//  - everything else happens wherever the player is standing.
		if r.pending != 0 {
			if len(r.view.You.Words) >= r.pending {
				// Landed while we were away.
				r.pending, r.pendingWord = 0, ""
			} else {
				// Still owed: send it again.
				return m.sendRoom(online.Guess(r.pending, r.pendingWord))
			}
		}
		if m.screen == screenGame && m.game != nil && m.game.room == r {
			keep := m.game.typing
			m.openRoomBoard()
			if r.pending == 0 {
				m.game.typing = keep
			}
		}
		m.finishRoomRound()
		switch m.screen {
		case screenRoom, screenRoomResult:
			if cond := m.roomOnScreen(r); cond {
				return m.showRoom()
			}
		}
		return nil

	case "refuse":
		text := ""
		switch msg.Reason {
		case "no_room":
			text = "no room with that code"
		case "full":
			text = "that room is full"
		case "started":
			text = "that room is playing a round — try again in a minute"
		case "bad_key":
			text = "that room has closed"
		case "left":
			text = "you left that room"
		default:
			return nil
		}
		if msg.Reason == "no_room" || msg.Reason == "bad_key" || msg.Reason == "left" {
			m.dropRoom(r.code)
		}
		m.roomSay(text, true)
		m.closeRoom()
		return nil

	case "gone":
		m.markGone(&online.GoneError{Message: msg.Message})
		return nil

	case "error":
		// Not a state we track; the socket keeps going.
		return nil

	case "joined":
		if msg.Player == nil {
			return nil
		}
		m.upsertPlayer(*msg.Player)
		return nil

	case "left":
		m.removePlayer(msg.Seat)
		return nil

	case "presence":
		if msg.Connected != nil && *msg.Connected {
			return nil
		}
		for i, p := range r.view.Players {
			if p.Seat == msg.Seat {
				r.view.Players[i].Connected = false
			}
		}
		return nil

	case "ready":
		if msg.Ready == nil {
			return nil
		}
		for i, p := range r.view.Players {
			if p.Seat == msg.Seat {
				r.view.Players[i].Ready = *msg.Ready
			}
		}
		return nil

	case "host":
		if msg.Seat < 1 {
			return nil
		}
		r.view.Host = msg.Seat
		return nil

	case "countdown":
		if msg.GoAt <= 0 || msg.Round < 1 {
			return nil
		}
		r.view.Phase = "countdown"
		r.view.GoAt = msg.GoAt
		r.view.Round = msg.Round
		// The seats connected when the countdown started are the round's.
		r.resetPlayers(true)
		r.resetYou(true)
		r.view.Results = nil
		return nil

	case "go":
		if msg.EndsAt <= 0 || msg.Round < 1 {
			return nil
		}
		if r.view.Mode == "async" {
			// An any-time round is the one player's own clock: EndsAt is
			// theirs, and InRound says a row can now be typed.
			r.view.You.InRound = true
		}
		r.view.EndsAt = msg.EndsAt
		// A round under way is the board's business, not this screen's: whoever
		// is looking at the room goes to their board now, and a watcher stays
		// to watch.
		if m.screen == screenRoom {
			r.resetYou(true)
			if r.view.You.InRound {
				m.openRoomBoard()
			} else {
				m.roomScreen = roomScreen{}
			}
		}
		return nil

	case "lobby":
		if msg.Round < 1 {
			return nil
		}
		r.resetPlayers(false)
		r.resetYou(false)
		r.view.Phase = "lobby"
		r.view.Round = msg.Round
		r.view.Results = nil
		r.view.GoAt, r.view.EndsAt = 0, 0
		if m.screen == screenRoomResult || m.screen == screenRoom {
			m.roomScreen = roomScreen{}
			m.screen = screenRoom
		}
		return nil

	default:
		// The five round messages delegate to the round half; anything the
		// client does not know is ignored, the same as an unknown field.
		return m.applyRoomRoundMsg(msg)
	}
}

// resetPlayers clears every player's round state: what a fresh round starts
// with. inRound is what InRound becomes — a countdown makes every connected
// seat one of the round; a lobby clears the last round's fact.
func (r *roomSession) resetPlayers(inRound bool) {
	for i, p := range r.view.Players {
		r.view.Players[i].InRound = inRound && p.Connected
		r.view.Players[i].Done, r.view.Players[i].Solved = false, false
		r.view.Players[i].Rows = nil
		r.view.Players[i].Guesses = 0
	}
}

// resetYou is our own row's share of the same reset, InRound among it.
func (r *roomSession) resetYou(inRound bool) {
	r.view.You = online.YouView{Seat: r.seat, InRound: inRound}
}

// upsertPlayer kinds a joined seat into the view, keeping the list sorted.
func (m *Model) upsertPlayer(p online.PlayerView) {
	r := m.room
	if r == nil {
		return
	}
	for i, old := range r.view.Players {
		if old.Seat == p.Seat {
			r.view.Players[i] = p
			sortPlayers(r.view.Players)
			return
		}
	}
	r.view.Players = append(r.view.Players, p)
	sortPlayers(r.view.Players)
}

func sortPlayers(ps []online.PlayerView) {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j].Seat < ps[j-1].Seat; j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

// removePlayer drops one seat from the view.
func (m *Model) removePlayer(seat int) {
	r := m.room
	if r == nil {
		return
	}
	kept := r.view.Players[:0]
	for _, p := range r.view.Players {
		if p.Seat != seat {
			kept = append(kept, p)
		}
	}
	r.view.Players = kept
}

// showRoom puts the room on screen in whatever form its state calls for: the
// lobby and the countdown are this screen's; a round under way is the board's
// when we are in it; a finished round is its result. The screens this leaves
// for are the ones that would draw what a round has already decided.
func (m *Model) showRoom() tea.Cmd {
	if m.room == nil {
		return nil
	}
	r := m.room
	m.roomJoin.busy = false
	m.roomMake.busy = false
	if m.screen == screenRoomJoin || m.screen == screenRoomMake {
		m.roomJoin = roomJoinScreen{}
		m.roomMake = roomMakeScreen{}
	}
	switch {
	case r.view.Mode == "async":
		if r.view.You.Done {
			m.openRoomResultFromView()
			return nil
		}
		if r.view.You.InRound {
			m.openRoomBoard()
			return nil
		}
		m.roomScreen = roomScreen{}
		m.screen = screenRoom
	case r.view.Phase == "lobby" || r.view.Phase == "countdown":
		m.roomScreen = roomScreen{}
		m.screen = screenRoom
	case r.view.Phase == "playing":
		if r.view.You.InRound {
			m.openRoomBoard()
			return nil
		}
		m.roomScreen = roomRoomWatching()
		m.screen = screenRoom
	case r.view.Phase == "results":
		m.openRoomResultFromView()
		return nil
	}
	return nil
}

// roomRoomWatching is the watcher's room screen: empty of notes.
func roomRoomWatching() roomScreen { return roomScreen{} }

// roomJoinRowCode is the row the code field sits on.
const roomJoinRowCode = 0

// updateRoomMake is the make screen's keyboard half, with the leave prompt's
// confirm routed through the same method the clicks use.
func (m *Model) updateRoomMake(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := &m.roomMake
	switch msg.String() {
	case "esc", "q":
		m.openSocialScreen()
		return m, nil
	case "up", "k":
		s.move(-1)
	case "down", "j":
		s.move(1)
	case "left", "h":
		s.cycle(-1)
	case "right", "l":
		s.cycle(1)
	case "enter", " ":
		return m, m.makeRoom()
	}
	return m, nil
}

// updateRoomJoin is the join screen's keyboard half. While the field owns
// text input, editField takes the keys; enter submits once whole.
func (m *Model) updateRoomJoin(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	submit, back := m.roomJoin.update(msg)
	switch {
	case back:
		m.openSocialScreen()
		return m, nil
	case submit:
		return m, m.joinRoom()
	}
	return m, nil
}

// updateRoomResult answers the result screen's keys. It is the one path both
// a keyboard and a click use, because a rematch may raise the lobby under the
// player before the screen is otherwise touched.
func (m *Model) updateRoomResult(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.openMenu()
	case "enter", "r":
		return m, m.reviewRoomResult()
	case "n":
		return m, m.rematchRoom()
	case "f":
		return m, m.refreshRoomResult()
	case "l":
		if room := m.room; room != nil && room.code == m.roomResult.r.code && room.mode == "live" {
			return m, m.leaveRoom()
		}
	}
	return m, nil
}

// refreshRoomResult reads the room's ranking again, so a card that has been
// sitting in the list can pick up everyone who played since. One ask at a
// time, and the answer is filed even after the player has moved on.
func (m *Model) refreshRoomResult() tea.Cmd {
	s := &m.roomResult
	g := s.r.g
	// Consent is the same one rule every network call goes through: the
	// screen's key alone is not the check.
	if s.refreshing || !s.canRefresh(g) || !m.consent() {
		return nil
	}
	s.refreshing = true
	s.note, s.noteErr = "refreshing…", false
	client, code, key, id := m.online, g.Room.Code, g.Room.Key, g.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		results, err := client.Results(ctx, code, key)
		return roomResultsMsg{id: id, results: results, err: err}
	}
}

// reviewRoomResult opens the saved round on the board: the board shows the
// round again, and enter there comes back to this screen (openResult).
func (m *Model) reviewRoomResult() tea.Cmd {
	if g := m.roomResult.r.g; g != nil {
		m.openGame(g, true)
	}
	return nil
}

// rematchRoom asks for the next round, only when there is an open live room

// rematchRoom asks for the next round, only when there is an open live room
// whose finished round is what this screen shows.
func (m *Model) rematchRoom() tea.Cmd {
	if room := m.room; room != nil && room.code == m.roomResult.r.code &&
		room.mode == "live" && room.conn != nil && room.view.Phase == "results" {
		return m.sendRoom(online.Rematch())
	}
	return nil
}

// roomOnScreen reports whether the screen the player is on belongs to this
// room: its room screen, its board, its result screen.
func (m *Model) roomOnScreen(r *roomSession) bool {
	if r == nil {
		return false
	}
	switch m.screen {
	case screenRoom:
		return true
	case screenGame:
		return m.game != nil && m.game.room == r
	case screenRoomResult:
		return m.roomResult.r.code == r.code
	}
	return false
}
