package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// roomRound is the game every store test here starts from: the same finished
// round the internal/game tests build, through the same constructor.
func roomRound(t *testing.T) *game.Game {
	t.Helper()
	finishedAt := time.Date(2026, 10, 11, 12, 34, 56, 0, time.UTC)
	info := game.RoomInfo{
		Code:  "K7M2QX",
		Mode:  "live",
		Round: 1,
		Seat:  2,
		Ranking: []game.RoomRank{
			{Seat: 1, Name: "sam", Solved: false, Guesses: 6, ElapsedMS: 600_000,
				Rows: roomRows("slate", "adieu", "pious", "trope", "lymph", "ghost")},
			{Seat: 2, Name: "nick", Solved: true, Guesses: 2, ElapsedMS: 81_000,
				Rows: roomRows("slate", "crane")},
		},
	}
	g, err := game.NewRoomRound(info, "crane", []string{"slate", "crane"}, 81_000, finishedAt)
	if err != nil {
		t.Fatalf("NewRoomRound: %v", err)
	}
	return g
}

func roomRows(words ...string) [][]game.Mark {
	rows := make([][]game.Mark, len(words))
	for i, w := range words {
		rows[i] = game.Score(w, "crane")
	}
	return rows
}

// testRoomEntry is the shape of one active room as it is stored.
func testRoomEntry(code string) RoomEntry {
	return RoomEntry{
		Code:     code,
		Key:      "0123456789abcdef0123456789abcdef",
		Mode:     "live",
		JoinedAt: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC),
	}
}

// kindStore is what the room tests need of a store.
type kindStore interface {
	Save(*game.Game) error
	Load(string) (*game.Game, error)
	Delete(string) error
	Rooms() []RoomEntry
	SaveRooms([]RoomEntry) error
	IDs() ([]string, error)
	List() ([]Summary, error)
	All() ([]*game.Game, error)
}

// rawByteAccess is how the tests reach the stored bytes and replace them.
type rawByteAccess interface {
	puzzle(id string) string
	setRooms([]byte) error
}

type jsonRoomsCase struct {
	dir string
}

func newJSONRoomsCase(t *testing.T) (kindStore, rawByteAccess) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, jsonRoomsCase{dir: dir}
}

func (c jsonRoomsCase) puzzle(id string) string {
	b, err := os.ReadFile(filepath.Join(c.dir, puzzleDir, id+".json"))
	if err != nil {
		return ""
	}
	return string(b)
}

func (c jsonRoomsCase) setRooms(b []byte) error {
	return os.WriteFile(filepath.Join(c.dir, roomsName), b, 0o600)
}

type kvRoomsCase struct{ kv KV }

func newKVRomsCase(t *testing.T) (kindStore, rawByteAccess) {
	t.Helper()
	kv := NewMemoryKV()
	return NewKV(kv), kvRoomsCase{kv: kv}
}

func (c kvRoomsCase) puzzle(id string) string {
	v, _ := c.kv.Get(kvPuzzleKey(id))
	return v
}

func (c kvRoomsCase) setRooms(b []byte) error {
	return c.kv.Set(kvRoomsKey, string(b))
}

// roomStores runs one case against both stores.
func roomStores(t *testing.T, fn func(t *testing.T, s kindStore, raw rawByteAccess)) {
	t.Helper()
	t.Run("json", func(t *testing.T) {
		s, raw := newJSONRoomsCase(t)
		fn(t, s, raw)
	})
	t.Run("kv", func(t *testing.T) {
		s, raw := newKVRomsCase(t)
		fn(t, s, raw)
	})
}

// TestRoomRoundRoundTrips: a room round is a puzzle like any other, whose
// bytes simply carry one more key.
func TestRoomRoundRoundTrips(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, raw rawByteAccess) {
		g := roomRound(t)
		if err := s.Save(g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Load(g.ID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !sameRound(got, g) {
			t.Errorf("Load = %+v, want %+v", got, g)
		}
		list, err := s.List()
		if err != nil || len(list) != 1 {
			t.Fatalf("List = %+v, %v; want one row", list, err)
		}
		if !list[0].Room {
			t.Error("List did not label the round a room round")
		}
		record := raw.puzzle(g.ID)
		if !strings.Contains(record, `"schema":1`) {
			t.Errorf("record carries no schema tag: %s", record)
		}
		if !strings.Contains(record, `"room":{`) {
			t.Errorf("record carries no room section: %s", record)
		}
	})
}

// sameRound compares a round on the pieces the round trip is expected to keep.
func sameRound(got, want *game.Game) bool {
	return got.ID == want.ID &&
		got.Length == want.Length &&
		got.Answer == want.Answer &&
		got.MaxAttempts == want.MaxAttempts &&
		got.Status == want.Status &&
		got.ElapsedMS == want.ElapsedMS &&
		got.Schema == schemaVersion &&
		slices.Equal(got.Guesses, want.Guesses) &&
		got.Room != nil &&
		got.Room.Code == want.Room.Code &&
		got.Room.Mode == want.Room.Mode &&
		got.Room.Round == want.Room.Round &&
		got.Room.Seat == want.Room.Seat &&
		got.Room.Key == want.Room.Key &&
		got.Room.Ranking != nil
}

