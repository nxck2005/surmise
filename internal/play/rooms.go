package play

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// Rooms: one session at a time. Opening another room closes the current
// connection without leaving it; only LeaveRoom sends a leave. The session is
// the truth and RoomView a snapshot of it. Every connection has a generation,
// and anything that arrives for another one is from a socket already closed
// and is dropped. A round reaches disk only finished, through
// game.NewRoomRound under its derived id, and its playtime is banked once.

// Note is something to tell the player, as a code the front end words.
// Seq grows with every note, so the same code twice still shows twice.
//
// The codes: no_room, full, started, bad_key, left (the server's refusals),
// unreachable, rooms_off, save_failed, round_running and room_ended (the
// startup check; Room names the room), not_a_word, too_short and
// not_playing (a refused guess), reconnecting, room_closed, refresh_failed
// and updated (a result's refresh).
type Note struct {
	Code string `json:"code"`
	Room string `json:"room,omitempty"`
	Seq  int    `json:"seq"`
}

var (
	// ErrOffline is a room call without consent, or with rooms switched off.
	ErrOffline = &Error{Code: "offline"}
	// ErrTooManyRooms is joining a sixth room (store.MaxRooms).
	ErrTooManyRooms = &Error{Code: "too_many_rooms"}
	// ErrBadRoomCode is a code that is not 6 letters and numbers.
	ErrBadRoomCode = &Error{Code: "bad_room_code"}
	// ErrNoRoom is a call that needs an open room when there is none.
	ErrNoRoom = &Error{Code: "no_room"}
)

// roomBackoff is the wait before reconnect attempt n (1-based): 1, 2, 4, 8,
// 16 seconds, then 30 for every later one. A variable so a test can run it
// in milliseconds.
var roomBackoff = func(n int) time.Duration {
	if n >= 1 && n <= 5 {
		return time.Second << (n - 1)
	}
	return 30 * time.Second
}

// roomSession is the room this run is connected to.
type roomSession struct {
	code string
	key  string // our seat key; "" until the first welcome of a join
	mode string // "live" or "async"; "" until the first welcome of a join
	seat int

	gen      int
	conn     online.Conn
	out      chan online.ClientMsg // the writer's queue; nil while not connected
	view     online.RoomView
	offset   time.Duration // server clock minus ours
	welcomed bool

	typing string
	// pending is the guess number sent and not yet answered; 0 is none.
	pending     int
	pendingWord string

	retries int
	retry   *time.Timer
	leaving bool
	probe   bool
}

func (r *roomSession) local(ms int64) int64 {
	if ms <= 0 {
		return 0
	}
	return time.UnixMilli(ms).Add(-r.offset).UnixMilli()
}

// roomStore is the rooms half of a store.
type roomStore interface {
	Rooms() []store.RoomEntry
	SaveRooms([]store.RoomEntry) error
}

func (a *App) say(code, room string) {
	a.net.roomNote = Note{Code: code, Room: room, Seq: a.net.roomNote.Seq + 1}
}

// roomsOnline is consent, a store with the rooms half, and a server that has
// not switched rooms off.
func (a *App) roomsOnline() bool {
	if _, ok := a.store.(roomStore); !ok {
		return false
	}
	return a.consent() && (a.net.status == nil || a.net.status.Features.Rooms)
}

// activeRooms is the rooms list, newest first.
func (a *App) activeRooms() []store.RoomEntry {
	rs, ok := a.store.(roomStore)
	if !ok {
		return nil
	}
	sorted := slices.Clone(rs.Rooms())
	slices.SortStableFunc(sorted, func(x, y store.RoomEntry) int { return y.JoinedAt.Compare(x.JoinedAt) })
	return sorted
}

// upsertRoom records a room, keeping an existing entry's joined time.
func (a *App) upsertRoom(e store.RoomEntry) {
	rs, ok := a.store.(roomStore)
	if !ok {
		return
	}
	rooms := rs.Rooms()
	found := false
	for i, old := range rooms {
		if old.Code == e.Code {
			e.JoinedAt = old.JoinedAt
			rooms[i], found = e, true
		}
	}
	if !found {
		rooms = append(rooms, e)
	}
	if rs.SaveRooms(rooms) != nil {
		a.say("save_failed", "")
	}
}

func (a *App) dropRoom(code string) {
	rs, ok := a.store.(roomStore)
	if !ok {
		return
	}
	rooms := rs.Rooms()
	kept := slices.DeleteFunc(slices.Clone(rooms), func(e store.RoomEntry) bool { return e.Code == code })
	if len(kept) != len(rooms) && rs.SaveRooms(kept) != nil {
		a.say("save_failed", "")
	}
}

// RoomEntryView is one row of the rooms list.
type RoomEntryView struct {
	Code       string `json:"code"`
	Mode       string `json:"mode"`
	JoinedAtMS int64  `json:"joinedAtMs"`
	// Open says this is the room the session is on.
	Open bool `json:"open"`
}

