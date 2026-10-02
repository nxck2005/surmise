//go:build !js

package ui

import (
	"os"

	"github.com/nxck2005/surmise/internal/brand"
)

// prefersReducedMotion reports whether the environment has asked for less
// animation. There is no terminal capability for this, so an environment
// variable is the whole answer.
//
// $NO_MOTION is the one the README documents: a convention like NO_COLOR, so a
// player sets it once for every program that honours it, not once per app.
// The prefixed spelling from brand.Env is read as well, because it is the one
// this function used to read on its own — which meant the documented variable
// did nothing — and an install that found and set it should keep working.
//
// Any non-empty value counts. Someone exporting NO_MOTION=0 is asking for the
// variable to do something, and the something it does is this.
func prefersReducedMotion() bool {
	return os.Getenv("NO_MOTION") != "" || os.Getenv(brand.Env("NO_MOTION")) != ""
}
