package componentmodel

import (
	"context"
	"fmt"
	"slices"

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
	realloc        func(originalPtr, originalSize, alignment, newSize uint32) uint32
	postreturn     api.Function
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

	// Check if memory/realloc are needed and provided
	needsMemory := !paramsFlat || !returnFlat
	if slices.ContainsFunc(fn.typ.ParamTypes, typeNeedsMemory) {
		needsMemory = true
	}
	if fn.typ.ResultType != nil && typeNeedsMemory(fn.typ.ResultType) {
		needsMemory = true
	}

	needsRealloc := false
	if fn.typ.ResultType != nil && typeNeedsRealloc(fn.typ.ResultType) {
		needsRealloc = true
	}

	hasMemory := false
	hasRealloc := false
	for _, opt := range d.astDef.Options {
		switch opt.(type) {
		case *ast.MemoryOpt:
			hasMemory = true
		case *ast.ReallocOpt:
			hasRealloc = true
		}
	}

	if err := validateCanonicalABIOptions(hasMemory, hasRealloc, needsMemory, needsRealloc); err != nil {
		return nil, "", nil, fmt.Errorf("canon lower validation failed: %w", err)
	}

	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
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

					paramValues = make([]Value, 0, len(fn.typ.ParamTypes))
					for i, pType := range fn.typ.ParamTypes {
						val, err := pType.liftFlat(llc, itr)
						if err != nil {
							panic(fmt.Errorf("failed to load parameter %d for canon lower: %w", i, err))
						}
						paramValues = append(paramValues, val)
					}
				} else {
					offset := uint32(itr())
					tt := TupleType(fn.typ.ParamTypes...)
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

				if fn.typ.ResultType != nil {
					if returnFlat {
						flatResults, err := fn.typ.ResultType.lowerFlat(llc, result)
						if err != nil {
							panic(fmt.Errorf("failed to lower result for canon lower: %w", err))
						}
						copy(stack, flatResults)
					} else {
						offset := uint32(itr())
						err := fn.typ.ResultType.store(llc, offset, result)
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
		return nil, "", nil, fmt.Errorf("failed to create canon lower stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate canon lower stub module: %w", err)
	}

	return modInst, "stub_function", mod.ExportedFunctions()["stub_function"], nil
}

func canonResourceNew(comp *Component, astDef *ast.CanonResourceNew) (coreFunctionDefinition, error) {
	id := fmt.Sprintf("canon_resource_new_%s_fn_%d", comp.id, len(comp.scope.coreFunctions))
	return &coreFunctionResourceNewDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceNewDefinition struct {
	id     string
	astDef *ast.CanonResourceNew
}

func (d *coreFunctionResourceNewDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	typeDef, err := scope.resolveComponentModelTypeDefinition(0, d.astDef.TypeIdx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon resource.new: %w", err)
	}
	resourceTypeGeneric, err := scope.resolveType(ctx, typeDef)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon resource.new: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, "", nil, fmt.Errorf("canon resource.new type is not a resource")
	}
	if err := validateResourceTypeDefinedInComponent(resourceType, scope.currentInstance()); err != nil {
		return nil, "", nil, fmt.Errorf("canon resource.new validation failed: %w", err)
	}
	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				rep := uint32(stack[0])
				instance := scope.currentInstance()
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
		return nil, "", nil, fmt.Errorf("failed to create resource new stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate resource new stub module: %w", err)
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
	typeDef, err := scope.resolveComponentModelTypeDefinition(0, d.astDef.TypeIdx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon drop: %w", err)
	}
	resourceTypeGeneric, err := scope.resolveType(ctx, typeDef)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon drop: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, "", nil, fmt.Errorf("canon drop type is not a resource")
	}
	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				instance := scope.currentInstance()
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
		return nil, "", nil, fmt.Errorf("failed to create resource drop stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
	}

	return modInst, "stub_function", mod.ExportedFunctions()["stub_function"], nil
}

