package stats

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

func played(id string, length int, status game.Status, guesses []string, elapsed time.Duration, at time.Time) *game.Game {
	return &game.Game{
		ID:          id,
		Length:      length,
		Status:      status,
		MaxAttempts: length + 1,
		Guesses:     guesses,
		ElapsedMS:   elapsed.Milliseconds(),
		UpdatedAt:   at,
	}
}

// trendGames is the five-letter fixture of the first test, in completion
// order, one minute apart, followed by the noise a trend must leave out.
func trendGames() []*game.Game {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	games := []*game.Game{
		played("a", 5, game.Won, []string{"slate", "trace", "crane"}, 60*time.Second, at(0)),
		played("b", 5, game.Won, []string{"crane", "slate"}, 41*time.Second, at(1)),
		played("c", 5, game.Lost, []string{"slate", "xxxxx", "xxxxx", "xxxxx", "xxxxx", "xxxxx"}, 120*time.Second, at(2)),
		played("d", 5, game.Won, []string{"slate", "trace", "grace", "crane"}, 90*time.Second, at(3)),
	}
	custom := played("n1", 5, game.Won, []string{"slate"}, time.Second, at(4))
	custom.Custom = true
	room := played("n2", 5, game.Won, []string{"slate"}, time.Second, at(5))
	room.Room = &game.RoomInfo{}
	deleted := played("n3", 5, game.Won, []string{"slate"}, time.Second, at(6))
	deleted.Deleted = true
	open := played("n4", 5, game.InProgress, []string{"slate"}, time.Second, at(7))
	four := played("n5", 4, game.Won, []string{"slat"}, time.Second, at(8))
	return append(games, custom, room, deleted, open, four)
}

func TestTrendsForm(t *testing.T) {
	tr := Trends(trendGames(), 5)
	want := Form{Games: 4, Won: 3, WinRate: 0.75, AvgAttempts: 3, AvgTime: 191 * time.Second / 3}
	if tr.AllTime != want {
		t.Errorf("AllTime = %+v, want %+v", tr.AllTime, want)
	}
	if tr.Recent != tr.AllTime {
		t.Errorf("Recent = %+v, want %+v", tr.Recent, tr.AllTime)
	}
	if tr.Length != 5 {
		t.Errorf("Length = %d, want 5", tr.Length)
	}
}

func TestTrendsRecentIsTheLatestTwenty(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var games []*game.Game
	for i := range 25 {
		status := game.Won
		if i < 5 {
			status = game.Lost
		}
		games = append(games, played(fmt.Sprintf("g%02d", i), 5, status,
			[]string{"slate", "crane"}, 30*time.Second, t0.Add(time.Duration(i)*time.Minute)))
	}
	tr := Trends(games, 5)
	if tr.Recent.Games != 20 || tr.Recent.Won != 20 {
		t.Errorf("Recent = %d games, %d won, want 20 and 20", tr.Recent.Games, tr.Recent.Won)
	}
	if tr.AllTime.Games != 25 || tr.AllTime.Won != 20 {
		t.Errorf("AllTime = %d games, %d won, want 25 and 20", tr.AllTime.Games, tr.AllTime.Won)
	}
}

func TestTrendsBests(t *testing.T) {
	games := trendGames()
	tr := Trends(games, 5)
	if tr.Fastest == nil || tr.Fastest.ID != "b" {
		t.Fatalf("Fastest = %+v, want game b", tr.Fastest)
	}
	if tr.Fewest == nil || tr.Fewest.ID != "b" || tr.Fewest.Attempts != 2 {
		t.Fatalf("Fewest = %+v, want game b with 2 guesses", tr.Fewest)
	}

	later := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	games = append(games, played("e", 5, game.Won, []string{"slate", "crane"}, 30*time.Second, later))
	tr = Trends(games, 5)
	if tr.Fewest.ID != "b" {
		t.Errorf("Fewest = %q after a tie, want b (the first to get there)", tr.Fewest.ID)
	}
	if tr.Fastest.ID != "e" {
		t.Errorf("Fastest = %q, want e (30s)", tr.Fastest.ID)
	}

	// A win with no recorded time is never the fastest.
	games = append(games, played("f", 5, game.Won, []string{"slate", "crane"}, 0, later.Add(time.Hour)))
	tr = Trends(games, 5)
	if tr.Fastest.ID != "e" {
		t.Errorf("Fastest = %q with an untimed win, want e", tr.Fastest.ID)
	}
	only := []*game.Game{played("f", 5, game.Won, []string{"slate", "crane"}, 0, later)}
	if tr := Trends(only, 5); tr.Fastest != nil {
		t.Errorf("Fastest = %+v for an untimed win alone, want nil", tr.Fastest)
	}
}

