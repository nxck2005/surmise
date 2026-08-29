package ui

import (
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/challenge"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
)

const testChallenge = "4500-820C-20A1-G73J"

func parsedChallenge(t *testing.T) challenge.Code {
	t.Helper()
	c, err := challenge.Parse(testChallenge)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSocialPlayOpensChallengeAndCustomChoices(t *testing.T) {
	m := newModel(t)
	m.menu.cursor = menuIndex(t, m, choiceSocial, 0)
	send(t, m, "enter")
	if m.screen != screenSocial {
		t.Fatalf("screen = %v, want social play", m.screen)
	}

	send(t, m, "enter")
	if m.screen != screenChallenge || !m.challenge.creating || m.challenge.code.String() == "" {
		t.Fatalf("new challenge did not open a generated code: screen=%v challenge=%+v", m.screen, m.challenge)
	}
	send(t, m, "esc", "down", "down", "enter")
	if m.screen != screenCustom {
		t.Fatalf("custom choice opened screen %v", m.screen)
	}
}

func TestEnteredChallengeOpensDeterministicBoard(t *testing.T) {
	m := newModel(t)
	m.challenge = newChallengeJoin(testChallenge)
	m.screen = screenChallenge
	send(t, m, "enter")

	c := parsedChallenge(t)
	if m.screen != screenGame || m.game.g.ID != c.ID() || m.game.g.Challenge == nil {
		t.Fatalf("entered challenge opened screen=%v game=%+v", m.screen, m.game)
	}
	if m.game.persisted {
		t.Error("an unplayed challenge was persisted")
	}
	list, err := m.store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("unplayed challenge list = %+v, %v", list, err)
	}
}

func TestChallengeStartupAndInvalidStartup(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{Challenge: testChallenge, Splash: splashOff, Motion: motionOffName})
	if m.screen != screenGame || m.game.g.ID != parsedChallenge(t).ID() {
		t.Fatalf("valid startup challenge opened screen=%v game=%+v", m.screen, m.game)
	}

	bad := New(s, nil, Options{Challenge: "not-a-code", Splash: splashOff, Motion: motionOffName})
	if bad.screen != screenChallenge || bad.challenge.msg == "" || !bad.challenge.entry.editing {
		t.Fatalf("invalid startup challenge = screen %v, msg %q, editing %v",
			bad.screen, bad.challenge.msg, bad.challenge.entry.editing)
	}
}

func TestChallengeResumesAndDeletedChallengeIsSpent(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Challenge: testChallenge, Splash: splashOff, Motion: motionOffName}
	m := New(s, nil, opts)
	c := parsedChallenge(t)
	answer, err := c.Answer()
	if err != nil {
		t.Fatal(err)
	}
	send(t, m, strings.Split(answer, "")...)
	send(t, m, "enter")
	if m.screen != screenResult || m.game.g.Status != game.Won {
		t.Fatalf("challenge did not finish: screen=%v status=%v", m.screen, m.game.g.Status)
	}

	reopened, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	resumed := New(reopened, nil, opts)
	if resumed.screen != screenGame || !resumed.game.persisted || resumed.game.g.Status != game.Won {
		t.Fatalf("saved challenge did not resume: screen=%v game=%+v", resumed.screen, resumed.game)
	}
	if err := reopened.Delete(c.ID()); err != nil {
		t.Fatal(err)
	}
	spent := New(reopened, nil, opts)
	if spent.screen != screenChallenge || spent.challenge.msg != errChallengeSpent.Error() {
		t.Fatalf("spent challenge = screen %v, msg %q", spent.screen, spent.challenge.msg)
	}
}

