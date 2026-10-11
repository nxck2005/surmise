package backup

// The room-round rules of an archive: a finished round moves with the history
// it was taken with, and the active-rooms list stays home.

import (
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

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
				Rows: roomRankRows(6)},
			{Seat: 2, Name: "nick", Solved: true, Guesses: 2, ElapsedMS: 81_000,
				Rows: [][]game.Mark{{0, 0, 2, 0, 2}, {2, 2, 2, 2, 2}}},
		},
	}
	g, err := game.NewRoomRound(info, "crane", []string{"slate", "crane"}, 81_000, finishedAt)
	if err != nil {
		t.Fatalf("NewRoomRound: %v", err)
	}
	return g
}

// roomRankRows is rows unplayed rows of a board the other seat spent.
func roomRankRows(n int) [][]game.Mark {
	rows := make([][]game.Mark, n)
	for i := range rows {
		rows[i] = make([]game.Mark, 5)
	}
	return rows
}

// TestRoomRoundsSurviveABackup: the round is a record like any other, so the
// restore puts it back with its room marker intact.
func TestRoomRoundsSurviveABackup(t *testing.T) {
	from := newStore(t)
	g := roomRound(t)
	if err := from.Save(g); err != nil {
		t.Fatal(err)
	}
	b := buildFrom(t, from, store.Settings{}, nil)

	to := newStore(t)
	res, err := Apply(b, to, store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.PuzzlesAdded != 1 {
		t.Fatalf("added %d records, want one", res.PuzzlesAdded)
	}
	on, err := to.Load(g.ID)
	if err != nil {
		t.Fatalf("Load after Apply: %v", err)
	}
	if on.Room == nil || on.Room.Code != "K7M2QX" || on.Room.Seat != 2 || on.Status != game.Won {
		t.Errorf("restored round = %+v, status %v; want the one archived", on.Room, on.Status)
	}
}

// TestBackupsCarryNoActiveRooms: rooms.json is how the player gets back into
// a room, and it is nobody else's business — the seat keys stay on the
// machine that joined.
func TestBackupsCarryNoActiveRooms(t *testing.T) {
	s := newStore(t)
	const key = "0123456789abcdef0123456789abcdef"
	rs, ok := s.(interface {
		SaveRooms([]store.RoomEntry) error
	})
	if !ok {
		t.Fatal("the store cannot save rooms, so this test cannot run")
	}
	if err := rs.SaveRooms([]store.RoomEntry{{
		Code:     "K7M2QX",
		Key:      key,
		Mode:     "live",
		JoinedAt: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatal(err)
	}
	b := buildFrom(t, s, store.Settings{}, nil)
	if strings.Contains(string(b), key) {
		t.Error("the archive carried a seat key out of the install")
	}
}
