package ui

import (
	"fmt"
	"time"
)

// formatDuration renders play time compactly: "42s", "3:07", "1:02:33".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "–"
	}
	d = d.Round(time.Second)

	h := int(d / time.Hour)
	m := int(d % time.Hour / time.Minute)
	s := int(d % time.Minute / time.Second)

	switch {
	case h > 0:
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	case m > 0:
		return fmt.Sprintf("%d:%02d", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func formatPercent(f float64) string {
	return fmt.Sprintf("%.0f%%", f*100)
}

// formatFloat drops the decimal for whole numbers, so "4" rather than "4.0".
func formatFloat(f float64) string {
	if f == 0 {
		return "–"
	}
	if f == float64(int(f)) {
		return fmt.Sprint(int(f))
	}
	return fmt.Sprintf("%.1f", f)
}
