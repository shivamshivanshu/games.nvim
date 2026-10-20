package game

import "math/rand/v2"

func OptionValues[T ~string](vs ...T) []OptionValue {
	out := make([]OptionValue, len(vs))
	for i, v := range vs {
		out[i] = OptionValue(v)
	}
	return out
}

type TimeControl string

const (
	TimeControlNone     TimeControl = "none"
	TimeControl3Plus2   TimeControl = "3+2"
	TimeControl5Plus0   TimeControl = "5+0"
	TimeControl10Plus5  TimeControl = "10+5"
	TimeControl15Plus10 TimeControl = "15+10"
)

func TimeControlOption() OptionSpec {
	return OptionSpec{
		Name:    OptionTimeControl,
		Values:  OptionValues(TimeControlNone, TimeControl3Plus2, TimeControl5Plus0, TimeControl10Plus5, TimeControl15Plus10),
		Default: OptionValue(TimeControl10Plus5),
	}
}

const OptionFirstMover OptionName = "first"

// FirstMover is relative to the challenger: FirstMoverMe means the challenger moves first.
type FirstMover string

const (
	FirstMoverRandom   FirstMover = "random"
	FirstMoverMe       FirstMover = "me"
	FirstMoverOpponent FirstMover = "opponent"
)

func FirstMoverOption() OptionSpec {
	return OptionSpec{
		Name:    OptionFirstMover,
		Values:  OptionValues(FirstMoverRandom, FirstMoverMe, FirstMoverOpponent),
		Default: OptionValue(FirstMoverRandom),
	}
}

// PickFirst reports whether the challenger moves first.
func PickFirst(choice FirstMover, rng *rand.Rand) bool {
	switch choice {
	case FirstMoverMe:
		return true
	case FirstMoverOpponent:
		return false
	}
	return rng.IntN(2) == 0
}
