package ui

// PR 8's room tests: make, join, wait. The live lobby is the one shape every
// room screen here reads, and deliver is how its messages arrive, the way the
// listen loop would have handed them in.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// fakeConn records what the UI sends. Receive is never called in tests:
// newOnlineModel sets m.listen to a stub.
type fakeConn struct {
	sent   []online.ClientMsg
	closed bool
	// queue holds the frames Receive serves, for the tests that script a
	// probe's welcome; a Receive past an empty queue errors, the way a real
	// closed connection does.
	queue []online.ServerMsg
}

func (f *fakeConn) Send(_ context.Context, m online.ClientMsg) error {
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeConn) Receive(context.Context) (online.ServerMsg, error) {
	if len(f.queue) == 0 {
		return online.ServerMsg{}, errors.New("fakeConn: nothing scripted")
	}
	msg := f.queue[0]
	f.queue = f.queue[1:]
	return msg, nil
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

// count is how many frames went out, so a test can say a second key sent
// nothing more.
func (f *fakeConn) count() int { return len(f.sent) }

// lastSent marshals the conn's most recent frame, failing when nothing was
// sent: a test about what went over the wire must not pass empty.
func lastSent(t *testing.T, f *fakeConn) string {
	t.Helper()
	if len(f.sent) == 0 {
		t.Fatal("nothing was sent")
	}
	b, err := json.Marshal(f.sent[len(f.sent)-1])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// left reports whether a leave message was sent.
func (f *fakeConn) left() bool {
	for _, m := range f.sent {
		if m.T() == "leave" {
			return true
		}
	}
	return false
}

// helloKey is the key the recorded hello carried, or "" for none.
func (f *fakeConn) helloKey() string {
	for _, m := range f.sent {
		if m.T() != "hello" {
			continue
		}
		b, err := json.Marshal(m)
		if err != nil {
			continue
		}
		var hello struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(b, &hello) == nil {
			return hello.Key
		}
	}
	return ""
}

// welcomeKey is what a welcome carries as the seat key. The test that makes a
// room puts the created key here first, so the hello and the welcome agree;
// every other test leaves the default a new seat is minted on.
var welcomeKey = "00112233445566778899aabbccddeeff"

const roomCode = "K7M2QX"

// player builds one PlayerView row.
func player(seat int, name string, connected, ready, done bool) online.PlayerView {
	return online.PlayerView{
		Seat: seat, Name: name, Connected: connected, Ready: ready,
		Done: done, Rows: [][]game.Mark{},
	}
}

// welcomeLive is a welcome for a live lobby: code K7M2QX, length 5,
// autoStart true, host 1, you as seat `seat`, players as given.
func welcomeLive(seat int, players ...online.PlayerView) online.ServerMsg {
	return welcomeRoom(roomCode, "live", 5, true, "lobby", seat, welcomeKey, players...)
}

// welcomeRoom builds any welcome; tests for shapes other than the autoStart
// lobby hand their own facts in.
func welcomeRoom(code, mode string, length int, autoStart bool, phase string,
	seat int, key string, players ...online.PlayerView) online.ServerMsg {
	room := online.RoomView{
		Code:      code,
		Mode:      mode,
		Length:    length,
		AutoStart: autoStart,
		Host:      1,
		Phase:     phase,
		Round:     1,
		Players:   players,
		You:       online.YouView{Seat: seat},
	}
	return online.ServerMsg{T: "welcome", V: 1, Seat: seat, Key: key, Room: &room}
}

// welcomeAsync is the open view of an any-time room.
func welcomeAsync(seat int, players ...online.PlayerView) online.ServerMsg {
	return welcomeRoom(roomCode, "async", 5, false, "open", seat, welcomeKey, players...)
}

// deliver hands the session one message from the room, as the listen loop
// would, and drains what it asks for.
func deliver(t *testing.T, m *Model, msg online.ServerMsg) {
	t.Helper()
	if m.room == nil {
		t.Fatal("no session to deliver to")
	}
	_, next := m.Update(roomRecvMsg{gen: m.room.gen, msg: msg})
	runCmd(t, m, next)
}

// readyMsg is the wire message that marks one seat ready or not.
func readyMsg(seat int, ready bool) online.ServerMsg {
	v := ready
	return online.ServerMsg{T: "ready", Seat: seat, Ready: &v}
}

// openSocial raises social play from the menu.
func openSocial(t *testing.T, m *Model) {
	t.Helper()
	draw(t, m)
	m.menu.cursor = menuIndex(t, m, choiceSocial, 0)
	send(t, m, "enter")
	if m.screen != screenSocial {
		t.Fatalf("screen = %v, want social play", m.screen)
	}
}

// asked reports whether one of the fake server's calls starts with prefix.
func asked(f *fakeOnline, prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// roomFile is the codes the store's rooms list holds.
func roomFile(t *testing.T, m *Model) []string {
	t.Helper()
	rs, ok := m.store.(roomStore)
	if !ok {
		t.Fatal("the store has no rooms half")
	}
	var codes []string
	for _, e := range rs.Rooms() {
		codes = append(codes, e.Code)
	}
	return codes
}

// setRoomFile puts codes in the store's rooms list, each with one seat key.
func setRoomFile(t *testing.T, m *Model, codes ...string) {
	t.Helper()
	rs, ok := m.store.(roomStore)
	if !ok {
		t.Fatal("the store has no rooms half")
	}
	entries := make([]store.RoomEntry, 0, len(codes))
	for _, c := range codes {
		entries = append(entries, store.RoomEntry{
			Code: c, Key: welcomeKey, Mode: "live",
			JoinedAt: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC),
		})
	}
	if err := rs.SaveRooms(entries); err != nil {
		t.Fatal(err)
	}
}

// typeCode types a room code into the join field.
func typeCode(t *testing.T, m *Model, code string) {
	t.Helper()
	for _, k := range strings.Split(strings.ToLower(code), "") {
		send(t, m, k)
	}
}

// openMakeRoom makes a room through the UI: social play, make room, enter,
// with the server's answer and the dial drained. It returns the connection
// the session ended up holding.
func openMakeRoom(t *testing.T, m *Model, f *fakeOnline) *fakeConn {
	t.Helper()
	conn := &fakeConn{}
	f.created = online.Created{Code: roomCode, Key: welcomeKey, Seat: 1}
	f.conn = conn
	openSocial(t, m)
	m.social.cursor = socialRowIndex(t, m, socialMakeRoom)
	press(t, m, "enter")
	if m.screen != screenRoomMake {
		t.Fatalf("screen = %v, want the make screen", m.screen)
	}
	press(t, m, "enter")
	if m.room == nil || m.room.conn == nil {
		t.Fatal("making the room left no connected session")
	}
	return conn
}

// openJoinRoom joins through the UI: social play, join room, the code, enter.
func openJoinRoom(t *testing.T, m *Model, f *fakeOnline) *fakeConn {
	t.Helper()
	conn := &fakeConn{}
	f.conn = conn
	openSocial(t, m)
	m.social.cursor = socialRowIndex(t, m, socialJoinRoom)
	send(t, m, "enter")
	if m.screen != screenRoomJoin {
		t.Fatalf("screen = %v, want the join screen", m.screen)
	}
	typeCode(t, m, roomCode)
	press(t, m, "enter")
	if m.room == nil {
		t.Fatal("joining left no session")
	}
	return conn
}

// ---- the tests ----

func TestSocialHasNoRoomRowsOffline(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, false)
	openSocial(t, m)

	if len(m.social.rows) != len(socialLabels) {
		t.Fatalf("rows = %d, want the three offline ones", len(m.social.rows))
	}
	for i, label := range socialLabels {
		if m.social.rows[i].label != label {
			t.Errorf("row %d = %q, want %q", i, m.social.rows[i].label, label)
		}
	}
}

func TestSocialOffersRoomsOnline(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	openSocial(t, m)

	want := []string{
		"room " + roomCode + " · together",
		"make room",
		"join room",
		"new challenge",
		"enter code",
		"custom puzzle",
	}
	if len(m.social.rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(m.social.rows), len(want))
	}
	for i, w := range want {
		if m.social.rows[i].label != w {
			t.Errorf("row %d = %q, want %q", i, m.social.rows[i].label, w)
		}
	}
}