// RoomsList is the rooms this install is in, newest first.
func (a *App) RoomsList() []RoomEntryView {
	out := []RoomEntryView{}
	for _, e := range a.activeRooms() {
		out = append(out, RoomEntryView{Code: e.Code, Mode: e.Mode, JoinedAtMS: e.JoinedAt.UnixMilli(),
			Open: a.net.room != nil && a.net.room.code == e.Code})
	}
	return out
}

func (a *App) roomHello(key string) online.ClientMsg {
	return online.Hello(a.net.host.ClientName, online.RoomName(a.Settings().DisplayName), key)
}

// --- the session's plumbing ---

// openRoom makes the session for a room after closing any other one. The
// room already open is left as it is: connected, dialling or walking back.
func (a *App) openRoom(code, key, mode string) {
	if r := a.net.room; r != nil && r.code == code {
		return
	}
	a.closeRoom()
	a.net.roomGen++
	a.net.room = &roomSession{code: code, key: key, mode: mode, gen: a.net.roomGen}
	a.dialRoom()
}

// dialRoom opens the socket and sends the hello, which carries the version,
// the cleaned name and the seat key held, if any. A hello that cannot be
// sent closes the socket: there is no second try on it.
func (a *App) dialRoom() {
	r := a.net.room
	client, code, hello, gen := a.net.host.Client, r.code, a.roomHello(r.key), r.gen
	type answer struct {
		conn online.Conn
		err  error
	}
	spawn(a, NotifyRoom, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
		defer cancel()
		conn, err := client.Dial(ctx, code)
		if err == nil {
			if err = conn.Send(ctx, hello); err != nil {
				_ = conn.Close()
				conn = nil
			}
		}
		return answer{conn, err}
	}, func(ans answer) {
		r := a.net.room
		if r == nil || r.gen != gen {
			if ans.conn != nil {
				_ = ans.conn.Close()
			}
			return
		}
		if ans.err != nil {
			a.roomLost(ans.err)
			return
		}
		a.attach(r, ans.conn)
	})
}

// attach makes conn the session's connection: a writer that sends in order,
// and a reader that hands each message back through Post.
func (a *App) attach(r *roomSession, conn online.Conn) {
	r.conn = conn
	r.out = make(chan online.ClientMsg, 32)
	go func(out <-chan online.ClientMsg) {
		defer conn.Close()
		for m := range out {
			ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
			// A failed send shows up as the reader's closed connection.
			_ = conn.Send(ctx, m)
			cancel()
		}
	}(r.out)
	h, gen := a.net.host, r.gen
	go func() {
		for {
			msg, err := conn.Receive(context.Background())
			h.Post(func() {
				a.received(gen, msg, err)
				if h.Notify != nil {
					h.Notify(NotifyRoom)
				}
			})
			if err != nil {
				return
			}
		}
	}()
}

func (a *App) received(gen int, msg online.ServerMsg, err error) {
	r := a.net.room
	if r == nil || r.gen != gen {
		return
	}
	if err != nil {
		a.roomLost(err)
		return
	}
	a.applyRoomMsg(msg)
}

// sendRoom queues one message on the open connection, in order. Without a
// connection it does nothing.
func (a *App) sendRoom(m online.ClientMsg) {
	r := a.net.room
	if r == nil || r.out == nil {
		return
	}
	select {
	case r.out <- m:
	default:
		// A queue this full belongs to a dead socket; the reader will say so.
	}
}

// detach ends the connection half of the session. The writer sends what it
// was given and closes the socket; closeNow closes it at once instead.
func (r *roomSession) detach(closeNow bool) {
	if r.retry != nil {
		r.retry.Stop()
		r.retry = nil
	}
	if r.out != nil {
		close(r.out)
		r.out = nil
	}
	if closeNow && r.conn != nil {
		_ = r.conn.Close()
	}
	r.conn = nil
}

// closeRoom drops the session without leaving: the room keeps running on
// the server. Opening another room, losing consent and being turned away all
// do this; only LeaveRoom sends a leave.
func (a *App) closeRoom() {
	if r := a.net.room; r != nil {
		r.detach(true)
	}
	a.net.room = nil
}

// roomLost is a closed or failed connection of the session: gone is gone,
// a room never welcomed is not retried, the player's own leave stays left,
// and anything else walks back with the wait doubling.
func (a *App) roomLost(err error) {
	r := a.net.room
	if r == nil {
		return
	}
	if a.markGone(err) {
		return
	}
	r.detach(true)
	if !r.welcomed {
		a.say("unreachable", r.code)
		a.closeRoom()
		return
	}
	if r.leaving {
		a.closeRoom()
		return
	}
	r.retries++
	gen, h := r.gen, a.net.host
	r.retry = time.AfterFunc(roomBackoff(r.retries), func() {
		h.Post(func() {
			a.retryRoom(gen)
			if h.Notify != nil {
				h.Notify(NotifyRoom)
			}
		})
	})
}

