package chesspuzzle

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	mode     Mode
	solver   chess.Color
	line     line
	ply      int
	position chess.Position
	history  []string
	last     *chess.MoveSquares
	misses   int
	result   game.Result
	finished bool
}

func newState(mode Mode, solver chess.Color, l line) state {
	return state{mode: mode, solver: solver, line: l, position: l.start}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	switch action.Kind {
	case ActionMove:
		return s.applyMove(action.Text)
	case ActionReveal:
		return s.applyReveal()
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyMove(text string) (game.State[Action, View], []game.Event, error) {
	move, err := s.position.ParseMove(text)
	if err != nil {
		return nil, nil, game.IllegalActionError{Msg: err.Error()}
	}
	if move == s.line.moves[s.ply] {
		next, replies := s.playLine().replyUntilSolverMoves()
		return next, append([]game.Event{MovedEvent{By: game.SeatSolo, SAN: s.line.san[s.ply]}}, replies...), nil
	}
	san := s.position.SAN(move)
	if s.mode == ModePuzzle && givesMate(s.position, move) {
		return s.play(move, san).endLine(won(ReasonSolved)), []game.Event{MovedEvent{By: game.SeatSolo, SAN: san}}, nil
	}
	s.misses++
	missed := []game.Event{MissedEvent{By: game.SeatSolo, SAN: san}}
	if s.mode == ModePuzzle {
		return s.finish(lost(ReasonWrongMove)), missed, nil
	}
	return s, missed, nil
}

func (s state) applyReveal() (game.State[Action, View], []game.Event, error) {
	if s.mode != ModeStudy {
		return nil, nil, game.IllegalActionError{Msg: "reveal is for study mode only"}
	}
	s.misses++
	next, replies := s.playLine().replyUntilSolverMoves()
	return next, append([]game.Event{RevealedEvent{By: game.SeatSolo, SAN: s.line.san[s.ply]}}, replies...), nil
}

// replyUntilSolverMoves plays the line for the other side until the solver is
// to move; reaching the end of the line finishes the game as won.
func (s state) replyUntilSolverMoves() (state, []game.Event) {
	var replies []game.Event
	for s.ply < len(s.line.moves) && s.line.sideAt(s.ply) != s.solver {
		replies = append(replies, RepliedEvent{By: game.SeatSolo, SAN: s.line.san[s.ply]})
		s = s.playLine()
	}
	if s.ply == len(s.line.moves) {
		s = s.finish(won(s.mode.endReason()))
	}
	return s, replies
}

func (s state) playLine() state {
	next := s.play(s.line.moves[s.ply], s.line.san[s.ply])
	next.ply++
	return next
}

func (s state) play(move chess.Move, san string) state {
	s.position = s.position.Play(move)
	s.history = append(slices.Clip(s.history), san)
	s.last = &chess.MoveSquares{From: move.From.String(), To: move.To.String()}
	return s
}

// endLine finishes the game off the line, so none of its moves remain.
func (s state) endLine(result game.Result) state {
	s.ply = len(s.line.moves)
	return s.finish(result)
}

func (s state) finish(result game.Result) state {
	s.result, s.finished = result, true
	return s
}

func givesMate(p chess.Position, m chess.Move) bool {
	next := p.Play(m)
	return next.InCheck() && len(next.LegalMoves()) == 0
}

func (m Mode) endReason() game.ResultReason {
	if m == ModeStudy {
		return ReasonCompleted
	}
	return ReasonSolved
}

func won(reason game.ResultReason) game.Result {
	return game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{game.SeatSolo}, Reason: reason}
}

func lost(reason game.ResultReason) game.Result {
	return game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, game.SeatSolo, reason)
}

func (s state) View(game.Seat) View {
	view := View{
		FEN:         s.position.FEN(),
		HistorySAN:  append([]string{}, s.history...),
		LastMove:    s.last,
		LegalMoves:  []string{},
		CheckSquare: s.position.CheckSquare(),
		Mode:        s.mode,
		Misses:      s.misses,
		Remaining:   s.line.movesBy(s.solver, s.ply),
		Solution:    []string{},
	}
	if s.finished {
		view.Solution = slices.Clone(s.line.san)
	} else {
		view.LegalMoves = xslices.Map(s.position.LegalMoves(), chess.Move.UCI)
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{game.SeatSolo}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game,
// which drops the legal moves and reveals the solution.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	return s.finish(result)
}
