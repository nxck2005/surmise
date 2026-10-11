package online

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
// to decode (or a 204 answers).
func (h *HTTP) do(ctx context.Context, method, path string, body, out any) error {
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

// compile-time checks.
var (
	_ Client = (*HTTP)(nil)
	_ error  = (*GoneError)(nil)
	_ error  = (*StatusError)(nil)
)