func (a *App) retryRoom(gen int) {
	r := a.net.room
	if r == nil || r.gen != gen || r.conn != nil {
		return
	}
	r.retry = nil
	if !a.consent() {
		a.closeRoom()
		return
	}
	a.net.roomGen++
	r.gen = a.net.roomGen
	a.dialRoom()
}

// --- what the room says ---

// applyRoomMsg updates the session from one message, checking every field
// it can vary rather than trusting the wire; a message that fails is
// ignored. A message carrying `now` resets the clock offset first.
func (a *App) applyRoomMsg(msg online.ServerMsg) {
	r := a.net.room
	if msg.Now != 0 {
		r.offset = time.UnixMilli(msg.Now).Sub(a.clock.Now())
	}
	switch msg.T {
	case "welcome":
		if msg.Room == nil || msg.Seat < 1 {
			return
		}
		first := !r.welcomed
		r.seat, r.key, r.mode = msg.Seat, msg.Key, msg.Room.Mode
		r.view = *msg.Room
		r.welcomed, r.retries = true, 0
		a.upsertRoom(store.RoomEntry{Code: r.view.Code, Key: r.key, Mode: r.mode, JoinedAt: a.clock.Now().UTC()})
		if first {
			a.finishRoomRound()
			return
		}
		// A re-welcome is the truth overtaking what we remember: a guess
		// that was out either landed while we were away or is sent again.
		if r.pending != 0 {
			if len(r.view.You.Words) >= r.pending {
				r.pending, r.pendingWord, r.typing = 0, "", ""
			} else {
				a.sendRoom(online.Guess(r.pending, r.pendingWord))
				return
			}
		}
		a.finishRoomRound()

	case "refuse":
		switch msg.Reason {
		case "no_room", "bad_key", "left":
			a.dropRoom(r.code)
		case "full", "started":
		default:
			return
		}
		a.say(msg.Reason, r.code)
		a.closeRoom()

	case "gone":
		a.markGone(&online.GoneError{Message: msg.Message})

	case "joined":
		if msg.Player == nil {
			return
		}
		p := *msg.Player
		if i := slices.IndexFunc(r.view.Players, func(o online.PlayerView) bool { return o.Seat == p.Seat }); i >= 0 {
			r.view.Players[i] = p
		} else {
			r.view.Players = append(r.view.Players, p)
		}
		slices.SortStableFunc(r.view.Players, func(x, y online.PlayerView) int { return x.Seat - y.Seat })

	case "left":
		r.view.Players = slices.DeleteFunc(r.view.Players, func(p online.PlayerView) bool { return p.Seat == msg.Seat })

	case "presence":
		if msg.Connected == nil {
			return
		}
		for i, p := range r.view.Players {
			if p.Seat == msg.Seat {
				r.view.Players[i].Connected = *msg.Connected
			}
		}

	case "ready":
		if msg.Ready == nil {
			return
		}
		for i, p := range r.view.Players {
			if p.Seat == msg.Seat {
				r.view.Players[i].Ready = *msg.Ready
			}
		}

	case "host":
		if msg.Seat >= 1 {
			r.view.Host = msg.Seat
		}

	case "countdown":
		if msg.GoAt <= 0 || msg.Round < 1 {
			return
		}
		r.view.Phase, r.view.GoAt, r.view.Round = "countdown", msg.GoAt, msg.Round
		// The seats connected when the countdown started are the round's.
		r.resetRound(true)

	case "go":
		if msg.EndsAt <= 0 || msg.Round < 1 {
			return
		}
		if r.view.Mode == "async" {
			// An any-time round is one player's own clock.
			r.view.You.InRound = true
		} else {
			r.view.Phase = "playing"
		}
		r.view.EndsAt = msg.EndsAt

	case "lobby":
		if msg.Round < 1 {
			return
		}
		r.resetRound(false)
		r.view.Phase, r.view.Round = "lobby", msg.Round
		r.view.GoAt, r.view.EndsAt = 0, 0

	case "marks":
		a.applyMarks(msg)
	case "reject":
		a.applyReject(msg)
	case "progress":
		a.applyProgress(msg)
	case "done":
		a.applyDone(msg)
	case "results":
		a.applyResults(msg)
	}
}

// resetRound clears every seat's round: a countdown makes each connected
// seat one of the round, a lobby clears the last round's fact.
func (r *roomSession) resetRound(inRound bool) {
	for i, p := range r.view.Players {
		r.view.Players[i].InRound = inRound && p.Connected
		r.view.Players[i].Done, r.view.Players[i].Solved = false, false
		r.view.Players[i].Rows, r.view.Players[i].Guesses = nil, 0
	}
	r.view.You = online.YouView{Seat: r.seat, InRound: inRound}
	r.view.Results = nil
	r.typing, r.pending, r.pendingWord = "", 0, ""
}

