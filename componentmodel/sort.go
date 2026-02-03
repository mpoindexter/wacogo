package componentmodel

import (
	"github.com/partite-ai/wacogo/ast"
)

type sort[V any, T Type] int

func (s sort[V, T]) typeName() string {
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

func (s sort[V, T]) addImport(defs *definitions, name string, typ typeResolver) (typeResolver, error) {
	idx := sortDefsFor(defs, s).add(newImportDefinition(s, name, typ))
	return newIndexTypeResolverOf[T](s, idx, ""), nil
}

func (s sort[V, T]) addTypeOnlyExportDefinition(scope *definitions, typResolver typeResolver) (componentExport, error) {
	idx := sortDefsFor(scope, s).add(newTypeOnlyDefinition[V, T](typResolver))
	return newDefComponentExport(s, idx), nil
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

const numSorts = 11

type genericSort interface {
	typeName() string
	addImport(defs *definitions, name string, typ typeResolver) (typeResolver, error)
	addTypeOnlyExportDefinition(defs *definitions, typResolver typeResolver) (componentExport, error)
}
