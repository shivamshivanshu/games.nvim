package main

import (
	"bytes"
	"encoding/json"
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
}

func newDocument(comments map[string]string, mapper func(reflect.Type) *jsonschema.Schema) *document {
	return &document{
		reflector: &jsonschema.Reflector{Anonymous: true, Mapper: mapper, CommentMap: comments},
		defs:      jsonschema.Definitions{},
		comments:  comments,
	}
}

// ref adds t and every type it holds to the document's $defs, and returns the
// schema referring to t.
func (d *document) ref(t reflect.Type) (*jsonschema.Schema, error) {
	s := d.reflector.ReflectFromType(t)
	for name, def := range s.Definitions {
		if err := d.define(name, def); err != nil {
			return nil, err
		}
	}
	return &jsonschema.Schema{Ref: s.Ref}, nil
}

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

// eventRef is e's schema with the kind field game.EncodeEvent adds to it. It
// is the def named after e's type, or after the type and the kind when that
// name is taken: by the type's own schema, which another schema uses, or by
// an event of another kind.
func (d *document) eventRef(e game.Event) (*jsonschema.Schema, error) {
	t := reflect.TypeOf(e)
	reflected := d.reflector.ReflectFromType(t)
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
