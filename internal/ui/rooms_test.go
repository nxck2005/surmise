package ui

// PR 8's room tests: make, join, wait. The live lobby is the one shape every
// room screen here reads, and deliver is how its messages arrive, the way the
// listen loop would have handed them in.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// fakeConn records what the UI sends. Receive is never called in tests:
// newOnlineModel sets m.listen to a stub.
type fakeConn struct {
	sent   []online.ClientMsg
	closed bool
}

func (f *fakeConn) Send(_ context.Context, m online.ClientMsg) error {
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeConn) Receive(context.Context) (online.ServerMsg, error) {
	return online.ServerMsg{}, errors.New("rooms_test: Receive is not scripted here")
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

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
	welcomeKey = f.created.Key

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
