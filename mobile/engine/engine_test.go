package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/online"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata")

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

const (
	fixedID = "3f2a7c1e-8b4d-4c6a-9e2f-1d5b8a7c3e90"
	otherID = "9b1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e"
)

func newEngine(t *testing.T) (*Engine, *fakeClock) {
	t.Helper()
	c := &fakeClock{t: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}
	e, err := Open(Config{
		DataDir: t.TempDir(), Zone: "Asia/Kolkata", Version: "0.12.0", ABI: "arm64-v8a",
		Client: "test-client", Clock: c,
	})
	if err != nil {
		t.Fatal(err)
	}
	return e, c
}

// openFixed opens an unsaved board whose answer is crane, with the letters
// already typed.
func openFixed(t *testing.T, e *Engine, typed string) {
	t.Helper()
	g, err := game.NewFrom(fixedID, "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	e.board = e.app.Open(g, false)
	for _, c := range typed {
		e.board.Type(c)
	}
}

// saveGame writes a puzzle with the given guesses.
func saveGame(t *testing.T, e *Engine, id, answer string, guesses ...string) {
	t.Helper()
	g, err := game.NewFrom(id, answer, len(answer))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range guesses {
		if err := g.Guess(w); err != nil {
			t.Fatal(err)
		}
	}
	// The wall clock would move the puzzle's calendar day under the goldens.
	g.StartedAt = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if err := e.st.Save(g); err != nil {
		t.Fatal(err)
	}
}

// call runs a method and splits the envelope.
func call(t *testing.T, e *Engine, method, req string) (ok json.RawMessage, code string) {
	t.Helper()
	var env struct {
		OK  json.RawMessage `json:"ok"`
		Err *errBody        `json:"err"`
	}
	raw := e.Call(method, []byte(req))
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("%s: envelope %s: %v", method, raw, err)
	}
	if env.Err != nil {
		return nil, env.Err.Code
	}
	return env.OK, ""
}

// goldenCase is one method's request against a known state. redact names the
// response keys whose values are random or wall-clock (a fresh puzzle's id,
// the store's own timestamps) and are replaced before comparing.
type goldenCase struct {
	method string
	setup  func(*testing.T, *Engine, *fakeClock)
	req    string
	redact []string
	// err says the golden is a refusal: its envelope is the err member.
	err bool
}

