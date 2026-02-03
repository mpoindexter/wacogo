package componentmodel

import (
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero/api"
)

const maxMemoryPages = 65536 // 4 GiB

type coreTypeWasmConstType byte

func (c coreTypeWasmConstType) isType() {}

func (t coreTypeWasmConstType) typeName() string {
	switch t {
	case coreTypeWasmConstTypeFromWazero(api.ValueTypeI32):
		return "i32"
	case coreTypeWasmConstTypeFromWazero(api.ValueTypeI64):
		return "i64"
	case coreTypeWasmConstTypeFromWazero(api.ValueTypeF32):
		return "f32"
	case coreTypeWasmConstTypeFromWazero(api.ValueTypeF64):
		return "f64"
	case coreTypeWasmConstTypeV128:
		return "v128"
	case coreTypeWasmConstTypeFuncref:
		return "funcref"
	case coreTypeWasmConstTypeFromWazero(api.ValueTypeExternref):
		return "externref"
	default:
		return fmt.Sprintf("unknown value type: %d", byte(t))
	}
}

func (t coreTypeWasmConstType) checkType(other Type, checker typeChecker) error {
	return assertTypeIdentityEqual(t, other)
}

func (t coreTypeWasmConstType) typeSize() int {
	return 1
}

func (t coreTypeWasmConstType) typeDepth() int {
	return 1
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

func astCoreValTypeToTypeResolver(defs *definitions, astType ast.CoreValType) (typeResolver, error) {
	switch astType := astType.(type) {
	case ast.CoreNumType:
		switch astType {
		case ast.CoreNumTypeI32:
			return newStaticTypeResolver(coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)), nil
		case ast.CoreNumTypeI64:
			return newStaticTypeResolver(coreTypeWasmConstTypeFromWazero(api.ValueTypeI64)), nil
		case ast.CoreNumTypeF32:
			return newStaticTypeResolver(coreTypeWasmConstTypeFromWazero(api.ValueTypeF32)), nil
		case ast.CoreNumTypeF64:
			return newStaticTypeResolver(coreTypeWasmConstTypeFromWazero(api.ValueTypeF64)), nil
		}
	case ast.CoreVecType:
		switch astType {
		case ast.CoreVecTypeV128:
			return newStaticTypeResolver(coreTypeWasmConstTypeV128), nil
		default:
			return nil, fmt.Errorf("unknown core vector type: %v", astType)
		}
	case *ast.CoreRefType:
		return astRefTypeToTypeResolver(defs, astType)
	}
	return nil, fmt.Errorf("unknown core value type: %T", astType)
}

func astRefTypeToTypeResolver(defs *definitions, astType *ast.CoreRefType) (typeResolver, error) {
	switch astType := astType.HeapType.(type) {
	case ast.CoreAbsHeapType:
		switch astType {
		case ast.CoreAbsHeapTypeExtern:
			return newStaticTypeResolver(coreTypeWasmConstTypeFromWazero(api.ValueTypeExternref)), nil
		case ast.CoreAbsHeapTypeFunc:
			return newStaticTypeResolver(coreTypeWasmConstTypeFuncref), nil
		default:
			return nil, fmt.Errorf("unsupported abstract heap type: %v", astType)
		}
	case *ast.CoreConcreteHeapType:
		return newIndexTypeResolver(sortCoreType, astType.TypeIdx, func(t Type) error {
			switch t.(type) {
			case *coreFunctionType:
				return nil
			default:
				return fmt.Errorf("unexpected core ref type: type index %d is a %s type", astType.TypeIdx, t.typeName())
			}
		}), nil
	default:
		return nil, fmt.Errorf("unknown reference type: %T", astType)
	}
}

func astRecTypeToTypeResolver(defs *definitions, astType *ast.CoreRecType) (typeResolver, error) {
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
		return nil, fmt.Errorf("only core function types are supported in recursive types, found %T", st.Type)
	}

	paramTypes := make([]typeResolver, len(fnType.Params.Types))
	for i, astParamType := range fnType.Params.Types {
		tr, err := astCoreValTypeToTypeResolver(defs, astParamType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter type %d: %w", i, err)
		}
		paramTypes[i] = tr
	}
	resultTypes := make([]typeResolver, len(fnType.Results.Types))
	for i, astResultType := range fnType.Results.Types {
		tr, err := astCoreValTypeToTypeResolver(defs, astResultType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert result type %d: %w", i, err)
		}
		resultTypes[i] = tr
	}
	return newCoreTypeFunctionTypeResolver(paramTypes, resultTypes), nil
}

