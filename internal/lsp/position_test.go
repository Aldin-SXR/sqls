package lsp

import "testing"

func TestPositionOffsets(t *testing.T) {
	text := "a😀b\r\nŁódź\n"
	for _, tc := range []struct {
		pos    Position
		offset int
	}{
		{Position{0, 0}, 0}, {Position{0, 1}, 1}, {Position{0, 3}, 5},
		{Position{0, 4}, 6}, {Position{1, 0}, 8}, {Position{1, 4}, 15},
		{Position{2, 0}, len(text)},
	} {
		offset, err := ByteOffset(text, tc.pos)
		if err != nil || offset != tc.offset {
			t.Errorf("ByteOffset(%+v) = %d, %v; want %d", tc.pos, offset, err, tc.offset)
		}
		if got := PositionAt(text, tc.offset); got != tc.pos {
			t.Errorf("PositionAt(%d) = %+v; want %+v", tc.offset, got, tc.pos)
		}
	}
	if got, err := ByteOffset(text, Position{0, 99}); err != nil || got != 6 {
		t.Errorf("clamped offset = %d, %v", got, err)
	}
	for _, pos := range []Position{{-1, 0}, {0, -1}, {3, 0}} {
		if _, err := ByteOffset(text, pos); err == nil {
			t.Errorf("accepted invalid position %+v", pos)
		}
	}
}
