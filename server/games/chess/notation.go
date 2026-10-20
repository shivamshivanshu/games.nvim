package chess

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	kingsideCastleSAN  = "O-O"
	queensideCastleSAN = "O-O-O"
	checkSuffixes      = "+#"
	annotationSuffixes = "+#!?"
)

var uciPattern = regexp.MustCompile(`^[a-h][1-8][a-h][1-8][qrbn]?$`)

func (p Position) SAN(m Move) string {
	return p.san(m, p.LegalMoves())
}

func (p Position) san(m Move, legal []Move) string {
	return p.sanWithoutCheckSuffix(m, legal) + p.checkSuffix(m)
}

func (p Position) sanWithoutCheckSuffix(m Move, legal []Move) string {
	moving := p.Board[m.From]
	if c, ok := castlingByKingMove(moving, m); ok {
		if c.kingTo.File() > c.kingFrom.File() {
			return kingsideCastleSAN
		}
		return queensideCastleSAN
	}
	var b strings.Builder
	isCapture := !p.Board[m.To].IsEmpty() || (moving.Kind == Pawn && m.From.File() != m.To.File())
	if moving.Kind == Pawn {
		if isCapture {
			b.WriteByte(byte('a' + m.From.File()))
		}
	} else {
		b.WriteByte(pieceLetters[moving.Kind])
		b.WriteString(p.disambiguation(m, legal))
	}
	if isCapture {
		b.WriteByte('x')
	}
	b.WriteString(m.To.String())
	if m.Promotion != NoPieceKind {
		b.WriteByte('=')
		b.WriteByte(pieceLetters[m.Promotion])
	}
	return b.String()
}

func (p Position) disambiguation(m Move, legal []Move) string {
	var sameFile, sameRank, ambiguous bool
	for _, other := range legal {
		if other.To != m.To || other.From == m.From || p.Board[other.From] != p.Board[m.From] {
			continue
		}
		ambiguous = true
		sameFile = sameFile || other.From.File() == m.From.File()
		sameRank = sameRank || other.From.Rank() == m.From.Rank()
	}
	from := m.From.String()
	switch {
	case !ambiguous:
		return ""
	case !sameFile:
		return from[:1]
	case !sameRank:
		return from[1:]
	}
	return from
}

func (p Position) checkSuffix(m Move) string {
	next := p.Play(m)
	if !next.InCheck() {
		return ""
	}
	if len(next.LegalMoves()) == 0 {
		return "#"
	}
	return "+"
}

func (p Position) ParseMove(input string) (Move, error) {
	return p.parseMove(input, p.LegalMoves())
}

func (p Position) parseMove(input string, legal []Move) (Move, error) {
	text := normalizeMoveText(input)
	if lowered := strings.ToLower(text); uciPattern.MatchString(lowered) {
		if i := slices.IndexFunc(legal, func(m Move) bool { return m.UCI() == lowered }); i >= 0 {
			return legal[i], nil
		}
	}
	for _, m := range legal {
		san := p.sanWithoutCheckSuffix(m, legal)
		if text == san || text == strings.Replace(san, "=", "", 1) {
			return m, nil
		}
	}
	return Move{}, fmt.Errorf("illegal move: %s", input)
}

func normalizeMoveText(input string) string {
	text := strings.TrimRight(strings.TrimSpace(input), annotationSuffixes)
	text = strings.Replace(text, "0-0-0", queensideCastleSAN, 1)
	return strings.Replace(text, "0-0", kingsideCastleSAN, 1)
}
