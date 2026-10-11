package game

import (
	"reflect"
	"testing"
	"time"
)

// validRoomInfo is the RoomInfo of a finished 5-letter round, seat 2, played
// by "nick" against "sam". Every room test starts from a game built with it.
func validRoomInfo() RoomInfo {
	return RoomInfo{
		Code:  "K7M2QX",
		Mode:  "live",
		Round: 1,
		Seat:  2,
		Ranking: []RoomRank{
			{
				Seat:      1,
				Name:      "sam",
				Solved:    false,
				Guesses:   6,
				ElapsedMS: 600_000,
				Rows:      scoredRows("slate", "adieu", "pious", "trope", "lymph", "ghost"),
			},
			{
				Seat:      2,
				Name:      "nick",
				Solved:    true,
				Guesses:   2,
				ElapsedMS: 81_000,
				Rows:      scoredRows("slate", "crane"),
			},
		},
	}
}

// scoredRows scores the words against the answer every room test uses.
func scoredRows(words ...string) [][]Mark {
	rows := make([][]Mark, len(words))
	for i, w := range words {
		rows[i] = Score(w, "crane")
	}
	return rows
}

func finishedRoomAt() time.Time {
	return time.Date(2026, 10, 11, 12, 34, 56, 0, time.UTC)
}

func TestRoomRoundIDIsStable(t *testing.T) {
	// Pinned on purpose: a saved round lives under this id. If this test fails,
	// the derivation changed, and every saved round would be orphaned — change
	// it back.
	want := "73144140-eaf5-8a2d-bb31-8f8cc2421824"
	if got := RoomRoundID("K7M2QX", 1, 2); got != want {
		t.Errorf("RoomRoundID(K7M2QX, 1, 2) = %s, want %s", got, want)
	}
	if !ValidID(want) {
		t.Errorf("a room round id %s is not a valid puzzle id", want)
	}
	if RoomRoundID("K7M2QX", 2, 2) == want || RoomRoundID("K7M2QX", 1, 3) == want {
		t.Error("two rounds or two seats shared an id")
	}
}

func TestNewRoomRoundBuildsAFinishedGame(t *testing.T) {
	g, err := NewRoomRound(validRoomInfo(), "crane", []string{"slate", "crane"}, 81_000, finishedRoomAt())
	if err != nil {
		t.Fatalf("NewRoomRound: %v", err)
	}
	if g.Status != Won {
		t.Errorf("status = %v, want won", g.Status)
	}
	if g.Attempts() != 2 {
		t.Errorf("attempts = %d, want 2", g.Attempts())
	}
	if want := Score("slate", "crane"); !reflect.DeepEqual(g.Marks[0], want) {
		t.Errorf("marks[0] = %v, want %v", g.Marks[0], want)
	}
	if info := validRoomInfo(); g.ID != RoomRoundID(info.Code, info.Round, info.Seat) {
		t.Errorf("id = %s, want a derived one", g.ID)
	}
	if g.CountsForStats() {
		t.Error("a room round counted for stats")
	}
	if g.Length != 5 || g.Answer != "crane" || g.MaxAttempts != 6 {
		t.Errorf("game shape = %d/%s/%d, want 5/crane/6", g.Length, g.Answer, g.MaxAttempts)
	}
	if !g.StartedAt.Equal(finishedRoomAt().Add(-81_000 * time.Millisecond)) {
		t.Errorf("startedAt = %v, want the finishedAt less 81s", g.StartedAt)
	}
}

func TestNewRoomRoundOutOfTime(t *testing.T) {
	g, err := NewRoomRound(validRoomInfo(), "crane", nil, 600_000, finishedRoomAt())
	if err != nil {
		t.Fatalf("NewRoomRound: %v", err)
	}
	if g.Status != Lost {
		t.Errorf("status = %v, want lost", g.Status)
	}
	if g.Attempts() != 0 {
		t.Errorf("attempts = %d, want none", g.Attempts())
	}
}

func TestRoomValidation(t *testing.T) {
	valid, err := NewRoomRound(validRoomInfo(), "crane", []string{"slate", "crane"}, 81_000, finishedRoomAt())
	if err != nil {
		t.Fatalf("NewRoomRound: %v", err)
	}

	breaks := []struct {
		name string
		mut  func(g *Game)
		want string
	}{
		{"another origin", func(g *Game) { g.Custom = true },
			"game: room round has another puzzle origin"},
		{"five characters", func(g *Game) { g.Room.Code = "K7M2Q" },
			"game: room code is not valid"},
		{"unknown mode", func(g *Game) { g.Room.Mode = "race" },
			`game: room mode "race" is not valid`},
		{"round 0", func(g *Game) { g.Room.Round = 0 },
			"game: room round or seat is out of range"},
		{"key on a live round", func(g *Game) { g.Room.Key = "1234567890abcdef1234567890abcdef" },
			"game: room key is not valid"},
		{"unfinished", func(g *Game) { g.Status = InProgress },
			"game: room round is not finished"},
		{"wrong id", func(g *Game) { g.ID = RoomRoundID(g.Room.Code, 2, g.Room.Seat) },
			"game: room round id does not match"},
		{"a second over the clock", func(g *Game) { g.ElapsedMS = 600_001 },
			"game: room round time is out of range"},
		{"no ranking", func(g *Game) { g.Room.Ranking = nil },
			"game: room ranking size is out of range"},
		{"a name nobody cleaned", func(g *Game) { g.Room.Ranking[0].Name = "bad name!" },
			"game: room ranking entry 0 is not valid"},
		{"not on the ranking", func(g *Game) { g.Room.Ranking[1].Seat = 3 },
			"game: room ranking does not include this player"},
	}
	for _, tt := range breaks {
		t.Run(tt.name, func(t *testing.T) {
			g := *valid // one field away from the valid game per row
			// The rows share the Room pointer, so each row gets its own deep
			// copy of it: a break must not survive into the next row.
			clone := *g.Room
			clone.Ranking = append([]RoomRank(nil), g.Room.Ranking...)
			g.Room = &clone
			mut := tt.mut
			mut(&g)
			if err := g.Validate(); err == nil {
				t.Fatalf("Validate accepted a broken %s", tt.name)
			} else if err.Error() != tt.want {
				t.Errorf("Validate error = %q, want %q", err.Error(), tt.want)
			}
		})
	}

	// The valid game they all start from is still valid.
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate rejected the valid game: %v", err)
	}
}

func TestRoomTombstoneKeepsTheMarker(t *testing.T) {
	valid, err := NewRoomRound(validRoomInfo(), "crane", []string{"slate", "crane"}, 81_000, finishedRoomAt())
	if err != nil {
		t.Fatal(err)
	}
	tb := valid.Tombstone()
	if tb.Room == nil {
		t.Fatal("a roomRound tombstone lost its room marker")
	}
	if empty := (RoomInfo{}); !reflect.DeepEqual(*tb.Room, empty) {
		t.Errorf("tombstone room = %+v, want an empty marker", *tb.Room)
	}
	if tb.CountsForStats() {
		t.Error("a deleted room round counted for stats")
	}
	if !tb.Deleted {
		t.Error("tombstone is not deleted")
	}
}
