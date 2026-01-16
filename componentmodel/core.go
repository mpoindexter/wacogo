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

type coreFunctionDefinition interface {
	resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error)
}

type coreFunctionExportDefinition struct {
	instanceIdx uint32
	funcName    string
}

func (d *coreFunctionExportDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	targetInstance, err := scope.resolveCoreInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, "", nil, err
	}
	fn := targetInstance.module.ExportedFunction(d.funcName)
	if fn == nil {
		return nil, "", nil, fmt.Errorf("function %s not found in core instance %d", d.funcName, d.instanceIdx)
	}
	return targetInstance.module, d.funcName, fn.Definition(), nil
}

type coreFunctionHostDefinition struct {
	mod    api.Module
	fnName string
	def    api.FunctionDefinition
}

func (d *coreFunctionHostDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	return d.mod, d.fnName, d.def, nil
}

type coreMemoryDefinition interface {
	resolveMemory(ctx context.Context, scope instanceScope) (api.Module, string, api.Memory, error)
}

type coreInstanceMemoryExportDefinition struct {
	instanceIdx uint32
	memoryName  string
}

func (d *coreInstanceMemoryExportDefinition) resolveMemory(ctx context.Context, scope instanceScope) (api.Module, string, api.Memory, error) {
	targetInstance, err := scope.resolveCoreInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, "", nil, err
	}
	mem := targetInstance.module.ExportedMemory(d.memoryName)
	if mem == nil {
		return nil, "", nil, fmt.Errorf("memory %s not found in core instance %d", d.memoryName, d.instanceIdx)
	}
	return targetInstance.module, d.memoryName, mem, nil
}

type coreGlobalDefinition interface {
	resolveGlobal(ctx context.Context, scope instanceScope) (api.Module, string, api.Global, error)
}

type coreInstanceGlobalExportDefinition struct {
	instanceIdx uint32
	globalName  string
}

func (d *coreInstanceGlobalExportDefinition) resolveGlobal(ctx context.Context, scope instanceScope) (api.Module, string, api.Global, error) {
	targetInstance, err := scope.resolveCoreInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, "", nil, err
	}
	glob := targetInstance.module.ExportedGlobal(d.globalName)
	if glob == nil {
		return nil, "", nil, fmt.Errorf("global %s not found in core instance %d", d.globalName, d.instanceIdx)
	}
	return targetInstance.module, d.globalName, glob, nil
}

type coreTableDefinition interface {
	resolveTable(ctx context.Context, scope instanceScope) (api.Module, string, *wasm.TableType, error)
}

type coreInstanceTableExportDefinition struct {
	instanceIdx uint32
	tableName   string
}

func (d *coreInstanceTableExportDefinition) resolveTable(ctx context.Context, scope instanceScope) (api.Module, string, *wasm.TableType, error) {
	targetInstance, err := scope.resolveCoreInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, "", nil, err
	}
	tt, ok := targetInstance.exportedTables[d.tableName]
	if !ok {
		return nil, "", nil, fmt.Errorf("table %s not found in core instance %d", d.tableName, d.instanceIdx)
	}
	return targetInstance.module, d.tableName, tt, nil
}

type coreModule struct {
	module     wazero.CompiledModule
	tableTypes map[string]*wasm.TableType
}

type coreModuleDefinition interface {
	resolveCoreModule(ctx context.Context, scope instanceScope) (*coreModule, error)
}

type coreModuleStaticDefinition struct {
	coreModule *coreModule
}

func (d *coreModuleStaticDefinition) resolveCoreModule(ctx context.Context, scope instanceScope) (*coreModule, error) {
	return d.coreModule, nil
}

type coreModuleInstanceExportAliasDefinition struct {
	instanceIdx uint32
	exportName  string
}

func (d *coreModuleInstanceExportAliasDefinition) resolveCoreModule(ctx context.Context, scope instanceScope) (*coreModule, error) {
	aliasInst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}
	compVal, ok := aliasInst.exports[d.exportName]
	if !ok {
		return nil, fmt.Errorf("export %s not found in instance %d", d.exportName, d.instanceIdx)
	}

	coreMod, ok := compVal.(*coreModule)
	if !ok {
		return nil, fmt.Errorf("export %s in instance %d is not a core module", d.exportName, d.instanceIdx)
	}

	return coreMod, nil
}

type coreModuleImportDefinition struct {
	name            string
	expectedTypeDef coreTypeDefinition
}

