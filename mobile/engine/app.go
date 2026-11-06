package engine

import (
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/play"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

func init() {
	register("app.info", (*Engine).appInfo)
	register("history.list", (*Engine).historyList)
	register("history.delete", (*Engine).historyDelete)
	register("settings.get", (*Engine).settingsGet)
	register("settings.update", (*Engine).settingsUpdate)
	register("themes.list", (*Engine).themesList)
	register("themes.tokens", (*Engine).themesTokens)
}

// Info is what the about screen and the logs want to know.
type Info struct {
	Version string `json:"version"`
	ABI     string `json:"abi"`
	Client  string `json:"client"`
	DataDir string `json:"dataDir"`
	Zone    string `json:"zone"`
}

func (e *Engine) appInfo(Empty) (Info, error) {
	return Info{Version: e.version, ABI: e.abi, Client: e.client, DataDir: e.dir, Zone: e.zone.String()}, nil
}

// HistoryItem is one saved puzzle as the history list shows it: enough to
// label and sort it, and nothing about the answer.
type HistoryItem struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Length    int    `json:"length"`
	Attempts  int    `json:"attempts"`
	UpdatedAt string `json:"updatedAt"` // RFC 3339, UTC
	Daily     string `json:"daily,omitempty"`
	// Day is the calendar day the puzzle is filed under (stats.PlayDay),
	// YYYY-MM-DD in the device zone, as the calendar files it; empty when
	// there is none.
	Day string `json:"day,omitempty"`
	// Marks is one string per guess, a letter per tile: g for correct, y for
	// present, a for absent. Colours only, so a mini board never shows a word.
	Marks []string `json:"marks"`
}

// History is every live puzzle, most recently updated first.
type History struct {
	Items []HistoryItem `json:"items"`
}

func (e *Engine) historyList(Empty) (History, error) {
	games, err := e.app.History()
	if err != nil {
		return History{}, err
	}
	// The games are the App's shared copy: sort a copy of the slice, never
	// the slice itself.
	live := make([]*game.Game, 0, len(games))
	for _, g := range games {
		if !g.Deleted {
			live = append(live, g)
		}
	}
	slices.SortStableFunc(live, func(a, b *game.Game) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	out := History{Items: make([]HistoryItem, len(live))}
	for i, g := range live {
		out.Items[i] = HistoryItem{
			ID: g.ID, Code: game.Code(g.ID), Kind: play.KindOf(g), Status: string(g.Status),
			Length: g.Length, Attempts: len(g.Guesses),
			UpdatedAt: g.UpdatedAt.UTC().Format(time.RFC3339), Daily: g.Daily,
			Marks: markStrings(g.Marks),
		}
		if d, ok := stats.PlayDay(g.Daily, g.StartedAt, e.zone); ok {
			out.Items[i].Day = d.String()
		}
	}
	return out, nil
}

// markStrings writes each guess's marks as g, y and a.
func markStrings(rows [][]game.Mark) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		b := make([]byte, len(row))
		for j, m := range row {
			switch m {
			case game.Correct:
				b[j] = 'g'
			case game.Present:
				b[j] = 'y'
			default:
				b[j] = 'a'
			}
		}
		out[i] = string(b)
	}
	return out
}

// DeleteRequest removes one puzzle.
type DeleteRequest struct {
	ID string `json:"id"`
}

// historyDelete goes through store.Delete, which leaves a tombstone for a
// finished puzzle and removes an unfinished one. Deleting the open board
// closes it without the Leave that would write it straight back.
func (e *Engine) historyDelete(r DeleteRequest) (Empty, error) {
	if err := e.st.Delete(r.ID); err != nil {
		return Empty{}, err
	}
	if e.board != nil && e.board.Game().ID == r.ID {
		e.board = nil
	}
	return Empty{}, nil
}

// Settings is the phone's view of the saved choices. The terminal-only ones
// (its theme, splash and tile outlines) are not in it: the phone neither shows
// nor changes them, and keeps its own look in DataStore.
type Settings struct {
	DisplayName  string `json:"displayName"`
	Length       int    `json:"length"`
	RememberLast bool   `json:"rememberLast"`
	Motion       string `json:"motion"`
	Network      bool   `json:"network"`
	PlaytimeMS   int64  `json:"playtimeMs"`
}

func settingsOf(s store.Settings) Settings {
	return Settings{
		DisplayName: s.DisplayName, Length: s.Length, RememberLast: s.RememberLast,
		Motion: s.Motion, Network: s.Network, PlaytimeMS: s.PlaytimeMS,
	}
}

func (e *Engine) settingsGet(Empty) (Settings, error) {
	return settingsOf(e.app.Settings()), nil
}

// SettingsPatch changes the fields it sets and leaves the rest. It is the only
// request that may set Network, the consent to go online.
type SettingsPatch struct {
	DisplayName  *string `json:"displayName,omitempty"`
	Length       *int    `json:"length,omitempty"`
	RememberLast *bool   `json:"rememberLast,omitempty"`
	Motion       *string `json:"motion,omitempty"`
	Network      *bool   `json:"network,omitempty"`
}

