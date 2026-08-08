package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings is what the player has chosen, as opposed to what they have played.
// It sits beside meta.json at the root of the data dir — small, rewritten
// rarely, and safe to lose: every field has a working zero value.
type Settings struct {
	Theme string `json:"theme,omitempty"`
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
	//	SplashDismiss "" (skip), "skip", "key", "fixed"
	//
	// The UI resolves each of them, and an unknown value is reported on its
	// error line rather than refused — art that stopped shipping must not cost
	// anyone a launch.
	Splash        string `json:"splash,omitempty"`
	SplashArt     string `json:"splash_art,omitempty"`
	SplashDismiss string `json:"splash_dismiss,omitempty"`

	// SplashMillis is how long a timed splash stays up. Zero is "nothing
	// chosen", which the UI reads as its own default — the same rule Length
	// follows, and the reason this is not a time.Duration: a duration's zero is
	// a legitimate value (no wait at all) and could not be told apart from an
	// older settings file that never had the field.
	SplashMillis int `json:"splash_ms,omitempty"`
}

const settingsName = "settings.json"

func (s *JSON) settingsPath() string { return filepath.Join(s.dir, settingsName) }

// Settings reads the saved preferences. A missing or damaged file yields the
// defaults rather than an error, the same way a damaged counter is recovered
// from rather than fatal: a bad settings file must never cost a puzzle.
func (s *JSON) Settings() Settings {
	b, err := os.ReadFile(s.settingsPath())
	if err != nil {
		return Settings{}
	}
	var out Settings
	if err := json.Unmarshal(b, &out); err != nil {
		return Settings{}
	}
	return out
}

// SaveSettings persists preferences, atomically like every other write here.
func (s *JSON) SaveSettings(v Settings) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode settings: %w", err)
	}
	return writeFileAtomic(s.settingsPath(), b)
}
