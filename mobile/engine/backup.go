package engine

import (
	"reflect"

	"github.com/nxck2005/surmise/internal/backup"
	"github.com/nxck2005/surmise/internal/theme"
)

// Export builds a backup of the whole install: the same archive the terminal's
// -export writes, so either app can restore the other's file.
func (e *Engine) Export() ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.board != nil {
		// The open board's running session belongs in the file.
		if err := e.board.Leave(); err != nil {
			return nil, err
		}
	}
	themes, _, err := theme.Files(theme.Dir(e.dir))
	if err != nil {
		// One unreadable theme directory is not worth losing the puzzles
		// over, as on the terminal.
		themes = nil
	}
	return backup.Build(e.st, e.st.Settings(), themes, e.client, e.clock.Now())
}

// ImportReport is what a restore did, in the terms a player asks about.
type ImportReport struct {
	PuzzlesAdded    int      `json:"puzzlesAdded"`
	PuzzlesKept     int      `json:"puzzlesKept"`
	SettingsFilled  []string `json:"settingsFilled"`
	PlaytimeAddedMS int64    `json:"playtimeAddedMs"`
	ThemesAdded     int      `json:"themesAdded"`
}

// WireTypes are the types that cross to Kotlin outside the method registry:
// ImportBackup answers outside Call, but in the same envelope.
func WireTypes() []reflect.Type {
	return []reflect.Type{reflect.TypeFor[ImportReport]()}
}

// CodeBadBackup is a file that is not a backup this build can read.
const CodeBadBackup = "bad_backup"

// Import merges a backup into the install and answers with an envelope, like
// Call. It only ever adds: a puzzle the store holds is kept, a chosen
// preference is kept, the play counter is raised to the larger figure, and
// consent to go online is never restored (backup.Apply leaves Network alone).
// Themes in the file are written to the themes directory untouched, so they
// survive the next export.
func (e *Engine) Import(b []byte) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	res, err := backup.Apply(b, e.st, e.st.Settings())
	if err != nil {
		return reply(nil, &Error{Code: CodeBadBackup, Detail: err.Error()})
	}
	if len(res.SettingsFilled) > 0 || res.PlaytimeAdded > 0 {
		if err := e.st.SaveSettings(res.Settings); err != nil {
			return reply(nil, err)
		}
	}
	rep := ImportReport{
		PuzzlesAdded: res.PuzzlesAdded, PuzzlesKept: res.PuzzlesKept,
		SettingsFilled: res.SettingsFilled, PlaytimeAddedMS: res.PlaytimeAdded.Milliseconds(),
	}
	if rep.SettingsFilled == nil {
		rep.SettingsFilled = []string{}
	}
	if len(res.Themes) > 0 {
		// A refused theme name must not make a restore that moved a whole
		// history look like it failed, so the count of what landed is the
		// whole answer.
		rep.ThemesAdded, _, _ = theme.WriteNew(theme.Dir(e.dir), res.Themes)
	}
	return reply(rep, nil)
}
