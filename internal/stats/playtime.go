package stats

import (
	"fmt"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// Playtime is the lifetime play counter: how long has been spent playing, ever.
//
// It is deliberately not one of Summary's fields. Summary answers "how well is
// this going" and so leaves out custom puzzles and deleted records; playtime
// answers "how long has this been played" and leaves out nothing. Time spent
// losing, time spent on a puzzle still in play and time spent on a word somebody
// else chose are all time spent.
//
// saved is the counter kept in the settings, banked as it is played, and it is
// the authority: a deleted puzzle keeps a tombstone that carries no ElapsedMS,
// so a total summed from the records alone would fall when a puzzle is deleted,
// and time already played would stop being permanent.
//
// The records are used as a floor rather than a source. That is what seeds an
// install whose history predates the counter (saved is zero, the floor is the
// whole of it) and what repairs a lost settings file. Once seeded the counter
// includes everything the floor does and the deleted puzzles besides, so it
// stays the larger of the two and deletion never lowers the answer.
func Playtime(saved time.Duration, games []*game.Game) time.Duration {
	var floor time.Duration
	for _, g := range games {
		// No skips here, by design — see above. A tombstone contributes nothing
		// because its ElapsedMS was destroyed with the rest of the record, which
		// is exactly why the counter and not this sum is the authority.
		floor += g.Elapsed()
	}
	if saved > floor {
		return saved
	}
	return floor
}

// FormatPlaytime renders a lifetime total: "42s", "8m", "12h 04m", "1d 03h".
//
// It is a separate rendering from the UI's formatDuration, which is built for
// one puzzle's solve and has no unit above hours — a total of a day and a half
// would read there as "37:12:04". This lives in stats rather than in the UI
// because the -playtime flag prints it too, and one figure should not have two
// spellings.
func FormatPlaytime(d time.Duration) string {
	if d <= 0 {
		return "none"
	}
	d = d.Round(time.Second)

	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %02dh", d/(24*time.Hour), d%(24*time.Hour)/time.Hour)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %02dm", d/time.Hour, d%time.Hour/time.Minute)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}
