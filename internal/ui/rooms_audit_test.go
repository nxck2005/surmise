package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
)

// finishThreeWay plays forum's round to a result with three rows: us first in
// one guess, priya second in three, and sam out of guesses after 1:35.
func finishThreeWay(t *testing.T) (*Model, *fakeConn) {
	t.Helper()
	m, _, conn := forum(t)
	withClock(t, &roundNow)
	draw(t, m)
	startLiveRound(t, m)
	send(t, m, "c", "r", "a", "n", "e")
	press(t, m, "enter")
	deliver(t, m, marksMsg(1, marksRowServer("crane", "crane")))
	deliver(t, m, doneMsg(1, true, 1, 2_000))
	deliver(t, m, resultsRound(roundNow, 1, "crane",
		rankEntry(1, "nick", true, 1, 2_000, scoreRows("crane")),
		rankEntry(3, "priya", true, 3, 125_000, scoreRows("slate", "ghost", "crane")),
		rankEntry(2, "sam", false, 6, 95_000, spentRows(5, 6)),
	))
	if m.screen != screenRoomResult {
		t.Fatalf("screen = %v, want the room result", m.screen)
	}
	return m, conn
}

func TestResultRowsAreExactAndAligned(t *testing.T) {
	m, _ := finishThreeWay(t)
	frame := plain(draw(t, m))
	rows := []string{
		" 1  › nick          1/6      2s",
		" 2    priya         3/6    2:05",
		" —    sam           X/6    1:35",
	}
	col := -1
	for _, want := range rows {
		var at int = -1
		for _, line := range strings.Split(frame, "\n") {
			if i := strings.Index(line, want); i >= 0 {
				at = lipgloss.Width(line[:i])
				break
			}
		}
		if at < 0 {
			t.Errorf("no ranking row %q in:\n%s", want, frame)
			continue
		}
		if col >= 0 && at != col {
			t.Errorf("row %q starts at column %d, the first at %d", want, at, col)
		}
		col = at
	}
}

func TestLiveResultHelpOffersRematchAndLeave(t *testing.T) {
	m, _ := finishThreeWay(t)
	frame := plain(draw(t, m))
	if want := "enter review · n rematch · l leave · esc menu"; !strings.Contains(frame, want) {
		t.Errorf("the help is not %q:\n%s", want, frame)
	}
}

func TestTheResultIsSizedWhenItOpens(t *testing.T) {
	m, _ := finishThreeWay(t)
	if m.roomResult.width != m.width || m.roomResult.height != m.height || m.height == 0 {
		t.Errorf("result size = %dx%d, terminal %dx%d",
			m.roomResult.width, m.roomResult.height, m.width, m.height)
	}
}

func TestASavedAsyncResultOffersRefresh(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	withClock(t, &roundNow)
	g, err := game.NewRoomRound(game.RoomInfo{
		Code: roomCode, Mode: "async", Round: 1, Seat: 1, Key: welcomeKey,
		Ranking: []game.RoomRank{
			{Seat: 1, Name: "nick", Solved: true, Guesses: 2, ElapsedMS: 81_000,
				Rows: scoreRows("slate", "crane")},
		},
	}, "crane", []string{"slate", "crane"}, 81_000, roundNow)
	if err != nil {
		t.Fatal(err)
	}
	m.openRoomResultFor(g)
	frame := plain(draw(t, m))
	if want := "enter review · f refresh · esc menu"; !strings.Contains(frame, want) {
		t.Errorf("the help is not %q:\n%s", want, frame)
	}
	if !strings.Contains(frame, "nick          2/6    1:21") {
		t.Errorf("the row does not score out of six:\n%s", frame)
	}
}

func TestReopeningAfterGoLandsOnTheBoard(t *testing.T) {
	m, _, _ := forum(t)
	withClock(t, &roundNow)
	deliver(t, m, countdownMsg(roundNow, roundNow.Add(3*time.Second), 1))
	press(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("screen = %v, want the menu", m.screen)
	}
	deliver(t, m, startRoundMsg(roundNow, roundNow.Add(10*time.Minute), 1))
	if m.screen != screenMenu {
		t.Fatalf("go moved a player who was on the menu to %v", m.screen)
	}
	m.showRoom()
	if m.screen != screenGame || m.game == nil || m.game.room != m.room {
		t.Errorf("reopening a running round shows %v, want its board", m.screen)
	}
}

func TestAWatcherStaysOnTheRoomAtGo(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	openJoinRoom(t, m, f)
	withClock(t, &roundNow)
	welcome := welcomeRoom(roomCode, "live", 5, true, "countdown", 3, welcomeKey,
		player(1, "nick", true, true, false),
		player(2, "sam", true, true, false),
		player(3, "priya", true, false, false),
	)
	welcome.Room.GoAt = roundNow.Add(3 * time.Second).UnixMilli()
	deliver(t, m, welcome)
	if m.screen != screenRoom {
		t.Fatalf("screen = %v, want the room", m.screen)
	}
	deliver(t, m, startRoundMsg(roundNow, roundNow.Add(10*time.Minute), 1))
	if m.screen != screenRoom || m.room.view.You.InRound {
		t.Errorf("a watcher went to %v (in round %v) at go", m.screen, m.room.view.You.InRound)
	}
	if m.room.view.Phase != "playing" {
		t.Errorf("phase = %q after go, want playing", m.room.view.Phase)
	}
}

func TestReviewingASavedRoundHasNoRestart(t *testing.T) {
	m, _ := finishThreeWay(t)
	press(t, m, "enter")
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the saved round's board", m.screen)
	}
	frame := plain(draw(t, m))
	if want := "enter result · esc menu"; !strings.Contains(frame, want) {
		t.Errorf("the help is not %q:\n%s", want, frame)
	}
	if strings.Contains(frame, "tab+enter") {
		t.Errorf("a room round offers a restart:\n%s", frame)
	}
}

func TestStandingsLinesAreOneWidth(t *testing.T) {
	away := online.PlayerView{Seat: 4, Name: "bartholomew", InRound: true}
	r := &roomSession{seat: 1, view: online.RoomView{Length: 5, Players: []online.PlayerView{
		{Seat: 1, Name: "nick", Connected: true, InRound: true},
		{Seat: 2, Name: "sam", Connected: true, InRound: true, Guesses: 1,
			Rows: scoreRows("slate")},
		{Seat: 3, Name: "", Connected: true, InRound: true},
		{Seat: 5, Name: "jo", Connected: true, InRound: true, Done: true, Solved: true,
			Guesses: 2, Rows: scoreRows("slate", "crane")},
		away,
	}}}
	for _, form := range []standingsForm{standingsFull, standingsCompact} {
		want := 19
		if form == standingsCompact {
			want = 13
		}
		for _, line := range strings.Split(renderStandings(r, form, 0), "\n") {
			if w := lipgloss.Width(line); w != want {
				t.Errorf("form %d: %q is %d wide, want %d", form, plain(line), w, want)
			}
		}
	}
}

func TestAClickDuringTheLeavePromptOpensNothing(t *testing.T) {
	f := &fakeOnline{}
	m := newOnlineModel(t, f, true)
	setRoomFile(t, m, roomCode)
	openSocial(t, m)
	m.social.cursor = 0
	press(t, m, "x")
	if !m.social.confirmLeave {
		t.Fatal("x did not arm the leave prompt")
	}
	runCmd(t, m, m.dispatch(action{kind: actSocialChoice, index: 0}))
	if m.screen != screenSocial || asked(f, "dial") {
		t.Errorf("a row opened under the prompt: screen %v, calls %v", m.screen, f.calls)
	}
}
