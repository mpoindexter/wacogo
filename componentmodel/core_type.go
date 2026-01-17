package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero/api"
)

type coreTypeDefinition interface {
	resolveCoreType(ctx context.Context, scope instanceScope) (coreType, error)
}

type coreType interface {
	isCompatible(other coreType) bool
}

type coreTypeImportExportChecker interface {
	checkImport(name moduleName, mod *coreModule) bool
	checkExport(name string, mod *coreModule) bool
}

type coreTypeStaticDefinition struct {
	typ coreType
}

func (d *coreTypeStaticDefinition) resolveCoreType(ctx context.Context, scope instanceScope) (coreType, error) {
	return d.typ, nil
}

type coreTypeFuncDefinition struct {
	paramTypeDefs  []coreTypeDefinition
	resultTypeDefs []coreTypeDefinition
}

func (d *coreTypeFuncDefinition) resolveCoreType(ctx context.Context, scope instanceScope) (coreType, error) {
	paramTypes := make([]coreType, len(d.paramTypeDefs))
	for i, paramTypeDef := range d.paramTypeDefs {
		ct, err := paramTypeDef.resolveCoreType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter type %d: %w", i, err)
		}
		paramTypes[i] = ct
	}
	resultTypes := make([]coreType, len(d.resultTypeDefs))
	for i, resultTypeDef := range d.resultTypeDefs {
		ct, err := resultTypeDef.resolveCoreType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve result type %d: %w", i, err)
		}
		resultTypes[i] = ct
	}
	return &coreTypeFunc{
		paramTypes:  paramTypes,
		resultTypes: resultTypes,
	}, nil
}

type coreTypeFunc struct {
	paramTypes  []coreType
	resultTypes []coreType
}

var _ coreTypeImportExportChecker = (*coreTypeFunc)(nil)

func (c *coreTypeFunc) checkImport(name moduleName, mod *coreModule) bool {
	for _, fnDef := range mod.module.ImportedFunctions() {
		modName, fnName, ok := fnDef.Import()
		if ok {
			if modName == name.module && fnName == name.name {
				expectedType := &coreTypeFunc{
					paramTypes:  c.paramTypes,
					resultTypes: c.resultTypes,
				}

				paramTypes := fnDef.ParamTypes()
				resultTypes := fnDef.ResultTypes()
				actualType := &coreTypeFunc{
					paramTypes:  make([]coreType, len(paramTypes)),
					resultTypes: make([]coreType, len(resultTypes)),
				}
				for i, typ := range paramTypes {
					actualType.paramTypes[i] = coreTypeWazero(typ)
				}
				for i, typ := range resultTypes {
					actualType.resultTypes[i] = coreTypeWazero(typ)
				}
				return expectedType.isCompatible(actualType)
			}
		}
	}
	return false
}

func (c *coreTypeFunc) checkExport(name string, mod *coreModule) bool {
	for fnName, fnDef := range mod.module.ExportedFunctions() {
		if fnName == name {
			expectedType := &coreTypeFunc{
				paramTypes:  c.paramTypes,
				resultTypes: c.resultTypes,
			}

			paramTypes := fnDef.ParamTypes()
			resultTypes := fnDef.ResultTypes()
			actualType := &coreTypeFunc{
				paramTypes:  make([]coreType, len(paramTypes)),
				resultTypes: make([]coreType, len(resultTypes)),
			}
			for i, typ := range paramTypes {
				actualType.paramTypes[i] = coreTypeWazero(typ)
			}
			for i, typ := range resultTypes {
				actualType.resultTypes[i] = coreTypeWazero(typ)
			}
			return expectedType.isCompatible(actualType)
		}
	}
	return false
}

func (c *coreTypeFunc) isCompatible(other coreType) bool {
	otherFuncType, ok := other.(*coreTypeFunc)
	if !ok {
		return false
	}
	if len(c.paramTypes) != len(otherFuncType.paramTypes) {
		return false
	}
	for i, t := range c.paramTypes {
		if t != otherFuncType.paramTypes[i] {
			return false
		}
	}
	if len(c.resultTypes) != len(otherFuncType.resultTypes) {
		return false
	}
	for i, t := range c.resultTypes {
		if t != otherFuncType.resultTypes[i] {
			return false
		}
	}
	return true
}

type moduleName struct {
	module string
	name   string
}

type coreModuleTypeDefinition struct {
	imports map[moduleName]coreTypeDefinition
	exports map[string]coreTypeDefinition
}

