package lsp

import (
	"fmt"
	"unicode/utf8"
)

// PositionAt converts a byte offset into an LSP UTF-16 position.
func PositionAt(text string, offset int) Position {
	pos := Position{}
	for i := 0; i < offset; {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch r {
		case '\r':
			if i < offset && text[i] == '\n' {
				i++
			}
			pos.Line++
			pos.Character = 0
		case '\n':
			pos.Line++
			pos.Character = 0
		default:
			pos.Character++
			if r > 0xffff {
				pos.Character++
			}
		}
	}
	return pos
}

// ByteOffset converts an LSP position to a byte offset. Characters beyond the
// end of a line are clamped, as required by the protocol.
func ByteOffset(text string, pos Position) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("negative position")
	}
	line, col := 0, 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if line == pos.Line && (col >= pos.Character || r == '\r' || r == '\n') {
			return i, nil
		}
		i += size
		if r == '\r' || r == '\n' {
			if r == '\r' && i < len(text) && text[i] == '\n' {
				i++
			}
			line++
			col = 0
		} else {
			col++
			if r > 0xffff {
				col++
			}
		}
	}
	if line == pos.Line {
		return len(text), nil
	}
	return 0, fmt.Errorf("line %d is outside document", pos.Line)
}
