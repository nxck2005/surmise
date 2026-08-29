package challenge

import "testing"

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "ABCD-EFGH-JKMP-QRST", "0000000000000000", "IIII-IIII-IIII-IIII"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		c, err := Parse(text)
		if err != nil {
			return
		}
		again, err := Parse(c.String())
		if err != nil || again.String() != c.String() || again.ID() != c.ID() {
			t.Fatalf("valid parse did not round trip: %q, %v", c.String(), err)
		}
	})
}
