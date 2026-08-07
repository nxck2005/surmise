package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
)

// renderBoard draws every attempt row: guesses already played, the row being
// typed, then empty rows. reveal shows the answer's letters for a lost game.
func renderBoard(g *game.Game, typing string, h *hitMap) string {
	rows := make([]string, 0, g.MaxAttempts)

	for i, guess := range g.Guesses {
		rows = append(rows, renderScoredRow(guess, g.Marks[i]))
	}

	if !g.Status.Done() {
		rows = append(rows, renderTypingRow(typing, g.Length, h))
	}

	for len(rows) < g.MaxAttempts {
		rows = append(rows, renderEmptyRow(g.Length))
	}

	return stackSpaced(rows)
}

// stackSpaced joins rows vertically with a blank line between each, so guess
// rows and keyboard rows breathe instead of stacking flush.
func stackSpaced(rows []string) string {
	spaced := make([]string, 0, len(rows)*2-1)
	for i, r := range rows {
		if i > 0 {
			spaced = append(spaced, "")
		}
		spaced = append(spaced, r)
	}
	return lipgloss.JoinVertical(lipgloss.Center, spaced...)
}

func renderScoredRow(guess string, marks []game.Mark) string {
	cells := make([]string, len(guess))
	for i := range guess {
		letter := strings.ToUpper(string(guess[i]))
		switch marks[i] {
		case game.Correct:
			cells[i] = st.tileCorrect.Render(letter)
		case game.Present:
			cells[i] = st.tilePresent.Render(letter)
		default:
			cells[i] = st.tileAbsent.Render(letter)
		}
	}
	return joinTiles(cells)
}

func renderTypingRow(typing string, length int, h *hitMap) string {
	cells := make([]string, length)
	for i := range cells {
		if i < len(typing) {
			// A typed letter is a click target: clicking it erases the row back
			// to that slot, which is how a mouse edits a mistake mid-word.
			trim := action{kind: actTrim, index: i}
			style := st.tileActive
			if h.hovered(trim) {
				style = st.hover(style)
			}
			cells[i] = h.mark(trim, style.Render(strings.ToUpper(string(typing[i]))))
		} else if i == len(typing) {
			// Mark the caret position so the player can see where input lands.
			cells[i] = st.caret.Render(st.glyph.Caret)
		} else {
			cells[i] = st.tileEmpty.Render(st.glyph.Empty)
		}
	}
	return joinTiles(cells)
}

func renderEmptyRow(length int) string {
	cells := make([]string, length)
	for i := range cells {
		cells[i] = st.tileEmpty.Render(st.glyph.Empty)
	}
	return joinTiles(cells)
}

// joinTiles separates board cells with a gutter. Without it the filled
// backgrounds of adjacent tiles run together into one solid bar instead of
// reading as distinct letters.
func joinTiles(cells []string) string {
	spaced := make([]string, 0, len(cells)*2-1)
	for i, c := range cells {
		if i > 0 {
			spaced = append(spaced, " ")
		}
		spaced = append(spaced, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, spaced...)
}

// legendSample is the letter the legend shows in each of the three scored
// states. Repeating one letter is what makes the row read as "the same letter,
// three answers" rather than as a word.
const legendSample = "A"

// renderLegend explains the tile colours. Themes are free to repaint correct,
// present and absent — several bundled ones drop the green/yellow convention
// entirely — so the board's own colours are not self-explanatory.
//
// It renders through the very styles renderScoredRow uses, so the legend cannot
// drift from the tiles it describes: a [style.tile.correct] override or a wider
// tile_width moves both together. There is no click target here, hence no
// hitMap: the legend is a label, not a control.
func renderLegend() string {
	groups := []string{
		legendEntry(st.tileCorrect, "correct spot"),
		legendEntry(st.tilePresent, "wrong spot"),
		legendEntry(st.tileAbsent, "not in word"),
	}
	return strings.Join(groups, st.help.Render(st.glyph.Separator))
}

// legendEntry is one swatch and its label, sharing the board's tile gutter so
// the filled background does not run into the text.
func legendEntry(tile lipgloss.Style, label string) string {
	return joinTiles([]string{tile.Render(legendSample), st.muted.Render(label)})
}

// keyboardRows is the QWERTY layout used for the letter-state display.
var keyboardRows = []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}

// Enter and backspace flank the bottom row, as on a phone keyboard. They
// are what a mouse submits and deletes with, and they cost no vertical space:
// row 0 stays the widest at 59 cells, row 2 grows from 41 to 53. Their glyphs
// come from the theme, since not every font draws ⏎ and ⌫ well.

// renderKeyboard shows the best-known state of every letter, which is the
// player's main aid for narrowing down the answer. Every cap is clickable.
func renderKeyboard(states map[byte]game.Mark, h *hitMap) string {
	// Width of the widest row, used to centre the shorter ones beneath it.
	// Measured without the hit map so the throwaway render marks nothing.
	width := lipgloss.Width(renderKeyboardRow(keyboardRows[0], states, nil))

	rows := make([]string, len(keyboardRows))
	for i, letters := range keyboardRows {
		row := renderKeyboardRow(letters, states, h)
		if i == len(keyboardRows)-1 {
			row = joinTiles([]string{
				renderCommandKey(st.glyph.Enter, action{kind: actSubmit}, h),
				row,
				renderCommandKey(st.glyph.Delete, action{kind: actBackspace}, h),
			})
		}
		rows[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(row)
	}
	return stackSpaced(rows)
}

// renderCommandKey draws one of the two non-letter caps.
func renderCommandKey(label string, a action, h *hitMap) string {
	style := st.keyUnused
	if h.hovered(a) {
		style = st.hover(style)
	}
	return h.mark(a, style.Render(label))
}

func renderKeyboardRow(letters string, states map[byte]game.Mark, h *hitMap) string {
	cells := make([]string, len(letters))
	for i := range letters {
		c := letters[i]
		letter := strings.ToUpper(string(c))

		style := st.keyUnused
		if mark, played := states[c]; played {
			switch mark {
			case game.Correct:
				style = st.keyCorrect
			case game.Present:
				style = st.keyPresent
			default:
				style = st.keyAbsent
			}
		}

		typeIt := action{kind: actLetter, letter: c}
		if h.hovered(typeIt) {
			style = st.hover(style)
		}
		cells[i] = h.mark(typeIt, style.Render(letter))
	}
	// A one-space gutter keeps adjacent keycaps' backgrounds from merging into
	// a single bar, the same reason the board tiles are spaced.
	return joinTiles(cells)
}
