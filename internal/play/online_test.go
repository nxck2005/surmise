package play

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// fakeClient is a server the test answers for. Every call is counted.
type fakeClient struct {
	mu       sync.Mutex
	status   online.Status
	err      error // returned by every HTTP call when set
	posts    []string
	asks     []string
	count    online.DailyCount
	created  online.Created
	results  online.Results
	conns    chan *fakeConn // the next Dial's connection
	dialed   []string
	refusals int // Dials to refuse before the next works
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		status: online.Status{Features: online.Features{Daily: true, Rooms: true}},
		count:  online.DailyCount{Played: 7, Solved: 5, Distribution: []int{0, 1, 2, 2, 0, 0}},
		conns:  make(chan *fakeConn, 8),
	}
}

func (f *fakeClient) Status(context.Context) (online.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, "status")
	return f.status, f.err
}

func (f *fakeClient) PostDaily(_ context.Context, day string, length int, solved bool, guesses int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, _ := json.Marshal([]any{day, length, solved, guesses})
	f.posts = append(f.posts, string(b))
	return f.err
}

func (f *fakeClient) Daily(_ context.Context, day string, length int) (online.DailyCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, "daily")
	c := f.count
	c.Distribution = make([]int, length+1)
	return c, f.err
}

func (f *fakeClient) CreateRoom(context.Context, online.RoomOptions) (online.Created, error) {
	return f.created, f.err
}

func (f *fakeClient) Results(context.Context, string, string) (online.Results, error) {
	return f.results, f.err
}

func (f *fakeClient) Dial(_ context.Context, code string) (online.Conn, error) {
	f.mu.Lock()
	f.dialed = append(f.dialed, code)
	if f.refusals > 0 {
		f.refusals--
		f.mu.Unlock()
		return nil, errors.New("no network")
	}
	f.mu.Unlock()
	select {
	case c := <-f.conns:
		return c, nil
	case <-time.After(2 * time.Second):
		return nil, errors.New("test: no connection queued")
	}
}

func (f *fakeClient) postCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.posts)
}

// fakeConn is one socket. The test pushes what the server says into in.
type fakeConn struct {
	mu     sync.Mutex
	sent   []online.ClientMsg
	in     chan online.ServerMsg
	closed chan struct{}
	once   sync.Once
	wrote  chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{in: make(chan online.ServerMsg, 16), closed: make(chan struct{}), wrote: make(chan struct{}, 64)}
}

func (c *fakeConn) Send(_ context.Context, m online.ClientMsg) error {
	c.mu.Lock()
	c.sent = append(c.sent, m)
	c.mu.Unlock()
	c.wrote <- struct{}{}
	return nil
}

func (c *fakeConn) Receive(context.Context) (online.ServerMsg, error) {
	select {
	case m, ok := <-c.in:
		if !ok {
			return online.ServerMsg{}, errors.New("closed by the server")
		}
		return m, nil
	case <-c.closed:
		return online.ServerMsg{}, errors.New("closed")
	}
}

func (c *fakeConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

// sentTypes waits until n messages were sent and returns their types.
func (c *fakeConn) sentTypes(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		c.mu.Lock()
		if len(c.sent) >= n {
			out := make([]string, len(c.sent))
			for i, m := range c.sent {
				out[i] = m.T()
			}
			c.mu.Unlock()
			return out
		}
		c.mu.Unlock()
		select {
		case <-c.wrote:
		case <-deadline:
			t.Fatalf("waited for %d sent messages", n)
		}
	}
}

func (c *fakeConn) last(t *testing.T, n int) map[string]any {
	t.Helper()
	c.sentTypes(t, n)
	c.mu.Lock()
	defer c.mu.Unlock()
	var m map[string]any
	b, _ := json.Marshal(c.sent[n-1])
	_ = json.Unmarshal(b, &m)
	return m
}

// testHost runs posted work on the test goroutine, one item per pump.
type testHost struct {
	posted   chan func()
	notified []string
}

func (h *testHost) host(c online.Client) Host {
	h.posted = make(chan func(), 64)
	return Host{
		Client:     c,
		ClientName: "surmise/test (android/arm64)",
		Post:       func(f func()) { h.posted <- f },
		Notify:     func(kind string) { h.notified = append(h.notified, kind) },
	}
}

// pump runs one piece of posted work, waiting for it.
func (h *testHost) pump(t *testing.T) {
	t.Helper()
	select {
	case f := <-h.posted:
		f()
	case <-time.After(2 * time.Second):
		t.Fatal("waited for posted work")
	}
}

