package ui

// The online parts: consent checks, the one status request, the daily result
// post and the daily count asks. Every call runs in a tea.Cmd with netTimeout,
// and the command never touches the model — it copies what it needs first.

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/words"
)

// netTimeout bounds one network call made from the UI.
const netTimeout = online.Timeout

// maxNoteRunes bounds text from the server that lands on Model.note. The
// server is ours, but a message is drawn on one line inside the panel, and an
// over-long one would widen the frame past the terminal.
const maxNoteRunes = 120

// serverNote is server text made fit for Model.note: cut to maxNoteRunes,
// ending in "…" when it was cut. safeText still runs when it is drawn.
func serverNote(s string) string {
	r := []rune(s)
	if len(r) <= maxNoteRunes {
		return s
	}
	return string(r[:maxNoteRunes-1]) + "…"
}

type statusMsg struct {
	status online.Status
	err    error
}

type dailyPostedMsg struct{ err error }

type dailyCountMsg struct {
	day    string
	length int
	count  online.DailyCount
	err    error
}

// consent reports whether the player allows the network right now, and the
// server has not turned this client away.
func (m *Model) consent() bool { return !m.gone && m.settingsOf().Network }

// dailyOnline reports whether the daily count may be used: consent, and a
// server that has not switched the feature off.
func (m *Model) dailyOnline() bool {
	return m.consent() && (m.status == nil || m.status.Features.Daily)
}

// statusCmd asks the server for its status, at most once a run, and only
// with consent. It is safe to call from anywhere; it returns nil when there
// is nothing to do.
func (m *Model) statusCmd() tea.Cmd {
	if m.statusAsked || !m.consent() {
		return nil
	}
	m.statusAsked = true
	client := m.online
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		status, err := client.Status(ctx)
		return statusMsg{status: status, err: err}
	}
}

// markGone handles a *online.GoneError from any call: stop using the
// network for this run and say why, once. It returns true when err was a
// GoneError (and was handled).
func (m *Model) markGone(err error) bool {
	var gone *online.GoneError
	if !errors.As(err, &gone) {
		return false
	}
	m.gone = true
	m.note = serverNote(gone.Message)
	return true
}

// postDailyCmd sends one finished daily's result. The finishing guess of a
// daily is the one moment it is sent: once, and never again on review or
// resume. An unfinished mode asks for nothing, so the screen cannot be
// hinted at by the count either.
func (m *Model) postDailyCmd(g *game.Game) tea.Cmd {
	if g.Daily == "" || !g.Status.Done() || !m.dailyOnline() {
		return nil
	}
	day, length := g.Daily, g.Length
	solved := g.Status == game.Won
	guesses := g.Attempts()
	client := m.online
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		err := client.PostDaily(ctx, day, length, solved, guesses)
		return dailyPostedMsg{err: err}
	}
}

// dailyCountCmd asks how everyone did on one mode of the day.
func (m *Model) dailyCountCmd(day string, length int) tea.Cmd {
	if !m.dailyOnline() {
		return nil
	}
	client := m.online
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		count, err := client.Daily(ctx, day, length)
		return dailyCountMsg{day: day, length: length, count: count, err: err}
	}
}

// menuCountEvery is how long the menu's daily count stays fresh. The menu asks
// again only while it is showing and only once this has passed, so a player
// sitting on it costs three small reads every five minutes and no more.
const menuCountEvery = 5 * time.Minute

// menuCountMsg carries the day's total of finished daily boards, all modes
// summed. It counts boards, not people — the count is anonymous, so a player
// who finishes all three modes is three — which is why the menu says
// "dailies" rather than "players".
type menuCountMsg struct {
	day    string
	played int
	err    error
}

// menuCountCmd asks for the day's total when the menu is showing, the daily
// count is on, nothing is already asking, and the last answer is stale. It is
// armed from the one-second clock, so opening the menu needs no call of its
// own and every way of reaching the menu is covered.
func (m *Model) menuCountCmd() tea.Cmd {
	if m.screen != screenMenu || m.menuCountBusy || !m.dailyOnline() {
		return nil
	}
	if !m.menuCountAt.IsZero() && timeNow().Sub(m.menuCountAt) < menuCountEvery {
		return nil
	}
	m.menuCountBusy = true
	m.menuCountAt = timeNow()
	day, client := m.day.String(), m.online
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		total := 0
		for _, n := range words.Lengths {
			c, err := client.Daily(ctx, day, n)
			if err != nil {
				// A partial sum would be a wrong number, so any failure shows
				// nothing.
				return menuCountMsg{day: day, err: err}
			}
			total += c.Played
		}
		return menuCountMsg{day: day, played: total}
	}
}
