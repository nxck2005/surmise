package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/backup"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// The backup screen, driven the way a player drives it.
//
// The platform's file access is a Transfer, so these tests supply one that
// keeps the bytes in memory. That is the whole reason the interface exists: the
// screen can be proved without a disk, and the browser's implementation is then
// the only untested part of the path rather than all of it.

// fakeTransfer is a file that lives in memory. offer is what a load hands back,
// and saved is what the last save wrote.
type fakeTransfer struct {
	saved []byte
	where string

	offer    []byte
	offerAs  string
	loadErr  error
	saveErr  error
	loadCall int
}

func (f *fakeTransfer) Save(b []byte) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = b
	if f.where == "" {
		f.where = "somewhere.json"
	}
	return f.where, nil
}

func (f *fakeTransfer) Load() ([]byte, string, error) {
	f.loadCall++
	return f.offer, f.offerAs, f.loadErr
}

// backupModel is a model that can move files, sitting on the backup screen.
func backupModel(t *testing.T, tr Transfer) *Model {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	m := New(s, nil, Options{Motion: motionOffName, Transfer: tr})
	m.screen = screenBackup
	m.backup.reset()
	return m
}

// playOne saves a finished puzzle, so there is a history worth carrying.
func playOne(t *testing.T, m *Model, answer string) *game.Game {
	t.Helper()
	g, err := game.New(len(answer))
	if err != nil {
		t.Fatal(err)
	}
	g.Answer = answer
	if err := g.Guess(answer); err != nil {
		t.Fatal(err)
	}
	if err := m.store.Save(g); err != nil {
		t.Fatal(err)
	}
	return g
}

// drain runs a command and feeds what it returns back into the model, which is
// what the framework does. The load path is a command, so a test that does not
// do this proves nothing about it.
func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	m.Update(msg)
}

// A build that cannot move files is offered no backup row, rather than one that
// fails when it is pressed. This is also what keeps every other menu test true.
func TestBackupRowOnlyWhenTheBuildCanMoveFiles(t *testing.T) {
	plain := newModel(t)
	for _, c := range plain.menu.choices {
		if c.kind == choiceBackup {
			t.Fatal("a build with no Transfer offers a backup row")
		}
	}

	able := backupModel(t, &fakeTransfer{})
	if menuIndex(t, able, choiceBackup, 0) < 0 {
		t.Error("a build that can move files offers no backup row")
	}
}

// Saving writes a real archive: what the screen produces is what backup.Read
// accepts, tombstones and all.
func TestBackupSaveWritesAnArchive(t *testing.T) {
	tr := &fakeTransfer{where: "backups/surmise-backup-2026-08-18.json"}
	m := backupModel(t, tr)
	g := playOne(t, m, "crane")

	frame := send(t, m, "enter") // the cursor opens on "save a backup"

	if len(tr.saved) == 0 {
		t.Fatal("save wrote nothing")
	}
	_, games, err := backup.Read(tr.saved)
	if err != nil {
		t.Fatalf("the screen wrote a file backup.Read refuses: %v", err)
	}
	if len(games) != 1 || games[0].ID != g.ID {
		t.Errorf("archive holds %d records, want the puzzle just played", len(games))
	}
	// The frame proves it is on screen; the report proves what it says. The two
	// are separate because the note wraps to the panel's width, so a path long
	// enough to be worth showing is never one line to match against.
	if !strings.Contains(plain(frame), "saved to") {
		t.Errorf("the screen does not say the file was written:\n%s", frame)
	}
	if got := strings.Join(m.backup.report, " "); !strings.Contains(got, tr.where) {
		t.Errorf("report = %q, want it to name %q", got, tr.where)
	}
}

