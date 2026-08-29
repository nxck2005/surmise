package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/challenge"
)

const challengeRowCode = 0

// challengeScreen is either a generated code ready to copy and play, or the
// one-field entry screen for a code received from somebody else.
type challengeScreen struct {
	creating bool
	length   int
	code     challenge.Code
	entry    textField
	msg      string
	copied   bool
}

func newChallengeCreate(length int) (challengeScreen, error) {
	if length == 0 {
		length = defaultLength
	}
	m := challengeScreen{creating: true, length: length}
	if err := m.generate(); err != nil {
		return challengeScreen{}, err
	}
	return m, nil
}

func newChallengeJoin(value string) challengeScreen {
	m := challengeScreen{entry: newChallengeField()}
	m.entry.set(value)
	m.entry.begin()
	return m
}

func newChallengeField() textField {
	return newTextField(19, func(r rune) (rune, bool) {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		return r, r == '-' || r == ' ' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z'
	})
}

func (m *challengeScreen) generate() error {
	c, err := challenge.Generate(m.length)
	if err != nil {
		return err
	}
	m.code = c
	m.msg = ""
	m.copied = false
	return nil
}

func (m *challengeScreen) parsed() (challenge.Code, error) {
	if m.creating {
		return m.code, nil
	}
	return challenge.Parse(m.entry.value)
}

func (m *challengeScreen) update(msg tea.KeyPressMsg) (open, copy, back bool, err error) {
	if !m.creating {
		if m.entry.editing {
			enter := msg.String() == "enter"
			editField(&m.entry, msg)
			return enter, false, false, nil
		}
		switch msg.String() {
		case "esc", "q":
			return false, false, true, nil
		case "enter", " ":
			m.entry.begin()
		}
		return false, false, false, nil
	}

	switch msg.String() {
	case "esc", "q":
		return false, false, true, nil
	case "left", "h":
		m.length = stepLength(m.length, -1)
		return false, false, false, m.generate()
	case "right", "l":
		m.length = stepLength(m.length, 1)
		return false, false, false, m.generate()
	case "r":
		return false, false, false, m.generate()
	case "c":
		return false, true, false, nil
	case "enter", " ":
		return true, false, false, nil
	}
	return false, false, false, nil
}

func (m *challengeScreen) view(h *hitMap) string {
	if !m.creating {
		note := "paste a 16-character challenge code"
		style := st.muted
		if m.msg != "" {
			note, style = m.msg, st.err
		}
		return lipgloss.JoinVertical(lipgloss.Center,
			st.title.Render("enter code"), "",
			renderFieldRow(h, challengeRowCode, true, "challenge code", &m.entry, "not set"),
			"", style.Render(note),
		)
	}

	prev := action{kind: actChallengePrev}
	next := action{kind: actChallengeNext}
	mode := st.muted.Render("mode  ") +
		h.mark(prev, st.accent.Render(st.glyph.ValuePrev)) +
		st.text.Render(fmt.Sprintf("  %d letters  ", m.length)) +
		h.mark(next, st.accent.Render(st.glyph.ValueNext))
	code := action{kind: actChallengeCopy}
	codeStyle := st.accent
	if h.hovered(code) {
		codeStyle = st.hover(codeStyle)
	}
	note := "copy the code, then play the same board"
	if m.copied {
		note = "copy requested"
	}
	if m.msg != "" {
		note = m.msg
	}
	return lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render("new challenge"), "", mode, "",
		h.mark(code, codeStyle.Render(m.code.String())), "", st.muted.Render(note),
	)
}

func (m *challengeScreen) help(h *hitMap) string {
	if !m.creating {
		if m.entry.editing {
			return fieldHelp(h, challengeRowCode, "play")
		}
		return renderHelp(h,
			helpItem{keys: "enter", label: "type", act: action{kind: actFieldEdit, index: challengeRowCode}},
			helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
		)
	}
	return renderHelp(h,
		helpItem{keys: "←/→", label: "mode", act: action{kind: actChallengeNext}},
		helpItem{keys: "r", label: "new code", act: action{kind: actChallengeGenerate}},
		helpItem{keys: "c", label: "copy", act: action{kind: actChallengeCopy}},
		helpItem{keys: "enter", label: "play", act: action{kind: actChallengeOpen}},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
	)
}

func challengeError(err error) string {
	return strings.TrimPrefix(err.Error(), "challenge: ")
}
