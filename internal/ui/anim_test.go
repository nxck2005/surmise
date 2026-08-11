package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/store"
)

// The load-bearing claim of this whole feature is that an animation repaints and
// nothing else: no cell moves, no target shifts, and a settled board is exactly
// the board that came before any of this existed. Most of what follows is that
// one claim, checked from a different angle each time.

// withClock holds time still so a phase can be stepped deliberately. Sleeping
// through a real animation would make these tests slow and flaky at once; this
// makes them neither. Shaped like withTheme, and restored the same way.
func withClock(t *testing.T, at *time.Time) {
	t.Helper()
	saved := timeNow
	timeNow = func() time.Time { return *at }
	t.Cleanup(func() { timeNow = saved })
}

// startBoard opens a 5-letter puzzle with a known answer, so a test can win,
// lose or be refused on demand.
func startBoard(t *testing.T, m *Model, answer string) {
	t.Helper()
	send(t, m, "down", "enter")
	m.game.g.Answer = answer
}

// advance moves the clock and delivers the frame that a real timer would.
func advance(t *testing.T, m *Model, now *time.Time, d time.Duration) {
	t.Helper()
	*now = now.Add(d)
	m.Update(animMsg(*now))
}

// settle runs the animation out and returns how many frames it took. It fails
// rather than looping forever: a chain that never dies is the bug this guards.
func settle(t *testing.T, m *Model, now *time.Time) int {
	t.Helper()
	for i := range 200 {
		if !m.anim.busy(*now) {
			return i
		}
		advance(t, m, now, frameInterval)
	}
	t.Fatal("animation never settled")
	return 0
}