// Loading restores a history into an install that does not have it, and says
// what it did.
func TestBackupLoadRestoresAHistory(t *testing.T) {
	// One install exports.
	source := backupModel(t, &fakeTransfer{})
	won := playOne(t, source, "crane")
	send(t, source, "enter")
	archive := source.transfer.(*fakeTransfer).saved

	// Another imports.
	tr := &fakeTransfer{offer: archive, offerAs: "mine.json"}
	m := backupModel(t, tr)
	_, cmd := m.Update(key("down")) // onto "load a backup"
	drain(t, m, cmd)
	_, cmd = m.Update(key("enter"))
	drain(t, m, cmd)

	frame := m.View().Content
	if !strings.Contains(frame, "restored 1 puzzle") {
		t.Errorf("the screen does not report the restore:\n%s", frame)
	}

	got, err := m.store.Load(won.ID)
	if err != nil {
		t.Fatalf("the puzzle did not reach the store: %v", err)
	}
	if got.Answer != "crane" {
		t.Errorf("answer = %q, want the board intact", got.Answer)
	}
}

// A second load of the same file changes nothing and says so. This is the
// promise the screen makes to a player before they press the button.
func TestBackupLoadTwiceSaysNothingIsNew(t *testing.T) {
	source := backupModel(t, &fakeTransfer{})
	playOne(t, source, "crane")
	send(t, source, "enter")
	archive := source.transfer.(*fakeTransfer).saved

	tr := &fakeTransfer{offer: archive, offerAs: "mine.json"}
	m := backupModel(t, tr)
	for range 2 {
		m.backup.point(backupRowLoad)
		_, cmd := m.Update(key("enter"))
		drain(t, m, cmd)
	}

	frame := m.View().Content
	if !strings.Contains(frame, "nothing new in that backup") {
		t.Errorf("a repeated load does not say it did nothing:\n%s", frame)
	}
}

// A file that is not an archive is refused on the screen, and the store is left
// exactly as it was.
func TestBackupLoadRefusesAStrangeFile(t *testing.T) {
	tr := &fakeTransfer{offer: []byte("this is not a backup"), offerAs: "notes.txt"}
	m := backupModel(t, tr)
	playOne(t, m, "crane")

	m.backup.point(backupRowLoad)
	_, cmd := m.Update(key("enter"))
	drain(t, m, cmd)

	if !strings.Contains(plain(m.View().Content), "backup:") {
		t.Errorf("the screen does not report the refusal:\n%s", m.View().Content)
	}
	if !strings.Contains(m.backup.failure, "not a surmise.backup file") {
		t.Errorf("failure = %q, want the reason the file was refused", m.backup.failure)
	}
	games, err := m.store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Errorf("the store holds %d records, want the one puzzle it started with", len(games))
	}
}

// A picker closed without choosing is not a failure, and must not be reported
// as one: pressing escape in a file dialog is an ordinary thing to do.
func TestBackupLoadCancelledIsNotAnError(t *testing.T) {
	m := backupModel(t, &fakeTransfer{}) // offers nothing, with no error

	m.backup.point(backupRowLoad)
	_, cmd := m.Update(key("enter"))
	drain(t, m, cmd)

	if m.backup.failure != "" {
		t.Errorf("a cancelled picker reported an error: %q", m.backup.failure)
	}
	frame := m.View().Content
	if !strings.Contains(frame, "no file chosen") {
		t.Errorf("the screen does not report the cancellation:\n%s", frame)
	}
}

// A platform that cannot write says why, on the screen, rather than failing
// silently.
func TestBackupSaveReportsAFailure(t *testing.T) {
	m := backupModel(t, &fakeTransfer{saveErr: errors.New("the disk is full")})

	frame := send(t, m, "enter")
	if !strings.Contains(frame, "the disk is full") {
		t.Errorf("the screen does not report a refused save:\n%s", frame)
	}
}

// While a picker is open the screen says what it is waiting for. A browser's
// file dialog can sit there for as long as the player likes, and a screen that
// looked idle would read as a button that did nothing.
func TestBackupSaysItIsWaiting(t *testing.T) {
	m := backupModel(t, &fakeTransfer{})

	m.backup.point(backupRowLoad)
	m.Update(key("enter")) // the command is deliberately not run

	if frame := m.View().Content; !strings.Contains(frame, "waiting for a file") {
		t.Errorf("the screen does not say it is waiting:\n%s", frame)
	}
}

