package game

import "github.com/shivamshivanshu/games.nvim/server/internal/xslices"

// Enum is a string kind the wire restricts to its EnumValues.
type Enum interface{ EnumValues() []string }

// OpenEnum is an Enum whose values are examples rather than the whole set:
// the core's, to which each game adds its own.
type OpenEnum interface {
	Enum
	OpenEnum()
}

// Strings is each value as its string, for an EnumValues method.
func Strings[T ~string](values ...T) []string {
	return xslices.Map(values, func(v T) string { return string(v) })
}

func (Capability) EnumValues() []string { return Strings(CapResign, CapDraw, CapAbort, CapClock) }

func (ResultKind) EnumValues() []string {
	return Strings(ResultDecisive, ResultDraw, ResultAborted)
}

func (ResultReason) EnumValues() []string {
	return Strings(ReasonResign, ReasonTimeout, ReasonDrawAgreed, ReasonAborted)
}

func (ResultReason) OpenEnum() {}

func (OptionRole) EnumValues() []string { return Strings(OptionRoleClock) }