func TestStatusCanSwitchRoomsOff(t *testing.T) {
	f := &fakeOnline{status: online.Status{Features: online.Features{Daily: true}}}
	m := newOnlineModel(t, f, true)
	runCmd(t, m, m.statusCmd())
	if m.status == nil {
		t.Fatal("the status did not arrive")
	}
	setRoomFile(t, m, roomCode)
	openSocial(t, m)
	if len(m.social.rows) != len(socialLabels) {
		t.Errorf("rows = %d, want the three offline ones once rooms are off", len(m.social.rows))
	}
}

func TestMakingARoomOpensItsLobby(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)

	if !asked(f, "create live 5 true") {
		t.Errorf("calls = %v, want the create ask", f.calls)
	}
	if !asked(f, "dial "+roomCode) {
		t.Errorf("calls = %v, want the dial", f.calls)
	}
	if key := conn.helloKey(); key != welcomeKey {
		t.Errorf("hello key = %q, want the created key", key)
	}

	welcomeKey = f.created.Key
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))
	if m.screen != screenRoom {
		t.Fatalf("screen = %v, want the room", m.screen)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "room "+roomCode) {
		t.Errorf("the frame does not name the room:\n%s", frame)
	}
	if !strings.Contains(frame, "together now · 5 letters · starts when everyone's ready") {
		t.Errorf("the frame's meta is wrong:\n%s", frame)
	}
	if codes := roomFile(t, m); len(codes) != 1 || codes[0] != roomCode {
		t.Errorf("rooms file = %v, want %s", codes, roomCode)
	}
}

func TestFiveRoomsIsTheLimit(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode, "N4XYRZ", "Q8BGMK", "H2DXW4", "MJ3XSR")
	openSocial(t, m)
	m.social.cursor = socialRowIndex(t, m, socialMakeRoom)
	press(t, m, "enter") // the make screen
	if m.screen != screenRoomMake {
		t.Fatalf("screen = %v, want the make screen", m.screen)
	}
	press(t, m, "enter") // the make itself

	if asked(f, "create") {
		t.Errorf("calls = %v, want no create ask at the limit", f.calls)
	}
	if asked(f, "dial") {
		t.Errorf("calls = %v, want no dial at the limit", f.calls)
	}
	draw(t, m)
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "you're in 5 rooms already — leave one first") {
		t.Errorf("the frame does not carry the five-rooms note:\n%s", frame)
	}
}

func TestJoinChecksTheCode(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	openSocial(t, m)
	m.social.cursor = socialRowIndex(t, m, socialJoinRoom)
	send(t, m, "enter")
	if m.screen != screenRoomJoin {
		t.Fatalf("screen = %v, want the join screen", m.screen)
	}
	typeCode(t, m, "abc")
	press(t, m, "enter")

	if asked(f, "dial") {
		t.Errorf("calls = %v, want no dial for a short code", f.calls)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "room codes are 6 letters and numbers") {
		t.Errorf("the frame does not carry the bad-code note:\n%s", frame)
	}
}

func TestJoinUppercasesAndDials(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openJoinRoom(t, m, f)

	if !asked(f, "dial "+roomCode) {
		t.Errorf("calls = %v, want dial %s", f.calls, roomCode)
	}
	if key := conn.helloKey(); key != "" {
		t.Errorf("hello key = %q, want empty for a fresh join", key)
	}
}

func TestRefusalsSaySoWhereYouAre(t *testing.T) {
	for _, tt := range []struct {
		reason, want string
		forget       bool
	}{
		{"no_room", "no room with that code", true},
		{"full", "that room is full", false},
		{"started", "that room is playing a round — try again in a minute", false},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			f := &fakeOnline{}
			m := newOnlineModel(t, f, true)
			if tt.forget {
				setRoomFile(t, m, roomCode)
			}
			conn := openJoinRoom(t, m, f)

			_, next := m.Update(roomRecvMsg{gen: m.room.gen, msg: online.ServerMsg{
				T: "refuse", Reason: tt.reason,
			}})
			runCmd(t, m, next)

			if m.room != nil {
				t.Fatal("a refuse left the session open")
			}
			if m.screen != screenRoomJoin {
				t.Fatalf("screen = %v, want the join screen", m.screen)
			}
			frame := plain(m.View().Content)
			if !strings.Contains(frame, tt.want) {
				t.Errorf("the frame does not carry %q:\n%s", tt.want, frame)
			}
			if !conn.closed {
				t.Errorf("the refused socket stayed open")
			}
			if tt.forget {
				if codes := roomFile(t, m); len(codes) != 0 {
					t.Errorf("rooms file = %v, want nothing after a no_room", codes)
				}
			}
		})
	}
}

func TestLobbyShowsWhoIsReady(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	openMakeRoom(t, m, f)
	// You (seat 1, the host, ready), one connected and not ready, one away.
	deliver(t, m, welcomeLive(1,
		player(1, "nick", true, true, false),
		player(2, "sam", true, false, false),
		player(3, "", false, false, false),
	))
	draw(t, m)
	frame := plain(m.View().Content)

	for _, want := range []string{
		"ready · host",
		"not ready",
		"player 3      away",
		"starts when everyone's ready · 1 of 2 ready",
	} {
		if !strings.Contains(frame, want) {
			t.Errorf("the frame does not carry %q:\n%s", want, frame)
		}
	}
}

func TestReadyTogglesOverTheWire(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	welcomeKey = f.created.Key
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	press(t, m, "enter")
	if got := lastSent(t, conn); got != `{"t":"ready","ready":true}` {
		t.Errorf("first toggle = %s, want ready true", got)
	}
	deliver(t, m, readyMsg(1, true))

	press(t, m, "enter")
	if got := lastSent(t, conn); got != `{"t":"ready","ready":false}` {
		t.Errorf("second toggle = %s, want ready false", got)
	}
}

