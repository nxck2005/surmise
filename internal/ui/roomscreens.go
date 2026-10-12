package ui

// The room screens: making a room, joining one by code, and the room itself
// with its lobby, countdown, watch and open views. The result screen arrives
// with PR 9; its stub here only keeps unreachable draws away from the real
// layout.
//
// Every player-facing string here is Appendix C's, verbatim.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// --- making a room ---

const (
	roomMakeLength = iota
	roomMakePlay
	roomMakeStart
	roomMakeRows
)

type roomMakeScreen struct {
	length    int
	async     bool
	autoStart bool
	cursor    int

	busy bool

	note    string
	noteErr bool
}

func newRoomMake(length int) roomMakeScreen {
	m := roomMakeScreen{length: length, autoStart: true}
	if !words.SupportedLength(m.length) {
		m.length = defaultLength
	}
	return m
}

// move skips the start row when it is disabled, exactly as settings does.
// A live round that waits for the host has no "when everyone's ready" choice.
func (m *roomMakeScreen) enabled(row int) bool {
	if row == roomMakeStart && m.async {
		return false
	}
	return true
}

func (m *roomMakeScreen) move(delta int) {
	for row := m.cursor + delta; row >= 0 && row < roomMakeRows; row += delta {
		if m.enabled(row) {
			m.cursor = row
			return
		}
	}
}

func (m *roomMakeScreen) point(row int) bool {
	if row < 0 || row >= roomMakeRows || !m.enabled(row) || m.cursor == row {
		return false
	}
	m.cursor = row
	return true
}

func (m *roomMakeScreen) cycle(delta int) {
	if !m.enabled(m.cursor) {
		return
	}
	switch m.cursor {
	case roomMakeLength:
		m.length = stepLength(m.length, delta)
	case roomMakePlay:
		m.async = !m.async
	case roomMakeStart:
		m.autoStart = !m.autoStart
	}
}

// rowNote is the one-line explanation under the rows, for the cursor row.
func (m *roomMakeScreen) rowNote() string {
	switch m.cursor {
	case roomMakeLength:
		return "everyone in the room gets a word this long"
	case roomMakePlay:
		if m.async {
			return "friends play whenever they like, for 30 days"
		}
		return "everyone plays at once and sees each other guess"
	case roomMakeStart:
		if m.autoStart {
			return "the round starts once everyone's ready"
		}
		return "everyone readies up, then you start it"
	}
	return ""
}

func (m *roomMakeScreen) startLabel() string {
	if m.async {
		// An any-time room starts on the player's own clock, so the row is dead.
		return "when you start it"
	}
	if m.autoStart {
		return "when everyone's ready"
	}
	return "when the host starts it"
}

// renderRow is sprint's row with settings' disabled rule: a dead row keeps its
// space, loses its arrows and never gains a target.
func (m *roomMakeScreen) renderRow(h *hitMap, row int, label, value string) string {
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	labelStyle, valueStyle := st.muted, st.muted

	if m.enabled(row) {
		prev := action{kind: actRoomMakePrev, index: row}
		next := action{kind: actRoomMakeNext, index: row}
		arrow := func(a action, glyph string) string {
			style := st.muted
			if h.hovered(a) {
				style = st.hover(st.accent)
			}
			return h.mark(a, style.Render(glyph))
		}
		if row == m.cursor {
			prefix = st.cursor.Render(st.glyph.Cursor)
			labelStyle, valueStyle = st.text, st.accent
		}
		if h.hovered(next) {
			valueStyle = st.hover(valueStyle)
		}
		cell := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center).
			Render(valueStyle.Render(value))
		cell = h.mark(next, cell)
		return prefix +
			labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
			arrow(prev, st.glyph.ValuePrev) +
			cell +
			arrow(next, st.glyph.ValueNext)
	}

	valueBox := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center)
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
		strings.Repeat(" ", lipgloss.Width(st.glyph.ValuePrev)) +
		valueBox.Render(valueStyle.Render(value)) +
		strings.Repeat(" ", lipgloss.Width(st.glyph.ValueNext))
}

