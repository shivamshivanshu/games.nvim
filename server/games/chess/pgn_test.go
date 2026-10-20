package chess

import (
	"strings"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// lateEveningNewYork is 2026-10-21 in New York but already 2026-10-22 in UTC.
var lateEveningNewYork = time.Date(2026, 10, 22, 2, 30, 0, 0, time.UTC)

func exportContext(result game.Result) game.ExportContext {
	return game.ExportContext{
		Players:   []game.Player{{User: alice, Seat: SeatWhite}, {User: bob, Seat: SeatBlack}},
		StartedAt: lateEveningNewYork.In(time.FixedZone("EDT", -4*60*60)),
		Result:    result,
	}
}

func TestFoolsMateIsExactPGN(t *testing.T) {
	st, _ := newGame(t, ColorWhite)
	st = playSANs(t, st, "f3 e5 g4 Qh4#")
	result, _ := st.Outcome()
	want := strings.Join([]string{
		`[Event "games.nvim"]`,
		`[Site "?"]`,
		`[Date "2026.10.22"]`,
		`[Round "-"]`,
		`[White "alice"]`,
		`[Black "bob"]`,
		`[Result "0-1"]`,
		"",
		"1. f3 e5 2. g4 Qh4# 0-1",
		"",
	}, "\n")
	if got := string(st.Exports(exportContext(result))[PGNFile]); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPGNResultFollowsTheWinnerSeatOrTheKind(t *testing.T) {
	tests := []struct {
		result game.Result
		want   pgnResult
	}{
		{game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatWhite}, Reason: game.ReasonResign}, pgnWhiteWins},
		{game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: game.ReasonTimeout}, pgnBlackWins},
		{game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}, pgnDraw},
		{game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}, pgnUndecided},
	}
	start := mustFEN(t, StartFEN)
	for _, tc := range tests {
		lines := strings.Split(strings.TrimSpace(buildPGN(start, []string{"e4"}, exportContext(tc.result))), "\n")
		if lines[6] != `[Result "`+string(tc.want)+`"]` || !strings.HasSuffix(lines[len(lines)-1], " "+string(tc.want)) {
			t.Errorf("%+v: got %q and %q, want %s", tc.result, lines[6], lines[len(lines)-1], tc.want)
		}
	}
}

func TestPGNNumbersMovesFromTheStartPosition(t *testing.T) {
	tests := []struct{ name, fen, sans, want string }{
		{"black to move", "4k3/8/8/8/8/8/8/R3K3 b - - 0 12", "Kd7 Ra7+ Ke6", "12... Kd7 13. Ra7+ Ke6 *"},
		{"white to move", "4k3/8/8/8/8/8/8/R3K3 w - - 0 12", "Kd2 Kd7 Ra7+", "12. Kd2 Kd7 13. Ra7+ *"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := buildPGN(mustFEN(t, tc.fen), strings.Fields(tc.sans), exportContext(game.Result{Kind: game.ResultAborted}))
			if !strings.Contains(text, `[SetUp "1"]`+"\n"+`[FEN "`+tc.fen+`"]`) {
				t.Errorf("no FEN tags in\n%s", text)
			}
			if got := lastLine(text); got != tc.want {
				t.Errorf("movetext %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPGNWrapsLongMovetext(t *testing.T) {
	sans := make([]string, 60)
	for i := range sans {
		sans[i] = map[bool]string{true: "Nf3", false: "Nf6"}[i%2 == 0]
	}
	text := buildPGN(mustFEN(t, StartFEN), sans, exportContext(game.Result{Kind: game.ResultDraw}))
	movetext := strings.Split(strings.TrimSpace(text), "\n")[8:]
	if len(movetext) < 2 {
		t.Fatalf("movetext not wrapped: %q", movetext)
	}
	for _, line := range movetext {
		if len(line) > pgnLineWidth {
			t.Errorf("line of %d bytes: %q", len(line), line)
		}
	}
	const moves, numbers, result = 60, 30, 1
	if got := len(strings.Fields(strings.Join(movetext, " "))); got != moves+numbers+result {
		t.Errorf("%d tokens, want %d", got, moves+numbers+result)
	}
}

func TestPGNEscapesTagValues(t *testing.T) {
	ctx := exportContext(game.Result{Kind: game.ResultAborted})
	ctx.Players[0].User = `a"b\c`
	ctx.Players[1].User = "b\no]\x00b\u202e"
	text := buildPGN(mustFEN(t, StartFEN), nil, ctx)
	if !strings.Contains(text, `[White "a\"b\\c"]`) || !strings.Contains(text, `[Black "bo]b"]`) {
		t.Errorf("unescaped tag in\n%s", text)
	}
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return lines[len(lines)-1]
}
