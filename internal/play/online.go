package play

import (
	"context"
	"errors"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// The online parts: consent, the one status request, the daily result post,
// the daily counts, and the rooms (rooms.go). They are the TUI's rules
// (internal/ui/online.go, rooms.go, roomsround.go, roomsretry.go).
//
// Nothing here blocks on the network. A call that needs the server starts a
// goroutine that copies what it needs first and touches no state; its answer
// comes back through Host.Post, which runs it in turn with every other call
// into the App, and then Host.Notify tells the front end which view to read
// again. A front end without a host has no network: every call is a no-op.

// Host is what a front end lends the App for work that waits on the network.
type Host struct {
	// Client is the server. online.Off, or nil, keeps everything offline.
	Client online.Client
	// ClientName is the X-Surmise-Client text (online.ClientName). A room's
	// hello carries it, since a WebSocket has no headers.
	ClientName string
	// Post runs f later, one at a time with every other call into the App.
	// It must not run f on the calling goroutine.
	Post func(f func())
	// Notify says a view changed: NotifyOnline or NotifyRoom.
	Notify func(kind string)
}

// What Host.Notify names.
const (
	NotifyOnline = "online"
	NotifyRoom   = "room"
)

// menuCountEvery is how long the menu's daily count stays fresh.
const menuCountEvery = 5 * time.Minute

type netState struct {
	host Host

	// gone is the server turning this client away: nothing online runs
	// again this run. goneMessage is what it said, cut to fit.
	gone        bool
	goneMessage string

	status      *online.Status
	statusAsked bool

	// counts holds the daily counts this run has, by day and length.
	counts     map[countKey]online.DailyCount
	countsBusy map[countKey]bool

	// menuCount is the day's total of finished dailies; -1 is none yet.
	menuCount    int
	menuCountDay string
	menuCountAt  time.Time
	menuBusy     bool

	room     *roomSession
	roomGen  int
	roomNote Note
	// making is a room being made; a second ask meanwhile does nothing.
	making bool
	// probed is the startup check having run this run.
	probed bool
	// refreshing holds the saved rounds whose ranking is being asked for.
	refreshing map[string]bool
}

type countKey struct {
	day    string
	length int
}

func newNetState() netState {
	return netState{
		host:       Host{Client: online.Off},
		counts:     map[countKey]online.DailyCount{},
		countsBusy: map[countKey]bool{},
		refreshing: map[string]bool{},
		menuCount:  -1,
	}
}

// SetHost connects the App to the network. Call it once, before anything
// online; until then the App is offline.
func (a *App) SetHost(h Host) {
	if h.Client == nil {
		h.Client = online.Off
	}
	a.net.host = h
}

// spawn runs work off the App's goroutine and settle back on it, then says
// kind changed. work must use only what it was handed.
func spawn[R any](a *App, kind string, work func() R, settle func(R)) {
	h := a.net.host
	if h.Post == nil {
		return
	}
	go func() {
		r := work()
		h.Post(func() {
			settle(r)
			if h.Notify != nil {
				h.Notify(kind)
			}
		})
	}()
}

// consent reports whether the player allows the network right now, the
// server has not turned this client away, and there is a network to use.
func (a *App) consent() bool {
	return !a.net.gone && a.net.host.Post != nil && a.net.host.Client != online.Off && a.Settings().Network
}

// dailyOnline is consent and a server that has not switched the daily count
// off. A status not heard yet counts as on.
func (a *App) dailyOnline() bool {
	return a.consent() && (a.net.status == nil || a.net.status.Features.Daily)
}

// maxNoteRunes bounds server text shown to the player: it is drawn on one
// line.
const maxNoteRunes = 120

func serverNote(s string) string {
	r := []rune(s)
	if len(r) <= maxNoteRunes {
		return s
	}
	return string(r[:maxNoteRunes-1]) + "…"
}

// markGone handles a GoneError from any call: stop using the network for
// this run, keep what the server said, and close any open room (the room
// learns from its socket closing). It reports whether err was one.
func (a *App) markGone(err error) bool {
	var gone *online.GoneError
	if !errors.As(err, &gone) {
		return false
	}
	a.net.gone = true
	a.net.goneMessage = serverNote(gone.Message)
	a.closeRoom()
	return true
}

// AskStatus asks the server for its status, at most once a run, and only
// with consent. Call it at start and whenever consent is given; extra calls
// do nothing.
func (a *App) AskStatus() {
	if a.net.statusAsked || !a.consent() {
		return
	}
	a.net.statusAsked = true
	client := a.net.host.Client
	type answer struct {
		status online.Status
		err    error
	}
	spawn(a, NotifyOnline, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		s, err := client.Status(ctx)
		return answer{s, err}
	}, func(r answer) {
		if r.err != nil {
			// A status that could not be read leaves every feature on: the
			// calls themselves will fail if the server is down.
			a.markGone(r.err)
			return
		}
		a.net.status = &r.status
	})
}

