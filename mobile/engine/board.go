package engine

import (
	"unicode/utf8"

	"github.com/nxck2005/surmise/internal/play"
)

// NewBoardRequest opens a fresh random board.
type NewBoardRequest struct {
	Length int `json:"length"`
}

// ResumeRequest opens a saved puzzle.
type ResumeRequest struct {
	ID string `json:"id"`
}

// TypeRequest adds one letter. Anything but a single a-z letter is ignored,
// as a stray key is on the terminal.
type TypeRequest struct {
	Letter string `json:"letter"`
}

// TrimRequest erases the typed row back to a slot.
type TrimRequest struct {
	Index int `json:"index"`
}

func init() {
	register("board.new", (*Engine).boardNew)
	register("board.resume", (*Engine).boardResume)
	register("board.view", (*Engine).boardView)
	register("board.type", (*Engine).boardType)
	register("board.delete", (*Engine).boardDelete)
	register("board.trim", (*Engine).boardTrim)
	register("board.submit", (*Engine).boardSubmit)
	register("board.leave", (*Engine).boardLeave)
	register("board.restart", (*Engine).boardRestart)
}

// open replaces the open board, leaving the old one first. A failed save of
// the old board keeps it open: its last guess is not on disk yet, and the
// player should hear so before it is dropped.
func (e *Engine) open(next func() (*play.Board, error)) (play.BoardView, error) {
	if e.board != nil {
		if err := e.board.Leave(); err != nil {
			return play.BoardView{}, err
		}
	}
	b, err := next()
	if err != nil {
		return play.BoardView{}, err
	}
	e.board = b
	return b.View(), nil
}

func (e *Engine) boardNew(r NewBoardRequest) (play.BoardView, error) {
	switch r.Length {
	case 4, 5, 6:
	default:
		return play.BoardView{}, invalid("length %d: want 4, 5 or 6", r.Length)
	}
	return e.open(func() (*play.Board, error) { return e.app.NewRandom(r.Length) })
}

func (e *Engine) boardResume(r ResumeRequest) (play.BoardView, error) {
	return e.open(func() (*play.Board, error) { return e.app.Resume(r.ID) })
}

// withBoard runs a move on the open board and answers with its view. A move
// that fails still answers with the error alone; the app asks board.view when
// it needs the state after a failed save.
func (e *Engine) withBoard(move func(*play.Board) error) (play.BoardView, error) {
	if e.board == nil {
		return play.BoardView{}, &Error{Code: CodeNoBoard}
	}
	if err := move(e.board); err != nil {
		return play.BoardView{}, err
	}
	return e.board.View(), nil
}

func (e *Engine) boardView(Empty) (play.BoardView, error) {
	return e.withBoard(func(*play.Board) error { return nil })
}

func (e *Engine) boardType(r TypeRequest) (play.BoardView, error) {
	return e.withBoard(func(b *play.Board) error {
		if c, n := utf8.DecodeRuneInString(r.Letter); n == len(r.Letter) && n > 0 {
			b.Type(c)
		}
		return nil
	})
}

func (e *Engine) boardDelete(Empty) (play.BoardView, error) {
	return e.withBoard(func(b *play.Board) error { b.Delete(); return nil })
}

func (e *Engine) boardTrim(r TrimRequest) (play.BoardView, error) {
	return e.withBoard(func(b *play.Board) error { b.Trim(r.Index); return nil })
}

func (e *Engine) boardSubmit(Empty) (play.BoardView, error) {
	return e.withBoard((*play.Board).Submit)
}

// boardLeave banks and saves; the board stays open, so coming back to it is
// board.view, and its clock starts again on the next letter.
func (e *Engine) boardLeave(Empty) (play.BoardView, error) {
	return e.withBoard((*play.Board).Leave)
}

func (e *Engine) boardRestart(Empty) (play.BoardView, error) {
	return e.withBoard((*play.Board).Restart)
}
