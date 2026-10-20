package checkers

import (
	"strconv"
	"strings"
)

// Move is one complete turn: the origin, every landing square, and the pieces jumped on the way.
type Move struct {
	Path     []Square
	Captured []Square
}

// From is the origin square.
func (m Move) From() Square { return m.Path[0] }

// To is the final landing square.
func (m Move) To() Square { return m.Path[len(m.Path)-1] }

// IsCapture reports whether the move jumps any piece.
func (m Move) IsCapture() bool { return len(m.Captured) > 0 }

const (
	stepSeparator    = "-"
	captureSeparator = "x"
)

// Text is the move in standard notation: 11-15, 15x24 or 15x24x31.
func (m Move) Text() string {
	squares := make([]string, len(m.Path))
	for i, sq := range m.Path {
		squares[i] = strconv.Itoa(int(sq))
	}
	separator := stepSeparator
	if m.IsCapture() {
		separator = captureSeparator
	}
	return strings.Join(squares, separator)
}
