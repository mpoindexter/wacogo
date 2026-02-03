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
	lentHandles    []ResourceHandle
}

type stringEncoding int

const (
	stringEncodingUTF8 stringEncoding = iota
	stringEncodingUTF16
	stringEncodingLatin1UTF16
)

func canonLower(id uint32, astDef *ast.CanonLower) (definition[*coreFunction, *coreFunctionType], error) {
	return &coreFunctionLoweredDefinition{
		id:     fmt.Sprintf("canon_lower_%d", id),
		astDef: astDef,
	}, nil
}

type coreFunctionLoweredDefinition struct {
	id     string
	astDef *ast.CanonLower
}

func (d *coreFunctionLoweredDefinition) isDefinition() {}

func (d *coreFunctionLoweredDefinition) createType(scope *scope) (*coreFunctionType, error) {
	fnType, err := sortScopeFor(scope, sortFunction).getType(d.astDef.FuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lower: %w", err)
	}
	flatParamTypes, flatResultTypes, _, _ := loweredCoreFunctionTypesFromFunctionType(fnType)
	paramTypes := make([]Type, len(flatParamTypes))
	for i, vt := range flatParamTypes {
		paramTypes[i] = coreTypeWasmConstTypeFromWazero(vt)
	}
	resultTypes := make([]Type, len(flatResultTypes))
	for i, vt := range flatResultTypes {
		resultTypes[i] = coreTypeWasmConstTypeFromWazero(vt)
	}
	return newCoreFunctionType(paramTypes, resultTypes), nil
}

func (d *coreFunctionLoweredDefinition) createInstance(ctx context.Context, scope *scope) (*coreFunction, error) {
	fn, err := sortScopeFor(scope, sortFunction).getInstance(d.astDef.FuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function for canon lower: %w", err)
	}

	fnTyp := fn.funcTyp

	flatParamTypes, flatResultTypes, paramsFlat, returnFlat := loweredCoreFunctionTypesFromFunctionType(fnTyp)

	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					panic(fmt.Errorf("failed to create lift/load context for canon lower: %w", err))
				}
				defer func() {
					for _, rh := range llc.lentHandles {
						rh.Drop()
					}
					llc.lentHandles = nil
				}()

				if err := llc.instance.checkLeave(); err != nil {
					panic(fmt.Errorf("cannot leave component instance during canon lower: %w", err))
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
					tt := NewTupleType(paramTypes...)
					if offset != alignTo(offset, tt.alignment()) {
						panic(fmt.Errorf("unaligned pointer for canon lower parameters"))
					}
					tup, err := tt.load(llc, offset)
					if err != nil {
						panic(fmt.Errorf("failed to load parameters for canon lower: %w", err))
					}
					paramValues = tup.(Record).fields
				}

				result, err := fn.invoke(ctx, paramValues)
				if err != nil {
					panic(fmt.Errorf("failed to call core function for canon lower: %w", err))
				}

				if fnTyp.ResultType != nil {
					func() {
						defer llc.instance.preventLeave()()
						if returnFlat {
							flatResults, err := fnTyp.ResultType.lowerFlat(llc, result)
							if err != nil {
								panic(fmt.Errorf("failed to lower result for canon lower: %w", err))
							}
							copy(stack, flatResults)
						} else {
							offset := uint32(itr())
							if offset != alignTo(offset, fnTyp.ResultType.alignment()) {
								panic(fmt.Errorf("unaligned pointer for canon lower results"))
							}
							err := fnTyp.ResultType.store(llc, offset, result)
							if err != nil {
								panic(fmt.Errorf("failed to store result for canon lower: %w", err))
							}
						}
					}()
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

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig().WithName(""))
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

func canonResourceNew(id uint32, astDef *ast.CanonResourceNew) (definition[*coreFunction, *coreFunctionType], error) {
	return &coreFunctionResourceNewDefinition{
		id:     fmt.Sprintf("canon_resource_new_%d", id),
		astDef: astDef,
	}, nil
}

type coreFunctionResourceNewDefinition struct {
	id     string
	astDef *ast.CanonResourceNew
}