var goldenCases = []goldenCase{
	{method: "app.info", req: `{}`, redact: []string{"dataDir"}},
	{method: "board.new", req: `{"length":5}`, redact: []string{"id", "code"}},
	{method: "board.resume", req: `{"id":"` + fixedID + `"}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { saveGame(t, e, fixedID, "crane", "slate") }},
	{method: "board.view", req: `{}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { openFixed(t, e, "cr") }},
	{method: "board.type", req: `{"letter":"a"}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { openFixed(t, e, "cr") }},
	{method: "board.delete", req: `{}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { openFixed(t, e, "cra") }},
	{method: "board.trim", req: `{"index":1}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { openFixed(t, e, "cra") }},
	{method: "board.submit", req: `{}`,
		setup: func(t *testing.T, e *Engine, c *fakeClock) {
			openFixed(t, e, "c")
			c.advance(9 * time.Second)
			typeRest(e, "rane")
		}},
	{method: "board.leave", req: `{}`,
		setup: func(t *testing.T, e *Engine, c *fakeClock) { openFixed(t, e, "sl"); c.advance(4 * time.Second) }},
	{method: "board.restart", req: `{}`, redact: []string{"id", "code"},
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { openFixed(t, e, "") }},
	{method: "history.list", req: `{}`, redact: []string{"updatedAt"},
		setup: func(t *testing.T, e *Engine, _ *fakeClock) {
			saveGame(t, e, fixedID, "crane", "slate", "crane")
			time.Sleep(2 * time.Millisecond) // UpdatedAt is the store's wall clock
			saveGame(t, e, otherID, "about", "slate")
		}},
	{method: "history.delete", req: `{"id":"` + fixedID + `"}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { saveGame(t, e, fixedID, "crane", "crane") }},
	{method: "settings.get", req: `{}`},
	{method: "settings.update", req: `{"displayName":"nick","network":true,"length":6}`},
	{method: "themes.list", req: `{}`},
	{method: "themes.tokens", req: `{"name":"night","tiles":""}`},

	{method: "daily.view", req: `{"day":""}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { call(t, e, "daily.open", `{"length":4}`) }},
	{method: "daily.open", req: `{"day":"","length":5}`},
	{method: "result.view", req: `{}`,
		setup: func(t *testing.T, e *Engine, c *fakeClock) {
			openFixed(t, e, "c")
			c.advance(42 * time.Second)
			typeRest(e, "rane")
			call(t, e, "board.submit", `{}`)
		}},
	{method: "challenge.new", req: `{"length":5}`, redact: []string{"code"}},
	{method: "challenge.open", req: `{"code":"` + challengeCode + `"}`},
	{method: "custom.check", req: `{"word":"qzxvw","length":5,"anyWord":false}`, err: true},
	{method: "custom.start", req: `{"word":"crane","length":5,"anyWord":false}`, redact: []string{"id", "code"}},
	{method: "sprint.options", req: `{}`},
	{method: "sprint.start", req: `{"length":5,"durationMs":30000}`, redact: sprintRedact},
	{method: "sprint.view", req: `{}`, redact: sprintRedact, setup: startSprint(10 * time.Second)},
	{method: "sprint.type", req: `{"letter":"a"}`, redact: sprintRedact, setup: startSprint(0)},
	{method: "sprint.delete", req: `{}`, redact: sprintRedact, setup: startSprint(0)},
	{method: "sprint.trim", req: `{"index":0}`, redact: sprintRedact, setup: startSprint(0)},
	{method: "sprint.submit", req: `{}`, redact: sprintRedact, err: true, setup: startSprint(0)},
	{method: "sprint.next", req: `{}`, redact: sprintRedact, setup: startSprint(0)},
	{method: "sprint.end", req: `{}`, redact: sprintRedact, setup: startSprint(5 * time.Second)},
	{method: "record.profile", req: `{}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { saveGame(t, e, fixedID, "crane", "slate", "crane") }},
	{method: "record.calendar", req: `{}`, redact: []string{"day"},
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { saveGame(t, e, fixedID, "crane", "slate", "crane") }},
	{method: "record.trends", req: `{"length":5}`,
		setup: func(t *testing.T, e *Engine, _ *fakeClock) { saveGame(t, e, fixedID, "crane", "slate", "crane") }},

	{method: "online.view", req: `{}`},
	{method: "online.status", req: `{}`},
	{method: "online.menuCount", req: `{}`},
	{method: "online.dailyCount", req: `{"day":"","length":5}`},
	{method: "rooms.list", req: `{}`},
	{method: "rooms.probe", req: `{}`},
	{method: "rooms.make", req: `{"mode":"live","length":5,"autoStart":false}`, err: true},
	{method: "rooms.join", req: `{"code":"ABC234"}`, err: true},
	{method: "rooms.view", req: `{}`},
	{method: "rooms.type", req: `{"letter":"a"}`},
	{method: "rooms.delete", req: `{}`},
	{method: "rooms.submit", req: `{}`, err: true},
	{method: "rooms.ready", req: `{"ready":true}`},
	{method: "rooms.start", req: `{}`},
	{method: "rooms.rematch", req: `{}`},
	{method: "rooms.leave", req: `{"code":"ABC234"}`},
	{method: "rooms.close", req: `{}`},
	{method: "rooms.result", req: `{"id":"` + fixedID + `"}`, err: true},
	{method: "rooms.refresh", req: `{"id":"` + fixedID + `"}`, err: true},
}

// challengeCode is one of internal/challenge's frozen vectors (answer faint).
const challengeCode = "4500-820C-20A1-G73J"

// A sprint's boards are random: their ids and marks are not pinned.
var sprintRedact = []string{"id", "code", "rows", "keys"}

// startSprint starts a 30-second run and lets the clock run d.
func startSprint(d time.Duration) func(*testing.T, *Engine, *fakeClock) {
	return func(t *testing.T, e *Engine, c *fakeClock) {
		if _, code := call(t, e, "sprint.start", `{"length":5,"durationMs":30000}`); code != "" {
			t.Fatal(code)
		}
		c.advance(d)
	}
}

