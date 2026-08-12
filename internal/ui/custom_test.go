package ui

import (
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// offList is a five-letter word no list carries, which is the whole point of
// the "any word" choice.
const offList = "nishu"

// plain strips the styling, so a test asks what the frame says rather than how
// it is coloured.
func plain(frame string) string { return sgr.ReplaceAllString(frame, "") }

// openCustom walks in through the menu, the way a player does, so the test
// covers the wiring as well as the screen.
func openCustom(t *testing.T, m *Model) {
	t.Helper()
	m.screen = screenMenu
	m.menu.cursor = menuIndex(t, m, choiceCustom, 0)
	send(t, m, "enter")
	if m.screen != screenCustom {
		t.Fatalf("screen = %v after choosing custom, want screenCustom", m.screen)
	}
}

// typeSecret types a word and presses enter, which both keeps the word and
// hands the terminal over — the gesture the screen is built around.
func typeSecret(t *testing.T, m *Model, word string) {
	t.Helper()
	m.custom.cursor = customRowSecret
	send(t, m, "enter")
	if !m.custom.secret.editing {
		t.Fatal("enter on the secret row did not start typing")
	}
	send(t, m, strings.Split(word, "")...)
	send(t, m, "enter")
}

func TestCustomOpensTheWordThatWasTyped(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	typeSecret(t, m, "crane")

	if m.screen != screenGame {
		t.Fatalf("screen = %v after handing over, want screenGame", m.screen)
	}
	if m.game.g.Answer != "crane" {
		t.Errorf("answer = %q, want crane", m.game.g.Answer)
	}
	if !m.game.g.Custom {
		t.Error("the board is not marked custom, so it would count in the figures")
	}
	if m.game.g.Daily != "" {
		t.Errorf("Daily = %q, want empty", m.game.g.Daily)
	}
}

func TestTheSecretLeavesTheScreenAtTheHandover(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	m.custom.cursor = customRowSecret
	send(t, m, "enter")
	send(t, m, "c", "r", "a", "n", "e")

	// Visible while it is being chosen: a typo has to be correctable.
	if view := plain(m.View().Content); !strings.Contains(view, "crane") {
		t.Error("the word was hidden while it was being typed")
	}

	send(t, m, "enter")

	if m.custom.secret.value != "" {
		t.Errorf("the screen still holds the word: %q", m.custom.secret.value)
	}
	if view := plain(m.View().Content); strings.Contains(view, "crane") {
		t.Errorf("the word is still on screen after the hand-over:\n%s", view)
	}
	// Nor may it come back by returning to the screen.
	send(t, m, "esc")
	openCustom(t, m)
	if view := plain(m.View().Content); strings.Contains(view, "crane") {
		t.Errorf("the word came back with the screen:\n%s", view)
	}
}

func TestCustomRefusesAWordItCannotUse(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)

	// Too short: the field holds four of five letters.
	typeSecret(t, m, "cran")
	if m.screen != screenCustom {
		t.Fatal("a short word was accepted")
	}
	if !strings.Contains(plain(m.View().Content), "needs 5 letters") {
		t.Errorf("no refusal shown for a short word:\n%s", plain(m.View().Content))
	}

	// Off the list, with "any word" off.
	m.custom = newCustomScreen(5)
	typeSecret(t, m, offList)
	if m.screen != screenCustom {
		t.Fatal("an off-list word was accepted with 'any word' off")
	}
	if !strings.Contains(plain(m.View().Content), "not in word list") {
		t.Errorf("no refusal shown for an off-list word:\n%s", plain(m.View().Content))
	}
}

func TestAnyWordLetsAnOffListSecretBePlayedAndWon(t *testing.T) {
	if words.IsValidGuess(5, offList) {
		t.Skipf("%q reached the word list; pick another non-word", offList)
	}
	m := newModel(t)
	openCustom(t, m)

	m.custom.cursor = customRowAnyWord
	send(t, m, "right")
	if !m.custom.anyWord {
		t.Fatal("stepping the 'any word' row did not turn it on")
	}
	typeSecret(t, m, offList)

	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the board", m.screen)
	}

	// The guesser can type the secret even though no list carries it — without
	// that the board could be scored forever and never won.
	send(t, m, strings.Split(offList, "")...)
	send(t, m, "enter")
	if m.game.g.Status != game.Won {
		t.Fatalf("status = %q after guessing the secret, want won", m.game.g.Status)
	}
}

