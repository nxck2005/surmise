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
