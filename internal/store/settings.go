package store

import (
	"os"
	"path/filepath"
)

// Settings is what the player has chosen, as opposed to what they have played.
// Native builds keep it at the root of the data dir; KVStore uses the same
// codec under its browser settings key. It is small, rewritten rarely and safe
// to lose: every field has a working zero value.
type Settings struct {
	Theme string `json:"theme,omitempty"`
	// DisplayName is local presentation on the profile screen. It is not an
	// account ID, puzzle owner, authentication claim, or uniqueness promise;
	// future networking can add identity without inheriting this field's rules.
	DisplayName string `json:"display_name,omitempty"`
	// Length is the word length the app opens on. Zero means "no choice made",
	// which the UI reads as its own default rather than as an invalid mode.
	Length int `json:"length,omitempty"`
	// RememberLast makes playing a mode set Length, so the app reopens on
	// whatever was last played. Off by default: the zero value is the one that
	// leaves Length alone.
	RememberLast bool `json:"remember_last,omitempty"`

	// The splash screen's three preferences. All strings, because the zero value
	// has to mean "nothing chosen" and the default for the first of them is on:
	// a bool could not tell "never chosen" from "chosen off".
	//
	//	Splash        "" (on), "on", "off"
	//	SplashArt     "" (the default art), "random", or a banner's name
	//	SplashDismiss "" (key), "skip", "key", "fixed"
	//
	// The UI resolves each of them, and an unknown value is reported on its
	// error line rather than refused — art that stopped shipping must not cost
	// anyone a launch.
	Splash        string `json:"splash,omitempty"`
	SplashArt     string `json:"splash_art,omitempty"`
	SplashDismiss string `json:"splash_dismiss,omitempty"`

	// Motion is how much the board animates: tile reveals, the invalid-word
	// cue, keycap pulses and the win accent. A string for the same reason the
	// splash preferences are strings — the default is not the zero value, so a
	// bool could not tell "never chosen" from "chosen off".
	//
	//	Motion  "" (pronounced), "off", "restrained", "pronounced"
	//
	// An unset value is also what lets the environment answer instead:
	// $NO_MOTION natively, prefers-reduced-motion in a browser. Choosing here
	// is deliberate and overrides both.
	Motion string `json:"motion,omitempty"`

	// SplashMillis is how long a timed splash stays up. Zero is "nothing
	// chosen", which the UI reads as its own default — the same rule Length
	// follows, and the reason this is not a time.Duration: a duration's zero is
	// a legitimate value (no wait at all) and could not be told apart from an
	// older settings file that never had the field.
	SplashMillis int `json:"splash_ms,omitempty"`

	// PlaytimeMS is the lifetime play counter, in milliseconds — the one field
	// here that is not a preference. It lives with the preferences because both
	// stores already carry this struct through one codec, so the browser build
	// needs no extra method to keep it.
	//
	// It is a counter and not a figure derived from the saved puzzles, which is
	// what makes time played permanent: a deleted puzzle leaves a tombstone
	// with no ElapsedMS, so a total summed from the records would shrink when a
	// puzzle is deleted. This only ever grows. Zero means nothing played yet,
	// which is also what an older settings file says, so stats.Playtime floors
	// it with what the records can still prove.
	PlaytimeMS int64 `json:"playtime_ms,omitempty"`
}

const settingsName = "settings.json"

func (s *JSON) settingsPath() string { return filepath.Join(s.dir, settingsName) }

// Settings reads the saved preferences. A missing or damaged file yields the
// defaults rather than an error: a bad settings file must never cost a puzzle.
func (s *JSON) Settings() Settings {
	b, err := os.ReadFile(s.settingsPath())
	if err != nil {
		return Settings{}
	}
	return decodeSettings(b)
}

// SaveSettings persists preferences, atomically like every other write here.
func (s *JSON) SaveSettings(v Settings) error {
	b, err := encodeSettings(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.settingsPath(), b)
}
