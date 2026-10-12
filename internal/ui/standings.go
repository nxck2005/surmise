package ui

// The standings that ride beside a live room's board: every other player in
// the round, their latest row in the board's own colours, and their state.
//
// The column borrows height from the board rather than the shed ladder: it is
// exactly the board's height, so boardLayout.rows needs no change for it, and
// the ladder's tests stay true. Its width falls back — full to compact to a
// count on the status line — as the terminal narrows, which is what keeps a
// crowded room readable on a small one.

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
)

type standingsForm int

const (
	standingsFull    standingsForm = iota // name, latest row, state
	standingsCompact                      // name, state
	standingsHidden                       // not drawn; the status line counts instead
)

// standingsFormFor picks the widest form that fits beside a board of
// boardWidth cells in a terminal of termWidth. A zero termWidth is an
// unmeasured terminal, which is unbounded — the same rule the shed ladder
// follows — so the headless tests draw the full column.
func standingsFormFor(boardWidth, termWidth, length int) standingsForm {
	full := 8 + 1 + length + 1 + 4
	const compact = 8 + 1 + 4 // = 13
	if termWidth <= 0 {
		return standingsFull
	}
	room := termWidth - 2*st.metric.PanelPadX - 2
	if boardWidth+3+full <= room {
		return standingsFull
	}
	if boardWidth+3+compact <= room {
		return standingsCompact
	}
	return standingsHidden
}

// standingsOrder ranks the column's players, then orders each group: solved
// first (fewer guesses, then seat), those still playing (more guesses first,
// then seat), then the ones out (seat order). Time is the server's ranking's
// business, not the live view's: a round in progress carries no elapsed
// times, so within ties the lowest seat goes first.
func standingsOrder(r *roomSession) []online.PlayerView {
	var sols, plays, outs []online.PlayerView
	for _, p := range r.view.Players {
		if p.Seat == r.seat || !p.InRound {
			continue
		}
		switch {
		case p.Done && p.Solved:
			sols = append(sols, p)
		case p.Done:
			outs = append(outs, p)
		default:
			plays = append(plays, p)
		}
	}
	// Fewer guesses first within the solved; more guess counts first within
	// the playing — the one with more rows is the one the leader's row is.
	// The server's ranking, at the result, sorts by time as well.
	sols = byGuesses(sols, false)
	plays = byGuesses(plays, true)
	outs = bySeat(outs)
	out := []online.PlayerView{}
	out = append(out, sols...)
	out = append(out, plays...)
	out = append(out, outs...)
	return out
}

func byGuesses(ps []online.PlayerView, moreFirst bool) []online.PlayerView {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0; j-- {
			var first bool
			switch {
			case ps[j].Guesses == ps[j-1].Guesses:
				first = ps[j].Seat < ps[j-1].Seat
			case moreFirst:
				first = ps[j].Guesses > ps[j-1].Guesses
			default:
				first = ps[j].Guesses < ps[j-1].Guesses
			}
			if first {
				ps[j], ps[j-1] = ps[j-1], ps[j]
			}
		}
	}
	return ps
}

func bySeat(ps []online.PlayerView) []online.PlayerView {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j].Seat < ps[j-1].Seat; j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
	return ps
}

// renderStandings draws the column, exactly height lines tall.
func renderStandings(r *roomSession, form standingsForm, height int) string {
	players := standingsOrder(r)
	lines := make([]string, 0, len(players))
	for _, p := range players {
		switch form {
		case standingsFull:
			lines = append(lines, standingsFullLine(r, p))
		default:
			lines = append(lines, standingsCompactLine(r, p))
		}
	}

	// Exactly height lines: pad short columns, and push the tail into a "+N
	// more" line when a round has more players than a board has rows (eight
	// players on six rows is the smallest such mismatch).
	out := make([]string, 0, max(height, len(lines)))
	for _, line := range lines {
		if height > 0 && len(out) >= height-1 {
			break
		}
		out = append(out, line)
	}
	if more := len(lines) - len(out); more > 0 {
		out = append(out, st.muted.Render(fmt.Sprintf("+%d more", more)))
	}
	// Cut to height when the last line was all there was room for.
	if height > 0 && len(out) > height {
		out = out[:height]
	}
	for len(out) < height {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// standingsName is the column's name cell: the player's name cut to 8 and
// padded, dimmed when they are away.
func standingsName(p online.PlayerView) string {
	name := p.Name
	if name == "" {
		name = fmt.Sprintf("player %d", p.Seat)
	}
	if len(name) > 8 {
		name = name[:7] + "…"
	}
	style := st.text
	if !p.Connected {
		style = st.muted
	}
	return fmt.Sprintf("%-8s", style.Render(name))
}

func standingsFullLine(r *roomSession, p online.PlayerView) string {
	latest := " "
	if marks := p.Rows; len(marks) > 0 {
		latest = strings.Join(standingsRow(marks[len(marks)-1]), "")
	} else if r.view.Length > 0 {
		latest = st.muted.Render(strings.Repeat(" · ", r.view.Length-1) + "·")
	}
	state, stateStyle := standingsState(r, p)
	return standingsName(p) + " " + latest + " " + fmt.Sprintf("%-4s", stateStyle.Render(state))
}

func standingsCompactLine(r *roomSession, p online.PlayerView) string {
	state, stateStyle := standingsState(r, p)
	return standingsName(p) + " " + fmt.Sprintf("%-4s", stateStyle.Render(state))
}

// standingsRow is one row of the column in the board's own colours: each mark
// as the character ■ wearing the matching tile's background, so the standings
// and the board cannot disagree about what a mark looks like.
func standingsRow(marks []game.Mark) []string {
	cells := make([]string, len(marks))
	for i, m := range marks {
		cells[i] = lipgloss.NewStyle().
			Foreground(tileStyle(m).GetBackground()).
			Render("■")
	}
	return cells
}

// standingsState is the column's final cell, padded to 4 by the caller. The
// guess cap is the board's: length+1, as everywhere the board itself reads.
func standingsState(r *roomSession, p online.PlayerView) (string, lipgloss.Style) {
	switch {
	case p.Done && p.Solved:
		return fmt.Sprintf("in %d", p.Guesses), st.accent
	case p.Done:
		return "out", st.err
	default:
		return fmt.Sprintf("%d/%d", p.Guesses, r.view.Length+1), st.muted
	}
}