func (m *roomMakeScreen) view(h *hitMap) string {
	rows := []string{
		m.renderRow(h, roomMakeLength, "length", fmt.Sprintf("%d letters", m.length)),
		m.renderRow(h, roomMakePlay, "play", makePlayLabel(m.async)),
		m.renderRow(h, roomMakeStart, "start", m.startLabel()),
	}
	note, noteStyle := m.rowNote(), st.muted
	if m.note != "" {
		note = m.note
		noteStyle = st.muted
		if m.noteErr {
			noteStyle = st.err
		}
	}
	noteBox := lipgloss.NewStyle().Width(maxNoteWidth()).Align(lipgloss.Center).
		Render(noteStyle.Render(safeText(note)))
	rows = append(rows, "", noteBox)
	return lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
}

func makePlayLabel(v bool) string {
	if v {
		return "any time"
	}
	return "together now"
}

// maxNoteWidth is the widest of the row notes: the box stays one width so
// moving the cursor does not resize the panel.
func maxNoteWidth() int {
	widest := lipgloss.Width("everyone in the room gets a word this long")
	widest = max(widest, lipgloss.Width("everyone plays at once and sees each other guess"))
	widest = max(widest, lipgloss.Width("friends play whenever they like, for 30 days"))
	widest = max(widest, lipgloss.Width("the round starts once everyone's ready"))
	widest = max(widest, lipgloss.Width("everyone readies up, then you start it"))
	return widest
}

func (m *roomMakeScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "←", label: "previous", act: action{kind: actRoomMakePrev, index: m.cursor}},
		helpItem{keys: "→", label: "next", act: action{kind: actRoomMakeNext, index: m.cursor}},
		helpItem{keys: "enter", label: "make room", act: action{kind: actRoomMakeStart}},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
	)
}

// makeRoom asks the server for one. It is the enter/click/actRoomMakeStart
// path; a second key while one is out is nothing.
func (m *Model) makeRoom() tea.Cmd {
	s := &m.roomMake
	if s.busy {
		return nil
	}
	if active := m.activeRooms(); len(active) >= store.MaxRooms {
		s.noteErr = true
		s.note = "you're in 5 rooms already — leave one first"
		return nil
	}
	s.busy = true
	s.note, s.noteErr = "making the room…", false
	mode := "live"
	if s.async {
		mode = "async"
	}
	opts := online.RoomOptions{Mode: mode, Length: s.length, AutoStart: s.autoStart && !s.async}
	return createRoomCmd(m.online, opts)
}

// --- joining one ---

// roomJoinScreen asks for the 6-character code. It is a field screen exactly
// like the challenge entry: one row, editor marks, help that follows.
type roomJoinScreen struct {
	entry textField

	busy bool

	note    string
	noteErr bool
}

func newRoomJoin() roomJoinScreen {
	m := roomJoinScreen{entry: newRoomCodeField()}
	m.entry.value = ""
	m.entry.begin()
	return m
}

// newRoomCodeField is the code's editor: upper-cases a–z, and accepts only the
// characters a code is made of.
func newRoomCodeField() textField {
	return newTextField(6, func(r rune) (rune, bool) {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		return r, r >= '0' && r <= '9' || r >= 'A' && r <= 'Z'
	})
}

func (m *roomJoinScreen) update(msg tea.KeyPressMsg) (submit bool, back bool) {
	if m.entry.editing {
		enter := msg.String() == "enter"
		editField(&m.entry, msg)
		return enter, false
	}
	switch msg.String() {
	case "esc", "q":
		return false, true
	case "enter", " ":
		m.entry.begin()
	}
	return false, false
}

func (m *roomJoinScreen) view(h *hitMap) string {
	note, noteStyle := "type the 6-character code", st.muted
	if m.note != "" {
		note = m.note
		if m.noteErr {
			noteStyle = st.err
		}
	}
	return lipgloss.JoinVertical(lipgloss.Center,
		renderFieldRow(h, 0, true, "room code", &m.entry, "not set"),
		"",
		noteStyle.Render(safeText(note)),
	)
}

func (m *roomJoinScreen) help(h *hitMap) string {
	if m.entry.editing {
		return fieldHelp(h, 0, "join")
	}
	return renderHelp(h,
		helpItem{keys: "enter", label: "type", act: action{kind: actFieldEdit, index: 0}},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
	)
}