// quiet checks nothing more was posted.
func (h *testHost) quiet(t *testing.T) {
	t.Helper()
	select {
	case <-h.posted:
		t.Fatal("work was posted that should not have been")
	case <-time.After(30 * time.Millisecond):
	}
}

func onlineApp(t *testing.T) (*App, *store.JSON, *fakeClock, *fakeClient, *testHost) {
	t.Helper()
	a, s, c := newApp(t)
	fc := newFakeClient()
	h := &testHost{}
	a.SetHost(h.host(fc))
	if err := a.UpdateSettings(func(s *store.Settings) { s.Network = true }); err != nil {
		t.Fatal(err)
	}
	return a, s, c, fc, h
}

func TestNoConsentNoNetwork(t *testing.T) {
	a, _, _, fc, h := onlineApp(t)
	if err := a.SetNetwork(false); err != nil {
		t.Fatal(err)
	}
	a.AskStatus()
	a.AskMenuCount()
	if err := a.JoinRoom("ABC234"); !errors.Is(err, ErrOffline) {
		t.Errorf("join without consent: %v", err)
	}
	solve(t, openDaily(t, a, 5))
	h.quiet(t)
	if fc.postCount() != 0 || len(fc.asks) != 0 {
		t.Errorf("without consent: %d posts, asks %v", fc.postCount(), fc.asks)
	}
	if v := a.OnlineView(); v.Consent || v.Daily || v.Rooms {
		t.Errorf("view without consent: %+v", v)
	}
}

func TestStatusOnceAndGone(t *testing.T) {
	a, _, _, fc, h := onlineApp(t)
	fc.err = &online.GoneError{Message: "please update"}
	a.AskStatus()
	a.AskStatus()
	h.pump(t)
	h.quiet(t)
	v := a.OnlineView()
	if !v.Gone || v.GoneMessage != "please update" || v.Daily || len(fc.asks) != 1 {
		t.Errorf("after a gone: %+v, asks %v", v, fc.asks)
	}
	a.AskMenuCount()
	h.quiet(t)
}

func TestDailyPostedOnceFromTheFinishingGuess(t *testing.T) {
	a, _, _, fc, h := onlineApp(t)
	b := openDaily(t, a, 5)
	a.AskDailyCount(testDay.String(), 5) // not finished: no ask
	guess(t, b, miss(b))
	h.quiet(t)
	solve(t, b)
	h.pump(t) // the post's answer, which asks for the count
	h.pump(t) // the count
	if fc.postCount() != 1 {
		t.Fatalf("posts = %v, want one", fc.posts)
	}
	want := `["2026-11-01",5,true,2]`
	if fc.posts[0] != want {
		t.Errorf("post %s, want %s", fc.posts[0], want)
	}
	v := a.OnlineView()
	if len(v.Counts) != 1 || v.Counts[0].Played != 7 || len(v.Counts[0].Distribution) != 6 {
		t.Errorf("counts %+v", v.Counts)
	}
	// Opening it again, done, sends nothing.
	again := openDaily(t, a, 5)
	_ = again.Submit()
	h.quiet(t)
	if fc.postCount() != 1 {
		t.Errorf("a review posted again: %v", fc.posts)
	}
}

func TestMenuCountSumsAndStaysFresh(t *testing.T) {
	a, _, c, _, h := onlineApp(t)
	a.AskMenuCount()
	h.pump(t)
	if v := a.OnlineView(); v.MenuCount != 21 {
		t.Errorf("menu count %d, want 3 × 7", v.MenuCount)
	}
	a.AskMenuCount()
	h.quiet(t)
	c.advance(menuCountEvery)
	a.AskMenuCount()
	h.pump(t)
}

// --- rooms ---

const testKey = "0123456789abcdef0123456789abcdef"

var lobby = online.RoomView{
	Code: "ABC234", Mode: "live", Length: 5, Host: 1, Phase: "lobby", Round: 1,
	Players: []online.PlayerView{{Seat: 1, Name: "nick", Connected: true}},
	You:     online.YouView{Seat: 1},
}

func welcome(room online.RoomView) online.ServerMsg {
	return online.ServerMsg{T: "welcome", Seat: 1, Key: testKey, Room: &room}
}

func ptr[T any](v T) *T { return &v }

