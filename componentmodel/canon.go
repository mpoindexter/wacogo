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
	ctx            context.Context
	instance       *Instance
	memory         api.Memory
	stringEncoding stringEncoding
	realloc        func(originalPtr, originalSize, alignment, newSize uint32) (uint32, error)
	postreturn     api.Function
}

type stringEncoding int

const (
	stringEncodingUTF8 stringEncoding = iota
	stringEncodingUTF16
	stringEncodingLatin1UTF16
)

func canonLower(comp *Component, astDef *ast.CanonLower) (definition[*coreFunction, *coreFunctionType], error) {
	id := fmt.Sprintf("canon_lower_%s_fn_%d", comp.id, defs(comp.scope, sortCoreFunction).len())
	fnDef, err := defs(comp.scope, sortFunction).get(astDef.FuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function for canon lower: %w", err)
	}
	fnType := fnDef.typ()
	return &coreFunctionLoweredDefinition{
		id:     id,
		astDef: astDef,
		fnType: fnType,
	}, nil
}

type coreFunctionLoweredDefinition struct {
	id     string
	astDef *ast.CanonLower
	fnType *FunctionType
}

func (d *coreFunctionLoweredDefinition) typ() *coreFunctionType {
	flatParamTypes, flatResultTypes, _, _ := loweredCoreFunctionTypesFromFunctionType(d.fnType)
	paramTypes := make([]Type, len(flatParamTypes))
	for i, vt := range flatParamTypes {
		paramTypes[i] = coreTypeWasmConstTypeFromWazero(vt)
	}
	resultTypes := make([]Type, len(flatResultTypes))
	for i, vt := range flatResultTypes {
		resultTypes[i] = coreTypeWasmConstTypeFromWazero(vt)
	}
	return newCoreFunctionType(paramTypes, resultTypes)
}

func (d *coreFunctionLoweredDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreFunction, error) {
	fn, err := resolve(ctx, scope, sortFunction, d.astDef.FuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function for canon lower: %w", err)
	}

	fnTyp := fn.typ()

	flatParamTypes, flatResultTypes, paramsFlat, returnFlat := loweredCoreFunctionTypesFromFunctionType(fnTyp)

	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					panic(fmt.Errorf("failed to create lift/load context for canon lower: %w", err))
				}

				remainingParams := stack
				itr := func() uint64 {
					val := remainingParams[0]
					remainingParams = remainingParams[1:]
					return val
				}

				var paramValues []Value
				if paramsFlat {

					paramValues = make([]Value, 0, len(fnTyp.Parameters))
					for i, pType := range fnTyp.Parameters {
						val, err := pType.Type.liftFlat(llc, itr)
						if err != nil {
							panic(fmt.Errorf("failed to load parameter %d for canon lower: %w", i, err))
						}
						paramValues = append(paramValues, val)
					}
				} else {
					offset := uint32(itr())
					paramTypes := make([]ValueType, len(fnTyp.Parameters))
					for i, p := range fnTyp.Parameters {
						paramTypes[i] = p.Type
					}
					tt := TupleType(paramTypes...)
					tup, err := tt.load(llc, offset)
					if err != nil {
						panic(fmt.Errorf("failed to load parameters for canon lower: %w", err))
					}
					paramValues = tup.(*Record).fields
				}

				result, err := fn.invoke(ctx, paramValues)
				if err != nil {
					panic(fmt.Errorf("failed to call core function for canon lower: %w", err))
				}

				if fnTyp.ResultType != nil {
					if returnFlat {
						flatResults, err := fnTyp.ResultType.lowerFlat(llc, result)
						if err != nil {
							panic(fmt.Errorf("failed to lower result for canon lower: %w", err))
						}
						copy(stack, flatResults)
					} else {
						offset := uint32(itr())
						err := fnTyp.ResultType.store(llc, offset, result)
						if err != nil {
							panic(fmt.Errorf("failed to store result for canon lower: %w", err))
						}
					}
				}
			}),
			flatParamTypes,
			flatResultTypes,
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create canon lower stub module: %w", err)
	}

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate canon lower stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func loweredCoreFunctionTypesFromFunctionType(fnType *FunctionType) ([]api.ValueType, []api.ValueType, bool, bool) {
	var flatParamTypes []api.ValueType
	var flatResultTypes []api.ValueType

	for _, p := range fnType.Parameters {
		flatParamTypes = append(flatParamTypes, p.Type.flatTypes()...)
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
		flatParamTypes = append(flatParamTypes, api.ValueTypeI32)
		flatResultTypes = []api.ValueType{}
	}
	return flatParamTypes, flatResultTypes, paramsFlat, returnFlat
}

