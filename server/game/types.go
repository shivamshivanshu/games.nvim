package game

import (
	"errors"
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

// CheckUserID rejects an empty name, or one with control or format characters,
// which would display differently than it reads or break the text it is
// written into, such as a PGN tag.
func CheckUserID(u UserID) error {
	switch {
	case u == "":
		return errors.New("user name must not be empty")
	case strings.ContainsFunc(string(u), IsHiddenRune):
		return fmt.Errorf("user name %q has control or format characters", u)
	}
	return nil
}

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

// ResultReason is open: the core's reasons below, and those each game adds.
type ResultReason string

const (
	ReasonResign     ResultReason = "resign"
	ReasonTimeout    ResultReason = "timeout"
	ReasonDrawAgreed ResultReason = "draw_agreed"
	ReasonAborted    ResultReason = "aborted"
)

// ResultKind encodes as its name, both on the wire and in the store.
type ResultKind string

const (
	ResultDecisive ResultKind = "decisive"
	ResultDraw     ResultKind = "draw"
	ResultAborted  ResultKind = "aborted"
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

// OptionKind is how a value is chosen: one of the spec's Values, or free text.
type OptionKind int

const (
	// OptionKindEnum takes exactly one of the spec's Values.
	OptionKindEnum OptionKind = iota
	// OptionKindText takes any text up to the spec's MaxLen.
	OptionKindText
)

// OptionRole marks an option whose meaning the core knows, so clients find it
// by role rather than by name.
type OptionRole string

// OptionRoleClock marks the option whose value is the time control.
const OptionRoleClock OptionRole = "clock"

// OptionSpec of OptionKindText has no Values and defaults to ""; MaxLen caps
// its value in bytes. Role is empty for an option only the game interprets.
type OptionSpec struct {
	Name    OptionName
	Kind    OptionKind
	Values  []OptionValue
	Default OptionValue
	MaxLen  int
	Role    OptionRole
}

type PlayerRange struct{ Min, Max int }

// AllowsSolo reports whether the game may seat one player alone, so it starts
// without a challenge, whether or not it also takes opponents.
func (r PlayerRange) AllowsSolo() bool { return r.Min == 1 }

type Info struct {
	Type         GameType
	Name         string
	Players      PlayerRange
	Capabilities []Capability
	Options      []OptionSpec
	Actions      []ActionSpec
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
