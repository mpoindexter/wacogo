package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

type coreModule struct {
	module            wazero.CompiledModule
	additionalExterns *wasm.Externs
}

func newCoreModule(module wazero.CompiledModule, additionalExterns *wasm.Externs) *coreModule {
	return &coreModule{
		module:            module,
		additionalExterns: additionalExterns,
	}
}

func (m *coreModule) typ() *coreModuleType {
	imports := make(map[moduleName]Type)
	exports := make(map[string]Type)

	for _, importDef := range m.module.ImportedFunctions() {
		modName, name, _ := importDef.Import()
		imports[moduleName{
			module: modName,
			name:   name,
		}] = wazeroFunctionDefinitionToCoreFunctionType(importDef)
	}

	for name, importDef := range m.additionalExterns.Imports.Tables {
		imports[moduleName{
			module: name.Module,
			name:   name.Name,
		}] = wasmTableTypeToCoreTableType(importDef)
	}

	for name, importDef := range m.additionalExterns.Imports.Globals {
		vt, _ := coreTypeWasmConstTypeFromWasmParser(importDef.ValType)
		imports[moduleName{
			module: name.Module,
			name:   name.Name,
		}] = newCoreGlobalType(vt, importDef.Mutable)
	}

	for name, importDef := range m.additionalExterns.Imports.Memories {
		imports[moduleName{
			module: name.Module,
			name:   name.Name,
		}] = wasmMemoryTypeToCoreMemoryType(importDef)
	}

	for name, exportDef := range m.module.ExportedFunctions() {
		exports[name] = wazeroFunctionDefinitionToCoreFunctionType(exportDef)
	}

	for name, exportDef := range m.additionalExterns.Exports.Tables {
		exports[name] = wasmTableTypeToCoreTableType(exportDef)
	}

	for name, exportDef := range m.additionalExterns.Exports.Globals {
		vt, _ := coreTypeWasmConstTypeFromWasmParser(exportDef.ValType)
		exports[name] = newCoreGlobalType(vt, exportDef.Mutable)
	}

	for name, exportDef := range m.additionalExterns.Exports.Memories {
		exports[name] = wasmMemoryTypeToCoreMemoryType(exportDef)
	}

	return newCoreModuleType(imports, exports)
}

type moduleName struct {
	module string
	name   string
}

type coreModuleTypeDefinition struct {
	imports map[moduleName]typeResolver
	exports map[string]typeResolver
	defs    *definitions
}

func newCoreModuleTypeDefinition(defs *definitions, imports map[moduleName]typeResolver, exports map[string]typeResolver) *coreModuleTypeDefinition {
	return &coreModuleTypeDefinition{
		imports: imports,
		exports: exports,
		defs:    defs,
	}
}

func (d *coreModuleTypeDefinition) isDefinition() {}

func (d *coreModuleTypeDefinition) createType(scope *scope) (Type, error) {
	placeholderArgs := make(map[string]Type)
	for name := range d.imports {
		placeholderArgs[name.name] = importPlaceholderType{}
	}
	modTypeScope := scope.componentScope(placeholderArgs)
	for _, binder := range d.defs.binders {
		if err := binder.bindType(modTypeScope); err != nil {
			return nil, err
		}
	}
	imports := make(map[moduleName]Type)
	for name, importDef := range d.imports {
		t, err := importDef.resolveType(modTypeScope)
		if err != nil {
			return nil, err
		}
		imports[name] = t
	}
	exports := make(map[string]Type)
	for name, exportDef := range d.exports {
		t, err := exportDef.resolveType(modTypeScope)
		if err != nil {
			return nil, err
		}
		exports[name] = t
	}
	return newCoreModuleType(imports, exports), nil
}

func (d *coreModuleTypeDefinition) createInstance(ctx context.Context, scope *scope) (Type, error) {
	return scope.currentType, nil
}

type coreModuleType struct {
	imports map[moduleName]Type
	exports map[string]Type
}

func newCoreModuleType(imports map[moduleName]Type, exports map[string]Type) *coreModuleType {
	return &coreModuleType{
		imports: imports,
		exports: exports,
	}
}

func (c *coreModuleType) isType() {}

func (c *coreModuleType) typeName() string {
	return "module"
}

func (c *coreModuleType) checkType(other Type, typeChecker typeChecker) error {
	oc, err := assertTypeKindIsSame(c, other)
	if err != nil {
		return err
	}

	// Assignable if other imports are a subset of this imports
	for name, oit := range oc.imports {
		it, ok := c.imports[name]
		if !ok {
			return fmt.Errorf("type mismatch: missing expected import `%s::%s` in core module type", name.module, name.name)
		}
		if err := typeChecker.checkTypeCompatible(oit, it); err != nil {
			return fmt.Errorf("type mismatch in import `%s::%s`: %w", name.module, name.name, err)
		}
	}

	// Assignable if this exports are a subset of other exports
	for name, et := range c.exports {
		oet, ok := oc.exports[name]
		if !ok {
			return fmt.Errorf("type mismatch: missing expected export `%s` in core module type", name)
		}

		if err := typeChecker.checkTypeCompatible(et, oet); err != nil {
			return fmt.Errorf("type mismatch in export `%s`: %w", name, err)
		}
	}
	return nil
}

func (c *coreModuleType) typeSize() int {
	size := 1
	for _, importType := range c.imports {
		size += importType.typeSize()
	}
	for _, exportType := range c.exports {
		size += exportType.typeSize()
	}
	return size
}

func (c *coreModuleType) typeDepth() int {
	maxDepth := 0
	for _, importType := range c.imports {
		depth := importType.typeDepth()
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	for _, exportType := range c.exports {
		depth := exportType.typeDepth()
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return 1 + maxDepth
}
