package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
)

// renderBoard draws every attempt row: guesses already played, the row being
// typed, then empty rows.
//
// a may be nil, which draws the settled board — every row scored, nothing
// flashing. That is what a caller with no animation state gets, and it is the
// frame every layout test compares against.
// tall is how many display rows a tile takes when the terminal can afford it,
// and flat is the one row it takes otherwise. Three is the smallest height with
// a middle for the letter to sit in.
const (
	flatTile = 1
	tallTile = 3
)

func renderBoard(g *game.Game, typing string, h *hitMap, a *anims, now time.Time, tiles, gap int, outline bool) string {
	rows := make([]string, 0, g.MaxAttempts)

	for i, guess := range g.Guesses {
		if shown, revealing := a.reveal(now, g.ID, i); revealing {
			rows = append(rows, renderRevealingRow(guess, g.Marks[i], shown, tiles, outline))
			continue
		}
		// A won row is walked once by a light after it has turned. It renders
		// through the same scored tiles either way, so the settled row is the
		// row this board always drew.
		if lit, celebrating := a.celebrating(now, g.ID, i); celebrating {
			rows = append(rows, renderCelebratingRow(guess, g.Marks[i], lit, tiles))
			continue
		}
		rows = append(rows, renderScoredRowAt(guess, g.Marks[i], tiles))
	}

	if !g.Status.Done() {
		rows = append(rows, renderTypingRow(typing, g.Length, h, a.rejected(now), tiles, outline))
	}

	for len(rows) < g.MaxAttempts {
		rows = append(rows, renderEmptyRow(g.Length, tiles, outline))
	}

	return stackSpaced(rows, gap)
}

// sized gives a tile its height. One row is the style exactly as the theme
// built it — the frame this board has always drawn — so nothing about a flat
// board goes through a different path from the one it used before.
func sized(s lipgloss.Style, rows int) lipgloss.Style {
	if rows <= flatTile {
		return s
	}
	return s.Height(rows).AlignVertical(lipgloss.Center)
}

// stackSpaced joins rows vertically with gap blank lines between each, so guess
// rows and keyboard rows breathe instead of stacking flush.
//
// A gap of zero is the tightened board a short terminal falls back to, and it
// is the one place this app lets filled backgrounds touch: see boardLayout in
// gamescreen.go for what it buys and why the alternative is worse. Everything
// outside the board being played passes one.
func stackSpaced(rows []string, gap int) string {
	spaced := make([]string, 0, len(rows)*(gap+1))
	for i, r := range rows {
		if i > 0 {
			for range gap {
				spaced = append(spaced, "")
			}
		}
		spaced = append(spaced, r)
	}
	return joinVerticalCentre(spaced...)
}

// renderScoredRow draws a finished row at one row tall. It is what everything
// outside the board — the debrief, the how-to-play examples, the theme preview —
// shows a scored word with, and those never grow: only the board a player is
// looking at is worth the height.
func renderScoredRow(guess string, marks []game.Mark) string {
	return renderScoredRowAt(guess, marks, flatTile)
}

func renderScoredRowAt(guess string, marks []game.Mark, tiles int) string {
	cells := make([]string, len(guess))
	for i := range guess {
		cells[i] = st.cell(faceForMark(marks[i]), upperLetter(guess[i]), tiles, false)
	}
	return joinTiles(cells)
}

// faceForMark maps a mark to the tile face that draws it.
func faceForMark(m game.Mark) cellFace {
	switch m {
	case game.Correct:
		return faceCorrect
	case game.Present:
		return facePresent
	default:
		return faceAbsent
	}
}

