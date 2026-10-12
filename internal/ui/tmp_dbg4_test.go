package ui

import "testing"

func TestDbgPendingTwo(t *testing.T) {
	m, _, conn := beginRoundWith(t, 2)
	t.Log("pending:", m.room.pending, "typing:", m.game.typing, "frame count:", conn.count())
	for i, s := range conn.sent {
		t.Log(i, s.T())
	}
}
