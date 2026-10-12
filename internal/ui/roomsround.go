package ui

// Playing a round: the board borrows gameScreen, the standings sit beside it,
// and a finished round reaches disk only through finishRoomRound.

import (
	"errors"
	"net/http"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/words"
)

// roomBoardGame is the board's picture of our round: our guesses and the
// server's marks, no answer. It is never saved — a room round reaches disk
// only when it is finished, through NewRoomRound — and an empty Answer is
// exactly what makes Validate refuse it if anything did try.
func roomBoardGame(r *roomSession) *game.Game {
	g := &game.Game{
		ID:          game.RoomRoundID(r.code, r.view.Round, r.seat),
		Length:      r.view.Length,
		MaxAttempts: r.view.Length + 1,
	}
	g.Guesses = append(g.Guesses, r.view.You.Words...)
	g.Marks = append(g.Marks, r.view.You.Marks...)
	switch {
	case r.view.You.Done && r.view.You.Solved:
		g.Status = game.Won
	case r.view.You.Done:
		g.Status = game.Lost
	default:
		g.Status = game.InProgress
	}
	return g
}

// openRoomBoard shows our round on the board. Unlike openGame it does not
// hand the screen a playtime sink: room time is counted once, when the round
// is saved, and anything else would count it twice.
func (m *Model) openRoomBoard() {
	m.anim.clear()
	m.game = newGameScreen(m.store, roomBoardGame(m.room), false)
	m.game.room = m.room
	m.game.anim = &m.anim
	m.game.outlines = m.outlines
	m.game.resize(m.width, m.height)
	m.screen = screenGame
}

// submitRoomGuess is the submit path while the board is a room's. It checks
// locally what the player can fix at once, then hands the guess to the server
// and keeps the row on screen until marks come back.
func (m *Model) submitRoomGuess() tea.Cmd {
	gs := m.game
	r := m.room
	if gs == nil || r == nil {
		return nil
	}
	// A terminal too short for the board takes no input, as the refusal says.
	if gs.refusing() {
		return nil
	}
	if gs.g.Status.Done() {
		// An enter on a finished room board is review: back to the result the
		// room already raised, or nothing when it never did.
		if r.view.Results != nil {
			return m.showRoom()
		}
		return nil
	}
	if r.pending != 0 {
		return nil
	}
	if len(gs.typing) < gs.g.Length {
		gs.notify("needs %d letters", gs.g.Length)
		gs.reject()
		return nil
	}
	if !words.IsValidGuess(gs.g.Length, gs.typing) {
		// The server checks again; this only saves a round trip.
		gs.notify("not in word list")
		gs.reject()
		return nil
	}
	if r.conn == nil {
		gs.notify("reconnecting — try again in a moment")
		return nil
	}
	n := len(gs.g.Guesses) + 1
	r.pending = n
	r.pendingWord = gs.typing
	return sendRoomCmd(r.conn, online.Guess(n, gs.typing))
}

// applyRoomRoundMsg is applyRoomMsg's round half: the messages PR 8 left out,
// in the shapes Appendix A names. A guest reader gets nothing that is not
// checked first: the marks land only when their shape and number fit.
func (m *Model) applyRoomRoundMsg(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	switch msg.T {
	case "marks":
		return m.applyMarks(msg)
	case "reject":
		return m.applyReject(msg)
	case "progress":
		return m.applyProgress(msg)
	case "done":
		return m.applyRoundDone(msg)
	case "results":
		return m.applyResults(msg)
	}
	return nil
}

func (m *Model) applyMarks(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	// A marks for a guess already in our list is a resend — the server
	// acknowledging again what it already scored — and nothing new is owed.
	if msg.N == 0 || msg.N <= len(r.view.You.Words) {
		return nil
	}
	if msg.N != r.pending {
		return nil
	}
	if !online.ValidMarks(msg.Marks, r.view.Length) {
		return nil
	}
	r.view.You.Words = append(r.view.You.Words, r.pendingWord)
	r.view.You.Marks = append(r.view.You.Marks, append([]game.Mark(nil), msg.Marks...))
	word := r.view.You.Words[len(r.view.You.Words)-1]
	r.pending, r.pendingWord = 0, ""
	if m.game == nil || m.game.room != r {
		return nil
	}
	// The board shows this room: the same row goes onto its display game, and
	// the row that just landed is the one it reveals.
	g := m.game.g
	g.Guesses = append(g.Guesses, word)
	g.Marks = append(g.Marks, append([]game.Mark(nil), msg.Marks...))
	m.game.typing = ""
	m.game.message = ""
	m.game.msgUntil = time.Time{}
	kind := animReveal
	switch {
	case allCorrect(g.Marks):
		g.Status = game.Won
		kind = animWin
	case len(g.Guesses) >= g.MaxAttempts:
		g.Status = game.Lost
		kind = animLoss
	}
	m.anim.beginBoard(kind, g.ID, len(g.Guesses)-1, g.Length)
	return nil
}

