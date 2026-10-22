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

// Assign seats the challenger first if challengerFirst, and the opponent in
// the other seat.
func (p Pair) Assign(challenger, opponent UserID, challengerFirst bool) map[UserID]Seat {
	if challengerFirst {
		return map[UserID]Seat{challenger: p[0], opponent: p[1]}
	}
	return map[UserID]Seat{challenger: p[1], opponent: p[0]}
}

// AssignFirst seats the challenger and its one invitee, the one first picks
// to move first in p[0].
func (p Pair) AssignFirst(first FirstMover, challenger UserID, invitees []UserID, rng *rand.Rand) map[UserID]Seat {
	return p.Assign(challenger, invitees[0], PickFirst(first, rng))
}
