package chess

import (
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	threefoldCount       = 3
	fivefoldCount        = 5
	fiftyMoveHalfmoves   = 100
	seventyFiveHalfmoves = 150
)

type RepetitionKey string

type Ending struct {
	Reason game.ResultReason
	Kind   game.ResultKind
	Winner *Color
}

func (p Position) RepetitionKey() RepetitionKey {
	var legal []Move
	if p.EnPassant != NoSquare {
		legal = p.LegalMoves()
	}
	return p.repetitionKey(legal)
}

// repetitionKey is the FEN without its move counters, keeping the en passant
// square only when a legal capture lands there.
func (p Position) repetitionKey(legal []Move) RepetitionKey {
	enPassant := p.EnPassant
	if !p.hasLegalEnPassantCapture(legal) {
		enPassant = NoSquare
	}
	var b strings.Builder
	p.writeKeyFields(&b, enPassant)
	return RepetitionKey(b.String())
}

func (p Position) hasLegalEnPassantCapture(legal []Move) bool {
	if p.EnPassant == NoSquare {
		return false
	}
	return slices.ContainsFunc(legal, func(m Move) bool {
		return m.To == p.EnPassant && p.Board[m.From].Kind == Pawn
	})
}

func DetectEnding(p Position, legal []Move, key RepetitionKey, history []RepetitionKey) (Ending, bool) {
	switch {
	case len(legal) == 0 && p.InCheck():
		winner := p.SideToMove.Opponent()
		return Ending{Reason: ReasonCheckmate, Kind: game.ResultDecisive, Winner: &winner}, true
	case len(legal) == 0:
		return Ending{Reason: ReasonStalemate, Kind: game.ResultDraw}, true
	case p.hasInsufficientMaterial():
		return Ending{Reason: ReasonInsufficientMaterial, Kind: game.ResultDraw}, true
	case repetitions(key, history) >= fivefoldCount:
		return Ending{Reason: ReasonFivefold, Kind: game.ResultDraw}, true
	case p.HalfmoveClock >= seventyFiveHalfmoves:
		return Ending{Reason: ReasonSeventyFive, Kind: game.ResultDraw}, true
	}
	return Ending{}, false
}

func ClaimableDraw(p Position, history []RepetitionKey) (game.ResultReason, bool) {
	return p.claimableDraw(p.RepetitionKey(), history)
}

// claimableDraw takes p's own key, which a caller holding it need not recompute.
func (p Position) claimableDraw(key RepetitionKey, history []RepetitionKey) (game.ResultReason, bool) {
	switch {
	case repetitions(key, history) >= threefoldCount:
		return ReasonThreefold, true
	case p.HalfmoveClock >= fiftyMoveHalfmoves:
		return ReasonFifty, true
	}
	return "", false
}

func repetitions(key RepetitionKey, history []RepetitionKey) int {
	count := 0
	for _, k := range history {
		if k == key {
			count++
		}
	}
	return count
}

func (p Position) hasInsufficientMaterial() bool {
	minors, knight, mixedColours, firstLight := 0, false, false, false
	for sq := range Square(boardSize) {
		switch p.Board[sq].Kind {
		case NoPieceKind, King:
			continue
		case Knight:
			knight = true
		case Bishop:
		default:
			return false
		}
		if minors == 0 {
			firstLight = sq.isLight()
		}
		mixedColours = mixedColours || sq.isLight() != firstLight
		minors++
	}
	return minors <= 1 || !knight && !mixedColours
}

func (s Square) isLight() bool { return (s.File()+s.Rank())%2 == 1 }
