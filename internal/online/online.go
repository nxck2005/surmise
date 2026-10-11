// Package online is the game's client for its server API: the daily player
// count now, and online rooms later.
//
// Nothing here decides whether to call the network. That is consent, and it
// lives in store.Settings.Network, which the UI checks before every call. A
// caller without consent uses Off, which never touches the network.
//
// Every request names the client in the X-Surmise-Client header, so the server
// can turn a version away. It does that with 410 Gone and a message, which
// every call returns as *GoneError; the UI then stops using the network for
// the rest of the run and shows the message once. That behaviour has to exist
// in the first release that calls the server at all, because a client without
// it can never learn it.
package online

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ProtocolVersion is the room protocol this client speaks (used from v0.10.0).
const ProtocolVersion = 1

// Timeout bounds one HTTP call.
const Timeout = 5 * time.Second

var (
	// ErrOff is what Off returns for every call.
	ErrOff = errors.New("online: network is off")
	// ErrBadResponse is a 2xx answer this client could not use.
	ErrBadResponse = errors.New("online: bad response")
)

// GoneError is the server turning this client away, with what to tell the
// player.
type GoneError struct{ Message string }

func (e *GoneError) Error() string { return "online: gone: " + e.Message }

// StatusError is any other answer outside 2xx.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("online: server answered %d", e.Code) }

// Features says which online parts the server offers right now.
type Features struct {
	Daily bool `json:"daily"`
	Rooms bool `json:"rooms"`
}

// Status is the server's answer to GET /status.
type Status struct {
	Features Features `json:"features"`
	Message  string   `json:"message"`
}

// DailyCount is how everyone did on one day's board of one length.
// Distribution has length+1 entries: index i is solves in i+1 guesses.
type DailyCount struct {
	Played       int   `json:"played"`
	Solved       int   `json:"solved"`
	Distribution []int `json:"distribution"`
}

// Client is everything the game asks of the server.
type Client interface {
	Status(ctx context.Context) (Status, error)
	PostDaily(ctx context.Context, day string, length int, solved bool, guesses int) error
	Daily(ctx context.Context, day string, length int) (DailyCount, error)
}

// ClientName is the X-Surmise-Client value: "surmise/0.9.0 (linux/amd64)".
// Characters the header's grammar does not allow are dropped from the
// version, and an empty result is "dev".
func ClientName(name, version, goos, goarch string) string {
	clean := make([]rune, 0, len(version))
	for _, r := range version {
		switch {
		case r >= '0' && r <= '9':
			clean = append(clean, r)
		case r >= 'A' && r <= 'Z':
			clean = append(clean, r)
		case r >= 'a' && r <= 'z':
			clean = append(clean, r)
		case r == '.' || r == '+' || r == '-':
			clean = append(clean, r)
		}
	}
	if len(clean) == 0 {
		clean = []rune("dev")
	}
	v := string(clean)
	return fmt.Sprintf("%s/%s (%s/%s)", name, v, goos, goarch)
}
