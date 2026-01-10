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

	passFlat := true
	if len(flatParamTypes) > 16 {
		passFlat = false
		flatParamTypes = []api.ValueType{api.ValueTypeI32}
	}

	returnFlat := true
	if len(flatResultTypes) > 1 {
		returnFlat = false
		flatParamTypes = append(flatParamTypes, api.ValueTypeI32)
		flatResultTypes = []api.ValueType{}
	}

	// TODO: Implement canon lower
	mod, err := scope.runtime().NewHostModuleBuilder(d.id).NewFunctionBuilder().
		WithGoModuleFunction(
			api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
				llc, err := newLiftLoadContext(ctx, d.astDef.Options, scope)
				if err != nil {
					panic(fmt.Errorf("failed to create lift/load context for canon lower: %w", err))
				}
				_ = passFlat
				_ = returnFlat
				_ = llc
				_ = fn
				// TODO: Implement canon lower logic
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
