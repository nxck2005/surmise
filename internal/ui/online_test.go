package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
)

// fakeOnline is a scripted server. Each field is what the matching call
// returns; calls are recorded in order.
type fakeOnline struct {
	status    online.Status
	statusErr error
	daily     map[int]online.DailyCount
	dailyErr  error
	postErr   error
	calls     []string // "status", "post 2026-08-06 5 true 3", "daily 2026-08-06 5"
}

func (f *fakeOnline) Status(context.Context) (online.Status, error) {
	f.calls = append(f.calls, "status")
	return f.status, f.statusErr
}

func (f *fakeOnline) PostDaily(_ context.Context, day string, length int, solved bool, guesses int) error {
	f.calls = append(f.calls, fmt.Sprintf("post %s %d %t %d", day, length, solved, guesses))
	return f.postErr
}

func (f *fakeOnline) Daily(_ context.Context, day string, length int) (online.DailyCount, error) {
	f.calls = append(f.calls, fmt.Sprintf("daily %s %d", day, length))
	return f.daily[length], f.dailyErr
}

// The room half of the client. PR 8 replaces these with the real fakes; the
// zero answers keep the rest of the UI tests compiling while the client lands.
func (f *fakeOnline) CreateRoom(context.Context, online.RoomOptions) (online.Created, error) {
	return online.Created{}, online.ErrOff
}

func (f *fakeOnline) Results(context.Context, string, string) (online.Results, error) {
	return online.Results{}, online.ErrOff
}

func (f *fakeOnline) Dial(context.Context, string) (online.Conn, error) {
	return nil, online.ErrOff
}

// posts reports how many result posts were recorded, and the one there was.
func (f *fakeOnline) posts() ([]string, int) {
	var got []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "post ") {
			got = append(got, c)
		}
	}
	return got, len(got)
}

// dailies reports the daily-count asks.
func (f *fakeOnline) dailies() []string {
	var got []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "daily ") {
			got = append(got, c)
		}
	}
	return got
}

// newOnlineModel is newModel with a scripted server. The day is pinned the way
// the daily tests pin it, the saved network choice is written before New reads
// the store, and the board is left still.
func newOnlineModel(t *testing.T, f *fakeOnline, network bool) *Model {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	if err := s.SaveSettings(store.Settings{Network: network}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	m := New(s, nil, Options{Online: f, Day: testDay, Motion: motionOffName})
	m.screen = screenMenu
	return m
}

// openDaily raises the daily screen from the menu and drains the asks it makes.
func openDaily(t *testing.T, m *Model) {
	t.Helper()
	m.menu.cursor = menuIndex(t, m, choiceDaily, 0)
	_, cmd := m.Update(key("enter"))
	runCmd(t, m, cmd)
	if m.screen != screenDaily {
		t.Fatalf("screen = %v, want the daily screen", m.screen)
	}
}

// winDaily plays the day's 5-letter daily to a win, draining the command that
// opens it and the one that reports the result.
func winDaily(t *testing.T, m *Model) {
	t.Helper()
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e")
	_, cmd := m.Update(key("enter")) // the finishing guess
	runCmd(t, m, cmd)
	if m.screen != screenResult {
		t.Fatalf("screen = %v, want the result screen", m.screen)
	}
}

func TestNothingOnlineWithoutConsent(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, false)
	m.Init() // not drained: its batch holds the clock
	if m.statusAsked {
		t.Error("Init asked for the status without consent")
	}
	winDaily(t, m)
	send(t, m, "esc") // back to the menu
	openDaily(t, m)
	if n := len(f.calls); n != 0 {
		t.Errorf("calls = %v, want none without consent", f.calls)
	}
}

func TestStatusIsAskedOnceWithConsent(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	// Init is not drained (its batch holds the clock); asking is what marks
	// the status as asked, and only statusCmd does that.
	m.Init()
	if !m.statusAsked {
		t.Fatal("Init did not ask for the status with consent")
	}

	// Toggling the network off and on in settings asks nothing again, with
	// every command each key returns drained.
	m.menu.cursor = menuIndex(t, m, choiceSettings, 0)
	press(t, m, "enter") // the settings screen
	send(t, m, "down", "down", "down")
	if m.settings.cursor != rowNetwork {
		t.Fatalf("cursor = %d, want the network row", m.settings.cursor)
	}
	press(t, m, "right") // off
	press(t, m, "right") // on
	if len(f.calls) != 0 {
		t.Errorf("calls = %v, want none: the status was already asked this run", f.calls)
	}
}

func TestTurningNetworkOnAsksForStatus(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, false)
	m.menu.cursor = menuIndex(t, m, choiceSettings, 0)
	send(t, m, "enter")
	send(t, m, "down", "down", "down")
	if m.settings.cursor != rowNetwork {
		t.Fatalf("cursor = %d, want the network row", m.settings.cursor)
	}
	_, cmd := m.Update(key("right"))
	runCmd(t, m, cmd)

	if _, n := f.posts(); n != 0 {
		t.Errorf("posts = %v, want none", f.calls)
	}
	if n := len(f.calls); n != 1 || f.calls[0] != "status" {
		t.Errorf("calls = %v, want exactly one status", f.calls)
	}
}