// joinRoom reads the typed code. The screen's note is where the answer goes,
// and a refused code resumes editing at once, like a challenge's does.
func (m *Model) joinRoom() tea.Cmd {
	s := &m.roomJoin
	if s.busy {
		return nil
	}
	code := s.entry.value
	if !game.ValidRoomCode(code) {
		s.noteErr = true
		s.note = "room codes are 6 letters and numbers"
		s.entry.begin()
		return nil
	}
	for _, e := range m.activeRooms() {
		if e.Code == code {
			return m.openRoomCode(e.Code, e.Key)
		}
	}
	if len(m.activeRooms()) >= store.MaxRooms {
		s.noteErr = true
		s.note = "you're in 5 rooms already — leave one first"
		return nil
	}
	s.busy = true
	s.note, s.noteErr = "connecting…", false
	return m.openRoomCode(code, "")
}

// --- the room itself ---

// roomScreen draws the room from the session; it keeps no copy of it. The
// result screen is PR 9's; nothing raises screenRoomResult here.
type roomScreen struct {
	note    string
	noteErr bool
}

// liveOrOpen is the shape of the first two lines every room view starts with.
func (m *roomScreen) header(h *hitMap, r *roomSession) []string {
	v := &r.view
	codeStyle := st.accent
	if h.hovered(action{kind: actRoomCopy}) {
		codeStyle = st.hover(codeStyle)
	}
	head := st.muted.Render("room ") + h.mark(action{kind: actRoomCopy}, codeStyle.Render(v.Code))
	meta := st.muted.Render(roomMeta(r))
	return []string{head, meta, ""}
}

func roomMeta(r *roomSession) string {
	v := &r.view
	if v.Mode == "async" {
		when := "closes today"
		if d := daysLeft(r.local(v.ExpiresAt)); d > 1 {
			when = fmt.Sprintf("open for %d more days", d)
		} else if d == 1 {
			when = "open for 1 more day"
		}
		return fmt.Sprintf("any time · %d letters · %s", v.Length, when)
	}
	starts := "starts when everyone's ready"
	if !v.AutoStart {
		starts = "the host starts it"
	}
	return fmt.Sprintf("together now · %d letters · %s", v.Length, starts)
}

// daysLeft is the whole days from now to the moment, rounded up.
func daysLeft(at time.Time) int {
	d := time.Until(at)
	if d <= 0 {
		return 0
	}
	return int((d + 23*time.Hour) / (24 * time.Hour))
}

// renderRoomPlayers is the list the lobby and the watching view share: one row
// per player, the cursor on ours, and the state column the caller supplies.
func renderRoomPlayers(r *roomSession, states func(online.PlayerView) (string, lipgloss.Style)) string {
	v := &r.view
	blank := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	lines := make([]string, 0, len(v.Players))
	for _, p := range v.Players {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("player %d", p.Seat)
		}
		nameStyle := st.text
		if !p.Connected {
			nameStyle = st.muted
		}
		prefix := blank
		if p.Seat == r.seat {
			prefix = st.cursor.Render(st.glyph.Cursor)
		}
		line := prefix + nameStyle.Render(fmt.Sprintf("%-12s", name)) + "  "
		state, stateStyle := states(p)
		line += stateStyle.Render(state)
		if r.view.Mode == "live" && p.Seat == v.Host {
			line += st.muted.Render(" · host")
		}
		lines = append(lines, line)
	}
	return block(strings.Join(lines, "\n"))
}

// roomPlayersLobby is the lobby's state column.
func roomPlayersLobby(r *roomSession) func(online.PlayerView) (string, lipgloss.Style) {
	return func(p online.PlayerView) (string, lipgloss.Style) {
		if p.Connected && p.Ready {
			return "ready", st.accent
		}
		if p.Connected {
			return "not ready", st.muted
		}
		return "away", st.muted
	}
}

