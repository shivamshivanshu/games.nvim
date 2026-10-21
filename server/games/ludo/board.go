package ludo

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	// TrackLength is the number of squares on the shared track.
	TrackLength = 52
	// TokensPerSeat is how many tokens each seat races home.
	TokensPerSeat = 4

	// ProgressBase is a token still in its base.
	ProgressBase Progress = -1
	// ProgressStart is a token on its seat's start square.
	ProgressStart     Progress = 0
	lastTrackProgress Progress = 50
	// ProgressHome is a finished token.
	ProgressHome Progress = 56

	minPlayers = 2
	maxPlayers = 4
	starOffset = 8
)

// seatOrder is the shared seat-colour order: the i-th player takes the i-th seat.
func seatOrder() []game.Seat {
	return []game.Seat{SeatBlue, SeatYellow, SeatRed, SeatGreen}
}

// trackStart places the seats clockwise so that seats 1 and 2 sit opposite
// and any three seats are adjacent.
func trackStart(seat game.Seat) Square {
	switch seat {
	case SeatBlue:
		return 0
	case SeatRed:
		return 13
	case SeatYellow:
		return 26
	case SeatGreen:
		return 39
	}
	panic("unreachable: no track start for seat " + string(seat))
}

// trackSquare is the shared-track square under a token, if it is on the track.
func trackSquare(seat game.Seat, p Progress) (Square, bool) {
	if p < ProgressStart || p > lastTrackProgress {
		return 0, false
	}
	return (trackStart(seat) + Square(p)) % TrackLength, true
}

// safeSquares are every start square and the star square starOffset past each, in track order.
func safeSquares() []Square {
	var safe []Square
	for _, seat := range clockwise(seatOrder()) {
		safe = append(safe, trackStart(seat), trackStart(seat)+starOffset)
	}
	return safe
}

func isSafe(square Square) bool { return slices.Contains(safeSquares(), square) }

func locationsOf(seat game.Seat, tokens Tokens) Locations {
	var locations Locations
	for i, p := range tokens {
		locations[i] = locationOf(seat, p)
	}
	return locations
}

// locationOf is where a token at p is drawn.
func locationOf(seat game.Seat, p Progress) Location {
	switch {
	case p == ProgressBase:
		return Location{Kind: LocationBase}
	case p == ProgressHome:
		return Location{Kind: LocationHome}
	case p > lastTrackProgress:
		step := int(p - lastTrackProgress)
		return Location{Kind: LocationHomeColumn, Step: &step}
	}
	square, _ := trackSquare(seat, p)
	return Location{Kind: LocationTrack, Square: &square}
}

// advance is where a token at from lands after roll, if the roll is usable at all.
func advance(from Progress, roll Roll) (Progress, bool) {
	if from == ProgressBase {
		return ProgressStart, roll == bonusRoll
	}
	to := from + Progress(roll)
	return to, to <= ProgressHome
}

func clockwise(seats []game.Seat) []game.Seat {
	return slices.SortedFunc(slices.Values(seats), func(a, b game.Seat) int { return int(trackStart(a) - trackStart(b)) })
}