// A file that arrives after the player has left is dropped. It was asked for
// from a screen that is no longer showing, and writing a history into the store
// with nothing on screen to report it is worse than losing the request.
func TestBackupFileArrivingLateIsDropped(t *testing.T) {
	source := backupModel(t, &fakeTransfer{})
	playOne(t, source, "crane")
	send(t, source, "enter")
	archive := source.transfer.(*fakeTransfer).saved

	m := backupModel(t, &fakeTransfer{offer: archive, offerAs: "mine.json"})
	m.backup.point(backupRowLoad)
	_, cmd := m.Update(key("enter"))

	m.screen = screenMenu // the player went back while the picker was open
	if cmd != nil {
		m.Update(cmd())
	}

	games, err := m.store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 0 {
		t.Errorf("a file that arrived after the screen was left restored %d records", len(games))
	}
}

// The archive carries the preferences, and a load fills in only what this
// install had not chosen. The screen names what it filled.
func TestBackupLoadFillsUnsetPreferences(t *testing.T) {
	source := backupModel(t, &fakeTransfer{})
	ss, ok := source.store.(interface {
		SaveSettings(store.Settings) error
	})
	if !ok {
		t.Fatal("the test store cannot save settings")
	}
	if err := ss.SaveSettings(store.Settings{DisplayName: "them"}); err != nil {
		t.Fatal(err)
	}
	send(t, source, "enter")
	archive := source.transfer.(*fakeTransfer).saved

	m := backupModel(t, &fakeTransfer{offer: archive, offerAs: "mine.json"})
	m.backup.point(backupRowLoad)
	_, cmd := m.Update(key("enter"))
	drain(t, m, cmd)

	if got := m.settingsOf().DisplayName; got != "them" {
		t.Errorf("display name = %q, want the archive's answer to a question nobody had answered", got)
	}
	if frame := m.View().Content; !strings.Contains(frame, "filled in display name") {
		t.Errorf("the screen does not name what it filled in:\n%s", frame)
	}
}

// Both rows are click targets, and clicking one does what pressing enter on it
// does. Mouse-only play is a rule of this UI, not a nicety.
func TestBackupRowsAreClickable(t *testing.T) {
	tr := &fakeTransfer{}
	m := backupModel(t, tr)
	playOne(t, m, "crane")

	click(t, m, action{kind: actBackupSave})
	if len(tr.saved) == 0 {
		t.Error("clicking save wrote nothing")
	}
	if m.backup.cursor != backupRowSave {
		t.Errorf("cursor = %d after clicking save, want it on the row that acted", m.backup.cursor)
	}

	// The load row's own command is returned by dispatch, so it is run the way
	// the framework would run it.
	tr.offer, tr.offerAs = tr.saved, "mine.json"
	r := target(t, m, action{kind: actBackupLoad})
	_, cmd := m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
	drain(t, m, cmd)

	if tr.loadCall != 1 {
		t.Errorf("clicking load asked for %d files, want 1", tr.loadCall)
	}
	if m.backup.cursor != backupRowLoad {
		t.Errorf("cursor = %d after clicking load, want it on the row that acted", m.backup.cursor)
	}
}

// esc leaves, and the next opening starts clean: a report of what happened an
// hour ago says nothing about what is on the machine now.
func TestBackupScreenOpensClean(t *testing.T) {
	m := backupModel(t, &fakeTransfer{})
	send(t, m, "enter") // save, leaving a report
	if len(m.backup.report) == 0 {
		t.Fatal("the save left no report to clear")
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("esc left the screen at %v, want the menu", m.screen)
	}
	m.menu.point(menuIndex(t, m, choiceBackup, 0))
	send(t, m, "enter")

	if len(m.backup.report) != 0 || m.backup.cursor != backupRowSave {
		t.Errorf("the screen reopened with %v at row %d, want it clean", m.backup.report, m.backup.cursor)
	}
}
