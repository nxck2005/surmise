package stats

import "time"

const (
	maxDuration = time.Duration(1<<63 - 1)
	minDuration = -maxDuration - 1
)

// addDurationSaturating adds durations without allowing the profile's lifetime
// figures to wrap when a hand-edited or imported history reaches the limit.
func addDurationSaturating(a, b time.Duration) time.Duration {
	switch {
	case b > 0 && a > maxDuration-b:
		return maxDuration
	case b < 0 && a < minDuration-b:
		return minDuration
	default:
		return a + b
	}
}