// renderCelebratingRow draws a solved row with one tile lit. The light is the
// tile's own background lifted, not a colour of its own, so every theme gets
// the effect without naming an element for it — and a terminal too poor to
// blend gets the settled row, because lift gives its input back.
func renderCelebratingRow(guess string, marks []game.Mark, lit, tiles int) string {
	cells := make([]string, len(guess))
	for i := range guess {
		if i == lit {
			// The lit tile is the only one that cannot come from the cache: its
			// background is mixed for this frame. Every other tile is the
			// settled one the board always draws.
			style := tileStyle(marks[i]).Background(lift(tileStyle(marks[i]).GetBackground(), 0.35))
			cells[i] = sized(style, tiles).Render(upperLetter(guess[i]))
			continue
		}
		cells[i] = st.cell(faceForMark(marks[i]), upperLetter(guess[i]), tiles, false)
	}
	return joinTiles(cells)
}

// tileStyle is the tile a mark is drawn with. It is the one place the three are
// chosen, so the board, the reveal, the celebration and the legend cannot
// disagree about what a mark looks like.
func tileStyle(mark game.Mark) lipgloss.Style {
	switch mark {
	case game.Correct:
		return st.tileCorrect
	case game.Present:
		return st.tilePresent
	default:
		return st.tileAbsent
	}
}

// upperLetters are the letters a board draws, already upper-cased. A stored
// guess is lowercase ASCII (game.Validate holds it to that), so looking one up
// is the whole conversion — the old strings.ToUpper(string(c)) allocated two
// strings per tile, and a board draws thirty tiles a frame.
var upperLetters = [26]string{
	"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M",
	"N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
}

// upperLetter upper-cases one byte of a stored word. Anything outside a–z goes
// through the rune conversion, which is what string(byte) meant before, so a
// caller that does hand over something else still gets the old answer.
func upperLetter(c byte) string {
	if c >= 'a' && c <= 'z' {
		return upperLetters[c-'a']
	}
	return strings.ToUpper(string(rune(c)))
}

// cellFace names the style one cell is drawn with. It is the key the cell cache
// uses for "which style", and the one place a face is mapped to a style.
type cellFace uint8

const (
	faceCorrect cellFace = iota
	facePresent
	faceAbsent
	faceActive
	faceActiveRefused
	faceCaret
	faceEmpty
	faceEmptyBlank
	faceKeyUnused
	faceKeyCorrect
	faceKeyPresent
	faceKeyAbsent
)

// slot reports whether this face is drawn as a slot rather than as a scored
// tile or keycap: only a slot goes through slotCell, whose outline and frame
// depend on the board's setting.
func (f cellFace) slot() bool {
	switch f {
	case faceActive, faceActiveRefused, faceCaret, faceEmpty, faceEmptyBlank:
		return true
	}
	return false
}

// faceStyle is the one mapping from a face to the styles it renders with.
func (s *styles) faceStyle(f cellFace) (style, frame lipgloss.Style) {
	switch f {
	case faceCorrect:
		return s.tileCorrect, s.muted
	case facePresent:
		return s.tilePresent, s.muted
	case faceAbsent:
		return s.tileAbsent, s.muted
	case faceActive:
		return s.tileActive, s.muted
	case faceActiveRefused:
		return s.tileRejected, s.muted
	case faceCaret:
		return s.caret, s.muted
	case faceEmpty:
		return s.tileEmpty, s.muted
	case faceEmptyBlank:
		return s.tileEmpty, s.emptyFrame
	case faceKeyCorrect:
		return s.keyCorrect, s.muted
	case faceKeyPresent:
		return s.keyPresent, s.muted
	case faceKeyAbsent:
		return s.keyAbsent, s.muted
	default:
		return s.keyUnused, s.muted
	}
}

// cellKey identifies one rendered atom. text is the letter or glyph already in
// its display form; outline only participates for a slot, and is normalised to
// false otherwise so the cache does not split one keycap across two entries.
type cellKey struct {
	face    cellFace
	text    string
	tiles   int
	outline bool
}

