package game

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type (
	UserID     string
	GameID     string
	Seat       string
	GameType   string
	Capability string
)

const (
	CapResign Capability = "resign"
	CapDraw   Capability = "draw"
	CapAbort  Capability = "abort"
	CapClock  Capability = "clock"
)

type ResultReason string

const (
	ReasonResign     ResultReason = "resign"
	ReasonTimeout    ResultReason = "timeout"
	ReasonDrawAgreed ResultReason = "draw_agreed"
	ReasonAborted    ResultReason = "aborted"
)

type ResultKind int

const (
	ResultDecisive ResultKind = iota
	ResultDraw
	ResultAborted
)

type Result struct {
	Kind    ResultKind
	Winners []Seat
	Reason  ResultReason
}

type OptionSpec struct {
	Name    string
	Values  []string
	Default string
}

type PlayerRange struct{ Min, Max int }

type Info struct {
	Type         GameType
	Name         string
	Players      PlayerRange
	Capabilities []Capability
	Options      []OptionSpec
}

type Event interface{ EventKind() string }

// Options is the validated, default-filled option set stored with a game.
type Options map[string]string

const OptionTimeControl = "time_control"

const timeControlNone = "none"

type ClockConfig struct {
	Initial, Increment time.Duration
	Enabled            bool
}

// Clock parses the "M+S" time control: M minutes initial, S seconds increment.
func (o Options) Clock() (ClockConfig, error) {
	spec, ok := o[OptionTimeControl]
	if !ok || spec == timeControlNone {
		return ClockConfig{}, nil
	}
	minutes, seconds, found := strings.Cut(spec, "+")
	m, errM := strconv.Atoi(minutes)
	s, errS := strconv.Atoi(seconds)
	if !found || errM != nil || errS != nil || m < 0 || s < 0 {
		return ClockConfig{}, fmt.Errorf("invalid %s %q: want \"M+S\" or %q", OptionTimeControl, spec, timeControlNone)
	}
	return ClockConfig{
		Initial:   time.Duration(m) * time.Minute,
		Increment: time.Duration(s) * time.Second,
		Enabled:   true,
	}, nil
}
