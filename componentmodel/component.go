package componentmodel

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

type Component struct {
	id             string
	definitions    *definitions
	componentScope *scope
	importTypes    map[string]typeResolver
	exports        map[string]componentExport
}

func newComponent(id string, runtime wazero.Runtime, definitions *definitions, scope *scope, imports map[string]typeResolver, exports map[string]componentExport) (*Component, error) {
	typeScope := sortScopeFor(scope, sortType)
	for idx := range typeScope.items {
		t, err := typeScope.getType(uint32(idx))
		if err != nil {
			return nil, err
		}
		if s := t.typeSize(); s > maxTypeSize {
			return nil, fmt.Errorf("effective type size exceeds the limit")
		}
	}

	return &Component{
		id:             id,
		definitions:    definitions,
		exports:        exports,
		importTypes:    imports,
		componentScope: scope,
	}, nil
}

func (c *Component) Instantiate(ctx context.Context, args map[string]any) (*Instance, error) {
	instanceArgs := make(map[string]*instanceArgument, len(args))
	for name, val := range args {
		switch v := val.(type) {
		case *Instance:
			instanceArgs[name] = &instanceArgument{val: v, typ: newInstanceType(v.exportSpecs, nil)}
		case *Function:
			instanceArgs[name] = &instanceArgument{val: v, typ: v.funcTyp}
		default:
			return nil, fmt.Errorf("unsupported argument type for %s: %T", name, val)
		}
	}
	return c.instantiate(ctx, instanceArgs)
}

func (c *Component) instantiate(ctx context.Context, args map[string]*instanceArgument) (*Instance, error) {
	instance := newInstance()
	instanceScope := c.componentScope.instanceScope(instance, args)

	instance.enter(ctx)
	defer instance.exit()

	for _, def := range c.definitions.binders {
		if err := def.bindInstance(ctx, instanceScope); err != nil {
			return nil, err
		}
	}

	for exportName, export := range c.exports {
		typ, err := export.typ(instanceScope)
		if err != nil {
			return nil, fmt.Errorf("failed to get type for export %s: %v", exportName, err)
		}
		val, err := export.resolve(instanceScope)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate export %s: %v", exportName, err)
		}

		instance.exports[exportName] = val
		instance.exportSpecs[exportName] = &exportSpec{typ: typ, sort: export.sort()}
	}

	return instance, nil
}

func (c *Component) clone(newParentScope *scope) (*Component, error) {
	placeholders := make(map[string]Type)
	for name := range c.importTypes {
		placeholders[name] = importPlaceholderType{}
	}
	componentScope := newParentScope.componentScope(placeholders)
	for _, binder := range c.definitions.binders {
		if err := binder.bindType(componentScope); err != nil {
			return nil, err
		}
	}

	return &Component{
		id:             c.id,
		definitions:    c.definitions,
		componentScope: componentScope,
		importTypes:    c.importTypes,
		exports:        c.exports,
	}, nil
}

type exportSpec struct {
	typ  Type
	sort genericSort
}

type componentExport interface {
	typ(scope *scope) (Type, error)
	resolve(scope *scope) (any, error)
	sort() genericSort
}

type defComponentExport[V any, T Type] struct {
	exportedSort sort[V, T]
	idx          uint32
}

func newDefComponentExport[V any, T Type](sort sort[V, T], idx uint32) *defComponentExport[V, T] {
	return &defComponentExport[V, T]{exportedSort: sort, idx: idx}
}

func (e *defComponentExport[V, T]) typ(scope *scope) (Type, error) {
	return sortScopeFor(scope, e.exportedSort).getType(e.idx)
}