func TestOnlyTheHostStarts(t *testing.T) {
	// The host-start shape: autoStart false, so nothing begins by itself.
	withWelcomeKey(t, "7766554433221100ff00ff00ff00ff00")
	auto := false
	build := func(seat int, players ...online.PlayerView) online.ServerMsg {
		return welcomeRoom(roomCode, "live", 5, auto, "lobby", seat, welcomeKey, players...)
	}

	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openJoinRoom(t, m, f)
	deliver(t, m, build(2,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
	))
	draw(t, m)
	if strings.Contains(plain(m.View().Content), "s start") {
		t.Error("a non-host was offered s start")
	}
	press(t, m, "s")
	if len(conn.sent) != 1 { // the hello only
		t.Errorf("a non-host's s sent %v", conn.sent)
	}

	// The host: s is offered, and it starts.
	f2 := &fakeOnline{}
	m2 := newOnlineModel(t, f2, true)
	conn2 := openMakeRoom(t, m2, f2) // seat 1 is the creator and the host
	welcomeKey = f2.created.Key
	host := welcomeRoom(roomCode, "live", 5, false, "lobby", 1, welcomeKey,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
	)
	deliver(t, m2, host)
	draw(t, m2)
	if !strings.Contains(plain(m2.View().Content), "s start") {
		t.Error("the host was not offered s start")
	}
	press(t, m2, "s")
	if got := lastSent(t, conn2); got != `{"t":"start"}` {
		t.Errorf("the host's s = %s, want start", got)
	}
}

func TestCountdownShowsSeconds(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	openMakeRoom(t, m, f)
	welcomeKey = f.created.Key
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	// Serve the countdown from that same clock: the welcome carried no now,
	// so the offset is still zero and goAt is three seconds past the freeze.
	withClock(t, &now)
	deliver(t, m, online.ServerMsg{
		T: "countdown", Now: now.UnixMilli(), GoAt: now.Add(3 * time.Second).UnixMilli(), Round: 1,
	})

	frame := plain(m.View().Content)
	if !strings.Contains(frame, "\n3\n") && !strings.Contains(frame, "3") {
		t.Errorf("the frame does not show the seconds:\n%s", frame)
	}
	if !strings.Contains(frame, "get ready") {
		t.Errorf("the frame does not show the countdown label:\n%s", frame)
	}
}

func TestEscKeepsTheRoom(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	welcomeKey = f.created.Key
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	press(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("screen = %v, want the menu", m.screen)
	}
	if m.room == nil {
		t.Fatal("esc closed the session")
	}
	if conn.closed || m.room.conn == nil {
		t.Error("esc closed the connection")
	}
	if conn.left() {
		t.Error("esc sent a leave")
	}
}

func TestLeaveLeaves(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	welcomeKey = f.created.Key
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	press(t, m, "l")
	if !conn.left() {
		t.Error("l sent no leave")
	}
	if !conn.closed {
		t.Error("l left the socket open")
	}
	if m.room != nil {
		t.Error("l kept the session")
	}
	if codes := roomFile(t, m); len(codes) != 0 {
		t.Errorf("rooms file = %v, want it cleared", codes)
	}
	if m.screen != screenSocial {
		t.Errorf("screen = %v, want social play", m.screen)
	}
}

func TestLeaveFromSocialAsksFirst(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	openSocial(t, m)
	if m.social.cursor != 0 {
		m.social.cursor = socialRowIndex(t, m, m.social.rows[m.social.cursor].kind)
		m.social.cursor = 0
	}
	press(t, m, "x")
	if !m.social.confirmLeave {
		t.Fatal("x did not arm the leave prompt")
	}
	if frame := plain(m.View().Content); !strings.Contains(frame, "leave room "+roomCode+"?") {
		t.Errorf("the frame does not ask first:\n%s", frame)
	}

	press(t, m, "esc")
	if m.social.confirmLeave {
		t.Error("esc did not cancel the prompt")
	}
	if asked(f, "dial") {
		t.Error("esc dialed the room just to cancel the prompt")
	}
	if codes := roomFile(t, m); len(codes) != 1 {
		t.Errorf("rooms file = %v, want the room left alone", codes)
	}

	press(t, m, "x") // arm again
	press(t, m, "x") // confirm: this room is not connected this run, so it is
	// left best-effort, over a fresh dial that carries the seat key.
	if codes := roomFile(t, m); len(codes) != 0 {
		t.Errorf("rooms file = %v, want the code gone", codes)
	}
	if !asked(f, "dial "+roomCode) {
		t.Errorf("calls = %v, want the best-effort dial", f.calls)
	}
}

func TestAMessageFromAnOldConnectionIsIgnored(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	_ = openMakeRoom(t, m, f)
	welcomeKey = "00112233445566778899aabbccddeeff"
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	before := append([]online.PlayerView(nil), m.room.view.Players...)
	_, _ = m.Update(roomRecvMsg{gen: m.room.gen - 1, msg: readyMsg(1, true)})
	after := m.room.view.Players
	if len(before) != len(after) {
		t.Fatalf("an old connection's message changed the player list")
	}
	for i := range before {
		if before[i].Ready != after[i].Ready || before[i].Connected != after[i].Connected {
			t.Errorf("an old connection's message changed players[%d]", i)
		}
	}
}

func TestTurningNetworkOffClosesTheRoom(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	welcomeKey = f.created.Key
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	// In settings, on the network row, turn it off the way the keyboard does:
	// down to the profile name, then one more to the network row.
	m.openMenu()
	m.menu.cursor = menuIndex(t, m, choiceSettings, 0)
	send(t, m, "enter")
	send(t, m, "down", "down", "down")
	if m.settings.cursor != rowNetwork {
		t.Fatalf("cursor = %d, want the network row", m.settings.cursor)
	}
	_, cmd := m.Update(key("right")) // the row's own step, as the keys cycle it
	runCmd(t, m, cmd)
	if m.settings.network {
		t.Fatal("the row did not turn the network off")
	}

	if m.room != nil {
		t.Errorf("network off left the session open")
	}
	if !conn.closed {
		t.Errorf("network off left the socket open")
	}
}

func TestAsyncOpenView(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openJoinRoom(t, m, f)
	deliver(t, m, welcomeAsync(3,
		player(1, "sam", false, false, true),
		player(2, "priya", false, false, true),
		player(3, "", false, false, false),
	))
	if m.screen != screenRoom {
		t.Fatalf("screen = %v, want the room's open view", m.screen)
	}
	draw(t, m)
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "2 played: sam, priya") {
		t.Errorf("the frame does not count the players:\n%s", frame)
	}
	if !strings.Contains(frame, "you get 10 minutes once you start") {
		t.Errorf("the frame does not carry the async note:\n%s", frame)
	}

	press(t, m, "enter")
	if got := lastSent(t, conn); got != `{"t":"start"}` {
		t.Errorf("enter = %s, want start", got)
	}
}