// joined makes a room, connects and is welcomed into the lobby.
func joined(t *testing.T) (*App, *store.JSON, *fakeClock, *fakeClient, *testHost, *fakeConn) {
	t.Helper()
	a, s, c, fc, h := onlineApp(t)
	fc.created = online.Created{Code: "ABC234", Key: testKey, Seat: 1}
	conn := newFakeConn()
	fc.conns <- conn
	if err := a.MakeRoom("live", 5, false); err != nil {
		t.Fatal(err)
	}
	h.pump(t) // created
	h.pump(t) // dialed
	if hello := conn.last(t, 1); hello["t"] != "hello" || hello["key"] != testKey || hello["client"] != "surmise/test (android/arm64)" {
		t.Fatalf("hello = %v", hello)
	}
	if v := a.RoomView(); v.Screen != "connecting" {
		t.Errorf("before the welcome: screen %q", v.Screen)
	}
	conn.in <- welcome(lobby)
	h.pump(t)
	return a, s, c, fc, h, conn
}

func TestRoomLobbyAndList(t *testing.T) {
	a, _, _, _, _, _ := joined(t)
	v := a.RoomView()
	if !v.Active || v.Screen != "lobby" || !v.IsHost || !v.Connected || len(v.Players) != 1 || !v.Players[0].You {
		t.Errorf("lobby view %+v", v)
	}
	if l := a.RoomsList(); len(l) != 1 || l[0].Code != "ABC234" || !l[0].Open {
		t.Errorf("rooms list %+v", l)
	}
}

func TestRoomRoundIsSavedOnceFinished(t *testing.T) {
	a, s, c, _, h, conn := joined(t)
	conn.in <- online.ServerMsg{T: "countdown", GoAt: 1, Round: 1}
	conn.in <- online.ServerMsg{T: "go", EndsAt: 2, Round: 1}
	h.pump(t)
	h.pump(t)
	if v := a.RoomView(); v.Screen != "board" || v.Board == nil || v.Board.Kind != KindRoom {
		t.Fatalf("after go: %+v", v)
	}
	for _, r := range "crane" {
		a.RoomType(r)
	}
	if err := a.RoomSubmit(); err != nil {
		t.Fatal(err)
	}
	g := conn.last(t, 2)
	if g["t"] != "guess" || g["word"] != "crane" || g["n"] != float64(1) {
		t.Fatalf("guess = %v", g)
	}
	a.RoomType('x') // the row is out: no typing
	if v := a.RoomView(); !v.Pending || v.Board.Typing != "crane" {
		t.Errorf("while pending: %+v", v.Board)
	}
	all := []game.Mark{game.Correct, game.Correct, game.Correct, game.Correct, game.Correct}
	conn.in <- online.ServerMsg{T: "marks", N: 1, Marks: all}
	conn.in <- online.ServerMsg{T: "done", Seat: 1, Solved: ptr(true), Guesses: 1, ElapsedMS: 9000}
	results := online.ServerMsg{T: "results", Round: 1, Word: "crane", Ranking: []online.RankEntry{
		{Seat: 1, Name: "nick", Solved: true, Guesses: 1, ElapsedMS: 9000, Rows: [][]game.Mark{all}},
	}}
	conn.in <- results
	for range 3 {
		h.pump(t)
	}
	v := a.RoomView()
	if v.Screen != "result" || v.Results == nil || v.Results.ID == "" || v.Results.Word != "crane" || !v.Results.CanRematch {
		t.Fatalf("result view %+v", v.Results)
	}
	saved, err := s.Load(game.RoomRoundID("ABC234", 1, 1))
	if err != nil || saved.Status != game.Won || saved.Room.Seat != 1 {
		t.Fatalf("saved round: %v %+v", err, saved)
	}
	if got := a.Settings().PlaytimeMS; got != 9000 {
		t.Errorf("playtime %d, want 9000", got)
	}
	// A reconnect replays the results: same record, time banked once.
	c.advance(time.Second)
	room := lobby
	room.Phase, room.Results = "results", &online.Results{Round: 1, Word: "crane", Ranking: results.Ranking}
	room.You = online.YouView{Seat: 1, InRound: true, Words: []string{"crane"}, Marks: [][]game.Mark{all}, Done: true, Solved: true, ElapsedMS: 9000}
	conn.in <- welcome(room)
	h.pump(t)
	if got := a.Settings().PlaytimeMS; got != 9000 {
		t.Errorf("playtime after a replay %d, want 9000", got)
	}
	a.RoomRematch()
	if m := conn.last(t, 3); m["t"] != "rematch" {
		t.Errorf("rematch sent %v", m)
	}
}

