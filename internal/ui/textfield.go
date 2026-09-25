package ui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/store"
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
// protects is a fixed-width cell in a terminal, not a buffer — and a byte cap
// rides along beside it, because a cell cap on its own is not a bound. See
// fieldBuilder.add.
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

// fieldBuilder accumulates a value while holding it to both of the field's
// caps. It exists so the two places that fill a field — typing, and cleaning
// what arrived from a settings file — cannot come to different answers about
// what "long enough" means.
type fieldBuilder struct {
	b     strings.Builder
	width int
}

// add puts r in if it still fits, and reports whether it did. A rune that does
// not fit ends the value rather than being skipped, so the cap is a prefix of
// what was offered and not a value with holes in it.
func (fb *fieldBuilder) add(r rune, maxCells int) bool {
	w := lipgloss.Width(string(r))
	// The byte cap is what a cell cap alone is not. A zero-width rune — a
	// combining mark, which every IsPrint filter admits — never advances width,
	// so a value of any length can pass a cell cap of any size. Bytes is also
	// the honest unit: this value is persisted, and a cell is not a thing a
	// file can measure.
	if fb.width+w > maxCells || fb.b.Len()+utf8.RuneLen(r) > store.MaxSettingFieldBytes {
		return false
	}
	fb.b.WriteRune(r)
	fb.width += w
	return true
}

// typeText appends what a keypress carried. It reads Text rather than a single
// rune so a paste or an input method delivering several runes at once arrives
// whole.
func (f *textField) typeText(text string) {
	if !f.editing || text == "" {
		return
	}
	var fb fieldBuilder
	fb.b.Grow(len(f.value) + len(text))
	fb.b.WriteString(f.value)
	fb.width = lipgloss.Width(f.value)
	for _, r := range text {
		r, ok := f.filter(r)
		if !ok {
			continue
		}
		if !fb.add(r, f.max) {
			break
		}
	}
	f.value = fb.b.String()
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
//
// It is also the last thing between a settings file somebody else wrote and a
// frame, so it shares the fieldBuilder that typeText uses: a name long enough
// to break the layout is refused the same way whether it was typed or imported.
func (f *textField) sanitize(s string) string {
	var fb fieldBuilder
	fb.b.Grow(len(s))
	for _, r := range s {
		r, ok := f.filter(r)
		if !ok {
			continue
		}
		if !fb.add(r, f.max) {
			break
		}
	}
	return strings.TrimSpace(fb.b.String())
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
