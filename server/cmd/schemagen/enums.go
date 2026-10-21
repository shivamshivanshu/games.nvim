package main

import (
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// enum is a string type the wire restricts to values. An open enum lists the
// core's values as examples, since each game adds its own.
type enum struct {
	typ    reflect.Type
	values []string
	open   bool
}

func closed[T ~string](values ...T) enum {
	return enum{typ: reflect.TypeFor[T](), values: xslices.Map(values, func(v T) string { return string(v) })}
}

func open[T ~string](values ...T) enum {
	e := closed(values...)
	e.open = true
	return e
}

func messageTypes() []protocol.MessageType {
	types := []protocol.MessageType{protocol.TypeOK, protocol.TypeError}
	for _, r := range protocol.Requests {
		types = append(types, r.Type)
	}
	for _, p := range protocol.Pushes {
		types = append(types, p.Type)
	}
	return types
}

// coreEnums are defined once, in the protocol schema, and referred to by every game's.
func coreEnums() []enum {
	return []enum{
		closed(messageTypes()...),
		closed(protocol.ErrorCodes...),
		closed(protocol.ChallengePending, protocol.ChallengeAccepted, protocol.ChallengeDeclined, protocol.ChallengeCancelled, protocol.ChallengeExpired),
		closed(protocol.StatusOngoing, protocol.StatusFinished),
		closed(protocol.ControlResign, protocol.ControlOfferDraw, protocol.ControlDeclineDraw, protocol.ControlAbort),
		closed(protocol.OptionEnum, protocol.OptionText),
		closed(game.OptionRoleClock),
		closed(game.CapResign, game.CapDraw, game.CapAbort, game.CapClock),
		closed(game.ResultDecisive, game.ResultDraw, game.ResultAborted),
		open(game.ReasonResign, game.ReasonTimeout, game.ReasonDrawAgreed, game.ReasonAborted),
	}
}

func (e enum) schema(description string) *jsonschema.Schema {
	values := xslices.Map(e.values, func(v string) any { return v })
	s := &jsonschema.Schema{Type: "string", Description: description}
	if e.open {
		s.Examples = values
	} else {
		s.Enum = values
	}
	return s
}

// enumRefs maps each enum's type to a reference into defsURI, the document
// whose $defs hold the enums.
func enumRefs(enums []enum, defsURI string) func(reflect.Type) *jsonschema.Schema {
	refs := make(map[reflect.Type]string, len(enums))
	for _, e := range enums {
		refs[e.typ] = defsURI + "#/$defs/" + e.typ.Name()
	}
	return func(t reflect.Type) *jsonschema.Schema {
		if ref, ok := refs[t]; ok {
			return &jsonschema.Schema{Ref: ref}
		}
		return nil
	}
}
