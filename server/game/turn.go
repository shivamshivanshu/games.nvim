package game

import "slices"

// CheckTurn fails with ErrNotYourTurn unless seat is one s waits on.
func CheckTurn(s interface{ ToAct() []Seat }, seat Seat) error {
	if !slices.Contains(s.ToAct(), seat) {
		return ErrNotYourTurn
	}
	return nil
}
