package play

import (
	"sort"
	"time"

	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
)

// The record is the profile, the calendar and the trends: stats' figures
// over the shared history, flattened for the wire. Durations go out as
// milliseconds and maps keyed by length as lists in length order.

// ModeView is one length's slice of the profile.
type ModeView struct {
	Length      int     `json:"length"`
	Played      int     `json:"played"`
	Won         int     `json:"won"`
	WinRate     float64 `json:"winRate"`
	AvgAttempts float64 `json:"avgAttempts"`
	AvgTimeMS   int64   `json:"avgTimeMs"`
	// Daily is nil for a length whose daily was never played.
	Daily *DailyStatsView `json:"daily"`
}

// DailyStatsView is one length's dailies. Its streaks are calendar days.
type DailyStatsView struct {
	Played        int     `json:"played"`
	Won           int     `json:"won"`
	WinRate       float64 `json:"winRate"`
	CurrentStreak int     `json:"currentStreak"`
	MaxStreak     int     `json:"maxStreak"`
}

// ProfileView is the profile screen's figures.
type ProfileView struct {
	DisplayName string  `json:"displayName"`
	Played      int     `json:"played"`
	Won         int     `json:"won"`
	Lost        int     `json:"lost"`
	InPlay      int     `json:"inPlay"`
	WinRate     float64 `json:"winRate"`
	AvgAttempts float64 `json:"avgAttempts"`
	AvgTimeMS   int64   `json:"avgTimeMs"`

	CurrentStreak int `json:"currentStreak"`
	MaxStreak     int `json:"maxStreak"`

	// Distribution counts wins by guesses: index i is wins in i+1 guesses,
	// up to the most guesses any win took.
	Distribution []int      `json:"distribution"`
	Modes        []ModeView `json:"modes"`
	// PlaytimeMS is the lifetime counter, never below what the records hold.
	PlaytimeMS int64 `json:"playtimeMs"`
}

// Profile is the profile as of today. Like the terminal's, it floors the
// lifetime playtime counter at what the saved puzzles add up to and writes
// the floor back the first time it wins: an install older than the counter
// gets its history counted once. The counter never falls.
func (a *App) Profile() (ProfileView, error) {
	games, err := a.History()
	if err != nil {
		return ProfileView{}, err
	}
	set := a.Settings()
	saved := time.Duration(set.PlaytimeMS) * time.Millisecond
	total := stats.Playtime(saved, games)
	if total > saved {
		_ = a.UpdateSettings(func(s *store.Settings) {
			s.PlaytimeMS = max(s.PlaytimeMS, total.Milliseconds())
		})
	}

	s := stats.ComputeAt(games, a.Today())
	v := ProfileView{
		DisplayName:   set.DisplayName,
		Played:        s.Played,
		Won:           s.Won,
		Lost:          s.Lost,
		InPlay:        s.InPlay,
		WinRate:       s.WinRate,
		AvgAttempts:   s.AvgAttempts,
		AvgTimeMS:     s.AvgTime.Milliseconds(),
		CurrentStreak: s.CurrentStreak,
		MaxStreak:     s.MaxStreak,
		Distribution:  []int{},
		Modes:         []ModeView{},
		PlaytimeMS:    total.Milliseconds(),
	}
	for n, wins := range s.Distribution {
		for len(v.Distribution) < n {
			v.Distribution = append(v.Distribution, 0)
		}
		v.Distribution[n-1] = wins
	}
	for n, m := range s.ByLength {
		mv := ModeView{Length: n, Played: m.Played, Won: m.Won, WinRate: m.WinRate,
			AvgAttempts: m.AvgAttempts, AvgTimeMS: m.AvgTime.Milliseconds()}
		if d, ok := s.Daily[n]; ok {
			mv.Daily = &DailyStatsView{Played: d.Played, Won: d.Won, WinRate: d.WinRate,
				CurrentStreak: d.CurrentStreak, MaxStreak: d.MaxStreak}
		}
		v.Modes = append(v.Modes, mv)
	}
	sort.Slice(v.Modes, func(i, j int) bool { return v.Modes[i].Length < v.Modes[j].Length })
	return v, nil
}

