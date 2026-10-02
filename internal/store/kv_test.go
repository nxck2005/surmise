package store

import "testing"

// Usage counts what a browser counts against the quota: UTF-16 code units of
// every key and value, this app's or not.
func TestKVUsageCountsEveryKeyInUTF16(t *testing.T) {
	kv := NewMemoryKV()
	s := NewKV(kv)
	if got := s.Usage(); got != 0 {
		t.Fatalf("empty Usage() = %d, want 0", got)
	}

	// "é" is one code unit and two UTF-8 bytes; "😀" is two code units and
	// four bytes. A foreign key counts like any other.
	if err := kv.Set("other", "é😀"); err != nil {
		t.Fatal(err)
	}
	if got, want := s.Usage(), len("other")+1+2; got != want {
		t.Fatalf("Usage() = %d, want %d", got, want)
	}

	g := newGame(t, 5)
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	v, _ := kv.Get(kvPuzzleKey(g.ID))
	if got, want := s.Usage(), len("other")+3+len(kvPuzzleKey(g.ID))+len(v); got != want {
		t.Errorf("Usage() after a save = %d, want %d", got, want)
	}
}
