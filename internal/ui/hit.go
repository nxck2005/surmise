package ui

// Mouse hit-testing.
//
// Where an element lands on screen is only knowable after three layers of
// centring: each screen's JoinVertical(Center, …), then renderPanel's border and
// padding, then lipgloss.Place in the root View. Predicting that arithmetic is
// possible but silently fragile — lipgloss even rounds the odd cell to the left
// when joining and to the right when placing.
//
// So instead of predicting positions we measure them. Every clickable atom is
// prefixed at render time with a zero-width marker carrying an id (mark); the
// root View then scans the finished frame for those markers to learn their real
// cell coordinates and strips them before the string reaches Bubble Tea (scan).
// The marker is an APC escape: terminals ignore APC strings outright, and the
// ANSI state machine lipgloss measures with counts them as zero width, so a
// marker cannot shift a single cell of layout — TestMarkersDoNotAffectLayout
// holds us to that.
//
// A *hitMap is threaded explicitly through view()/help() rather than kept in a
// package-level manager, and every method is nil-safe, so rendering without
// collecting hit regions stays possible (which is what the layout test needs).

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	markerStart = "\x1b_"
	markerEnd   = "\x1b\\"
)

// actionKind is what a click does. Every keybind that changes state has a kind
// here, which is what makes mouse-only play possible.
type actionKind int

const (
	actNone       actionKind = iota
	actMenuChoice            // index: menu row
	actListRow               // index: puzzle list row
	actLetter                // letter: on-screen keyboard cap
	actSubmit                // enter
	actBackspace             // backspace
	actTrim                  // index: erase the typed row back to this slot
	actNewPuzzle             // tab+enter
	actCancelNew             // dismiss the armed new-puzzle prompt
	actBack                  // esc
	actQuit
)

// action identifies one clickable thing. It is comparable so it doubles as the
// hover key: a screen asks hovered(a) while rendering the very atom it marks.
type action struct {
	kind   actionKind
	index  int
	letter byte
}

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type zone struct {
	act  action
	rect rect
}

// hitMap collects the clickable regions of a single frame. It is rebuilt on
// every render; only hover survives from the previous one.
type hitMap struct {
	zones []zone
	hover action
}

// mark registers s as the click target for a and returns it prefixed with the
// marker scan looks for. The atom's size is taken here, where it is known;
// scan fills in the position.
func (h *hitMap) mark(a action, s string) string {
	if h == nil || a.kind == actNone {
		return s
	}
	id := len(h.zones)
	h.zones = append(h.zones, zone{
		act:  a,
		rect: rect{w: lipgloss.Width(s), h: lipgloss.Height(s)},
	})
	return markerStart + strconv.Itoa(id) + markerEnd + s
}

// hovered reports whether the pointer is currently over a.
func (h *hitMap) hovered(a action) bool {
	return h != nil && a.kind != actNone && h.hover == a
}

// scan records where every marked atom landed and returns the frame with the
// markers removed, ready to hand to the renderer.
func (h *hitMap) scan(frame string) string {
	if h == nil || !strings.Contains(frame, markerStart) {
		return frame
	}

	var b strings.Builder
	b.Grow(len(frame))

	for y, line := range strings.Split(frame, "\n") {
		if y > 0 {
			b.WriteByte('\n')
		}
		// Columns are counted in display cells, not bytes, so styled and wide
		// text place their markers correctly.
		col := 0
		for {
			i := strings.Index(line, markerStart)
			if i < 0 {
				break
			}
			j := strings.Index(line[i:], markerEnd)
			if j < 0 {
				break
			}

			b.WriteString(line[:i])
			col += lipgloss.Width(line[:i])

			if id, err := strconv.Atoi(line[i+len(markerStart) : i+j]); err == nil && id < len(h.zones) {
				h.zones[id].rect.x = col
				h.zones[id].rect.y = y
			}
			line = line[i+j+len(markerEnd):]
		}
		b.WriteString(line)
	}
	return b.String()
}

// at returns the action at a screen cell. Later marks win, so an atom drawn
// inside another is still reachable.
func (h *hitMap) at(x, y int) (action, bool) {
	if h == nil {
		return action{}, false
	}
	for i := len(h.zones) - 1; i >= 0; i-- {
		if z := h.zones[i]; z.rect.w > 0 && z.rect.contains(x, y) {
			return z.act, true
		}
	}
	return action{}, false
}

// find returns where an action was drawn. It is how tests click a target
// without hard-coding coordinates that every layout tweak would invalidate.
func (h *hitMap) find(a action) (rect, bool) {
	if h == nil {
		return rect{}, false
	}
	for _, z := range h.zones {
		if z.act == a {
			return z.rect, true
		}
	}
	return rect{}, false
}
