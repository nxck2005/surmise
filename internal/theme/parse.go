package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// The theme file format is TOML's readable subset: comments, [sections] and
// `key = value` lines. It is parsed here rather than with a TOML library
// because that is the whole of it — the schema is flat, and the project has no
// non-Charm dependencies to spend.
//
// Everything is forgiving on purpose, the way the store recovers from a damaged
// counter instead of refusing to start: a line it cannot make sense of becomes
// a Warning naming the line number, and the rest of the theme still loads. Only
// an unreadable file is an error.

// Warning is one thing wrong with a theme file, reported rather than fatal.
type Warning struct {
	Line int
	Msg  string
}

func (w Warning) String() string { return fmt.Sprintf("line %d: %s", w.Line, w.Msg) }

// Parse reads a theme file, overlaid on Default. name is the fallback display
// name for a file that does not set one.
func Parse(name string, data []byte) (*Theme, []Warning) {
	t := Default().clone()
	t.Name = name

	var warns []Warning
	warn := func(line int, format string, args ...any) {
		warns = append(warns, Warning{Line: line, Msg: fmt.Sprintf(format, args...)})
	}

	section := ""
	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				warn(lineNo, "unclosed section header")
				continue
			}
			section = strings.TrimSpace(line[1:end])
			continue
		}

		eq := strings.Index(line, "=")
		if eq < 0 {
			warn(lineNo, "expected `key = value`")
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value, err := parseValue(line[eq+1:])
		if err != nil {
			warn(lineNo, "%v", err)
			continue
		}
		if key == "" {
			warn(lineNo, "missing key")
			continue
		}
		if section != "" {
			key = section + "." + key
		}
		if err := t.set(key, value); err != nil {
			warn(lineNo, "%v", err)
		}
	}
	return t, warns
}

// parseValue strips quotes and any trailing comment. A comment inside a quoted
// string is part of the string — which matters, since every colour starts with
// a '#'.
func parseValue(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("missing value")
	}
	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		end := strings.IndexByte(s[1:], quote)
		if end < 0 {
			return "", fmt.Errorf("unterminated string")
		}
		return s[1 : 1+end], nil
	}
	if i := strings.IndexByte(s, '#'); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s, nil
}

// set applies one fully-qualified key. Both `bg = "#000"` and a `[colors]`
// section work, since the flat form is what people write by hand and the
// sectioned form is what the bundled files use.
func (t *Theme) set(key, value string) error {
	key = strings.TrimPrefix(key, "colors.")

	switch key {
	case "name":
		t.Name = value
		return nil
	case "author":
		t.Author = value
		return nil
	}

	if isPaletteKey(key) {
		c, err := t.parseColor(value)
		if err != nil {
			return err
		}
		t.palette[key] = c
		return nil
	}

	if rest, ok := strings.CutPrefix(key, "glyphs."); ok {
		return t.setGlyph(rest, value)
	}
	if rest, ok := strings.CutPrefix(key, "metrics."); ok {
		return t.setMetric(rest, value)
	}
	if rest, ok := strings.CutPrefix(key, "style."); ok {
		return t.setStyle(rest, value)
	}
	return fmt.Errorf("unknown key %q", key)
}

func isPaletteKey(key string) bool {
	if _, ok := derivedKeys[key]; ok {
		return true
	}
	for _, k := range baseKeys {
		if k == key {
			return true
		}
	}
	return false
}

// parseColor accepts a hex literal, an ANSI number, or the name of another
// palette entry — the last so a theme can say `accent = "correct"` instead of
// repeating a hex code, and so per-element overrides read in palette terms.
func (t *Theme) parseColor(value string) (color.Color, error) {
	switch {
	case strings.HasPrefix(value, "#"):
		if len(value) != 4 && len(value) != 7 {
			return nil, fmt.Errorf("bad hex colour %q: want #rgb or #rrggbb", value)
		}
		for _, r := range value[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return nil, fmt.Errorf("bad hex colour %q", value)
			}
		}
		return lipgloss.Color(value), nil

	case isPaletteKey(value):
		return t.Color(value), nil
	}

	if n, err := strconv.Atoi(value); err == nil {
		if n < 0 || n > 255 {
			return nil, fmt.Errorf("ANSI colour %d out of range 0-255", n)
		}
		return lipgloss.Color(value), nil
	}
	return nil, fmt.Errorf("unknown colour %q: want #rrggbb, 0-255, or a palette name", value)
}

func (t *Theme) setGlyph(key, value string) error {
	targets := map[string]*string{
		"caret": &t.Glyphs.Caret, "empty": &t.Glyphs.Empty,
		"cursor": &t.Glyphs.Cursor, "cursor_right": &t.Glyphs.CursorRight,
		"separator": &t.Glyphs.Separator,
		"enter":     &t.Glyphs.Enter, "delete": &t.Glyphs.Delete,
		"bar": &t.Glyphs.Bar, "close": &t.Glyphs.Close,
		"border": &t.Glyphs.Border,
	}
	p, ok := targets[key]
	if !ok {
		return fmt.Errorf("unknown glyph %q", key)
	}
	if key == "border" && !knownBorder(value) {
		return fmt.Errorf("unknown border %q: want rounded, normal, thick, double, hidden or block", value)
	}
	*p = value
	return nil
}

func knownBorder(name string) bool {
	switch name {
	case "rounded", "normal", "thick", "double", "hidden", "block":
		return true
	}
	return false
}

func (t *Theme) setMetric(key, value string) error {
	targets := map[string]*int{
		"tile_width": &t.Metrics.TileWidth, "key_pad_x": &t.Metrics.KeyPadX,
		"panel_pad_x": &t.Metrics.PanelPadX, "panel_pad_y": &t.Metrics.PanelPadY,
	}
	p, ok := targets[key]
	if !ok {
		return fmt.Errorf("unknown metric %q", key)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("metric %s: %q is not a number", key, value)
	}
	// Metrics feed straight into widths and padding; a negative one would panic
	// deep inside lipgloss, and an enormous tile would push the board off screen.
	if n < 0 || n > 40 {
		return fmt.Errorf("metric %s: %d out of range 0-40", key, n)
	}
	*p = n
	return nil
}

// setStyle applies one attribute of one element, e.g. `style.tile.correct.bold`.
func (t *Theme) setStyle(key, value string) error {
	dot := strings.LastIndex(key, ".")
	if dot < 0 {
		return fmt.Errorf("style key %q needs an element and an attribute", key)
	}
	element, attr := key[:dot], key[dot+1:]
	if !isElement(element) {
		return fmt.Errorf("unknown element %q", element)
	}

	o := t.Styles[element]
	switch attr {
	case "fg", "bg":
		c, err := t.parseColor(value)
		if err != nil {
			return err
		}
		if attr == "fg" {
			o.FG = c
		} else {
			o.BG = c
		}
	case "bold", "italic", "underline", "faint":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s.%s: %q is not true or false", element, attr, value)
		}
		switch attr {
		case "bold":
			o.Bold = &b
		case "italic":
			o.Italic = &b
		case "underline":
			o.Underline = &b
		case "faint":
			o.Faint = &b
		}
	default:
		return fmt.Errorf("unknown attribute %q on %s", attr, element)
	}
	t.Styles[element] = o
	return nil
}

func isElement(name string) bool {
	for _, e := range Elements {
		if e == name {
			return true
		}
	}
	return false
}

// mustColor is for the built-in palette, whose literals are checked by tests.
func mustColor(hex string) color.Color { return lipgloss.Color(hex) }
