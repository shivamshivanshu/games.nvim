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
	repetitionKeyFields  = 4
)

type RepetitionKey string

type Ending struct {
	Reason game.ResultReason
	Kind   game.ResultKind
	Winner *Color
}

func (p Position) RepetitionKey() RepetitionKey {
	return p.repetitionKey(p.LegalMoves())
}

func (p Position) repetitionKey(legal []Move) RepetitionKey {
	key := p
	if !p.hasLegalEnPassantCapture(legal) {
		key.EnPassant = NoSquare
	}
	return RepetitionKey(strings.Join(strings.Fields(key.FEN())[:repetitionKeyFields], " "))
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
	switch {
	case repetitions(p.RepetitionKey(), history) >= threefoldCount:
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
	var minors []Square
	for sq := range Square(boardSize) {
		switch p.Board[sq].Kind {
		case NoPieceKind, King:
		case Knight, Bishop:
			minors = append(minors, sq)
		default:
			return false
		}
	}
	if len(minors) <= 1 {
		return true
	}
	onlyBishops := !slices.ContainsFunc(minors, func(sq Square) bool { return p.Board[sq].Kind == Knight })
	sameColour := !slices.ContainsFunc(minors, func(sq Square) bool { return sq.isLight() != minors[0].isLight() })
	return onlyBishops && sameColour
}

func (s Square) isLight() bool { return (s.File()+s.Rank())%2 == 1 }
