package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero/api"
)

const memoryPageSize = 65536
const maxMemoryPages = 65536 // 4 GiB

type coreTypeStaticDefinition struct {
	staticType Type
}

func newCoreTypeStaticDefinition(staticType Type) *coreTypeStaticDefinition {
	return &coreTypeStaticDefinition{
		staticType: staticType,
	}
}

func (d *coreTypeStaticDefinition) typ() Type {
	return d.staticType
}

func (d *coreTypeStaticDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	return d.staticType, nil
}

type coreTypeWasmConstType byte

func (t coreTypeWasmConstType) typ() Type {
	return t
}

func (t coreTypeWasmConstType) assignableFrom(other Type) bool {
	otherWazeroType, ok := other.(coreTypeWasmConstType)
	if !ok {
		return false
	}
	return t == otherWazeroType
}

var coreTypeWasmConstTypeV128 = coreTypeWasmConstType(0x7B)
var coreTypeWasmConstTypeFuncref = coreTypeWasmConstType(0x70)

func coreTypeWasmConstTypeFromWazero(vt api.ValueType) coreTypeWasmConstType {
	return coreTypeWasmConstType(byte(vt))
}

func coreTypeWasmConstTypeFromWasmParser(vt wasm.Type) (coreTypeWasmConstType, error) {
	switch vt.(type) {
	case wasm.I32:
		return coreTypeWasmConstType(api.ValueTypeI32), nil
	case wasm.I64:
		return coreTypeWasmConstType(api.ValueTypeI64), nil
	case wasm.F32:
		return coreTypeWasmConstType(api.ValueTypeF32), nil
	case wasm.F64:
		return coreTypeWasmConstType(api.ValueTypeF64), nil
	case wasm.V128:
		return coreTypeWasmConstTypeV128, nil
	case wasm.FuncRef:
		return coreTypeWasmConstTypeFuncref, nil
	case wasm.ExternRef:
		return coreTypeWasmConstType(api.ValueTypeExternref), nil
	default:
		return 0, fmt.Errorf("unknown wasm value type: %d", vt)
	}
}

func astCoreValTypeToCoreTypeDefinition(scope *definitionScope, astType ast.CoreValType) (definition[Type, Type], error) {
	switch astType := astType.(type) {
	case ast.CoreNumType:
		switch astType {
		case ast.CoreNumTypeI32:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)), nil
		case ast.CoreNumTypeI64:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFromWazero(api.ValueTypeI64)), nil
		case ast.CoreNumTypeF32:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFromWazero(api.ValueTypeF32)), nil
		case ast.CoreNumTypeF64:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFromWazero(api.ValueTypeF64)), nil
		}
	case ast.CoreVecType:
		switch astType {
		case ast.CoreVecTypeV128:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeV128), nil
		default:
			return nil, fmt.Errorf("unknown core vector type: %v", astType)
		}
	case *ast.CoreRefType:
		return astRefTypeToCoreTypeDefinition(scope, astType)
	}
	return nil, fmt.Errorf("unknown core value type: %T", astType)
}

func astRefTypeToCoreTypeDefinition(scope *definitionScope, astType *ast.CoreRefType) (definition[Type, Type], error) {
	switch astType := astType.HeapType.(type) {
	case ast.CoreAbsHeapType:
		switch astType {
		case ast.CoreAbsHeapTypeExtern:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFromWazero(api.ValueTypeExternref)), nil
		case ast.CoreAbsHeapTypeFunc:
			return newCoreTypeStaticDefinition(coreTypeWasmConstTypeFuncref), nil
		default:
			return nil, fmt.Errorf("unsupported abstract heap type: %v", astType)
		}
	case *ast.CoreConcreteHeapType:
		return defs(scope, sortCoreType).get(astType.TypeIdx)
	default:
		return nil, fmt.Errorf("unknown reference type: %T", astType)
	}
}

func astRecTypeToCoreTypeDefinition(scope *definitionScope, astType *ast.CoreRecType) (definition[Type, Type], error) {
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

	paramTypes := make([]definition[Type, Type], len(fnType.Params.Types))
	for i, astParamType := range fnType.Params.Types {
		ct, err := astCoreValTypeToCoreTypeDefinition(scope, astParamType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter type %d: %w", i, err)
		}
		paramTypes[i] = ct
	}
	resultTypes := make([]definition[Type, Type], len(fnType.Results.Types))
	for i, astResultType := range fnType.Results.Types {
		ct, err := astCoreValTypeToCoreTypeDefinition(scope, astResultType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert result type %d: %w", i, err)
		}
		resultTypes[i] = ct
	}
	return newCoreTypeFunctionDefinition(paramTypes, resultTypes), nil
}