func (d *coreModuleTypeDefinition) resolveCoreType(ctx context.Context, scope instanceScope) (coreType, error) {
	imports := make(map[moduleName]coreType)
	for name, importDef := range d.imports {
		ct, err := importDef.resolveCoreType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve import %s.%s: %w", name.module, name.name, err)
		}
		imports[name] = ct
	}
	exports := make(map[string]coreType)
	for name, exportDef := range d.exports {
		ct, err := exportDef.resolveCoreType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve export %s: %w", name, err)
		}
		exports[name] = ct
	}
	return &coreModuleType{
		imports: imports,
		exports: exports,
	}, nil
}

type coreModuleType struct {
	imports map[moduleName]coreType
	exports map[string]coreType
}

func (c *coreModuleType) validateModule(mod *coreModule) error {
	for name, importType := range c.imports {
		if checker, ok := importType.(coreTypeImportExportChecker); ok {
			if !checker.checkImport(name, mod) {
				return fmt.Errorf("import %s.%s does not match expected type", name.module, name.name)
			}
		}
	}

	for name, exportType := range c.exports {
		if checker, ok := exportType.(coreTypeImportExportChecker); ok {
			if !checker.checkExport(name, mod) {
				return fmt.Errorf("export %s does not match expected type", name)
			}
		}
	}

	// TODO: we should validate tables and functions here as well, but wazero does not expose these
	// in the API. We could parse them from the binary ourselves, but for now, we skip this validation.

	return nil
}

func (c *coreModuleType) isCompatible(other coreType) bool {
	otherModuleType, ok := other.(*coreModuleType)
	if !ok {
		return false
	}
	if len(c.imports) != len(otherModuleType.imports) {
		return false
	}
	for name, t := range c.imports {
		otherType, ok := otherModuleType.imports[name]
		if !ok || !t.isCompatible(otherType) {
			return false
		}
	}
	if len(c.exports) != len(otherModuleType.exports) {
		return false
	}
	for name, t := range c.exports {
		otherType, ok := otherModuleType.exports[name]
		if !ok || !t.isCompatible(otherType) {
			return false
		}
	}
	return true
}

type coreTypeWazero api.ValueType

func (t coreTypeWazero) isCompatible(other coreType) bool {
	otherWazeroType, ok := other.(coreTypeWazero)
	if !ok {
		return false
	}
	return t == otherWazeroType
}

type coreTypeV128 struct{}

func (t coreTypeV128) isCompatible(other coreType) bool {
	_, ok := other.(coreTypeV128)
	return ok
}

type coreTypeTableDefinition struct {
	elementType coreTypeDefinition
	min         uint32
	max         *uint32
}

func (d *coreTypeTableDefinition) resolveCoreType(ctx context.Context, scope instanceScope) (coreType, error) {
	elemType, err := d.elementType.resolveCoreType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve table element type: %w", err)
	}
	return &coreTypeTable{
		elementType: elemType,
		min:         d.min,
		max:         d.max,
	}, nil
}

type coreTypeTable struct {
	elementType coreType
	min         uint32
	max         *uint32
}

func (t *coreTypeTable) isCompatible(other coreType) bool {
	otherTable, ok := other.(*coreTypeTable)
	if !ok {
		return false
	}
	if !t.elementType.isCompatible(otherTable.elementType) {
		return false
	}
	if t.min != otherTable.min {
		return false
	}
	if (t.max == nil) != (otherTable.max == nil) {
		return false
	}
	if t.max != nil && otherTable.max != nil && *t.max != *otherTable.max {
		return false
	}
	return true
}

type coreTypeFuncref struct{}

func (t coreTypeFuncref) isCompatible(other coreType) bool {
	_, ok := other.(coreTypeFuncref)
	return ok
}

type coreMemType struct {
	min uint32
	max *uint32
}

var _ coreTypeImportExportChecker = (*coreTypeFunc)(nil)

func (t *coreMemType) checkImport(name moduleName, mod *coreModule) bool {
	for _, memDef := range mod.module.ImportedMemories() {
		modName, memName, ok := memDef.Import()
		if ok {
			if modName == name.module && memName == name.name {
				expectedType := &coreMemType{
					min: t.min,
					max: t.max,
				}

				actualType := &coreMemType{
					min: memDef.Min(),
					max: nil,
				}
				if max, ok := memDef.Max(); ok {
					actualType.max = &max
				}
				return expectedType.isCompatible(actualType)
			}
		}
	}
	return false
}

func (t *coreMemType) checkExport(name string, mod *coreModule) bool {
	for memName, memDef := range mod.module.ExportedMemories() {
		if memName == name {
			expectedType := &coreMemType{
				min: t.min,
				max: t.max,
			}

			actualType := &coreMemType{
				min: memDef.Min(),
				max: nil,
			}
			if max, ok := memDef.Max(); ok {
				actualType.max = &max
			}
			return expectedType.isCompatible(actualType)
		}
	}
	return false
}