// TestDeletingARoomRoundLeavesAMarker: the tombstone rules hold for a room
// round like any other, and a marker that forgot the origin would read as an
// ordinary loss.
func TestDeletingARoomRoundLeavesAMarker(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, raw rawByteAccess) {
		g := roomRound(t)
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(g.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		games, err := s.All()
		if err != nil || len(games) != 1 {
			t.Fatalf("All = %+v, %v; want the tombstone", games, err)
		}
		if games[0].Room == nil || games[0].Answer != "" {
			t.Errorf("tombstone = %+v, want a marker without the answer", games[0])
		}
		record := raw.puzzle(g.ID)
		if !strings.Contains(record, `"room":{}`) {
			t.Errorf("tombstone bytes carry no empty marker: %s", record)
		}
		if strings.Contains(record, `"crane"`) {
			t.Errorf("tombstone bytes kept the answer: %s", record)
		}
	})
}

func TestRoomsFileRoundTrips(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, _ rawByteAccess) {
		want := []RoomEntry{testRoomEntry("K7M2QX"), testRoomEntry("N4XYRZ")}
		if err := s.SaveRooms(want); err != nil {
			t.Fatalf("SaveRooms: %v", err)
		}
		got := s.Rooms()
		if !slices.Equal(got, want) {
			t.Errorf("Rooms = %+v, want %+v", got, want)
		}
	})
}

func TestRoomsFileRefusesBadInput(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, _ rawByteAccess) {
		six := make([]RoomEntry, MaxRooms+1)
		for i := range six {
			six[i] = testRoomEntry(entryCode(i))
		}
		if err := s.SaveRooms(six); err == nil {
			t.Error("SaveRooms accepted more rooms than an install may sit in")
		}
		bad := []RoomEntry{testRoomEntry("K7M2QX")}
		bad[0].Key = "abc"
		if err := s.SaveRooms(bad); err == nil {
			t.Error("SaveRooms accepted an entry with a mangled seat key")
		}
		if err := s.SaveRooms([]RoomEntry{testRoomEntry("K7M2QX"), testRoomEntry("K7M2QX")}); err == nil {
			t.Error("SaveRooms accepted the same room twice")
		}
		// A refusal must not have written anything.
		if got := s.Rooms(); len(got) != 0 {
			t.Errorf("Rooms = %+v, want nothing after refusals", got)
		}
	})
}

func TestRoomsFileHealsOnRead(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, raw rawByteAccess) {
		// Raw bytes in a shape no SaveRooms would write: one valid entry, one
		// invalid entry, one repeat of the valid code.
		valid := testRoomEntry("K7M2QX")
		bogus := testRoomEntry("N4XYRZ")
		bogus.Key = "abc"
		b, err := json.Marshal(roomsFile{Schema: schemaVersion, Rooms: []RoomEntry{valid, bogus, valid}})
		if err != nil {
			t.Fatal(err)
		}
		if err := raw.setRooms(b); err != nil {
			t.Fatal(err)
		}

		got := s.Rooms()
		if len(got) != 1 || got[0].Code != "K7M2QX" {
			t.Errorf("Rooms = %+v, want only the one valid entry", got)
		}
	})
}

func TestRoomsAreNotRecords(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, _ rawByteAccess) {
		if err := s.SaveRooms([]RoomEntry{testRoomEntry("K7M2QX")}); err != nil {
			t.Fatal(err)
		}
		ids, err := s.IDs()
		if err != nil || len(ids) != 0 {
			t.Errorf("IDs = %v, %v; want nothing", ids, err)
		}
		list, err := s.List()
		if err != nil || len(list) != 0 {
			t.Errorf("List = %+v, %v; want nothing", list, err)
		}
		games, err := s.All()
		if err != nil || len(games) != 0 {
			t.Errorf("All = %+v, %v; want nothing", games, err)
		}
	})
}

func TestUnfinishedRoomRoundIsRefused(t *testing.T) {
	roomStores(t, func(t *testing.T, s kindStore, _ rawByteAccess) {
		g := roomRound(t)
		g.Status = game.InProgress
		err := s.Save(g)
		if err == nil {
			t.Fatal("Save accepted an unfinished round over the room origin")
		}
		if !strings.Contains(err.Error(), "not finished") {
			t.Errorf("Save error = %v, want one about being not finished", err)
		}
	})
}

func entryCode(i int) string {
	codes := []string{"K7M2QX", "N4XYRZ", "Q8BGMK", "H2DXW4", "MJ3XSR", "TW4KVN"}
	return codes[i]
}