// allCorrect is the solved shape: every mark of the last row is right. It
// does not end the round — only the server says that — it is what the board's
// winning row is drawn as.
func allCorrect(marks [][]game.Mark) bool {
	if len(marks) == 0 {
		return false
	}
	last := marks[len(marks)-1]
	for _, m := range last {
		if m != game.Correct {
			return false
		}
	}
	return true
}

func (m *Model) applyReject(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	if msg.N != r.pending {
		return nil
	}
	pending := r.pendingWord
	r.pending, r.pendingWord = 0, ""
	if m.game == nil || m.game.room != r {
		return nil
	}
	switch msg.Reason {
	case "not_a_word":
		m.game.notify("not in word list")
		m.game.reject()
	case "wrong_length":
		m.game.notify("needs %d letters", r.view.Length)
		m.game.reject()
	case "not_playing":
		m.game.notify("the round isn't running")
		m.game.reject()
	case "out_of_order":
		// The server disagrees with the row we hold: its view is the truth.
		if m.game.typing == pending {
			m.openRoomBoard()
		}
	}
	return nil
}

func (m *Model) applyProgress(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil || msg.Seat == r.seat {
		return nil
	}
	for i, p := range r.view.Players {
		if p.Seat != msg.Seat {
			continue
		}
		if msg.N != len(r.view.Players[i].Rows)+1 {
			return nil
		}
		if !online.ValidMarks(msg.Marks, r.view.Length) {
			return nil
		}
		// A copy: the view's rows are handed to frames and never owned.
		r.view.Players[i].Rows = append(r.view.Players[i].Rows, append([]game.Mark(nil), msg.Marks...))
		r.view.Players[i].Guesses = len(r.view.Players[i].Rows)
		break
	}
	return nil
}

func (m *Model) applyRoundDone(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil {
		return nil
	}
	if msg.Solved == nil {
		return nil
	}
	if msg.Seat == r.seat {
		r.view.You.Done = true
		r.view.You.Solved = *msg.Solved
		r.view.You.ElapsedMS = msg.ElapsedMS
		if m.game != nil && m.game.room == r {
			if *msg.Solved {
				m.game.g.Status = game.Won
			} else {
				m.game.g.Status = game.Lost
			}
		}
		return nil
	}
	for i, p := range r.view.Players {
		if p.Seat == msg.Seat {
			r.view.Players[i].Done = true
			r.view.Players[i].Solved = *msg.Solved
			r.view.Players[i].Guesses = msg.Guesses
			break
		}
	}
	return nil
}

func (m *Model) applyResults(msg online.ServerMsg) tea.Cmd {
	r := m.room
	if r == nil || !online.ValidWord(msg.Word, r.view.Length) {
		return nil
	}
	for i, e := range msg.Ranking {
		if len(e.Rows) != e.Guesses || e.ElapsedMS < 0 || e.ElapsedMS > game.RoomRoundMS {
			return nil
		}
		for _, row := range e.Rows {
			if !online.ValidMarks(row, r.view.Length) {
				return nil
			}
		}
		if !game.ValidRoomName(e.Name) {
			msg.Ranking[i].Name = ""
		}
	}
	res := &online.Results{Round: msg.Round, Word: msg.Word, Ranking: msg.Ranking}
	r.view.Results = res
	if r.view.Mode == "live" {
		r.view.Phase = "results"
	}
	m.finishRoomRound()
	return nil
}