func (a *App) applyMarks(msg online.ServerMsg) {
	r := a.net.room
	// A marks for a guess already in our list is a resend.
	if msg.N == 0 || msg.N <= len(r.view.You.Words) || msg.N != r.pending {
		return
	}
	if !online.ValidMarks(msg.Marks, r.view.Length) {
		return
	}
	r.view.You.Words = append(r.view.You.Words, r.pendingWord)
	r.view.You.Marks = append(r.view.You.Marks, slices.Clone(msg.Marks))
	r.pending, r.pendingWord, r.typing = 0, "", ""
}

func (a *App) applyReject(msg online.ServerMsg) {
	r := a.net.room
	if msg.N != r.pending {
		return
	}
	pending := r.pendingWord
	r.pending, r.pendingWord = 0, ""
	switch msg.Reason {
	case "not_a_word", "not_playing":
		a.say(msg.Reason, r.code)
	case "wrong_length":
		a.say("too_short", r.code)
	case "out_of_order":
		// The server's rows are the truth: drop the row it would not take.
		if r.typing == pending {
			r.typing = ""
		}
	}
}

func (a *App) applyProgress(msg online.ServerMsg) {
	r := a.net.room
	if msg.Seat == r.seat {
		return
	}
	for i, p := range r.view.Players {
		if p.Seat != msg.Seat {
			continue
		}
		if msg.N != len(p.Rows)+1 || !online.ValidMarks(msg.Marks, r.view.Length) {
			return
		}
		r.view.Players[i].Rows = append(r.view.Players[i].Rows, slices.Clone(msg.Marks))
		r.view.Players[i].Guesses = len(r.view.Players[i].Rows)
		return
	}
}

func (a *App) applyDone(msg online.ServerMsg) {
	r := a.net.room
	if msg.Solved == nil {
		return
	}
	if msg.Seat == r.seat {
		r.view.You.Done, r.view.You.Solved, r.view.You.ElapsedMS = true, *msg.Solved, msg.ElapsedMS
		r.typing = ""
		return
	}
	for i, p := range r.view.Players {
		if p.Seat == msg.Seat {
			r.view.Players[i].Done, r.view.Players[i].Solved = true, *msg.Solved
			r.view.Players[i].Guesses = msg.Guesses
			return
		}
	}
}

func (a *App) applyResults(msg online.ServerMsg) {
	r := a.net.room
	if !online.ValidWord(msg.Word, r.view.Length) {
		return
	}
	for i, e := range msg.Ranking {
		if len(e.Rows) != e.Guesses || e.ElapsedMS < 0 || e.ElapsedMS > game.RoomRoundMS {
			return
		}
		for _, row := range e.Rows {
			if !online.ValidMarks(row, r.view.Length) {
				return
			}
		}
		if !game.ValidRoomName(e.Name) {
			msg.Ranking[i].Name = ""
		}
	}
	r.view.Results = &online.Results{Round: msg.Round, Word: msg.Word, Ranking: msg.Ranking}
	if r.view.Mode == "live" {
		r.view.Phase = "results"
	}
	a.finishRoomRound()
}

func convertRanking(entries []online.RankEntry) []game.RoomRank {
	out := make([]game.RoomRank, len(entries))
	for i, e := range entries {
		name := ""
		if game.ValidRoomName(e.Name) {
			name = e.Name
		}
		out[i] = game.RoomRank{Seat: e.Seat, Name: name, Solved: e.Solved, Guesses: e.Guesses, ElapsedMS: e.ElapsedMS, Rows: e.Rows}
	}
	return out
}

// finishRoomRound saves our finished round. A watcher has nothing of their
// own to save. The marks are scored again here from the word, and a server
// whose marks disagree cannot put a round on disk. A round saved again
// after a reconnect lands on the same record, and its time is banked once.
func (a *App) finishRoomRound() {
	r := a.net.room
	res := r.view.Results
	if res == nil {
		return
	}
	mine := slices.IndexFunc(res.Ranking, func(e online.RankEntry) bool { return e.Seat == r.seat })
	if mine < 0 {
		return
	}
	entry := res.Ranking[mine]
	info := game.RoomInfo{Code: r.code, Mode: r.mode, Round: res.Round, Seat: r.seat, Ranking: convertRanking(res.Ranking)}
	if r.mode == "async" {
		// Its key reads the newer results of an any-time room.
		info.Key = r.key
	}
	g, err := game.NewRoomRound(info, res.Word, r.view.You.Words, entry.ElapsedMS, a.clock.Now())
	if err != nil || !slices.EqualFunc(g.Marks, r.view.You.Marks, slices.Equal[[]game.Mark]) {
		a.say("save_failed", r.code)
		return
	}
	_, loadErr := a.store.Load(g.ID)
	if err := a.store.Save(g); err != nil {
		a.say("save_failed", r.code)
		return
	}
	if loadErr != nil {
		a.bankPlaytime(time.Duration(entry.ElapsedMS) * time.Millisecond)
	}
	if r.mode == "async" {
		// Done, so not a room we are waiting on.
		a.dropRoom(r.code)
	}
}

// --- what the player does ---

