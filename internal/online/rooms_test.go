package online

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/nxck2005/surmise/internal/game"
)

// socketServer is a websocket room the tests point a client at. The handler
// accepts every upgrade and per-connection does its own scripting.
type socketServer struct {
	*httptest.Server

	mu    sync.Mutex
	paths []string
}

// okServer starts the wrapper and remembers every path a connection named.
func okServer(t *testing.T, perConn func(t *testing.T, path string, conn *websocket.Conn)) *socketServer {
	t.Helper()
	wrapper := &socketServer{}
	wrapper.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapper.mu.Lock()
		wrapper.paths = append(wrapper.paths, r.URL.Path)
		wrapper.mu.Unlock()
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusInternalError, "")
		perConn(t, r.URL.Path, conn)
	}))
	t.Cleanup(wrapper.Server.Close)
	return wrapper
}

// writeText sends one text frame for a test.
func writeText(t *testing.T, ctx context.Context, conn *websocket.Conn, s string) {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageText, []byte(s)); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, ctx context.Context, conn *websocket.Conn) string {
	t.Helper()
	typ, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageText {
		t.Fatal("a binary frame")
	}
	return string(b)
}

// phonedClient points a room client at a plain-HTTP test server plus the
// /api/v1 prefix Dials append to.
func phonedClient(base string) *HTTP {
	return New(base+"/api/v1", "surmise/0.10.0 (linux/amd64)")
}

// echoServer answers `ping` with `pong`, the way the protocol's heartbeat
// goes, and echoes any other frame as itself.
func echoServer(t *testing.T) *socketServer {
	t.Helper()
	return okServer(t, func(t *testing.T, _ string, conn *websocket.Conn) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			if string(data) == "ping" {
				writeText(t, ctx, conn, "pong")
				continue
			}
			writeText(t, ctx, conn, string(data))
		}
	})
}

// silentServer accepts and then reads everything and answers nothing: the
// heartbeat's dead window is what ends the connection.
func silentServer(t *testing.T) *socketServer {
	t.Helper()
	return okServer(t, func(t *testing.T, _ string, conn *websocket.Conn) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for {
			if typ, _, err := conn.Read(ctx); err != nil || typ != websocket.MessageText {
				return
			}
		}
	})
}

func TestHelloAlwaysSendsTheKey(t *testing.T) {
	got := make(chan string, 1)
	held := make(chan struct{})
	server := okServer(t, func(t *testing.T, _ string, conn *websocket.Conn) {
		defer func() { held <- struct{}{} }()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		got <- readText(t, ctx, conn)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		for {
			if _, _, err := conn.Read(ctx2); err != nil {
				return
			}
		}
	})

	client := phonedClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()
	conn, err := client.Dial(ctx, "K7M2QX")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, Hello("surmise/0.10.0 (linux/amd64)", "nick", "")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var firstFrame string
	select {
	case firstFrame = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("the server read nothing")
	}
	if !strings.Contains(firstFrame, `"key":""`) || !strings.Contains(firstFrame, `"v":1`) {
		t.Errorf("the hello frame = %s, want an explicit empty key and version 1", firstFrame)
	}
}

func TestClientMessagesHaveExactlyTheirFields(t *testing.T) {
	for _, m := range []struct {
		msg  ClientMsg
		want string
	}{
		{Hello("surmise/0.10.0 (linux/amd64)", "nick", ""),
			`{"t":"hello","v":1,"client":"surmise/0.10.0 (linux/amd64)","name":"nick","key":""}`},
		{Hello("c", "n", "0123456789abcdef0123456789abcdef"),
			`{"t":"hello","v":1,"client":"c","name":"n","key":"0123456789abcdef0123456789abcdef"}`},
		{SetReady(true), `{"t":"ready","ready":true}`},
		{SetReady(false), `{"t":"ready","ready":false}`},
		{Guess(3, "crane"), `{"t":"guess","n":3,"word":"crane"}`},
		{Start(), `{"t":"start"}`},
		{Rematch(), `{"t":"rematch"}`},
		{Leave(), `{"t":"leave"}`},
	} {
		b, err := json.Marshal(m.msg)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != m.want {
			t.Errorf("%s marshal = %s, want %s", m.msg.T(), b, m.want)
		}
	}
}

