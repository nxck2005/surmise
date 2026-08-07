// Package daily builds the puzzle everyone plays on a given day.
//
// A daily is an ordinary game.Game whose id and answer are both derived from
// its date, so every player gets the same board and the same code with nothing
// to coordinate. The two derivations are deliberately separate:
//
//   - the id comes from ID, which uses nothing secret. It is what makes the
//     puzzle the same puzzle for everybody, and what makes its file on disk
//     findable — which is how "have I played today's?" is answered without an
//     index.
//   - the answer comes from a Source's seed, which is the only part that has to
//     be hard to guess.
//
// That split is what lets the seed source be replaced later without disturbing
// anything a player has already played: swapping Source changes future answers
// only, never an id, a code, or a puzzle already on disk.
package daily

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// Seed is the secret input a day's answer is drawn from.
type Seed [32]byte

// Source supplies the seed for a day's puzzle.
//
// This interface is the seam. Local (local.go) is the offline implementation
// shipped today; a verified remote one drops in here without game, words, store
// or the screens changing.
//
// It carries a context and returns an error even though Local needs neither,
// because a remote source has to be cancellable and has to be able to say it is
// offline. Paying for both now is what keeps the swap from turning the UI's
// synchronous "open the daily" into an asynchronous one later.
type Source interface {
	Seed(ctx context.Context, d Day, length int) (Seed, error)
}

// New builds the day's puzzle for a length.
//
// It never re-rolls its id the way the random path does (see newPuzzleWith in
// internal/ui): the id is the whole point — a daily whose id moved would be a
// different puzzle, and two players would stop sharing a code.
func New(ctx context.Context, src Source, d Day, length int) (*game.Game, error) {
	if !words.SupportedLength(length) {
		return nil, fmt.Errorf("daily: unsupported length %d", length)
	}

	seed, err := src.Seed(ctx, d, length)
	if err != nil {
		return nil, err
	}
	answer, err := answerFor(seed, length)
	if err != nil {
		return nil, err
	}

	g, err := game.NewFrom(ID(d, length), answer, length)
	if err != nil {
		return nil, err
	}
	g.Daily = d.String()
	return g, nil
}

// answerFor reduces a seed to a word.
//
// The reduction lives here rather than in words so that the whole derivation
// sits in one file. Sixty-four bits over a pool of ~1200 makes the modulo bias
// far too small to favour any word.
//
// The pool is the shipped, sorted answer list, so regenerating the word lists
// with tools/genwords moves the answer for every date not yet played. Days
// already played are unaffected: their answer is written in their save. Treat a
// regeneration as something that happens at a release boundary.
func answerFor(seed Seed, length int) (string, error) {
	count, err := words.AnswerCount(length)
	if err != nil {
		return "", err
	}
	i := binary.BigEndian.Uint64(seed[:8]) % uint64(count)
	return words.AnswerAt(length, int(i))
}
