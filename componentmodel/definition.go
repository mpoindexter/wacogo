package componentmodel

import (
	"context"
	"fmt"
	"iter"

	"github.com/partite-ai/wacogo/ast"
)

type definition[T any, TT Type] interface {
	typ() TT
	resolve(ctx context.Context, scope *instanceScope) (T, error)
}

type sort[T any, TT Type] int

func (s sort[T, TT]) typeName() string {
	switch ast.Sort(s) {
	case ast.SortCoreFunc:
		return "core function"
	case ast.SortCoreMemory:
		return "core memory"
	case ast.SortCoreTable:
		return "core table"
	case ast.SortCoreGlobal:
		return "core global"
	case ast.SortCoreType:
		return "core type"
	case ast.SortCoreModule:
		return "module"
	case ast.SortCoreInstance:
		return "core instance"
	case ast.SortFunc:
		return "function"
	case ast.SortType:
		return "type"
	case ast.SortComponent:
		return "component"
	case ast.SortInstance:
		return "instance"
	default:
		return "unknown"
	}
}

var sortCoreFunction sort[*coreFunction, *coreFunctionType] = sort[*coreFunction, *coreFunctionType](ast.SortCoreFunc)
var sortCoreMemory sort[*coreMemory, *coreMemoryType] = sort[*coreMemory, *coreMemoryType](ast.SortCoreMemory)
var sortCoreTable sort[*coreTable, *coreTableType] = sort[*coreTable, *coreTableType](ast.SortCoreTable)
var sortCoreGlobal sort[*coreGlobal, *coreGlobalType] = sort[*coreGlobal, *coreGlobalType](ast.SortCoreGlobal)
var sortCoreType sort[Type, Type] = sort[Type, Type](ast.SortCoreType)
var sortCoreModule sort[*coreModule, *coreModuleType] = sort[*coreModule, *coreModuleType](ast.SortCoreModule)
var sortCoreInstance sort[*coreInstance, *coreInstanceType] = sort[*coreInstance, *coreInstanceType](ast.SortCoreInstance)
var sortFunction sort[*Function, *FunctionType] = sort[*Function, *FunctionType](ast.SortFunc)
var sortType sort[Type, Type] = sort[Type, Type](ast.SortType)
var sortComponent sort[*Component, *componentType] = sort[*Component, *componentType](ast.SortComponent)
var sortInstance sort[*Instance, *instanceType] = sort[*Instance, *instanceType](ast.SortInstance)

type definitionScope struct {
	parent *definitionScope
	defs   map[any]any
}

func newDefinitionScope(parent *definitionScope) *definitionScope {
	return &definitionScope{
		parent: parent,
		defs:   make(map[any]any),
	}
}

type definitions[T any, TT Type] interface {
	get(index uint32) (definition[T, TT], error)
	add(def definition[T, TT]) uint32
	len() int
	iterator() iter.Seq2[int, definition[T, TT]]
}

type definitionsImpl[T any, TT Type] struct {
	defs []definition[T, TT]
	sort sort[T, TT]
}

func (d *definitionsImpl[T, TT]) get(index uint32) (definition[T, TT], error) {
	if int(index) >= len(d.defs) {
		return nil, fmt.Errorf("%s index out of bounds", d.sort.typeName())
	}
	return d.defs[index], nil
}

func (d *definitionsImpl[T, TT]) add(def definition[T, TT]) uint32 {
	d.defs = append(d.defs, def)
	return uint32(len(d.defs) - 1)
}

func (d *definitionsImpl[T, TT]) iterator() iter.Seq2[int, definition[T, TT]] {
	return func(yield func(int, definition[T, TT]) bool) {
		for i, def := range d.defs {
			if !yield(i, def) {
				return
			}
		}
	}
}

func (d *definitionsImpl[T, TT]) len() int {
	return len(d.defs)
}

func defs[T any, TT Type](scope *definitionScope, sort sort[T, TT]) definitions[T, TT] {
	defs, ok := scope.defs[sort]
	if !ok {
		newDefs := &definitionsImpl[T, TT]{
			sort: sort,
		}
		scope.defs[sort] = newDefs
		return newDefs
	}
	return defs.(definitions[T, TT])
}

func nestedDefs[T any, TT Type](scope *definitionScope, sort sort[T, TT], nestingLevel uint32) (definitions[T, TT], error) {
	targetScope := scope
	for range nestingLevel {
		if targetScope.parent == nil {
			return nil, fmt.Errorf("invalid outer alias count of %d", nestingLevel)
		}
		targetScope = targetScope.parent
	}
	return defs(targetScope, sort), nil
}

func coreSortIdxDef(scope *definitionScope, sortIdx ast.CoreSortIdx) (any, Type, error) {
	switch sortIdx.Sort {
	case ast.CoreSortFunc:
		def, err := defs(scope, sortCoreFunction).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortGlobal:
		def, err := defs(scope, sortCoreGlobal).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortMemory:
		def, err := defs(scope, sortCoreMemory).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortTable:
		def, err := defs(scope, sortCoreTable).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortType:
		def, err := defs(scope, sortCoreType).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortInstance:
		def, err := defs(scope, sortCoreInstance).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.CoreSortModule:
		def, err := defs(scope, sortCoreModule).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported core export sort: %v", sortIdx.Sort)
	}
}

func sortIdxDef(scope *definitionScope, sortIdx *ast.SortIdx) (any, Type, error) {
	switch sortIdx.Sort {
	case ast.SortCoreFunc, ast.SortCoreGlobal, ast.SortCoreMemory, ast.SortCoreTable, ast.SortCoreType, ast.SortCoreInstance, ast.SortCoreModule:
		return coreSortIdxDef(scope, ast.CoreSortIdx{
			Sort: ast.CoreSort(sortIdx.Sort),
			Idx:  sortIdx.Idx,
		})
	case ast.SortFunc:
		def, err := defs(scope, sortFunction).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.SortType:
		def, err := defs(scope, sortType).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.SortComponent:
		def, err := defs(scope, sortComponent).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	case ast.SortInstance:
		def, err := defs(scope, sortInstance).get(sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return def, def.typ(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported sort: %v", sortIdx.Sort)
	}
}

type importDefinition[T resolvedInstance[TT], TT Type] struct {
	importName string
	importType TT
}

func newImportDefinition[T resolvedInstance[TT], TT Type](importName string, importType TT) *importDefinition[T, TT] {
	return &importDefinition[T, TT]{
		importName: importName,
		importType: importType,
	}
}

func (d *importDefinition[T, TT]) typ() TT {
	return d.importType
}

func (d *importDefinition[T, TT]) resolve(ctx context.Context, scope *instanceScope) (T, error) {
	importedValue, importedType, err := scope.resolveArgument(d.importName)
	if err != nil {
		return *new(T), err
	}
	if !d.importType.assignableFrom(importedType) {
		return *new(T), fmt.Errorf("import %s is not of expected type", d.importName)
	}
	typedVal, ok := importedValue.(T)
	if !ok {
		return *new(T), fmt.Errorf("export %s in import instance is not of expected type", d.importName)
	}
	return typedVal, nil
}
