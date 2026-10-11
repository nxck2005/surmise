package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
)

// RoomEntry is one online room this install is in and has not finished
// with: enough to rejoin it. It is not a puzzle and not a preference, so it
// lives in its own small file — rooms.json natively, <name>/v1/rooms in a
// browser — outside Store's records and outside every backup.
//
// It is not a field of Settings because backup.Build compares Settings values
// with ==, and a slice field would not compile there; the rooms list is also
// state of a different kind: what the player is *doing*, not what they chose.
type RoomEntry struct {
	Code     string    `json:"code"`
	Key      string    `json:"key"`
	Mode     string    `json:"mode"` // "live" or "async"
	JoinedAt time.Time `json:"joined_at"`
}

// MaxRooms is how many rooms an install can be in at once.
const MaxRooms = 5

// ValidRoomEntry reports whether e is the shape a room entry is saved in: a
// valid code, a 32-hex seat key, mode live or async, and a joined time.
func ValidRoomEntry(e RoomEntry) bool {
	return game.ValidRoomCode(e.Code) &&
		validSeatKey(e.Key) &&
		(e.Mode == "live" || e.Mode == "async") &&
		!e.JoinedAt.IsZero()
}

// validSeatKey reports whether s is 32 lowercase hex characters, the shape the
// room server mints seat keys in.
func validSeatKey(s string) bool {
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

// roomsFile is the shape of rooms.json and of the browser key's value. It
// carries the same schema tag every record and the settings carry, so a
// future format is a decision rather than a guess.
type roomsFile struct {
	Schema int         `json:"schema"`
	Rooms  []RoomEntry `json:"rooms"`
}

// encodeRooms renders the list for storage. It refuses rather than heals:
// SaveRooms is called with the UI's own list, and a list that is too long, an
// entry nobody minted, or the same room twice is a bug there worth hearing
// about, not a favourite to quietly drop.
func encodeRooms(rs []RoomEntry) ([]byte, error) {
	if len(rs) > MaxRooms {
		return nil, fmt.Errorf("store: %d rooms is more than the %d an install may sit in", len(rs), MaxRooms)
	}
	seen := make(map[string]struct{}, len(rs))
	for _, e := range rs {
		if !ValidRoomEntry(e) {
			return nil, fmt.Errorf("store: room entry %q is not valid", e.Code)
		}
		if _, dup := seen[e.Code]; dup {
			return nil, fmt.Errorf("store: room %q appears twice", e.Code)
		}
		seen[e.Code] = struct{}{}
	}
	b, err := json.Marshal(roomsFile{Schema: schemaVersion, Rooms: rs})
	if err != nil {
		return nil, fmt.Errorf("store: encode rooms: %w", err)
	}
	return b, nil
}

// decodeRooms reads back what encodeRooms wrote. It never fails: the file is
// state the UI rebuilds from the room screen's own experience, so a blob a
// stranger replaced reads as "not in any room" rather than as an error, and an
// entry the reader does not trust is dropped rather than refused. Repeats and
// extras fall away the same way.
func decodeRooms(b []byte) []RoomEntry {
	var f roomsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil
	}
	if f.Schema != 0 && f.Schema != schemaVersion {
		return nil
	}
	out := make([]RoomEntry, 0, len(f.Rooms))
	seen := make(map[string]struct{}, len(f.Rooms))
	for _, e := range f.Rooms {
		if !ValidRoomEntry(e) {
			continue
		}
		if _, dup := seen[e.Code]; dup {
			continue
		}
		seen[e.Code] = struct{}{}
		out = append(out, e)
		if len(out) == MaxRooms {
			break
		}
	}
	return out
}

// --- native ---

const roomsName = "rooms.json"

func (s *JSON) roomsPath() string { return filepath.Join(s.dir, roomsName) }

// Rooms reads the active-rooms list. A missing or damaged file is not an
// error; it reads as "not in any room", which is what an install that has
// never joined one is anyway.
func (s *JSON) Rooms() []RoomEntry {
	b, err := readLimited(s.roomsPath())
	if err != nil {
		return nil
	}
	return decodeRooms(b)
}

// SaveRooms persists the active-rooms list, atomically like every other
// write.
func (s *JSON) SaveRooms(rs []RoomEntry) error {
	b, err := encodeRooms(rs)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.roomsPath(), b)
}

// --- browser ---

// kvRoomsKey holds the active-rooms list beside the settings blob. It is
// versioned from the start like the rest of the key space, and it names the
// product the way every other stored key does — from brand, never from a
// literal.
var kvRoomsKey = brand.Name + "/v1/rooms"

func (s *KVStore) Rooms() []RoomEntry {
	v, ok := s.kv.Get(kvRoomsKey)
	if !ok || len(v) > MaxRecordBytes {
		return nil
	}
	return decodeRooms([]byte(v))
}

func (s *KVStore) SaveRooms(rs []RoomEntry) error {
	b, err := encodeRooms(rs)
	if err != nil {
		return err
	}
	return s.kv.Set(kvRoomsKey, string(b))
}
