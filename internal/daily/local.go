package daily

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrUnavailable means a day's seed could not be obtained: offline, or a day
// the source cannot speak for. The local source never returns it — it is here
// so the UI can recognise the condition before there is a source that has it.
var ErrUnavailable = errors.New("daily: seed unavailable")

// seedVersion names the derivation, so it can be rotated without touching ID.
//
// The two are versioned separately on purpose: bumping this changes future
// answers, which is harmless, while changing ID would rename every daily on
// disk and orphan them. Bump this; never change ID.
const seedVersion = "wortle-daily-v1"

// pepper keys the local derivation.
//
// It is NOT a secret, and nothing here should be read as though it were. Every
// install has to derive the same answer for the same date, offline, with no
// coordination — so the derivation can only use public inputs and whatever
// ships in the binary, and therefore its key ships in the binary too. `strings`
// on the binary, or this file, recovers it in under a minute, and with it every
// future word.
//
// What it does buy is domain separation: the answer is not literally "hash the
// date", so it is not given away by accident or by anyone who guesses the
// obvious construction. That is all. The real property — a word that does not
// exist on the player's machine until its day arrives — needs a Source that
// fetches a committed reveal, which is what this interface exists for.
//
// It must not be injected at build time (-ldflags): two builds would then
// disagree about the word, which destroys the only property a daily has.
var pepper = []byte("wortle/daily/2026: not a secret, see local.go")

// Local is the offline seed source: an HMAC over the date and length.
//
// The construction matches what a remote source will use — HMAC-SHA256 keyed on
// the day's secret — so only the key's origin changes when one lands.
func Local() Source { return localSource{} }

type localSource struct{}

func (localSource) Seed(_ context.Context, d Day, length int) (Seed, error) {
	mac := hmac.New(sha256.New, pepper)
	fmt.Fprint(mac, label(d, length))

	var s Seed
	copy(s[:], mac.Sum(nil))
	return s, nil
}

// label is the message both the local source and any future one derive from:
// the versioned scheme, the date, and the mode.
func label(d Day, length int) string {
	return fmt.Sprintf("%s|%s|%d", seedVersion, d, length)
}
