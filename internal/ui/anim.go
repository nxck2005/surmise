package ui

import "time"

// Animation here is deliberately small: it repaints styles and nothing else.
// Every effect below leaves the frame the same size it would have been, because
// click targets are measured from the rendered frame (see hit.go). A board that
// grew or shifted would move every target under a stationary pointer for the
// duration — which is why an invalid word flashes rather than shakes.
//
// The invariant every test leans on: an effect that has elapsed renders exactly
// the bytes the un-animated frame renders. Effects differ only inside their own
// window, so "the settled frame" and "the frame with motion off" are the same
// statement.
//
// Nothing here is persisted, and nothing here belongs to the domain: an
// animation is a property of the screen showing a game, not of the game.

// timeNow is the clock the screens read. It is a variable so a test can hold
// time still and step it deliberately: an animation derived from the real clock
// could only be tested by sleeping, and a sleeping test is a flaky test.
//
// Read only on the UI goroutine, in Update and View. Nothing inside a tea.Cmd
// reads it — the tick callbacks take the framework's own timestamp — so the
// race detector has nothing to find.
var timeNow = time.Now

// motion is how strongly the board animates. Off is a first-class choice rather
// than a zero-length duration, so the render path skips the whole business and
// produces exactly the frame it produced before any of this existed.
type motion int

const (
	motionOff motion = iota
	motionRestrained
	motionPronounced
)

// The saved and requested spellings, mirroring splashOn/splashOff.
const (
	motionOffName        = "off"
	motionRestrainedName = "restrained"
	motionPronouncedName = "pronounced"
)

// motionOrder is the cycle the settings row steps through. Off comes first
// because it is the answer someone hunting for this setting usually wants.
var motionOrder = []motion{motionOff, motionRestrained, motionPronounced}

func (m motion) label() string {
	switch m {
	case motionOff:
		return motionOffName
	case motionPronounced:
		return motionPronouncedName
	default:
		return motionRestrainedName
	}
}

// setting is what label() means in a settings file, so the two cannot drift.
func (m motion) setting() string { return m.label() }

// parseMotion reads a saved or requested value. An empty string is "nothing
// chosen", which is the fullest motion there is — the feedback is the feature,
// so it introduces itself rather than waiting to be found in a settings screen.
// That is also why Settings.Motion is a string: the default is not the zero
// value, so a bool could not tell "never chosen" from "chosen off".
//
// An unknown value is reported rather than refused, so a settings file written
// by a newer build costs a note on the error line and not a launch.
func parseMotion(s string) (motion, bool) {
	switch s {
	case "":
		return motionPronounced, true
	case motionOffName:
		return motionOff, true
	case motionRestrainedName:
		return motionRestrained, true
	case motionPronouncedName:
		return motionPronounced, true
	}
	return motionPronounced, false
}

// animKind is what a running effect is showing. The zero value is "nothing", so
// an unset slot is inert without needing a separate flag.
type animKind int

const (
	animNone animKind = iota
	// animReveal turns a submitted row from what was typed into what it scored,
	// left to right, so the result is read the way the word was written.
	animReveal
	// animWin is the same reveal with the border accent after it: the board
	// finishes saying what the guess scored before it says you won.
	animWin
	// animLoss reveals faster. A loss is the reveal nobody is waiting for, so
	// getting to the answer is kinder than dwelling on each tile.
	animLoss
	// animInvalid flashes the row that was refused — the only feedback for a
	// rejected guess that costs no line of text.
	animInvalid
	// animKeycap lights the cap just struck, which is what makes a clicked
	// keyboard feel connected to the board.
	animKeycap
	// animAnswer emphasises the word on the debrief after a loss. It is the
	// other half of the fast loss reveal: the board hurries so that this is
	// what the eye lands on.
	animAnswer
)

// frameInterval is how often a running effect repaints. Twenty-five frames a
// second is ample for something that only swaps styles, and the chain exists
// only between an event and the moment its last effect elapses: an idle board
// arms no timer at all, which is what keeps the browser's single thread free.
const frameInterval = 40 * time.Millisecond