// roomPlayersPlaying is the watching view's state column.
func roomPlayersPlaying(h *hitMap) func(online.PlayerView) (string, lipgloss.Style) {
	return func(p online.PlayerView) (string, lipgloss.Style) {
		switch {
		case p.Solved:
			return fmt.Sprintf("solved in %d", p.Guesses), st.accent
		case p.Done:
			return "out", st.err
		default:
			return "playing", st.muted
		}
	}
}

// playerStates in question — the shape each form's note line takes.
func (m *roomScreen) lobbyNote(r *roomSession) (string, lipgloss.Style) {
	if m.note != "" {
		if m.noteErr {
			return m.note, st.err
		}
		return m.note, st.muted
	}
	v := &r.view
	if _, ok := r.me(); !ok {
		return "", st.muted
	}

	connected := make([]online.PlayerView, 0, len(v.Players))
	for _, p := range v.Players {
		if p.Connected {
			connected = append(connected, p)
		}
	}
	ready := 0
	for _, p := range connected {
		if p.Ready {
			ready++
		}
	}

	allReady := len(connected) > 0 && ready == len(connected)
	switch {
	case len(connected) < 2:
		return "waiting for someone to join — share the code", st.muted
	case v.AutoStart && !allReady:
		return fmt.Sprintf("starts when everyone's ready · %d of %d ready", ready, len(connected)), st.muted
	case !v.AutoStart && !allReady:
		return fmt.Sprintf("%d of %d ready", ready, len(connected)), st.muted
	case allReady && v.Host == r.seat:
		return "everyone's ready — press s to start", st.accent
	case allReady:
		return "everyone's ready — waiting for the host", st.muted
	}
	return "", st.muted
}

