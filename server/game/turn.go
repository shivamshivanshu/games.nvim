package game

import "slices"

// CheckTurn fails with ErrNotYourTurn unless seat is one s waits on.
func CheckTurn(s interface{ ToAct() []Seat }, seat Seat) error {
	if !slices.Contains(s.ToAct(), seat) {
		return ErrNotYourTurn
	}
	return nil
}

// Turn is embedded by a state whose seats move one at a time: whose move it
// is, and how the game ended.
type Turn struct {
	Ending
	toMove Seat
}

// TurnTo is seat to move.
func TurnTo(seat Seat) Turn { return Turn{toMove: seat} }

// To is the turn passed to seat; a finished turn stays as it is.
func (t Turn) To(seat Seat) Turn {
	if t.Over() {
		return t
	}
	return Turn{toMove: seat}
}

// Conclude keeps a result already reached; otherwise result ends the game.
func (t Turn) Conclude(result Result) Turn {
	return Turn{Ending: t.Concluded(result), toMove: t.toMove}
}

// ToAct is the seat to move, none once finished.
func (t Turn) ToAct() []Seat { return t.ToActOf(t.toMove) }