func TestTrendsOpeners(t *testing.T) {
	tr := Trends(trendGames(), 5)
	want := []Opener{
		{Word: "slate", Games: 3, Won: 2, WinRate: 2.0 / 3, AvgAttempts: 3.5},
		{Word: "crane", Games: 1, Won: 1, WinRate: 1, AvgAttempts: 2},
	}
	if !reflect.DeepEqual(tr.Openers, want) {
		t.Errorf("Openers = %+v, want %+v", tr.Openers, want)
	}

	// Seven distinct openers: five come back, by games, then wins, then word.
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	games := []*game.Game{
		played("1", 5, game.Won, []string{"zesty"}, time.Second, at(1)),
		played("2", 5, game.Won, []string{"zesty"}, time.Second, at(2)),
		played("3", 5, game.Won, []string{"mango"}, time.Second, at(3)),
		played("4", 5, game.Lost, []string{"mango"}, time.Second, at(4)),
		played("5", 5, game.Won, []string{"apple"}, time.Second, at(5)),
		played("6", 5, game.Won, []string{"berry"}, time.Second, at(6)),
		played("7", 5, game.Lost, []string{"cacao"}, time.Second, at(7)),
		played("8", 5, game.Lost, []string{"dates"}, time.Second, at(8)),
		played("9", 5, game.Lost, []string{"elder"}, time.Second, at(9)),
	}
	var got []string
	for _, o := range Trends(games, 5).Openers {
		got = append(got, o.Word)
	}
	wantWords := []string{"zesty", "mango", "apple", "berry", "cacao"}
	if !reflect.DeepEqual(got, wantWords) {
		t.Errorf("openers = %v, want %v", got, wantWords)
	}
}

func TestTrendsAgreesWithTheProfile(t *testing.T) {
	games := trendGames()
	m := ComputeAt(games, daily.Today()).ByLength[5]
	a := Trends(games, 5).AllTime
	if m.WinRate != a.WinRate || m.AvgAttempts != a.AvgAttempts || m.AvgTime != a.AvgTime {
		t.Errorf("profile = %+v, trends = %+v", m, a)
	}
}

func TestTrendsLeavesTheGamesAlone(t *testing.T) {
	games := trendGames()
	for i, j := 0, len(games)-1; i < j; i, j = i+1, j-1 {
		games[i], games[j] = games[j], games[i]
	}
	before := append([]*game.Game(nil), games...)
	Trends(games, 5)
	for i := range games {
		if games[i] != before[i] {
			t.Fatalf("games[%d] changed from %q to %q", i, before[i].ID, games[i].ID)
		}
	}
}

func TestIsBest(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	first := played("a", 5, game.Won, []string{"slate", "trace", "crane"}, 60*time.Second, t0)
	if f, w := IsBest([]*game.Game{first}, first); f || w {
		t.Errorf("IsBest of a first win = (%v, %v), want (false, false)", f, w)
	}

	second := played("b", 5, game.Won, []string{"slate", "crane"}, 40*time.Second, t0.Add(time.Minute))
	games := []*game.Game{first, second}
	if f, w := IsBest(games, second); !f || !w {
		t.Errorf("IsBest of the better second win = (%v, %v), want (true, true)", f, w)
	}
	if f, w := IsBest(games, first); f || w {
		t.Errorf("IsBest of the worse first win = (%v, %v), want (false, false)", f, w)
	}

	custom := played("c", 5, game.Won, []string{"crane"}, time.Second, t0.Add(2*time.Minute))
	custom.Custom = true
	games = append(games, custom)
	if f, w := IsBest(games, custom); f || w {
		t.Errorf("IsBest of a custom win = (%v, %v), want (false, false)", f, w)
	}

	lost := played("d", 5, game.Lost, []string{"slate"}, time.Second, t0.Add(3*time.Minute))
	games = append(games, lost)
	if f, w := IsBest(games, lost); f || w {
		t.Errorf("IsBest of a loss = (%v, %v), want (false, false)", f, w)
	}
}