// cell returns the rendered atom for a face, height and text, drawing it once
// per style set.
//
// Every input is a function of the style set (or of the board's outline
// setting, which is in the key), so a cached atom is the same bytes Render
// would produce. The cache is lazy rather than prebuilt because a theme preview
// walks a whole library and only a handful of faces are ever drawn from each;
// prebuilding 26 letters across twelve faces would pay for cells nobody sees.
func (s *styles) cell(face cellFace, text string, tiles int, outline bool) string {
	slot := face.slot()
	if !slot {
		outline = false
	}
	key := cellKey{face: face, text: text, tiles: tiles, outline: outline}
	if v, ok := s.cells[key]; ok {
		return v
	}
	style, frame := s.faceStyle(face)
	var v string
	if slot {
		v = slotCell(style, frame, text, tiles, outline)
	} else {
		v = sized(style, tiles).Render(text)
	}
	if s.cells == nil {
		s.cells = make(map[cellKey]string, 64)
	}
	s.cells[key] = v
	return v
}

// renderRevealingRow draws a scored row part-way through its reveal: the first
// shown tiles in the colours they scored, the rest still wearing the style they
// were typed in. Both come from tile(), so both are exactly TileWidth wide — the
// flip is a repaint, and the row resolves from what you wrote into what it
// scored without anything moving.
func renderRevealingRow(guess string, marks []game.Mark, shown, tiles int, outline bool) string {
	if shown >= len(guess) {
		return renderScoredRowAt(guess, marks, tiles)
	}
	cells := make([]string, len(guess))
	for i := range guess {
		letter := upperLetter(guess[i])
		if i >= shown {
			cells[i] = st.cell(faceActive, letter, tiles, outline)
			continue
		}
		cells[i] = st.cell(faceForMark(marks[i]), letter, tiles, false)
	}
	return joinTiles(cells)
}

// renderTypingRow draws the row being written. rejected flashes it in the error
// colour: tileActive fills nothing, so this repaints the letters and leaves the
// row exactly where it was. A positional shake would read better for a moment
// and cost more than it is worth — the typed tiles are click targets (actTrim
// below), and a target sliding under a stationary pointer trims to a slot the
// player was not pointing at.
func renderTypingRow(typing string, length int, h *hitMap, rejected bool, tiles int, outline bool) string {
	cells := make([]string, length)
	for i := range cells {
		if i < len(typing) {
			// A typed letter is a click target: clicking it erases the row back
			// to that slot, which is how a mouse edits a mistake mid-word.
			trim := action{kind: actTrim, index: i}
			face := faceActive
			if rejected {
				face = faceActiveRefused
			}
			cell := st.cell(face, upperLetter(typing[i]), tiles, outline)
			if h.hovered(trim) {
				// Hover is one atom at a time and changes per frame, so it is
				// the one cell that is not taken from the cache.
				style := st.tileActive
				if rejected {
					style = st.tileRejected
				}
				cell = slotCell(st.hover(style), st.muted, upperLetter(typing[i]), tiles, outline)
			}
			// The whole cell is the target, however tall it is: mark records the
			// atom's height, so a tall tile is clickable across all of it.
			cells[i] = h.mark(trim, cell)
		} else if i == len(typing) {
			// Mark the caret position so the player can see where input lands.
			cells[i] = st.cell(faceCaret, st.glyph.Caret, tiles, outline)
		} else {
			cells[i] = st.cell(faceEmpty, st.glyph.Empty, tiles, outline)
		}
	}
	return joinTiles(cells)
}

func renderEmptyRow(length, tiles int, outline bool) string {
	cells := make([]string, length)
	// Every empty cell of a row is the same atom.
	cell := st.cell(faceEmptyBlank, st.glyph.Empty, tiles, outline)
	for i := range cells {
		cells[i] = cell
	}
	return joinTiles(cells)
}