// MakeRoom asks the server for a room and joins it as seat 1. mode is "live"
// or "async"; an any-time room never starts by itself.
func (a *App) MakeRoom(mode string, length int, autoStart bool) error {
	if mode != "live" && mode != "async" || !slices.Contains(words.Lengths, length) {
		return &Error{Code: "bad_request"}
	}
	if !a.roomsOnline() {
		return ErrOffline
	}
	if len(a.activeRooms()) >= store.MaxRooms {
		return ErrTooManyRooms
	}
	if a.net.making {
		return nil
	}
	a.net.making = true
	opts := online.RoomOptions{Mode: mode, Length: length, AutoStart: autoStart && mode == "live"}
	client := a.net.host.Client
	type answer struct {
		created online.Created
		err     error
	}
	spawn(a, NotifyRoom, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		c, err := client.CreateRoom(ctx, opts)
		return answer{c, err}
	}, func(ans answer) {
		a.net.making = false
		if ans.err != nil {
			if a.markGone(ans.err) {
				return
			}
			var status *online.StatusError
			if errors.As(ans.err, &status) && status.Code == http.StatusServiceUnavailable {
				a.say("rooms_off", "")
			} else {
				a.say("unreachable", "")
			}
			return
		}
		a.upsertRoom(store.RoomEntry{Code: ans.created.Code, Key: ans.created.Key, Mode: opts.Mode, JoinedAt: a.clock.Now().UTC()})
		a.openRoom(ans.created.Code, ans.created.Key, opts.Mode)
	})
	return nil
}

// JoinRoom joins a room by its code, or rejoins one this install is in.
func (a *App) JoinRoom(code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !game.ValidRoomCode(code) {
		return ErrBadRoomCode
	}
	if !a.roomsOnline() {
		return ErrOffline
	}
	active := a.activeRooms()
	for _, e := range active {
		if e.Code == code {
			a.openRoom(e.Code, e.Key, e.Mode)
			return nil
		}
	}
	if len(active) >= store.MaxRooms {
		return ErrTooManyRooms
	}
	a.openRoom(code, "", "")
	return nil
}

// RoomType adds a letter to our row, while the round lets us type.
func (a *App) RoomType(c rune) {
	r := a.net.room
	if r == nil || !r.canType() {
		return
	}
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	if c < 'a' || c > 'z' || len(r.typing) >= r.view.Length {
		return
	}
	r.typing += string(c)
}

// RoomDelete removes the last typed letter.
func (a *App) RoomDelete() {
	if r := a.net.room; r != nil && r.canType() && r.typing != "" {
		r.typing = r.typing[:len(r.typing)-1]
	}
}

func (r *roomSession) canType() bool {
	y := r.view.You
	return r.welcomed && y.InRound && !y.Done && r.pending == 0 &&
		len(y.Words) < r.view.Length+1 && (r.view.Mode == "async" || r.view.Phase == "playing")
}

// RoomSubmit sends the typed row. What the player can fix at once is refused
// here; the server checks again, and the row stays on screen until its marks
// come back.
func (a *App) RoomSubmit() error {
	r := a.net.room
	if r == nil {
		return ErrNoRoom
	}
	if !r.canType() {
		return nil
	}
	if len(r.typing) < r.view.Length {
		return ErrTooShort
	}
	if !words.IsValidGuess(r.view.Length, r.typing) {
		return ErrNotAWord
	}
	if r.conn == nil {
		return &Error{Code: "reconnecting"}
	}
	r.pending = len(r.view.You.Words) + 1
	r.pendingWord = r.typing
	a.sendRoom(online.Guess(r.pending, r.typing))
	return nil
}

// RoomReady turns our seat's ready flag on or off.
func (a *App) RoomReady(ready bool) { a.sendRoom(online.SetReady(ready)) }

// RoomStart asks for the round: the host starts a live one, a seat its own
// any-time one.
func (a *App) RoomStart() { a.sendRoom(online.Start()) }

// RoomRematch asks for a live room's next round, from its results.
func (a *App) RoomRematch() {
	if r := a.net.room; r != nil && r.mode == "live" && r.view.Phase == "results" {
		a.sendRoom(online.Rematch())
	}
}

// LeaveRoom is the player choosing to go: one leave however the connection
// stands, and the room forgotten. code names the room, open or not.
func (a *App) LeaveRoom(code string) {
	if r := a.net.room; r != nil && r.code == code {
		r.leaving = true
		if r.out != nil {
			a.sendRoom(online.Leave())
			r.detach(false)
			a.net.room = nil
			a.dropRoom(code)
			return
		}
		a.closeRoom()
	}
	key := ""
	for _, e := range a.activeRooms() {
		if e.Code == code {
			key = e.Key
		}
	}
	a.dropRoom(code)
	if !a.consent() {
		return
	}
	// Not connected: dial, hello with the key, leave, and never mind what
	// any of it answered.
	client, hello := a.net.host.Client, a.roomHello(key)
	spawn(a, NotifyRoom, func() struct{} {
		ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
		defer cancel()
		conn, err := client.Dial(ctx, code)
		if err != nil {
			return struct{}{}
		}
		if conn.Send(ctx, hello) == nil {
			_ = conn.Send(ctx, online.Leave())
		}
		_ = conn.Close()
		return struct{}{}
	}, func(struct{}) {})
}

