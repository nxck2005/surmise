package ui

import (
	"strings"

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
	// splashSkip is the default: it goes on its own after splashDuration, and
	// any key or click cuts that short.
	splashSkip splashMode = iota
	// splashKey waits for input and never times out — a title screen proper.
	splashKey
	// splashFixed times out and ignores input, for a launch that always looks
	// the same. ctrl+c still quits, since that is handled above every screen.
	splashFixed
)

// splashModes is the order the settings screen cycles through, default first.
var splashModes = []splashMode{splashSkip, splashKey, splashFixed}

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
		return splashSkip, true
	case "skip":
		return splashSkip, true
	case "key":
		return splashKey, true
	case "fixed":
		return splashFixed, true
	}
	return splashSkip, false
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
