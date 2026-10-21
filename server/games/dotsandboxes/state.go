package dotsandboxes

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// state is immutable: every change returns a copy with fresh maps.
type state struct {
	side     int
	edges    grid.Edges
	drawn    map[grid.Edge]game.Seat
	owners   map[grid.Point]game.Seat
	last     *grid.Edge
	toMove   game.Seat
	result   game.Result
	finished bool
}

func newState(side int) state {
	return state{
		side:   side,
		edges:  grid.NewEdges(side, side),
		drawn:  map[grid.Edge]game.Seat{},
		owners: map[grid.Point]game.Seat{},
		toMove: sides[0],
	}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	edge, err := s.undrawnEdge(action)
	if err != nil {
		return nil, nil, err
	}
	next, completed := s.withEdge(seat, edge)
	again := !next.finished && next.toMove == seat
	return next, []game.Event{DrawnEvent{By: seat, Edge: lineOf(edge), Boxes: xslices.Map(completed, boxOf), Again: again}}, nil
}

// undrawnEdge is the edge action draws, or why it may not.
func (s state) undrawnEdge(action Action) (grid.Edge, error) {
	if action.Kind != ActionDraw {
		return grid.Edge{}, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	edge, ok := catalogueEdges().Parse(action.Edge)
	if !ok {
		return grid.Edge{}, game.IllegalActionError{Msg: fmt.Sprintf("edge must be a dot then h or v, such as b2h, got %q", action.Edge)}
	}
	name := s.edges.Name(edge)
	if !s.edges.Contains(edge) {
		return grid.Edge{}, game.IllegalActionError{Msg: fmt.Sprintf("%s is off the board of %d boxes a side", name, s.side)}
	}
	if _, taken := s.drawn[edge]; taken {
		return grid.Edge{}, game.IllegalActionError{Msg: name + " is already drawn"}
	}
	return edge, nil
}

// withEdge draws edge for seat and gives seat each box it completes. Seat moves again after
// completing one; otherwise the move passes. The game ends once every box is complete.
func (s state) withEdge(seat game.Seat, edge grid.Edge) (state, []grid.Point) {
	next := s
	next.drawn = maps.Clone(s.drawn)
	next.drawn[edge] = seat
	next.owners = maps.Clone(s.owners)
	next.last = &edge
	completed := []grid.Point{}
	for _, cell := range s.edges.Bordering(edge) {
		if next.closed(cell) {
			next.owners[cell] = seat
			completed = append(completed, cell)
		}
	}
	switch {
	case len(next.owners) == s.side*s.side:
		next = next.ended()
	case len(completed) == 0:
		next.toMove = sides.Other(seat)
	}
	return next, completed
}

func (s state) closed(cell grid.Point) bool {
	return !slices.ContainsFunc(grid.Sides(cell), func(side grid.Edge) bool {
		_, drawn := s.drawn[side]
		return !drawn
	})
}

// ended decides the game on the boxes each seat holds.
func (s state) ended() state {
	scores := s.scores()
	first, second := sides[0], sides[1]
	switch {
	case scores[first] > scores[second]:
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{first}, Reason: ReasonMoreBoxes}
	case scores[second] > scores[first]:
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{second}, Reason: ReasonMoreBoxes}
	default:
		s.result = game.Result{Kind: game.ResultDraw, Reason: ReasonEqualBoxes}
	}
	s.finished = true
	return s
}

func (s state) scores() map[game.Seat]int {
	scores := make(map[game.Seat]int, len(sides))
	for _, seat := range sides {
		scores[seat] = 0
	}
	for _, owner := range s.owners {
		scores[owner]++
	}
	return scores
}

func (s state) View(viewer game.Seat) View {
	legal := []Line{}
	if slices.Contains(s.ToAct(), viewer) {
		legal = xslices.Map(slices.DeleteFunc(s.edges.All(), func(edge grid.Edge) bool {
			_, drawn := s.drawn[edge]
			return drawn
		}), lineOf)
	}
	var last *Line
	if s.last != nil {
		line := lineOf(*s.last)
		last = &line
	}
	return View{
		Across:     s.drawers(grid.EdgeAcross, s.side+1, s.side),
		Down:       s.drawers(grid.EdgeDown, s.side, s.side+1),
		Boxes:      s.boxOwners(),
		Scores:     s.scores(),
		Last:       last,
		LegalEdges: legal,
	}
}

// drawers is the seat that drew each edge lying orientation, by its dot's row and column, "" for
// one undrawn.
func (s state) drawers(orientation grid.EdgeOrientation, rows, cols int) [][]game.Seat {
	return seatRows(rows, cols, func(dot grid.Point) game.Seat {
		return s.drawn[grid.Edge{Dot: dot, Orientation: orientation}]
	})
}

func (s state) boxOwners() [][]game.Seat {
	return seatRows(s.side, s.side, func(cell grid.Point) game.Seat { return s.owners[cell] })
}

// seatRows is rows of cols seats, each the one at gives for its point.
func seatRows(rows, cols int, at func(grid.Point) game.Seat) [][]game.Seat {
	out := make([][]game.Seat, rows)
	for r := range out {
		out[r] = make([]game.Seat, cols)
		for c := range out[r] {
			out[r][c] = at(grid.Point{Row: r, Col: c})
		}
	}
	return out
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is offered
// an edge.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}

func lineOf(edge grid.Edge) Line {
	return Line{Row: edge.Dot.Row + 1, Col: edge.Dot.Col + 1, Orientation: edge.Orientation}
}

func boxOf(cell grid.Point) Box { return Box{Row: cell.Row + 1, Col: cell.Col + 1} }
