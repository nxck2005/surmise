package words

import "testing"

func TestOffensive(t *testing.T) {
	for name, want := range map[string]bool{
		"abbo":    true,
		"Abbo_99": true,
		"xabbo":   false,
		"":        false,
		"nick":    false,
	} {
		if got := Offensive(name); got != want {
			t.Errorf("Offensive(%q) = %v, want %v", name, got, want)
		}
	}
}