func (p SettingsPatch) validate() error {
	if p.DisplayName != nil {
		if len(*p.DisplayName) > store.MaxSettingFieldBytes || !utf8.ValidString(*p.DisplayName) {
			return invalid("display name: want valid text of at most %d bytes", store.MaxSettingFieldBytes)
		}
	}
	if p.Length != nil {
		switch *p.Length {
		case 0, 4, 5, 6:
		default:
			return invalid("length %d: want 0, 4, 5 or 6", *p.Length)
		}
	}
	if p.Motion != nil {
		switch *p.Motion {
		case "", "off", "restrained", "pronounced":
		default:
			return invalid("motion %q", *p.Motion)
		}
	}
	return nil
}

func (e *Engine) settingsUpdate(p SettingsPatch) (Settings, error) {
	if err := p.validate(); err != nil {
		return Settings{}, err
	}
	err := e.app.UpdateSettings(func(s *store.Settings) {
		if p.DisplayName != nil {
			s.DisplayName = *p.DisplayName
		}
		if p.Length != nil {
			s.Length = *p.Length
		}
		if p.RememberLast != nil {
			s.RememberLast = *p.RememberLast
		}
		if p.Motion != nil {
			s.Motion = *p.Motion
		}
	})
	if err != nil {
		return Settings{}, err
	}
	// Consent goes through play, which asks for the status on a yes and
	// closes any open room on a no.
	if p.Network != nil {
		if err := e.app.SetNetwork(*p.Network); err != nil {
			return Settings{}, err
		}
	}
	return settingsOf(e.app.Settings()), nil
}

// ThemePreview is a few of a theme's colours, enough to draw its card in the
// picker without asking for every token of every theme.
type ThemePreview struct {
	Canvas  string `json:"canvas"`
	Card    string `json:"card"`
	Ink     string `json:"ink"`
	Accent  string `json:"accent"`
	Action  string `json:"action"`
	Correct string `json:"correct"`
	Present string `json:"present"`
	Absent  string `json:"absent"`
}

// ThemeInfo is one theme the app can offer.
type ThemeInfo struct {
	Name    string       `json:"name"`
	Author  string       `json:"author,omitempty"`
	Dark    bool         `json:"dark"`
	Pair    string       `json:"pair,omitempty"`
	BuiltIn bool         `json:"builtIn"`
	Preview ThemePreview `json:"preview"`
}

// Themes is every theme the app can offer, by name.
type Themes struct {
	Items []ThemeInfo `json:"items"`
}

// library is the bundled themes plus the player's files in <data>/themes,
// which is where a restored backup writes them. It is read on every ask: the
// picker asks rarely, and a restore may have added files since.
func (e *Engine) library() *theme.Library {
	dir := theme.Dir(e.dir)
	if _, err := os.Stat(dir); err != nil {
		dir = ""
	}
	return theme.Open(dir)
}

// themesList leaves out a theme built from ANSI numbers (the terminal's own
// colours, which a phone has none of) and a file that failed to load. Showing
// a pair as one theme is the app's job; this only reports the pair.
func (e *Engine) themesList(Empty) (Themes, error) {
	var out Themes
	for _, en := range e.library().Entries() {
		if en.Theme == nil || en.Theme.ANSIOnly() {
			continue
		}
		tok := theme.App(en.Theme, theme.TilesStandard)
		out.Items = append(out.Items, ThemeInfo{
			Name: en.Name, Author: en.Theme.Author, Dark: tok.Dark, Pair: tok.Pair, BuiltIn: en.Builtin(),
			Preview: ThemePreview{
				Canvas: tok.Canvas, Card: tok.Card, Ink: tok.Ink, Accent: tok.Accent, Action: tok.Action,
				Correct: tok.Tiles.Correct.Fill, Present: tok.Tiles.Present.Fill, Absent: tok.Tiles.Absent.Fill,
			},
		})
	}
	return out, nil
}

// TokensRequest asks for one theme's tokens with a tile palette.
type TokensRequest struct {
	Name  string `json:"name"`
	Tiles string `json:"tiles"` // "", "colorblind" or "high-contrast"
}

// FallbackTheme is what an unknown, unreadable or ANSI-only name resolves to,
// so a theme that stopped shipping or a deleted file never costs a launch.
const FallbackTheme = "night"

func (e *Engine) themesTokens(r TokensRequest) (theme.AppTokens, error) {
	tiles := theme.TilePalette(r.Tiles)
	if !slices.Contains(theme.TilePalettes, tiles) {
		return theme.AppTokens{}, invalid("tiles %q", r.Tiles)
	}
	lib := e.library()
	t, ok := lib.Get(r.Name)
	if !ok || t.ANSIOnly() {
		if t, ok = lib.Get(FallbackTheme); !ok {
			t = theme.Default()
		}
	}
	return theme.App(t, tiles), nil
}
