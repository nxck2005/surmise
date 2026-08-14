package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
)

// renderBoard draws every attempt row: guesses already played, the row being
// typed, then empty rows.
//
// a may be nil, which draws the settled board — every row scored, nothing
// flashing. That is what a caller with no animation state gets, and it is the
// frame every layout test compares against.
func renderBoard(g *game.Game, typing string, h *hitMap, a *anims, now time.Time) string {
	rows := make([]string, 0, g.MaxAttempts)

	for i, guess := range g.Guesses {
		if shown, revealing := a.reveal(now, g.ID, i); revealing {
			rows = append(rows, renderRevealingRow(guess, g.Marks[i], shown))
			continue
		}
		// A won row is walked once by a light after it has turned. It renders
		// through the same scored tiles either way, so the settled row is the
		// row this board always drew.
		if lit, celebrating := a.celebrating(now, g.ID, i); celebrating {
			rows = append(rows, renderCelebratingRow(guess, g.Marks[i], lit))
			continue
		}
		rows = append(rows, renderScoredRow(guess, g.Marks[i]))
	}

	if !g.Status.Done() {
		rows = append(rows, renderTypingRow(typing, g.Length, h, a.rejected(now)))
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
		cells[i] = tileStyle(marks[i]).Render(strings.ToUpper(string(guess[i])))
	}
	return joinTiles(cells)
}

// renderCelebratingRow draws a solved row with one tile lit. The light is the
// tile's own background lifted, not a colour of its own, so every theme gets
// the effect without naming an element for it — and a terminal too poor to
// blend gets the settled row, because lift gives its input back.
func renderCelebratingRow(guess string, marks []game.Mark, lit int) string {
	cells := make([]string, len(guess))
	for i := range guess {
		letter := strings.ToUpper(string(guess[i]))
		style := tileStyle(marks[i])
		if i == lit {
			style = style.Background(lift(style.GetBackground(), 0.35))
		}
		cells[i] = style.Render(letter)
	}
	return joinTiles(cells)
}

// tileStyle is the tile a mark is drawn with. It is the one place the three are
// chosen, so the board, the reveal, the celebration and the legend cannot
// disagree about what a mark looks like.
func tileStyle(mark game.Mark) lipgloss.Style {
	switch mark {
	case game.Correct:
		return st.tileCorrect
	case game.Present:
		return st.tilePresent
	default:
		return st.tileAbsent
	}
}

// renderRevealingRow draws a scored row part-way through its reveal: the first
// shown tiles in the colours they scored, the rest still wearing the style they
// were typed in. Both come from tile(), so both are exactly TileWidth wide — the
// flip is a repaint, and the row resolves from what you wrote into what it
// scored without anything moving.
func renderRevealingRow(guess string, marks []game.Mark, shown int) string {
	if shown >= len(guess) {
		return renderScoredRow(guess, marks)
	}
	cells := make([]string, len(guess))
	for i := range guess {
		letter := strings.ToUpper(string(guess[i]))
		if i >= shown {
			cells[i] = st.tileActive.Render(letter)
			continue
		}
		cells[i] = tileStyle(marks[i]).Render(letter)
	}
	return joinTiles(cells)
}

// renderTypingRow draws the row being written. rejected flashes it in the error
// colour: tileActive fills nothing, so this repaints the letters and leaves the
// row exactly where it was. A positional shake would read better for a moment
// and cost more than it is worth — the typed tiles are click targets (actTrim
// below), and a target sliding under a stationary pointer trims to a slot the
// player was not pointing at.
func renderTypingRow(typing string, length int, h *hitMap, rejected bool) string {
	cells := make([]string, length)
	for i := range cells {
		if i < len(typing) {
			// A typed letter is a click target: clicking it erases the row back
			// to that slot, which is how a mouse edits a mistake mid-word.
			trim := action{kind: actTrim, index: i}
			style := st.tileActive
			if rejected {
				style = style.Foreground(st.err.GetForeground())
			}
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
func renderKeyboard(states map[byte]game.Mark, h *hitMap, a *anims, now time.Time) string {
	// Width of the widest row, used to centre the shorter ones beneath it.
	// Measured without the hit map so the throwaway render marks nothing, and
	// without the animation so a pulsing cap cannot change the measurement.
	width := lipgloss.Width(renderKeyboardRow(keyboardRows[0], states, nil, nil, now))

	rows := make([]string, len(keyboardRows))
	for i, letters := range keyboardRows {
		row := renderKeyboardRow(letters, states, h, a, now)
		if i == len(keyboardRows)-1 {
			row = joinTiles([]string{
				renderCommandKey(st.glyph.Enter, action{kind: actSubmit}, h, a, now),
				row,
				renderCommandKey(st.glyph.Delete, action{kind: actBackspace}, h, a, now),
			})
		}
		rows[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(row)
	}
	return stackSpaced(rows)
}

// renderCommandKey draws one of the two non-letter caps.
func renderCommandKey(label string, act action, h *hitMap, a *anims, now time.Time) string {
	style := st.keyUnused
	if h.hovered(act) || a.pressed(now, 0, act.kind) {
		style = st.hover(style)
	}
	return h.mark(act, style.Render(label))
}

// renderKeyboardRow draws one row of caps. A cap just struck wears the hover
// cue for a moment: the pulse and the pointer say the same thing — "this one" —
// so they are deliberately the same cue rather than a second one to learn.
func renderKeyboardRow(letters string, states map[byte]game.Mark, h *hitMap, a *anims, now time.Time) string {
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
		if h.hovered(typeIt) || a.pressed(now, c, actLetter) {
			style = st.hover(style)
		}
		cells[i] = h.mark(typeIt, style.Render(letter))
	}
	// A one-space gutter keeps adjacent keycaps' backgrounds from merging into
	// a single bar, the same reason the board tiles are spaced.
	return joinTiles(cells)
}