// finishRoomRound saves our finished round, once, and shows its result when
// the player is looking at this room. A watcher has nothing of their own to
// save: the ranking carries what they saw, not what they played.
func (m *Model) finishRoomRound() {
	r := m.room
	if r == nil {
		return
	}
	res := r.view.Results
	if res == nil {
		return
	}
	mine := -1
	for i, e := range res.Ranking {
		if e.Seat == r.seat {
			mine = i
			break
		}
	}
	if mine < 0 {
		// A watcher's result has them nowhere; nothing of ours to write.
		return
	}
	entry := res.Ranking[mine]
	ranking := make([]game.RoomRank, len(res.Ranking))
	for i, e := range res.Ranking {
		name := ""
		if game.ValidRoomName(e.Name) {
			name = e.Name
		}
		ranking[i] = game.RoomRank{
			Seat: e.Seat, Name: name, Solved: e.Solved, Guesses: e.Guesses,
			ElapsedMS: e.ElapsedMS, Rows: e.Rows,
		}
	}
	info := game.RoomInfo{
		Code: r.code, Mode: r.mode, Round: res.Round, Seat: r.seat,
		Ranking: ranking,
	}
	// An any-time room keeps reading newer results after this round: its key
	// is what the results endpoint answers.
	if r.mode == "async" {
		info.Key = r.key
	}
	g, err := game.NewRoomRound(info, res.Word, r.view.You.Words, entry.ElapsedMS, timeNow())
	if err != nil || !marksMatch(g.Marks, r.view.You.Marks) {
		m.note = "couldn't save this round"
		return
	}
	_, loadErr := m.store.Load(g.ID)
	existed := loadErr == nil
	if err := m.store.Save(g); err != nil {
		m.note = saveFailed(err)
		return
	}
	// A round replayed after a reconnect saves again and lands on the same
	// record; the playtime joins it once.
	if !existed {
		m.bankPlaytime(time.Duration(entry.ElapsedMS) * time.Millisecond)
	}
	if r.mode == "async" {
		// The round is done, so the room is not one we are waiting on.
		m.dropRoom(r.code)
	}
	if m.screen == screenRoom ||
		(m.screen == screenGame && m.game != nil && m.game.room == r) {
		if m.anim.busy(timeNow()) {
			m.pendingRoomResult = true
			return
		}
		m.showRoom()
	}
}

// marksMatch compares two rows of marks, position by position. The board's
// own scoring is the authority, so a server whose marks and word disagree
// cannot put a round on disk.
func marksMatch(got, want [][]game.Mark) bool {
	if len(got) != len(want) {
		return false
	}
	for i, row := range got {
		if len(row) != len(want[i]) {
			return false
		}
		for j, m := range row {
			if m != want[i][j] {
				return false
			}
		}
	}
	return true
}

// settleRefresh files a refresh's answer. The screen it belongs to is the
// judge of whether it still matters: one that is gone is one whose ranking
// nobody asked to update.
func (m *Model) settleRefresh(msg roomResultsMsg) tea.Cmd {
	if s := &m.roomResult; s.refreshing {
		s.refreshing = false
	}
	if m.screen != screenRoomResult || m.roomResult.r.g == nil ||
		m.roomResult.r.g.ID != msg.id {
		// The screen has been left, or the round on it is not the round the
		// answer names: nothing the answer is owed is on screen.
		return nil
	}
	if msg.err != nil {
		if m.markGone(msg.err) {
			return nil
		}
		var gone *online.StatusError
		if errors.As(msg.err, &gone) && gone.Code == http.StatusNotFound {
			m.roomSay("this room has closed", true)
			return nil
		}
		m.roomSay("couldn't refresh — try again", true)
		return nil
	}

	// The saved round grows: its copy carries the new ranking, with names the
	// room's own rule would still show, and it is refused when it no longer
	// validates — a record this build wrote cannot become one it would not.
	g := *m.roomResult.r.g
	g.Room = &game.RoomInfo{
		Code:    g.Room.Code,
		Mode:    g.Room.Mode,
		Round:   g.Room.Round,
		Seat:    g.Room.Seat,
		Key:     g.Room.Key,
		Ranking: convertRanking(msg.results.Ranking),
	}
	if g.Validate() != nil {
		m.roomSay("couldn't refresh — try again", true)
		return nil
	}
	if err := m.store.Save(&g); err != nil {
		m.note = saveFailed(err)
		return nil
	}
	m.roomResult = roomResultScreen{r: buildRoomResult(&g), note: "updated"}
	m.roomResult.resize(m.width, m.height)
	m.screen = screenRoomResult
	return nil
}

// convertRanking is one ranking's shape change at the refresh's edge.
func convertRanking(entries []online.RankEntry) []game.RoomRank {
	out := make([]game.RoomRank, len(entries))
	for i, e := range entries {
		name := ""
		if game.ValidRoomName(e.Name) {
			name = e.Name
		}
		out[i] = game.RoomRank{
			Seat: e.Seat, Name: name, Solved: e.Solved, Guesses: e.Guesses,
			ElapsedMS: e.ElapsedMS, Rows: e.Rows,
		}
	}
	return out
}

// buildRoomResult is one saved round's card.
func buildRoomResult(g *game.Game) roomResult {
	return roomResult{
		code:    g.Room.Code,
		mode:    g.Room.Mode,
		round:   g.Room.Round,
		seat:    g.Room.Seat,
		length:  g.Length,
		word:    g.Answer,
		ranking: g.Room.Ranking,
		g:       g,
	}
}

// roomResultsMsg is one refresh's answer, named by the round it belongs to.
type roomResultsMsg struct {
	id      string // the saved round's game id
	results online.Results
	err     error
}