func typeRest(e *Engine, s string) {
	for _, c := range s {
		e.board.Type(c)
	}
}

func redact(v any, keys []string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if contains(keys, k) {
				x[k] = "REDACTED"
				continue
			}
			x[k] = redact(val, keys)
		}
	case []any:
		for i := range x {
			x[i] = redact(x[i], keys)
		}
	}
	return v
}

func contains(s []string, k string) bool {
	for _, v := range s {
		if v == k {
			return true
		}
	}
	return false
}

// TestGoldens pins every method's request and answer. Run with -update after
// a deliberate change to the wire format, and read the diff: the Kotlin side
// decodes exactly these shapes.
func TestGoldens(t *testing.T) {
	for _, gc := range goldenCases {
		t.Run(gc.method, func(t *testing.T) {
			e, c := newEngine(t)
			if gc.setup != nil {
				gc.setup(t, e, c)
			}
			raw := e.Call(gc.method, []byte(gc.req))
			var resp, req any
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(gc.req), &req); err != nil {
				t.Fatal(err)
			}
			if m, ok := resp.(map[string]any); ok {
				if _, isErr := m["err"]; isErr != gc.err {
					t.Fatalf("%s answered an error: %s", gc.method, raw)
				}
			}
			got, err := json.MarshalIndent(map[string]any{"request": req, "response": redact(resp, gc.redact)}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", gc.method+".json")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run go test -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden:\n got: %s\nwant: %s", gc.method, got, want)
			}
		})
	}
}

// Every registered method has a golden, and every golden a method, so a new
// method cannot ship with an unpinned wire format.
func TestEveryMethodHasAGolden(t *testing.T) {
	have := map[string]bool{}
	for _, gc := range goldenCases {
		have[gc.method] = true
	}
	for _, m := range Methods() {
		if !have[m.Name] {
			t.Errorf("method %s has no golden case", m.Name)
		}
	}
	files, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if _, ok := methods[name]; !ok {
			t.Errorf("golden %s has no method", f)
		}
	}
}

func TestRefusalsAreCodes(t *testing.T) {
	e, _ := newEngine(t)
	cases := []struct {
		method, req, want string
	}{
		{"nope.nope", `{}`, CodeUnknownMethod},
		{"board.view", `{}`, CodeNoBoard},
		{"board.new", `{"length":7}`, CodeInvalid},
		{"board.new", `{"length":5,"extra":1}`, CodeBadRequest},
		{"board.new", `{"length":`, CodeBadRequest},
		{"board.resume", `{"id":"` + otherID + `"}`, CodeNotFound},
		{"settings.update", `{"motion":"wild"}`, CodeInvalid},
		{"settings.update", `{"length":3}`, CodeInvalid},
		{"themes.tokens", `{"name":"night","tiles":"neon"}`, CodeInvalid},
	}
	for _, tc := range cases {
		if _, code := call(t, e, tc.method, tc.req); code != tc.want {
			t.Errorf("%s %s: code %q, want %q", tc.method, tc.req, code, tc.want)
		}
	}

	openFixed(t, e, "abo")
	if _, code := call(t, e, "board.submit", `{}`); code != "too_short" {
		t.Errorf("short guess: code %q, want too_short", code)
	}
	call(t, e, "board.trim", `{"index":0}`)
	for _, c := range "zzzzz" {
		call(t, e, "board.type", `{"letter":"`+string(c)+`"}`)
	}
	if _, code := call(t, e, "board.submit", `{}`); code != "not_a_word" {
		t.Errorf("non-word: code %q, want not_a_word", code)
	}
	openFixed(t, e, "")
	e.board.Game().Daily = "2026-11-01"
	if _, code := call(t, e, "board.restart", `{}`); code != "restart_daily" {
		t.Errorf("daily restart: code %q, want restart_daily", code)
	}
}

// A non-letter or a multi-letter request is ignored, as a stray key is.
func TestTypeIgnoresAnythingButOneLetter(t *testing.T) {
	e, _ := newEngine(t)
	openFixed(t, e, "")
	for _, l := range []string{"", "ab", "1", "é", " "} {
		call(t, e, "board.type", `{"letter":"`+l+`"}`)
	}
	if got := e.board.Typing(); got != "" {
		t.Errorf("typing = %q, want nothing", got)
	}
}

