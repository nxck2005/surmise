package engine

import (
	"context"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/play"
)

// The ways to play beyond a random board, and the record. Each opener goes
// through open, so the board it replaces is left first, and a sprint run in
// progress ends.

func init() {
	register("daily.view", (*Engine).dailyView)
	register("daily.open", (*Engine).dailyOpen)
	register("result.view", (*Engine).resultView)
	register("challenge.new", (*Engine).challengeNew)
	register("challenge.open", (*Engine).challengeOpen)
	register("custom.check", (*Engine).customCheck)
	register("custom.start", (*Engine).customStart)
	register("sprint.options", (*Engine).sprintOptions)
	register("sprint.start", (*Engine).sprintStart)
	register("sprint.view", (*Engine).sprintView)
	register("sprint.type", (*Engine).sprintType)
	register("sprint.delete", (*Engine).sprintDelete)
	register("sprint.trim", (*Engine).sprintTrim)
	register("sprint.submit", (*Engine).sprintSubmit)
	register("sprint.next", (*Engine).sprintNext)
	register("sprint.end", (*Engine).sprintEnd)
	register("record.profile", (*Engine).recordProfile)
	register("record.calendar", (*Engine).recordCalendar)
	register("record.trends", (*Engine).recordTrends)
}

// DayRequest names a day, "YYYY-MM-DD"; empty is today.
type DayRequest struct {
	Day string `json:"day"`
}

func (e *Engine) day(s string) (daily.Day, error) {
	if s == "" {
		return e.app.Today(), nil
	}
	d, err := daily.ParseDay(s)
	if err != nil {
		return daily.Day{}, invalid("day %q", s)
	}
	return d, nil
}

func (e *Engine) dailyView(r DayRequest) (play.DailyView, error) {
	d, err := e.day(r.Day)
	if err != nil {
		return play.DailyView{}, err
	}
	return e.app.DailyView(d)
}

// DailyOpenRequest opens one length of a day's daily; an empty day is today.
type DailyOpenRequest struct {
	Day    string `json:"day"`
	Length int    `json:"length"`
}

func (e *Engine) dailyOpen(r DailyOpenRequest) (play.BoardView, error) {
	d, err := e.day(r.Day)
	if err != nil {
		return play.BoardView{}, err
	}
	if err := checkLength(r.Length); err != nil {
		return play.BoardView{}, err
	}
	return e.open(func() (*play.Board, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return e.app.OpenDaily(ctx, d, r.Length)
	})
}

func checkLength(n int) error {
	switch n {
	case 4, 5, 6:
		return nil
	}
	return invalid("length %d: want 4, 5 or 6", n)
}

// resultView is the debrief for the open board, refused while it is in play.
func (e *Engine) resultView(Empty) (play.ResultView, error) {
	if e.board == nil {
		return play.ResultView{}, &Error{Code: CodeNoBoard}
	}
	return e.app.Result(e.board)
}

// LengthRequest names a word length.
type LengthRequest struct {
	Length int `json:"length"`
}

// ChallengeCode is a shareable challenge.
type ChallengeCode struct {
	Code string `json:"code"`
}

func (e *Engine) challengeNew(r LengthRequest) (ChallengeCode, error) {
	if err := checkLength(r.Length); err != nil {
		return ChallengeCode{}, err
	}
	code, err := play.NewChallenge(r.Length)
	return ChallengeCode{Code: code}, err
}

func (e *Engine) challengeOpen(r ChallengeCode) (play.BoardView, error) {
	return e.open(func() (*play.Board, error) { return e.app.OpenChallenge(r.Code) })
}

// CustomRequest is a hand-set answer. The word never comes back in any
// answer before the board is over.
type CustomRequest struct {
	Word    string `json:"word"`
	Length  int    `json:"length"`
	AnyWord bool   `json:"anyWord"`
}