func (d *coreFunctionResourceNewDefinition) isDefinition() {}

func (d *coreFunctionResourceNewDefinition) createType(scope *scope) (*coreFunctionType, error) {
	resourceType, err := sortScopeFor(scope, sortType).getType(d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}
	rt, ok := resourceType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.new type not a resource type")
	}

	if rt.instance != scope.instance {
		return nil, fmt.Errorf("canon resource.new type not a local resource")
	}

	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
	), nil
}

func (d *coreFunctionResourceNewDefinition) createInstance(ctx context.Context, scope *scope) (*coreFunction, error) {
	resourceTypeGeneric, err := sortScopeFor(scope, sortType).getInstance(d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.new: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.new type is not a resource")
	}

	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				instance := scope.instance
				if err := scope.instance.checkLeave(); err != nil {
					panic(fmt.Errorf("cannot leave component instance during canon resource.new: %w", err))
				}
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

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource new stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonResourceDrop(id uint32, astDef *ast.CanonResourceDrop) (definition[*coreFunction, *coreFunctionType], error) {
	return &coreFunctionResourceDropDefinition{
		id:     fmt.Sprintf("canon_resource_drop_%d", id),
		astDef: astDef,
	}, nil
}

type coreFunctionResourceDropDefinition struct {
	id     string
	astDef *ast.CanonResourceDrop
}

func (d *coreFunctionResourceDropDefinition) isDefinition() {}

func (d *coreFunctionResourceDropDefinition) createType(scope *scope) (*coreFunctionType, error) {
	resourceType, err := sortScopeFor(scope, sortType).getType(d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}

	if ti := newTypeInfo(resourceType); !ti.isResource {
		return nil, fmt.Errorf("canon resource.drop type not a resource type")
	}
	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{},
	), nil
}

func (d *coreFunctionResourceDropDefinition) createInstance(ctx context.Context, scope *scope) (*coreFunction, error) {
	resourceTypeGeneric, err := sortScopeFor(scope, sortType).getInstance(d.astDef.TypeIdx)
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
				instance := scope.instance
				if err := scope.instance.checkLeave(); err != nil {
					panic(fmt.Errorf("cannot leave component instance during canon resource.drop: %w", err))
				}
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

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonResourceRep(id uint32, astDef *ast.CanonResourceRep) (definition[*coreFunction, *coreFunctionType], error) {
	return &coreFunctionResourceRepDefinition{
		id:     fmt.Sprintf("canon_resource_rep_%d", id),
		astDef: astDef,
	}, nil
}

type coreFunctionResourceRepDefinition struct {
	id     string
	astDef *ast.CanonResourceRep
}

func (d *coreFunctionResourceRepDefinition) isDefinition() {}

func (d *coreFunctionResourceRepDefinition) createType(scope *scope) (*coreFunctionType, error) {
	resourceType, err := sortScopeFor(scope, sortType).getType(d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}

	rt, ok := resourceType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.rep type not a resource type")
	}
	if rt.instance != scope.instance {
		return nil, fmt.Errorf("canon resource.rep type not a local resource")
	}

	return newCoreFunctionType(
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
		[]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)},
	), nil
}

func (d *coreFunctionResourceRepDefinition) createInstance(ctx context.Context, scope *scope) (*coreFunction, error) {
	resourceTypeGeneric, err := sortScopeFor(scope, sortType).getType(d.astDef.TypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("canon resource.rep type is not a resource")
	}
	mod, err := scope.runtime.NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				instance := scope.instance
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

	modInst, err := scope.runtime.InstantiateModule(ctx, mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
	}

	return newCoreFunction(
		modInst, "stub_function", mod.ExportedFunctions()["stub_function"],
	), nil
}

func canonLift(id uint32, astDef *ast.CanonLift) (definition[*Function, *FunctionType], error) {
	return &functionLiftedDefinition{
		id:     fmt.Sprintf("canon_lift_%d", id),
		astDef: astDef,
	}, nil
}

type functionLiftedDefinition struct {
	id     string
	astDef *ast.CanonLift
}

