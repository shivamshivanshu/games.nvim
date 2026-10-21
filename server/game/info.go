package game

import (
	"errors"
	"fmt"
	"slices"
)

// Check reports a catalogue the core cannot serve: its options, actions or
// events are malformed or missing, or it offers a clock without a time control
// option or the other way round.
func (i Info) Check() error {
	return errors.Join(i.checkOptions(), i.checkActions(), i.checkEvents(), i.checkClock())
}

func (i Info) checkEvents() error {
	if len(i.Events) == 0 {
		return fmt.Errorf("game %s: declares no events", i.Type)
	}
	seen := make(map[EventKind]bool, len(i.Events))
	for _, e := range i.Events {
		if seen[e.EventKind()] {
			return fmt.Errorf("game %s: event %q declared twice", i.Type, e.EventKind())
		}
		seen[e.EventKind()] = true
	}
	return nil
}

func (i Info) checkClock() error {
	clocked := slices.Contains(i.Capabilities, CapClock)
	timed := slices.ContainsFunc(i.Options, func(s OptionSpec) bool { return s.Role == OptionRoleClock })
	if clocked != timed {
		return fmt.Errorf("game %s: the %q capability and an option of role %q need each other", i.Type, CapClock, OptionRoleClock)
	}
	return nil
}
