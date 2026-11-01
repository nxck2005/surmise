package play

import (
	"fmt"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
)

// The share texts are the TUI's (internal/ui/resultscreen.go and
// dailyscreen.go), byte for byte, so a result pasted from the phone reads the
// same as one pasted from a terminal. They carry marks and figures only:
// never the answer, never a guessed letter.

// ShareLegend explains the glyphs ShareGrid writes, for a reader whose client
// shows them as bare squares.
const ShareLegend = "■ correct  □ present  · absent"

// ShareGrid is a scored board as public text: one line per played guess, one
// fixed glyph per mark, with a trailing newline. The glyphs are not themed: a
// share is read on somebody else's screen.
func ShareGrid(marks [][]string) string {
	var b strings.Builder
	for _, row := range marks {
		for _, mark := range row {
			switch mark {
			case "correct":
				b.WriteRune('■')
			case "present":
				b.WriteRune('□')
			default:
				b.WriteRune('·')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// attemptsText is a score in the debrief's words: X/max for a loss.
func attemptsText(lost bool, attempts, maxAttempts int) string {
	if lost {
		return fmt.Sprintf("X/%d", maxAttempts)
	}
	return fmt.Sprintf("%d/%d", attempts, maxAttempts)
}

// ShareResult is one finished puzzle as public text. A challenge carries its
// code, so the reader can play the same board.
func ShareResult(g *game.Game) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s #%s %s\n", brand.Name, game.Code(g.ID),
		attemptsText(g.Status == game.Lost, g.Attempts(), g.MaxAttempts))
	switch {
	case g.Custom:
		fmt.Fprintf(&b, "custom · %d letters · %s\n", g.Length, FormatDuration(g.Elapsed()))
	case g.Daily != "":
		fmt.Fprintf(&b, "daily %s · %d letters · %s\n", g.Daily, g.Length, FormatDuration(g.Elapsed()))
	case g.Challenge != nil:
		fmt.Fprintf(&b, "challenge %s · %d letters · %s\n", g.Challenge.Code, g.Length, FormatDuration(g.Elapsed()))
	default:
		fmt.Fprintf(&b, "%d letters · %s\n", g.Length, FormatDuration(g.Elapsed()))
	}
	b.WriteString(ShareGrid(markNames(g.Marks)))
	b.WriteString(ShareLegend)
	return b.String()
}

// ShareTrio is a day's three boards under one heading. The date names the day
// rather than three codes: everyone playing that date has the same boards.
func ShareTrio(day string, rows []DailyRow) string {
	var b strings.Builder
	var guesses int
	var elapsed time.Duration
	for _, r := range rows {
		guesses += r.Attempts
		elapsed += time.Duration(r.ElapsedMS) * time.Millisecond
	}
	fmt.Fprintf(&b, "%s daily %s · %d/%d\n", brand.Name, day, len(rows), len(rows))
	fmt.Fprintf(&b, "%d guesses · %s\n", guesses, FormatDuration(elapsed))
	for _, r := range rows {
		fmt.Fprintf(&b, "\n%d letters %s\n", r.Length, attemptsText(r.State == DailyLost, r.Attempts, r.MaxAttempts))
		b.WriteString(ShareGrid(r.Marks))
	}
	b.WriteString("\n" + ShareLegend)
	return b.String()
}

// FormatDuration is the TUI's duration format (internal/ui/format.go): "–"
// for nothing, then 42s, 3:07 or 1:02:03. The share texts use it, so it must
// match the terminal's to the byte.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "–"
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d % time.Hour / time.Minute)
	s := int(d % time.Minute / time.Second)
	switch {
	case h > 0:
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	case m > 0:
		return fmt.Sprintf("%d:%02d", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