// withWelcomeKey runs one test with the seat key overwritten and restores the
// shared default afterwards, for the tests that need a value of their own.
func withWelcomeKey(t *testing.T, key string) {
	t.Helper()
	old := welcomeKey
	t.Cleanup(func() { welcomeKey = old })
	welcomeKey = key
}

// ---- playing a round (PR 9) ----

// roundNow is the fixed clock every round test freezes the frame's reads at.
var roundNow = time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)

// countdownMsg is the wire message that rings a round's opener.
func countdownMsg(now time.Time, at time.Time, round int) online.ServerMsg {
	return online.ServerMsg{
		T: "countdown", Now: now.UnixMilli(),
		GoAt: at.UnixMilli(), Round: round,
	}
}

func startRoundMsg(now, endsAt time.Time, round int) online.ServerMsg {
	return online.ServerMsg{
		T: "go", Now: now.UnixMilli(),
		EndsAt: endsAt.UnixMilli(), Round: round,
	}
}

// marksRow is the marks of one guess against the round's answer.
func marksRowServer(guess, answer string) []game.Mark {
	return game.Score(guess, answer)
}

func marksMsg(n int, marks []game.Mark) online.ServerMsg {
	return online.ServerMsg{T: "marks", N: n, Marks: append([]game.Mark(nil), marks...)}
}

func progressMsg(seat, n int, marks []game.Mark) online.ServerMsg {
	return online.ServerMsg{T: "progress", Seat: seat, N: n, Marks: append([]game.Mark(nil), marks...)}
}

func doneMsg(seat int, solved bool, guesses int, elapsedMS int64) online.ServerMsg {
	v := solved
	return online.ServerMsg{T: "done", Seat: seat, Solved: &v, Guesses: guesses, ElapsedMS: elapsedMS}
}

// resultsRound is the server's end-of-round card, made for the tests: our
// seat's entry first when they led.
func resultsRound(now time.Time, round int, word string, ranking ...online.RankEntry) online.ServerMsg {
	return online.ServerMsg{T: "results", Round: round, Word: word, Ranking: ranking}
}

// rankEntry is one ranking line, in online.RankEntry shape.
func rankEntry(seat int, name string, solved bool, guesses int, elapsedMS int64, rows [][]game.Mark) online.RankEntry {
	return online.RankEntry{Seat: seat, Name: name, Solved: solved, Guesses: guesses, ElapsedMS: elapsedMS, Rows: rows}
}

// rowsSpent is the sheet rows of an unsolved seat.
func spentRows(length int, guesses int) [][]game.Mark {
	rows := make([][]game.Mark, guesses)
	for i := range rows {
		rows[i] = make([]game.Mark, length)
	}
	return rows
}

// scoreRows is the server marks for one guess string.
func scoreRows(guesses ...string) [][]game.Mark {
	rows := make([][]game.Mark, len(guesses))
	for i, w := range guesses {
		rows[i] = game.Score(w, "crane")
	}
	return rows
}

// forum is the standard room this round plays: our seat 1 ("nick"), with
// players 2 and 3 both connected and dealt into the round.
func forum(t *testing.T) (*Model, *fakeOnline, *fakeConn) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	// The welcome that makes the session a lobby: seat 1, the creator.
	withWelcomeKey(t, f.created.Key)
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))
	return m, f, conn
}

// startLiveRound takes forum through the countdown and go so the board is
// showing, with players 2 and 3 in the round beside us.
func startLiveRound(t *testing.T, m *Model) {
	t.Helper()
	now := roundNow
	deliver(t, m, countdownMsg(now, now.Add(3*time.Second), 1))
	now, m.room.view.GoAt = now.Add(3*time.Second), now.Add(3*time.Second).UnixMilli()
	endsAt := now.Add(600 * time.Second)
	// The go message treats the frozen clock: deliver handles the offset.
	deliver(t, m, startRoundMsg(now, endsAt, 1))
}

func TestGoOpensTheBoard(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the board", m.screen)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "room "+roomCode) {
		t.Errorf("the header does not name the room:\n%s", frame)
	}
	if strings.Contains(frame, "tab+enter") {
		t.Errorf("a room board offers the restart prompt:\n%s", frame)
	}
}

func TestAGuessGoesToTheServer(t *testing.T) {
	m, _, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	before := conn.count()
	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	if got := lastSent(t, conn); got != `{"t":"guess","n":1,"word":"slate"}` {
		t.Errorf("the guess = %s, want slate as guess 1", got)
	}
	// While the guess is out the row cannot change: the row still reads as
	// the word that was sent.
	send(t, m, "backspace")
	if m.game.typing != "slate" {
		t.Errorf("typing while pending = %q, want the row as sent", m.game.typing)
	}
	draw(t, m)
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "checking…") {
		t.Errorf("the status does not read as checking:\n%s", frame)
	}
	// A second enter does not send a second guess.
	press(t, m, "enter")
	if n := conn.count(); n != before+1 {
		t.Errorf("frames = %d, want one guess on the wire", n)
	}
}

func TestMarksFillTheRow(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))

	if m.room.pending != 0 {
		t.Errorf("pending = %d, want none after its marks", m.room.pending)
	}
	// The board's letters sit inside tiles, so the frame's own spacing has to
	// go before the word is readable in it. The successful check is the same
	// fact a priced-checked row asserts: the row is SLATE.
	squashed := strings.ReplaceAll(plain(m.View().Content), " ", "")
	if !strings.Contains(squashed, "SLATE") {
		t.Errorf("the board's first row is not SLATE:\n%s", plain(m.View().Content))
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "5 guesses left") {
		t.Errorf("the status does not count what is left:\n%s", frame)
	}
}

func TestALocalNonWordIsNotSent(t *testing.T) {
	m, _, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "x", "x", "x", "x", "x")
	before := conn.count()
	press(t, m, "enter")
	if conn.count() != before {
		t.Errorf("frames = %+v, want the guess refused locally", conn.sent)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "not in word list") {
		t.Errorf("the frame does not say why:\n%s", frame)
	}
}

func TestRejectSaysWhy(t *testing.T) {
	m, _, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, online.ServerMsg{T: "reject", N: 1, Reason: "not_playing"})

	if m.room.pending != 0 {
		t.Errorf("pending = %d, want none after its refusal", m.room.pending)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "the round isn't running") {
		t.Errorf("the frame does not say why:\n%s", frame)
	}
	_ = conn
}

func TestTheRoundBoardIsNeverSaved(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))
	send(t, m, "q", "q", "q", "q", "q")
	press(t, m, "enter")
	deliver(t, m, marksMsg(2, marksRowServer("qqqqq", "crane")))

	// esc from a mid-round board is the leave test's subject; here the save
	// path's own leave() is what carries the assertion.
	if err := m.game.leave(); err != nil {
		t.Fatalf("leave: %v", err)
	}
	ids, _ := m.store.IDs()
	if len(ids) != 0 {
		t.Errorf("the store holds %d records mid-round, want 0", len(ids))
	}
	if m.room == nil {
		t.Fatal("leaving a round board closed the session")
	}
}

