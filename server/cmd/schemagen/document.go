package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// document is one schema file: the $defs of every type reflected into it.
type document struct {
	reflector *jsonschema.Reflector
	defs      jsonschema.Definitions
	comments  map[string]string
	enums     map[reflect.Type]enum
	named     map[string]reflect.Type
	errs      []error
}

// newDocument refers each type in refs to its schema there; it gives each
// other enum a def, a map keyed by an enum its keys' names, and every
// unsigned integer type a minimum of 0.
func newDocument(comments map[string]string, refs map[reflect.Type]string) *document {
	d := &document{defs: jsonschema.Definitions{}, comments: comments, enums: map[reflect.Type]enum{}, named: map[string]reflect.Type{}}
	d.reflector = &jsonschema.Reflector{Anonymous: true, Mapper: func(t reflect.Type) *jsonschema.Schema {
		if ref, ok := refs[t]; ok {
			return &jsonschema.Schema{Ref: ref}
		}
		return d.mapUnreferenced(t)
	}, Namer: d.claimName, CommentMap: comments}
	return d
}

func (d *document) mapUnreferenced(t reflect.Type) *jsonschema.Schema {
	if e, ok := enumOf(t); ok {
		d.enums[t] = e
		return &jsonschema.Schema{Ref: defsPrefix + t.Name()}
	}
	switch t.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &jsonschema.Schema{Type: "integer", Minimum: "0"}
	case reflect.String:
		if isGameKind(t) {
			d.errs = append(d.errs, fmt.Errorf("%s is a string kind without EnumValues", t))
		}
	case reflect.Map:
		if t.Key().Implements(enumInterface) {
			return d.enumKeyedMap(t)
		}
	}
	return nil
}

// enumKeyedMap is t's schema with its keys restricted to their enum, which the
// reflector leaves open.
func (d *document) enumKeyedMap(t reflect.Type) *jsonschema.Schema {
	values := d.reflector.ReflectFromType(t.Elem())
	for name, def := range values.Definitions {
		if err := d.define(name, def); err != nil {
			d.errs = append(d.errs, err)
		}
	}
	values.Version, values.Definitions = "", nil
	return &jsonschema.Schema{Type: "object", PropertyNames: d.reflector.Mapper(t.Key()), AdditionalProperties: values}
}

// claimName fails on two types of one name, which the reflector would give
// one def. It names t as the reflector does.
func (d *document) claimName(t reflect.Type) string {
	if t.Name() == "" || t.PkgPath() == "" {
		return ""
	}
	if prev, ok := d.named[t.Name()]; ok && prev != t {
		d.errs = append(d.errs, fmt.Errorf("types %s and %s are both named %s", prev, t, t.Name()))
	}
	d.named[t.Name()] = t
	return ""
}

// ref adds t and every type it holds to the document's $defs, and returns the
// schema referring to t.
func (d *document) ref(t reflect.Type) (*jsonschema.Schema, error) {
	s := d.reflector.ReflectFromType(t)
	if err := d.err(); err != nil {
		return nil, err
	}
	for name, def := range s.Definitions {
		if err := d.define(name, def); err != nil {
			return nil, err
		}
	}
	return &jsonschema.Schema{Ref: s.Ref}, nil
}

func (d *document) err() error { return errors.Join(d.errs...) }

// define fails on two types of one name whose schemas differ.
func (d *document) define(name string, def *jsonschema.Schema) error {
	if prev, ok := d.defs[name]; ok {
		same, err := sameSchema(prev, def)
		if err != nil || !same {
			return fmt.Errorf("two types named %s with different schemas (%v)", name, err)
		}
	}
	d.defs[name] = def
	return nil
}

func (d *document) defineEnum(e enum) error {
	return d.define(e.typ.Name(), e.schema(d.comments[e.typ.PkgPath()+"."+e.typ.Name()]))
}

// defineEnums adds a def for each enum the document's types hold.
func (d *document) defineEnums() error {
	for _, e := range d.enums {
		if err := d.defineEnum(e); err != nil {
			return err
		}
	}
	return nil
}

// eventRef is e's schema with the kind field game.EncodeEvent adds to it. It
// is the def named after e's type, or after the type and the kind when that
// name is taken: by the type's own schema, which another schema uses, or by
// an event of another kind.
func (d *document) eventRef(e game.Event) (*jsonschema.Schema, error) {
	t := reflect.TypeOf(e)
	reflected := d.reflector.ReflectFromType(t)
	if err := d.err(); err != nil {
		return nil, err
	}
	for name, def := range reflected.Definitions {
		if name == t.Name() {
			continue
		}
		if err := d.define(name, def); err != nil {
			return nil, err
		}
	}
	name := t.Name()
	if _, taken := d.defs[name]; taken {
		name += "_" + string(e.EventKind())
	}
	if err := d.define(name, withKind(reflected.Definitions[t.Name()], e.EventKind())); err != nil {
		return nil, err
	}
	return &jsonschema.Schema{Ref: defsPrefix + name}, nil
}

func withKind(def *jsonschema.Schema, kind game.EventKind) *jsonschema.Schema {
	kinded := *def
	kinded.Properties = orderedmap.New[string, *jsonschema.Schema]()
	kinded.Properties.Set(game.EventKindField, &jsonschema.Schema{Const: string(kind)})
	for p := def.Properties.Oldest(); p != nil; p = p.Next() {
		kinded.Properties.Set(p.Key, p.Value)
	}
	kinded.Required = slices.Insert(slices.Clone(def.Required), 0, game.EventKindField)
	return &kinded
}

func sameSchema(a, b *jsonschema.Schema) (bool, error) {
	x, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	y, err := json.Marshal(b)
	return bytes.Equal(x, y), err
}
