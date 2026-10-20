package game

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode"
)

// TextOption is a free-text option of at most maxLen bytes, empty by default.
func TextOption(name OptionName, maxLen int) OptionSpec {
	return OptionSpec{Name: name, Kind: OptionKindText, Values: []OptionValue{}, MaxLen: maxLen}
}

// CheckOptions reports a spec whose kind is unknown, whose default its own
// check rejects, or that gives a text option Values.
func (i Info) CheckOptions() error {
	for _, spec := range i.Options {
		if spec.Kind == OptionKindText && len(spec.Values) > 0 {
			return fmt.Errorf("game %s: text option %q has values", i.Type, spec.Name)
		}
		if err := spec.check(spec.Default); err != nil {
			return fmt.Errorf("game %s: default: %w", i.Type, err)
		}
	}
	return nil
}

func (s OptionSpec) check(value OptionValue) error {
	switch s.Kind {
	case OptionKindEnum:
		return s.checkEnum(value)
	case OptionKindText:
		return s.checkText(value)
	}
	return fmt.Errorf("option %q: unknown kind %d", s.Name, s.Kind)
}

func (s OptionSpec) checkEnum(value OptionValue) error {
	if !slices.Contains(s.Values, value) {
		return BadRequestError{Msg: fmt.Sprintf("option %q: invalid value %q, want one of %v", s.Name, value, s.Values)}
	}
	return nil
}

func (s OptionSpec) checkText(value OptionValue) error {
	if len(value) > s.MaxLen {
		return BadRequestError{Msg: fmt.Sprintf("option %q: %d bytes, want at most %d", s.Name, len(value), s.MaxLen)}
	}
	if strings.ContainsFunc(string(value), IsHiddenRune) {
		return BadRequestError{Msg: fmt.Sprintf("option %q: control and format characters are not allowed", s.Name)}
	}
	return nil
}

// IsHiddenRune covers controls and format characters such as bidi overrides
// and zero-width joiners, which would make text display differently than it reads.
func IsHiddenRune(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

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
		Role:    OptionRoleClock,
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
