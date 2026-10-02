//go:build !js

package ui

import (
	"testing"

	"github.com/nxck2005/surmise/internal/brand"
)

// $NO_MOTION is the variable the README promises. The function used to read
// only the prefixed spelling, so the documented one never did anything.
func TestNoMotionEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name  string
		env   map[string]string
		wants bool
	}{
		{"unset", nil, false},
		{"NO_MOTION", map[string]string{"NO_MOTION": "1"}, true},
		{"prefixed", map[string]string{brand.Env("NO_MOTION"): "1"}, true},
		{"any value", map[string]string{"NO_MOTION": "0"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_MOTION", "")
			t.Setenv(brand.Env("NO_MOTION"), "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := prefersReducedMotion(); got != tc.wants {
				t.Errorf("prefersReducedMotion() = %v, want %v", got, tc.wants)
			}
		})
	}
}