func canonResourceNew(comp *Component, astDef *ast.CanonResourceNew) (definition[*coreFunction, *coreFunctionType], error) {
	id := fmt.Sprintf("canon_resource_new_%s_fn_%d", comp.id, defs(comp.scope, sortCoreFunction).len())
	return &coreFunctionResourceNewDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceNewDefinition struct {
	id     string
	astDef *ast.CanonResourceNew
}

func (d *coreFunctionResourceNewDefinition) typ() *coreFunctionType {
	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
	)
}

func (d *coreFunctionResourceNewDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreFunction, error) {
	resourceTypeGeneric, err := resolve(ctx, scope, sortType, d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.new: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.new type is not a resource")
	}
	if err := validateResourceTypeDefinedInComponent(resourceType, scope.currentInstance); err != nil {
		return nil, fmt.Errorf("canon resource.new validation failed: %w", err)
	}
	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				instance := scope.currentInstance
				handle := NewResourceHandle(instance, resourceType, rep)
				handleIdx := instance.loweredHandles.add(handle)
				stack[0] = uint64(handleIdx)
			}),
			[]api.ValueType{api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32},
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource new stub module: %w", err)
	}

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource new stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonResourceDrop(comp *Component, astDef *ast.CanonResourceDrop) (definition[*coreFunction, *coreFunctionType], error) {
	id := fmt.Sprintf("canon_resource_drop_%s_fn_%d", comp.id, defs(comp.scope, sortCoreFunction).len())
	return &coreFunctionResourceDropDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceDropDefinition struct {
	id     string
	astDef *ast.CanonResourceDrop
}

func (d *coreFunctionResourceDropDefinition) typ() *coreFunctionType {
	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{},
	)
}

func (d *coreFunctionResourceDropDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreFunction, error) {
	resourceTypeGeneric, err := resolve(ctx, scope, sortType, d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon drop: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon drop type is not a resource")
	}
	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				instance := scope.currentInstance
				resourceIdx := uint32(stack[0])
				handle := instance.loweredHandles.remove(uint32(resourceIdx))
				if handle.resourceType() != resourceType {
					panic(fmt.Errorf("resource type mismatch in canon drop"))
				}
				if handle.isBorrowed() {
					panic(fmt.Errorf("cannot drop resource with outstanding lends"))
				}
				handle.Drop()
			}),
			[]api.ValueType{api.ValueTypeI32},
			[]api.ValueType{},
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource drop stub module: %w", err)
	}

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonResourceRep(comp *Component, astDef *ast.CanonResourceRep) (definition[*coreFunction, *coreFunctionType], error) {
	id := fmt.Sprintf("canon_resource_rep_%s_fn_%d", comp.id, defs(comp.scope, sortCoreFunction).len())
	return &coreFunctionResourceRepDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceRepDefinition struct {
	id     string
	astDef *ast.CanonResourceRep
}

func (d *coreFunctionResourceRepDefinition) typ() *coreFunctionType {
	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
	)
}