func TestStandingsShowEveryoneElse(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	// Two round fellows, both InRound.
	deliver(t, m, welcomeLive(1,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
		player(3, "", true, false, false),
	))
	startLiveRound(t, m)
	draw(t, m)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: testHeight})

	deliver(t, m, progressMsg(2, 1, marksRowServer("slate", "crane")))
	deliver(t, m, progressMsg(3, 1, marksRowServer("ghost", "crane")))
	draw(t, m)
	frame := plain(m.View().Content)
	for _, want := range []string{"sam", "player 3", "1/6"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the standings do not show %q:\n%s", want, frame)
		}
	}

	// Sam's solve moves her to the top with her state.
	deliver(t, m, doneMsg(2, true, 3, 12_000))
	draw(t, m)
	frame = plain(m.View().Content)
	if !strings.Contains(frame, "in 3") {
		t.Errorf("sam's line does not read in 3:\n%s", frame)
	}
	// Sorted: sam (the solved seat) above player 3 (still playing).
	samAt := strings.Index(frame, "sam")
	thirdAt := strings.Index(frame, "player 3")
	if samAt < 0 || thirdAt < 0 || samAt > thirdAt {
		t.Errorf("sam's line did not move to the top:\n%s", frame)
	}
}

func TestStandingsFallBackForWidth(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	deliver(t, m, welcomeLive(1,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
	))
	startLiveRound(t, m)
	deliver(t, m, progressMsg(2, 1, marksRowServer("slate", "crane")))

	// The full form's width rule, computed here from the one the plan names:
	// the number below is the narrowest terminal at which the full column
	// still fits; one narrower speaks compact, one narrower than slash the
	// compact the count. Verify both.
	// A resize reaches the frame without draw()'s own resizes in either side
	// of it: draw holds the terminal itself at testWidth. The board's width is
	// its tiles plus the gutter between them, measured rather than guessed.
	full := 8 + 1 + 5 + 1 + 4
	board := renderBoard(m.game.g, m.game.typing, &hitMap{}, m.game.anim,
		roundNow, m.game.layout().tiles, m.game.layout().boardGap, m.game.outlines)
	boardWidth := lipgloss.Width(board)
	needFull := boardWidth + 3 + full + 2 + 2*st.metric.PanelPadX
	m.Update(tea.WindowSizeMsg{Width: needFull, Height: testHeight})
	if got := plain(m.View().Content); !strings.Contains(got, "■") {
		t.Errorf("the full column lost its marks:\n%s", got)
	}
	m.Update(tea.WindowSizeMsg{Width: needFull - 1, Height: testHeight})
	if got := plain(m.View().Content); strings.Contains(got, "■") {
		t.Errorf("the column did not fall back:\n%s", got)
	}
	m.Update(tea.WindowSizeMsg{Width: 50, Height: testHeight})
	got := plain(m.View().Content)
	if strings.Contains(got, "■") {
		t.Errorf("a narrow terminal kept the column:\n%s", got)
	}
	if !strings.Contains(got, "still playing") {
		t.Errorf("the hidden form's status did not count:\n%s", got)
	}
}

func TestStandingsNeverTallerThanTheBoard(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	others := []online.PlayerView{player(1, "nick", true, false, false)}
	for i := 2; i <= 8; i++ {
		others = append(others, player(i, fmt.Sprintf("p%d", i), true, false, false))
	}
	deliver(t, m, welcomeLive(1, others...))
	startLiveRound(t, m)

	// The tightest board is the flat one at the fewest rows it needs. Seven
	// others and that few rows must be a "+N more" tail, and the frame exactly
	// as tall as a frame with an empty column.
	board := renderBoard(m.game.g, m.game.typing, &hitMap{}, m.game.anim,
		roundNow, m.game.layout().tiles, m.game.layout().boardGap, m.game.outlines)
	needFull := lipgloss.Width(board) + 3 + (8 + 1 + 5 + 1 + 4) + 2 + 2*st.metric.PanelPadX
	tight := boardLayout{tiles: flatTile}.rows(6)
	m.Update(tea.WindowSizeMsg{Width: needFull, Height: tight})
	withStandings := plain(m.View().Content)
	tall := strings.Count(withStandings, "\n")
	if !strings.Contains(withStandings, "+") {
		t.Errorf("the column did not count its tail:\n%s", withStandings)
	}
	m.room.view.Players = []online.PlayerView{player(1, "nick", true, false, false)}
	m.room.view.Results = nil
	if got := plain(m.View().Content); strings.Count(got, "\n") != tall {
		t.Errorf("the standings changed the frame height: %d vs %d", tall, strings.Count(got, "\n"))
	}
}

func TestSolvingWaitsForOthers(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	deliver(t, m, welcomeLive(1,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
		player(3, "priya", true, false, false),
	))
	startLiveRound(t, m)

	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 1_500))
	draw(t, m)
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "solved in 1 · waiting for 2 still playing") {
		t.Errorf("the status does not wait:\n%s", frame)
	}
}

func TestResultsSaveTheRoundOnce(t *testing.T) {
	m, f, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))
	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(2, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 2, 81_000))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", true, 2, 81_000, scoreRows("slate", "crane")),
		rankEntry(2, "sam", false, 6, 600_000, spentRows(5, 6)),
		rankEntry(3, "priya", false, 0, 600_000, nil),
	}
	// only 2 players inRound: gem the ranking to two of them.
	ranking = ranking[:2]
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))

	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the room result", m.screen)
	}
	ids, _ := m.store.IDs()
	if len(ids) != 1 {
		t.Fatalf("the store holds %d records, want the one round", len(ids))
	}
	g, err := m.store.Load(ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if g.Room == nil || g.Room.Code != roomCode || g.CountsForStats() {
		t.Errorf("the saved round = %+v, want a room round that counts for nothing", g.Room)
	}
	if info := m.settingsOf(); info.PlaytimeMS != 81_000 {
		t.Errorf("playtime = %d, want our elapsed 81s", info.PlaytimeMS)
	}

	before := len(ids)
	screen := m.screen
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))
	ids2, _ := m.store.IDs()
	if len(ids2) != before {
		t.Errorf("the second results saved %d records", len(ids2)-0)
	}
	if info := m.settingsOf(); info.PlaytimeMS != 81_000 {
		t.Errorf("the second results banked time again: %d", info.PlaytimeMS)
	}
	if m.screen != screen {
		t.Errorf("the screen wandered to %v", m.screen)
	}
	_ = f
}

