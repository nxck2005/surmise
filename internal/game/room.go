package game

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/words"
)

// RoomRoundMS is the longest a room round can last; the server ends it then.
const RoomRoundMS = 600_000

// MaxRoomRanking is the most entries a saved room ranking may hold: the size
// of the largest room.
const MaxRoomRanking = 50

// roomRoundTag makes a saved round's id. It is wire format, like the daily's
// derivation tags: brand-neutral, and never changed once released.
const roomRoundTag = "room-round-v1\x00"

// RoomInfo marks a finished round played in an online room, and carries what
// the result screen needs to show it again. The pointer is the origin marker;
// a tombstone keeps an empty RoomInfo and drops everything else.
type RoomInfo struct {
	Code    string     `json:"code,omitempty"`
	Mode    string     `json:"mode,omitempty"` // "live" or "async"
	Round   int        `json:"round,omitempty"`
	Seat    int        `json:"seat,omitempty"`
	Key     string     `json:"key,omitempty"` // async only: reads newer results
	Ranking []RoomRank `json:"ranking,omitempty"`
}

// RoomRank is one line of a saved ranking: colours only, never letters. The
// names travel with the result so the ranking can be shown after the fact;
// each one is the room name's cleaned form, not the display name.
type RoomRank struct {
	Seat      int      `json:"seat"`
	Name      string   `json:"name"`
	Solved    bool     `json:"solved"`
	Guesses   int      `json:"guesses"`
	ElapsedMS int64    `json:"elapsedMs"`
	Rows      [][]Mark `json:"rows"`
}

// roomCodeAlphabet is the alphabet a room code is drawn from, capitals only:
// a code is read aloud and typed by hand, so I, L, O and U do not appear in
// it. The server draws from the same characters; this list holds what may be
// saved.
const roomCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ValidRoomCode reports whether s is a room code: six characters of
// 0123456789ABCDEFGHJKMNPQRSTVWXYZ.
func ValidRoomCode(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(roomCodeAlphabet, rune(s[i])) {
			return false
		}
	}
	return true
}

// ValidRoomName reports whether s is a cleaned room name: 0–12 bytes, each
// one of A–Z a–z 0–9 space _ . -, with no leading or trailing space.
func ValidRoomName(s string) bool {
	if len(s) > 12 || s != "" && (s[0] == ' ' || s[len(s)-1] == ' ') {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == ' ' || c == '_' || c == '.' || c == '-':
		default:
			return false
		}
	}
	return true
}

// RoomRoundID is the id a finished round is saved under: a UUIDv8 from
// SHA-256(roomRoundTag + code + "\x00" + round + "\x00" + seat), with round
// and seat written in decimal. The same round saved twice — after a
// reconnect, say — lands on the same record instead of a second one.
func RoomRoundID(code string, round, seat int) string {
	h := sha256.Sum256([]byte(roomRoundTag + code + "\x00" + strconv.Itoa(round) + "\x00" + strconv.Itoa(seat)))
	var b [16]byte
	copy(b[:], h[:16])
	return FormatID(b, 8)
}

// NewRoomRound builds the finished game for a round the server has scored.
// The marks are scored here from the answer, and the caller compares them
// with the server's (see the UI); a game.Game always scores itself.
func NewRoomRound(info RoomInfo, answer string, guesses []string, elapsedMS int64, finishedAt time.Time) (*Game, error) {
	answer = words.Normalize(answer)
	length := len(answer)
	g := &Game{
		ID:          RoomRoundID(info.Code, info.Round, info.Seat),
		Length:      length,
		Answer:      answer,
		MaxAttempts: attemptsFor(length),
		ElapsedMS:   elapsedMS,
		// The board's clock is the round's: it started elapsedMS ago and ended
		// when the round did, so the playtime is that window.
		StartedAt: finishedAt.Add(-time.Duration(elapsedMS) * time.Millisecond).UTC(),
		UpdatedAt: finishedAt.UTC(),
	}
	// info is a value parameter, so a pointer to it is a copy of the caller's
	// struct rather than an alias of it.
	g.Room = &info

	for i, guess := range guesses {
		if len(guess) != length || !lettersOnly(guess) {
			return nil, fmt.Errorf("game: room guess %d %q is not a %d-letter word", i, guess, length)
		}
		g.Guesses = append(g.Guesses, guess)
		g.Marks = append(g.Marks, Score(guess, answer))
	}

	if len(g.Guesses) > 0 && g.Guesses[len(g.Guesses)-1] == answer {
		g.Status = Won
	} else {
		g.Status = Lost
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

// validRoom holds a record that marks a room round to its own origin: the
// id answers for the round and seat, the round is finished — an unfinished
// one lives only on the server and in memory — and the ranking is the size
// a room can hold, with the player themselves among it.
func validRoom(g *Game) error {
	r := g.Room
	switch {
	case g.Custom || g.Daily != "" || g.Challenge != nil:
		return fmt.Errorf("game: room round has another puzzle origin")
	case !ValidRoomCode(r.Code):
		return fmt.Errorf("game: room code is not valid")
	case r.Mode != "live" && r.Mode != "async":
		return fmt.Errorf("game: room mode %q is not valid", r.Mode)
	case r.Round < 1 || r.Round > 10_000 || r.Seat < 1 || r.Seat > MaxRoomRanking:
		return fmt.Errorf("game: room round or seat is out of range")
	case r.Key != "" && !validHex32(r.Key), r.Mode == "live" && r.Key != "":
		return fmt.Errorf("game: room key is not valid")
	case !g.Status.Done():
		return fmt.Errorf("game: room round is not finished")
	case g.ID != RoomRoundID(r.Code, r.Round, r.Seat):
		return fmt.Errorf("game: room round id does not match")
	case g.ElapsedMS > RoomRoundMS:
		return fmt.Errorf("game: room round time is out of range")
	case len(r.Ranking) == 0 || len(r.Ranking) > MaxRoomRanking:
		return fmt.Errorf("game: room ranking size is out of range")
	}
	for i, e := range r.Ranking {
		switch {
		case e.Seat < 1 || e.Seat > MaxRoomRanking:
			return fmt.Errorf("game: room ranking entry %d is not valid", i)
		case !ValidRoomName(e.Name):
			return fmt.Errorf("game: room ranking entry %d is not valid", i)
		case e.Guesses < 0 || e.Guesses > g.MaxAttempts:
			return fmt.Errorf("game: room ranking entry %d is not valid", i)
		case e.ElapsedMS < 0 || e.ElapsedMS > RoomRoundMS:
			return fmt.Errorf("game: room ranking entry %d is not valid", i)
		case len(e.Rows) != e.Guesses:
			return fmt.Errorf("game: room ranking entry %d is not valid", i)
		}
		for _, row := range e.Rows {
			if len(row) != g.Length {
				return fmt.Errorf("game: room ranking entry %d is not valid", i)
			}
			for _, m := range row {
				if m < Absent || m > Correct {
					return fmt.Errorf("game: room ranking entry %d is not valid", i)
				}
			}
		}
	}
	for _, e := range r.Ranking {
		if e.Seat == r.Seat {
			return nil
		}
	}
	return fmt.Errorf("game: room ranking does not include this player")
}

// validHex32 reports whether s is 32 lowercase hex characters, the shape of a
// seat key. An empty string is not one: callers that allow no key check for
// that separately.
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