func (e *Engine) customCheck(r CustomRequest) (Empty, error) {
	if err := checkLength(r.Length); err != nil {
		return Empty{}, err
	}
	return Empty{}, play.CheckCustom(r.Word, r.Length, r.AnyWord)
}

func (e *Engine) customStart(r CustomRequest) (play.BoardView, error) {
	if err := checkLength(r.Length); err != nil {
		return play.BoardView{}, err
	}
	return e.open(func() (*play.Board, error) { return e.app.StartCustom(r.Word, r.Length, r.AnyWord) })
}

// SprintOptions is what a run's clock can be set to.
type SprintOptions struct {
	DurationsMS []int64 `json:"durationsMs"`
	DefaultMS   int64   `json:"defaultMs"`
}

func (e *Engine) sprintOptions(Empty) (SprintOptions, error) {
	out := SprintOptions{DefaultMS: play.DefaultSprintDuration.Milliseconds()}
	for _, d := range play.SprintDurations {
		out.DurationsMS = append(out.DurationsMS, d.Milliseconds())
	}
	return out, nil
}

// SprintRequest starts a run; a duration not on the list is the default.
type SprintRequest struct {
	Length     int   `json:"length"`
	DurationMS int64 `json:"durationMs"`
}

func (e *Engine) sprintStart(r SprintRequest) (play.SprintView, error) {
	if err := checkLength(r.Length); err != nil {
		return play.SprintView{}, err
	}
	var s *play.Sprint
	if _, err := e.open(func() (*play.Board, error) {
		var err error
		s, err = e.app.StartSprint(r.Length, time.Duration(r.DurationMS)*time.Millisecond)
		if err != nil {
			return nil, err
		}
		return s.Board(), nil
	}); err != nil {
		return play.SprintView{}, err
	}
	e.sprint = s
	return s.View(), nil
}

// withSprint runs a move on the run and answers with its view. The open
// board follows the run's, so leaving and exporting bank the right one.
func (e *Engine) withSprint(move func(*play.Sprint) error) (play.SprintView, error) {
	s := e.sprint
	if s == nil {
		return play.SprintView{}, &Error{Code: CodeNoBoard}
	}
	err := move(s)
	e.board = s.Board()
	if err != nil {
		return play.SprintView{}, err
	}
	return s.View(), nil
}

func (e *Engine) sprintView(Empty) (play.SprintView, error) {
	return e.withSprint(func(*play.Sprint) error { return nil })
}

func (e *Engine) sprintType(r TypeRequest) (play.SprintView, error) {
	return e.withSprint(func(s *play.Sprint) error {
		if c, ok := oneLetter(r.Letter); ok {
			return s.Type(c)
		}
		return nil
	})
}

func (e *Engine) sprintDelete(Empty) (play.SprintView, error) {
	return e.withSprint((*play.Sprint).Delete)
}

func (e *Engine) sprintTrim(r TrimRequest) (play.SprintView, error) {
	return e.withSprint(func(s *play.Sprint) error { return s.Trim(r.Index) })
}

func (e *Engine) sprintSubmit(Empty) (play.SprintView, error) {
	return e.withSprint((*play.Sprint).Submit)
}

func (e *Engine) sprintNext(Empty) (play.SprintView, error) {
	return e.withSprint((*play.Sprint).Next)
}

// sprintEnd stops the run and answers with its summary. The run stays
// readable through sprint.view until another board opens.
func (e *Engine) sprintEnd(Empty) (play.SprintView, error) {
	return e.withSprint((*play.Sprint).End)
}

func (e *Engine) recordProfile(Empty) (play.ProfileView, error) {
	return e.app.Profile()
}

// recordCalendar counts the history by day in the device's zone.
func (e *Engine) recordCalendar(Empty) (play.CalendarView, error) {
	return e.app.Calendar(e.zone)
}

func (e *Engine) recordTrends(r LengthRequest) (play.TrendsView, error) {
	if err := checkLength(r.Length); err != nil {
		return play.TrendsView{}, err
	}
	return e.app.Trends(r.Length)
}