// Opening a board leaves the one before it, banking its time.
func TestOpeningABoardLeavesTheLastOne(t *testing.T) {
	e, c := newEngine(t)
	saveGame(t, e, fixedID, "crane", "slate")
	if _, code := call(t, e, "board.resume", `{"id":"`+fixedID+`"}`); code != "" {
		t.Fatal(code)
	}
	call(t, e, "board.type", `{"letter":"c"}`)
	c.advance(6 * time.Second)
	if _, code := call(t, e, "board.new", `{"length":4}`); code != "" {
		t.Fatal(code)
	}
	g, err := e.st.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if g.ElapsedMS != 6000 {
		t.Errorf("the left board banked %d ms, want 6000", g.ElapsedMS)
	}
}

// Deleting the open board closes it: a later leave must not write it back.
func TestDeletingTheOpenBoardClosesIt(t *testing.T) {
	e, _ := newEngine(t)
	openFixed(t, e, "slate")
	call(t, e, "board.submit", `{}`)
	if _, code := call(t, e, "history.delete", `{"id":"`+fixedID+`"}`); code != "" {
		t.Fatal(code)
	}
	if _, code := call(t, e, "board.view", `{}`); code != CodeNoBoard {
		t.Errorf("board after delete: code %q, want no_board", code)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.Load(fixedID); err == nil {
		t.Error("the deleted puzzle came back")
	}
}

// history.list leaves tombstones out and puts the newest first.
func TestHistoryListsLivePuzzlesNewestFirst(t *testing.T) {
	e, _ := newEngine(t)
	saveGame(t, e, fixedID, "crane", "crane")
	time.Sleep(2 * time.Millisecond) // UpdatedAt is the store's wall clock
	saveGame(t, e, otherID, "about", "slate")
	ok, code := call(t, e, "history.list", `{}`)
	if code != "" {
		t.Fatal(code)
	}
	var h History
	if err := json.Unmarshal(ok, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Items) != 2 || h.Items[0].ID != otherID {
		t.Fatalf("history = %+v, want the newer first", h.Items)
	}
	// slate against about: a and t are present, the rest absent. Only marks,
	// never letters.
	if got := h.Items[0].Marks; len(got) != 1 || got[0] != "aayya" {
		t.Errorf("marks = %q, want [aayya]", got)
	}
	if got := h.Items[1].Marks; len(got) != 1 || got[0] != "ggggg" {
		t.Errorf("marks = %q, want [ggggg]", got)
	}
	call(t, e, "history.delete", `{"id":"`+fixedID+`"}`) // finished: a tombstone stays
	ok, _ = call(t, e, "history.list", `{}`)
	h = History{}
	if err := json.Unmarshal(ok, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Items) != 1 {
		t.Errorf("history after a delete = %+v, want one", h.Items)
	}
}

// settings.update changes only what it names.
func TestSettingsPatchKeepsTheRest(t *testing.T) {
	e, _ := newEngine(t)
	call(t, e, "settings.update", `{"displayName":"nick","network":true}`)
	ok, _ := call(t, e, "settings.update", `{"length":4}`)
	var s Settings
	if err := json.Unmarshal(ok, &s); err != nil {
		t.Fatal(err)
	}
	if s.DisplayName != "nick" || !s.Network || s.Length != 4 {
		t.Errorf("settings = %+v", s)
	}
}

func TestThemes(t *testing.T) {
	e, _ := newEngine(t)
	ok, _ := call(t, e, "themes.list", `{}`)
	var th Themes
	if err := json.Unmarshal(ok, &th); err != nil {
		t.Fatal(err)
	}
	names := map[string]ThemeInfo{}
	for _, i := range th.Items {
		names[i.Name] = i
	}
	if _, ok := names["terminal"]; ok {
		t.Error("the ANSI-only terminal theme is listed")
	}
	if n, ok := names["night"]; !ok || !n.Dark || n.Pair != "day" || !n.BuiltIn {
		t.Errorf("night = %+v", n)
	}
	if d, ok := names["day"]; !ok || d.Dark {
		t.Errorf("day = %+v", d)
	}

	// An unknown name and the ANSI-only theme both fall back to night.
	night, _ := call(t, e, "themes.tokens", `{"name":"night","tiles":""}`)
	for _, name := range []string{"no such theme", "terminal"} {
		got, code := call(t, e, "themes.tokens", `{"name":"`+name+`","tiles":""}`)
		if code != "" || !bytes.Equal(got, night) {
			t.Errorf("%q did not fall back to night (code %q)", name, code)
		}
	}

	// A player's file in the themes directory is listed beside the bundled.
	dir := filepath.Join(e.dir, "themes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mine.toml"), []byte("name = \"mine\"\n[colors]\nbg = \"#101010\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, _ = call(t, e, "themes.list", `{}`)
	if !strings.Contains(string(ok), `"name":"mine"`) {
		t.Error("a player's theme file is not listed")
	}
}

// A backup made here restores into a fresh install, and consent to go online
// is never restored.
func TestBackupRoundTrip(t *testing.T) {
	src, _ := newEngine(t)
	saveGame(t, src, fixedID, "crane", "slate", "crane")
	call(t, src, "settings.update", `{"displayName":"nick","network":true}`)
	b, err := src.Export()
	if err != nil {
		t.Fatal(err)
	}

	dst, _ := newEngine(t)
	var env struct {
		OK ImportReport `json:"ok"`
	}
	if err := json.Unmarshal(dst.Import(b), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK.PuzzlesAdded != 1 {
		t.Errorf("report = %+v, want one puzzle added", env.OK)
	}
	if _, err := dst.st.Load(fixedID); err != nil {
		t.Errorf("restored puzzle: %v", err)
	}
	s := dst.st.Settings()
	if s.DisplayName != "nick" || s.Network {
		t.Errorf("restored settings = %+v, want the name and no consent", s)
	}

	if _, code := func() (json.RawMessage, string) {
		var env struct {
			Err *errBody `json:"err"`
		}
		_ = json.Unmarshal(dst.Import([]byte("not a backup")), &env)
		if env.Err == nil {
			return nil, ""
		}
		return nil, env.Err.Code
	}(); code != CodeBadBackup {
		t.Errorf("garbage import: code %q, want bad_backup", code)
	}
}

func TestZone(t *testing.T) {
	e, _ := newEngine(t)
	if err := e.SetZone("Europe/Berlin"); err != nil {
		t.Fatal(err)
	}
	if err := e.SetZone("Not/AZone"); err == nil {
		t.Error("an unknown zone was accepted")
	}
	if e.zone.String() != "Europe/Berlin" {
		t.Errorf("zone = %s", e.zone)
	}
	if _, err := Open(Config{DataDir: t.TempDir(), Zone: "Not/AZone"}); err == nil {
		t.Error("Open accepted an unknown zone")
	}
}

// statusClient answers the status and nothing else.
type statusClient struct{ online.Client }

func (statusClient) Status(context.Context) (online.Status, error) {
	return online.Status{Features: online.Features{Daily: true, Rooms: false}, Message: "hi"}, nil
}

// Consent goes through play: saying yes asks for the status, whose answer
// comes back through the post queue as an "online" event.
func TestConsentAsksForTheStatus(t *testing.T) {
	events := make(chan string, 4)
	e, err := Open(Config{
		DataDir: t.TempDir(), Zone: "UTC", Client: "test-client", Online: statusClient{},
		Emit: func(kind string, _ []byte) { events <- kind },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, code := call(t, e, "settings.update", `{"network":true}`); code != "" {
		t.Fatal(code)
	}
	select {
	case kind := <-events:
		if kind != "online" {
			t.Errorf("event %q, want online", kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event after consent")
	}
	ok, _ := call(t, e, "online.view", `{}`)
	var v struct {
		Consent, Daily, Rooms bool
		Message               string
	}
	if err := json.Unmarshal(ok, &v); err != nil {
		t.Fatal(err)
	}
	if !v.Consent || !v.Daily || v.Rooms || v.Message != "hi" {
		t.Errorf("online view after the status: %+v", v)
	}
}