// DayView is one calendar day with something on it.
type DayView struct {
	Day    string `json:"day"`
	Games  int    `json:"games"`
	Won    int    `json:"won"`
	Lost   int    `json:"lost"`
	InPlay int    `json:"inPlay"`
}

// CalendarView is every day with play, oldest first. Days with nothing are
// left out; the front end lays the year out itself.
type CalendarView struct {
	Today string    `json:"today"`
	Days  []DayView `json:"days"`
}

// Calendar counts the history by the day each puzzle was played in zone
// (stats.PlayDay): a daily on its own date, anything else on the local day
// it was started.
func (a *App) Calendar(zone *time.Location) (CalendarView, error) {
	games, err := a.History()
	if err != nil {
		return CalendarView{}, err
	}
	v := CalendarView{Today: a.Today().String(), Days: []DayView{}}
	for d, t := range stats.Activity(games, zone) {
		v.Days = append(v.Days, DayView{Day: d.String(), Games: t.Games, Won: t.Won, Lost: t.Lost, InPlay: t.InPlay})
	}
	// ISO dates sort as text.
	sort.Slice(v.Days, func(i, j int) bool { return v.Days[i].Day < v.Days[j].Day })
	return v, nil
}

// FormView is a run of finished games; the averages are over wins.
type FormView struct {
	Games       int     `json:"games"`
	Won         int     `json:"won"`
	WinRate     float64 `json:"winRate"`
	AvgAttempts float64 `json:"avgAttempts"`
	AvgTimeMS   int64   `json:"avgTimeMs"`
}

// BestView is the puzzle holding a personal best.
type BestView struct {
	ID        string `json:"id"`
	Attempts  int    `json:"attempts"`
	ElapsedMS int64  `json:"elapsedMs"`
}

// OpenerView is one first guess and how its games went.
type OpenerView struct {
	Word        string  `json:"word"`
	Games       int     `json:"games"`
	Won         int     `json:"won"`
	WinRate     float64 `json:"winRate"`
	AvgAttempts float64 `json:"avgAttempts"`
}

// TrendsView is one length's recent form against all time.
type TrendsView struct {
	Length int `json:"length"`
	// RecentWindow is how many games Recent covers at most.
	RecentWindow int          `json:"recentWindow"`
	Recent       FormView     `json:"recent"`
	AllTime      FormView     `json:"allTime"`
	Fastest      *BestView    `json:"fastest"`
	Fewest       *BestView    `json:"fewest"`
	Openers      []OpenerView `json:"openers"`
}

// Trends is stats.Trends for one length.
func (a *App) Trends(length int) (TrendsView, error) {
	games, err := a.History()
	if err != nil {
		return TrendsView{}, err
	}
	t := stats.Trends(games, length)
	form := func(f stats.Form) FormView {
		return FormView{Games: f.Games, Won: f.Won, WinRate: f.WinRate, AvgAttempts: f.AvgAttempts, AvgTimeMS: f.AvgTime.Milliseconds()}
	}
	best := func(b *stats.Best) *BestView {
		if b == nil {
			return nil
		}
		return &BestView{ID: b.ID, Attempts: b.Attempts, ElapsedMS: b.Elapsed.Milliseconds()}
	}
	v := TrendsView{Length: length, RecentWindow: stats.RecentWindow, Recent: form(t.Recent), AllTime: form(t.AllTime),
		Fastest: best(t.Fastest), Fewest: best(t.Fewest), Openers: []OpenerView{}}
	for _, o := range t.Openers {
		v.Openers = append(v.Openers, OpenerView{Word: o.Word, Games: o.Games, Won: o.Won, WinRate: o.WinRate, AvgAttempts: o.AvgAttempts})
	}
	return v, nil
}
