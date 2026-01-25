package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

type coreModule struct {
	module            wazero.CompiledModule
	additionalExports *wasm.Exports
}

func newCoreModule(module wazero.CompiledModule, additionalExports *wasm.Exports) *coreModule {
	return &coreModule{
		module:            module,
		additionalExports: additionalExports,
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

	for _, importDef := range m.module.ImportedMemories() {
		modName, name, _ := importDef.Import()
		imports[moduleName{
			module: modName,
			name:   name,
		}] = wazeroMemoryDefinitionToCoreMemoryType(importDef)
	}

	for name, exportDef := range m.module.ExportedFunctions() {
		exports[name] = wazeroFunctionDefinitionToCoreFunctionType(exportDef)
	}

	for name, exportDef := range m.module.ExportedMemories() {
		exports[name] = wazeroMemoryDefinitionToCoreMemoryType(exportDef)
	}

	for name, exportDef := range m.additionalExports.Tables {
		exports[name] = wasmTableTypeToCoreTableType(exportDef)
	}

	for name, exportDef := range m.additionalExports.Globals {
		vt, _ := coreTypeWasmConstTypeFromWasmParser(exportDef.ValType)
		exports[name] = newCoreGlobalType(vt, exportDef.Mutable)
	}

	for name, exportDef := range m.additionalExports.Memories {
		exports[name] = wasmMemoryTypeToCoreMemoryType(exportDef)
	}

	return newCoreModuleType(imports, exports)
}

type coreModuleStaticDefinition struct {
	coreModule *coreModule
}

func newCoreModuleStaticDefinition(coreMod *coreModule) *coreModuleStaticDefinition {
	return &coreModuleStaticDefinition{
		coreModule: coreMod,
	}
}

func (d *coreModuleStaticDefinition) typ() *coreModuleType {
	return d.coreModule.typ()
}

func (d *coreModuleStaticDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreModule, error) {
	return d.coreModule, nil
}

type moduleName struct {
	module string
	name   string
}

type coreModuleTypeDefinition struct {
	imports map[moduleName]definition[Type, Type]
	exports map[string]definition[Type, Type]
}

func newCoreModuleTypeDefinition(imports map[moduleName]definition[Type, Type], exports map[string]definition[Type, Type]) *coreModuleTypeDefinition {
	return &coreModuleTypeDefinition{
		imports: imports,
		exports: exports,
	}
}

func (d *coreModuleTypeDefinition) typ() Type {
	imports := make(map[moduleName]Type)
	for name, importDef := range d.imports {
		imports[name] = importDef.typ()
	}
	exports := make(map[string]Type)
	for name, exportDef := range d.exports {
		exports[name] = exportDef.typ()
	}
	return newCoreModuleType(imports, exports)
}

func (d *coreModuleTypeDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	imports := make(map[moduleName]Type)
	for name, importDef := range d.imports {
		ct, err := importDef.resolve(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve import %s.%s: %w", name.module, name.name, err)
		}
		imports[name] = ct
	}
	exports := make(map[string]Type)
	for name, exportDef := range d.exports {
		ct, err := exportDef.resolve(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve export %s: %w", name, err)
		}
		exports[name] = ct
	}
	return newCoreModuleType(imports, exports), nil
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

func (t *coreModuleType) typ() Type {
	return t
}

func (c *coreModuleType) assignableFrom(other Type) bool {
	otherModuleType, ok := other.(*coreModuleType)
	if !ok {
		return false
	}

	// Assignable if other imports are a subset of this imports
	for name, t := range otherModuleType.imports {
		thisType, ok := c.imports[name]
		if !ok || !t.assignableFrom(thisType) {
			return false
		}
	}

	// Assignable if this exports are a subset of other exports
	for name, t := range c.exports {
		otherType, ok := otherModuleType.exports[name]
		if !ok || !t.assignableFrom(otherType) {
			return false
		}
	}
	return true
}
