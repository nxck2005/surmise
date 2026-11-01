package engine

import (
	"fmt"

	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/play"
)

// The online calls. None of them waits on the network: each starts the work
// and returns the view as it stands; the answer lands later through the post
// queue, and the app hears of it as an event ("online" or "room") and reads
// the view again.

func init() {
	register("online.view", (*Engine).onlineView)
	register("online.status", (*Engine).onlineStatus)
	register("online.menuCount", (*Engine).onlineMenuCount)
	register("online.dailyCount", (*Engine).onlineDailyCount)
	register("rooms.list", (*Engine).roomsList)
	register("rooms.probe", (*Engine).roomsProbe)
	register("rooms.make", (*Engine).roomsMake)
	register("rooms.join", (*Engine).roomsJoin)
	register("rooms.view", (*Engine).roomsView)
	register("rooms.type", (*Engine).roomsType)
	register("rooms.delete", (*Engine).roomsDelete)
	register("rooms.submit", (*Engine).roomsSubmit)
	register("rooms.ready", (*Engine).roomsReady)
	register("rooms.start", (*Engine).roomsStart)
	register("rooms.rematch", (*Engine).roomsRematch)
	register("rooms.leave", (*Engine).roomsLeave)
	register("rooms.close", (*Engine).roomsClose)
	register("rooms.result", (*Engine).roomsResult)
	register("rooms.refresh", (*Engine).roomsRefresh)
}

// postQueue is how far the network's answers may run ahead of the worker
// that applies them. A full queue holds the sender back, which is a socket
// reader or a finished request: never a call into the engine.
const postQueue = 256

// startHost connects the App to the network: every answer is applied by one
// worker, in the order it was posted, under the engine's lock, and then the
// app is told which view changed.
func (e *Engine) startHost(client online.Client) {
	e.posts = make(chan func(), postQueue)
	go func() {
		for f := range e.posts {
			e.runPosted(f)
		}
	}()
	e.app.SetHost(play.Host{
		Client:     client,
		ClientName: e.client,
		Post:       func(f func()) { e.posts <- f },
		Notify: func(kind string) {
			if e.emit != nil {
				e.emit(kind, []byte("{}"))
			}
		},
	})
}

// runPosted applies one answer. A panic in it is reported as an event
// rather than taking the app down, like a panic in Call.
func (e *Engine) runPosted(f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() {
		if r := recover(); r != nil && e.emit != nil {
			e.emit("panic", reply(nil, &Error{Code: CodeInternal, Detail: fmt.Sprint(r)}))
		}
	}()
	f()
}

func (e *Engine) onlineView(Empty) (play.OnlineView, error) {
	return e.app.OnlineView(), nil
}

// onlineStatus asks for the server's status, once a run.
func (e *Engine) onlineStatus(Empty) (play.OnlineView, error) {
	e.app.AskStatus()
	return e.app.OnlineView(), nil
}

// onlineMenuCount asks for today's finished dailies, when the last answer
// is stale. The home screen calls it while it shows.
func (e *Engine) onlineMenuCount(Empty) (play.OnlineView, error) {
	e.app.AskMenuCount()
	return e.app.OnlineView(), nil
}

// CountRequest names one daily board.
type CountRequest struct {
	Day    string `json:"day"`
	Length int    `json:"length"`
}

func (e *Engine) onlineDailyCount(r CountRequest) (play.OnlineView, error) {
	d, err := e.day(r.Day)
	if err != nil {
		return play.OnlineView{}, err
	}
	e.app.AskDailyCount(d.String(), r.Length)
	return e.app.OnlineView(), nil
}

// RoomsList is the rooms this install is in, newest first.
type RoomsList struct {
	Items []play.RoomEntryView `json:"items"`
}

func (e *Engine) roomsList(Empty) (RoomsList, error) {
	return RoomsList{Items: e.app.RoomsList()}, nil
}

// roomsProbe is the startup check: a round of ours still running.
func (e *Engine) roomsProbe(Empty) (play.RoomView, error) {
	e.app.ProbeRooms()
	return e.app.RoomView(), nil
}

// MakeRoomRequest makes a room. Mode is "live" or "async".
type MakeRoomRequest struct {
	Mode      string `json:"mode"`
	Length    int    `json:"length"`
	AutoStart bool   `json:"autoStart"`
}

func (e *Engine) roomsMake(r MakeRoomRequest) (play.RoomView, error) {
	if err := e.app.MakeRoom(r.Mode, r.Length, r.AutoStart); err != nil {
		return play.RoomView{}, err
	}
	return e.app.RoomView(), nil
}

// RoomCode names a room.
type RoomCode struct {
	Code string `json:"code"`
}

func (e *Engine) roomsJoin(r RoomCode) (play.RoomView, error) {
	if err := e.app.JoinRoom(r.Code); err != nil {
		return play.RoomView{}, err
	}
	return e.app.RoomView(), nil
}

// withRoom runs a move on the session and answers with its view.
func (e *Engine) withRoom(move func(*play.App) error) (play.RoomView, error) {
	if err := move(e.app); err != nil {
		return play.RoomView{}, err
	}
	return e.app.RoomView(), nil
}

func (e *Engine) roomsView(Empty) (play.RoomView, error) {
	return e.app.RoomView(), nil
}

func (e *Engine) roomsType(r TypeRequest) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error {
		if c, ok := oneLetter(r.Letter); ok {
			a.RoomType(c)
		}
		return nil
	})
}

func (e *Engine) roomsDelete(Empty) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.RoomDelete(); return nil })
}

func (e *Engine) roomsSubmit(Empty) (play.RoomView, error) {
	return e.withRoom((*play.App).RoomSubmit)
}

// ReadyRequest sets our seat's ready flag.
type ReadyRequest struct {
	Ready bool `json:"ready"`
}

func (e *Engine) roomsReady(r ReadyRequest) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.RoomReady(r.Ready); return nil })
}

func (e *Engine) roomsStart(Empty) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.RoomStart(); return nil })
}

func (e *Engine) roomsRematch(Empty) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.RoomRematch(); return nil })
}

// roomsLeave leaves a room for good, open or not.
func (e *Engine) roomsLeave(r RoomCode) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.LeaveRoom(r.Code); return nil })
}

// roomsClose steps away from the open room; it keeps our seat.
func (e *Engine) roomsClose(Empty) (play.RoomView, error) {
	return e.withRoom(func(a *play.App) error { a.CloseRoom(); return nil })
}

// IDRequest names a saved puzzle.
type IDRequest struct {
	ID string `json:"id"`
}

func (e *Engine) roomsResult(r IDRequest) (play.RoomResultView, error) {
	return e.app.RoomResult(r.ID)
}

// roomsRefresh asks for an any-time round's newer ranking; the answer lands
// in the saved round, and the room note says how it went.
func (e *Engine) roomsRefresh(r IDRequest) (play.RoomResultView, error) {
	if err := e.app.RefreshRoomResult(r.ID); err != nil {
		return play.RoomResultView{}, err
	}
	return e.app.RoomResult(r.ID)
}
