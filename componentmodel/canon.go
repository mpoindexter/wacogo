package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	maxFlatParams  = 16
	maxFlatResults = 1
)

type LiftLoadContext struct {
	instance       *Instance
	memory         api.Memory
	stringEncoding stringEncoding
	realloc        func(originalPtr, originalSize, alignment, newSize uint32) uint32
	cleanups       []func(ctx context.Context)
}

func (llc *LiftLoadContext) addCleanup(fn func(ctx context.Context)) {
	llc.cleanups = append(llc.cleanups, fn)
}

func (llc *LiftLoadContext) cleanup(ctx context.Context) {
	for _, fn := range llc.cleanups {
		fn(ctx)
	}
}

type stringEncoding int

const (
	stringEncodingUTF8 stringEncoding = iota
	stringEncodingUTF16
	stringEncodingLatin1UTF16
)

func canonLower(comp *Component, astDef *ast.CanonLower) (coreFunctionDefinition, error) {
	id := fmt.Sprintf("canon_lower_%s_fn_%d", comp.id, len(comp.scope.coreFunctions))
	return &coreFunctionLoweredDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionLoweredDefinition struct {
	id     string
	astDef *ast.CanonLower
}

func (d *coreFunctionLoweredDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	fnDef, err := scope.resolveFunctionDefinition(0, d.astDef.FuncIdx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve function for canon lower: %w", err)
	}
	fn, err := fnDef.resolveFunction(ctx, scope)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve function for canon lower: %w", err)
	}

	var flatParamTypes []api.ValueType
	var flatResultTypes []api.ValueType

	for _, p := range fn.typ.ParamTypes {
		flatParamTypes = append(flatParamTypes, p.flatTypes()...)
	}

	if fn.typ.ResultType != nil {
		flatResultTypes = append(flatResultTypes, fn.typ.ResultType.flatTypes()...)
	}

	paramsFlat := true
	if len(flatParamTypes) > maxFlatParams {
		paramsFlat = false
		flatParamTypes = []api.ValueType{api.ValueTypeI32}
	}

	returnFlat := true
	if len(flatResultTypes) > maxFlatResults {
		returnFlat = false
		flatParamTypes = append(flatParamTypes, api.ValueTypeI32)
		flatResultTypes = []api.ValueType{}
	}

	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					panic(fmt.Errorf("failed to create lift/load context for canon lower: %w", err))
				}
				defer llc.cleanup(ctx)

				remainingParams := stack
				itr := func() uint64 {
					val := remainingParams[0]
					remainingParams = remainingParams[1:]
					return val
				}

				var paramValues []Value
				if paramsFlat {

					paramValues = make([]Value, 0, len(fn.typ.ParamTypes))
					for i, pType := range fn.typ.ParamTypes {
						val, err := pType.liftFlat(llc, itr)
						if err != nil {
							panic(fmt.Errorf("failed to load parameter %d for canon lower: %w", i, err))
						}
						paramValues = append(paramValues, val)
					}
				} else {
					// Too many parameters to pass flatly - pass a pointer to a memory block instead
					offset := uint32(itr())
					tt := TupleType(fn.typ.ParamTypes...)
					tup, err := tt.load(llc, offset)
					if err != nil {
						panic(fmt.Errorf("failed to load parameters for canon lower: %w", err))
					}
					paramValues = tup.(*Record).fields
				}

				result := fn.invoke(ctx, paramValues)

				if fn.typ.ResultType != nil {
					if returnFlat {
						flatResults, err := fn.typ.ResultType.lowerFlat(llc, result)
						if err != nil {
							panic(fmt.Errorf("failed to lower result for canon lower: %w", err))
						}
						copy(stack, flatResults)
					} else {
						// Too many results to return flatly - write to a memory block instead
						offset := uint32(itr())
						err := fn.typ.ResultType.store(llc, offset, result)
						if err != nil {
							panic(fmt.Errorf("failed to store result for canon lower: %w", err))
						}
					}
				}

				// TODO: Handle post return options

			}),
			flatParamTypes,
			flatResultTypes,
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create canon lower stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate canon lower stub module: %w", err)
	}

	return modInst, "stub_function", mod.ExportedFunctions()["stub_function"], nil
}

func canonResourceDrop(comp *Component, astDef *ast.CanonResourceDrop) (coreFunctionDefinition, error) {
	id := fmt.Sprintf("canon_resource_drop_%s_fn_%d", comp.id, len(comp.scope.coreFunctions))
	return &coreFunctionResourceDropDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceDropDefinition struct {
	id     string
	astDef *ast.CanonResourceDrop
}

func (d *coreFunctionResourceDropDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				// tODO
			}),
			[]api.ValueType{api.ValueTypeI32},
			[]api.ValueType{},
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create canon lower stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate canon lower stub module: %w", err)
	}

	return modInst, "stub_function", mod.ExportedFunctions()["stub_function"], nil
}

