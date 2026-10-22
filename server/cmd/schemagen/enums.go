package main

import (
	"reflect"
	"slices"

	"github.com/invopop/jsonschema"
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/mastermind"
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

var (
	enumInterface     = reflect.TypeFor[game.Enum]()
	openEnumInterface = reflect.TypeFor[game.OpenEnum]()
)

// corePackages hold the identifiers, such as game.Seat, that are named
// strings without being kinds.
var corePackages = []string{reflect.TypeFor[game.Seat]().PkgPath(), reflect.TypeFor[protocol.MessageType]().PkgPath()}

// freeText are the games' named strings that hold text of their own rather
// than one of a set.
var freeText = []reflect.Type{reflect.TypeFor[mastermind.Code]()}

func closed[T ~string](values ...T) enum {
	return enum{typ: reflect.TypeFor[T](), values: game.Strings(values...)}
}

// enumOf is t's enum, if t is a string type that implements game.Enum.
func enumOf(t reflect.Type) (enum, bool) {
	if t.Kind() != reflect.String || !t.Implements(enumInterface) {
		return enum{}, false
	}
	values := reflect.Zero(t).Interface().(game.Enum).EnumValues()
	return enum{typ: t, values: values, open: t.Implements(openEnumInterface)}, true
}

// isGameKind reports whether t is a game's string kind, which the wire can
// restrict only if it implements game.Enum.
func isGameKind(t reflect.Type) bool {
	return t.Kind() == reflect.String && t.PkgPath() != "" && !slices.Contains(corePackages, t.PkgPath()) && !slices.Contains(freeText, t)
}

func (e enum) schema(description string) *jsonschema.Schema {
	values := xslices.Any(e.values)
	s := &jsonschema.Schema{Type: "string", Description: description}
	if e.open {
		s.Examples = values
	} else {
		s.Enum = values
	}
	return s
}

// enumRefs maps each type to a reference to its def in defsURI.
func enumRefs(types []reflect.Type, defsURI string) map[reflect.Type]string {
	refs := make(map[reflect.Type]string, len(types))
	for _, t := range types {
		refs[t] = defsURI + defsPrefix + t.Name()
	}
	return refs
}
