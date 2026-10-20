package game

import (
	"fmt"
	"slices"
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

// Digest is the lowercase hex SHA-256 of a finished game's summary bytes.
type Digest string

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

// ResultKind encodes as its name, both on the wire and in the store.
type ResultKind int

const (
	ResultDecisive ResultKind = iota
	ResultDraw
	ResultAborted
)

var resultKindNames = [...]string{ResultDecisive: "decisive", ResultDraw: "draw", ResultAborted: "aborted"}

func (k ResultKind) MarshalText() ([]byte, error) {
	if k < 0 || int(k) >= len(resultKindNames) {
		return nil, fmt.Errorf("unknown result kind %d", k)
	}
	return []byte(resultKindNames[k]), nil
}

func (k *ResultKind) UnmarshalText(text []byte) error {
	i := slices.Index(resultKindNames[:], string(text))
	if i < 0 {
		return fmt.Errorf("unknown result kind %q", text)
	}
	*k = ResultKind(i)
	return nil
}

type Result struct {
	Kind    ResultKind
	Winners []Seat
	Reason  ResultReason
}

type (
	OptionName  string
	OptionValue string
)

// OptionKind is how a value is chosen: one of the spec's Values, or free text.
type OptionKind int

const (
	// OptionKindEnum takes exactly one of the spec's Values.
	OptionKindEnum OptionKind = iota
	// OptionKindText takes any text up to the spec's MaxLen.
	OptionKindText
)

// OptionSpec of OptionKindText has no Values and defaults to ""; MaxLen caps
// its value in bytes.
type OptionSpec struct {
	Name    OptionName
	Kind    OptionKind
	Values  []OptionValue
	Default OptionValue
	MaxLen  int
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