func (t *coreMemType) isCompatible(other coreType) bool {
	otherMem, ok := other.(*coreMemType)
	if !ok {
		return false
	}
	if t.min != otherMem.min {
		return false
	}
	if (t.max == nil) != (otherMem.max == nil) {
		return false
	}
	if t.max != nil && otherMem.max != nil && *t.max != *otherMem.max {
		return false
	}
	return true
}

func astCoreValTypeToCoreTypeDefinition(scope definitionScope, astType ast.CoreValType) (coreTypeDefinition, error) {
	switch astType := astType.(type) {
	case ast.CoreNumType:
		switch astType {
		case ast.CoreNumTypeI32:
			return &coreTypeStaticDefinition{typ: coreTypeWazero(api.ValueTypeI32)}, nil
		case ast.CoreNumTypeI64:
			return &coreTypeStaticDefinition{typ: coreTypeWazero(api.ValueTypeI64)}, nil
		case ast.CoreNumTypeF32:
			return &coreTypeStaticDefinition{typ: coreTypeWazero(api.ValueTypeF32)}, nil
		case ast.CoreNumTypeF64:
			return &coreTypeStaticDefinition{typ: coreTypeWazero(api.ValueTypeF64)}, nil
		}
	case ast.CoreVecType:
		switch astType {
		case ast.CoreVecTypeV128:
			return &coreTypeStaticDefinition{typ: coreTypeV128{}}, nil
		default:
			return nil, fmt.Errorf("unknown core vector type: %v", astType)
		}
	case *ast.CoreRefType:
		return astRefTypeToCoreTypeDefinition(scope, astType)
	}
	return nil, fmt.Errorf("unknown core value type: %T", astType)
}

func astRefTypeToCoreTypeDefinition(scope definitionScope, astType *ast.CoreRefType) (coreTypeDefinition, error) {
	switch astType := astType.HeapType.(type) {
	case ast.CoreAbsHeapType:
		switch astType {
		case ast.CoreAbsHeapTypeExtern:
			return &coreTypeStaticDefinition{typ: coreTypeWazero(api.ValueTypeExternref)}, nil
		case ast.CoreAbsHeapTypeFunc:
			return &coreTypeStaticDefinition{typ: coreTypeFuncref{}}, nil
		default:
			return nil, fmt.Errorf("unsupported abstract heap type: %v", astType)
		}
	case *ast.CoreConcreteHeapType:
		return scope.resolveCoreTypeDefinition(0, astType.TypeIdx)
	default:
		return nil, fmt.Errorf("unknown reference type: %T", astType)
	}
}

func astRecTypeToCoreTypeDefinition(scope definitionScope, astType *ast.CoreRecType) (coreTypeDefinition, error) {
	// Currently, only function types are supported
	if len(astType.SubTypes) != 1 {
		return nil, fmt.Errorf("core recursive type with multiple types not supported")
	}
	st := astType.SubTypes[0]
	if !st.Final {
		return nil, fmt.Errorf("non-final core recursive types not supported")
	}
	fnType, ok := st.Type.(*ast.CoreFuncType)
	if !ok {
		return nil, fmt.Errorf("only core function types are supported in recursive types, got %T", st.Type)
	}

	paramTypes := make([]coreTypeDefinition, len(fnType.Params.Types))
	for i, astParamType := range fnType.Params.Types {
		ct, err := astCoreValTypeToCoreTypeDefinition(scope, astParamType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter type %d: %w", i, err)
		}
		paramTypes[i] = ct
	}
	resultTypes := make([]coreTypeDefinition, len(fnType.Results.Types))
	for i, astResultType := range fnType.Results.Types {
		ct, err := astCoreValTypeToCoreTypeDefinition(scope, astResultType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert result type %d: %w", i, err)
		}
		resultTypes[i] = ct
	}
	return &coreTypeFuncDefinition{
		paramTypeDefs:  paramTypes,
		resultTypeDefs: resultTypes,
	}, nil
}