// view draws one of the four forms. It takes the session: the screen is a
// function of it.
func (m *roomScreen) view(h *hitMap, r *roomSession) string {
	if r == nil {
		return ""
	}
	v := &r.view
	lines := m.header(h, r)

	switch {
	case v.Mode == "live" && v.Phase == "lobby":
		lines = append(lines, renderRoomPlayers(r, roomPlayersLobby(r)), "")
		note, style := m.lobbyNote(r)
		lines = append(lines, style.Render(note))

	case v.Mode == "live" && v.Phase == "countdown":
		seconds := r.local(v.GoAt).Sub(timeNow()).Seconds()
		n := max(int(ceilSeconds(seconds)), 1)
		lines = append(lines, "", st.accent.Render(strconv.Itoa(n)), "", st.muted.Render("get ready"), "")

	case v.Mode == "live" && (v.Phase == "playing" || v.Phase == "results"):
		lines = append(lines, st.muted.Render("a round is under way — you'll play the next one"), "")
		lines = append(lines, renderRoomPlayers(r, roomPlayersPlaying(h)), "")

	default: // async "open"
		var who []string
		for _, p := range v.Players {
			if p.Done {
				name := p.Name
				if name == "" {
					name = fmt.Sprintf("player %d", p.Seat)
				}
				who = append(who, name)
			}
		}
		var played string
		if len(who) == 0 {
			played = "nobody has played yet"
		} else {
			played = fmt.Sprintf("%d played: ", len(who))
			across, added, width := played, 0, lipgloss.Width(played)
			for i, name := range who {
				part := name
				if i > 0 {
					part = ", " + name
				}
				// The line stays at the plan's bound; whoever did not fit is a
				// " and N more" at the end.
				if width+lipgloss.Width(part) <= 45 {
					across, width, added = across+part, width+lipgloss.Width(part), added+1
				}
			}
			if added < len(who) {
				across += fmt.Sprintf(" and %d more", len(who)-added)
			}
			played = across
		}
		lines = append(lines, st.muted.Render(played), "")
		if m.note != "" {
			style := st.muted
			if m.noteErr {
				style = st.err
			}
			lines = append(lines, style.Render(safeText(m.note)))
		} else {
			lines = append(lines, st.muted.Render("you get 10 minutes once you start"), "")
		}
	}
	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

// ceilSeconds is a duration's rounded-up seconds, so a countdown caught
// exactly on the second does not overstate by one.
func ceilSeconds(s float64) int {
	return int(math.Ceil(s))
}

// help is the bottom bar for whichever form is showing.
func (m *roomScreen) help(h *hitMap, r *roomSession) string {
	if r == nil {
		return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
	}
	v := &r.view
	switch {
	case v.Mode == "live" && v.Phase == "lobby":
		me, ok := r.me()
		ready := "ready up"
		if ok && me.Ready {
			ready = "not ready"
		}
		items := []helpItem{
			{keys: "enter", label: ready, act: action{kind: actRoomReady}},
		}
		if ok && v.Host == r.seat && !v.AutoStart {
			items = append(items, helpItem{keys: "s", label: "start", act: action{kind: actRoomStart}})
		}
		return renderHelp(h, append(items,
			helpItem{keys: "c", label: "copy code", act: action{kind: actRoomCopy}},
			helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)...)
	case v.Mode == "live" && v.Phase == "countdown":
		return renderHelp(h,
			helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	case v.Mode == "live" && (v.Phase == "playing" || v.Phase == "results"):
		return renderHelp(h,
			helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	default:
		return renderHelp(h,
			helpItem{keys: "enter", label: "start", act: action{kind: actRoomPlay}},
			helpItem{keys: "c", label: "copy code", act: action{kind: actRoomCopy}},
			helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	}
}

// key answers the room screen's keys. It reports nothing: every answer is a
// command the root returns.
func (m *Model) updateRoom(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	r := m.room
	if r == nil {
		m.openMenu()
		return m, nil
	}
	v := &r.view
	me, ourSeat := r.me()
	switch msg.String() {
	case "esc", "q":
		m.openMenu()
		return m, nil
	case "c":
		if r.view.Code != "" {
			m.roomScreen.note, m.roomScreen.noteErr = "copy requested", false
			return m, m.copyText(r.view.Code)
		}
	case "l":
		return m, m.leaveRoom()
	}
	if !ourSeat {
		return m, nil
	}
	switch {
	case v.Mode == "live" && v.Phase == "lobby":
		switch msg.String() {
		case "enter", " ":
			return m, m.sendRoom(online.SetReady(!me.Ready))
		case "s":
			if v.Host == r.seat && !v.AutoStart {
				return m, m.sendRoom(online.Start())
			}
		}
	case v.Mode == "async":
		switch msg.String() {
		case "enter", " ":
			return m, m.sendRoom(online.Start())
		}
	}
	return m, nil
}

// sendRoom writes one message on the open connection. nil when there is none:
// the listen loop reports the connection's state.
func (m *Model) sendRoom(msg online.ClientMsg) tea.Cmd {
	if m.room == nil || m.room.conn == nil {
		return nil
	}
	return sendRoomCmd(m.room.conn, msg)
}

// --- the round's result ---

// roomResult is one finished round as the result screen shows it: from a
// saved game (yours) or from the live view (a watcher's).
type roomResult struct {
	code string
	mode string

	round   int
	seat    int // ours; 0 for a watcher
	length  int
	word    string
	ranking []game.RoomRank
	g       *game.Game // the saved round, or nil
}

// roomResultScreen is what the result of a room round reads: the round, and
// a note when something could not be done. room is the session this screen
// belongs to, or nil: the keys above need to know whether this room is still
// open, and a viewer whose result came from a saved game reads the same field.
type roomResultScreen struct {
	room    *roomSession
	r       roomResult
	note    string
	noteErr bool
	// refreshing holds while the server is being asked for a new ranking; a
	// second f while it is out does nothing, and its answer is dropped when
	// the screen has been left.
	refreshing bool

	// width and height are the terminal's, pushed down by the root. The
	// ranking sheds to fit them; zero is unmeasured, which is unbounded.
	width, height int
}

// openRoomResultFor shows a saved round: the board we played, the ranking the
// server gave it, and the word now spoken.
func (m *Model) openRoomResultFor(g *game.Game) {
	m.anim.clear()
	m.roomResult = roomResultScreen{
		r: roomResult{
			code:    g.Room.Code,
			mode:    g.Room.Mode,
			round:   g.Room.Round,
			seat:    g.Room.Seat,
			length:  g.Length,
			word:    g.Answer,
			ranking: g.Room.Ranking,
			g:       g,
		},
	}
	m.screen = screenRoomResult
}

// openRoomResultFromView raises the round's result from the live view: the
// saved round when it exists, and a watcher's card when it does not.
func (m *Model) openRoomResultFromView() {
	r := m.room
	if r == nil || r.view.Results == nil {
		return
	}
	res := r.view.Results
	if g, err := m.store.Load(game.RoomRoundID(r.code, res.Round, r.seat)); err == nil && g.Room != nil {
		// Ours: the saved record is the same card the live one would build,
		// from disk, in case the room's view is older than the save.
		m.openRoomResultFor(g)
		return
	}
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
	m.anim.clear()
	m.roomResult = roomResultScreen{r: roomResult{
		code: r.code, mode: r.mode, round: res.Round,
		seat: 0, length: r.view.Length, word: res.Word,
		ranking: ranking,
	}}
	m.screen = screenRoomResult
}

// resize records the terminal size for the result screen's one decision: how
// many ranking rows it can afford.
func (m *roomResultScreen) resize(w, h int) { m.width, m.height = w, h }

// ordinal renders a place as 1st, 2nd, 3rd, 4th…, with 11th, 12th and 13th —
// and everything three off the tens — as th.
func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// view draws the result card: our outcome, the round we played, the ranking
// the server gave it, and the word the round kept until now.
func (m *roomResultScreen) view(h *hitMap) string {
	result := &m.r

	// The outcome: our place among the solved, counting from one; not this
	// time when we never solved; or the round number for a watcher, whose
	// own card is everybody else's.
	var outcome string
	switch {
	case result.seat == 0:
		outcome = st.muted.Render(fmt.Sprintf("round %d", result.round))
	case solvedBy(result.ranking, result.seat):
		place := 0
		for _, e := range result.ranking {
			if !e.Solved {
				place++
				continue
			}
			place++
			if e.Seat == result.seat {
				break
			}
		}
		if result.mode == "async" {
			outcome = st.accent.Render(fmt.Sprintf("%s of %d so far", ordinal(place), len(result.ranking)))
		} else {
			outcome = st.accent.Render(fmt.Sprintf("%s of %d", ordinal(place), len(result.ranking)))
		}
	default:
		outcome = st.err.Render("not this time")
	}

	title := fmt.Sprintf("room %s · round %d", result.code, result.round)
	if result.mode == "async" {
		title = fmt.Sprintf("room %s", result.code)
	}
	meta := fmt.Sprintf("%s · %d letters", metaMode(result.mode), result.length)
	if g := result.g; g != nil {
		meta += fmt.Sprintf(" · %s · %s", resultAttempts(g), formatDuration(g.Elapsed()))
	}

	rows := make([]string, 0, len(result.ranking))
	for i, e := range result.ranking {
		rows = append(rows, resultRankRow(h, result, i, e))
	}
	rows = m.fitRankingRows(rows)

	cards := []string{outcome, st.title.Render(title), st.muted.Render(meta), ""}
	cards = append(cards, rows...)
	cards = append(cards, "", answerLine(result))
	if m.note != "" {
		style := st.muted
		if m.noteErr {
			style = st.err
		}
		cards = append(cards, "", style.Render(safeText(m.note)))
	}
	return lipgloss.JoinVertical(lipgloss.Center, cards...)
}

// answerLine is the word the round kept until now, in capitals.
func answerLine(result *roomResult) string {
	return st.muted.Render("answer ") + st.text.Render(strings.ToUpper(result.word))
}

// metaMode is the mode word behind both the room screen's meta and this one's.
func metaMode(mode string) string {
	if mode == "async" {
		return "any time"
	}
	return "together now"
}

// solvedBy reports whether a seat's entry says it solved the round. A
// watcher's own seat is not in the ranking at all.
func solvedBy(ranking []game.RoomRank, seat int) bool {
	for _, e := range ranking {
		if e.Seat == seat {
			return e.Solved
		}
	}
	return false
}

// fitRankingRows sheds a ranking too long for the screen: the head with our
// row inside it and a "+N more", or the head, a mark where the fold is, and our
// row. An unmeasured height shows every row.
func (m *roomResultScreen) fitRankingRows(rows []string) []string {
	const otherLines = 9 // outcome, title, meta, three blanks, answer, note
	budget := bodyBudget(m.height) - otherLines
	if budget < 3 || len(rows) <= budget {
		return rows
	}
	ours := -1
	for i := range m.r.ranking {
		if m.r.ranking[i].Seat == m.r.seat {
			ours = i
			break
		}
	}
	if ours < 0 || ours <= budget-1 {
		return append(rows[:budget-1],
			st.muted.Render(fmt.Sprintf("+%d more", len(rows)-(budget-1))))
	}
	out := make([]string, 0, budget)
	out = append(out, rows[:budget-2]...)
	out = append(out, st.muted.Render("…"), rows[ours])
	return out
}

// resultRankRow is one line of the ranking: the place for solved entries in
// order (— for unsolved), the cursor on ours, the name, the score and the
// time.
func resultRankRow(h *hitMap, result *roomResult, index int, e game.RoomRank) string {
	place := "—"
	if e.Solved {
		place = ordinal(rankPlace(result, index))
	}
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	name := fmt.Sprintf("%-12s", e.Name)
	if e.Name == "" {
		name = fmt.Sprintf("%-12s", fmt.Sprintf("player %d", e.Seat))
	}
	nameStyle := st.muted
	if e.Seat == result.seat {
		nameStyle = st.text
		prefix = st.cursor.Render(st.glyph.Cursor)
	}
	played := fmt.Sprintf("X/%d", result.length)
	if e.Solved {
		played = fmt.Sprintf("%d/%d", e.Guesses, result.length)
	}
	playedStyle := st.muted
	if e.Seat == result.seat {
		playedStyle = st.accent
	}
	spent := st.muted.Render(formatDuration(time.Duration(e.ElapsedMS) * time.Millisecond))
	if !e.Solved {
		// Unsolved entries have no meaningful time; the board held ten exact
		// minutes and the row shows that long a gap.
		spent = st.muted.Render(formatDuration(time.Duration(game.RoomRoundMS) * time.Millisecond))
	}
	return fmt.Sprintf("%2s", st.muted.Render(place)) + "  " +
		prefix + nameStyle.Render(name) + "  " +
		fmt.Sprintf("%-4s", playedStyle.Render(played)) + "  " + spent
}

// rankPlace is a solved entry's place: 1, 2, … through the solved entries in
// rank order (a client never re-sorts what arrived).
func rankPlace(result *roomResult, index int) int {
	place := 0
	for i := 0; i <= index && i < len(result.ranking); i++ {
		if result.ranking[i].Solved {
			place++
		}
	}
	return place
}

// help lists exactly the actions this end offers, in this order. A watcher
// reviews nothing and has no room to rematch or leave.
func (m *roomResultScreen) help(h *hitMap) string {
	result := &m.r
	items := []helpItem{}
	if result.g != nil {
		items = append(items, helpItem{keys: "enter", label: "review", act: action{kind: actRoomReview}})
	}
	// Rematch and leave belong to a live room that is still open; an
	// any-time room's saved result is what f refreshes, for thirty days.
	if room := m.room; room != nil && room.code == result.code {
		switch {
		case room.mode == "live" && room.conn != nil && room.view.Phase == "results":
			items = append(items,
				helpItem{keys: "n", label: "rematch", act: action{kind: actRoomRematch}},
				helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}})
		case room.mode == "live":
			items = append(items, helpItem{keys: "l", label: "leave", act: action{kind: actRoomLeave}})
		case m.canRefresh(result.g):
			items = append(items, helpItem{keys: "f", label: "refresh", act: action{kind: actRoomRefresh}})
		}
	}
	return renderHelp(h, append(items,
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})...)
}

// canRefresh reports whether this screen's saved result may be read again:
// the round was played in an any-time room whose key is on record, the
// room's thirty days have not run out, and the player has consent.
func (s *roomResultScreen) canRefresh(g *game.Game) bool {
	if g == nil || g.Room == nil || g.Room.Mode != "async" || g.Room.Key == "" {
		return false
	}
	return timeNow().Before(g.StartedAt.Add(30 * 24 * time.Hour))
}
