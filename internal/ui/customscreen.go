package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/words"
)

// customScreen sets a puzzle by hand: one person types a secret word, then
// hands the terminal to another to solve it. No server, no protocol — the whole
// feature is a word and a hand-over.
//
// The word is shown while it is typed, deliberately. The person choosing it has
// to see a typo, and the person about to guess is meant to look away; hiding it
// would protect against the wrong reader and cost the right one. What matters is
// that it is gone the moment the board opens, which is what confirm guarantees.
type customScreen struct {
	length  int
	secret  textField
	anyWord bool

	cursor int
	// msg is the refusal shown under the rows. It never quotes the word: the
	// screen is looked at by whoever is about to guess.
	msg string
}

// The rows, in display order. Their order is the index carried by a click, and
// customRowSecret is the row the text field lives on.
const (
	customRowMode = iota
	customRowSecret
	customRowAnyWord
	customRows
)

// newCustomScreen opens the screen on a mode, with nothing typed.
func newCustomScreen(length int) customScreen {
	m := customScreen{length: length}
	if !words.SupportedLength(m.length) {
		m.length = defaultLength
	}
	m.secret = newSecretField(m.length)
	return m
}

// newSecretField is the secret word's editor: plain letters, and no more of them
// than the board has tiles. Nothing else can reach a board, so nothing else can
// be typed.
func newSecretField(length int) textField {
	return newTextField(length, func(r rune) (rune, bool) {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		return r, r >= 'a' && r <= 'z'
	})
}

// enabled mirrors the settings screen: while the field owns text input, every
// other row is inert, so a typed letter cannot change the mode underneath it.
func (m *customScreen) enabled(row int) bool {
	if m.secret.editing {
		return row == customRowSecret
	}
	return true
}

// update edits the word and steps the other two rows. It reports whether the
// player asked to start the game or to leave.
func (m *customScreen) update(msg tea.KeyPressMsg) (start, back bool) {
	if m.secret.editing {
		enter := msg.String() == "enter"
		editField(&m.secret, msg)
		// enter finishes the word and hands over in one stroke: typing the last
		// letter and pressing enter is the whole gesture, and a second enter to
		// confirm would only invite the chooser to sit on a screen showing the
		// word. A word that cannot be used keeps the screen and says why.
		if enter {
			m.msg = m.check()
			return m.msg == "", false
		}
		return false, false
	}

	switch msg.String() {
	case "esc", "q":
		return false, true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		m.cycle(-1)
	case "right", "l":
		m.cycle(1)
	case "enter", " ":
		switch m.cursor {
		case customRowSecret:
			m.begin()
		case customRowAnyWord:
			m.cycle(1)
		default:
			return true, false
		}
	}
	return false, false
}

func (m *customScreen) begin() {
	m.cursor = customRowSecret
	m.msg = ""
	m.secret.begin()
}

func (m *customScreen) move(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), customRows-1)
}

func (m *customScreen) point(row int) {
	if row >= 0 && row < customRows && m.enabled(row) {
		m.cursor = row
	}
}

// cycle steps the row under the cursor. Changing the mode clears the word,
// because a five-letter secret is not a four-letter one — silently truncating it
// would hand over a word nobody chose.
func (m *customScreen) cycle(delta int) {
	switch m.cursor {
	case customRowMode:
		length := stepLength(m.length, delta)
		if length == m.length {
			return
		}
		m.length = length
		m.secret = newSecretField(m.length)
		m.msg = ""
	case customRowAnyWord:
		m.anyWord = !m.anyWord
		m.msg = ""
	}
}

// check reports why the word cannot be used yet, or "" when it can.
func (m *customScreen) check() string {
	word := m.secret.value
	switch {
	case len(word) != m.length:
		return fmt.Sprintf("needs %d letters", m.length)
	case !m.anyWord && !words.IsValidGuess(m.length, word):
		// The wording the board uses for a rejected guess, plus the way out.
		return "not in word list — allow any word to use it"
	default:
		return ""
	}
}

// clear forgets the word. It is called at the hand-over, before the board is
// shown, so that no later frame can redraw what was typed.
func (m *customScreen) clear() {
	m.secret.clear()
	m.msg = ""
}

func (m *customScreen) view(h *hitMap) string {
	rows := []string{
		m.renderRow(h, customRowMode, "mode", fmt.Sprintf("%d letters", m.length)),
		renderFieldRow(h, customRowSecret, m.cursor == customRowSecret,
			"answer", &m.secret, "not set"),
		m.renderRow(h, customRowAnyWord, "any word", onOff(m.anyWord)),
	}

	note := m.msg
	style := st.err
	if note == "" {
		note, style = m.note(), st.muted
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render("custom"),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		lipgloss.NewStyle().Width(noteWidth()).Align(lipgloss.Center).
			Render(style.Render(note)),
	)
}

// renderRow is the settings screen's cycling row, in the same two columns, so
// the two screens read as one app.
func (m *customScreen) renderRow(h *hitMap, row int, label, value string) string {
	prev := action{kind: actCustomPrev, index: row}
	next := action{kind: actCustomNext, index: row}

	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	labelStyle, valueStyle := st.muted, st.muted
	if row == m.cursor {
		prefix = st.cursor.Render(st.glyph.Cursor)
		labelStyle, valueStyle = st.text, st.accent
	}

	arrow := func(a action, glyph string) string {
		if !m.enabled(row) {
			return strings.Repeat(" ", lipgloss.Width(glyph))
		}
		style := st.muted
		if h.hovered(a) {
			style = st.hover(st.accent)
		}
		return h.mark(a, style.Render(glyph))
	}
	if h.hovered(next) {
		valueStyle = st.hover(valueStyle)
	}

	cell := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center).
		Render(valueStyle.Render(value))
	if m.enabled(row) {
		cell = h.mark(next, cell)
	}
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
		arrow(prev, st.glyph.ValuePrev) +
		cell +
		arrow(next, st.glyph.ValueNext)
}

// customNotes are the one-line explanations, sized like the settings screen's so
// moving the cursor does not resize the panel.
var customNotes = struct{ mode, secret, listed, anyWord, ready string }{
	mode:    "how long the word is, and how many tries it gets",
	secret:  "answer to the puzzle",
	listed:  "the word must be one the guesser could type",
	anyWord: "names and anything else are allowed",
	ready:   "enter hands over — the word leaves the screen",
}

func (m *customScreen) note() string {
	if m.secret.editing {
		return customNotes.secret
	}
	switch m.cursor {
	case customRowMode:
		return customNotes.mode
	case customRowAnyWord:
		if m.anyWord {
			return customNotes.anyWord
		}
		return customNotes.listed
	default:
		if m.check() == "" {
			return customNotes.ready
		}
		return customNotes.secret
	}
}

func (m *customScreen) help(h *hitMap) string {
	if m.secret.editing {
		return fieldHelp(h, customRowSecret, "keep")
	}
	items := []helpItem{{keys: "↑/↓", label: "move"}}
	if m.cursor == customRowSecret {
		items = append(items, helpItem{keys: "enter", label: "type",
			act: action{kind: actFieldEdit, index: customRowSecret}})
	} else {
		items = append(items, helpItem{keys: "←/→", label: "change",
			act: action{kind: actCustomNext, index: m.cursor}})
	}
	// Hand over is always a button, whichever row the cursor is on. It is the
	// one thing this screen is for, so it must never be a target the pointer has
	// to go looking for — and clicking it when the word is not usable says why,
	// exactly as enter does.
	return renderHelp(h, append(items,
		helpItem{keys: "enter", label: "hand over", act: action{kind: actCustomStart}},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}})...)
}