func TestChallengeCountsListsAndShares(t *testing.T) {
	m := newModel(t)
	c := parsedChallenge(t)
	m.challenge = newChallengeJoin(testChallenge)
	m.screen = screenChallenge
	m.openChallenge(c)
	answer, _ := c.Answer()
	send(t, m, strings.Split(answer, "")...)
	send(t, m, "enter")

	games, err := m.store.All()
	if err != nil {
		t.Fatal(err)
	}
	summary := stats.Compute(games)
	if summary.Played != 1 || summary.Won != 1 {
		t.Errorf("challenge summary = %+v, want one win", summary)
	}
	list, err := m.store.List()
	if err != nil || len(list) != 1 || !list[0].Challenge {
		t.Fatalf("challenge list = %+v, %v", list, err)
	}
	m.list.reload(m.store)
	if view := plain(m.list.view(nil)); !strings.Contains(view, "challenge") {
		t.Errorf("challenge is not labelled in the list:\n%s", view)
	}

	shared := shareResult(m.game.g)
	if !strings.Contains(shared, testChallenge) || strings.Contains(shared, answer) {
		t.Errorf("challenge share leaked or omitted data:\n%s", shared)
	}
	send(t, m, "n")
	if m.screen != screenSocial {
		t.Fatalf("challenge next opened screen %v, want social", m.screen)
	}
}

func TestChallengeCannotBeRestarted(t *testing.T) {
	m := newModel(t)
	m.challenge = newChallengeJoin(testChallenge)
	m.screen = screenChallenge
	m.openChallenge(parsedChallenge(t))
	id := m.game.g.ID
	send(t, m, "tab", "enter")
	if m.game.g.ID != id || m.game.confirmNew {
		t.Errorf("tab changed challenge %q to %q or left confirmation armed", id, m.game.g.ID)
	}
}

func TestChallengeByClickingOnly(t *testing.T) {
	m := newModel(t)
	draw(t, m)
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceSocial, 0)})
	draw(t, m)
	click(t, m, action{kind: actSocialChoice, index: socialNewChallenge})
	if m.screen != screenChallenge || !m.challenge.creating {
		t.Fatalf("clicking new challenge opened screen %v", m.screen)
	}
	draw(t, m)
	old := m.challenge.code.String()
	click(t, m, action{kind: actChallengeGenerate})
	if m.challenge.code.String() == old {
		t.Error("new-code click did not replace the code")
	}
	draw(t, m)
	click(t, m, action{kind: actChallengeCopy})
	if !m.challenge.copied {
		t.Error("copy click did not report its request")
	}
	draw(t, m)
	click(t, m, action{kind: actChallengeOpen})
	if m.screen != screenGame || m.game.g.Challenge == nil {
		t.Fatalf("play click opened screen %v", m.screen)
	}
}

func TestEnteredChallengeFieldHasMouseParity(t *testing.T) {
	m := newModel(t)
	m.openSocialScreen()
	draw(t, m)
	click(t, m, action{kind: actSocialChoice, index: socialEnterChallenge})
	if m.screen != screenChallenge || !m.challenge.entry.editing {
		t.Fatalf("enter-code click opened screen %v, editing %v", m.screen, m.challenge.entry.editing)
	}
	send(t, m, strings.Split(testChallenge, "")...)
	draw(t, m)
	click(t, m, action{kind: actFieldDone, index: challengeRowCode})
	if m.screen != screenGame || m.game.g.ID != parsedChallenge(t).ID() {
		t.Fatalf("field play click opened screen %v, game %+v", m.screen, m.game)
	}
}

func TestSocialScreensFitShortTerminal(t *testing.T) {
	m := newModel(t)
	m.openSocialScreen()
	if frame := drawAt(t, m, 18); len(strings.Split(frame, "\n")) > 18 {
		t.Errorf("social screen overflowed 18 rows:\n%s", frame)
	}
	challenge, err := newChallengeCreate(5)
	if err != nil {
		t.Fatal(err)
	}
	m.challenge, m.screen = challenge, screenChallenge
	if frame := drawAt(t, m, 18); len(strings.Split(frame, "\n")) > 18 {
		t.Errorf("challenge screen overflowed 18 rows:\n%s", frame)
	}
}
