package chess

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

const (
	fenFieldCount = 6
	noneField     = "-"
)

func ParseFEN(fen string) (Position, error) {
	fields := strings.Fields(fen)
	if len(fields) != fenFieldCount {
		return Position{}, fmt.Errorf("fen: want %d fields, got %d", fenFieldCount, len(fields))
	}
	var p Position
	if err := parsePlacement(&p, fields[0]); err != nil {
		return Position{}, err
	}
	var err error
	if p.SideToMove, err = parseSideToMove(fields[1]); err != nil {
		return Position{}, err
	}
	if p.Castling, err = parseCastling(fields[2]); err != nil {
		return Position{}, err
	}
	if p.EnPassant, err = parseEnPassant(fields[3]); err != nil {
		return Position{}, err
	}
	if p.HalfmoveClock, err = parseCounter(fields[4], "halfmove clock", 0); err != nil {
		return Position{}, err
	}
	if p.FullmoveNumber, err = parseCounter(fields[5], "fullmove number", 1); err != nil {
		return Position{}, err
	}
	return p, nil
}

func parsePlacement(p *Position, placement string) error {
	ranks := strings.Split(placement, "/")
	if len(ranks) != fileCount {
		return fmt.Errorf("fen: want %d ranks, got %d", fileCount, len(ranks))
	}
	for i, rank := range ranks {
		if err := parseRank(p, fileCount-1-i, rank); err != nil {
			return err
		}
	}
	return nil
}

func parseRank(p *Position, rank int, text string) error {
	file := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= '1' && c <= '8' {
			file += int(c - '0')
			continue
		}
		piece, ok := pieceFromLetter(c)
		if !ok {
			return fmt.Errorf("fen: invalid piece %q", c)
		}
		if file >= fileCount {
			return fmt.Errorf("fen: rank %d has more than %d files", rank+1, fileCount)
		}
		p.Board[NewSquare(file, rank)] = piece
		file++
	}
	if file != fileCount {
		return fmt.Errorf("fen: rank %d has %d files, want %d", rank+1, file, fileCount)
	}
	return nil
}

func parseSideToMove(s string) (Color, error) {
	switch s {
	case "w":
		return White, nil
	case "b":
		return Black, nil
	}
	return White, fmt.Errorf("fen: invalid side to move %q", s)
}

func parseCastling(s string) (CastlingRights, error) {
	if s == noneField {
		return 0, nil
	}
	var rights CastlingRights
	for i := range len(s) {
		idx := slices.IndexFunc(castlings[:], func(c castling) bool { return c.letter == s[i] })
		if idx < 0 || rights&castlings[idx].right != 0 {
			return 0, fmt.Errorf("fen: invalid castling %q", s)
		}
		rights |= castlings[idx].right
	}
	return rights, nil
}

func parseEnPassant(s string) (Square, error) {
	if s == noneField {
		return NoSquare, nil
	}
	sq, err := ParseSquare(s)
	if err != nil {
		return NoSquare, fmt.Errorf("fen: en passant: %w", err)
	}
	if sq.Rank() != 2 && sq.Rank() != 5 {
		return NoSquare, fmt.Errorf("fen: en passant square %q not on rank 3 or 6", s)
	}
	return sq, nil
}

func parseCounter(s, name string, atLeast int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < atLeast {
		return 0, fmt.Errorf("fen: invalid %s %q", name, s)
	}
	return n, nil
}

func (p Position) FEN() string {
	var b strings.Builder
	for rank := fileCount - 1; rank >= 0; rank-- {
		writeRank(&b, p, rank)
		if rank > 0 {
			b.WriteByte('/')
		}
	}
	side := "w"
	if p.SideToMove == Black {
		side = "b"
	}
	fmt.Fprintf(&b, " %s %s %s %d %d", side, p.castlingField(), enPassantField(p.EnPassant), p.HalfmoveClock, p.FullmoveNumber)
	return b.String()
}

func writeRank(b *strings.Builder, p Position, rank int) {
	empty := 0
	flush := func() {
		if empty > 0 {
			b.WriteByte(byte('0' + empty))
			empty = 0
		}
	}
	for file := range fileCount {
		piece := p.Board[NewSquare(file, rank)]
		if piece.IsEmpty() {
			empty++
			continue
		}
		flush()
		b.WriteByte(piece.letter())
	}
	flush()
}

func (p Position) castlingField() string {
	var field []byte
	for _, c := range castlings {
		if p.Castling&c.right != 0 {
			field = append(field, c.letter)
		}
	}
	if len(field) == 0 {
		return noneField
	}
	return string(field)
}

func enPassantField(sq Square) string {
	if sq == NoSquare {
		return noneField
	}
	return sq.String()
}
