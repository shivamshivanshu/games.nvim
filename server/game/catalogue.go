package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

// ActionKindField is the field of an action that holds its kind.
const ActionKindField = "kind"

// catalogue holds the action catalogue as the core enforces it, before a
// game parses an action: the action type's fields by their exact wire names,
// and per kind its input, nil for a kind that is its kind alone.
type catalogue struct {
	fields []string
	inputs map[ActionKind]*compiledInput
}

type compiledInput struct {
	ActionInput
	pattern *regexp.Regexp
}

func compileCatalogue(specs []ActionSpec, action reflect.Type) (catalogue, error) {
	c := catalogue{fields: wireFieldNames(action), inputs: make(map[ActionKind]*compiledInput, len(specs))}
	for _, spec := range specs {
		if spec.Input == nil {
			c.inputs[spec.Kind] = nil
			continue
		}
		in := &compiledInput{ActionInput: *spec.Input}
		if in.Pattern != "" {
			pattern, err := regexp.Compile(in.Pattern)
			if err != nil {
				return catalogue{}, err
			}
			in.pattern = pattern
		}
		c.inputs[spec.Kind] = in
	}
	return c, nil
}

// wireFieldNames are the names encoding/json gives the direct fields of
// struct type t, which CheckGame keeps from embedding any.
func wireFieldNames(t reflect.Type) []string {
	var names []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch {
		case !f.IsExported() || name == "-":
		case name == "":
			names = append(names, f.Name)
		default:
			names = append(names, name)
		}
	}
	return names
}

// checkActionType fails on an action type that is not a struct, or that
// embeds one, whose promoted fields the core does not name as encoding/json
// does.
func checkActionType(t reflect.Type) error {
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("action type %s is not a struct", t)
	}
	for i := range t.NumField() {
		if f := t.Field(i); f.Anonymous {
			return fmt.Errorf("action type %s embeds %s: name it as a field", t, f.Type)
		}
	}
	return nil
}

// actionFields are an action's fields as sent, and its kind.
type actionFields struct {
	kind   ActionKind
	fields map[string]json.RawMessage
}

func decodeActionFields(raw json.RawMessage) (actionFields, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return actionFields{}, err
	}
	var kind ActionKind
	if raw, ok := fields[ActionKindField]; ok {
		if err := json.Unmarshal(raw, &kind); err != nil {
			return actionFields{}, err
		}
	}
	return actionFields{kind: kind, fields: fields}, nil
}

// checkFields fails on a field the action type lacks, by its exact name, so
// that encoding/json's case folding takes none, and on an action of a
// catalogued kind that carries the input of another kind.
func (c catalogue) checkFields(a actionFields) error {
	for name := range a.fields {
		if !slices.Contains(c.fields, name) {
			return BadRequestf("malformed action: no field %q", name)
		}
	}
	own, catalogued := c.inputs[a.kind]
	if !catalogued {
		return nil
	}
	for kind, in := range c.inputs {
		if _, carried := a.fields[in.fieldName()]; carried && in.fieldName() != own.fieldName() {
			return BadRequestf("malformed action: %s takes no field %q, which is %s's", a.kind, in.Field, kind)
		}
	}
	return nil
}

func (in *compiledInput) fieldName() string {
	if in == nil {
		return ""
	}
	return in.Field
}

// checkInput fails on a catalogued input whose text is not one of its values,
// or does not match its pattern; a missing input is empty text.
func (c catalogue) checkInput(a actionFields) error {
	in := c.inputs[a.kind]
	if in == nil {
		return nil
	}
	var text string
	if raw, ok := a.fields[in.Field]; ok {
		if err := json.Unmarshal(raw, &text); err != nil {
			return BadRequestf("malformed action: %s is not text: %v", in.Field, err)
		}
	}
	switch {
	case in.pattern == nil && !slices.Contains(in.Values, text):
		return Illegalf("%s must be one of %s; got %q", in.Field, strings.Join(in.Values, ", "), text)
	case in.pattern != nil && !in.pattern.MatchString(text):
		return Illegalf("%s %q is not valid: %s", in.Field, text, in.Description)
	}
	return nil
}
