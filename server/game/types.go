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
	EventKind  string
	Seq        uint64
	Seed       uint64
)

type Player struct {
	User UserID `json:"user"`
	Seat Seat   `json:"seat"`
}

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

type (
	OptionName  string
	OptionValue string
)

type OptionSpec struct {
	Name    OptionName
	Values  []OptionValue
	Default OptionValue
}

type PlayerRange struct{ Min, Max int }

type Info struct {
	Type         GameType
	Name         string
	Players      PlayerRange
	Capabilities []Capability
	Options      []OptionSpec
}

type Event interface{ EventKind() EventKind }

// Options is the validated, default-filled option set stored with a game.
type Options map[OptionName]OptionValue

const OptionTimeControl OptionName = "time_control"

type ClockConfig struct {
	Initial, Increment time.Duration
	Enabled            bool
}

func (o Options) Clock() (ClockConfig, error) {
	spec, ok := o[OptionTimeControl]
	if !ok || spec == OptionValue(TimeControlNone) {
		return ClockConfig{}, nil
	}
	minutes, seconds, found := strings.Cut(string(spec), "+")
	m, errM := strconv.Atoi(minutes)
	s, errS := strconv.Atoi(seconds)
	if !found || errM != nil || errS != nil || m < 0 || s < 0 {
		return ClockConfig{}, fmt.Errorf("invalid %s %q: want \"M+S\" or %q", OptionTimeControl, spec, TimeControlNone)
	}
	return ClockConfig{
		Initial:   time.Duration(m) * time.Minute,
		Increment: time.Duration(s) * time.Second,
		Enabled:   true,
	}, nil
}