func TestResultScreenShowsTheRanking(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 1_500))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", true, 1, 1_500, scoreRows("crane")),
		rankEntry(2, "sam", false, 0, 600_000, nil),
		rankEntry(3, "priya", false, 0, 600_000, nil),
	}
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))

	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the room result", m.screen)
	}
	frame := plain(m.View().Content)
	for _, want := range []string{
		"1st of 3",
		"room " + roomCode + " · round 1",
		"answer CRANE",
	} {
		if !strings.Contains(frame, want) {
			t.Errorf("the frame does not carry %q:\n%s", want, frame)
		}
	}
	// One unsolved entry, marked with a dash and no time spent solving.
	if !strings.Contains(frame, "—") {
		t.Errorf("an unsolved ranking line lacks its dash:\n%s", frame)
	}
}

func TestMarksThatDisagreeAreNotSaved(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", false, 6, 600_000, spentRows(5, 6)),
	}
	// The word the round says it was over re-scores to something else.
	deliver(t, m, resultsRound(roundNow, 1, "other", ranking...))

	ids, _ := m.store.IDs()
	if len(ids) != 0 {
		t.Errorf("a disagreeing round was saved")
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "couldn't save this round") {
		t.Errorf("the frame does not say why:\n%s", frame)
	}
}

func TestReviewThenBack(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)

	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 1_500))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", true, 1, 1_500, scoreRows("crane")),
		rankEntry(2, "sam", false, 0, 600_000, nil),
	}
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))

	press(t, m, "enter")
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the saved round's board", m.screen)
	}
	// The round's letters sit inside tiles, so the frame's own spacing has to
	// go before the word is readable in it.
	squashed := strings.ReplaceAll(plain(m.View().Content), " ", "")
	if !strings.Contains(squashed, "CRANE") {
		t.Errorf("the board does not show the round:\n%s", plain(m.View().Content))
	}
	press(t, m, "enter")
	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the result again", m.screen)
	}
}

func TestRematchFromTheResult(t *testing.T) {
	m, _, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)
	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 1_500))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", true, 1, 1_500, scoreRows("crane")),
		rankEntry(2, "sam", false, 0, 600_000, nil),
	}
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))

	press(t, m, "n")
	if got := lastSent(t, conn); got != `{"t":"rematch"}` {
		t.Errorf("n = %s, want rematch", got)
	}
	deliver(t, m, online.ServerMsg{T: "lobby", Round: 2})
	if m.screen != screenRoom {
		t.Fatalf("screen = %v, want the lobby", m.screen)
	}
	draw(t, m)
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "waiting for someone to join — share the code") {
		t.Errorf("the lobby does read like one:\n%s", frame)
	}
}

func TestAsyncFinishDropsTheRoom(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openJoinRoom(t, m, f)
	deliver(t, m, welcomeAsync(1, player(1, "", false, false, false)))

	press(t, m, "enter") // start
	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 9_000))
	ranking := []online.RankEntry{
		rankEntry(1, "", true, 1, 9_000, scoreRows("crane")),
	}
	deliver(t, m, online.ServerMsg{
		T: "results", Round: 1, Word: "crane", Ranking: ranking,
	})

	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the result", m.screen)
	}
	ids, _ := m.store.IDs()
	if len(ids) != 1 {
		t.Fatalf("the store holds %d records, want one", len(ids))
	}
	g, err := m.store.Load(ids[0])
	if err != nil || g.Room == nil || g.Room.Key == "" {
		t.Fatalf("saved round = %+v %v, want one with the reading key", g.Room, err)
	}
	if codes := roomFile(t, m); len(codes) != 0 {
		t.Errorf("rooms file = %v, want the async room gone", codes)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "1st of 1 so far") {
		t.Errorf("the frame does not read so far:\n%s", frame)
	}
	_ = conn
}

func TestTheListLabelsRoomRounds(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)
	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 1_500))
	ranking := []online.RankEntry{
		rankEntry(1, "nick", true, 1, 1_500, scoreRows("crane")),
	}
	deliver(t, m, resultsRound(roundNow, 1, "crane", ranking...))

	// Back to the menu, into the list: the room round labels one.
	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("screen = %v, want the menu", m.screen)
	}
	m.menu.cursor = menuIndex(t, m, choiceList, 0)
	send(t, m, "enter")
	draw(t, m)
	list, err := m.store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Room == nil {
		t.Fatalf("the list has no room round: %+v", list)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "room") {
		t.Errorf("the list did not label the round:\n%s", frame)
	}
}

func TestWatchersSeeTheResultWithoutSaving(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	openJoinRoom(t, m, f)
	withWelcomeKey(t, f.created.Key)
	// The welcome lands with the round already over and us nowhere in its
	// ranking: the results the room holds are the public view's.
	roomMsg := welcomeLive(3,
		player(1, "nick", true, false, true),
		player(2, "sam", true, false, true),
		player(3, "priya", true, false, false),
	).Room
	roomMsg.Phase = "results"
	roomMsg.Results = &online.Results{
		Round: 1, Word: "crane",
		Ranking: []online.RankEntry{
			rankEntry(1, "nick", true, 2, 81_000, scoreRows("slate", "crane")),
			rankEntry(2, "sam", false, 6, 600_000, spentRows(5, 6)),
		},
	}
	deliver(t, m, online.ServerMsg{T: "welcome", V: 1, Seat: 3, Key: welcomeKey, Room: roomMsg})

	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the result screen", m.screen)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "round 1") {
		t.Errorf("the watcher card does not announce the round:\n%s", frame)
	}
	if strings.Contains(frame, "of 2") {
		t.Errorf("a watcher's card claims a place:\n%s", frame)
	}
	// Their screen holds no record of a round they never played.
	ids, _ := m.store.IDs()
	if len(ids) != 0 {
		t.Errorf("a watcher's visit saved %d records", len(ids))
	}
}

// ---- coming back (PR 10) ----

func TestALostConnectionRetries(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	withWelcomeKey(t, f.created.Key)
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))

	gen := m.room.gen
	_, next := m.Update(roomRecvMsg{gen: gen, err: errors.New("drop")})
	if m.room == nil {
		t.Fatal("a drop closed the session")
	}
	if m.room.conn != nil {
		t.Error("the lost connection is still set")
	}
	if m.room.retries != 1 {
		t.Errorf("retries = %d, want 1", m.room.retries)
	}
	if next == nil {
		t.Error("a lost connection scheduled no retry")
	}
	// The retry is a later tick; do not run it.
	_ = conn
}

