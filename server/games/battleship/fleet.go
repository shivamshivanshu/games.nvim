package battleship

import "github.com/shivamshivanshu/games.nvim/server/game/grid"

const boardSize = 10

// ShipName names a ship of the fleet.
type ShipName string

// The ships, largest first.
const (
	ShipCarrier    ShipName = "carrier"
	ShipBattleship ShipName = "battleship"
	ShipCruiser    ShipName = "cruiser"
	ShipSubmarine  ShipName = "submarine"
	ShipDestroyer  ShipName = "destroyer"
)

// Fleet is every seat's fleet, in the order its ships are placed.
func Fleet() []ShipSpec {
	return []ShipSpec{
		{Name: ShipCarrier, Size: 5},
		{Name: ShipBattleship, Size: 4},
		{Name: ShipCruiser, Size: 3},
		{Name: ShipSubmarine, Size: 3},
		{Name: ShipDestroyer, Size: 2},
	}
}

// Orientation is the way a ship lies from its anchor, its left or top end.
type Orientation string

// The orientations.
const (
	Across Orientation = "across"
	Down   Orientation = "down"
)

func orientations() []Orientation { return []Orientation{Across, Down} }

// footprint is the size squares a ship covers from anchor, some of them off
// the board when it does not fit.
func footprint(anchor grid.Point, size int, o Orientation) []grid.Point {
	points := make([]grid.Point, size)
	for i := range points {
		points[i] = anchor
		if o == Across {
			points[i].Col += i
		} else {
			points[i].Row += i
		}
	}
	return points
}

func coords() grid.Coords { return grid.NewCoords(boardSize, boardSize) }

func squareAt(p grid.Point) Square { return Square{Row: p.Row + 1, Col: p.Col + 1} }