func (e *defComponentExport[V, T]) resolve(scope *scope) (any, error) {
	r, err := sortScopeFor(scope, e.exportedSort).getInstance(e.idx)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (e *defComponentExport[V, T]) sort() genericSort {
	return e.exportedSort
}

type componentDefinition struct {
	comp *Component
}

func newComponentDefinition(comp *Component) *componentDefinition {
	return &componentDefinition{comp: comp}
}

func (d *componentDefinition) isDefinition() {}

func (d *componentDefinition) createType(scope *scope) (*componentType, error) {
	clone, err := d.comp.clone(scope)
	if err != nil {
		return nil, err
	}
	importTypes := make(map[string]Type, len(clone.importTypes))
	for name, importDef := range clone.importTypes {
		t, err := importDef.resolveType(clone.componentScope)
		if err != nil {
			return nil, err
		}
		importTypes[name] = t
	}
	exportTypes := make(map[string]Type, len(clone.exports))
	for name, export := range clone.exports {
		typ, err := export.typ(clone.componentScope)
		if err != nil {
			return nil, err
		}
		exportTypes[name] = typ
	}
	ct := newComponentType(importTypes, exportTypes, clone)
	if ct.typeSize() > maxTypeSize {
		return nil, fmt.Errorf("effective type size exceeds the limit")
	}
	return ct, nil
}

func (d *componentDefinition) createInstance(ctx context.Context, scope *scope) (*Component, error) {
	ct := scope.currentType.(*componentType)
	return ct.component, nil
}

type componentType struct {
	imports   map[string]Type
	exports   map[string]Type
	component *Component
}

func newComponentType(imports map[string]Type, exports map[string]Type, component *Component) *componentType {
	return &componentType{
		imports:   imports,
		exports:   exports,
		component: component,
	}
}

func (ct *componentType) isType() {}

func (ct *componentType) typeName() string {
	return "component"
}

func (ct *componentType) instanceType(scope *scope, args map[string]Type) (*instanceType, error) {
	instantiateArgs := make(map[string]*instanceArgument)
	for name, typ := range args {
		instantiateArgs[name] = &instanceArgument{typ: typ}
	}
	instanceScope := ct.component.componentScope.instanceScope(nil, instantiateArgs)
	for _, binder := range ct.component.definitions.binders {
		if err := binder.bindType(instanceScope); err != nil {
			return nil, err
		}
	}
	exportSpecs := make(map[string]*exportSpec, len(ct.component.exports))
	for name, export := range ct.component.exports {
		typ, err := export.typ(instanceScope)
		if err != nil {
			return nil, err
		}
		exportSpecs[name] = &exportSpec{typ: typ, sort: export.sort()}
	}

	return newInstanceType(exportSpecs, nil), nil
}

func (ct *componentType) checkType(other Type, typeChecker typeChecker) error {
	otherCt, err := assertTypeKindIsSame(ct, other)
	if err != nil {
		return err
	}

	// if other type has imports that are not in this type, it's a mismatch
	for name, importType := range otherCt.imports {
		thisImportType, ok := ct.imports[name]
		if !ok {
			return fmt.Errorf("type mismatch: extra import %s in component type", name)
		}
		if err := typeChecker.checkTypeCompatible(importType, thisImportType); err != nil {
			return fmt.Errorf("type mismatch in import `%s`: %w", name, err)
		}
	}

	// if this type has exports that are not in the other type, it's a mismatch
	for name, exportType := range ct.exports {
		otherExportType, ok := otherCt.exports[name]
		if !ok {
			return fmt.Errorf("type mismatch: missing export %s in component type", name)
		}
		if err := typeChecker.checkTypeCompatible(exportType, otherExportType); err != nil {
			return fmt.Errorf("type mismatch in export `%s`: %v", name, err)
		}
	}
	return nil
}

func (ct *componentType) typeSize() int {
	size := 1
	for _, importType := range ct.imports {
		size += importType.typeSize()
	}
	for _, exportType := range ct.exports {
		size += exportType.typeSize()
	}
	return size
}

func (ct *componentType) typeDepth() int {
	maxDepth := 0
	for _, importType := range ct.imports {
		if d := importType.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	for _, exportType := range ct.exports {
		if d := exportType.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	return 1 + maxDepth
}

type componentTypeResolver struct {
	definitions *definitions
	imports     map[string]typeResolver
	exports     map[string]componentExport
}

func newComponentTypeResolver(definitions *definitions, imports map[string]typeResolver, exports map[string]componentExport) *componentTypeResolver {
	return &componentTypeResolver{
		definitions: definitions,
		imports:     imports,
		exports:     exports,
	}
}

func (r *componentTypeResolver) resolveType(scope *scope) (Type, error) {
	placeholderTypes := make(map[string]Type)
	for name := range r.imports {
		placeholderTypes[name] = importPlaceholderType{}
	}
	compScope := scope.componentScope(placeholderTypes)
	for _, binder := range r.definitions.binders {
		if err := binder.bindType(compScope); err != nil {
			return nil, err
		}
	}

	aliasResources := make(map[*ResourceType]struct{})
	for i, typ := range sortScopeFor(compScope, sortType).items {
		def, err := sortDefsFor(r.definitions, sortType).get(uint32(i))
		if err != nil {
			return nil, err
		}
		rt, isResourceType := typ.getType().(*ResourceType)
		if _, exportAlias := def.(*exportAliasDefinition[Type, Type, *Instance, *instanceType]); isResourceType && exportAlias {
			aliasResources[rt] = struct{}{}
			continue
		}

		if outerAlias, isOuterAlias := def.(*outerAliasDefinition[Type, Type]); isResourceType && isOuterAlias {
			if outerAlias.outerIdx > 0 {
				aliasResources[rt] = struct{}{}
			}
			continue
		}
	}

	importTypes := make(map[string]Type)
	for name, typ := range r.imports {
		typ, err := typ.resolveType(compScope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve component import type: %w", err)
		}
		importTypes[name] = typ
	}

	exportTypes := make(map[string]Type)
	for name, export := range r.exports {
		typ, err := export.typ(compScope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve component export type: %w", err)
		}
		exportTypes[name] = typ
	}

	comp := &Component{
		definitions:    r.definitions,
		componentScope: compScope,
		importTypes:    r.imports,
		exports:        r.exports,
	}
	ct := newComponentType(importTypes, exportTypes, comp)
	if ct.typeSize() > maxTypeSize {
		return nil, fmt.Errorf("effective type size exceeds the limit")
	}
	return ct, nil
}

func (r *componentTypeResolver) typeInfo(scope *scope) *typeInfo {
	size := 1
	for _, importType := range r.imports {
		size += importType.typeInfo(scope).size
	}
	for _, export := range r.exports {
		typ, err := export.typ(scope)
		if err != nil {
			// Handle error appropriately, possibly returning nil or logging
			continue
		}
		size += typ.typeSize()
	}
	return &typeInfo{
		typeName: "component",
		size:     size,
	}
}
