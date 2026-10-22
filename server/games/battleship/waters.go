package battleship

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// placedShip is a ship of the fleet on the squares it covers.
type placedShip struct {
	spec   ShipSpec
	points []grid.Point
}

// waters is one seat's board: the ships it has placed, in fleet order, and the
// squares its opponent has fired at. It is immutable: every change returns a copy.
type waters struct {
	ships []placedShip
	shot  []bool
}

func newWaters() waters {
	return waters{ships: []placedShip{}, shot: make([]bool, boardSize*boardSize)}
}

func index(p grid.Point) int { return p.Row*boardSize + p.Col }

// next is the ship to place, false once the fleet is complete.
func (w waters) next() (ShipSpec, bool) {
	fleet := Fleet()
	if len(w.ships) == len(fleet) {
		return ShipSpec{}, false
	}
	return fleet[len(w.ships)], true
}

func (w waters) complete() bool {
	_, placing := w.next()
	return !placing
}

// shipAt is the index of the ship covering p, false for open water.
func (w waters) shipAt(p grid.Point) (int, bool) {
	i := slices.IndexFunc(w.ships, func(s placedShip) bool { return slices.Contains(s.points, p) })
	return i, i >= 0
}

func (w waters) sunk(ship int) bool {
	return !slices.ContainsFunc(w.ships[ship].points, func(p grid.Point) bool { return !w.shot[index(p)] })
}

func (w waters) allSunk() bool {
	for i := range w.ships {
		if !w.sunk(i) {
			return false
		}
	}
	return true
}

// blocked is why a ship of size cannot lie from anchor as o: "" when it fits,
// else the reason as a phrase.
func (w waters) blocked(anchor grid.Point, size int, o Orientation) string {
	for _, p := range footprint(anchor, size, o) {
		if !p.Within(boardSize, boardSize) {
			return "runs off the board"
		}
		if i, ok := w.shipAt(p); ok {
			return "overlaps your " + string(w.ships[i].spec.Name)
		}
	}
	return ""
}

// anchors are the squares a ship of size may start on as o, row by row.
func (w waters) anchors(size int, o Orientation) []grid.Point {
	points := []grid.Point{}
	for i := range boardSize * boardSize {
		p := grid.Point{Row: i / boardSize, Col: i % boardSize}
		if w.blocked(p, size, o) == "" {
			points = append(points, p)
		}
	}
	return points
}

// place is w with the next ship lying from anchor as o, which must fit.
func (w waters) place(anchor grid.Point, o Orientation) waters {
	spec, _ := w.next()
	ship := placedShip{spec: spec, points: footprint(anchor, spec.Size, o)}
	return waters{ships: append(slices.Clone(w.ships), ship), shot: w.shot}
}

// fire is w with p fired at, which must not have been, and what the shot found.
func (w waters) fire(p grid.Point) (waters, ShotOutcome, ShipName) {
	next := waters{ships: w.ships, shot: slices.Clone(w.shot)}
	next.shot[index(p)] = true
	ship, ok := next.shipAt(p)
	switch {
	case !ok:
		return next, ShotMiss, ""
	case next.sunk(ship):
		return next, ShotSunk, next.ships[ship].spec.Name
	}
	return next, ShotHit, ""
}

// unshot are the squares not yet fired at, row by row.
func (w waters) unshot() []grid.Point {
	points := []grid.Point{}
	for i, shot := range w.shot {
		if !shot {
			points = append(points, grid.Point{Row: i / boardSize, Col: i % boardSize})
		}
	}
	return points
}

// tile is how p shows: to its owner when own, else to the opponent, who sees
// the ships only where it has hit them, or everywhere once revealed.
func (w waters) tile(p grid.Point, own, revealed bool) Tile {
	ship, isShip := w.shipAt(p)
	shot := w.shot[index(p)]
	switch {
	case isShip && shot && w.sunk(ship):
		return TileSunk
	case isShip && shot:
		return TileHit
	case shot:
		return TileMiss
	case isShip && (own || revealed):
		return TileShip
	}
	return TileEmpty
}

func (w waters) tiles(own, revealed bool) [][]Tile {
	return grid.Rows(boardSize, boardSize, func(p grid.Point) Tile { return w.tile(p, own, revealed) })
}

// fleet is every ship's status as its owner sees it when own, else as the
// opponent does, to whom a ship not yet sunk is afloat.
func (w waters) fleet(own bool) []ShipStatus {
	statuses := []ShipStatus{}
	for i, spec := range Fleet() {
		state := ShipAfloat
		switch {
		case i < len(w.ships) && w.sunk(i):
			state = ShipSunk
		case i >= len(w.ships) && own:
			state = ShipUnplaced
		}
		statuses = append(statuses, ShipStatus{Name: spec.Name, Size: spec.Size, State: state})
	}
	return statuses
}
