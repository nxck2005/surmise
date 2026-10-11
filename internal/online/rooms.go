package online

import (
	"encoding/json"
	"fmt"

	"github.com/nxck2005/surmise/internal/game"
)

// RoomOptions is what making a room asks for.
type RoomOptions struct {
	Mode      string `json:"mode"` // "live" or "async"
	Length    int    `json:"length"`
	AutoStart bool   `json:"autoStart"`
}

// Created is the server's answer to making a room: its code, and the
// creator's seat key (the creator is always seat 1).
type Created struct {
	Code string `json:"code"`
	Key  string `json:"key"`
	Seat int    `json:"seat"`
}

// RankEntry is one line of a room's ranking. Rows are marks, never letters.
type RankEntry struct {
	Seat      int           `json:"seat"`
	Name      string        `json:"name"`
	Solved    bool          `json:"solved"`
	Guesses   int           `json:"guesses"`
	ElapsedMS int64         `json:"elapsedMs"`
	Rows      [][]game.Mark `json:"rows"`
}

// Results is one finished round as the room speaks it: the word is named in
// full, because a round that is over has nothing left to spoil.
type Results struct {
	Round   int         `json:"round"`
	Word    string      `json:"word"`
	Ranking []RankEntry `json:"ranking"`
}

// PlayerView is one seat as the room describes it.
type PlayerView struct {
	Seat      int           `json:"seat"`
	Name      string        `json:"name"`
	Connected bool          `json:"connected"`
	Ready     bool          `json:"ready"`
	InRound   bool          `json:"inRound"`
	Guesses   int           `json:"guesses"`
	Done      bool          `json:"done"`
	Solved    bool          `json:"solved"`
	Rows      [][]game.Mark `json:"rows"`
}

// YouView is our own seat's round: ours carries words as well as marks.
type YouView struct {
	Seat      int           `json:"seat"`
	InRound   bool          `json:"inRound"`
	Words     []string      `json:"words"`
	Marks     [][]game.Mark `json:"marks"`
	Done      bool          `json:"done"`
	Solved    bool          `json:"solved"`
	ElapsedMS int64         `json:"elapsedMs"`
}

// RoomView is the whole room, the way the server sees it right now.
type RoomView struct {
	Code      string       `json:"code"`
	Mode      string       `json:"mode"`
	Length    int          `json:"length"`
	AutoStart bool         `json:"autoStart"`
	Host      int          `json:"host"`
	Phase     string       `json:"phase"`
	Round     int          `json:"round"`
	GoAt      int64        `json:"goAt"`
	EndsAt    int64        `json:"endsAt"`
	ExpiresAt int64        `json:"expiresAt"`
	Players   []PlayerView `json:"players"`
	You       YouView      `json:"you"`
	Results   *Results     `json:"results"`
}

// ServerMsg is any message from a room. Only the fields of its type (T) are
// set; see the room protocol appendix. The pointer fields tell "false" from
// "absent".
type ServerMsg struct {
	T string `json:"t"`

	V       int       `json:"v"`
	Now     int64     `json:"now"`
	Seat    int       `json:"seat"`
	Key     string    `json:"key"`
	Room    *RoomView `json:"room"`
	Reason  string    `json:"reason"`
	Message string    `json:"message"`

	Player    *PlayerView `json:"player"`
	Connected *bool       `json:"connected"`
	Ready     *bool       `json:"ready"`
	GoAt      int64       `json:"goAt"`
	EndsAt    int64       `json:"endsAt"`
	Round     int         `json:"round"`
	N         int         `json:"n"`
	Marks     []game.Mark `json:"marks"`

	Solved    *bool       `json:"solved"`
	Guesses   int         `json:"guesses"`
	ElapsedMS int64       `json:"elapsedMs"`
	Word      string      `json:"word"`
	Ranking   []RankEntry `json:"ranking"`
}

// ClientMsg is a message to a room. Build one with Hello, SetReady, Start,
// Guess, Rematch or Leave: its MarshalJSON writes exactly the fields its
// type has — hello's "key" in particular must be present even when empty,
// because a server reading JSON cannot tell an empty key from an absent one.
type ClientMsg struct {
	t      string
	client string
	name   string
	key    string
	ready  bool
	n      int
	word   string
}

// Hello joins or rejoins a room. key is the seat's key; new seats send "".
func Hello(client, name, key string) ClientMsg {
	return ClientMsg{t: "hello", client: client, name: name, key: key}
}

// SetReady turns our seat's ready flag on or off.
func SetReady(ready bool) ClientMsg { return ClientMsg{t: "ready", ready: ready} }

// Start asks for the round: the host starts a live one, a seat starts its own
// async one.
func Start() ClientMsg { return ClientMsg{t: "start"} }

// Guess plays one word.
func Guess(n int, word string) ClientMsg { return ClientMsg{t: "guess", n: n, word: word} }

// Rematch asks for the next round of a live room.
func Rematch() ClientMsg { return ClientMsg{t: "rematch"} }

// Leave leaves the room for good.
func Leave() ClientMsg { return ClientMsg{t: "leave"} }

// T reports the message type, for tests and the UI.
func (m ClientMsg) T() string { return m.t }

// MarshalJSON writes exactly the fields the message's type carries. Built from
// small struct literals so strings are escaped the way encoding/json does.
func (m ClientMsg) MarshalJSON() ([]byte, error) {
	switch m.t {
	case "hello":
		return json.Marshal(struct {
			T      string `json:"t"`
			V      int    `json:"v"`
			Client string `json:"client"`
			Name   string `json:"name"`
			Key    string `json:"key"`
		}{T: "hello", V: ProtocolVersion, Client: m.client, Name: m.name, Key: m.key})
	case "ready":
		return json.Marshal(struct {
			T     string `json:"t"`
			Ready bool   `json:"ready"`
		}{T: "ready", Ready: m.ready})
	case "guess":
		return json.Marshal(struct {
			T    string `json:"t"`
			N    int    `json:"n"`
			Word string `json:"word"`
		}{T: "guess", N: m.n, Word: m.word})
	case "start", "rematch", "leave":
		return json.Marshal(struct {
			T string `json:"t"`
		}{T: m.t})
	default:
		return nil, fmt.Errorf("online: unknown client message %q", m.t)
	}
}
