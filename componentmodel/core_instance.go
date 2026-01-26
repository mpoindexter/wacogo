package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

type coreExport struct {
	typ Type
	val any
}

type coreInstance struct {
	module  api.Module
	exports map[string]*coreExport
}

func newCoreInstance(module api.Module, additionalExports *wasm.Exports) (*coreInstance, error) {
	exports := make(map[string]*coreExport)
	for name := range additionalExports.Globals {
		global := module.ExportedGlobal(name)
		if global == nil {
			return nil, fmt.Errorf("exported global %s not found in module", name)
		}
		_, isMutable := global.(api.MutableGlobal)

		actualType := newCoreGlobalType(coreTypeWasmConstTypeFromWazero(global.Type()), isMutable)

		exports[name] = &coreExport{
			typ: actualType,
			val: newCoreGlobal(module, name, global),
		}
	}

	for name, rawType := range additionalExports.Tables {
		vt, err := coreTypeWasmConstTypeFromWasmParser(rawType.ElemType)
		if err != nil {
			return nil, fmt.Errorf("invalid expected table element type for %s: %w", name, err)
		}

		var max *uint32
		if rawType.Limits.HasMax {
			max = &rawType.Limits.Max
		}
		exports[name] = &coreExport{
			typ: newCoreTableType(
				vt,
				rawType.Limits.Min,
				max,
			),
			val: newCoreTable(module, name, rawType),
		}
	}

	for name := range additionalExports.Memories {

		memory := module.ExportedMemory(name)
		if memory == nil {
			return nil, fmt.Errorf("exported memory %s not found in module", name)
		}
		var actualMax *uint32
		if max, ok := memory.Definition().Max(); ok {
			actualMax = &max
		}
		actualType := newCoreMemoryType(memory.Definition().Min(), actualMax)

		exports[name] = &coreExport{
			typ: actualType,
			val: newCoreMemory(module, name, memory),
		}
	}

	for name, def := range module.ExportedFunctionDefinitions() {
		actualType := wazeroFunctionDefinitionToCoreFunctionType(def)
		exports[name] = &coreExport{
			typ: actualType,
			val: newCoreFunction(module, name, def),
		}
	}

	return &coreInstance{
		module:  module,
		exports: exports,
	}, nil
}

func (ci *coreInstance) typ() *coreInstanceType {
	exportTypes := make(map[string]Type)
	for name, export := range ci.exports {
		exportTypes[name] = export.typ
	}
	return newCoreInstanceType(exportTypes)
}

func (ci *coreInstance) getExport(name string) (any, Type, bool) {
	export, ok := ci.exports[name]
	if !ok {
		return nil, nil, false
	}
	return export.val, export.typ, true
}

type coreInstanceType struct {
	exports map[string]Type
}

func newCoreInstanceType(exports map[string]Type) *coreInstanceType {
	return &coreInstanceType{
		exports: exports,
	}
}

func (t *coreInstanceType) typ() Type {
	return t
}

func (t *coreInstanceType) exportType(name string) (Type, bool) {
	typ, ok := t.exports[name]
	if !ok {
		return nil, false
	}
	return typ, true
}

func (c *coreInstanceType) assignableFrom(other Type) bool {
	otherInstanceType, ok := other.(*coreInstanceType)
	if !ok {
		return false
	}
	for name, t := range c.exports {
		otherType, ok := otherInstanceType.exports[name]
		if !ok || !t.assignableFrom(otherType) {
			return false
		}
	}
	return true
}

type coreInstantiateDefinition struct {
	moduleIndex  uint32
	args         map[string]uint32
	instanceType *coreInstanceType
}

func newCoreInstantiateDefinition(
	moduleIndex uint32,
	args map[string]uint32,
	instanceType *coreInstanceType,
) *coreInstantiateDefinition {
	return &coreInstantiateDefinition{
		moduleIndex:  moduleIndex,
		args:         args,
		instanceType: instanceType,
	}
}

func (d *coreInstantiateDefinition) typ() *coreInstanceType {
	return d.instanceType
}

func (d *coreInstantiateDefinition) exportType(name string) (Type, error) {
	et, ok := d.instanceType.exports[name]
	if !ok {
		return nil, fmt.Errorf("core instance has no export named `%s`", name)
	}
	return et, nil
}

