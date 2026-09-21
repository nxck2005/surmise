package daily

import (
	"errors"
	"fmt"

	"github.com/nxck2005/surmise/internal/game"
)

// ValidateGame checks that persisted daily metadata still describes the game
// carrying it: a puzzle saved as a day's board must have the id that day and
// length derive. Stores call this at their shared codec boundary, beside
// challenge.ValidateGame, so a hand-edited or imported record cannot pose as a
// daily nobody played — or move the daily streaks with a date it never had.
//
// A custom puzzle is never a daily, and that rule lives here rather than in
// game.Validate because it has to hold for tombstones too: a deleted daily
// keeps its date, and a tombstone that was also custom would read as a day
// played. game.Validate's tombstone branch checks only identity, so the codec
// asks this package instead.
//
// The answer is deliberately not checked. A daily's answer comes from a seed
// the store does not have, and a day already played keeps whatever answer it
// was played with — history is not re-derived.
func ValidateGame(g *game.Game) error {
	if g.Daily == "" {
		return nil
	}
	if g.Custom {
		return errors.New("daily: a custom puzzle cannot be a daily")
	}
	d, err := ParseDay(g.Daily)
	if err != nil {
		return err
	}
	if g.ID != ID(d, g.Length) {
		return fmt.Errorf("daily: saved date %q does not match puzzle id", g.Daily)
	}
	return nil
}
