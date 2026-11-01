// Package engine is the app's whole Go side behind one call: a method name and
// a JSON request in, a JSON envelope out. The bound package (mobile/core) is a
// thin shell over it, because gomobile can carry only basic types and []byte;
// everything with a real type lives here, where the generator can reflect
// over it (mobile/cmd/gen).
//
// Go decides, Kotlin draws: every board call answers with the whole
// play.BoardView, and refusals are codes, never sentences, so all player copy
// lives on the Android side.
package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/internal/play"
	"github.com/nxck2005/surmise/internal/store"
)

// Engine is one open install. Calls are serialised by a mutex; the app calls
// it from one background dispatcher anyway, and the lock is what makes a stray
// second caller safe rather than racy.
type Engine struct {
	mu sync.Mutex

	dir     string
	zone    *time.Location
	version string
	abi     string
	client  string

	st    *store.JSON
	app   *play.App
	clock play.Clock

	// board is the one open board, or nil. Opening another leaves this one
	// first, so its time is banked and its last guess saved.
	board *play.Board
	// sprint is the timed run in progress, or nil. Its board is board.
	sprint *play.Sprint

	// emit sends an event to the app. Nil sends nothing.
	emit func(kind string, payload []byte)

	// posts carries the network's answers to the one worker that applies
	// them (startHost); nil with no network.
	posts chan func()
}

// Config is what Open needs from the platform.
type Config struct {
	DataDir string // the install's data dir; the native store.JSON layout
	Zone    string // an IANA zone name from the device
	Version string // the app's version name
	ABI     string // the device's primary ABI, for the client name
	Client  string // the client name sent to the server
	Emit    func(kind string, payload []byte)

	// Clock is the time source. Nil is the wall clock; the tests stop it.
	Clock play.Clock

	// Online is the server. Nil keeps the engine offline whatever the
	// player's consent says.
	Online online.Client
}

// Open opens an install. The zone is loaded here, from the embedded zone
// database the bound package imports: Go's time.Local is UTC on Android.
func Open(c Config) (*Engine, error) {
	loc, err := time.LoadLocation(c.Zone)
	if err != nil {
		return nil, fmt.Errorf("zone %q: %w", c.Zone, err)
	}
	st, err := store.NewJSON(c.DataDir)
	if err != nil {
		return nil, err
	}
	clock := c.Clock
	if clock == nil {
		clock = play.SystemClock{}
	}
	e := &Engine{
		dir: c.DataDir, zone: loc, version: c.Version, abi: c.ABI, client: c.Client,
		st: st, app: play.New(st, clock), clock: clock, emit: c.Emit,
	}
	if c.Online != nil {
		e.startHost(c.Online)
	}
	return e, nil
}

// SetZone changes the device zone, on ACTION_TIMEZONE_CHANGED. An unknown name
// keeps the zone already set.
func (e *Engine) SetZone(name string) error {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.zone = loc
	return nil
}

// Close leaves the open board, banking its time and saving it, and steps
// away from any open room without leaving it.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.app.CloseRoom()
	if e.board == nil {
		return nil
	}
	err := e.board.Leave()
	e.board = nil
	return err
}

// Error codes the engine adds to play's. They are the contract with the app,
// so a code once shipped keeps its spelling.
const (
	CodeBadRequest    = "bad_request"
	CodeUnknownMethod = "unknown_method"
	CodeNoBoard       = "no_board"
	CodeNotFound      = "not_found"
	CodeInvalid       = "invalid"
	CodeInternal      = "internal"
)

// Error is a refusal the engine raises itself.
type Error struct {
	Code   string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func invalid(format string, args ...any) error {
	return &Error{Code: CodeInvalid, Detail: fmt.Sprintf(format, args...)}
}

// codeOf maps an error to its wire code.
func codeOf(err error) string {
	var ee *Error
	if errors.As(err, &ee) {
		return ee.Code
	}
	if c := play.CodeOf(err); c != "" {
		return c
	}
	if errors.Is(err, store.ErrNotFound) {
		return CodeNotFound
	}
	return CodeInternal
}

// errBody is the envelope's err member.
type errBody struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// reply writes the envelope: {"ok": v} or {"err": {"code", "detail"}}.
func reply(v any, err error) []byte {
	var out any
	if err != nil {
		out = struct {
			Err errBody `json:"err"`
		}{errBody{Code: codeOf(err), Detail: err.Error()}}
	} else {
		out = struct {
			OK any `json:"ok"`
		}{v}
	}
	b, merr := json.Marshal(out)
	if merr != nil {
		b, _ = json.Marshal(struct {
			Err errBody `json:"err"`
		}{errBody{Code: CodeInternal, Detail: merr.Error()}})
	}
	return b
}

// Call runs one method. It never panics into the platform: a handler that
// panics answers with an internal error instead of taking the app down.
func (e *Engine) Call(method string, req []byte) (out []byte) {
	m, ok := methods[method]
	if !ok {
		return reply(nil, &Error{Code: CodeUnknownMethod, Detail: method})
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			out = reply(nil, &Error{Code: CodeInternal, Detail: fmt.Sprint(r)})
		}
	}()
	return reply(m.call(e, req))
}
