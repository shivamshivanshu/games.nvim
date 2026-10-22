package dotsandboxes

import (
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// state is immutable: every change returns a copy with fresh maps.
type state struct {
	game.Turn
	side   int
	edges  grid.Edges
	drawn  map[grid.Edge]game.Seat
	owners map[grid.Point]game.Seat
	last   *grid.Edge
}

func newState(side int) state {
	return state{
		Turn:   game.TurnTo(sides[0]),
		side:   side,
		edges:  grid.NewEdges(side, side),
		drawn:  map[grid.Edge]game.Seat{},
		owners: map[grid.Point]game.Seat{},
	}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	edge, err := s.undrawnEdge(action)
	if err != nil {
		return nil, nil, err
	}
	next, completed := s.withEdge(seat, edge)
	again := slices.Contains(next.ToAct(), seat)
	return next, []game.Event{DrawnEvent{By: seat, Edge: lineOf(edge), Boxes: grid.Cells(completed), Again: again}}, nil
}

// undrawnEdge is the edge action draws, or why it may not.
func (s state) undrawnEdge(action Action) (grid.Edge, error) {
	if action.Kind != ActionDraw {
		return grid.Edge{}, game.UnknownAction(action.Kind)
	}
	edge, ok := catalogueEdges().Parse(action.Edge)
	if !ok {
		return grid.Edge{}, game.Illegalf("edge must be a dot then h or v, such as b2h, got %q", action.Edge)
	}
	name := s.edges.Name(edge)
	if !s.edges.Contains(edge) {
		return grid.Edge{}, game.Illegalf("%s is off the board of %d boxes a side", name, s.side)
	}
	if _, taken := s.drawn[edge]; taken {
		return grid.Edge{}, game.Illegalf("%s is already drawn", name)
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
		next.Turn = s.To(sides.Other(seat))
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
		s.Turn = s.Turn.Conclude(game.WinFor(ReasonMoreBoxes, first))
	case scores[second] > scores[first]:
		s.Turn = s.Turn.Conclude(game.WinFor(ReasonMoreBoxes, second))
	default:
		s.Turn = s.Turn.Conclude(game.DrawBy(ReasonEqualBoxes))
	}
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
	return grid.Rows(rows, cols, func(dot grid.Point) game.Seat {
		return s.drawn[grid.Edge{Dot: dot, Orientation: orientation}]
	})
}

func (s state) boxOwners() [][]game.Seat {
	return grid.Rows(s.side, s.side, func(cell grid.Point) game.Seat { return s.owners[cell] })
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is offered
// an edge.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Turn = s.Turn.Conclude(result)
	return s
}

func lineOf(edge grid.Edge) Line {
	return Line{Cell: edge.Dot.Cell(), Orientation: edge.Orientation}
}