func (d *coreInstantiateDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreInstance, error) {
	coreModDef, err := defs(scope.definitionScope, sortCoreModule).get(d.moduleIndex)
	if err != nil {
		return nil, err
	}
	coreMod, err := coreModDef.resolve(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Build instantiation arguments
	importModules := make(map[string]api.Module)
	for name, coreInstanceIdx := range d.args {
		argInstance, err := resolve(ctx, scope, sortCoreInstance, coreInstanceIdx)
		if err != nil {
			return nil, err
		}
		importModules[name] = argInstance.module
	}

	cnf := wazero.NewModuleConfig().
		WithName("") // Always anonymous for core modules - this will get assigned during instantiation

	modCtx := experimental.WithImportResolver(ctx, experimental.ImportResolver(func(name string) api.Module {
		if name == "$$BLANK$$" {
			name = ""
		}
		return importModules[name]
	}))
	modInst, err := scope.runtime.InstantiateModule(modCtx, coreMod.module, cnf)
	if err != nil {
		return nil, err
	}

	return newCoreInstance(modInst, coreMod.additionalExports)
}

type coreInlineExportsDefinition struct {
	exports      []ast.CoreInlineExport
	instanceType *coreInstanceType
}

func newCoreInlineExportsDefinition(
	exports []ast.CoreInlineExport,
	instanceType *coreInstanceType,
) *coreInlineExportsDefinition {
	return &coreInlineExportsDefinition{
		exports:      exports,
		instanceType: instanceType,
	}
}

func (d *coreInlineExportsDefinition) typ() *coreInstanceType {
	return d.instanceType
}

func (d *coreInlineExportsDefinition) exportType(name string) (Type, error) {
	et, ok := d.instanceType.exports[name]
	if !ok {
		return nil, fmt.Errorf("core instance has no export named `%s`", name)
	}
	return et, nil
}

func (d *coreInlineExportsDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreInstance, error) {
	// We have to synthesize a module that imports the exported definitions and then reexports them
	modMap := make(map[string]api.Module)

	modBuilder := wasm.NewBuilder()

	var types wasm.TypeSection
	var imports wasm.ImportSection
	var exports wasm.ExportSection
	var funcs wasm.FuncSection
	var fnIdx uint32
	var memIdx uint32
	var tableIdx uint32
	var globalIdx uint32

	additionalExports := &wasm.Exports{
		Memories: make(map[string]*wasm.MemoryType),
		Tables:   make(map[string]*wasm.TableType),
		Globals:  make(map[string]*wasm.GlobalType),
	}
	for _, astExport := range d.exports {
		switch astExport.SortIdx.Sort {
		case ast.CoreSortFunc:
			fn, err := resolve(ctx, scope, sortCoreFunction, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = fn.module

			if err := types.AddFuncDef(fn.def); err != nil {
				return nil, err
			}

			fnTypeIdx := uint32(len(types.Types) - 1)

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   fn.name,
				ImportDesc: &wasm.FuncType{
					TypeIdx: fnTypeIdx,
				},
			})

			exports.Exports = append(exports.Exports, &wasm.Export{
				Name: astExport.Name,
				ExportDesc: &wasm.FuncExport{
					Idx: fnIdx,
				},
			})

			funcs.FuncTypeIndices = append(funcs.FuncTypeIndices, fnTypeIdx)
			fnIdx++
		case ast.CoreSortMemory:
			mem, err := resolve(ctx, scope, sortCoreMemory, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = mem.module

			min := mem.memory.Definition().Min()
			max, hasMax := mem.memory.Definition().Max()
			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   mem.name,
				ImportDesc: &wasm.MemoryType{
					Min:    min,
					Max:    max,
					HasMax: hasMax,
				},
			})

			exports.Exports = append(exports.Exports, &wasm.Export{
				Name: astExport.Name,
				ExportDesc: &wasm.MemoryExport{
					Idx: memIdx,
				},
			})
			additionalExports.Memories[astExport.Name] = &wasm.MemoryType{
				Min:    min,
				Max:    max,
				HasMax: hasMax,
			}
			memIdx++
		case ast.CoreSortTable:
			table, err := resolve(ctx, scope, sortCoreTable, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}

			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = table.module

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module:     modName,
				Name:       table.name,
				ImportDesc: table.def,
			})

			exports.Exports = append(exports.Exports, &wasm.Export{
				Name: astExport.Name,
				ExportDesc: &wasm.TableExport{
					Idx: tableIdx,
				},
			})

			additionalExports.Tables[astExport.Name] = table.def
			tableIdx++
		case ast.CoreSortGlobal:
			global, err := resolve(ctx, scope, sortCoreGlobal, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}

			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = global.module

			vt, err := wasm.WazeroValueTypeToValueType(global.global.Type())
			_, isMutable := global.global.(api.MutableGlobal)

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   global.name,
				ImportDesc: &wasm.GlobalType{
					ValType: vt,
					Mutable: isMutable,
				},
			})

			exports.Exports = append(exports.Exports, &wasm.Export{
				Name: astExport.Name,
				ExportDesc: &wasm.GlobalExport{
					Idx: globalIdx,
				},
			})

			additionalExports.Globals[astExport.Name] = &wasm.GlobalType{
				ValType: vt,
				Mutable: isMutable,
			}
			globalIdx++
		default:
			return nil, fmt.Errorf("unsupported core inline export type: %v", astExport.SortIdx.Sort)
		}
	}
	modBuilder.AddSection(&types)
	modBuilder.AddSection(&imports)
	modBuilder.AddSection(&exports)

	synthModule, err := modBuilder.Build()
	if err != nil {
		return nil, err
	}

	cnf := wazero.NewModuleConfig().
		WithName("") // Always anonymous for core modules - this will get assigned during instantiation

	modCtx := experimental.WithImportResolver(ctx, experimental.ImportResolver(func(name string) api.Module {
		return modMap[name]
	}))
	modInst, err := scope.runtime.InstantiateWithConfig(modCtx, synthModule, cnf)
	if err != nil {
		return nil, err
	}

	return newCoreInstance(modInst, additionalExports)
}