func astModuleTypeToCoreModuleTypeDefinition(scope definitionScope, astType *ast.CoreModuleType) (*coreModuleTypeDefinition, error) {
	imports := make(map[moduleName]coreTypeDefinition)
	exports := make(map[string]coreTypeDefinition)
	moduleScope := coreTypeModuleTypeScope{
		parent: scope,
	}
	for _, decl := range astType.Declarations {
		switch decl := decl.(type) {
		case *ast.CoreTypeDecl:
			switch defType := decl.Type.DefType.(type) {
			case *ast.CoreRecType:
				typDef, err := astRecTypeToCoreTypeDefinition(&moduleScope, defType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert recursive type: %w", err)
				}
				moduleScope.types = append(moduleScope.types, typDef)
			case *ast.CoreModuleType:
				return nil, fmt.Errorf("nested module types are not supported")
			}
		case *ast.CoreImportDecl:
			def, err := astCoreImportDescToCoreTypeDefinition(&moduleScope, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s.%s: %w", decl.Module, decl.Name, err)
			}
			imports[moduleName{module: decl.Module, name: decl.Name}] = def
		case *ast.CoreExportDecl:
			def, err := astCoreImportDescToCoreTypeDefinition(&moduleScope, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s: %w", decl.Name, err)
			}
			exports[decl.Name] = def
		case *ast.CoreAliasDecl:
			alias := decl.Target.(*ast.CoreOuterAlias)
			switch decl.Sort {
			case ast.CoreSortType:
				def, err := moduleScope.resolveCoreTypeDefinition(alias.Count, alias.Idx)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve core type definition for alias: %w", err)
				}
				moduleScope.types = append(moduleScope.types, def)
				continue
			default:
				return nil, fmt.Errorf("unsupported core alias sort: %v", decl.Sort)
			}
		default:
			return nil, fmt.Errorf("unsupported module type declaration: %T", decl)
		}
	}

	return &coreModuleTypeDefinition{
		imports: imports,
		exports: exports,
	}, nil
}

func astCoreImportDescToCoreTypeDefinition(moduleScope *coreTypeModuleTypeScope, desc ast.CoreImportDesc) (coreTypeDefinition, error) {
	switch desc := desc.(type) {
	case *ast.CoreFuncImport:
		funcTypeDef, err := moduleScope.resolveCoreTypeDefinition(0, desc.TypeIdx)
		if err != nil {
			return nil, err
		}
		moduleScope.types = append(moduleScope.types, funcTypeDef)
		return funcTypeDef, nil
	case *ast.CoreTableImport:
		elemType, err := astRefTypeToCoreTypeDefinition(moduleScope, desc.Type.ElemType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert table element type: %w", err)
		}
		tableTypeDef := &coreTypeTableDefinition{
			elementType: elemType,
			min:         desc.Type.Limits.Min,
			max:         desc.Type.Limits.Max,
		}
		moduleScope.types = append(moduleScope.types, tableTypeDef)
		return tableTypeDef, nil
	case *ast.CoreMemoryImport:
		memType := &coreTypeStaticDefinition{
			typ: &coreMemType{
				min: desc.Type.Limits.Min,
				max: desc.Type.Limits.Max,
			},
		}
		moduleScope.types = append(moduleScope.types, memType)
		return memType, nil
	case *ast.CoreGlobalImport:
		valTypeDef, err := astCoreValTypeToCoreTypeDefinition(moduleScope, desc.Type.Val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert global value type: %w", err)
		}
		moduleScope.types = append(moduleScope.types, valTypeDef)
		return valTypeDef, nil
	case *ast.CoreTagImport:
		return nil, fmt.Errorf("core tag imports are not supported")
	default:
		return nil, fmt.Errorf("unsupported import descriptor: %T", desc)
	}
}

type coreTypeModuleTypeScope struct {
	parent definitionScope
	types  []coreTypeDefinition
}

func (s *coreTypeModuleTypeScope) resolveFunctionDefinition(count uint32, idx uint32) (functionDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveInstanceDefinition(count uint32, idx uint32) (instanceDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveComponentDefinition(count uint32, idx uint32) (componentDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreFunctionDefinition(count uint32, idx uint32) (coreFunctionDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreMemoryDefinition(count uint32, idx uint32) (coreMemoryDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreTableDefinition(count uint32, idx uint32) (coreTableDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreGlobalDefinition(count uint32, idx uint32) (coreGlobalDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreModuleDefinition(count uint32, idx uint32) (coreModuleDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveCoreInstanceDefinition(count uint32, idx uint32) (coreInstanceDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}
func (s *coreTypeModuleTypeScope) resolveComponentModelTypeDefinition(count uint32, idx uint32) (componentModelTypeDefinition, error) {
	return nil, fmt.Errorf("cannot resolve a definition of this type in this context")
}

func (s *coreTypeModuleTypeScope) resolveCoreTypeDefinition(count uint32, idx uint32) (coreTypeDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("outer scope not found at count %d", count)
		}
		return s.parent.resolveCoreTypeDefinition(count-1, idx)
	}
	if int(idx) >= len(s.types) {
		return nil, fmt.Errorf("core type index out of bounds: %d", idx)
	}
	return s.types[idx], nil
}
