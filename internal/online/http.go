package online

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/coder/websocket"

	"github.com/nxck2005/surmise/internal/game"
)

// goneDefault is what a 410 with no message tells the player.
const goneDefault = "this version can't use online features any more"

// HTTP is the real client.
type HTTP struct {
	base   string
	client string
	http   *http.Client
}

// New returns a client for an API base such as brand.API, naming itself
// with client (see ClientName). A trailing slash on base is ignored.
func New(base, client string) *HTTP {
	return &HTTP{
		base:   strings.TrimRight(base, "/"),
		client: client,
		http:   &http.Client{Timeout: Timeout},
	}
}

// do makes one call. body is nil for GET; out is nil when nothing comes back
// to decode (or a 204 answers). headers carries any extra ones a call needs;
// the client header is set for every call regardless.
func (h *HTTP) do(ctx context.Context, method, path string, body, out any, headers ...string) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Surmise-Client", h.client)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return err
	}

	code := resp.StatusCode
	if code == http.StatusGone {
		var gone struct {
			Message string `json:"message"`
		}
		if len(data) > 0 && json.Unmarshal(data, &gone) == nil && gone.Message != "" {
			return &GoneError{Message: gone.Message}
		}
		return &GoneError{Message: goneDefault}
	}
	if code < 200 || code > 299 {
		return &StatusError{Code: code}
	}
	if code == http.StatusNoContent || out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return ErrBadResponse
	}
	return nil
}

// Status asks the server what it offers right now.
func (h *HTTP) Status(ctx context.Context) (Status, error) {
	var status Status
	err := h.do(ctx, http.MethodGet, "/status", nil, &status)
	return status, err
}

// PostDaily reports how one daily went, with no identity.
func (h *HTTP) PostDaily(ctx context.Context, day string, length int, solved bool, guesses int) error {
	body := struct {
		Solved  bool `json:"solved"`
		Guesses int  `json:"guesses"`
	}{Solved: solved, Guesses: guesses}
	return h.do(ctx, http.MethodPost, "/daily/"+url.PathEscape(day)+"/"+strconv.Itoa(length), body, nil)
}

// Daily asks how everyone did on one day's board. The answer is checked for
// shape, so a server that has drifted cannot put a broken figure on screen.
func (h *HTTP) Daily(ctx context.Context, day string, length int) (DailyCount, error) {
	var count DailyCount
	err := h.do(ctx, http.MethodGet, "/daily/"+url.PathEscape(day)+"/"+strconv.Itoa(length), nil, &count)
	if err != nil {
		return DailyCount{}, err
	}
	if len(count.Distribution) != length+1 || count.Solved < 0 || count.Played < count.Solved {
		return DailyCount{}, ErrBadResponse
	}
	for _, n := range count.Distribution {
		if n < 0 {
			return DailyCount{}, ErrBadResponse
		}
	}
	return count, nil
}

// CreateRoom makes one. The answer is held to what the game can use before
// the room screens ever see it.
func (h *HTTP) CreateRoom(ctx context.Context, opts RoomOptions) (Created, error) {
	var created Created
	err := h.do(ctx, http.MethodPost, "/rooms", opts, &created)
	if err != nil {
		return Created{}, err
	}
	if !game.ValidRoomCode(created.Code) || !validHex32(created.Key) || created.Seat != 1 {
		return Created{}, ErrBadResponse
	}
	return created, nil
}

// Results reads an any-time room's ranking so far. Only a finished seat of
// that room may read it; the key proves the seat.
func (h *HTTP) Results(ctx context.Context, code, key string) (Results, error) {
	var results Results
	err := h.do(ctx, http.MethodGet, "/rooms/"+url.PathEscape(code)+"/results", nil, &results,
		"Authorization", "Bearer "+key)
	if err != nil {
		return Results{}, err
	}
	if !ValidWord(results.Word, len(results.Word)) || len(results.Word) < 4 || len(results.Word) > 6 {
		return Results{}, ErrBadResponse
	}
	length := len(results.Word)
	if len(results.Ranking) == 0 || len(results.Ranking) > game.MaxRoomRanking {
		return Results{}, ErrBadResponse
	}
	for i, e := range results.Ranking {
		if e.Seat < 1 || e.Guesses < 0 || e.Guesses > length+1 || e.ElapsedMS < 0 ||
			e.ElapsedMS > game.RoomRoundMS || len(e.Rows) != e.Guesses {
			return Results{}, ErrBadResponse
		}
		for _, row := range e.Rows {
			if !ValidMarks(row, length) {
				return Results{}, ErrBadResponse
			}
		}
		// A name the room's own rules would not have produced is blanked
		// rather than believed: the result screen draws these as text.
		if !game.ValidRoomName(e.Name) {
			results.Ranking[i].Name = ""
		}
	}
	return results, nil
}

// Dial opens the room's socket. The WebSocket is kept time-limited only while
// it opens; the connection itself lives past this call and reads with the
// caller's contexts.
func (h *HTTP) Dial(ctx context.Context, code string) (Conn, error) {
	base := h.base
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	url := base + "/rooms/" + url.PathEscape(code) + "/socket"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(64 << 10)
	return newWSConn(conn), nil
}

// compile-time checks.
var (
	_ Client = (*HTTP)(nil)
	_ error  = (*GoneError)(nil)
	_ error  = (*StatusError)(nil)
)

// validHex32 reports whether s is 32 lowercase hex characters, the shape a
// seat key arrives in.
func validHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
