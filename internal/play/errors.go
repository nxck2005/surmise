package play

import "errors"

// Error is a refusal a player can be told about. It carries a code, not a
// sentence: the front ends own their copy (the phone's lives on the Kotlin
// side), and a code is what lets both say the same thing in their own words.
type Error struct {
	Code string
	// Err is the cause, for the codes that wrap one (a failed save). Nil for
	// a plain refusal.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// Is matches on the code, so errors.Is(err, ErrSaveFailed) holds for any
// failed save whatever its cause.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// The refusals. Each code is part of the engine's contract with the app, so
// a code once shipped keeps its spelling.
var (
	// ErrTooShort is a guess with fewer letters than the board.
	ErrTooShort = &Error{Code: "too_short"}
	// ErrNotAWord is a guess outside the word list.
	ErrNotAWord = &Error{Code: "not_a_word"}
	// ErrFinished is input on a puzzle that is already over.
	ErrFinished = &Error{Code: "finished"}

	// Restart refusals: "start another" on a board that is not a random one.
	ErrRestartDaily     = &Error{Code: "restart_daily"}
	ErrRestartCustom    = &Error{Code: "restart_custom"}
	ErrRestartChallenge = &Error{Code: "restart_challenge"}
	ErrRestartRoom      = &Error{Code: "restart_room"}
	ErrRestartSprint    = &Error{Code: "restart_sprint"}

	// ErrSaveFailed wraps a store error. The move it followed still happened:
	// a guess the store could not write is still on the board.
	ErrSaveFailed = &Error{Code: "save_failed"}
	// ErrNewPuzzle wraps a failure to make a fresh puzzle.
	ErrNewPuzzle = &Error{Code: "new_puzzle_failed"}
)

func saveFailed(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: ErrSaveFailed.Code, Err: err}
}

// CodeOf is the code of a refusal, or "" for an error that is not one.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