func canonLift(comp *Component, astDef *ast.CanonLift) (functionDefinition, error) {
	id := fmt.Sprintf("canon_lift_%s_fn_%d", comp.id, len(comp.scope.functions))
	return &functionLiftedDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type functionLiftedDefinition struct {
	id     string
	astDef *ast.CanonLift
}

func (d *functionLiftedDefinition) resolveFunction(ctx context.Context, scope instanceScope) (*Function, error) {
	fnTypeDef, err := scope.resolveComponentModelTypeDefinition(0, d.astDef.FunctionTypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnTypeGeneric, err := fnTypeDef.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnType, ok := fnTypeGeneric.(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("canon lift function type is not a function")
	}
	coreFnDef, err := scope.resolveCoreFunctionDefinition(0, d.astDef.CoreFuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core function for canon lift: %w", err)
	}
	module, name, _, err := coreFnDef.resolveCoreFunction(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core function for canon lift: %w", err)
	}
	coreFn := module.ExportedFunction(name)

	var flatParamTypes []api.ValueType
	var flatResultTypes []api.ValueType

	for _, p := range fnType.ParamTypes {
		flatParamTypes = append(flatParamTypes, p.flatTypes()...)
	}

	if fnType.ResultType != nil {
		flatResultTypes = append(flatResultTypes, fnType.ResultType.flatTypes()...)
	}

	paramsFlat := true
	if len(flatParamTypes) > maxFlatParams {
		paramsFlat = false
		flatParamTypes = []api.ValueType{api.ValueTypeI32}
	}

	returnFlat := true
	if len(flatResultTypes) > maxFlatResults {
		returnFlat = false
		flatResultTypes = []api.ValueType{api.ValueTypeI32}
	}

	return NewFunction(
		scope.currentInstance(),
		fnType,
		func(ctx context.Context, params []Value) Value {
			llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
			if err != nil {
				panic(fmt.Errorf("failed to create lift/load context for canon lower: %w", err))
			}
			defer llc.cleanup(ctx)

			var flatParams []uint64
			if paramsFlat {
				for i, pType := range fnType.ParamTypes {
					flatVals, err := pType.lowerFlat(llc, params[i])
					if err != nil {
						panic(fmt.Errorf("failed to lower parameter %d for canon lift: %w", i, err))
					}
					flatParams = append(flatParams, flatVals...)
				}
			} else {
				tt := TupleType(fnType.ParamTypes...)
				offset := llc.realloc(0, 0, uint32(tt.alignment()), uint32(tt.elementSize()))
				err := tt.store(llc, offset, &Record{fields: params})
				if err != nil {
					panic(fmt.Errorf("failed to load parameters for canon lower: %w", err))
				}
				flatParams = []uint64{uint64(offset)}
			}

			results, err := coreFn.Call(ctx, flatParams...)
			if err != nil {
				panic(fmt.Errorf("failed to call core function for canon lift: %w", err))
			}

			// TODO: Handle post return options

			if fnType.ResultType != nil {
				if returnFlat {
					val, err := fnType.ResultType.liftFlat(llc, func() uint64 {
						val := results[0]
						results = results[1:]
						return val
					})
					if err != nil {
						panic(fmt.Errorf("failed to lift result for canon lift: %w", err))
					}
					return val
				} else {
					// Too many results to return flatly - read from a memory block instead
					offset := uint32(results[0])
					val, err := fnType.ResultType.load(llc, offset)
					if err != nil {
						panic(fmt.Errorf("failed to load result for canon lift: %w", err))
					}
					return val
				}
			}
			return nil
		},
	), nil
}

func newLiftLoadContext(ctx context.Context, opts []ast.CanonOpt, scope instanceScope) (*LiftLoadContext, error) {
	llc := &LiftLoadContext{
		instance: scope.currentInstance(),
	}

	for _, opt := range opts {
		switch o := opt.(type) {
		case *ast.StringEncodingOpt:
			switch o.Encoding {
			case ast.StringEncodingUTF8:
				llc.stringEncoding = stringEncodingUTF8
			case ast.StringEncodingUTF16:
				llc.stringEncoding = stringEncodingUTF16
			case ast.StringEncodingLatin1UTF16:
				llc.stringEncoding = stringEncodingLatin1UTF16
			}
		case *ast.MemoryOpt:
			memDef, err := scope.resolveCoreMemoryDefinition(0, o.MemoryIdx)
			if err != nil {
				return nil, err
			}

			_, _, mem, err := memDef.resolveMemory(ctx, scope)
			if err != nil {
				return nil, err
			}
			llc.memory = mem
		case *ast.ReallocOpt:
			coreFnDef, err := scope.resolveCoreFunctionDefinition(0, o.FuncIdx)
			if err != nil {
				return nil, err
			}
			mod, fnName, _, err := coreFnDef.resolveCoreFunction(ctx, scope)
			if err != nil {
				return nil, err
			}
			reallocFn := mod.ExportedFunction(fnName)
			llc.realloc = func(originalPtr, originalSize, alignment, newSize uint32) uint32 {
				results, err := reallocFn.Call(ctx, uint64(originalPtr), uint64(originalSize), uint64(alignment), uint64(newSize))
				if err != nil || len(results) != 1 {
					return 0
				}
				return uint32(results[0])
			}
		case *ast.PostReturnOpt:
			// TODO: Implement post return handling
		default:
			return nil, fmt.Errorf("unknown canon lift/load option: %T", opt)
		}
	}

	return llc, nil
}