func (d *functionLiftedDefinition) isDefinition() {}

func (d *functionLiftedDefinition) createType(scope *scope) (*FunctionType, error) {
	fnType, err := sortScopeFor(scope, sortType).getType(d.astDef.FunctionTypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnTypeCast, ok := fnType.(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("canon lift function type is not a function")
	}
	return fnTypeCast, nil
}

func (d *functionLiftedDefinition) createInstance(ctx context.Context, scope *scope) (*Function, error) {
	fnTypeGeneric, err := sortScopeFor(scope, sortType).getType(d.astDef.FunctionTypeIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function type for canon lift: %w", err)
	}
	fnType, ok := fnTypeGeneric.(*FunctionType)
	if !ok {
		return nil, fmt.Errorf("canon lift function type is not a function")
	}
	coreFn, err := sortScopeFor(scope, sortCoreFunction).getInstance(d.astDef.CoreFuncIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve core function for canon lift: %w", err)
	}

	_, _, paramsFlat, returnFlat := liftedCoreFunctionTypesFromFunctionType(fnType)

	return NewFunction(
		fnType,
		func(ctx context.Context, params []Value) (Value, error) {
			inst := scope.instance
			if err := inst.enter(ctx); err != nil {
				return nil, err
			}

			result, err := func() (Value, error) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					return nil, fmt.Errorf("failed to create lift/load context for canon lower: %w", err)
				}

				flatParams, err := func() ([]uint64, error) {
					defer inst.preventLeave()()
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
						tt := NewTupleType(paramTypes...)
						offset, err := llc.realloc(0, 0, uint32(tt.alignment()), uint32(tt.elementSize()))
						if err != nil {
							return nil, fmt.Errorf("failed to realloc for canon lower parameters: %w", err)
						}
						if offset != alignTo(offset, tt.alignment()) {
							return nil, fmt.Errorf("unaligned pointer for canon lower parameters")
						}
						err = tt.store(llc, offset, Record{fields: params})
						if err != nil {
							return nil, fmt.Errorf("failed to load parameters for canon lower: %w", err)
						}
						flatParams = []uint64{uint64(offset)}
					}
					return flatParams, nil
				}()

				if err != nil {
					return nil, err
				}

				coreFnInst := coreFn.module.ExportedFunction(coreFn.name)
				results, err := coreFnInst.Call(ctx, flatParams...)
				if err != nil {
					return nil, fmt.Errorf("failed to call core function for canon lift: %w", err)
				}

				var returnValue Value

				if fnType.ResultType != nil {
					remainingResults := results
					if returnFlat {
						val, err := fnType.ResultType.liftFlat(llc, func() uint64 {
							val := remainingResults[0]
							remainingResults = remainingResults[1:]
							return val
						})
						if err != nil {
							return nil, fmt.Errorf("failed to lift result for canon lift: %w", err)
						}
						returnValue = val
					} else {
						// Too many results to return flatly - read from a memory block instead
						offset := uint32(remainingResults[0])
						if offset != alignTo(offset, fnType.ResultType.alignment()) {
							return nil, fmt.Errorf("unaligned pointer for canon lift result")
						}
						val, err := fnType.ResultType.load(llc, offset)
						if err != nil {
							return nil, fmt.Errorf("failed to load result for canon lift: %w", err)
						}
						returnValue = val
					}
				}

				if llc.postreturn != nil {
					defer llc.instance.preventLeave()()
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

func newLiftLoadContext(ctx context.Context, opts []ast.CanonOpt, scope *scope) (*LiftLoadContext, error) {
	llc := &LiftLoadContext{
		instance: scope.instance,
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
			mem, err := sortScopeFor(scope, sortCoreMemory).getInstance(o.MemoryIdx)
			if err != nil {
				return nil, err
			}
			llc.memory = mem.memory
		case *ast.ReallocOpt:
			coreFn, err := sortScopeFor(scope, sortCoreFunction).getInstance(o.FuncIdx)
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
			coreFn, err := sortScopeFor(scope, sortCoreFunction).getInstance(o.FuncIdx)
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