// slotCell draws a tile that has not been scored yet: an empty slot, or a
// letter still being typed. One row tall it is the tile style alone, as it has
// always been. With the tile-outline setting on (outline; off by default), a
// tall tile is drawn as a box in the theme's border glyphs instead, because a filled tile three rows high beside a lone glyph in three
// rows of nothing made the unplayed part of the board read as missing rather
// than waiting. frame colours the box: the slot colour for an empty row, muted
// for the row being typed, so the row that takes the next letter stands out.
//
// The box occupies exactly the cells the tile did — the border takes the
// outer cells and the content keeps the middle — so scoring a row is still a
// repaint, never a move. A tile too narrow for a border and its content
// keeps the flat form, and a block border, which would fill the ring
// solid, outlines with the plain glyphs instead.
func slotCell(style, frame lipgloss.Style, content string, tiles int, outline bool) string {
	w := st.metric.TileWidth
	if !outline || tiles < tallTile || w < 3 {
		return sized(style, tiles).Render(content)
	}
	b := st.borderRunes()
	if st.glyph.Border == "block" {
		b = lipgloss.NormalBorder()
	}
	inner := strings.Split(sized(style.Width(w-2), tiles-2).Render(content), "\n")
	lines := make([]string, 0, tiles)
	lines = append(lines, frame.Render(b.TopLeft+strings.Repeat(b.Top, w-2)+b.TopRight))
	for _, l := range inner {
		lines = append(lines, frame.Render(b.Left)+l+frame.Render(b.Right))
	}
	lines = append(lines, frame.Render(b.BottomLeft+strings.Repeat(b.Bottom, w-2)+b.BottomRight))
	return strings.Join(lines, "\n")
}

// joinTiles separates board cells with a gutter. Without it the filled
// backgrounds of adjacent tiles run together into one solid bar instead of
// reading as distinct letters.
func joinTiles(cells []string) string {
	if len(cells) == 0 {
		return ""
	}
	// The flat case — every cell one row — is the board, the keyboard, the
	// legend and every preview: JoinHorizontal would split each cell into
	// lines, measure it, find the tallest block is one line, and concatenate
	// with one space, which is exactly strings.Join. The check keeps the tall
	// board's three-row tiles, where the blocks really are joined line by line,
	// on the library path.
	flat := true
	for _, c := range cells {
		if strings.IndexByte(c, '\n') >= 0 ||
			strings.IndexByte(c, '\t') >= 0 || strings.Contains(c, "\r\n") {
			flat = false
			break
		}
	}
	if flat {
		return strings.Join(cells, " ")
	}
	spaced := make([]string, 0, len(cells)*2-1)
	for i, c := range cells {
		if i > 0 {
			spaced = append(spaced, " ")
		}
		spaced = append(spaced, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, spaced...)
}

// legendSample is the letter the legend shows in each of the three scored
// states. Repeating one letter is what makes the row read as "the same letter,
// three answers" rather than as a word.
const legendSample = "A"

// renderLegend explains the tile colours. Themes are free to repaint correct,
// present and absent — several bundled ones drop the green/yellow convention
// entirely — so the board's own colours are not self-explanatory.
//
// It renders through the very styles renderScoredRow uses, so the legend cannot
// drift from the tiles it describes: a [style.tile.correct] override or a wider
// tile_width moves both together. There is no click target here, hence no
// hitMap: the legend is a label, not a control.
//
// st.legendText is the reader: it is a function of the theme alone, and the
// board asks for it (and its width) on every frame, so it is rendered once per
// style set.
func (s *styles) renderLegend() string {
	groups := []string{
		s.legendEntry(s.tileCorrect, "correct spot"),
		s.legendEntry(s.tilePresent, "wrong spot"),
		s.legendEntry(s.tileAbsent, "not in word"),
	}
	return strings.Join(groups, s.help.Render(s.glyph.Separator))
}

// legendText is the colour key for this style set, rendered on first use and
// kept. A theme change builds a new set and with it a new key, so nothing here
// can outlive the colours it explains.
func (s *styles) legendText() string {
	if !s.legend.ready {
		s.legend.text = s.renderLegend()
		s.legend.width = lipgloss.Width(s.legend.text)
		s.legend.ready = true
	}
	return s.legend.text
}

// legendWidth is how wide the key draws, without rendering it again. It is what
// gameScreen's ladder consults on every layout — twice a frame, once from the
// screen and once from the help bar — before the key itself is ever drawn.
func (s *styles) legendWidth() int {
	s.legendText()
	return s.legend.width
}

// legendEntry is one swatch and its label, sharing the board's tile gutter so
// the filled background does not run into the text.
func legendEntry(tile lipgloss.Style, label string) string {
	return st.legendEntry(tile, label)
}

func (s *styles) legendEntry(tile lipgloss.Style, label string) string {
	return joinTiles([]string{tile.Render(legendSample), s.muted.Render(label)})
}

// keyboardRows is the QWERTY layout used for the letter-state display.
var keyboardRows = []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}