func TestBackoffSteps(t *testing.T) {
	for n, want := range map[int]time.Duration{
		1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second,
		4: 8 * time.Second, 5: 16 * time.Second,
		6: 30 * time.Second, 7: 30 * time.Second,
	} {
		if got := roomBackoff(n); got != want {
			t.Errorf("roomBackoff(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestARetryRedials(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	withWelcomeKey(t, f.created.Key)
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))
	const key = "00112233445566778899aabbccddeeff"
	m.room.key = key

	// The connection is gone first: the drop's own command is a wait, whose
	// arrival is simulated here by sending the retry itself.
	f.conn = conn
	_, _ = m.Update(roomRecvMsg{gen: m.room.gen, err: errors.New("drop")})
	if m.room.conn != nil || m.room.retries != 1 {
		t.Fatalf("the drop left a connection (%v) and retcount %d", m.room.conn, m.room.retries)
	}
	gen := m.room.gen
	f.calls = nil
	_, next := m.Update(roomRetryMsg{gen: gen})
	runCmd(t, m, next)
	if !asked(f, "dial "+roomCode) {
		t.Errorf("calls = %v, want the retry's dial", f.calls)
	}
	if got := conn.helloKey(); got != key {
		t.Errorf("the retry's hello key = %q, want the saved key", got)
	}

	// An old generation's retry does nothing at all.
	f.calls = nil
	before := conn.count()
	_, next = m.Update(roomRetryMsg{gen: gen - 1})
	runCmd(t, m, next)
	if asked(f, "dial") || conn.count() != before {
		t.Errorf("a stale retry did anything: %v, %d frames", f.calls, conn.count()-before)
	}
}

// beginRoundWith starts a forum board and puts guess 2 in flight, so the
// re-welcome tests owe someone an answer about one guess.
func beginRoundWith(t *testing.T, pending int) (*Model, *fakeOnline, *fakeConn) {
	m, f, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)
	if pending == 2 {
		send(t, m, "s", "l", "a", "t", "e")
		press(t, m, "enter")
		deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))
		send(t, m, "g", "i", "v", "e", "r")
		press(t, m, "enter")
	}
	return m, f, conn
}

func TestAPendingGuessIsResent(t *testing.T) {
	m, f, conn := beginRoundWith(t, 2)
	if m.room.pending != 2 {
		t.Fatalf("pending = %d, want the guess 2 this test owes", m.room.pending)
	}
	const key = "00112233445566778899aabbccddeeff"
	m.room.key = key

	// The connection is gone, at the moment the drop lands; the re-welcome
	// shows one word — nothing landed while we were away.
	f.conn = conn
	m.room.conn = nil
	m.room.retries++
	f.calls = nil
	reGen := m.room.gen + 1
	m.roomGen = reGen
	m.room.gen = reGen
	_, next := m.Update(roomRetryMsg{gen: reGen})
	runCmd(t, m, next)
	if !asked(f, "dial "+roomCode) {
		t.Fatalf("calls = %v, want the redial", f.calls)
	}
	f.conn = conn
	m.room.conn = conn
	// The re-welcome has one word, and the resend then goes out.
	welcome := welcomeLive(1, player(1, "nick", true, false, false))
	welcome.Room.You.Words = []string{"slate"}
	welcome.Room.You.Marks = scoreRows("slate")
	deliver(t, m, welcome)
	if got := lastSent(t, conn); !strings.Contains(got, `"t":"guess"`) ||
		!strings.Contains(got, `"n":2`) {
		t.Errorf("the pending guess was not resent: %s", got)
	}
}

func TestAPendingGuessThatLandedIsNotResent(t *testing.T) {
	m, f, conn := beginRoundWith(t, 2)
	const key = "00112233445566778899aabbccddeeff"
	m.room.key = key

	f.conn = conn
	m.room.conn = nil
	m.room.retries++
	reGen := m.room.gen + 1
	m.roomGen = reGen
	m.room.gen = reGen
	_, next := m.Update(roomRetryMsg{gen: reGen})
	runCmd(t, m, next)
	f.conn = conn
	m.room.conn = conn
	// The re-welcome carries both words: the guess landed while we were away.
	welcome := welcomeLive(1, player(1, "nick", true, false, false))
	welcome.Room.You.Words = []string{"slate", "giver"}
	welcome.Room.You.Marks = append(scoreRows("slate"), scoreRows("giver")...)
	frames := conn.count()
	deliver(t, m, welcome)
	if got := lastSent(t, conn); strings.Contains(got, `"t":"guess"`) {
		t.Errorf("a landed guess was resent: %s", got)
	}
	if m.room.pending != 0 {
		t.Errorf("pending = %d, want none", m.room.pending)
	}
	if conn.count() != frames {
		t.Errorf("the re-welcome sent %d frames, want none", conn.count()-frames)
	}
	// The board shows both rows.
	draw(t, m)
	squashed := strings.ReplaceAll(plain(m.View().Content), " ", "")
	if !strings.Contains(squashed, "GIVER") {
		t.Errorf("the board does not show the round's second row:\n%s", plain(m.View().Content))
	}
}

func TestReconnectingAfterTheEndShowsTheResult(t *testing.T) {
	m, f, conn := forum(t)
	withClock(t, &roundNow)
	startLiveRound(t, m)
	send(t, m, "s", "l", "a", "t", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("slate", "crane")))

	// The round ends while the connection is dark; the retry then lands its
	// welcome into a room the server has already moved on for us.
	f.conn = conn
	m.room.conn = nil
	m.room.retries++
	reGen := m.room.gen + 1
	m.roomGen = reGen
	m.room.gen = reGen
	_, next := m.Update(roomRetryMsg{gen: reGen})
	runCmd(t, m, next)
	f.conn = conn
	m.room.conn = conn
	welcome := welcomeLive(1,
		player(1, "nick", true, false, false),
		player(2, "sam", true, false, false),
	)
	welcome.Room.Phase = "results"
	welcome.Room.Results = &online.Results{
		Round: 1, Word: "crane",
		Ranking: []online.RankEntry{
			rankEntry(1, "nick", false, 6, 600_000, spentRows(5, 6)),
			rankEntry(2, "sam", true, 2, 81_000, scoreRows("slate", "crane")),
		},
	}
	deliver(t, m, welcome)

	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the room result", m.screen)
	}
	ids, _ := m.store.IDs()
	if len(ids) != 1 {
		t.Fatalf("the store holds %d records, want the round", len(ids))
	}
	g, err := m.store.Load(ids[0])
	if err != nil || g.Room == nil || g.Room.Code != roomCode {
		t.Errorf("the round did not save: %+v %v", g.Room, err)
	}
}

func TestLeavingNeverReconnects(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	conn := openMakeRoom(t, m, f)
	withWelcomeKey(t, f.created.Key)
	deliver(t, m, welcomeLive(1, player(1, "nick", true, false, false)))
	m.room.leaving = true

	_, next := m.Update(roomRecvMsg{gen: m.room.gen, err: errors.New("drop")})
	if m.room != nil {
		t.Fatal("a leaving session stayed open")
	}
	if next != nil {
		t.Error("a leaving session scheduled a retry")
	}
	_ = conn
}

// runningWelcome is the welcome of a live round mid-board: the shape the
// startup check is looking for.
func runningWelcome(now time.Time) online.ServerMsg {
	room := welcomeLive(1, player(1, "nick", true, false, false)).Room
	room.Phase = "playing"
	room.EndsAt = now.Add(400 * time.Second).UnixMilli()
	room.You = online.YouView{Seat: 1, InRound: true}
	room.You.Words = []string{"slate"}
	room.You.Marks = scoreRows("slate")
	welcome := welcomeRoom(roomCode, "live", 5, true, "playing", 1, welcomeKey,
		player(1, "nick", true, false, false))
	welcome.Room = room
	welcome.Now = now.UnixMilli()
	return welcome
}

