package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/theme"
)

// themeScreen picks the look. Moving the cursor applies the theme immediately
// rather than on enter: the whole point of a theme is what it looks like, and
// the fastest way to show that is to just show it. Enter keeps the previewed
// theme; esc puts back the one that was saved.
type themeScreen struct {
	entries []theme.Entry
	cursor  int
	offset  int

	// height is the terminal's, pushed down by the root; zero means unmeasured.
	height int

	// saved is the committed choice, restored when the player backs out.
	saved string
}

func (m *themeScreen) resize(h int) {
	m.height = h
	m.clampOffset()
}

// rows is the size of the visible window.
func (m *themeScreen) rows() int { return windowRows(m.height, len(m.entries)) }

func (m *themeScreen) reload(lib *theme.Library, current string) {
	m.entries = lib.Entries()
	m.saved = current
	m.cursor, m.offset = 0, 0
	for i, e := range m.entries {
		if e.Name == current {
			m.cursor = i
		}
	}
	m.clampOffset()
}

// refresh takes a reloaded library without moving the player. Unlike reload it
// keeps the cursor where it is, and keeps it by **name**: a theme file added
// above the highlighted one shifts every index below it, and re-homing on the
// index would slide the selection out from under the pointer mid-edit.
//
// The theme that was highlighted may have been the one just deleted, so the
// cursor is clamped first and only then re-found.
func (m *themeScreen) refresh(lib *theme.Library) {
	want := ""
	if e, ok := m.selected(); ok {
		want = e.Name
	}

	m.entries = lib.Entries()
	m.cursor = min(max(m.cursor, 0), max(len(m.entries)-1, 0))
	for i, e := range m.entries {
		if e.Name == want {
			m.cursor = i
			break
		}
	}

	// A list that shrank can leave the window past its end, which clampOffset
	// cannot fix — it only ever pulls the window towards the cursor. This is
	// the clamp scroll uses.
	m.offset = min(max(m.offset, 0), max(len(m.entries)-m.rows(), 0))
	m.clampOffset()
}

func (m *themeScreen) selected() (theme.Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return theme.Entry{}, false
	}
	return m.entries[m.cursor], true
}

// preview applies the highlighted theme. A theme that failed to load has
// nothing to apply, so the previous one stays up.
func (m *themeScreen) preview() {
	if e, ok := m.selected(); ok && e.Theme != nil {
		setTheme(e.Theme)
	}
}

// update moves the cursor, reporting whether the player committed a theme,
// asked to go back, or asked for the directory to be read again.
func (m *themeScreen) update(msg tea.KeyPressMsg) (commit, back, reload bool) {
	switch msg.String() {
	case "esc", "q":
		return false, true, false
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "home", "g":
		m.jumpTop()
	case "end", "G":
		m.jumpBottom()
	case "r":
		// The directory is polled anyway; this is for the edit the poll cannot
		// see, and for not waiting out the interval.
		return false, false, true
	case "enter", " ":
		if e, ok := m.selected(); ok && e.Theme != nil {
			return true, false, false
		}
	}
	m.preview()
	return false, false, false
}

// jumpTop and jumpBottom are the ends of the list, shared by the keys and by
// the counter's click targets.
func (m *themeScreen) jumpTop() {
	m.cursor = 0
	m.clampOffset()
}

func (m *themeScreen) jumpBottom() {
	m.cursor = max(len(m.entries)-1, 0)
	m.clampOffset()
}

func (m *themeScreen) move(delta int) {
	if len(m.entries) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.entries)-1)
	m.clampOffset()
}

// point selects the row under the pointer and previews it, so hovering the list
// flips through themes exactly the way arrowing through it does.
func (m *themeScreen) point(row int) {
	if row >= 0 && row < len(m.entries) {
		m.cursor = row
		m.clampOffset()
		m.preview()
	}
}

