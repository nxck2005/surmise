package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/game"
)

// renderBoard draws every attempt row: guesses already played, the row being
// typed, then empty rows. reveal shows the answer's letters for a lost game.
func renderBoard(g *game.Game, typing string) string {
	rows := make([]string, 0, g.MaxAttempts)

	for i, guess := range g.Guesses {
		rows = append(rows, renderScoredRow(guess, g.Marks[i]))
	}

	if !g.Status.Done() {
		rows = append(rows, renderTypingRow(typing, g.Length))
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
			cells[i] = tileCorrect.Render(letter)
		case game.Present:
			cells[i] = tilePresent.Render(letter)
		default:
			cells[i] = tileAbsent.Render(letter)
		}
	}
	return joinTiles(cells)
}

func renderTypingRow(typing string, length int) string {
	cells := make([]string, length)
	for i := range cells {
		if i < len(typing) {
			cells[i] = tileActive.Render(strings.ToUpper(string(typing[i])))
		} else if i == len(typing) {
			// Mark the caret position so the player can see where input lands.
			cells[i] = tileEmpty.Foreground(colorAccent).Render("_")
		} else {
			cells[i] = tileEmpty.Render("·")
		}
	}
	return joinTiles(cells)
}

func renderEmptyRow(length int) string {
	cells := make([]string, length)
	for i := range cells {
		cells[i] = tileEmpty.Render("·")
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

// keyboardRows is the QWERTY layout used for the letter-state display.
var keyboardRows = []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}

// renderKeyboard shows the best-known state of every letter, which is the
// player's main aid for narrowing down the answer.
func renderKeyboard(states map[byte]game.Mark) string {
	// Width of the widest row, used to centre the shorter ones beneath it.
	width := lipgloss.Width(renderKeyboardRow(keyboardRows[0], states))

	rows := make([]string, len(keyboardRows))
	for i, letters := range keyboardRows {
		row := renderKeyboardRow(letters, states)
		rows[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(row)
	}
	return stackSpaced(rows)
}

func renderKeyboardRow(letters string, states map[byte]game.Mark) string {
	cells := make([]string, len(letters))
	for i := range letters {
		c := letters[i]
		letter := strings.ToUpper(string(c))

		style := keyUnused
		if mark, played := states[c]; played {
			switch mark {
			case game.Correct:
				style = keyCorrect
			case game.Present:
				style = keyPresent
			default:
				style = keyAbsent
			}
		}
		cells[i] = style.Render(letter)
	}
	// A one-space gutter keeps adjacent keycaps' backgrounds from merging into
	// a single bar, the same reason the board tiles are spaced.
	return joinTiles(cells)
}