// CloseRoom steps away without leaving: the room keeps our seat.
func (a *App) CloseRoom() { a.closeRoom() }

// --- the startup check ---

// ProbeRooms asks, once a run, whether a round of ours is still running in
// one of our rooms. The first one found becomes the session, with the note
// round_running; rooms the server no longer knows are dropped, with
// room_ended when nothing was found. A session opened meanwhile wins.
func (a *App) ProbeRooms() {
	if a.net.probed || !a.roomsOnline() {
		return
	}
	entries := a.activeRooms()
	if len(entries) == 0 {
		return
	}
	a.net.probed = true
	client := a.net.host.Client
	hellos := make([]online.ClientMsg, len(entries))
	for i, e := range entries {
		hellos[i] = a.roomHello(e.Key)
	}
	type answer struct {
		entry   store.RoomEntry
		conn    online.Conn
		welcome online.ServerMsg
		dropped []string
		gone    string
	}
	spawn(a, NotifyRoom, func() answer {
		var ans answer
		for i, e := range entries {
			ctx, cancel := context.WithTimeout(context.Background(), online.DialTimeout)
			conn, err := client.Dial(ctx, e.Code)
			if err != nil {
				cancel()
				continue
			}
			if conn.Send(ctx, hellos[i]) != nil {
				cancel()
				_ = conn.Close()
				continue
			}
			cancel()
			readCtx, readCancel := context.WithTimeout(context.Background(), online.Timeout)
			served, err := conn.Receive(readCtx)
			readCancel()
			switch {
			case err != nil:
			case served.T == "welcome" && roundRunning(served.Room):
				ans.entry, ans.conn, ans.welcome = e, conn, served
				return ans
			case served.T == "refuse" && (served.Reason == "no_room" || served.Reason == "bad_key" || served.Reason == "left"):
				ans.dropped = append(ans.dropped, e.Code)
			case served.T == "gone":
				ans.gone = served.Message
				_ = conn.Close()
				return ans
			}
			_ = conn.Close()
		}
		return ans
	}, func(ans answer) {
		for _, code := range ans.dropped {
			a.dropRoom(code)
		}
		if len(ans.dropped) > 0 && ans.conn == nil {
			a.say("room_ended", "")
		}
		if ans.gone != "" {
			a.markGone(&online.GoneError{Message: ans.gone})
			return
		}
		if ans.conn == nil {
			return
		}
		if a.net.room != nil || !a.consent() {
			_ = ans.conn.Close()
			return
		}
		a.net.roomGen++
		w := ans.welcome
		r := &roomSession{code: ans.entry.Code, gen: a.net.roomGen, probe: true, welcomed: true,
			seat: w.Seat, key: w.Key, mode: w.Room.Mode, view: *w.Room}
		if w.Now != 0 {
			r.offset = time.UnixMilli(w.Now).Sub(a.clock.Now())
		}
		a.net.room = r
		a.upsertRoom(store.RoomEntry{Code: r.view.Code, Key: r.key, Mode: r.mode, JoinedAt: a.clock.Now().UTC()})
		a.attach(r, ans.conn)
		a.say("round_running", r.code)
	})
}

// roundRunning is a welcome whose round a player would want back: running,
// with our seat in it and not yet done.
func roundRunning(room *online.RoomView) bool {
	if room == nil || room.Mode != "async" && room.Phase != "countdown" && room.Phase != "playing" {
		return false
	}
	return room.You.InRound && !room.You.Done
}

// --- views ---

// RoomPlayerView is one seat. Rows are marks only, never letters.
type RoomPlayerView struct {
	Seat int `json:"seat"`
	// Name is the cleaned room name; empty means "player N".
	Name      string     `json:"name"`
	Connected bool       `json:"connected"`
	Ready     bool       `json:"ready"`
	InRound   bool       `json:"inRound"`
	Guesses   int        `json:"guesses"`
	Done      bool       `json:"done"`
	Solved    bool       `json:"solved"`
	Rows      [][]string `json:"rows"`
	You       bool       `json:"you"`
}

// RankView is one line of a round's ranking.
type RankView struct {
	Seat      int        `json:"seat"`
	Name      string     `json:"name"`
	Solved    bool       `json:"solved"`
	Guesses   int        `json:"guesses"`
	ElapsedMS int64      `json:"elapsedMs"`
	Rows      [][]string `json:"rows"`
	You       bool       `json:"you"`
}

// RoomResultView is one finished round: from the saved record when it is
// ours (ID set), or from the room for a watcher (Seat 0).
type RoomResultView struct {
	ID      string     `json:"id,omitempty"`
	Code    string     `json:"code"`
	Mode    string     `json:"mode"`
	Round   int        `json:"round"`
	Seat    int        `json:"seat"`
	Length  int        `json:"length"`
	Word    string     `json:"word"`
	Ranking []RankView `json:"ranking"`
	// CanRefresh: an any-time round whose results can still grow.
	CanRefresh bool `json:"canRefresh"`
	// CanRematch: the open live room is on this round's results.
	CanRematch bool `json:"canRematch"`
}

