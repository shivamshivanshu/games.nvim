package game

import "math/rand/v2"

// Pair is the two seats of a two-player game, the first mover's first.
type Pair [2]Seat

// Other is the seat facing s.
func (p Pair) Other(s Seat) Seat {
	if s == p[0] {
		return p[1]
	}
	return p[0]
}

func (p Pair) Seats() []Seat { return []Seat{p[0], p[1]} }

// Assign seats the challenger and its one invitee, the one first picks to
// move first in p[0].
func (p Pair) Assign(first FirstMover, challenger UserID, invitees []UserID, rng *rand.Rand) map[UserID]Seat {
	opponent := invitees[0]
	if PickFirst(first, rng) {
		return map[UserID]Seat{challenger: p[0], opponent: p[1]}
	}
	return map[UserID]Seat{challenger: p[1], opponent: p[0]}
}