func TestFinishingADailyPostsOnce(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	winDaily(t, m)

	posts, n := f.posts()
	if n != 1 {
		t.Fatalf("posts = %v, want exactly one", posts)
	}
	if !strings.HasPrefix(posts[0], "post "+testDay+" 5 true ") {
		t.Errorf("post = %q, want it for the 2026-08-06 five-letter mode", posts[0])
	}

	// Reviewing the finished board and pressing enter again is not a second
	// result. press drains what each key asks for, so a post would be seen.
	press(t, m, "enter") // review
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the reviewed board", m.screen)
	}
	press(t, m, "enter") // back to the result
	if _, n := f.posts(); n != 1 {
		t.Errorf("after re-entering a finished board, posts = %v", f.calls)
	}
}

func TestDailyScreenShowsCountsForFinishedModesOnly(t *testing.T) {
	f := &fakeOnline{daily: map[int]online.DailyCount{
		5: {Played: 1204, Solved: 1047, Distribution: []int{0, 12, 140, 410, 330, 155}},
	}}
	m := newOnlineModel(t, f, true)
	winDaily(t, m)
	send(t, m, "esc") // back to the menu
	openDaily(t, m)

	if got := f.dailies(); len(got) != 1 || got[0] != "daily "+testDay+" 5" {
		t.Errorf("dailies = %v, want one ask for the finished mode only", got)
	}
	frame := draw(t, m)
	want := "5 letters · 1,204 played · 86% solved · most in 4"
	if !strings.Contains(frame, want) {
		t.Errorf("frame does not carry the count line %q:\n%s", want, frame)
	}
}

func TestNobodySolvedLine(t *testing.T) {
	f := &fakeOnline{daily: map[int]online.DailyCount{
		5: {Played: 3, Solved: 0, Distribution: []int{0, 0, 0, 0, 0, 0}},
	}}
	m := newOnlineModel(t, f, true)
	winDaily(t, m)
	send(t, m, "esc")
	openDaily(t, m)

	frame := draw(t, m)
	want := "5 letters · 3 played · nobody solved it yet"
	if !strings.Contains(frame, want) {
		t.Errorf("frame does not carry the count line %q:\n%s", want, frame)
	}
}

func TestGoneStopsEverything(t *testing.T) {
	f := &fakeOnline{statusErr: &online.GoneError{Message: "please update"}}
	m := newOnlineModel(t, f, true)
	runCmd(t, m, m.statusCmd())

	if frame := draw(t, m); !strings.Contains(frame, "please update") {
		t.Errorf("frame does not say why the server refused this client:\n%s", frame)
	}
	if frame := draw(t, m); !strings.Contains(frame, "please update") {
		t.Errorf("drawing again dropped the note before any key:\n%s", frame)
	}

	// One key spends the note, like an error line.
	if frame := send(t, m, "a"); strings.Contains(frame, "please update") {
		t.Errorf("the note outlived a key press:\n%s", frame)
	}

	// And nothing online is tried again this run.
	winDaily(t, m)
	if _, n := f.posts(); n != 0 {
		t.Errorf("posts = %v, want none after the server said gone", f.calls)
	}
}

func TestStatusCanSwitchTheDailyOff(t *testing.T) {
	f := &fakeOnline{status: online.Status{Features: online.Features{Daily: false}}}
	m := newOnlineModel(t, f, true)
	runCmd(t, m, m.statusCmd())

	winDaily(t, m)
	if _, n := f.posts(); n != 0 {
		t.Errorf("posts = %v, want none with the daily switched off", f.calls)
	}
	send(t, m, "esc")
	openDaily(t, m)
	if got := f.dailies(); len(got) != 0 {
		t.Errorf("dailies = %v, want none with the daily switched off", got)
	}
}

func TestStatusMessageShowsOnce(t *testing.T) {
	f := &fakeOnline{status: online.Status{Message: "maintenance tonight"}}
	m := newOnlineModel(t, f, true)
	runCmd(t, m, m.statusCmd())

	if frame := draw(t, m); !strings.Contains(frame, "maintenance tonight") {
		t.Errorf("frame does not carry the server's message:\n%s", frame)
	}
	if frame := send(t, m, "a"); strings.Contains(frame, "maintenance tonight") {
		t.Errorf("the message outlived a key press:\n%s", frame)
	}
}

func TestFormatCount(t *testing.T) {
	for got, want := range map[int]string{
		0:       "0",
		999:     "999",
		1204:    "1,204",
		1234567: "1,234,567",
	} {
		if have := formatCount(got); have != want {
			t.Errorf("formatCount(%d) = %q, want %q", got, have, want)
		}
	}
}