func TestReceiveSkipsPongAndJunk(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusInternalError, "")
		ctx := context.Background()
		writeText(t, ctx, conn, "pong")
		writeText(t, ctx, conn, "not json")
		writeText(t, ctx, conn, `{"t":"welcome","seat":2,"room":{"code":"K7M2QX"}}`)
		<-block
	}))
	defer server.Close()

	client := phonedClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()
	conn, err := client.Dial(ctx, "K7M2QX")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	msg, err := conn.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if msg.T != "welcome" || msg.Seat != 2 || msg.Room == nil || msg.Room.Code != "K7M2QX" {
		t.Errorf("Receive = %+v, want the welcome past pong and junk", msg)
	}
}

func TestDialUsesTheSocketPath(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	server := okServer(t, func(t *testing.T, path string, conn *websocket.Conn) {
		t.Logf("dialed path %s", path)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	})

	client := phonedClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), DialTimeout)
	defer cancel()
	conn, err := client.Dial(ctx, "K7M2QX")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if want := "/api/v1/rooms/K7M2QX/socket"; server.paths[len(server.paths)-1] != want {
		t.Errorf("dial path = %s, want %s", server.paths[len(server.paths)-1], want)
	}
}

func TestHeartbeatPingsAndDetectsADeadServer(t *testing.T) {
	oldPing, oldDead := pingEvery, deadAfter
	pingEvery, deadAfter = 20*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { pingEvery, deadAfter = oldPing, oldDead })

	t.Run("answered pings keep the connection", func(t *testing.T) {
		client := phonedClient(echoServer(t).URL)
		conn, err := client.Dial(context.Background(), "K7M2QX")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Receive stays blocked for the whole window: the only failure it can
		// report inside it is the test's own deadline.
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		_, err = conn.Receive(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Receive over an answered connection = %v, want the deadline", err)
		}
	})

	t.Run("a silent server is dead", func(t *testing.T) {
		client := phonedClient(silentServer(t).URL)
		conn, err := client.Dial(context.Background(), "K7M2QX")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		type received struct {
			msg ServerMsg
			err error
		}
		got := make(chan received, 1)
		go func() {
			msg, err := conn.Receive(context.Background())
			got <- received{msg, err}
		}()
		select {
		case r := <-got:
			if r.err == nil {
				t.Fatal("Receive answered a message from a silent server")
			}
			if errors.Is(r.err, context.DeadlineExceeded) {
				t.Errorf("Receive reported the context, not a dead connection: %v", r.err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Receive missed a dead server within 1s")
		}
	})
}

func TestCloseIsIdempotent(t *testing.T) {
	client := phonedClient(echoServer(t).URL)
	conn, err := client.Dial(context.Background(), "K7M2QX")
	if err != nil {
		t.Fatal(err)
	}
	first := conn.Close()
	if err := conn.Close(); err != nil {
		t.Errorf("the second Close returned %v, want nil (%v first)", err, first)
	}
}

func TestCreateRoomChecksTheAnswer(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		status  int
		wantErr error
	}{
		{"a whole answer", `{"code":"K7M2QX","key":"0123456789abcdef0123456789abcdef","seat":1}`, http.StatusOK, nil},
		{"a bad code", `{"code":"bad","key":"0123456789abcdef0123456789abcdef","seat":1}`, http.StatusOK, ErrBadResponse},
		{"gone", `{"message":"update"}`, http.StatusGone, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.answer))
			}))
			defer server.Close()

			client := New(server.URL, "surmise/0.10.0 (linux/amd64)")
			created, err := client.CreateRoom(context.Background(), RoomOptions{Mode: "live", Length: 5})
			if tt.wantErr == nil && tt.status == http.StatusOK {
				if err != nil {
					t.Fatalf("CreateRoom: %v", err)
				}
				if created.Code != "K7M2QX" || created.Seat != 1 {
					t.Errorf("created = %+v, want the code and seat 1", created)
				}
				return
			}
			if tt.status == http.StatusGone {
				// A gone answer is a GoneError, told by its message.
				var gone *GoneError
				if !errors.As(err, &gone) || gone.Message != "update" {
					t.Errorf("CreateRoom error = %v, want a *GoneError saying update", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateRoom error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestResultsSendsTheKeyAndChecksRows(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	wantBadRow := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+key {
			t.Errorf("Authorization = %q, want Bearer <key>", got)
		}
		answer := `{"word":"crane","ranking":[{"seat":1,"name":"bad name!","solved":true,` +
			`"guesses":2,"elapsedMs":81000,"rows":[[2,2,2,2,2],[2,2,2,2,2]]}]}`
		if wantBadRow {
			// A deliberate row of the wrong length.
			answer = `{"word":"crane","ranking":[{"seat":1,"name":"x","solved":true,` +
				`"guesses":2,"elapsedMs":0,"rows":[[2,2],[2,2]]}]}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answer))
	}))
	defer server.Close()

	client := New(server.URL, "surmise/0.10.0 (linux/amd64)")

	results, err := client.Results(context.Background(), "K7M2QX", key)
	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	if results.Word != "crane" || len(results.Ranking) != 1 || results.Ranking[0].Name != "" {
		t.Errorf("results = %+v, want the word and a blanked bad name", results)
	}

	wantBadRow = true
	if _, err := client.Results(context.Background(), "K7M2QX", key); !errors.Is(err, ErrBadResponse) {
		t.Errorf("a wrong-length row = %v, want ErrBadResponse", err)
	}
}

func TestRoomName(t *testing.T) {
	for name, want := range map[string]string{
		"Nishchal":              "Nishchal",
		"  sam  ":               "sam",
		"a very long name here": "a very long",
		"émile":                 "mile",
		"<b>x</b>":              "bxb",
		"abbo123":               "",
		"xabbo":                 "xabbo",
		"!!!":                   "",
	} {
		if got := RoomName(name); got != want {
			t.Errorf("RoomName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestOffRefusesRooms(t *testing.T) {
	ctx := context.Background()
	if created, err := Off.CreateRoom(ctx, RoomOptions{Mode: "live", Length: 5}); !errors.Is(err, ErrOff) || created != (Created{}) {
		t.Errorf("Off.CreateRoom = %+v, %v; want zero and ErrOff", created, err)
	}
	if results, err := Off.Results(ctx, "K7M2QX", "key"); !errors.Is(err, ErrOff) || results.Word != "" || results.Ranking != nil {
		t.Errorf("Off.Results = %+v, %v; want zero and ErrOff", results, err)
	}
	if conn, err := Off.Dial(ctx, "K7M2QX"); !errors.Is(err, ErrOff) || conn != nil {
		t.Errorf("Off.Dial = %v, %v; want nil and ErrOff", conn, err)
	}
}

// The marks rules are the wire's, and nothing here may re-derive them: the
// result view and the room rules both go through the same checks.

func TestValidMarksAndWordsAreTheWireRules(t *testing.T) {
	if !ValidMarks([]game.Mark{game.Absent, game.Correct}, 2) {
		t.Error("ValidMarks refused a row of the exact length")
	}
	if ValidMarks([]game.Mark{game.Absent}, 2) || ValidMarks(make([]game.Mark, 3), 2) {
		t.Error("ValidMarks accepted the wrong row length")
	}
	if ValidMarks([]game.Mark{game.Absent, 9}, 2) {
		t.Error("ValidMarks accepted an invented mark")
	}
	if !ValidWord("crane", 5) {
		t.Error("ValidWord refused lowercase letters of the exact length")
	}
	if ValidWord("Crane", 5) || ValidWord("cran", 5) || ValidWord("crane", 6) {
		t.Error("ValidWord accepted something off the shape")
	}
}
