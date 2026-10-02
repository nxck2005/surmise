package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// escapeFrame is a model whose only content is text with escape sequences
// planted in it, drawn once and then quit.
type escapeFrame struct{ content string }

type escapeFrameDone struct{}

func (e escapeFrame) Init() tea.Cmd {
	// Long enough for the renderer to have flushed a frame; the program quits
	// itself, so the test never waits on a terminal.
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return escapeFrameDone{} })
}

func (e escapeFrame) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(escapeFrameDone); ok {
		return e, tea.Quit
	}
	return e, nil
}

func (e escapeFrame) View() tea.View {
	return tea.View{AltScreen: true, Content: e.content}
}

// TestRendererDropsEscapesInContent holds the dependency to the property the
// rest of this package leans on: an escape sequence inside a frame's content
// does not reach the terminal. bubbletea's cell renderer parses the content
// into cells, keeps SGR (as cell styles) and OSC 8 (as hyperlinks), and drops
// every other sequence — the clipboard (OSC 52), the title (OSC 0), cursor and
// erase commands (CSI) and string sequences (APC).
//
// That is the strongest of the three layers between outside text and a
// terminal: the boundary checks (game.Validate, theme.Parse) and safeText sit
// in front of it, and the browser's clipboard grant behind it. It belongs to a
// dependency, though, and a renderer that started replaying sequences — newer
// ultraviolet passes APC, DCS, SOS and PM through a cell — would change it
// without any code here changing. This test is how an upgrade says so.
//
// The profile is the browser build's: true colour, which is also the deepest a
// terminal can report, so nothing is dropped for want of colour.
func TestRendererDropsEscapesInContent(t *testing.T) {
	planted := []string{
		"\x1b]52;c;cGxhbnRlZA==\x07", // clipboard write
		"\x1b]0;planted\x07",         // window title
		"\x1b[2J",                    // erase the screen
		"\x1b_planted\x1b\\",         // APC, the hit map's own marker shape
	}
	var content strings.Builder
	for i, seq := range planted {
		// Each sequence in several positions: mid-line, at the end of a line,
		// alone on a line, after a wide rune and after a combining mark.
		content.WriteString("a" + seq + "b\n")
		content.WriteString("c" + seq + "\n")
		content.WriteString(seq + "\n")
		content.WriteString("字" + seq + "d\n")
		content.WriteString("é" + seq + "f")
		if i < len(planted)-1 {
			content.WriteString("\n")
		}
	}
	content.WriteString(planted[0]) // and at the very end of the frame

	var out bytes.Buffer
	p := tea.NewProgram(escapeFrame{content: content.String()},
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(&out),
		tea.WithWindowSize(80, 40),
		tea.WithoutSignalHandler(),
		tea.WithEnvironment([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}),
		tea.WithColorProfile(colorprofile.TrueColor),
	)
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("the program did not finish")
	}

	got := out.String()
	if !strings.Contains(got, "ab") {
		t.Fatalf("the frame was never drawn; output: %q", got)
	}
	for _, leak := range []string{"\x1b]52;", "planted", "\x1b_"} {
		if strings.Contains(got, leak) {
			t.Errorf("the renderer passed %q through from frame content", leak)
		}
	}
	// The renderer erases the screen itself once, on entering the alternate
	// screen; the planted erase would be a second.
	if n := strings.Count(got, "\x1b[2J"); n > 1 {
		t.Errorf("the output erases the screen %d times; a planted erase got through", n)
	}
}
