package game

import (
	"fmt"
	"regexp"
	"strconv"
)

// ActionKind is the "kind" field every action object carries.
type ActionKind string

// ActionSpec is one entry of a game's action catalogue. Input is nil for an
// action that is its kind alone.
type ActionSpec struct {
	Kind        ActionKind
	Description string
	Input       *ActionInput
}

// ActionInput is the one text field an action takes from its player: the
// text matches Pattern, an RE2 expression, or is one of Values.
type ActionInput struct {
	Field       string
	Description string
	Pattern     string
	Values      []string
}

// CheckActions reports an empty catalogue, a duplicate kind, or an input that
// has both or neither of Pattern and Values, or whose Pattern does not compile.
func (i Info) CheckActions() error {
	if len(i.Actions) == 0 {
		return fmt.Errorf("game %s: declares no actions", i.Type)
	}
	seen := make(map[ActionKind]bool, len(i.Actions))
	for _, spec := range i.Actions {
		if seen[spec.Kind] {
			return fmt.Errorf("game %s: action %q declared twice", i.Type, spec.Kind)
		}
		seen[spec.Kind] = true
		if err := spec.Input.check(); err != nil {
			return fmt.Errorf("game %s: action %q: %w", i.Type, spec.Kind, err)
		}
	}
	return nil
}

func (in *ActionInput) check() error {
	if in == nil {
		return nil
	}
	if (in.Pattern == "") == (len(in.Values) == 0) {
		return fmt.Errorf("input %q needs exactly one of a pattern or values", in.Field)
	}
	if in.Pattern == "" {
		return nil
	}
	if _, err := regexp.Compile(in.Pattern); err != nil {
		return fmt.Errorf("input %q: %w", in.Field, err)
	}
	return nil
}

// NumberedValues are the input values "1" to "n", in order.
func NumberedValues(n int) []string {
	values := make([]string, n)
	for i := range values {
		values[i] = strconv.Itoa(i + 1)
	}
	return values
}

// ParseNumbered is the number text names if it is exactly one of
// NumberedValues(n): no sign, no leading zero, no space.
func ParseNumbered(text string, n int) (int, bool) {
	i, err := strconv.Atoi(text)
	if err != nil || i < 1 || i > n || strconv.Itoa(i) != text {
		return 0, false
	}
	return i, true
}
