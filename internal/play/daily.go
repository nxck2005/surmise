package play

import (
	"context"
	"errors"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// OpenDaily opens a day's puzzle for one length, the TUI's openDaily.
//
// The order matters. What is on disk is read first, so resuming or reviewing
// a daily never needs a seed, which keeps both working offline once seeds
// come from a network. A daily that was played and deleted is refused
// (ErrDailySpent): Load reports its tombstone as not found, and rebuilding it
// would write over the tombstone at the first guess. Only a day with nothing
// saved asks the seed source.
func (a *App) OpenDaily(ctx context.Context, day daily.Day, length int) (*Board, error) {
	id := daily.ID(day, length)
	switch g, err := a.store.Load(id); {
	case err == nil:
		return a.Open(g, true), nil
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}
	if spent, err := a.exists(id); err != nil {
		return nil, err
	} else if spent {
		return nil, ErrDailySpent
	}
	g, err := daily.New(ctx, a.daily, day, length)
	if err != nil {
		return nil, &Error{Code: ErrNewPuzzle.Code, Err: err}
	}
	return a.Open(g, false), nil
}

// exists reports whether a record with this id is in the store, tombstones
// included. An id-only read: it decodes nothing.
func (a *App) exists(id string) (bool, error) {
	ids, err := a.store.IDs()
	if err != nil {
		return false, err
	}
	for _, have := range ids {
		if have == id {
			return true, nil
		}
	}
	return false, nil
}

// The states a day's board can be in, as DailyRow.State names them.
const (
	DailyNew     = "new"
	DailyPlaying = "playing"
	DailyWon     = "won"
	DailyLost    = "lost"
	DailySpent   = "spent" // played, then deleted
)

// DailyView is the daily screen: one row per length and the trio across
// them. Rows carry marks, never guesses or answers, so nothing on the screen
// is a spoiler to show or to paste.
type DailyView struct {
	Day string `json:"day"`
	// ResetsAtMS is when the next day starts, as Unix milliseconds.
	ResetsAtMS int64      `json:"resetsAtMs"`
	Rows       []DailyRow `json:"rows"`
	Trio       Trio       `json:"trio"`
	// Share is the trio as public text, set only once all three are done.
	Share string `json:"share,omitempty"`
}

// DailyRow is one length's daily.
type DailyRow struct {
	Length      int        `json:"length"`
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	State       string     `json:"state"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"maxAttempts"`
	ElapsedMS   int64      `json:"elapsedMs"`
	Marks       [][]string `json:"marks"`
}

// done is a finished board that still has its record. A spent one is not:
// the tombstone keeps the status and drops the marks, so there is no board
// left to show.
func (r DailyRow) done() bool { return r.State == DailyWon || r.State == DailyLost }

// Trio is a day across its lengths. It is derived every time and never
// stored, and it is not a stats figure: the modes stay independent, so
// missing one length never costs a run at another.
type Trio struct {
	Done      int   `json:"done"`
	Of        int   `json:"of"`
	Guesses   int   `json:"guesses"`
	ElapsedMS int64 `json:"elapsedMs"`
	Complete  bool  `json:"complete"`
}

// DailyView reads what has been played of a day. It asks for ids first and
// loads only the day's own records, so opening the screen costs three reads
// whatever the history holds. An id that is present but reports not found is
// a tombstone: spent. Any other read failure is returned.
func (a *App) DailyView(day daily.Day) (DailyView, error) {
	v := DailyView{
		Day:        day.String(),
		ResetsAtMS: day.ResetsAt().UnixMilli(),
		Rows:       make([]DailyRow, 0, len(words.Lengths)),
	}
	ids, err := a.store.IDs()
	if err != nil {
		return v, err
	}
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	for _, n := range words.Lengths {
		id := daily.ID(day, n)
		row := DailyRow{Length: n, ID: id, Code: game.Code(id), State: DailyNew, MaxAttempts: n + 1, Marks: [][]string{}}
		if present[id] {
			g, err := a.store.Load(id)
			switch {
			case err == nil && g.Deleted:
				row.State = DailySpent
			case err == nil:
				row.State = stateOf(g.Status)
				row.Attempts, row.MaxAttempts = g.Attempts(), g.MaxAttempts
				row.ElapsedMS = g.Elapsed().Milliseconds()
				row.Marks = markNames(g.Marks)
			case errors.Is(err, store.ErrNotFound):
				row.State = DailySpent
			default:
				return v, err
			}
		}
		v.Rows = append(v.Rows, row)
	}
	v.Trio = trioOf(v.Rows)
	if v.Trio.Complete {
		v.Share = ShareTrio(v.Day, v.Rows)
	}
	return v, nil
}

func stateOf(s game.Status) string {
	switch s {
	case game.Won:
		return DailyWon
	case game.Lost:
		return DailyLost
	}
	return DailyPlaying
}

func trioOf(rows []DailyRow) Trio {
	t := Trio{Of: len(rows)}
	for _, r := range rows {
		if !r.done() {
			continue
		}
		t.Done++
		t.Guesses += r.Attempts
		t.ElapsedMS += r.ElapsedMS
	}
	t.Complete = t.Of > 0 && t.Done == t.Of
	return t
}

func markNames(marks [][]game.Mark) [][]string {
	out := make([][]string, len(marks))
	for i, row := range marks {
		out[i] = make([]string, len(row))
		for j, m := range row {
			out[i][j] = m.String()
		}
	}
	return out
}