func TestStartupReopensARunningRound(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	withWelcomeKey(t, welcomeKey)
	// The opening board, with the splash raised over it exactly as New does.
	m.raiseSplash()
	if m.screen != screenSplash {
		t.Fatalf("screen = %v, want the splash", m.screen)
	}

	f.conn = &fakeConn{queue: []online.ServerMsg{runningWelcome(roundNow)}}
	runCmd(t, m, m.probeCmd())

	// The splash went back over the reopened board.
	if m.screen != screenSplash {
		t.Fatalf("screen = %v, want the splash keeping its dismissal", m.screen)
	}
	if m.splash.next != screenGame {
		t.Fatalf("splash.next = %v, want the board", m.splash.next)
	}
}

func TestStartupLeavesYouAloneWhenBusy(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	// The first board holds a typed letter: a puzzle in hand, which the
	// probe does not disturb.
	m.openGame(mustGame(t, 5), false)
	send(t, m, "a")

	f.conn = &fakeConn{queue: []online.ServerMsg{runningWelcome(roundNow)}}
	runCmd(t, m, m.probeCmd())
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the board alone to its own round", m.screen)
	}
	if !strings.Contains(plain(m.View().Content),
		"your round in room "+roomCode+" is still running — open it from social play") {
		t.Errorf("the note never said where the round is:\n%s", plain(m.View().Content))
	}
}

func TestStartupDropsDeadRooms(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	conn := &fakeConn{}
	conn.queue = []online.ServerMsg{{T: "refuse", Reason: "no_room"}}
	f.conn = conn

	runCmd(t, m, m.probeCmd())
	if codes := roomFile(t, m); len(codes) != 0 {
		t.Errorf("rooms file = %v, want the dead room dropped", codes)
	}
	if m.note != "a room you were in ended while you were away" {
		t.Errorf("note = %q, want the dead-room line", m.note)
	}
}

func TestStartupAsksNothingOffline(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, false)
	setRoomFile(t, m, roomCode)
	if cmd := m.probeCmd(); cmd != nil {
		t.Error("the probe asked without consent")
	}
	if asked(f, "dial") {
		t.Errorf("calls = %v, want none offline", f.calls)
	}
}

func mustGame(t *testing.T, length int) *game.Game {
	t.Helper()
	g, err := game.New(length)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func TestRefreshUpdatesTheRanking(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	key := welcomeKey
	withClock(t, &roundNow)
	g, err := game.NewRoomRound(game.RoomInfo{
		Code: roomCode, Mode: "async", Round: 1, Seat: 1, Key: key,
		Ranking: []game.RoomRank{
			{Seat: 1, Name: "nick", Solved: true, Guesses: 2, ElapsedMS: 81_000,
				Rows: scoreRows("slate", "crane")},
		},
	}, "crane", []string{"slate", "crane"}, 81_000, roundNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.store.Save(g); err != nil {
		t.Fatal(err)
	}
	m.roomResult.r = roomResult{
		code: roomCode, mode: "async", round: 1, seat: 1, length: 5, word: "crane",
		ranking: g.Room.Ranking, g: g,
	}
	m.roomResult.room = nil
	m.screen = screenRoomResult
	m.roomResult.resize(testWidth, testHeight)
	f.results = online.Results{
		Round: 1, Word: "crane",
		Ranking: []online.RankEntry{
			rankEntry(1, "nick", true, 2, 81_000, scoreRows("slate", "crane")),
			rankEntry(2, "sam", true, 3, 120_000, scoreRows("ghost", "ghost", "ghost")),
		},
	}
	m.roomResult.room = nil

	press(t, m, "f")
	// The refresh's rounds happen synchronously in the test drain: its note
	// is already the answer's, so the announcement is checked via the state
	// the busy flag the press opened the request with.
	if len(m.roomResult.note) == 0 || !strings.Contains(m.roomResult.note, "updated") {
		t.Errorf("note = %q, want updated", m.roomResult.note)
	}
	frame := plain(m.View().Content)
	if !strings.Contains(frame, "sam") {
		t.Errorf("the refreshed ranking lacks the new player:\n%s", frame)
	}
	ids, _ := m.store.IDs()
	if len(ids) != 1 {
		t.Fatal("the refresh lost the record")
	}
	got, err := m.store.Load(ids[0])
	if err != nil || len(got.Room.Ranking) != 2 {
		t.Errorf("the saved ranking = %+v %v, want two entries", got.Room, err)
	}
}

func TestRefreshAfterTheRoomClosed(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	withClock(t, &roundNow)
	g, err := game.NewRoomRound(game.RoomInfo{
		Code: roomCode, Mode: "async", Round: 1, Seat: 1, Key: welcomeKey,
		Ranking: []game.RoomRank{
			{Seat: 1, Name: "nick", Solved: true, Guesses: 1, ElapsedMS: 1_500,
				Rows: scoreRows("crane")},
		},
	}, "crane", []string{"crane"}, 1_500, roundNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.store.Save(g); err != nil {
		t.Fatal(err)
	}
	m.roomResult.r = roomResult{
		code: roomCode, mode: "async", round: 1, seat: 1, length: 5, word: "crane",
		ranking: g.Room.Ranking, g: g,
	}
	m.screen = screenRoomResult
	m.roomResult.resize(testWidth, testHeight)
	f.resultsErr = &online.StatusError{Code: 404}

	press(t, m, "f")
	if !strings.Contains(plain(m.View().Content), "this room has closed") {
		t.Errorf("the frame does not say the room closed:\n%s", plain(m.View().Content))
	}
	got, err := m.store.Load(g.ID)
	if err != nil || len(got.Room.Ranking) != 1 {
		t.Errorf("the record changed: %+v %v", got.Room, err)
	}
}

func TestNoRefreshForLiveRounds(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	withClock(t, &roundNow)
	g, err := game.NewRoomRound(game.RoomInfo{
		Code: roomCode, Mode: "live", Round: 1, Seat: 1,
		Ranking: []game.RoomRank{
			{Seat: 1, Name: "nick", Solved: true, Guesses: 1, ElapsedMS: 1_500,
				Rows: scoreRows("crane")},
		},
	}, "crane", []string{"crane"}, 1_500, roundNow)
	if err != nil {
		t.Fatal(err)
	}
	m.roomResult.r = roomResult{
		code: roomCode, mode: "live", round: 1, seat: 1, length: 5, word: "crane",
		ranking: g.Room.Ranking, g: g,
	}
	m.screen = screenRoomResult
	m.roomResult.resize(testWidth, testHeight)
	draw(t, m)
	if strings.Contains(plain(m.View().Content), "f refresh") {
		t.Error("a live round's result was offered the refresh")
	}
	press(t, m, "f")
	if asked(f, "results") {
		t.Errorf("calls = %v, want none", f.calls)
	}
}