func TestRoomMarksThatDisagreeAreNotSaved(t *testing.T) {
	a, s, _, _, h, conn := joined(t)
	conn.in <- online.ServerMsg{T: "countdown", GoAt: 1, Round: 1}
	conn.in <- online.ServerMsg{T: "go", EndsAt: 2, Round: 1}
	h.pump(t)
	h.pump(t)
	for _, r := range "crane" {
		a.RoomType(r)
	}
	_ = a.RoomSubmit()
	all := []game.Mark{game.Correct, game.Correct, game.Correct, game.Correct, game.Correct}
	conn.in <- online.ServerMsg{T: "marks", N: 1, Marks: all}
	// The server names a word our row did not solve.
	conn.in <- online.ServerMsg{T: "results", Round: 1, Word: "slate", Ranking: []online.RankEntry{
		{Seat: 1, Solved: true, Guesses: 1, ElapsedMS: 9000, Rows: [][]game.Mark{all}},
	}}
	h.pump(t)
	h.pump(t)
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("a disagreeing round was saved: %v", ids)
	}
	if n := a.RoomView().Note; n.Code != "save_failed" {
		t.Errorf("note %+v", n)
	}
}

func TestRoomRefuseDropsTheRoom(t *testing.T) {
	a, _, _, fc, h := onlineApp(t)
	conn := newFakeConn()
	fc.conns <- conn
	if err := a.JoinRoom(" abc234 "); err != nil {
		t.Fatal(err)
	}
	h.pump(t)
	conn.in <- online.ServerMsg{T: "refuse", Reason: "no_room"}
	h.pump(t)
	v := a.RoomView()
	if v.Active || v.Note.Code != "no_room" || len(a.RoomsList()) != 0 {
		t.Errorf("after a refusal: %+v, list %v", v, a.RoomsList())
	}
	if err := a.JoinRoom("abc"); !errors.Is(err, ErrBadRoomCode) {
		t.Errorf("a short code: %v", err)
	}
}

func TestRoomReconnectsAndResendsTheGuess(t *testing.T) {
	defer func(old func(int) time.Duration) { roomBackoff = old }(roomBackoff)
	roomBackoff = func(int) time.Duration { return time.Millisecond }

	a, _, _, fc, h, conn := joined(t)
	conn.in <- online.ServerMsg{T: "countdown", GoAt: 1, Round: 1}
	conn.in <- online.ServerMsg{T: "go", EndsAt: 2, Round: 1}
	h.pump(t)
	h.pump(t)
	for _, r := range "crane" {
		a.RoomType(r)
	}
	_ = a.RoomSubmit()
	conn.sentTypes(t, 2)

	next := newFakeConn()
	fc.conns <- next
	close(conn.in) // the socket drops
	h.pump(t)
	if v := a.RoomView(); !v.Reconnecting {
		t.Fatalf("after the drop: %+v", v)
	}
	if err := a.RoomSubmit(); CodeOf(err) != "" && CodeOf(err) != "reconnecting" {
		t.Errorf("submit while away: %v", err)
	}
	h.pump(t) // the retry
	h.pump(t) // dialed
	if hello := next.last(t, 1); hello["key"] != testKey {
		t.Errorf("rejoin hello %v", hello)
	}
	room := lobby
	room.Phase = "playing"
	room.You = online.YouView{Seat: 1, InRound: true}
	next.in <- welcome(room)
	h.pump(t)
	if g := next.last(t, 2); g["t"] != "guess" || g["word"] != "crane" {
		t.Errorf("the owed guess was not sent again: %v", g)
	}
	if v := a.RoomView(); v.Reconnecting || !v.Connected {
		t.Errorf("after the rejoin: %+v", v)
	}
}

func TestLeaveRoomSendsLeaveAndForgets(t *testing.T) {
	a, _, _, _, _, conn := joined(t)
	a.LeaveRoom("ABC234")
	if types := conn.sentTypes(t, 2); types[1] != "leave" {
		t.Errorf("sent %v", types)
	}
	if a.RoomView().Active || len(a.RoomsList()) != 0 {
		t.Error("the room is still open or listed")
	}
}

func TestProbeFindsARunningRound(t *testing.T) {
	a, s, _, fc, h := onlineApp(t)
	if err := s.SaveRooms([]store.RoomEntry{{Code: "ABC234", Key: testKey, Mode: "live", JoinedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	conn := newFakeConn()
	room := lobby
	room.Phase = "playing"
	room.You = online.YouView{Seat: 1, InRound: true}
	conn.in <- welcome(room)
	fc.conns <- conn
	a.ProbeRooms()
	h.pump(t)
	v := a.RoomView()
	if !v.Probe || v.Screen != "board" || v.Note.Code != "round_running" || v.Note.Room != "ABC234" {
		t.Errorf("probe view %+v", v)
	}
	a.ProbeRooms() // once a run
	h.quiet(t)
}

// The day the daily tests share is the one these tests run on.
var _ = daily.ID(testDay, 5)