func TestChangingTheModeClearsTheWord(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	// Committed rather than typed here: enter would hand over, and what this
	// test is about is what a mode change does to a word already sitting there.
	m.custom.secret.value = "crane"

	m.custom.cursor = customRowMode
	send(t, m, "right")
	if m.custom.secret.value != "" {
		t.Errorf("the word survived a mode change: %q", m.custom.secret.value)
	}
	if m.custom.secret.max != m.custom.length {
		t.Errorf("field width = %d, want %d", m.custom.secret.max, m.custom.length)
	}
}

func TestEscapeDiscardsTheDraft(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	m.custom.cursor = customRowSecret
	send(t, m, "enter", "c", "r", "a", "esc")

	if m.custom.secret.editing {
		t.Error("esc left the editor open")
	}
	if m.custom.secret.value != "" {
		t.Errorf("esc kept the draft: %q", m.custom.secret.value)
	}
	if m.screen != screenCustom {
		t.Error("esc left the screen as well as the draft")
	}
}

func TestCustomIsListedButCountsForNothing(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	typeSecret(t, m, "crane")
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.game.g.Status != game.Won {
		t.Fatalf("status = %q, want won", m.game.g.Status)
	}

	// It is saved and browsable, labelled for what it is.
	summaries, err := m.store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || !summaries[0].Custom {
		t.Fatalf("summaries = %+v, want one custom puzzle", summaries)
	}
	m.list.reload(m.store)
	m.screen = screenList
	if view := plain(m.View().Content); !strings.Contains(view, "custom") {
		t.Errorf("the list does not say what the puzzle is:\n%s", view)
	}

	// And it moves nothing on the profile.
	m.profile.reload(m.store, m.day, "", 0)
	m.screen = screenProfile
	view := plain(m.View().Content)
	if strings.Contains(view, "100%") {
		t.Errorf("a custom win reached the win rate:\n%s", view)
	}
}

func TestTabDoesNotDiscardAWordSomebodyElseChose(t *testing.T) {
	m := newModel(t)
	openCustom(t, m)
	typeSecret(t, m, "crane")

	send(t, m, "tab", "enter")
	if m.game.g.Answer != "crane" {
		t.Errorf("answer = %q after tab+enter, want the board to be kept", m.game.g.Answer)
	}
}

func TestCustomByClickingOnly(t *testing.T) {
	m := newModel(t)
	draw(t, m)
	m.screen = screenMenu
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceCustom, 0)})
	if m.screen != screenCustom {
		t.Fatalf("clicking the menu entry did not open the screen: %v", m.screen)
	}

	// Every control the keyboard has, the pointer has too.
	click(t, m, action{kind: actCustomNext, index: customRowAnyWord})
	if !m.custom.anyWord {
		t.Error("clicking the 'any word' row did not turn it on")
	}
	click(t, m, action{kind: actCustomPrev, index: customRowAnyWord})
	if m.custom.anyWord {
		t.Error("clicking the back arrow did not turn it off again")
	}

	click(t, m, action{kind: actFieldEdit, index: customRowSecret})
	if !m.custom.secret.editing {
		t.Fatal("clicking the secret row did not start typing")
	}
	send(t, m, "c", "r", "a", "n", "x")
	click(t, m, action{kind: actFieldBackspace, index: customRowSecret})
	send(t, m, "e")
	click(t, m, action{kind: actFieldDone, index: customRowSecret})
	if m.custom.secret.value != "crane" {
		t.Fatalf("value = %q, want crane", m.custom.secret.value)
	}

	click(t, m, action{kind: actCustomStart})
	if m.screen != screenGame || m.game.g.Answer != "crane" {
		t.Fatalf("clicking hand over did not open the board: screen=%v", m.screen)
	}
}