// RoomView is the session now.
type RoomView struct {
	// Active is false with no session; only Note is then set.
	Active bool `json:"active"`
	// Screen is where the room belongs: connecting, lobby (also the
	// countdown), board, watch (a live round we are not in) or result.
	Screen       string `json:"screen"`
	Code         string `json:"code"`
	Mode         string `json:"mode"`
	Length       int    `json:"length"`
	AutoStart    bool   `json:"autoStart"`
	Seat         int    `json:"seat"`
	Host         int    `json:"host"`
	IsHost       bool   `json:"isHost"`
	Connected    bool   `json:"connected"`
	Reconnecting bool   `json:"reconnecting"`
	Phase        string `json:"phase"`
	Round        int    `json:"round"`
	// The room's times, already on our clock; 0 is none.
	GoAtMS      int64            `json:"goAtMs"`
	EndsAtMS    int64            `json:"endsAtMs"`
	ExpiresAtMS int64            `json:"expiresAtMs"`
	Players     []RoomPlayerView `json:"players"`
	// Board is our round, when we are in one; its row being sent is
	// Pending.
	Board   *BoardView      `json:"board"`
	Pending bool            `json:"pending"`
	Results *RoomResultView `json:"results"`
	// Probe marks a session the startup check found.
	Probe bool `json:"probe"`
	Note  Note `json:"note"`
}

func markRows(rows [][]game.Mark) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		for j, m := range row {
			out[i][j] = m.String()
		}
	}
	return out
}

// RoomView is the session's snapshot.
func (a *App) RoomView() RoomView {
	r := a.net.room
	if r == nil {
		return RoomView{Note: a.net.roomNote, Players: []RoomPlayerView{}}
	}
	v := r.view
	out := RoomView{
		Active: true, Code: r.code, Mode: r.mode, Length: v.Length, AutoStart: v.AutoStart,
		Seat: r.seat, Host: v.Host, IsHost: r.seat != 0 && v.Host == r.seat,
		Connected: r.conn != nil && r.welcomed, Reconnecting: r.welcomed && r.conn == nil,
		Phase: v.Phase, Round: v.Round,
		GoAtMS: r.local(v.GoAt), EndsAtMS: r.local(v.EndsAt), ExpiresAtMS: r.local(v.ExpiresAt),
		Players: []RoomPlayerView{}, Pending: r.pending != 0, Probe: r.probe, Note: a.net.roomNote,
	}
	for _, p := range v.Players {
		name := p.Name
		if !game.ValidRoomName(name) {
			name = ""
		}
		out.Players = append(out.Players, RoomPlayerView{Seat: p.Seat, Name: name, Connected: p.Connected,
			Ready: p.Ready, InRound: p.InRound, Guesses: p.Guesses, Done: p.Done, Solved: p.Solved,
			Rows: markRows(p.Rows), You: p.Seat == r.seat})
	}
	if v.You.InRound || len(v.You.Words) > 0 {
		b := a.roomBoard(r)
		out.Board = &b
	}
	if v.Results != nil {
		res := a.liveResult(r)
		out.Results = &res
	}
	out.Screen = r.screen()
	return out
}

// screen is where the room belongs now, by the TUI's showRoom.
func (r *roomSession) screen() string {
	v := r.view
	switch {
	case !r.welcomed:
		return "connecting"
	case v.Mode == "async":
		switch {
		case v.You.Done && v.Results != nil:
			return "result"
		case v.You.InRound:
			return "board"
		}
		return "lobby"
	case v.Phase == "playing":
		if v.You.InRound {
			return "board"
		}
		return "watch"
	case v.Phase == "results":
		return "result"
	}
	return "lobby"
}

// roomBoard is our round as a board: our words and the server's marks, no
// answer until the round's results name it.
func (a *App) roomBoard(r *roomSession) BoardView {
	y := r.view.You
	g := &game.Game{
		ID:          game.RoomRoundID(r.code, r.view.Round, r.seat),
		Length:      r.view.Length,
		MaxAttempts: r.view.Length + 1,
		Guesses:     slices.Clone(y.Words),
		Marks:       slices.Clone(y.Marks),
	}
	g.Status = game.InProgress
	switch {
	case y.Done && y.Solved:
		g.Status = game.Won
	case y.Done:
		g.Status = game.Lost
	case allCorrect(g.Marks):
		// What the winning row is drawn as before the server says so.
		g.Status = game.Won
	}
	typing := r.typing
	if r.pending != 0 {
		typing = r.pendingWord
	}
	v := BoardView{
		ID: g.ID, Code: game.Code(g.ID), Length: g.Length, MaxAttempts: g.MaxAttempts,
		Rows: make([]Row, len(g.Guesses)), Typing: typing, Keys: Keys(g), Status: string(g.Status),
		ElapsedMS: y.ElapsedMS, Running: y.InRound && !y.Done, Kind: KindRoom,
	}
	marks := markRows(g.Marks)
	for i, w := range g.Guesses {
		v.Rows[i] = Row{Guess: w, Marks: marks[i]}
	}
	if res := r.view.Results; res != nil && g.Status.Done() {
		v.Answer = res.Word
	}
	return v
}

