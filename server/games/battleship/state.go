package battleship

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// state holds both seats' waters; sides[0] fires first once both fleets are placed.
type state struct {
	game.Ending
	waters map[game.Seat]waters
	toFire game.Seat
	last   *ShotEvent
}

func newState() state {
	all := make(map[game.Seat]waters, len(sides))
	for _, seat := range sides {
		all[seat] = newWaters()
	}
	return state{waters: all, toFire: sides[0]}
}

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	var (
		next   state
		events []game.Event
		err    error
	)
	switch action.Kind {
	case ActionPlaceAcross:
		next, events, err = s.placeTyped(seat, action.Cell, Across)
	case ActionPlaceDown:
		next, events, err = s.placeTyped(seat, action.Cell, Down)
	case ActionRandom:
		next, events, err = s.placeRandom(seat, rng)
	case ActionFire:
		next, events, err = s.fire(seat, action.Cell)
	default:
		return nil, nil, game.UnknownAction(action.Kind)
	}
	if err != nil {
		return nil, nil, err
	}
	return next, events, nil
}

func parseSquare(text string) (grid.Point, error) {
	c := coords()
	p, ok := c.Parse(text)
	if !ok {
		return grid.Point{}, game.Illegalf("cell must be %s, a letter then a number, got %q", c.Range(), text)
	}
	return p, nil
}

func (s state) placeTyped(seat game.Seat, cell string, o Orientation) (state, []game.Event, error) {
	if s.phase() != PhaseSetup {
		return state{}, nil, game.Illegalf("the fleets are placed; fire at a cell")
	}
	anchor, err := parseSquare(cell)
	if err != nil {
		return state{}, nil, err
	}
	own := s.waters[seat]
	spec, _ := own.next()
	if why := own.blocked(anchor, spec.Size, o); why != "" {
		return state{}, nil, game.Illegalf("the %s %s from %s %s", spec.Name, o, coords().Name(anchor), why)
	}
	next, events := s.withPlaced(seat, own.place(anchor, o))
	return next, events, nil
}

// placeRandom places the rest of seat's fleet, each ship on one of its legal
// anchors and orientations drawn from rng.
func (s state) placeRandom(seat game.Seat, rng *rand.Rand) (state, []game.Event, error) {
	if s.phase() != PhaseSetup {
		return state{}, nil, game.Illegalf("the fleets are placed; fire at a cell")
	}
	own := s.waters[seat]
	for spec, placing := own.next(); placing; spec, placing = own.next() {
		type berth struct {
			anchor grid.Point
			o      Orientation
		}
		var berths []berth
		for _, o := range orientations() {
			for _, anchor := range own.anchors(spec.Size, o) {
				berths = append(berths, berth{anchor: anchor, o: o})
			}
		}
		if len(berths) == 0 {
			return state{}, nil, game.Illegalf("no room left for the %s", spec.Name)
		}
		chosen := berths[rng.IntN(len(berths))]
		own = own.place(chosen.anchor, chosen.o)
	}
	next, events := s.withPlaced(seat, own)
	return next, events, nil
}

// withPlaced is s with seat's waters replaced; a completed fleet reports
// ready, and the second one starts the battle.
func (s state) withPlaced(seat game.Seat, own waters) (state, []game.Event) {
	s.waters = xslices.With(s.waters, seat, own)
	if !own.complete() {
		return s, []game.Event{}
	}
	return s, []game.Event{ReadyEvent{By: seat}}
}

func (s state) fire(seat game.Seat, cell string) (state, []game.Event, error) {
	if s.phase() != PhaseBattle {
		return state{}, nil, game.Illegalf("fire once both fleets are placed")
	}
	p, err := parseSquare(cell)
	if err != nil {
		return state{}, nil, err
	}
	opponent := sides.Other(seat)
	target := s.waters[opponent]
	if target.shot[index(p)] {
		return state{}, nil, game.Illegalf("you have already fired at %s", coords().Name(p))
	}
	hit, outcome, ship := target.fire(p)
	shot := ShotEvent{By: seat, Cell: squareAt(p), Outcome: outcome, Ship: ship}
	s.waters = xslices.With(s.waters, opponent, hit)
	s.last = &shot
	s.toFire = opponent
	if hit.allSunk() {
		s.Ending = s.Concluded(game.WinFor(ReasonFleetSunk, seat))
	}
	return s, []game.Event{shot}, nil
}

func (s state) phase() Phase {
	switch {
	case s.Over():
		return PhaseOver
	case slices.ContainsFunc(sides[:], func(seat game.Seat) bool { return !s.waters[seat].complete() }):
		return PhaseSetup
	}
	return PhaseBattle
}

func (s state) View(viewer game.Seat) View {
	own, enemy := s.waters[viewer], s.waters[sides.Other(viewer)]
	view := View{
		Phase:   s.phase(),
		Own:     own.tiles(true, false),
		Target:  enemy.tiles(false, s.Over()),
		Fleet:   own.fleet(true),
		Enemy:   enemy.fleet(false),
		Anchors: Anchors{Across: []Square{}, Down: []Square{}},
		Targets: []Square{},
		Last:    s.last,
	}
	if !slices.Contains(s.ToAct(), viewer) {
		return view
	}
	if spec, placing := own.next(); placing {
		view.Next = &spec
		view.Anchors = Anchors{
			Across: xslices.Map(own.anchors(spec.Size, Across), squareAt),
			Down:   xslices.Map(own.anchors(spec.Size, Down), squareAt),
		}
		return view
	}
	view.Targets = xslices.Map(enemy.unshot(), squareAt)
	return view
}

func (s state) ToAct() []game.Seat {
	switch s.phase() {
	case PhaseOver:
		return nil
	case PhaseSetup:
		return slices.DeleteFunc(sides.Seats(), func(seat game.Seat) bool { return s.waters[seat].complete() })
	}
	return []game.Seat{s.toFire}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so
// no seat is offered a placement or a target, and each sees the other's fleet.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