// postDaily sends one finished daily's result. The finishing guess is the
// one moment it is sent: once, and never on review or resume.
func (a *App) postDaily(g *game.Game) {
	if g.Daily == "" || !g.Status.Done() || !a.dailyOnline() {
		return
	}
	day, length, solved, guesses := g.Daily, g.Length, g.Status == game.Won, g.Attempts()
	client := a.net.host.Client
	spawn(a, NotifyOnline, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		return client.PostDaily(ctx, day, length, solved, guesses)
	}, func(err error) {
		a.markGone(err)
		// The count the player is about to look at includes this result.
		delete(a.net.counts, countKey{day, length})
		a.AskDailyCount(day, length)
	})
}

// AskDailyCount asks how everyone did on one day's board of one length. Only
// a board finished here may ask: an open board's count would hint at it.
// The answer lands in OnlineView.Counts.
func (a *App) AskDailyCount(day string, length int) {
	if !a.dailyOnline() {
		return
	}
	d, err := daily.ParseDay(day)
	if err != nil {
		return
	}
	if g, err := a.store.Load(daily.ID(d, length)); err != nil || !g.Status.Done() {
		return
	}
	k := countKey{day, length}
	if a.net.countsBusy[k] {
		return
	}
	a.net.countsBusy[k] = true
	client := a.net.host.Client
	type answer struct {
		count online.DailyCount
		err   error
	}
	spawn(a, NotifyOnline, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		c, err := client.Daily(ctx, day, length)
		return answer{c, err}
	}, func(r answer) {
		delete(a.net.countsBusy, k)
		if r.err != nil {
			a.markGone(r.err)
			return
		}
		if len(r.count.Distribution) != length+1 {
			return
		}
		a.net.counts[k] = r.count
	})
}

// AskMenuCount asks for today's total of finished daily boards, all lengths
// summed, when the last answer is older than menuCountEvery. The home screen
// calls it while it shows; it asks at most once in the window.
func (a *App) AskMenuCount() {
	if a.net.menuBusy || !a.dailyOnline() {
		return
	}
	now := a.clock.Now()
	day := a.Today().String()
	if day == a.net.menuCountDay && !a.net.menuCountAt.IsZero() && now.Sub(a.net.menuCountAt) < menuCountEvery {
		return
	}
	a.net.menuBusy = true
	a.net.menuCountAt = now
	client := a.net.host.Client
	type answer struct {
		total int
		err   error
	}
	spawn(a, NotifyOnline, func() answer {
		ctx, cancel := context.WithTimeout(context.Background(), online.Timeout)
		defer cancel()
		total := 0
		for _, n := range words.Lengths {
			c, err := client.Daily(ctx, day, n)
			if err != nil {
				// A partial sum would be a wrong number.
				return answer{err: err}
			}
			total += c.Played
		}
		return answer{total: total}
	}, func(r answer) {
		a.net.menuBusy = false
		if r.err != nil {
			a.markGone(r.err)
			return
		}
		a.net.menuCount, a.net.menuCountDay = r.total, day
	})
}

// CountView is how everyone did on one daily board.
type CountView struct {
	Day    string `json:"day"`
	Length int    `json:"length"`
	Played int    `json:"played"`
	Solved int    `json:"solved"`
	// Distribution is solves by guesses: index i is solves in i+1.
	Distribution []int `json:"distribution"`
}

// OnlineView is the online state as the front end shows it.
type OnlineView struct {
	// Consent is the player's choice (settings), whatever the server says.
	Consent bool `json:"consent"`
	// Gone is the server having turned this version away for the run;
	// GoneMessage is its words, shown once.
	Gone        bool   `json:"gone"`
	GoneMessage string `json:"goneMessage"`
	// Daily and Rooms say the feature may be used now.
	Daily bool `json:"daily"`
	Rooms bool `json:"rooms"`
	// Message is the server's status message, if it had one.
	Message string `json:"message"`
	// MenuCount is today's finished dailies across lengths; -1 is unknown.
	MenuCount int         `json:"menuCount"`
	Counts    []CountView `json:"counts"`
}

// OnlineView is the online state now.
func (a *App) OnlineView() OnlineView {
	v := OnlineView{
		Consent:     a.Settings().Network,
		Gone:        a.net.gone,
		GoneMessage: a.net.goneMessage,
		Daily:       a.dailyOnline(),
		Rooms:       a.roomsOnline(),
		MenuCount:   -1,
		Counts:      []CountView{},
	}
	if a.net.status != nil {
		v.Message = serverNote(a.net.status.Message)
	}
	today := a.Today().String()
	if v.Daily && a.net.menuCountDay == today {
		v.MenuCount = a.net.menuCount
	}
	if v.Daily {
		for _, n := range words.Lengths {
			if c, ok := a.net.counts[countKey{today, n}]; ok {
				v.Counts = append(v.Counts, CountView{Day: today, Length: n, Played: c.Played, Solved: c.Solved, Distribution: c.Distribution})
			}
		}
	}
	return v
}

// SetNetwork is the consent row: the one call that may change Network. Giving
// consent asks for the status; taking it away closes any open room without
// leaving it.
func (a *App) SetNetwork(on bool) error {
	if err := a.UpdateSettings(func(s *store.Settings) { s.Network = on }); err != nil {
		return err
	}
	if on {
		a.AskStatus()
		a.ProbeRooms()
	} else {
		a.closeRoom()
	}
	return nil
}
