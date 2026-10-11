package online

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientNameCleansTheVersion(t *testing.T) {
	cases := []struct{ name, version, os, arch, want string }{
		{"surmise", "0.9.0", "linux", "amd64", "surmise/0.9.0 (linux/amd64)"},
		{"surmise", "(devel)", "js", "wasm", "surmise/devel (js/wasm)"},
		{"surmise", "", "js", "wasm", "surmise/dev (js/wasm)"},
	}
	for _, c := range cases {
		if got := ClientName(c.name, c.version, c.os, c.arch); got != c.want {
			t.Errorf("ClientName(%q, %q, %q, %q) = %q, want %q", c.name, c.version, c.os, c.arch, got, c.want)
		}
	}
}

// goneServer is a test server answering the named status with the named body.
func goneServer(t *testing.T, code int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
}

func TestEveryRequestNamesTheClient(t *testing.T) {
	const name = "surmise/0.9.0 (linux/amd64)"
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("X-Surmise-Client"); got != name {
			t.Errorf("%s %s carried %q, want %q", r.Method, r.URL.Path, got, name)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			t.Errorf("path = %q, want it under /api/v1/", r.URL.Path)
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/status") {
			_, _ = w.Write([]byte(`{"features":{"daily":true,"rooms":false},"message":""}`))
			return
		}
		_, _ = w.Write([]byte(`{"played":1,"solved":1,"distribution":[0,1,0,0,0,0]}`))
	}))
	t.Cleanup(s.Close)
	c := New(s.URL+"/api/v1", name)
	ctx := context.Background()
	if _, err := c.Status(ctx); err != nil {
		t.Errorf("Status: %v", err)
	}
	if err := c.PostDaily(ctx, "2026-10-11", 5, true, 3); err != nil {
		t.Errorf("PostDaily: %v", err)
	}
	if _, err := c.Daily(ctx, "2026-10-11", 5); err != nil {
		t.Errorf("Daily(): %v", err)
	}
	if calls != 3 {
		t.Errorf("server saw %d calls, want 3", calls)
	}
}

func TestGoneIsAGoneError(t *testing.T) {
	cases := []struct {
		name string
		code int
		body string
	}{
		{name: "with a message", code: http.StatusGone, body: `{"message":"update please"}`},
		{name: "with no body", code: http.StatusGone},
	}
	for _, c := range cases {
		s := goneServer(t, c.code, c.body)
		client := New(s.URL, ClientName("surmise", "0.9.0", "linux", "amd64"))
		ctx := context.Background()

		_, err := client.Status(ctx)
		var gone *GoneError
		if !errors.As(err, &gone) {
			t.Errorf("%s: Status error = %v, want GoneError", c.name, err)
		}
		want := goneDefault
		if c.body != "" {
			want = "update please"
		}
		if gone.Message != want {
			t.Errorf("%s: message = %q, want %q", c.name, gone.Message, want)
		}

		err = client.PostDaily(ctx, "2026-10-11", 5, true, 3)
		if !errors.As(err, &gone) {
			t.Errorf("%s: PostDaily error = %v, want GoneError", c.name, err)
		}

		_, err = client.Daily(ctx, "2026-10-11", 5)
		if !errors.As(err, &gone) {
			t.Errorf("%s: Daily() error = %v, want GoneError", c.name, err)
		}
	}
}

func TestOtherStatusesAreStatusErrors(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		s := goneServer(t, code, "{}")
		c := New(s.URL, ClientName("surmise", "0.9.0", "linux", "amd64"))
		ctx := context.Background()

		_, err := c.Status(ctx)
		var se *StatusError
		if !errors.As(err, &se) || se.Code != code {
			t.Errorf("status %d: Status error = %v, want StatusError(%d)", code, err, code)
		}

		err = c.PostDaily(ctx, "2026-10-11", 5, true, 3)
		if !errors.As(err, &se) || se.Code != code {
			t.Errorf("status %d: PostDaily error = %v, want StatusError(%d)", code, err, code)
		}
	}
}

func TestPostDailySendsTheResult(t *testing.T) {
	path, body := "", ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(s.Close)
	c := New(s.URL, ClientName("surmise", "0.9.0", "linux", "amd64"))
	if err := c.PostDaily(context.Background(), "2026-10-11", 5, true, 3); err != nil {
		t.Fatalf("PostDaily: %v", err)
	}
	wantPath := "POST /daily/2026-10-11/5"
	if path != wantPath {
		t.Errorf("request was %q, want %q", path, wantPath)
	}
	var sent struct {
		Solved  bool `json:"solved"`
		Guesses int  `json:"guesses"`
	}
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("body %q does not decode: %v", body, err)
	}
	if sent.Solved != true || sent.Guesses != 3 {
		t.Errorf("body was %q, want the solved-in-3 result", body)
	}
}

func TestDailyRefusesABadShape(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "distribution of the wrong length", body: `{"played":3,"solved":3,"distribution":[1,2]}`},
		{name: "solved above played", body: `{"played":1,"solved":4,"distribution":[0,1,0,0,0,0]}`},
	}
	for _, c := range cases {
		s := goneServer(t, http.StatusOK, c.body)
		client := New(s.URL, ClientName("surmise", "0.9.0", "linux", "amd64"))
		_, err := client.Daily(context.Background(), "2026-10-11", 5)
		if !errors.Is(err, ErrBadResponse) {
			t.Errorf("%s: err = %v, want ErrBadResponse", c.name, err)
		}
	}
}

func TestOffNeverCalls(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	_ = s // Off must never reach even this server
	c := Off
	ctx := context.Background()
	if _, err := c.Status(ctx); !errors.Is(err, ErrOff) {
		t.Errorf("Off.Status err = %v, want ErrOff", err)
	}
	if err := c.PostDaily(ctx, "2026-10-11", 5, true, 3); !errors.Is(err, ErrOff) {
		t.Errorf("Off.PostDaily err = %v, want ErrOff", err)
	}
	if _, err := c.Daily(ctx, "2026-10-11", 5); !errors.Is(err, ErrOff) {
		t.Errorf("Off.Daily() err = %v, want ErrOff", err)
	}
	if calls != 0 {
		t.Errorf("the server saw %d calls, want none", calls)
	}
}
