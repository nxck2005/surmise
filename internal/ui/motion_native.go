//go:build !js

package ui

import (
	"os"

	"github.com/nxck2005/surmise/internal/brand"
)

// prefersReducedMotion reports whether the environment has asked for less
// animation. There is no terminal capability for this, so the environment
// variable is the whole answer — the same convention NO_COLOR established, and
// read through brand.Env like every other option this app takes from the shell.
//
// Any non-empty value counts. Someone exporting NO_MOTION=0 is asking for the
// variable to do something, and the something it does is this.
func prefersReducedMotion() bool { return os.Getenv(brand.Env("NO_MOTION")) != "" }