func TestMostIn(t *testing.T) {
	for dist, want := range map[string]int{
		"0,12,140,410,330,155": 4,
		"0,5,5,0,0,0":          2,
	} {
		var d []int
		for _, s := range strings.Split(dist, ",") {
			var n int
			fmt.Sscan(strings.TrimSpace(s), &n)
			d = append(d, n)
		}
		if got := mostIn(online.DailyCount{Distribution: d}); got != want {
			t.Errorf("mostIn(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestServerNoteIsCapped(t *testing.T) {
	long := strings.Repeat("é", 500)
	f := &fakeOnline{statusErr: &online.GoneError{Message: long}}
	m := newOnlineModel(t, f, true)
	runCmd(t, m, m.statusCmd())
	if n := len([]rune(m.note)); n != maxNoteRunes {
		t.Errorf("note is %d runes, want %d", n, maxNoteRunes)
	}
	if !strings.HasSuffix(m.note, "…") {
		t.Errorf("a cut note should end in an ellipsis: %q", m.note)
	}
	if got := serverNote("short"); got != "short" {
		t.Errorf("serverNote(short) = %q", got)
	}
}

// threeModes is a count for every mode of the day: 1,000 + 2,000 + 410 boards.
func threeModes() map[int]online.DailyCount {
	return map[int]online.DailyCount{
		4: {Played: 1000, Distribution: make([]int, 5)},
		5: {Played: 2000, Distribution: make([]int, 6)},
		6: {Played: 410, Distribution: make([]int, 7)},
	}
}

func TestMenuShowsDailiesPlayed(t *testing.T) {
	f := &fakeOnline{daily: threeModes()}
	m := newOnlineModel(t, f, true)
	m.openMenu()
	runCmd(t, m, m.menuCountCmd())

	if got := f.dailies(); len(got) != 3 {
		t.Fatalf("dailies = %v, want one ask per mode", got)
	}
	if frame := draw(t, m); !strings.Contains(frame, "3,410 dailies today") {
		t.Errorf("menu does not carry the day's count:\n%s", frame)
	}
}

func TestMenuCountSaysOneDaily(t *testing.T) {
	m := newModel(t)
	m.menu.played = 1
	if got := m.menu.status(); got != "1 daily today" {
		t.Errorf("status = %q", got)
	}
}

func TestMenuCountJoinsTheStatusLine(t *testing.T) {
	m := newModel(t)
	m.menu.modesDone, m.menu.modesOf, m.menu.streak, m.menu.played = 1, 3, 4, 3410
	want := "daily 1/3 · streak 4 · 3,410 dailies today"
	if got := m.menu.status(); got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

func TestMenuCountNeedsConsent(t *testing.T) {
	f := &fakeOnline{daily: threeModes()}
	m := newOnlineModel(t, f, false)
	m.openMenu()
	if cmd := m.menuCountCmd(); cmd != nil {
		t.Error("the menu asked for a count without consent")
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %v, want none", f.calls)
	}
}

func TestMenuCountOnlyOnTheMenu(t *testing.T) {
	f := &fakeOnline{daily: threeModes()}
	m := newOnlineModel(t, f, true)
	m.openMenu()
	press(t, m, "enter") // a board
	if m.screen == screenMenu {
		t.Fatal("still on the menu")
	}
	if cmd := m.menuCountCmd(); cmd != nil {
		t.Error("asked for the menu's count away from the menu")
	}
}

func TestMenuCountIsNotAskedEveryTick(t *testing.T) {
	f := &fakeOnline{daily: threeModes()}
	m := newOnlineModel(t, f, true)
	m.openMenu()

	// The clock arms it: one tick on the menu sends the ask out.
	m.Update(tickMsg(timeNow()))
	if !m.menuCountBusy {
		t.Fatal("a tick on the menu did not ask for the count")
	}
	// While it is out, and after it lands, the next ticks ask nothing more
	// until menuCountEvery has passed.
	if cmd := m.menuCountCmd(); cmd != nil {
		t.Error("asked again while the first ask was out")
	}
	m.Update(menuCountMsg{day: m.day.String(), played: 5})
	if cmd := m.menuCountCmd(); cmd != nil {
		t.Error("asked again before the count went stale")
	}
	m.menuCountAt = timeNow().Add(-menuCountEvery - time.Second)
	if cmd := m.menuCountCmd(); cmd == nil {
		t.Error("did not ask again once the count was stale")
	}
}

func TestMenuCountGoesWithTheNetwork(t *testing.T) {
	f := &fakeOnline{daily: threeModes()}
	m := newOnlineModel(t, f, true)
	m.openMenu()
	runCmd(t, m, m.menuCountCmd())
	if m.menu.played == 0 {
		t.Fatal("no count to begin with")
	}
	s := m.settingsOf()
	s.Network = false
	m.saveSettings(s)
	m.openMenu()
	if strings.Contains(draw(t, m), "dailies today") {
		t.Error("the count outlived turning the network off")
	}
}

func TestMenuCountIgnoresAFailedMode(t *testing.T) {
	f := &fakeOnline{daily: threeModes(), dailyErr: &online.StatusError{Code: 500}}
	m := newOnlineModel(t, f, true)
	m.openMenu()
	runCmd(t, m, m.menuCountCmd())
	if m.menu.played != 0 {
		t.Errorf("played = %d, want nothing shown after a failure", m.menu.played)
	}
	if m.menuCountBusy {
		t.Error("a failed ask stayed out")
	}
}