func (d *coreFunctionResourceRepDefinition) resolve(ctx context.Context, scope *instanceScope) (*coreFunction, error) {
	resourceTypeGeneric, err := resolve(ctx, scope, sortType, d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.rep type is not a resource")
	}
	if err := validateResourceTypeDefinedInComponent(resourceType, scope.currentInstance); err != nil {
		return nil, fmt.Errorf("canon resource.rep validation failed: %w", err)
	}
	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				instance := scope.currentInstance
				resourceIdx := uint32(stack[0])
				handle := instance.loweredHandles.get(uint32(resourceIdx))
				if handle.resourceType() != resourceType {
					panic(fmt.Errorf("resource type mismatch in canon drop"))
				}
				rep := handle.Resource()
				if u32, ok := rep.(uint32); ok {
					stack[0] = uint64(u32)
				} else {
					panic(fmt.Errorf("resource representation is not uint32"))
				}
			}),
			[]api.ValueType{api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32},
		).
		Export("stub_function").
		Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource drop stub module: %w", err)
	}

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonLift(comp *Component, astDef *ast.CanonLift) (definition[*Function, *FunctionType], error) {
	id := fmt.Sprintf("canon_lift_%s_fn_%d", comp.id, defs(comp.scope, sortFunction).len())
	fnTypeDef, err := defs(comp.scope, sortType).get(astDef.FunctionTypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnType, ok := fnTypeDef.typ().(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("canon lift function type is not a function")
	}
	return &functionLiftedDefinition{
		id:     id,
		astDef: astDef,
		fnType: fnType,
	}, nil
}

type functionLiftedDefinition struct {
	id     string
	astDef *ast.CanonLift
	fnType *FunctionType
}

func (d *functionLiftedDefinition) typ() *FunctionType {
	return d.fnType
}

func (d *functionLiftedDefinition) resolve(ctx context.Context, scope *instanceScope) (*Function, error) {
	fnTypeGeneric, err := resolve(ctx, scope, sortType, d.astDef.FunctionTypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnType, ok := fnTypeGeneric.(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("canon lift function type is not a function")
	}
	coreFn, err := resolve(ctx, scope, sortCoreFunction, d.astDef.CoreFuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core function for canon lift: %w", err)
	}

	_, _, paramsFlat, returnFlat := liftedCoreFunctionTypesFromFunctionType(fnType)

	return NewFunction(
		fnType,
		func(ctx context.Context, params []Value) (Value, error) {
			inst := scope.currentInstance
			if err := inst.enter(ctx); err != nil {
				return nil, err
			}

			result, err := func() (Value, error) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					return nil, fmt.Errorf("failed to create lift/load context for canon lower: %w", err)
				}

				var flatParams []uint64
				if paramsFlat {
					for i, pType := range fnType.Parameters {
						flatVals, err := pType.Type.lowerFlat(llc, params[i])
						if err != nil {
							return nil, fmt.Errorf("failed to lower parameter %d for canon lift: %w", i, err)
						}
						flatParams = append(flatParams, flatVals...)
					}
				} else {
					paramTypes := make([]ValueType, len(fnType.Parameters))
					for i, p := range fnType.Parameters {
						paramTypes[i] = p.Type
					}
					tt := TupleType(paramTypes...)
					offset, err := llc.realloc(0, 0, uint32(tt.alignment()), uint32(tt.elementSize()))
					if err != nil {
						return nil, fmt.Errorf("failed to realloc for canon lower parameters: %w", err)
					}
					err = tt.store(llc, offset, &Record{fields: params})
					if err != nil {
						return nil, fmt.Errorf("failed to load parameters for canon lower: %w", err)
					}
					flatParams = []uint64{uint64(offset)}
				}

				coreFnInst := coreFn.module.ExportedFunction(coreFn.name)
				results, err := coreFnInst.Call(ctx, flatParams...)
				if err != nil {
					return nil, fmt.Errorf("failed to call core function for canon lift: %w", err)
				}

				var returnValue Value

				if fnType.ResultType != nil {
					if returnFlat {
						val, err := fnType.ResultType.liftFlat(llc, func() uint64 {
							val := results[0]
							results = results[1:]
							return val
						})
						if err != nil {
							return nil, fmt.Errorf("failed to lift result for canon lift: %w", err)
						}
						returnValue = val
					} else {
						// Too many results to return flatly - read from a memory block instead
						offset := uint32(results[0])
						val, err := fnType.ResultType.load(llc, offset)
						if err != nil {
							return nil, fmt.Errorf("failed to load result for canon lift: %w", err)
						}
						returnValue = val
					}
				}

				if llc.postreturn != nil {
					_, err := llc.postreturn.Call(ctx, results...)
					if err != nil {
						return nil, fmt.Errorf("failed to call post return function for canon lift: %w", err)
					}
				}
				return returnValue, nil
			}()

			exitErr := inst.exit()
			if err != nil {
				return nil, err
			}
			return result, exitErr
		},
	), nil
}

func liftedCoreFunctionTypesFromFunctionType(fnType *FunctionType) ([]api.ValueType, []api.ValueType, bool, bool) {
	var flatParamTypes []api.ValueType
	var flatResultTypes []api.ValueType

	for _, p := range fnType.Parameters {
		flatParamTypes = append(flatParamTypes, p.Type.flatTypes()...)
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

	return flatParamTypes, flatResultTypes, paramsFlat, returnFlat
}

func newLiftLoadContext(ctx context.Context, opts []ast.CanonOpt, scope *instanceScope) (*LiftLoadContext, error) {
	llc := &LiftLoadContext{
		instance: scope.currentInstance,
		ctx:      ctx,
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
			mem, err := resolve(ctx, scope, sortCoreMemory, o.MemoryIdx)
			if err != nil {
				return nil, err
			}
			llc.memory = mem.memory
		case *ast.ReallocOpt:
			coreFn, err := resolve(ctx, scope, sortCoreFunction, o.FuncIdx)
			if err != nil {
				return nil, err
			}
			reallocFn := coreFn.module.ExportedFunction(coreFn.name)
			llc.realloc = func(originalPtr, originalSize, alignment, newSize uint32) (uint32, error) {
				results, err := reallocFn.Call(ctx, uint64(originalPtr), uint64(originalSize), uint64(alignment), uint64(newSize))
				if err != nil || len(results) != 1 {
					return 0, err
				}
				ptr := uint32(results[0])
				if ptr == 0xffffffff {
					return 0, fmt.Errorf("realloc return: beyond end of memory")
				}
				return ptr, nil
			}
		case *ast.PostReturnOpt:
			coreFn, err := resolve(ctx, scope, sortCoreFunction, o.FuncIdx)
			if err != nil {
				return nil, err
			}
			postReturnFn := coreFn.module.ExportedFunction(coreFn.name)
			llc.postreturn = postReturnFn
		default:
			return nil, fmt.Errorf("unknown canon lift/load option: %T", opt)
		}
	}

	return llc, nil
}
