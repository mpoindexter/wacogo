package componentmodel

import (
	"context"
	"fmt"
)

type definition[V any, T Type] interface {
	assertDefinition
	createType(scope *scope) (T, error)
	createInstance(ctx context.Context, scope *scope) (V, error)
}

// Used to mark definition types
type assertDefinition interface {
	isDefinition()
}

type boundDefinition[V any, T Type] struct {
	scope       *scope
	def         definition[V, T]
	typ         T
	val         V
	valResolved bool
}

func (d *boundDefinition[V, T]) getType() T {
	return d.typ
}

func (d *boundDefinition[V, T]) getInstance() (V, error) {
	if !d.valResolved {
		return d.val, fmt.Errorf("instance not resolved")
	}
	return d.val, nil
}

type definitions struct {
	defs    map[any]any
	binders []definitionBinder
}

func newDefinitions() *definitions {
	return &definitions{
		defs: make(map[any]any),
	}
}

type sortDefinitions[V any, T Type] struct {
	definitions *definitions
	items       []definition[V, T]
	sort        sort[V, T]
}

func (d *sortDefinitions[V, T]) get(index uint32) (definition[V, T], error) {
	if int(index) >= len(d.items) {
		return nil, fmt.Errorf("unknown %s: %s index out of bounds", d.sort.typeName(), d.sort.typeName())
	}
	return d.items[index], nil
}

func (d *sortDefinitions[V, T]) add(def definition[V, T]) uint32 {
	d.items = append(d.items, def)
	d.definitions.binders = append(d.definitions.binders, &definitionBinderImpl[V, T]{def: def, sort: d.sort})
	return uint32(len(d.items) - 1)
}

func (d *sortDefinitions[V, T]) len() uint32 {
	return uint32(len(d.items))
}

func sortDefsFor[V any, T Type](defs *definitions, sort sort[V, T]) *sortDefinitions[V, T] {
	sortDefs, ok := defs.defs[sort]
	if !ok {
		newDefs := &sortDefinitions[V, T]{
			sort:        sort,
			definitions: defs,
		}
		defs.defs[sort] = newDefs
		return newDefs
	}
	return sortDefs.(*sortDefinitions[V, T])
}

type definitionBinder interface {
	bindType(scope *scope) error
	bindInstance(ctx context.Context, scope *scope) error
}

type definitionBinderImpl[V any, T Type] struct {
	sort sort[V, T]
	def  definition[V, T]
}

func (b *definitionBinderImpl[V, T]) bindType(scope *scope) error {
	typ, err := b.def.createType(scope)
	if err != nil {
		return err
	}
	if any(typ) == Type(nil) {
		return fmt.Errorf("definition produced nil type")
	}
	ss := sortScopeFor(scope, b.sort)
	ss.items = append(ss.items, &boundDefinition[V, T]{
		scope: scope,
		def:   b.def,
		typ:   typ,
	})
	return nil
}

func (b *definitionBinderImpl[V, T]) bindInstance(ctx context.Context, scope *scope) error {
	typ, err := b.def.createType(scope)
	if err != nil {
		return err
	}
	scope.currentType = typ
	defer func() { scope.currentType = nil }()

	val, err := b.def.createInstance(ctx, scope)
	if err != nil {
		return err
	}
	ss := sortScopeFor(scope, b.sort)
	ss.items = append(ss.items, &boundDefinition[V, T]{
		scope:       scope,
		def:         b.def,
		typ:         typ,
		val:         val,
		valResolved: true,
	})
	return nil
}
