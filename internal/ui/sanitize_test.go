package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeTextRepairsControlCharacters(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain text", "no problems here", "no problems here"},
		{"escape sequence", "\x1b[2Jclear", "�[2Jclear"},
		{"osc terminated by bell", "\x1b]52;c;x\x07", "�]52;c;x�"},
		{"newline", "two\nlines", "two�lines"},
		{"non-ascii is left alone", "josé ⏎", "josé ⏎"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeText(c.in); got != c.want {
				t.Errorf("safeText(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The status line is the app's catch-all for anything that went wrong, so a
// control character that arrives inside an error must not become one on the
// frame — the message survives, the escape does not.
func TestErrorLineNeutralisesControlCharacters(t *testing.T) {
	m := newModel(t)
	m.err = errors.New("boom \x1b[2J clear")
	view := m.View().Content
	if strings.Contains(view, "\x1b[2J") {
		t.Error("the error line carried an injected escape sequence into the frame")
	}
	if !strings.Contains(view, "boom") {
		t.Error("the error line dropped the message it was reporting")
	}
}

// -data is the shell's to choose, so the about screen shows a path nothing has
// validated. It must reach the frame as text, not as an escape.
func TestAboutScreenNeutralisesTheDataPath(t *testing.T) {
	a := aboutScreen{rows: aboutRows("/tmp/\x1b]0;pwned\x07")}
	view := a.view(&hitMap{})
	if strings.Contains(view, "\x1b]0;") || strings.Contains(view, "\x07") {
		t.Error("the about screen carried an injected escape sequence into the frame")
	}
	if !strings.Contains(view, "pwned") {
		t.Error("the about screen dropped the path it was showing")
	}
}

// The backup report names a path under -data. Every line of it is filtered,
// not only the ones known today to carry outside text.
func TestBackupReportNeutralisesControlCharacters(t *testing.T) {
	var b backupScreen
	b.saved("/tmp/\x1b]0;pwned\x07/backup.json", nil)
	note := b.note()
	if strings.Contains(note, "\x1b]0;") || strings.Contains(note, "\x07") {
		t.Error("the backup report carried an injected escape sequence into the frame")
	}
	if !strings.Contains(note, "backup.json") {
		t.Error("the backup report dropped the path it was reporting")
	}
}