func (d *coreModuleImportDefinition) resolveCoreModule(ctx context.Context, scope instanceScope) (*coreModule, error) {
	val, err := scope.resolveArgument(d.name)
	if err != nil {
		return nil, err
	}
	coreMod, ok := val.(*coreModule)
	if !ok {
		return nil, fmt.Errorf("import %s is not a core module", d.name)
	}

	expectedType, err := d.expectedTypeDef.resolveCoreType(ctx, scope)
	if err != nil {
		return nil, err
	}

	modType, ok := expectedType.(*coreModuleType)
	if !ok {
		return nil, fmt.Errorf("expected type for core module import %s is not a core module type", d.name)
	}

	if err := modType.validateModule(coreMod); err != nil {
		return nil, fmt.Errorf("core module import %s does not match expected type: %w", d.name, err)
	}

	return coreMod, nil
}

type coreInstanceDefinition interface {
	resolveCoreInstance(ctx context.Context, scope instanceScope) (*coreInstance, error)
}

type coreInstantiateDefinition struct {
	moduleIndex uint32
	args        map[string]uint32
	instanceIdx uint32
}

func (d *coreInstantiateDefinition) resolveCoreInstance(ctx context.Context, scope instanceScope) (*coreInstance, error) {
	coreModDef, err := scope.resolveCoreModuleDefinition(0, d.moduleIndex)
	if err != nil {
		return nil, err
	}
	coreMod, err := coreModDef.resolveCoreModule(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Build instantiation arguments
	importModules := make(map[string]api.Module)
	for name, coreInstanceIdx := range d.args {
		argInstance, err := scope.resolveCoreInstance(ctx, coreInstanceIdx)
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
	modInst, err := scope.runtime().InstantiateModule(modCtx, coreMod.module, cnf)
	if err != nil {
		return nil, err
	}

	return &coreInstance{
		module:         modInst,
		exportedTables: coreMod.tableTypes,
	}, nil
}

type coreInlineExportsDefinition struct {
	exports     []ast.CoreInlineExport
	instanceIdx uint32
}

func (d *coreInlineExportsDefinition) resolveCoreInstance(ctx context.Context, scope instanceScope) (*coreInstance, error) {
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
	for _, astExport := range d.exports {
		switch astExport.SortIdx.Sort {
		case ast.CoreSortFunc:
			fnDef, err := scope.resolveCoreFunctionDefinition(0, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}
			mod, fnName, fnDecl, err := fnDef.resolveCoreFunction(ctx, scope)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = mod

			if err := types.AddFuncDef(fnDecl); err != nil {
				return nil, err
			}

			fnTypeIdx := uint32(len(types.Types) - 1)

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   fnName,
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
			memDef, err := scope.resolveCoreMemoryDefinition(0, astExport.SortIdx.Idx)
			mod, memName, mem, err := memDef.resolveMemory(ctx, scope)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = mod

			min := mem.Definition().Min()
			max, hasMax := mem.Definition().Max()
			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   memName,
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
			memIdx++
		case ast.CoreSortTable:
			tableDef, err := scope.resolveCoreTableDefinition(0, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}
			mod, tableName, tableType, err := tableDef.resolveTable(ctx, scope)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = mod

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module:     modName,
				Name:       tableName,
				ImportDesc: tableType,
			})

			exports.Exports = append(exports.Exports, &wasm.Export{
				Name: astExport.Name,
				ExportDesc: &wasm.TableExport{
					Idx: tableIdx,
				},
			})
			tableIdx++
		case ast.CoreSortGlobal:
			globalDef, err := scope.resolveCoreGlobalDefinition(0, astExport.SortIdx.Idx)
			if err != nil {
				return nil, err
			}
			mod, globalName, global, err := globalDef.resolveGlobal(ctx, scope)
			if err != nil {
				return nil, err
			}
			modName := fmt.Sprintf("_%v", len(modMap))
			modMap[modName] = mod

			vt, err := wasm.WazeroValueTypeToValueType(global.Type())
			_, isMutable := global.(api.MutableGlobal)

			imports.Imports = append(imports.Imports, &wasm.Import{
				Module: modName,
				Name:   globalName,
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
	modInst, err := scope.runtime().InstantiateWithConfig(modCtx, synthModule, cnf)
	if err != nil {
		return nil, err
	}

	return &coreInstance{
		module: modInst,
	}, nil
}

type coreInstanceAliasDefinition struct {
	instanceIdx uint32
	exportName  string
}

func (d *coreInstanceAliasDefinition) resolveCoreInstance(ctx context.Context, scope instanceScope) (*coreInstance, error) {
	aliasInst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}
	compVal, ok := aliasInst.exports[d.exportName]
	if !ok {
		return nil, fmt.Errorf("export %s not found in instance %d", d.exportName, d.instanceIdx)
	}

	coreInst, ok := compVal.(*coreInstance)
	if !ok {
		return nil, fmt.Errorf("export %s in instance %d is not a core instance", d.exportName, d.instanceIdx)
	}

	return coreInst, nil
}

type coreInstance struct {
	module         api.Module
	exportedTables map[string]*wasm.TableType
}