func astModuleTypeToCoreModuleTypeDefinition(scope *definitionScope, astType *ast.CoreModuleType) (*coreModuleTypeDefinition, error) {
	imports := make(map[moduleName]definition[Type, Type])
	exports := make(map[string]definition[Type, Type])
	moduleScope := newDefinitionScope(scope)
	for _, decl := range astType.Declarations {
		switch decl := decl.(type) {
		case *ast.CoreTypeDecl:
			switch defType := decl.Type.DefType.(type) {
			case *ast.CoreRecType:
				typDef, err := astRecTypeToCoreTypeDefinition(moduleScope, defType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert recursive type: %w", err)
				}
				defs(moduleScope, sortCoreType).add(typDef)
			case *ast.CoreModuleType:
				return nil, fmt.Errorf("nested module types are not supported")
			}
		case *ast.CoreImportDecl:
			def, err := astCoreImportDescToCoreTypeDefinition(moduleScope, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s.%s: %w", decl.Module, decl.Name, err)
			}
			importName := moduleName{module: decl.Module, name: decl.Name}
			if _, exists := imports[importName]; exists {
				return nil, fmt.Errorf("duplicate import name: %s.%s", decl.Module, decl.Name)
			}
			imports[importName] = def
		case *ast.CoreExportDecl:
			def, err := astCoreImportDescToCoreTypeDefinition(moduleScope, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s: %w", decl.Name, err)
			}
			if _, exists := exports[decl.Name]; exists {
				return nil, fmt.Errorf("export name `%s` already defined", decl.Name)
			}
			exports[decl.Name] = def
		case *ast.CoreAliasDecl:
			alias := decl.Target.(*ast.CoreOuterAlias)
			switch decl.Sort {
			case ast.CoreSortType:
				coreTypeDefs, err := nestedDefs(moduleScope, sortCoreType, alias.Count)
				if err != nil {
					return nil, fmt.Errorf("failed to get nested core type definitions for alias: %w", err)
				}
				def, err := coreTypeDefs.get(alias.Idx)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve core type definition for alias: %w", err)
				}
				defs(moduleScope, sortCoreType).add(def)
				continue
			default:
				return nil, fmt.Errorf("unsupported core alias sort: %v", decl.Sort)
			}
		default:
			return nil, fmt.Errorf("unsupported module type declaration: %T", decl)
		}
	}

	return newCoreModuleTypeDefinition(
		imports,
		exports,
	), nil
}

func astCoreImportDescToCoreTypeDefinition(moduleScope *definitionScope, desc ast.CoreImportDesc) (definition[Type, Type], error) {
	switch desc := desc.(type) {
	case *ast.CoreFuncImport:
		funcTypeDef, err := defs(moduleScope, sortCoreType).get(desc.TypeIdx)
		if err != nil {
			return nil, err
		}
		defs(moduleScope, sortCoreType).add(funcTypeDef)
		return funcTypeDef, nil
	case *ast.CoreTableImport:
		elemType, err := astRefTypeToCoreTypeDefinition(moduleScope, desc.Type.ElemType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert table element type: %w", err)
		}
		tableTypeDef := newCoreTypeTableDefinition(elemType, desc.Type.Limits.Min, desc.Type.Limits.Max)
		defs(moduleScope, sortCoreType).add(tableTypeDef)
		return tableTypeDef, nil
	case *ast.CoreMemoryImport:
		if desc.Type.Limits.Max != nil {
			if *desc.Type.Limits.Max > maxMemoryPages {
				return nil, fmt.Errorf("memory size must be at most %d", maxMemoryPages)
			}

			if *desc.Type.Limits.Max < desc.Type.Limits.Min {
				return nil, fmt.Errorf("memory import max pages %d is less than min pages %d", *desc.Type.Limits.Max, desc.Type.Limits.Min)
			}
		}

		if desc.Type.Limits.Min > maxMemoryPages {
			return nil, fmt.Errorf("memory size must be at most %d", maxMemoryPages)
		}
		memType := newCoreTypeStaticDefinition(newCoreMemoryType(
			desc.Type.Limits.Min,
			desc.Type.Limits.Max,
		))
		defs(moduleScope, sortCoreType).add(memType)
		return memType, nil
	case *ast.CoreGlobalImport:
		valTypeDef, err := astCoreValTypeToCoreTypeDefinition(moduleScope, desc.Type.Val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert global value type: %w", err)
		}
		defs(moduleScope, sortCoreType).add(valTypeDef)
		return valTypeDef, nil
	case *ast.CoreTagImport:
		return nil, fmt.Errorf("core tag imports are not supported")
	default:
		return nil, fmt.Errorf("unsupported import descriptor: %T", desc)
	}
}

func wazeroFunctionDefinitionToCoreFunctionType(fnDef api.FunctionDefinition) *coreFunctionType {
	params := make([]Type, len(fnDef.ParamTypes()))
	for i, p := range fnDef.ParamTypes() {
		params[i] = coreTypeWasmConstTypeFromWazero(p)
	}
	results := make([]Type, len(fnDef.ResultTypes()))
	for i, r := range fnDef.ResultTypes() {
		results[i] = coreTypeWasmConstTypeFromWazero(r)
	}
	return newCoreFunctionType(params, results)
}

func wazeroMemoryDefinitionToCoreMemoryType(memType api.MemoryDefinition) *coreMemoryType {
	var max *uint32
	if defMax, ok := memType.Max(); ok {
		max = &defMax
	}
	return newCoreMemoryType(memType.Min(), max)
}

func wasmMemoryTypeToCoreMemoryType(mt *wasm.MemoryType) *coreMemoryType {
	var max *uint32
	if mt.HasMax {
		max = &mt.Max
	}
	return newCoreMemoryType(mt.Min, max)
}

func wasmTableTypeToCoreTableType(tt *wasm.TableType) *coreTableType {
	var elementType Type
	switch tt.ElemType.(type) {
	case wasm.FuncRef:
		elementType = coreTypeWasmConstTypeFuncref
	case wasm.ExternRef:
		elementType = coreTypeWasmConstTypeFromWazero(api.ValueTypeExternref)
	}

	max := &tt.Limits.Max
	if !tt.Limits.HasMax {
		max = nil
	}
	return newCoreTableType(elementType, tt.Limits.Min, max)
}