func astModuleTypeToCoreModuleTypeResolver(defs *definitions, astType *ast.CoreModuleType) (*coreModuleTypeDefinition, error) {
	imports := make(map[moduleName]typeResolver)
	exports := make(map[string]typeResolver)
	moduleDefs := newDefinitions()

	for _, decl := range astType.Declarations {
		switch decl := decl.(type) {
		case *ast.CoreTypeDecl:
			switch defType := decl.Type.DefType.(type) {
			case *ast.CoreRecType:
				tr, err := astRecTypeToTypeResolver(moduleDefs, defType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert recursive type: %w", err)
				}
				sortDefsFor(moduleDefs, sortCoreType).add(newTypeResolverDefinition(tr))
			case *ast.CoreModuleType:
				return nil, fmt.Errorf("nested module types are not supported")
			default:
				return nil, fmt.Errorf("unsupported core type definition in module type: %T", defType)
			}
		case *ast.CoreImportDecl:
			tr, err := astCoreImportDescToTypeResolver(moduleDefs, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s.%s: %w", decl.Module, decl.Name, err)
			}
			importName := moduleName{module: decl.Module, name: decl.Name}
			if _, exists := imports[importName]; exists {
				return nil, fmt.Errorf("duplicate import name `%s:%s`", decl.Module, decl.Name)
			}
			imports[importName] = tr
		case *ast.CoreExportDecl:
			tr, err := astCoreImportDescToTypeResolver(moduleDefs, decl.Desc)
			if err != nil {
				return nil, fmt.Errorf("failed to convert import %s: %w", decl.Name, err)
			}
			if _, exists := exports[decl.Name]; exists {
				return nil, fmt.Errorf("export name `%s` already defined", decl.Name)
			}
			exports[decl.Name] = tr
		case *ast.CoreAliasDecl:
			alias := decl.Target.(*ast.CoreOuterAlias)
			switch decl.Sort {
			case ast.CoreSortType:
				sortDefsFor(moduleDefs, sortCoreType).add(newOuterAliasDefinition(alias.Count, sortCoreType, alias.Idx, false))
			default:
				return nil, fmt.Errorf("unsupported core alias sort: %v", decl.Sort)
			}
		default:
			return nil, fmt.Errorf("unsupported module type declaration: %T", decl)
		}
	}

	return newCoreModuleTypeDefinition(
		moduleDefs,
		imports,
		exports,
	), nil
}

func astCoreImportDescToTypeResolver(defs *definitions, desc ast.CoreImportDesc) (typeResolver, error) {
	switch desc := desc.(type) {
	case *ast.CoreFuncImport:
		tr := newIndexTypeResolverOf[*coreFunctionType](sortCoreType, desc.TypeIdx, "")
		def := newTypeOnlyDefinition[*coreFunction, *coreFunctionType](tr)
		sortDefsFor(defs, sortCoreFunction).add(def)
		return tr, nil
	case *ast.CoreTableImport:
		elemTypeDef, err := astRefTypeToTypeResolver(defs, desc.Type.ElemType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert table element type: %w", err)
		}
		tr := newCoreTableTypeResolver(
			elemTypeDef,
			desc.Type.Limits.Min,
			desc.Type.Limits.Max,
		)

		def := newTypeOnlyDefinition[*coreTable, *coreTableType](tr)
		sortDefsFor(defs, sortCoreTable).add(def)
		return tr, nil
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
		tr := newStaticTypeResolver(newCoreMemoryType(
			desc.Type.Limits.Min,
			desc.Type.Limits.Max,
		))
		def := newTypeOnlyDefinition[*coreMemory, *coreMemoryType](tr)
		sortDefsFor(defs, sortCoreMemory).add(def)
		return tr, nil
	case *ast.CoreGlobalImport:
		elementResolver, err := astCoreValTypeToTypeResolver(defs, desc.Type.Val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert global value type: %w", err)
		}

		tr := newCoreGlobalTypeResolver(elementResolver, bool(desc.Type.Mut))
		def := newTypeOnlyDefinition[*coreGlobal, *coreGlobalType](tr)
		sortDefsFor(defs, sortCoreGlobal).add(def)
		return tr, nil
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
