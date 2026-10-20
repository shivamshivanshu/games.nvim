package chess

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// PGNFile is the name of the export holding the game in PGN.
const PGNFile = "game.pgn"

const (
	pgnEvent     = "games.nvim"
	pgnUnknown   = "?"
	pgnRound     = "-"
	pgnDate      = "2006.01.02"
	pgnLineWidth = 79
)

type pgnResult string

const (
	pgnWhiteWins pgnResult = "1-0"
	pgnBlackWins pgnResult = "0-1"
	pgnDraw      pgnResult = "1/2-1/2"
	pgnUndecided pgnResult = "*"
)

func (s state) Exports(ctx game.ExportContext) map[string][]byte {
	return map[string][]byte{PGNFile: []byte(buildPGN(s.start, s.san, ctx))}
}

// buildPGN is the game in PGN export format: the Seven Tag Roster, with the
// date the game started on in UTC, the FEN tags when it did not start from the
// standard position, then the movetext.
func buildPGN(start Position, sans []string, ctx game.ExportContext) string {
	result := pgnResultOf(ctx.Result)
	lines := []string{
		pgnTag("Event", pgnEvent),
		pgnTag("Site", pgnUnknown),
		pgnTag("Date", ctx.StartedAt.UTC().Format(pgnDate)),
		pgnTag("Round", pgnRound),
		pgnTag("White", userAt(ctx.Players, SeatWhite)),
		pgnTag("Black", userAt(ctx.Players, SeatBlack)),
		pgnTag("Result", string(result)),
	}
	if fen := start.FEN(); fen != StartFEN {
		lines = append(lines, pgnTag("SetUp", "1"), pgnTag("FEN", fen))
	}
	lines = append(lines, "")
	lines = append(lines, wrapTokens(movetextTokens(start, sans, result), pgnLineWidth)...)
	return strings.Join(lines, "\n") + "\n"
}

func pgnResultOf(r game.Result) pgnResult {
	switch {
	case r.Kind == game.ResultDraw:
		return pgnDraw
	case r.Kind != game.ResultDecisive:
		return pgnUndecided
	case slices.Contains(r.Winners, SeatWhite):
		return pgnWhiteWins
	case slices.Contains(r.Winners, SeatBlack):
		return pgnBlackWins
	}
	return pgnUndecided
}

var pgnEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func pgnTag(name, value string) string {
	return "[" + name + ` "` + pgnEscaper.Replace(value) + `"]`
}

func userAt(players []game.Player, seat game.Seat) string {
	if i := slices.IndexFunc(players, func(p game.Player) bool { return p.Seat == seat }); i >= 0 {
		return string(players[i].User)
	}
	return pgnUnknown
}

// movetextTokens numbers each white move, and a first move by black "N...".
func movetextTokens(start Position, sans []string, result pgnResult) []string {
	tokens := make([]string, 0, len(sans)*3/2+2)
	number, side := start.FullmoveNumber, start.SideToMove
	for i, san := range sans {
		switch {
		case side == White:
			tokens = append(tokens, fmt.Sprintf("%d.", number))
		case i == 0:
			tokens = append(tokens, fmt.Sprintf("%d...", number))
		}
		tokens = append(tokens, san)
		if side == Black {
			number++
		}
		side = side.Opponent()
	}
	return append(tokens, string(result))
}

func wrapTokens(tokens []string, width int) []string {
	lines := []string{tokens[0]}
	for _, token := range tokens[1:] {
		last := &lines[len(lines)-1]
		if len(*last)+1+len(token) > width {
			lines = append(lines, token)
			continue
		}
		*last += " " + token
	}
	return lines
}