// Enter and backspace flank the bottom row, as on a phone keyboard. They
// are what a mouse submits and deletes with, and they cost no vertical space:
// row 0 stays the widest at 59 cells, row 2 grows from 41 to 53. Their glyphs
// come from the theme, since not every font draws ⏎ and ⌫ well.

// renderKeyboard shows the best-known state of every letter, which is the
// player's main aid for narrowing down the answer. Every cap is clickable.
func renderKeyboard(states map[byte]game.Mark, h *hitMap, a *anims, now time.Time, gap int) string {
	rows := make([]string, len(keyboardRows))
	for i, letters := range keyboardRows {
		row := renderKeyboardRow(letters, states, h, a, now)
		if i == len(keyboardRows)-1 {
			row = joinTiles([]string{
				renderCommandKey(st.glyph.Enter, action{kind: actSubmit}, h, a, now),
				row,
				renderCommandKey(st.glyph.Delete, action{kind: actBackspace}, h, a, now),
			})
		}
		rows[i] = row
	}

	// Width of the widest row, used to centre the shorter ones beneath it. Row 0
	// is the widest the keyboard gets — 59 cells at the default metrics — and it
	// is the row just rendered, so it is measured rather than rendered a second
	// time to be measured. That is only sound because a marker is zero-width to
	// the measurement and the animation and hover cues are repaints: both are
	// pinned by tests (TestMarkersMeasureAsZeroWidth, and the board's own
	// settled-frame and layout checks).
	width := lipgloss.Width(rows[0])
	for i := range rows {
		rows[i] = centreLine(rows[i], width)
	}
	return stackSpaced(rows, gap)
}

// renderCommandKey draws one of the two non-letter caps.
func renderCommandKey(label string, act action, h *hitMap, a *anims, now time.Time) string {
	style := st.keyUnused
	if h.hovered(act) || a.pressed(now, 0, act.kind) {
		return h.mark(act, st.hover(style).Render(label))
	}
	return h.mark(act, st.cell(faceKeyUnused, label, flatTile, false))
}

// renderKeyboardRow draws one row of caps. A cap just struck wears the hover
// cue for a moment: the pulse and the pointer say the same thing — "this one" —
// so they are deliberately the same cue rather than a second one to learn.
func renderKeyboardRow(letters string, states map[byte]game.Mark, h *hitMap, a *anims, now time.Time) string {
	cells := make([]string, len(letters))
	for i := range letters {
		c := letters[i]
		letter := upperLetter(c)

		face := faceKeyUnused
		if mark, played := states[c]; played {
			switch mark {
			case game.Correct:
				face = faceKeyCorrect
			case game.Present:
				face = faceKeyPresent
			default:
				face = faceKeyAbsent
			}
		}

		typeIt := action{kind: actLetter, letter: c}
		if h.hovered(typeIt) || a.pressed(now, c, actLetter) {
			// The pulse fades, so it is built for this frame rather than cached.
			style, _ := st.faceStyle(face)
			cells[i] = h.mark(typeIt, st.hover(style).Render(letter))
			continue
		}
		cells[i] = h.mark(typeIt, st.cell(face, letter, flatTile, false))
	}
	// A one-space gutter keeps adjacent keycaps' backgrounds from merging into
	// a single bar, the same reason the board tiles are spaced.
	return joinTiles(cells)
}
