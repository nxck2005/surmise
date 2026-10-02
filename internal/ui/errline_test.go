package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/store"
)

// The error line belongs to the last thing that went wrong. It used to stay for
// the rest of the session, drawn on every screen after the one that raised it.
func TestErrorLineClearsOnTheNextInput(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{Motion: motionOffName, Theme: "no such theme"})
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	if m.err == nil {
		t.Fatal("an unknown theme was not reported")
	}

	// The key that dismisses the splash is not a key spent reading the error,
	// so the line is still there on the screen the splash reveals.
	if m.screen != screenSplash {
		t.Fatal("the app did not open on the splash")
	}
	m.Update(key("x"))
	if m.screen == screenSplash {
		t.Fatal("the splash did not dismiss")
	}
	if !strings.Contains(m.View().Content, "no theme named") {
		t.Fatal("the startup error did not survive dismissing the splash")
	}

	m.Update(key("esc")) // the board's way to the menu: the player moved on
	if m.err != nil {
		t.Errorf("err = %v after the next key, want it cleared", m.err)
	}
	if strings.Contains(m.View().Content, "no theme named") {
		t.Error("the error line is still drawn after the player acted")
	}
}

// A value echoed into the error line is cut short. Nothing in a frame truncates
// horizontally, so the full value — a browser query parameter can be as long as
// a URL — used to widen the whole panel to its length.
func TestErrorLineDoesNotEchoALongValue(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("A", 10_000)
	for _, opts := range []Options{
		{Theme: long},
		{Motion: long},
		{Splash: long},
		{Day: long},
	} {
		opts.Motion = firstNonEmpty(opts.Motion, motionOffName)
		m := New(s, nil, opts)
		if m.err == nil {
			t.Fatalf("%+.20v: no error reported", opts)
		}
		m.screen = screenMenu
		m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
		if w := lipgloss.Width(m.View().Content); w > testWidth {
			t.Errorf("error %.60q widened the frame to %d cells", m.err.Error(), w)
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
