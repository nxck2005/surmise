package stats

import (
	"slices"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// RecentWindow is how many of the latest finished games "recent form" covers.
const RecentWindow = 20

// TopOpeners is how many opening words a trend lists.
const TopOpeners = 5

// Trend is one mode's recent form against all time, its personal bests and
// its most used opening words. It covers the games the profile counts
// (game.CountsForStats) and never a deleted one.
type Trend struct {
	Length  int
	Recent  Form // the latest RecentWindow finished games
	AllTime Form

	// Fastest is the quickest win with a recorded time, and Fewest the win in
	// the fewest guesses. A tie goes to the win that got there first. Nil
	// when there is no such win.
	Fastest *Best
	Fewest  *Best

	Openers []Opener // most used first, at most TopOpeners
}

// Form is a run of finished games. Averages cover wins only, as on the
// profile.
type Form struct {
	Games, Won  int
	WinRate     float64
	AvgAttempts float64
	AvgTime     time.Duration
}

// Best is the game that holds a personal best.
type Best struct {
	ID       string
	Attempts int
	Elapsed  time.Duration
}

// Opener is one first guess and how the games it opened went.
type Opener struct {
	Word        string
	Games, Won  int
	WinRate     float64
	AvgAttempts float64 // over the wins
}

// Trends works out one mode's Trend. The games are only read; it sorts its
// own copy.
func Trends(games []*game.Game, length int) Trend {
	t := Trend{Length: length}
	var finished []*game.Game
	for _, g := range games {
		if g.Deleted || !g.CountsForStats() || g.Length != length || !g.Status.Done() {
			continue
		}
		finished = append(finished, g)
	}
	// Completion order, as the streaks walk it, with the id breaking ties so
	// the answer never depends on the order the store listed the files in.
	slices.SortFunc(finished, compareGames)

	t.AllTime = formOf(finished)
	recent := finished
	if len(recent) > RecentWindow {
		recent = recent[len(recent)-RecentWindow:]
	}
	t.Recent = formOf(recent)

	for _, g := range finished {
		if g.Status != game.Won {
			continue
		}
		if g.ElapsedMS > 0 && (t.Fastest == nil || g.Elapsed() < t.Fastest.Elapsed) {
			t.Fastest = bestOf(g)
		}
		if t.Fewest == nil || g.Attempts() < t.Fewest.Attempts {
			t.Fewest = bestOf(g)
		}
	}
	t.Openers = openersOf(finished)
	return t
}

func formOf(games []*game.Game) Form {
	var (
		f        Form
		attempts int
		total    time.Duration
	)
	for _, g := range games {
		f.Games++
		if g.Status == game.Won {
			f.Won++
			attempts += g.Attempts()
			total = addDurationSaturating(total, g.Elapsed())
		}
	}
	if f.Games > 0 {
		f.WinRate = float64(f.Won) / float64(f.Games)
	}
	if f.Won > 0 {
		f.AvgAttempts = float64(attempts) / float64(f.Won)
		f.AvgTime = total / time.Duration(f.Won)
	}
	return f
}

func bestOf(g *game.Game) *Best {
	return &Best{ID: g.ID, Attempts: g.Attempts(), Elapsed: g.Elapsed()}
}

// openersOf ranks first guesses by how often they were played, then by how
// many of those games were won, then alphabetically.
func openersOf(games []*game.Game) []Opener {
	type tally struct{ games, won, attempts int }
	by := map[string]*tally{}
	for _, g := range games {
		if len(g.Guesses) == 0 {
			continue
		}
		w := g.Guesses[0]
		t, ok := by[w]
		if !ok {
			t = &tally{}
			by[w] = t
		}
		t.games++
		if g.Status == game.Won {
			t.won++
			t.attempts += g.Attempts()
		}
	}
	out := make([]Opener, 0, len(by))
	for w, t := range by {
		o := Opener{Word: w, Games: t.games, Won: t.won,
			WinRate: float64(t.won) / float64(t.games)}
		if t.won > 0 {
			o.AvgAttempts = float64(t.attempts) / float64(t.won)
		}
		out = append(out, o)
	}
	slices.SortFunc(out, func(a, b Opener) int {
		switch {
		case a.Games != b.Games:
			return b.Games - a.Games
		case a.Won != b.Won:
			return b.Won - a.Won
		}
		return strings.Compare(a.Word, b.Word)
	})
	if len(out) > TopOpeners {
		out = out[:TopOpeners]
	}
	return out
}

// IsBest reports whether g holds its mode's fastest win and its fewest-guess
// win among games. Neither is reported before the mode has two wins: a first
// win is trivially both, and saying so would mean nothing.
func IsBest(games []*game.Game, g *game.Game) (fastest, fewest bool) {
	if g == nil || g.Deleted || g.Status != game.Won || !g.CountsForStats() {
		return false, false
	}
	// One pass, not Trends: the result screen asks this on every finished
	// puzzle, and Trends sorts the mode's whole history (and tallies its
	// opening words) to answer two comparisons. A tie still goes to the win
	// that set the figure first, which is the completion order Trends walks:
	// UpdatedAt, then the id to break equal clocks.
	var (
		wins      int
		quickest  *game.Game
		fewestWin *game.Game
	)
	for _, other := range games {
		if other.Deleted || !other.CountsForStats() ||
			other.Length != g.Length || other.Status != game.Won {
			continue
		}
		wins++
		if other.ElapsedMS > 0 &&
			(quickest == nil || other.Elapsed() < quickest.Elapsed() ||
				(other.Elapsed() == quickest.Elapsed() && earlier(other, quickest))) {
			quickest = other
		}
		if fewestWin == nil || other.Attempts() < fewestWin.Attempts() ||
			(other.Attempts() == fewestWin.Attempts() && earlier(other, fewestWin)) {
			fewestWin = other
		}
	}
	if wins < 2 {
		return false, false
	}
	return quickest != nil && quickest.ID == g.ID, fewestWin.ID == g.ID
}

// earlier orders two wins by completion: UpdatedAt, then the id, exactly as
// Trends sorts them. It is only asked about equal figures, so a tie stays with
// the game that got there first.
func earlier(a, b *game.Game) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.Before(b.UpdatedAt)
	}
	return a.ID < b.ID
}
