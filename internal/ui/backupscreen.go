package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/backup"
)

// The backup screen: two actions and a report.
//
// It exists because `-export` and `-import` cannot be typed in a browser, which
// is where a backup matters most — history there is origin-local, so clearing
// site data destroys it and nothing can bring it back. The flags stay for a
// terminal player who wants a path of their own; this is the way in that both
// builds have.
//
// The screen holds no file access of its own. Where the bytes come from and go
// to is a Transfer, which the platform supplies (see Options.Transfer), for the
// same reason the UI never opens a path anywhere else.

// Transfer moves one backup file between the app and wherever the platform
// keeps files: a directory natively, a download and a file picker in a browser.
//
// A nil Transfer means this build cannot move files at all, and the menu offers
// no backup row rather than a row that fails when it is pressed.
type Transfer interface {
	// Save writes a finished archive and returns where it went, in words the
	// player can act on: a path natively, a file name in a browser.
	Save(b []byte) (where string, err error)

	// Load asks for an archive.
	//
	// It may block — a browser's picker answers only when the player has
	// chosen — so the UI calls it from a tea.Cmd and never from Update. A
	// player who cancels gets (nil, "", nil): choosing nothing is not a
	// failure, and must not be reported as one.
	Load() (b []byte, from string, err error)
}

// The two rows, in display order.
const (
	backupRowSave = iota
	backupRowLoad
	backupRows
)

// backupScreen is the state of the screen: which row the cursor is on, whether
// a load is still waiting on the player, and what the last action did.
type backupScreen struct {
	cursor int

	// waiting is set while a load is out with the platform. A browser's file
	// picker can sit open for as long as the player likes, and the screen has
	// to say why nothing is happening.
	waiting bool

	// report is what the last action did, one line per thing that changed, and
	// failure is why it did not happen. They are exclusive: an action sets one
	// and clears the other, so the screen never shows a stale success beside a
	// fresh error.
	report  []string
	failure string
}

// reset clears the last report. The screen is opened fresh every time — a
// report from an hour ago says nothing about what is on the machine now — the
// same reason the how-to screen opens on its first page.
func (b *backupScreen) reset() {
	b.cursor = backupRowSave
	b.waiting = false
	b.report = nil
	b.failure = ""
}

// saved reports a written archive.
func (b *backupScreen) saved(where string) {
	b.waiting = false
	b.failure = ""
	b.report = []string{"saved to " + where}
}

// loaded reports a restore, in the terms a player would ask about it. It is the
// screen's copy of what `-import` prints, and it says the same things in the
// same order.
func (b *backupScreen) loaded(res backup.Result, themesAdded int) {
	b.waiting = false
	b.failure = ""

	if !res.Any() && themesAdded == 0 {
		b.report = []string{"nothing new in that backup", "this install already has all of it"}
		return
	}

	var lines []string
	if res.PuzzlesAdded > 0 {
		line := fmt.Sprintf("restored %s", countOf(res.PuzzlesAdded, "puzzle"))
		if res.PuzzlesKept > 0 {
			line += fmt.Sprintf(", kept %s already here", countOf(res.PuzzlesKept, "puzzle"))
		}
		lines = append(lines, line)
	}
	if len(res.SettingsFilled) > 0 {
		lines = append(lines, "filled in "+strings.Join(res.SettingsFilled, ", "))
	}
	if res.PlaytimeAdded > 0 {
		lines = append(lines, "time played is now the larger figure")
	}
	if themesAdded > 0 {
		lines = append(lines, "added "+countOf(themesAdded, "theme"))
	}
	b.report = lines
}

// refused reports a file that could not be used. The error is the player's to
// read: backup.Read's refusals name what is wrong with the file.
func (b *backupScreen) refused(err error) {
	b.waiting = false
	b.report = nil
	b.failure = err.Error()
}

// cancelled reports a picker the player closed without choosing. It is not a
// failure, so it does not go on the error line.
func (b *backupScreen) cancelled() {
	b.waiting = false
	b.failure = ""
	b.report = []string{"no file chosen"}
}

func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// update answers the keys. It returns the row to act on, or none.
func (b *backupScreen) update(msg tea.KeyPressMsg) (row int, act bool) {
	switch msg.String() {
	case "up", "k":
		b.cursor = max(b.cursor-1, 0)
	case "down", "j":
		b.cursor = min(b.cursor+1, backupRows-1)
	case "enter", " ":
		return b.cursor, true
	}
	return 0, false
}

// point selects a row, for the pointer to move the cursor with.
func (b *backupScreen) point(i int) {
	if i >= 0 && i < backupRows {
		b.cursor = i
	}
}

// backupBlurb is the rule, in the player's terms. It is on the screen rather
// than in the docs because it is the whole reason the load button is safe to
// press: nothing here can cost them anything.
var backupBlurb = []string{
	"everything you have played, in one file.",
	"a load only ever adds — nothing already",
	"here is replaced or removed.",
}

func (b *backupScreen) view(h *hitMap) string {
	lines := make([]string, 0, len(backupBlurb))
	for _, l := range backupBlurb {
		lines = append(lines, st.muted.Render(l))
	}
	blurb := strings.Join(lines, "\n")

	rows := strings.Join([]string{
		b.renderRow(h, backupRowSave, "save a backup", action{kind: actBackupSave}),
		b.renderRow(h, backupRowLoad, "load a backup", action{kind: actBackupLoad}),
	}, "\n")

	sections := []string{blurb, "", block(rows)}
	if note := b.note(); note != "" {
		sections = append(sections, "", note)
	}

	return titled("backup", lipgloss.JoinVertical(lipgloss.Center, sections...))
}

// note is the report, the failure, or the waiting line — whichever the last
// action left. It is padded to the width of the blurb so that acting on a row
// does not resize the panel around it.
func (b *backupScreen) note() string {
	var (
		lines []string
		style = st.muted
	)
	switch {
	case b.waiting:
		lines = []string{"waiting for a file…"}
	case b.failure != "":
		lines, style = []string{b.failure}, st.err
	case len(b.report) > 0:
		lines = b.report
	default:
		return ""
	}

	width := 0
	for _, l := range backupBlurb {
		if w := lipgloss.Width(l); w > width {
			width = w
		}
	}
	box := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = box.Render(style.Render(l))
	}
	return strings.Join(out, "\n")
}

// renderRow draws one action, marked whole so the click target is the row
// rather than the word in it.
func (b *backupScreen) renderRow(h *hitMap, row int, label string, a action) string {
	style := st.muted
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	trail := strings.Repeat(" ", lipgloss.Width(st.glyph.CursorRight))
	if row == b.cursor {
		prefix = st.cursor.Render(st.glyph.Cursor)
		trail = st.cursor.Render(st.glyph.CursorRight)
		style = st.accent
	}
	if h.hovered(a) {
		style = st.hover(style)
	}
	return h.mark(a, prefix+style.Render(label)+trail)
}

func (b *backupScreen) help(h *hitMap) string {
	act := action{kind: actBackupSave}
	if b.cursor == backupRowLoad {
		act = action{kind: actBackupLoad}
	}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "enter", label: "do it", act: act},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