func TestMotionOffRendersTheSettledFrame(t *testing.T) {
	cases := []struct {
		name  string
		keys  []string
		after string
	}{
		{"a scored guess", []string{"s", "l", "a", "t", "e", "enter"}, ""},
		{"a refused guess", []string{"z", "z", "z", "z", "z", "enter"}, ""},
		{"a letter typed", []string{"c"}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := time.Now()
			withClock(t, &now)

			still := newModel(t) // motion off
			startBoard(t, still, "crane")

			moving := animModel(t)
			startBoard(t, moving, "crane")
			// The same board, not merely the same answer: a puzzle's code is
			// drawn from its id, and two random puzzles would differ on the
			// header alone.
			moving.game.g.ID = still.game.g.ID

			send(t, still, c.keys...)
			send(t, moving, c.keys...)
			settle(t, moving, &now)

			// Stop the header's clock in both, the way every other
			// frame-comparison test does.
			freezeClock(still)
			freezeClock(moving)

			if got, want := moving.frame(nil), still.frame(nil); got != want {
				t.Errorf("settled frame differs from the un-animated one\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

func TestAnimationRepaintsWithoutMovingAnything(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	send(t, m, "s", "l", "a", "t", "e", "enter")

	// The text of every frame across the whole reveal must equal the text of
	// the settled one. Only the styling may differ — that is what "repaint, not
	// move" means, and it is what keeps click targets where they were.
	var frames []string
	for m.anim.busy(now) {
		frames = append(frames, sgr.ReplaceAllString(m.frame(nil), ""))
		advance(t, m, &now, frameInterval)
	}
	if len(frames) < 2 {
		t.Fatalf("reveal produced %d frames, expected several", len(frames))
	}

	settled := sgr.ReplaceAllString(m.frame(nil), "")
	for i, f := range frames {
		if f != settled {
			t.Fatalf("frame %d moved the layout\n got: %q\nwant: %q", i, f, settled)
		}
	}
}

func TestRevealTurnsTilesLeftToRight(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	send(t, m, "s", "l", "a", "t", "e", "enter")

	shown, revealing := m.anim.reveal(now, m.game.g.ID, 0)
	if !revealing || shown != 0 {
		t.Fatalf("reveal starts at %d tiles (revealing=%v), want 0", shown, revealing)
	}

	last := 0
	for m.anim.busy(now) {
		shown, _ := m.anim.reveal(now, m.game.g.ID, 0)
		if shown < last {
			t.Fatalf("reveal went backwards: %d after %d", shown, last)
		}
		last = shown
		advance(t, m, &now, frameInterval)
	}
	if last < m.game.g.Length {
		t.Errorf("reveal stopped at %d of %d tiles", last, m.game.g.Length)
	}
}

func TestAnimationChainStopsWhenIdle(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	if cmd := m.animCmd(); cmd != nil {
		t.Fatal("a quiet board armed a frame timer")
	}

	send(t, m, "s", "l", "a", "t", "e", "enter")
	if !m.anim.busy(now) {
		t.Fatal("a submitted guess started nothing")
	}
	settle(t, m, &now)

	if cmd := m.animCmd(); cmd != nil {
		t.Error("the chain re-armed itself after everything settled")
	}
}

func TestOneChainAtATime(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")

	// Two effects back to back: the second must join the chain the first armed.
	send(t, m, "c")
	if !m.anim.live {
		t.Fatal("the first effect armed no chain")
	}
	send(t, m, "r")
	if cmd := m.animCmd(); cmd != nil {
		t.Error("a second effect armed a second chain")
	}
}

func TestWinningRevealsBeforeTheResult(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	send(t, m, "c", "r", "a", "n", "e", "enter")

	if m.screen != screenGame || !m.pendingResult {
		t.Fatalf("screen = %v, pendingResult = %v: the win should stay on the board while it reveals",
			m.screen, m.pendingResult)
	}
	// Banking and saving happen with the guess, never after the animation.
	if m.game.g.Elapsed() < 0 || !m.game.persisted {
		t.Error("the winning guess was not banked and saved before its reveal")
	}

	settle(t, m, &now)
	if m.screen != screenResult {
		t.Errorf("screen = %v once the reveal ended, want the result", m.screen)
	}
}

func TestAnyInputSkipsToTheResult(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if !m.pendingResult {
		t.Fatal("the win did not defer its result")
	}

	// "n" is the result screen's "next puzzle". Skipping must spend it, not act
	// on it, or a player who hurries lands on a puzzle they never asked for.
	before := m.game.g.ID
	send(t, m, "n")

	if m.screen != screenResult {
		t.Fatalf("screen = %v after a key mid-reveal, want the result", m.screen)
	}
	if m.game.g.ID != before {
		t.Error("the skipping key also started a new puzzle")
	}
	if m.anim.busy(now) {
		t.Error("the animation kept running after being skipped")
	}
}

func TestClickTargetsSurviveAnAnimation(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := animModel(t)
	startBoard(t, m, "crane")
	send(t, m, "s", "l", "a", "t", "e", "enter")

	// Mid-reveal, every keycap is still where it was and still does its job.
	advance(t, m, &now, frameInterval)
	q := action{kind: actLetter, letter: 'q'}
	frame := draw(t, m)
	r, ok := m.hits.find(q)
	if !ok {
		t.Fatal("the Q key vanished during a reveal")
	}
	if got := strings.TrimSpace(at(t, frame, r)); got != "Q" {
		t.Errorf("the Q key's rect covers %q mid-reveal", got)
	}

	click(t, m, q)
	if m.game.typing != "q" {
		t.Errorf("typing = %q after clicking Q mid-reveal, want %q", m.game.typing, "q")
	}
}

func TestMotionOffIsNeverBusy(t *testing.T) {
	now := time.Now()
	withClock(t, &now)

	m := newModel(t) // motion off
	startBoard(t, m, "crane")
	send(t, m, "c", "r", "a", "n", "e", "enter")

	if m.anim.busy(now) {
		t.Error("motion off still started an effect")
	}
	if m.screen != screenResult {
		t.Errorf("screen = %v, want the result in the same update", m.screen)
	}
}

func TestParseMotion(t *testing.T) {
	cases := []struct {
		in   string
		want motion
		ok   bool
	}{
		{"", motionPronounced, true}, // nothing chosen is the fullest motion
		{motionOffName, motionOff, true},
		{motionRestrainedName, motionRestrained, true},
		{motionPronouncedName, motionPronounced, true},
		{"sideways", motionPronounced, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := parseMotion(c.in)
			if got != c.want || ok != c.ok {
				t.Errorf("parseMotion(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
			}
			// Whatever comes back must survive a round trip through the label
			// the settings screen shows and the settings file stores.
			if again, _ := parseMotion(got.setting()); again != got {
				t.Errorf("%v does not round-trip through %q", got, got.setting())
			}
		})
	}
}

func TestMotionResolution(t *testing.T) {
	cases := []struct {
		name  string
		saved string
		opts  Options
		want  motion
	}{
		{"a settings file written before motion existed", "", Options{}, motionPronounced},
		{"a saved choice", motionRestrainedName, Options{}, motionRestrained},
		{"an override beats the saved choice", motionOffName, Options{Motion: motionPronouncedName}, motionPronounced},
		{"an unreadable value falls back", "sideways", Options{}, motionPronounced},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := newStore(t)
			if err := s.SaveSettings(store.Settings{Motion: c.saved}); err != nil {
				t.Fatal(err)
			}
			m := New(s, nil, c.opts)
			if m.anim.motion != c.want {
				t.Errorf("motion = %v, want %v", m.anim.motion, c.want)
			}
		})
	}
}

func TestUnreadableMotionIsReportedNotFatal(t *testing.T) {
	s, _ := newStore(t)
	if err := s.SaveSettings(store.Settings{Motion: "sideways"}); err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{})
	if m.err == nil {
		t.Error("an unknown motion setting was swallowed")
	}
	if m.game == nil {
		t.Error("an unknown motion setting cost the player their puzzle")
	}
}

func TestPronouncedIsSlowerThanRestrained(t *testing.T) {
	a := anim{kind: animReveal, length: 5}
	if got, want := a.duration(motionPronounced), a.duration(motionRestrained); got <= want {
		t.Errorf("pronounced reveal %v, restrained %v: pronounced should be longer", got, want)
	}
	if got := a.duration(motionOff); got != 0 {
		t.Errorf("motion off has a %v reveal, want none", got)
	}
	// A loss hurries: the answer on the result screen is what is wanted.
	loss := anim{kind: animLoss, length: 5}
	if got, want := loss.duration(motionRestrained), a.duration(motionRestrained); got >= want {
		t.Errorf("loss reveal %v is not faster than a plain reveal %v", got, want)
	}
}

func TestNilAnimsRenderTheSettledBoard(t *testing.T) {
	// A caller with no animation state at all — the layout tests today, a theme
	// previewer later — must be able to render without one.
	var a *anims
	if a.on() || a.busy(time.Now()) || a.rejected(time.Now()) || a.accented(time.Now()) {
		t.Error("a nil anims claimed to be animating")
	}
	if _, revealing := a.reveal(time.Now(), "id", 0); revealing {
		t.Error("a nil anims claimed to be revealing")
	}
	a.clear() // must not panic
}
