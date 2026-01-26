package componentmodel

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

type Component struct {
	id          string
	runtime     wazero.Runtime
	scope       *definitionScope
	importTypes map[string]Type
	exports     map[string]componentExport
}

func newComponent(id string, runtime wazero.Runtime, parent *definitionScope) *Component {
	return &Component{
		id:          id,
		runtime:     runtime,
		scope:       newDefinitionScope(parent),
		exports:     make(map[string]componentExport),
		importTypes: make(map[string]Type),
	}
}

func (c *Component) Instantiate(ctx context.Context, args map[string]any) (*Instance, error) {
	instanceArgs := make(map[string]*instanceArgument, len(args))
	for name, val := range args {
		switch v := val.(type) {
		case *Instance:
			instanceArgs[name] = &instanceArgument{val: v, typ: v.typ()}
		case *Function:
			instanceArgs[name] = &instanceArgument{val: v, typ: v.typ()}
		default:
			return nil, fmt.Errorf("unsupported argument type for %s: %T", name, val)
		}
	}
	return c.instantiate(ctx, instanceArgs, nil)
}

func (c *Component) instantiate(ctx context.Context, args map[string]*instanceArgument, parentScope *instanceScope) (*Instance, error) {
	instance := newInstance()
	instantiation := newInstanceScope(parentScope, c.scope, instance, c.runtime, args)

	instance.enter(ctx)
	defer instance.exit()

	coreInstanceDefs := defs(c.scope, sortCoreInstance)
	for i := range coreInstanceDefs.iterator() {
		def, _ := defs(c.scope, sortCoreInstance).get(uint32(i))
		switch def.(type) {
		case *coreInstantiateDefinition:
			_, err := resolve(ctx, instantiation, sortCoreInstance, uint32(i))
			if err != nil {
				return nil, err
			}
		}
	}

	instanceDefs := defs(c.scope, sortInstance)
	for i := range instanceDefs.iterator() {
		def, _ := defs(c.scope, sortInstance).get(uint32(i))
		switch def.(type) {
		case *instantiateDefinition:
			_, err := resolve(ctx, instantiation, sortInstance, uint32(i))
			if err != nil {
				return nil, err
			}
		}
	}

	for exportName, export := range c.exports {
		val, typ, err := export.resolve(ctx, instantiation)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate export %s: %v", exportName, err)
		}
		instance.exports[exportName] = val
		instance.exportTypes[exportName] = typ
	}

	return instance, nil
}

func (c *Component) typ() *componentType {
	exportTypes := make(map[string]Type, len(c.exports))
	for name, export := range c.exports {
		exportTypes[name] = export.typ()
	}
	return newComponentType(c.importTypes, exportTypes)
}

type componentExport interface {
	typ() Type
	resolve(ctx context.Context, scope *instanceScope) (any, Type, error)
}

type defComponentExport[T resolvedInstance[TT], TT Type] struct {
	sort       sort[T, TT]
	idx        uint32
	exportType TT
}

func newDefComponentExport[T resolvedInstance[TT], TT Type](sort sort[T, TT], idx uint32, typ TT) *defComponentExport[T, TT] {
	return &defComponentExport[T, TT]{sort: sort, idx: idx, exportType: typ}
}

func (e *defComponentExport[T, TT]) typ() Type {
	return e.exportType
}

func (e *defComponentExport[T, TT]) resolve(ctx context.Context, scope *instanceScope) (any, Type, error) {
	r, err := resolve(ctx, scope, e.sort, e.idx)
	if err != nil {
		return nil, nil, err
	}
	return r, r.typ(), nil
}

type componentStaticDefinition struct {
	component *Component
}

func newComponentStaticDefinition(component *Component) *componentStaticDefinition {
	return &componentStaticDefinition{
		component: component,
	}
}

func (d *componentStaticDefinition) typ() *componentType {
	return d.component.typ()
}

func (d *componentStaticDefinition) resolve(ctx context.Context, scope *instanceScope) (*Component, error) {
	return d.component, nil
}

type componentType struct {
	imports map[string]Type
	exports map[string]Type
}

func newComponentType(imports map[string]Type, exports map[string]Type) *componentType {
	return &componentType{
		imports: imports,
		exports: exports,
	}
}

func (ct *componentType) typ() Type {
	return ct
}

func (ct *componentType) assignableFrom(other Type) bool {
	otherCt, ok := other.(*componentType)
	if !ok {
		return false
	}
	// assignable if the other type does not have any imports that are not in this type
	for name, importType := range otherCt.imports {
		thisImportType, ok := ct.imports[name]
		if !ok || !importType.assignableFrom(thisImportType) {
			return false
		}
	}

	// and all exports in this type are present in the other type
	for name, exportType := range ct.exports {
		otherExportType, ok := otherCt.exports[name]
		if !ok || !exportType.assignableFrom(otherExportType) {
			return false
		}
	}
	return true
}