// stagger is the gap between one tile turning and the next. A reveal's whole
// duration follows from it and the word's length, so a 6-letter board takes
// proportionally longer than a 4-letter one instead of racing to fit a fixed
// total.
func (m motion) stagger(kind animKind) time.Duration {
	if m == motionOff {
		return 0
	}
	if kind == animLoss {
		if m == motionPronounced {
			return 70 * time.Millisecond
		}
		return 40 * time.Millisecond
	}
	if m == motionPronounced {
		return 120 * time.Millisecond
	}
	return 60 * time.Millisecond
}

// hold is how long a whole-element effect lasts, for the kinds that do not
// march across tiles.
func (m motion) hold(kind animKind) time.Duration {
	if m == motionOff {
		return 0
	}
	pronounced := m == motionPronounced
	switch kind {
	case animInvalid:
		if pronounced {
			return 320 * time.Millisecond
		}
		return 160 * time.Millisecond
	case animKeycap:
		if pronounced {
			return 120 * time.Millisecond
		}
		return 80 * time.Millisecond
	case animWin:
		if pronounced {
			return 900 * time.Millisecond
		}
		return 480 * time.Millisecond
	case animAnswer:
		if pronounced {
			return 720 * time.Millisecond
		}
		return 360 * time.Millisecond
	}
	return 0
}

// flashes is how many times a blinking effect blinks. Pronounced repeats in the
// same colour; it is the repetition that reads as emphasis, not a new hue.
func (m motion) flashes() int {
	if m == motionPronounced {
		return 4
	}
	return 2
}

// anim is one effect. A zero startedAt means "not running", the same state
// gameScreen.sessionStart uses for "no session in progress": a state the screen
// genuinely has, rather than a value invented so tests can reach it.
type anim struct {
	kind      animKind
	startedAt time.Time

	// puzzle is the id of the game the effect belongs to. startNew swaps the
	// board's game in place without reopening the screen, so without this a
	// reveal could bleed onto the next puzzle.
	puzzle string

	// row is which board row is revealing and length how many tiles it has.
	row, length int

	// letter is the keycap being pulsed, as the lowercase byte the keyboard is
	// keyed by. Zero means one of the command caps, named by cmd.
	letter byte
	cmd    actionKind
}

// duration is how long this effect lasts in total.
func (a anim) duration(m motion) time.Duration {
	switch a.kind {
	case animReveal, animLoss:
		// One stagger per tile, and one more so the last tile is seen turned
		// rather than appearing only in the settled frame after the effect.
		return time.Duration(a.length+1) * m.stagger(a.kind)
	case animWin:
		return time.Duration(a.length+1)*m.stagger(a.kind) + m.hold(animWin)
	default:
		return m.hold(a.kind)
	}
}

// running reports whether the effect still has something to show at now. An
// effect whose time is up is indistinguishable from no effect at all, so a
// caller never has to clear one to get a settled frame.
func (a anim) running(now time.Time, m motion) bool {
	if a.kind == animNone || a.startedAt.IsZero() || m == motionOff {
		return false
	}
	return now.Sub(a.startedAt) < a.duration(m)
}

// phase is how far through the effect now is, from 0 to 1. Derived from wall
// time rather than counted in frames on purpose: a late or dropped frame then
// lands on the right phase instead of leaving the effect stuck half way.
func (a anim) phase(now time.Time, m motion) float64 {
	d := a.duration(m)
	if d <= 0 {
		return 1
	}
	switch elapsed := now.Sub(a.startedAt); {
	case elapsed <= 0:
		return 0
	case elapsed >= d:
		return 1
	default:
		return float64(elapsed) / float64(d)
	}
}

// revealed is how many tiles of the row have turned by now. Anything past it is
// still showing as typed, so the row never changes width as it resolves.
func (a anim) revealed(now time.Time, m motion) int {
	step := m.stagger(a.kind)
	if step <= 0 {
		return a.length
	}
	switch n := int(now.Sub(a.startedAt) / step); {
	case n < 0:
		return 0
	case n > a.length:
		return a.length
	default:
		return n
	}
}

