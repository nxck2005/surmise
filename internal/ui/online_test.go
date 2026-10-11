package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

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
	runCmd(t, m, m.Init())
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
	runCmd(t, m, m.Init())
	if n := len(f.calls); n != 1 || f.calls[0] != "status" {
		t.Fatalf("calls = %v, want exactly one status", f.calls)
	}

	// Toggling the network off and on in settings asks nothing again.
	m.menu.cursor = menuIndex(t, m, choiceSettings, 0)
	send(t, m, "enter") // the settings screen
	send(t, m, "down", "down", "down")
	if m.settings.cursor != rowNetwork {
		t.Fatalf("cursor = %d, want the network row", m.settings.cursor)
	}
	send(t, m, "right") // off; a commit runs, and asks nothing
	send(t, m, "right") // on; likewise
	if n := len(f.calls); n != 1 {
		t.Errorf("calls = %v, want the one status still", f.calls)
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
	// result.
	send(t, m, "enter")
	send(t, m, "enter")
	if _, n := f.posts(); n != 1 {
		t.Errorf("after re-entering a finished board, posts = %v", posts)
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
	runCmd(t, m, m.Init())

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
	runCmd(t, m, m.Init())

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
	runCmd(t, m, m.Init())

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
