//go:build js && wasm

package ui

import "syscall/js"

// prefersReducedMotion asks the page whether the reader has turned animation
// down at the operating-system level. In a browser that preference already
// exists and is already set, so honouring it is the difference between an
// accessible default and one that has to be found in a settings screen.
//
// Nothing here may assume a browser. syscall/js panics on a Get against an
// undefined value, and the wasm binary is smoke-tested under Node, which has
// neither window nor matchMedia — so every step is guarded and the missing case
// falls through to "no preference", exactly as loadConfig does for the query
// string.
func prefersReducedMotion() bool {
	mm := js.Global().Get("matchMedia")
	if !mm.Truthy() {
		return false
	}
	q := js.Global().Call("matchMedia", "(prefers-reduced-motion: reduce)")
	if !q.Truthy() {
		return false
	}
	return q.Get("matches").Truthy()
}