func canonResourceRep(comp *Component, astDef *ast.CanonResourceRep) (coreFunctionDefinition, error) {
	id := fmt.Sprintf("canon_resource_rep_%s_fn_%d", comp.id, len(comp.scope.coreFunctions))
	return &coreFunctionResourceRepDefinition{
		id:     id,
		astDef: astDef,
	}, nil
}

type coreFunctionResourceRepDefinition struct {
	id     string
	astDef *ast.CanonResourceRep
}

func (d *coreFunctionResourceRepDefinition) resolveCoreFunction(ctx context.Context, scope instanceScope) (api.Module, string, api.FunctionDefinition, error) {
	typeDef, err := scope.resolveComponentModelTypeDefinition(0, d.astDef.TypeIdx)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}
	resourceTypeGeneric, err := scope.resolveType(ctx, typeDef)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve resource type for canon resource.rep: %w", err)
	}
	resourceType, ok := resourceTypeGeneric.(*ResourceType)
	if !ok {
		return nil, "", nil, fmt.Errorf("canon resource.rep type is not a resource")
	}
	if err := validateResourceTypeDefinedInComponent(resourceType, scope.currentInstance()); err != nil {
		return nil, "", nil, fmt.Errorf("canon resource.rep validation failed: %w", err)
	}
	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				instance := scope.currentInstance()
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
		return nil, "", nil, fmt.Errorf("failed to create resource drop stub module: %w", err)
	}

	modInst, err := scope.runtime().InstantiateModule(ctx, mod, wazero.NewModuleConfig())
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to instantiate resource drop stub module: %w", err)
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
	fnTypeGeneric, err := scope.resolveType(ctx, fnTypeDef)
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

	// Check if memory/realloc are needed and provided
	needsMemory := !paramsFlat || !returnFlat
	if slices.ContainsFunc(fnType.ParamTypes, typeNeedsMemory) {
		needsMemory = true
	}
	if fnType.ResultType != nil && typeNeedsMemory(fnType.ResultType) {
		needsMemory = true
	}

	needsRealloc := slices.ContainsFunc(fnType.ParamTypes, typeNeedsRealloc)

	hasMemory := false
	hasRealloc := false
	for _, opt := range d.astDef.Options {
		switch opt.(type) {
		case *ast.MemoryOpt:
			hasMemory = true
		case *ast.ReallocOpt:
			hasRealloc = true
		}
	}

	if err := validateCanonicalABIOptions(hasMemory, hasRealloc, needsMemory, needsRealloc); err != nil {
		return nil, fmt.Errorf("canon lift validation failed: %w", err)
	}

	return NewFunction(
		fnType,
		func(ctx context.Context, params []Value) (Value, error) {
			inst := scope.currentInstance()
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
					for i, pType := range fnType.ParamTypes {
						flatVals, err := pType.lowerFlat(llc, params[i])
						if err != nil {
							return nil, fmt.Errorf("failed to lower parameter %d for canon lift: %w", i, err)
						}
						flatParams = append(flatParams, flatVals...)
					}
				} else {
					tt := TupleType(fnType.ParamTypes...)
					offset := llc.realloc(0, 0, uint32(tt.alignment()), uint32(tt.elementSize()))
					err := tt.store(llc, offset, &Record{fields: params})
					if err != nil {
						return nil, fmt.Errorf("failed to load parameters for canon lower: %w", err)
					}
					flatParams = []uint64{uint64(offset)}
				}

				results, err := coreFn.Call(ctx, flatParams...)
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

func newLiftLoadContext(ctx context.Context, opts []ast.CanonOpt, scope instanceScope) (*LiftLoadContext, error) {
	llc := &LiftLoadContext{
		instance: scope.currentInstance(),
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
			coreFnDef, err := scope.resolveCoreFunctionDefinition(0, o.FuncIdx)
			if err != nil {
				return nil, err
			}
			mod, fnName, _, err := coreFnDef.resolveCoreFunction(ctx, scope)
			if err != nil {
				return nil, err
			}
			postReturnFn := mod.ExportedFunction(fnName)
			llc.postreturn = postReturnFn
		default:
			return nil, fmt.Errorf("unknown canon lift/load option: %T", opt)
		}
	}

	return llc, nil
}