func allCorrect(marks [][]game.Mark) bool {
	if len(marks) == 0 {
		return false
	}
	for _, m := range marks[len(marks)-1] {
		if m != game.Correct {
			return false
		}
	}
	return true
}

func rankViews(ranking []game.RoomRank, seat int) []RankView {
	out := make([]RankView, len(ranking))
	for i, e := range ranking {
		out[i] = RankView{Seat: e.Seat, Name: e.Name, Solved: e.Solved, Guesses: e.Guesses,
			ElapsedMS: e.ElapsedMS, Rows: markRows(e.Rows), You: seat != 0 && e.Seat == seat}
	}
	return out
}

// liveResult is the session's results: our saved round when there is one,
// since the record may be newer than the view; a watcher's from the view.
func (a *App) liveResult(r *roomSession) RoomResultView {
	res := r.view.Results
	if g, err := a.store.Load(game.RoomRoundID(r.code, res.Round, r.seat)); err == nil && g.Room != nil {
		return a.savedResult(g)
	}
	v := RoomResultView{Code: r.code, Mode: r.mode, Round: res.Round, Length: r.view.Length,
		Word: res.Word, Ranking: rankViews(convertRanking(res.Ranking), 0)}
	v.CanRematch = r.mode == "live" && r.conn != nil && r.view.Phase == "results"
	return v
}

func (a *App) savedResult(g *game.Game) RoomResultView {
	v := RoomResultView{ID: g.ID, Code: g.Room.Code, Mode: g.Room.Mode, Round: g.Room.Round, Seat: g.Room.Seat,
		Length: g.Length, Word: g.Answer, Ranking: rankViews(g.Room.Ranking, g.Room.Seat), CanRefresh: a.canRefresh(g)}
	if r := a.net.room; r != nil && r.code == v.Code && r.mode == "live" && r.conn != nil &&
		r.view.Phase == "results" && r.view.Results != nil && r.view.Results.Round == v.Round {
		v.CanRematch = true
	}
	return v
}

// refreshFor is how long an any-time room's results can still be read.
const refreshFor = 30 * 24 * time.Hour

func (a *App) canRefresh(g *game.Game) bool {
	if g.Room == nil || g.Room.Mode != "async" || g.Room.Key == "" {
		return false
	}
	return a.clock.Now().Before(g.StartedAt.Add(refreshFor))
}

// RoomResult is a saved round's result, for the history.
func (a *App) RoomResult(id string) (RoomResultView, error) {
	g, err := a.store.Load(id)
	if err != nil {
		return RoomResultView{}, notFound(err)
	}
	if g.Room == nil || g.Room.Code == "" {
		return RoomResultView{}, &Error{Code: "not_found"}
	}
	return a.savedResult(g), nil
}

// RefreshRoomResult reads an any-time round's ranking again, so everyone who
// played since joins it. The answer lands in the saved record, with the note
// updated, room_closed or refresh_failed.
func (a *App) RefreshRoomResult(id string) error {
	g, err := a.store.Load(id)
	if err != nil {
		return notFound(err)
	}
	if !a.canRefresh(g) {
		return nil
	}
	if !a.consent() {
		return ErrOffline
	}
	if a.net.refreshing[id] {
		return nil
	}
	a.net.refreshing[id] = true
	client, code, key := a.net.host.Client, g.Room.Code, g.Room.Key
	type answer struct {
		results online.Results
		err     error
	}
	spawn(a, NotifyRoom, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		res, err := client.Results(ctx, code, key)
		return answer{res, err}
	}, func(ans answer) {
		delete(a.net.refreshing, id)
		if ans.err != nil {
			if a.markGone(ans.err) {
				return
			}
			var status *online.StatusError
			if errors.As(ans.err, &status) && status.Code == http.StatusNotFound {
				a.say("room_closed", code)
			} else {
				a.say("refresh_failed", code)
			}
			return
		}
		g, err := a.store.Load(id)
		if err != nil || g.Room == nil {
			return
		}
		// A record this build wrote cannot become one it would not.
		next := *g
		room := *g.Room
		room.Ranking = convertRanking(ans.results.Ranking)
		next.Room = &room
		if next.Validate() != nil {
			a.say("refresh_failed", code)
			return
		}
		if a.store.Save(&next) != nil {
			a.say("save_failed", code)
			return
		}
		a.say("updated", code)
	})
	return nil
}

// notFound is a load's error with a missing record named by its code.
func notFound(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return &Error{Code: "not_found", Err: err}
	}
	return err
}