func (m *themeScreen) scroll(delta int) {
	m.offset = min(max(m.offset+delta, 0), max(len(m.entries)-m.rows(), 0))
}

func (m *themeScreen) clampOffset() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if rows := m.rows(); m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(m.offset, 0)
}

func (m *themeScreen) view(h *hitMap) string {
	if len(m.entries) == 0 {
		return st.title.Render("themes") + "\n\n" +
			st.muted.Render("no themes found")
	}

	var list strings.Builder
	end := min(m.offset+m.rows(), len(m.entries))
	for i := m.offset; i < end; i++ {
		list.WriteString(h.mark(action{kind: actThemeRow, index: i},
			m.renderRow(m.entries[i], i == m.cursor)))
		list.WriteString("\n")
	}

	rows := strings.TrimRight(list.String(), "\n")
	// The picker used to give no sign that the list continued past the window,
	// which is the information half of home/end having no click target.
	if len(m.entries) > m.rows() {
		rows = block(rows) + "\n\n" + scrollCounter(h, m.offset+1, end, len(m.entries))
	}

	sections := []string{
		st.title.Render("themes"),
		"",
		rows,
		"",
		renderThemePreview(),
	}
	if note := m.note(); note != "" {
		sections = append(sections, "", note)
	}
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// renderRow lays out one theme: the name, where it came from, and a tick on the
// one that is actually saved — so a player who is midway through previewing can
// still see what they will fall back to.
func (m *themeScreen) renderRow(e theme.Entry, selected bool) string {
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	nameStyle := st.muted
	if selected {
		prefix = st.cursor.Render(st.glyph.Cursor)
		nameStyle = st.text
	}

	origin := "built-in"
	if !e.Builtin() {
		origin = "custom"
	}
	if e.Err != nil || len(e.Warnings) > 0 {
		origin = "error"
	}

	mark := " "
	if e.Name == m.saved {
		mark = "•"
	}

	return prefix +
		st.accent.Render(mark) + " " +
		nameStyle.Render(fmt.Sprintf("%-24s", e.Name)) +
		st.muted.Render(origin)
}

// note explains whatever is wrong with the highlighted theme, since a file the
// picker silently skipped is a file the author cannot debug.
func (m *themeScreen) note() string {
	e, ok := m.selected()
	if !ok {
		return ""
	}
	switch {
	case e.Err != nil:
		return st.err.Render(fmt.Sprintf("%s: %v", e.Source, e.Err))
	case len(e.Warnings) > 0:
		lines := make([]string, 0, len(e.Warnings)+1)
		lines = append(lines, st.err.Render(e.Source))
		for _, w := range e.Warnings {
			lines = append(lines, st.err.Render("  "+w.String()))
		}
		return strings.Join(lines, "\n")
	case !e.Builtin():
		return st.muted.Render(e.Source)
	case e.Theme.Author != "":
		return st.muted.Render("by " + e.Theme.Author)
	}
	return ""
}

// renderThemePreview draws a sample of every tile and keycap state in the
// theme being previewed. The rest of the screen is text; without this the
// board colours — the ones that actually matter — would be invisible here.
func renderThemePreview() string {
	tiles := joinTiles([]string{
		st.tileCorrect.Render("W"),
		st.tilePresent.Render("O"),
		st.tileAbsent.Render("R"),
		st.tileActive.Render("D"),
		st.tileEmpty.Render(st.glyph.Empty),
	})
	keys := joinTiles([]string{
		st.keyUnused.Render("A"),
		st.keyCorrect.Render("B"),
		st.keyPresent.Render("C"),
		st.keyAbsent.Render("D"),
	})
	return lipgloss.JoinVertical(lipgloss.Center, tiles, "", keys)
}

func (m *themeScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "preview"},
		helpItem{keys: "enter", label: "keep", act: action{kind: actThemeRow, index: m.cursor}},
		helpItem{keys: "r", label: "reload", act: action{kind: actThemeReload}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
