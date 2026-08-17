package store

import (
	"sort"
	"strings"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
)

// KV is a flat string-to-string store with no directories and no atomic rename
// — that is, a browser's localStorage, which is what this exists for.
//
// The methods are synchronous and return no context, because Store's are.
// localStorage fits that; IndexedDB would not, and adopting it later means
// changing Store itself rather than writing another KV.
type KV interface {
	// Get returns the value for a key, and whether it was there at all. A
	// missing key is not an error.
	Get(key string) (string, bool)
	// Set writes a value. It errors when the backing store refuses — a browser
	// raises QuotaExceededError when its origin allowance is full.
	Set(key, value string) error
	// Delete removes a key. Removing a key that is not there is not an error.
	Delete(key string) error
	// Keys returns every key present, in no particular order.
	Keys() []string
}

// KVStore is a Store over a KV.
//
// It holds exactly the bytes JSON holds, under a key instead of a path, so the
// two are the same history in two places and the tombstone rules come from one
// codec (see codec.go). Nothing here is browser-specific; the browser part is
// the KV it is given, so all of this is testable on any platform.
type KVStore struct{ kv KV }

// KVStore has to satisfy both Store and the preferences pair the UI looks for
// by type assertion (see settingsStore in internal/ui/app.go). That assertion
// fails silently — settings simply stop persisting — so the requirement is
// pinned here, where dropping a method is a compile error instead.
var _ interface {
	Store
	Settings() Settings
	SaveSettings(Settings) error
} = (*KVStore)(nil)

// NewKV wraps a KV as a Store.
func NewKV(kv KV) *KVStore { return &KVStore{kv: kv} }

// The key space is versioned from the start. Nothing reads v1 as anything but a
// literal today; it is there so a later format can live beside this one rather
// than having to guess at what an unprefixed key holds. The product name comes
// from brand, never from a literal, like every other user-visible string.
var (
	kvPuzzlePrefix = brand.Name + "/v1/puzzle/"
	kvSettingsKey  = brand.Name + "/v1/settings"
)

func kvPuzzleKey(id string) string { return kvPuzzlePrefix + id }

func (s *KVStore) Save(g *game.Game) error {
	b, err := encodeRecord(g)
	if err != nil {
		return err
	}
	return s.kv.Set(kvPuzzleKey(g.ID), string(b))
}

// Load returns a playable puzzle. A tombstone is reported as ErrNotFound: it is
// a record of the sequence of play, not a puzzle, and nothing may resume one.
func (s *KVStore) Load(id string) (*game.Game, error) {
	g, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if g.Deleted {
		return nil, ErrNotFound
	}
	return g, nil
}

// load reads whatever is stored, tombstones included. Only Delete and All,
// which have to see deletions, use it directly.
func (s *KVStore) load(id string) (*game.Game, error) {
	v, ok := s.kv.Get(kvPuzzleKey(id))
	if !ok {
		return nil, ErrNotFound
	}
	return decodeGame(id, []byte(v))
}

// Delete removes a puzzle, leaving a tombstone for a finished one.
//
// The rule is JSON's, for the same reason: a deleted loss still has to break a
// run of wins, or deleting it would merge the runs either side and inflate the
// longest streak. There is no atomic-write dance here because there is nothing
// to make atomic — a single key is written whole or not at all.
func (s *KVStore) Delete(id string) error {
	g, err := s.load(id)
	if err != nil {
		return err
	}
	if g.Deleted {
		return ErrNotFound
	}

	if g.Status.Done() {
		b, err := encodeTombstone(g.Tombstone())
		if err != nil {
			return err
		}
		return s.kv.Set(kvPuzzleKey(id), string(b))
	}

	return s.kv.Delete(kvPuzzleKey(id))
}

// All returns every readable record, tombstones included — stats need them to
// see where a deleted puzzle broke a streak. An unreadable value is skipped
// rather than failing the whole call, so one bad record cannot lock the player
// out of their history.
func (s *KVStore) All() ([]*game.Game, error) {
	keys := s.kv.Keys()
	games := make([]*game.Game, 0, len(keys))
	for _, k := range keys {
		id, ok := strings.CutPrefix(k, kvPuzzlePrefix)
		if !ok || id == "" {
			continue
		}
		g, err := s.load(id)
		if err != nil {
			continue
		}
		games = append(games, g)
	}
	// Keys() has no defined order, and All feeds the streak walks, which read
	// their input in order. JSON gets its order from the directory listing; give
	// this one the same determinism rather than leaving it to the browser.
	sort.Slice(games, func(i, j int) bool { return games[i].ID < games[j].ID })
	return games, nil
}

func (s *KVStore) List() ([]Summary, error) {
	games, err := s.All()
	if err != nil {
		return nil, err
	}
	return summaries(games), nil
}

// Settings reads the saved preferences. A missing or damaged value yields the
// defaults rather than an error: a bad settings blob must never cost a puzzle.
func (s *KVStore) Settings() Settings {
	v, ok := s.kv.Get(kvSettingsKey)
	if !ok {
		return Settings{}
	}
	return decodeSettings([]byte(v))
}

func (s *KVStore) SaveSettings(v Settings) error {
	b, err := encodeSettings(v)
	if err != nil {
		return err
	}
	return s.kv.Set(kvSettingsKey, string(b))
}

// NewMemoryKV is a KV in a map. It is the test double, and it is also the
// fallback when a browser refuses to hand over localStorage at all — Safari
// throws on it in private mode. Falling back means the game is playable and
// forgets everything on reload, which is a better answer than a blank page.
func NewMemoryKV() KV { return memoryKV{m: map[string]string{}} }

type memoryKV struct{ m map[string]string }

func (k memoryKV) Get(key string) (string, bool) { v, ok := k.m[key]; return v, ok }
func (k memoryKV) Set(key, value string) error   { k.m[key] = value; return nil }
func (k memoryKV) Delete(key string) error       { delete(k.m, key); return nil }

func (k memoryKV) Keys() []string {
	out := make([]string, 0, len(k.m))
	for key := range k.m {
		out = append(out, key)
	}
	return out
}
