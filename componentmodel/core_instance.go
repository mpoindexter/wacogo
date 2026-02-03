package componentmodel

import (
	"context"
	"fmt"
	"strings"

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

func newCoreInstance(module api.Module, additionalExterns *wasm.Externs) (*coreInstance, error) {
	exports := make(map[string]*coreExport)
	for name := range additionalExterns.Exports.Globals {
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

	for name, rawType := range additionalExterns.Exports.Tables {
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

	for name := range additionalExterns.Exports.Memories {

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

func (ci *coreInstance) getExport(name string) (any, error) {
	export, ok := ci.exports[name]
	if !ok {
		return nil, fmt.Errorf("core instance has no export named `%s`", name)
	}
	return export.val, nil
}

type coreInstanceType struct {
	exports map[string]Type
}

func newCoreInstanceType(exports map[string]Type) *coreInstanceType {
	return &coreInstanceType{
		exports: exports,
	}
}

func (c *coreInstanceType) isType() {}

func (t *coreInstanceType) typeName() string {
	return "core instance"
}

func (t *coreInstanceType) exportType(name string) (Type, bool) {
	typ, ok := t.exports[name]
	if !ok {
		return nil, false
	}
	return typ, ok
}

func (c *coreInstanceType) checkType(other Type, typeChecker typeChecker) error {
	oc, err := assertTypeKindIsSame(c, other)
	if err != nil {
		return err
	}
	for name, t := range c.exports {
		ot, ok := oc.exports[name]
		if !ok {
			return fmt.Errorf("type mismatch: missing export %s in core instance type", name)
		}
		if err := typeChecker.checkTypeCompatible(t, ot); err != nil {
			return fmt.Errorf("type mismatch in export `%s`: %w", name, err)
		}
	}
	return nil
}

func (t *coreInstanceType) typeSize() int {
	size := 1
	for _, exportType := range t.exports {
		size += exportType.typeSize()
	}
	return size
}

func (t *coreInstanceType) typeDepth() int {
	maxDepth := 0
	for _, exportType := range t.exports {
		depth := exportType.typeDepth()
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return 1 + maxDepth
}

type coreInstantiateDefinition struct {
	astDef *ast.CoreInstantiate
}

func newCoreInstantiateDefinition(
	astDef *ast.CoreInstantiate,
) *coreInstantiateDefinition {
	return &coreInstantiateDefinition{
		astDef: astDef,
	}
}

func (d *coreInstantiateDefinition) isDefinition() {}

func (d *coreInstantiateDefinition) createType(scope *scope) (*coreInstanceType, error) {
	modType, err := sortScopeFor(scope, sortCoreModule).getType(d.astDef.ModuleIdx)
	if err != nil {
		return nil, fmt.Errorf("unknown module type: %w", err)
	}

	args := make(map[string]uint32)

	requiredModuleNames := make(map[string]struct{})
	for importName := range modType.imports {
		requiredModuleNames[importName.module] = struct{}{}
	}

	for _, astArg := range d.astDef.Args {
		delete(requiredModuleNames, astArg.Name)
		if _, exists := args[astArg.Name]; exists {
			return nil, fmt.Errorf("duplicate module instantiation argument named `%s`", astArg.Name)
		}
		args[astArg.Name] = astArg.CoreInstanceIdx

		instanceType, err := sortScopeFor(scope, sortCoreInstance).getType(astArg.CoreInstanceIdx)
		if err != nil {
			return nil, err
		}

		typeChecker := newTypeChecker()
		for importName, importType := range modType.imports {
			if importName.module == astArg.Name {
				itemType, ok := instanceType.exportType(importName.name)
				if !ok {
					return nil, fmt.Errorf("module instantiation argument `%s` does not export an item named `%s`", astArg.Name, importName.name)
				}

				if err := typeChecker.checkTypeCompatible(importType, itemType); err != nil {
					return nil, fmt.Errorf("type mismatch: import `%s` in module %d is not assignable from export `%s` in core instance %d: %w", importName.name, d.astDef.ModuleIdx, importName.name, astArg.CoreInstanceIdx, err)
				}
			}
		}
	}

	if len(requiredModuleNames) > 0 {
		missingNames := make([]string, 0, len(requiredModuleNames))
		for name := range requiredModuleNames {
			missingNames = append(missingNames, name)
		}
		return nil, fmt.Errorf("missing module instantiation argument: %s", strings.Join(missingNames, ", "))
	}

	return newCoreInstanceType(modType.exports), nil
}

func (d *coreInstantiateDefinition) createInstance(ctx context.Context, scope *scope) (*coreInstance, error) {
	coreMod, err := sortScopeFor(scope, sortCoreModule).getInstance(d.astDef.ModuleIdx)
	if err != nil {
		return nil, err
	}

	// Build instantiation arguments
	importModules := make(map[string]api.Module)
	for _, arg := range d.astDef.Args {
		argInstance, err := sortScopeFor(scope, sortCoreInstance).getInstance(arg.CoreInstanceIdx)
		if err != nil {
			return nil, err
		}
		importModules[arg.Name] = argInstance.module
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

	return newCoreInstance(modInst, coreMod.additionalExterns)
}

type coreInlineExportsDefinition struct {
	astDef *ast.CoreInlineExports
}

func newCoreInlineExportsDefinition(
	astDef *ast.CoreInlineExports,
) *coreInlineExportsDefinition {
	return &coreInlineExportsDefinition{
		astDef: astDef,
	}
}

func (d *coreInlineExportsDefinition) isDefinition() {}

func (d *coreInlineExportsDefinition) createType(scope *scope) (*coreInstanceType, error) {

	exportTypes := make(map[string]Type)
	for _, export := range d.astDef.Exports {
		if _, exists := exportTypes[export.Name]; exists {
			return nil, fmt.Errorf("export name `%s` already defined", export.Name)
		}
		typ, err := typeForCoreSortIdx(scope, export.SortIdx)
		if err != nil {
			return nil, err
		}
		exportTypes[export.Name] = typ
	}

	return newCoreInstanceType(exportTypes), nil
}

func (d *coreInlineExportsDefinition) createInstance(ctx context.Context, scope *scope) (*coreInstance, error) {
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
	for _, astExport := range d.astDef.Exports {
		switch astExport.SortIdx.Sort {
		case ast.CoreSortFunc:
			fn, err := sortScopeFor(scope, sortCoreFunction).getInstance(astExport.SortIdx.Idx)
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
			mem, err := sortScopeFor(scope, sortCoreMemory).getInstance(astExport.SortIdx.Idx)
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
			table, err := sortScopeFor(scope, sortCoreTable).getInstance(astExport.SortIdx.Idx)
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
			global, err := sortScopeFor(scope, sortCoreGlobal).getInstance(astExport.SortIdx.Idx)
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

	return newCoreInstance(modInst, &wasm.Externs{
		Imports: &wasm.Imports{},
		Exports: additionalExports,
	})
}
