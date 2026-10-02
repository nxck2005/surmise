package ui

import (
	"errors"

	"github.com/nxck2005/surmise/internal/store"
)

// saveFailed is what the player is told when a save is refused. Storage that
// is full gets its own words: the browser's are a DOMException name, and the
// player needs to know that nothing more is being kept and where to look.
func saveFailed(err error) string {
	if errors.Is(err, store.ErrFull) {
		return "storage is full — not saved (see backup)"
	}
	return "could not save: " + err.Error()
}

// measureStorage reads how full the store is into the backup screen, which is
// the only place that shows it. It costs a read of every stored value in a
// browser, so it runs when that screen opens and after a restore, not per frame.
func (m *Model) measureStorage() {
	if m.storage == nil {
		m.backup.used, m.backup.limit = 0, 0
		return
	}
	m.backup.used, m.backup.limit = m.storage()
}
