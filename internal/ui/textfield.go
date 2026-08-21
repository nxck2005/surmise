package ui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// textField is a staged single-line editor: enter begins, enter keeps, esc puts
// back what was there before. It is the profile name's editor generalised, so
// that a custom puzzle's secret word types into the same code the profile name
// does and the two cannot drift apart.
//
// Staging is the point. A cycling setting saves on every step because a wrong
// step is one step back, but half-typed text is not a value anybody meant, so
// nothing leaves the field until it is committed.
//
// Width is counted in display cells rather than bytes, because what the cap
// protects is a fixed-width cell in a terminal, not a buffer.
type textField struct {
	value   string // the committed text
	before  string // what value was when editing began, for esc
	editing bool

	// max is the width in display cells the value may reach.
	max int
	// filter decides which runes may be typed, and may fold one into another
	// (the profile name folds every kind of space into a plain one). A rune it
	// refuses is dropped rather than ending the input, so pasting a name with a
	// tab in it keeps the name.
	filter func(rune) (rune, bool)
}

func newTextField(max int, filter func(rune) (rune, bool)) textField {
	return textField{max: max, filter: filter}
}

// set replaces the committed value, cleaning it on the way in. It is for values
// arriving from outside the screen — a settings file somebody has edited by
// hand — and it abandons any edit in progress.
func (f *textField) set(v string) {
	f.value = f.sanitize(v)
	f.before = ""
	f.editing = false
}

// begin starts an edit, remembering what to put back if it is abandoned.
func (f *textField) begin() {
	if f.editing {
		return
	}
	f.before = f.value
	f.editing = true
}

// finish ends an edit and reports whether the committed value actually changed,
// which is what tells the caller there is anything to save.
func (f *textField) finish(save bool) bool {
	if !f.editing {
		return false
	}
	f.editing = false
	if !save {
		f.value = f.before
		return false
	}
	f.value = f.sanitize(f.value)
	return f.value != f.before
}

// clear empties the field outright, edit and undo history included. It is for a
// value that must not survive the screen it was typed on.
func (f *textField) clear() {
	f.value, f.before, f.editing = "", "", false
}

// deleteRune erases the last rune, not the last byte.
func (f *textField) deleteRune() {
	if !f.editing || f.value == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(f.value)
	f.value = f.value[:len(f.value)-size]
}

// typeText appends what a keypress carried. It reads Text rather than a single
// rune so a paste or an input method delivering several runes at once arrives
// whole.
func (f *textField) typeText(text string) {
	if !f.editing || text == "" {
		return
	}
	var b strings.Builder
	b.Grow(len(f.value) + len(text))
	b.WriteString(f.value)
	width := lipgloss.Width(f.value)
	for _, r := range text {
		r, ok := f.filter(r)
		if !ok {
			continue
		}
		runeWidth := lipgloss.Width(string(r))
		if width+runeWidth > f.max {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	f.value = b.String()
}

// editField is the keyboard half of an edit in progress: enter keeps, esc puts
// back, backspace erases, and everything else is text. It lives here rather than
// on each screen so that every field answers the same keys, and it reports
// whether the committed value changed.
//
// It takes msg.Text rather than msg.String() so that a paste or an input method
// delivering several runes at once arrives whole.
func editField(f *textField, msg tea.KeyPressMsg) (changed bool) {
	switch msg.String() {
	case "esc":
		f.finish(false)
	case "enter":
		return f.finish(true)
	case "backspace":
		f.deleteRune()
	default:
		f.typeText(msg.Text)
	}
	return false
}

// fieldHelp is the help bar while a field owns text input. Every control the
// editor answers to is a button, which is what keeps a text field playable with
// the mouse alone. The verb differs by screen — a setting is saved, a secret is
// handed over — so the caller supplies it.
func fieldHelp(h *hitMap, row int, keep string) string {
	return renderHelp(h,
		helpItem{keys: "⌫", label: "erase", act: action{kind: actFieldBackspace, index: row}},
		helpItem{keys: "enter", label: keep, act: action{kind: actFieldDone, index: row}},
		helpItem{keys: "esc", label: "cancel", act: action{kind: actFieldCancel, index: row}},
	)
}

// sanitize is what protects the terminal and a fixed-width row from text that
// did not come from typeText — a hand-edited settings file, most of all.
func (f *textField) sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	width := 0
	for _, r := range s {
		r, ok := f.filter(r)
		if !ok {
			continue
		}
		runeWidth := lipgloss.Width(string(r))
		if width+runeWidth > f.max {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	return strings.TrimSpace(b.String())
}

// display is what the row shows: the text, with a caret while it is being
// edited, or the placeholder when there is nothing and nobody is typing.
func (f *textField) display(placeholder string) string {
	if f.editing {
		return f.value + st.glyph.Caret
	}
	if f.value == "" {
		return placeholder
	}
	return f.value
}
