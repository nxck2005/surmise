package ui

import (
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/banner"
)

// splashScreen is the title moment at startup: a piece of ASCII art over the
// tagline, shown in front of a board that is already built and waiting behind
// it. Dismissing it is therefore a screen swap and nothing else — no puzzle is
// created here, and no time is banked, because the clock does not start until
// the first letter is typed.
//
// Which art it draws and how it goes away are both preferences (see
// store.Settings), so this screen holds the resolved values rather than reading
// them each frame.
type splashScreen struct {
	art  banner.Banner
	mode splashMode

	// duration is how long a timed mode waits. It is held here rather than read
	// from the settings each time the timer is armed, so the splash cannot
	// change length under a player who is looking at it.
	duration time.Duration

	// next is where dismissing lands, captured when the splash was raised. It is
	// normally the board, but a startup that failed to make a puzzle sits on the
	// menu instead, and the splash must not send that player to an empty board.
	next screen

	width, height int
}

// The three values that are not the name of a piece of art. They are what the
// splash setting and the -splash override are written with, and "off" is the
// one of them that has to survive a rename of every banner there is.
const (
	splashOn     = "on"
	splashOff    = "off"
	splashRandom = "random"
)

// splashMode is how the splash ends.
type splashMode int

const (
	// splashSkip goes on its own after splashDuration, and any key or click
	// cuts that short.
	splashSkip splashMode = iota
	// splashKey is the default. It waits for input and never times out — a title
	// screen proper.
	splashKey
	// splashFixed times out and ignores input, for a launch that always looks
	// the same. ctrl+c still quits, since that is handled above every screen.
	splashFixed
)

// splashModes is the order the settings screen cycles through, default first.
var splashModes = []splashMode{splashKey, splashSkip, splashFixed}

// splashDuration is the built-in wait for a timed splash: long enough to read,
// short enough that someone who launched to play a puzzle does not wait on it.
const splashDuration = 1200 * time.Millisecond

// splashDurations are what the settings screen steps through. A short list of
// round numbers rather than a free number: the value only has to be roughly
// right, and stepping to it must stay one keypress.
//
// The built-in default is one of them on purpose — a screen whose current value
// is not among the ones it offers has to snap somewhere the moment it is
// touched, and that is a change nobody asked for.
var splashDurations = []time.Duration{
	600 * time.Millisecond,
	splashDuration,
	2 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

// maxSplashDuration bounds a hand-edited settings file. Past this the splash
// stops reading as an opening and starts reading as a hang.
const maxSplashDuration = time.Minute

// parseSplashDuration reads a saved length in milliseconds. Zero is "nothing
// chosen" and yields the default; anything outside the sane range is refused,
// for the caller to report the way an unsupported -length is reported.
func parseSplashDuration(ms int) (time.Duration, bool) {
	switch {
	case ms == 0:
		return splashDuration, true
	case ms < 0 || time.Duration(ms)*time.Millisecond > maxSplashDuration:
		return splashDuration, false
	default:
		return time.Duration(ms) * time.Millisecond, true
	}
}

// splashDurationLabel writes a length the way the settings screen shows it:
// "0.6s", "1.2s", "2s".
func splashDurationLabel(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'g', -1, 64) + "s"
}

// stepSplashDuration walks the offered lengths, wrapping. A value from a
// hand-edited file that is not among them starts from the nearest one, so the
// first step goes somewhere predictable rather than back to the beginning.
func stepSplashDuration(current time.Duration, delta int) time.Duration {
	at, best := 0, time.Duration(math.MaxInt64)
	for i, d := range splashDurations {
		if gap := (d - current).Abs(); gap < best {
			at, best = i, gap
		}
	}
	return splashDurations[wrap(at, delta, len(splashDurations))]
}

// setting is how a mode is written to settings.json; label is how the settings
// screen shows it. They are kept apart so the file stays terse and stable while
// the on-screen wording can be reworded freely.
func (m splashMode) setting() string {
	switch m {
	case splashKey:
		return "key"
	case splashFixed:
		return "fixed"
	default:
		return "skip"
	}
}

func (m splashMode) label() string {
	switch m {
	case splashKey:
		return "any key"
	case splashFixed:
		return "timed only"
	default:
		return "timed + skip"
	}
}

// timed reports whether the mode ends on its own.
func (m splashMode) timed() bool { return m != splashKey }

// dismissible reports whether input cuts the splash short.
func (m splashMode) dismissible() bool { return m != splashFixed }

// parseSplashMode reads a saved or overridden mode. An empty value is "nothing
// chosen", which is the default rather than an error.
func parseSplashMode(s string) (splashMode, bool) {
	switch s {
	case "":
		return splashKey, true
	case "skip":
		return splashSkip, true
	case "key":
		return splashKey, true
	case "fixed":
		return splashFixed, true
	}
	return splashKey, false
}

func (m *splashScreen) resize(w, h int) { m.width, m.height = w, h }

// fits reports whether the terminal can hold the art. A splash that does not
// fit is skipped entirely rather than clipped: it is decoration, and the frame
// it would overflow loses its top rows (see hitMap.clip), which is a far worse
// first impression than no splash at all.
//
// An unmeasured size counts as unbounded, as it does everywhere else here —
// before the first WindowSizeMsg there is nothing to measure against, and the
// check runs again the moment one arrives.
func (m *splashScreen) fits() bool {
	if m.art.Empty() {
		return false
	}
	if m.width > 0 {
		// The panel's border and padding sit either side of the art.
		if m.art.Width+2*st.metric.PanelPadX+2 > m.width {
			return false
		}
		if lipgloss.Width(tagline)+2*st.metric.PanelPadX+2 > m.width {
			return false
		}
	}
	// The body is the art, a blank line, and the tagline.
	if budget := bodyBudget(m.height); budget > 0 && budget < m.art.Height+2 {
		return false
	}
	return true
}

func (m *splashScreen) view(h *hitMap) string {
	lines := make([]string, len(m.art.Lines))
	for i, line := range m.art.Lines {
		lines[i] = st.splash.Render(line)
	}
	// Squared off first, so the outer centring moves the drawing as one block
	// rather than centring each of its lines on its own width — which would
	// shear the art apart down the middle.
	art := block(strings.Join(lines, "\n"))
	if m.mode.dismissible() {
		// The whole drawing is the button. Anywhere on a splash is where someone
		// clicks to get past it, and the help bar carries the same action.
		art = h.mark(action{kind: actSplashDismiss}, art)
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		art,
		"",
		st.muted.Render(tagline),
	)
}

func (m *splashScreen) help(h *hitMap) string {
	if !m.mode.dismissible() {
		// No hint, because there is no key that would do anything: the bar must
		// not promise something the screen will not honour.
		return renderHelp(h)
	}
	// One whole sentence rather than the usual "key label" pair, because the key
	// here is every key: there is nothing to print in the key column that would
	// read as one.
	return renderHelp(h,
		helpItem{label: "press any key to continue", act: action{kind: actSplashDismiss}},
	)
}
