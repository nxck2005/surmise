package play

import (
	"errors"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// Board is one open puzzle and the rules for playing it. It is the only
// writer of its puzzle while it is open.
//
// The lifecycle is the TUI's (internal/ui/gamescreen.go):
//
//   - A fresh puzzle is not saved until its first guess, so a board nobody
//     played never becomes a 0/6 record.
//   - The clock runs from the first letter typed, not from opening the
//     board, and stops at the guess that finishes the puzzle or at Leave.
//   - bank is the only caller of game.AddElapsed, and it bumps the lifetime
//     playtime with it, so the two can never learn of a session apart.
//   - Every accepted guess is saved at once.
type Board struct {
	app *App
	g   *game.Game

	typing string

	// persisted is false for a fresh puzzle until its first guess.
	persisted bool

	// sessionStart is when the player last started typing on this board.
	// Zero means no session is running: nothing typed yet, or the time is
	// already banked.
	sessionStart time.Time

	// sprint marks a board dealt by a timed run. The run deals its own
	// boards, so the board refuses a restart.
	sprint bool
}

// NewRandom opens a fresh random puzzle of the given length. Nothing is
// written until its first guess.
func (a *App) NewRandom(length int) (*Board, error) {
	g, err := a.newPuzzle(length)
	if err != nil {
		return nil, &Error{Code: ErrNewPuzzle.Code, Err: err}
	}
	return &Board{app: a, g: g}, nil
}

// Resume opens a saved puzzle by id. Its clock is not running until the next
// letter, so opening a board to look at it costs no time.
func (a *App) Resume(id string) (*Board, error) {
	g, err := a.store.Load(id)
	if err != nil {
		return nil, err
	}
	return &Board{app: a, g: g, persisted: true}, nil
}

// Open wraps a puzzle an opener made (a daily, a challenge, a custom word).
// saved says whether it is already on disk.
func (a *App) Open(g *game.Game, saved bool) *Board {
	return &Board{app: a, g: g, persisted: saved}
}

// Game is the puzzle under the board. It is the board's to change: read it,
// never write it.
func (b *Board) Game() *game.Game { return b.g }

// Typing is the row being typed, not yet submitted.
func (b *Board) Typing() string { return b.typing }

// SetSprint marks the board as dealt by a timed run.
func (b *Board) SetSprint(on bool) { b.sprint = on }

// Type adds a letter to the row. Anything but a-z (either case) is ignored,
// as is a letter on a full row or a finished puzzle. The clock starts here
// and only here, so a key and a tap on the on-screen keyboard start it alike.
func (b *Board) Type(c rune) {
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	if c < 'a' || c > 'z' {
		return
	}
	if b.g.Status.Done() || len(b.typing) >= b.g.Length {
		return
	}
	if b.sessionStart.IsZero() {
		b.sessionStart = b.app.clock.Now()
	}
	b.typing += string(c)
}

// Delete removes the last typed letter.
func (b *Board) Delete() {
	if len(b.typing) > 0 {
		b.typing = b.typing[:len(b.typing)-1]
	}
}

// Trim erases the typed row back to slot i, so tapping the third tile keeps
// the first two letters. An index past the typing does nothing.
func (b *Board) Trim(i int) {
	if i >= 0 && i < len(b.typing) {
		b.typing = b.typing[:i]
	}
}

// Submit plays the typed row. A refused guess (ErrTooShort, ErrNotAWord,
// ErrFinished) leaves the board as it was, typing included, so the player
// can fix it. An ErrSaveFailed means the guess was played and is on the
// board, but the store did not keep it.
func (b *Board) Submit() error {
	if b.g.Status.Done() {
		return ErrFinished
	}
	if len(b.typing) < b.g.Length {
		return ErrTooShort
	}
	switch err := b.g.Guess(b.typing); {
	case errors.Is(err, game.ErrNotAWord):
		return ErrNotAWord
	case errors.Is(err, game.ErrFinished):
		return ErrFinished
	case err != nil:
		return err
	}
	b.typing = ""
	// Time is banked on the finishing guess, so the recorded solve is the
	// time actually spent solving. The clock can be stopped here (a row typed
	// before the last Leave), and a zero start would bank every second since
	// the epoch.
	if b.g.Status.Done() && !b.sessionStart.IsZero() {
		b.bank(b.app.clock.Now().Sub(b.sessionStart))
		b.sessionStart = time.Time{}
	}
	b.persisted = true
	err := saveFailed(b.app.store.Save(b.g))
	// The finishing guess is the one moment a daily's result is sent.
	if b.g.Status.Done() {
		b.app.postDaily(b.g)
	}
	return err
}

// bank records a finished session on the puzzle and in the lifetime counter.
// It is the package's only caller of AddElapsed: that method bumps
// UpdatedAt, the order the streaks are walked in, so nothing that only reads
// may call it.
func (b *Board) bank(d time.Duration) {
	b.g.AddElapsed(d)
	b.app.bankPlaytime(d)
}

// Leave banks the running session and saves. Call it on every way off the
// board and when the app goes to the background; the board stays usable, and
// its clock starts again on the next letter. Calling it twice banks once.
//
// A puzzle never guessed is not saved, but its minutes still reach the
// lifetime counter, which counts time in the app rather than time on a
// record. A finished puzzle banks nothing: its time was banked by the guess
// that ended it, and time spent reviewing it is not solving.
func (b *Board) Leave() error {
	if !b.sessionStart.IsZero() && !b.g.Status.Done() {
		d := b.app.clock.Now().Sub(b.sessionStart)
		if b.persisted {
			b.bank(d)
		} else {
			b.app.bankPlaytime(d)
		}
	}
	b.sessionStart = time.Time{}
	if !b.persisted {
		return nil
	}
	return saveFailed(b.app.store.Save(b.g))
}

// Elapsed is the play time including the session running now.
func (b *Board) Elapsed() time.Duration {
	d := b.g.Elapsed()
	if !b.sessionStart.IsZero() && !b.g.Status.Done() {
		d += b.app.clock.Now().Sub(b.sessionStart)
	}
	return d
}

// restartRefusal is why this board may not be replaced by a fresh one, or
// nil. A daily is one shared puzzle a day; a custom word was set by someone
// who handed the phone over; a challenge has one board; a room's server and a
// sprint's run deal their own.
func (b *Board) restartRefusal() error {
	switch {
	case b.g.Room != nil:
		return ErrRestartRoom
	case b.sprint:
		return ErrRestartSprint
	case b.g.Daily != "":
		return ErrRestartDaily
	case b.g.Custom:
		return ErrRestartCustom
	case b.g.Challenge != nil:
		return ErrRestartChallenge
	}
	return nil
}

// Restart replaces the board with a fresh random puzzle of the same length,
// leaving the old one first (banked and saved). On a refusal or a failure the
// board is unchanged and still open.
func (b *Board) Restart() error {
	if err := b.restartRefusal(); err != nil {
		return err
	}
	if err := b.Leave(); err != nil {
		return err
	}
	g, err := b.app.newPuzzle(b.g.Length)
	if err != nil {
		return &Error{Code: ErrNewPuzzle.Code, Err: err}
	}
	b.g, b.typing, b.persisted, b.sessionStart = g, "", false, time.Time{}
	return nil
}