// lit reports whether a blinking effect is in an "on" interval at now. The
// count is even and the final interval is off, so the effect settles on the
// un-animated frame rather than on its highlight.
func (a anim) lit(now time.Time, m motion) bool {
	if !a.running(now, m) {
		return false
	}
	slot := int(a.phase(now, m) * float64(m.flashes()))
	return slot%2 == 0
}

// accented reports whether a winning board is in its border-accent tail, which
// begins once every tile has turned.
func (a anim) accented(now time.Time, m motion) bool {
	return a.kind == animWin && a.running(now, m) && a.revealed(now, m) >= a.length
}

// anims is everything currently animating. Two slots, because a keycap pulse
// and a board effect are independent — striking a key during a reveal must not
// cancel either.
//
// Every method is nil-safe, exactly as hitMap's are. That is what lets a caller
// with no animation state at all — the layout tests today, a theme previewer
// later — render a settled board by passing nil.
type anims struct {
	motion motion

	// live reports that a frame is already in flight. A second effect starting
	// mid-animation joins that chain rather than arming another: two chains
	// would double the repaint rate, and neither would ever die.
	live bool

	board anim
	key   anim
}

// on reports whether animation is wanted at all.
func (a *anims) on() bool { return a != nil && a.motion != motionOff }

// beginBoard starts a board effect, replacing whatever was there. A superseded
// effect simply shows its settled state, which is the un-animated one.
func (a *anims) beginBoard(kind animKind, puzzle string, row, length int) {
	if !a.on() {
		return
	}
	a.board = anim{
		kind:      kind,
		startedAt: timeNow(),
		puzzle:    puzzle,
		row:       row,
		length:    length,
	}
}

// beginKey pulses a cap. letter is zero for the command caps, which name
// themselves through cmd.
func (a *anims) beginKey(letter byte, cmd actionKind) {
	if !a.on() {
		return
	}
	a.key = anim{kind: animKeycap, startedAt: timeNow(), letter: letter, cmd: cmd}
}

// busy reports whether anything is still running at now.
func (a *anims) busy(now time.Time) bool {
	if !a.on() {
		return false
	}
	return a.board.running(now, a.motion) || a.key.running(now, a.motion)
}

// clear drops every effect, settling the frame at once. The chain then dies on
// its next tick, because nothing is left to re-arm it.
func (a *anims) clear() {
	if a == nil {
		return
	}
	a.board, a.key = anim{}, anim{}
}

// reveal reports how many tiles of the given row are showing their marks, and
// whether that row is mid-reveal at all. A row belonging to another puzzle, or
// to another attempt, is fully revealed like every other settled row.
func (a *anims) reveal(now time.Time, puzzle string, row int) (shown int, revealing bool) {
	if !a.on() || a.board.puzzle != puzzle || a.board.row != row {
		return 0, false
	}
	switch a.board.kind {
	case animReveal, animWin, animLoss:
	default:
		return 0, false
	}
	if !a.board.running(now, a.motion) {
		return 0, false
	}
	return a.board.revealed(now, a.motion), true
}

// rejected reports whether the typed row should be flashing its refusal.
func (a *anims) rejected(now time.Time) bool {
	return a.on() && a.board.kind == animInvalid && a.board.lit(now, a.motion)
}

// accented reports whether the panel border is wearing the win accent.
func (a *anims) accented(now time.Time) bool {
	return a.on() && a.board.accented(now, a.motion)
}

// beginAnswer emphasises the answer on the debrief after a loss.
func (a *anims) beginAnswer() {
	if !a.on() {
		return
	}
	a.board = anim{kind: animAnswer, startedAt: timeNow()}
}

// answering reports whether the losing answer is in a highlighted interval.
// Like every blink it ends off, so the settled debrief is the plain one.
func (a *anims) answering(now time.Time) bool {
	return a.on() && a.board.kind == animAnswer && a.board.lit(now, a.motion)
}

// pressed reports whether the given cap is mid-pulse.
func (a *anims) pressed(now time.Time, letter byte, cmd actionKind) bool {
	if !a.on() || !a.key.running(now, a.motion) {
		return false
	}
	return a.key.letter == letter && a.key.cmd == cmd
}
