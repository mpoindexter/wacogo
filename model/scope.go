package model

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero"
)

type definitionScope interface {
	resolveFunctionDefinition(count uint32, idx uint32) (functionDefinition, error)
	resolveInstanceDefinition(count uint32, idx uint32) (instanceDefinition, error)
	resolveComponentDefinition(count uint32, idx uint32) (componentDefinition, error)
	resolveCoreFunctionDefinition(count uint32, idx uint32) (coreFunctionDefinition, error)
	resolveCoreMemoryDefinition(count uint32, idx uint32) (coreMemoryDefinition, error)
	resolveCoreTableDefinition(count uint32, idx uint32) (coreTableDefinition, error)
	resolveCoreGlobalDefinition(count uint32, idx uint32) (coreGlobalDefinition, error)
	resolveCoreModuleDefinition(count uint32, idx uint32) (coreModuleDefinition, error)
	resolveCoreInstanceDefinition(count uint32, idx uint32) (coreInstanceDefinition, error)
	resolveComponentModelTypeDefinition(count uint32, idx uint32) (componentModelTypeDefinition, error)
	resolveCoreTypeDefinition(count uint32, idx uint32) (coreTypeDefinition, error)
}

type instanceScope interface {
	definitionScope

	currentInstance() *Instance
	resolveInstance(ctx context.Context, idx uint32) (*Instance, error)
	resolveCoreInstance(ctx context.Context, idx uint32) (*coreInstance, error)
	resolveArgument(name string) (any, error)
	runtime() wazero.Runtime
}

type componentDefinitionScope struct {
	parent definitionScope

	functions           []functionDefinition
	instances           []instanceDefinition
	components          []componentDefinition
	componentModelTypes []componentModelTypeDefinition

	coreFunctions []coreFunctionDefinition
	coreMemories  []coreMemoryDefinition
	coreTables    []coreTableDefinition
	coreGlobals   []coreGlobalDefinition
	coreModules   []coreModuleDefinition
	coreInstances []coreInstanceDefinition
	coreTypes     []coreTypeDefinition
}

var _ definitionScope = (*componentDefinitionScope)(nil)

func (s *componentDefinitionScope) resolveFunctionDefinition(count uint32, idx uint32) (functionDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve function definition")
		}
		return s.parent.resolveFunctionDefinition(count-1, idx)
	}
	if int(idx) >= len(s.functions) {
		return nil, fmt.Errorf("function index %d not found", idx)
	}
	return s.functions[idx], nil
}

func (s *componentDefinitionScope) resolveInstanceDefinition(count uint32, idx uint32) (instanceDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve instance definition")
		}
		return s.parent.resolveInstanceDefinition(count-1, idx)
	}
	if int(idx) >= len(s.instances) {
		return nil, fmt.Errorf("instance index %d not found", idx)
	}
	return s.instances[idx], nil
}

func (s *componentDefinitionScope) resolveComponentDefinition(count uint32, idx uint32) (componentDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve component definition")
		}
		return s.parent.resolveComponentDefinition(count-1, idx)
	}
	if int(idx) >= len(s.components) {
		return nil, fmt.Errorf("component index %d not found", idx)
	}
	return s.components[idx], nil
}

func (s *componentDefinitionScope) resolveCoreFunctionDefinition(count uint32, idx uint32) (coreFunctionDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core function definition")
		}
		return s.parent.resolveCoreFunctionDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreFunctions) {
		return nil, fmt.Errorf("core function index %d not found", idx)
	}
	return s.coreFunctions[idx], nil
}

func (s *componentDefinitionScope) resolveCoreMemoryDefinition(count uint32, idx uint32) (coreMemoryDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core memory definition")
		}
		return s.parent.resolveCoreMemoryDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreMemories) {
		return nil, fmt.Errorf("core memory index %d not found", idx)
	}
	return s.coreMemories[idx], nil
}

func (s *componentDefinitionScope) resolveCoreTableDefinition(count uint32, idx uint32) (coreTableDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core table definition")
		}
		return s.parent.resolveCoreTableDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreTables) {
		return nil, fmt.Errorf("core table index %d not found", idx)
	}
	return s.coreTables[idx], nil
}

func (s *componentDefinitionScope) resolveCoreGlobalDefinition(count uint32, idx uint32) (coreGlobalDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core global definition")
		}
		return s.parent.resolveCoreGlobalDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreGlobals) {
		return nil, fmt.Errorf("core global index %d not found", idx)
	}
	return s.coreGlobals[idx], nil
}

func (s *componentDefinitionScope) resolveCoreModuleDefinition(count uint32, idx uint32) (coreModuleDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core module definition")
		}
		return s.parent.resolveCoreModuleDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreModules) {
		return nil, fmt.Errorf("core module index %d not found", idx)
	}
	return s.coreModules[idx], nil
}

func (s *componentDefinitionScope) resolveCoreInstanceDefinition(count uint32, idx uint32) (coreInstanceDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core instance definition")
		}
		return s.parent.resolveCoreInstanceDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreInstances) {
		return nil, fmt.Errorf("core instance index %d not found", idx)
	}
	return s.coreInstances[idx], nil
}

func (s *componentDefinitionScope) resolveComponentModelTypeDefinition(count uint32, idx uint32) (componentModelTypeDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve component model type definition")
		}
		return s.parent.resolveComponentModelTypeDefinition(count-1, idx)
	}
	if int(idx) >= len(s.componentModelTypes) {
		return nil, fmt.Errorf("component model type index %d not found", idx)
	}
	return s.componentModelTypes[idx], nil
}

func (s *componentDefinitionScope) resolveCoreTypeDefinition(count uint32, idx uint32) (coreTypeDefinition, error) {
	if count > 0 {
		if s.parent == nil {
			return nil, fmt.Errorf("no parent scope to resolve core type definition")
		}
		return s.parent.resolveCoreTypeDefinition(count-1, idx)
	}
	if int(idx) >= len(s.coreTypes) {
		return nil, fmt.Errorf("core type index %d not found", idx)
	}
	return s.coreTypes[idx], nil
}

func resolveSortIdx(ctx context.Context, scope instanceScope, sortIdx *ast.SortIdx) (any, error) {
	switch sortIdx.Sort {
	case ast.SortCoreFunc:
		def, err := scope.resolveCoreFunctionDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		_, _, fnDecl, err := def.resolveCoreFunction(ctx, scope)
		if err != nil {
			return nil, err
		}
		return fnDecl, nil
	case ast.SortCoreTable:
		return nil, fmt.Errorf("core table resolution not yet supported")
	case ast.SortCoreMemory:
		def, err := scope.resolveCoreMemoryDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		_, _, mem, err := def.resolveMemory(ctx, scope)
		if err != nil {
			return nil, err
		}
		return mem, nil
	case ast.SortCoreGlobal:
		def, err := scope.resolveCoreGlobalDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		_, _, glob, err := def.resolveGlobal(ctx, scope)
		if err != nil {
			return nil, err
		}
		return glob, nil
	case ast.SortCoreType:
		def, err := scope.resolveCoreTypeDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return def.resolveCoreType(ctx, scope)
	case ast.SortCoreModule:
		def, err := scope.resolveCoreModuleDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return def.resolveCoreModule(ctx, scope)
	case ast.SortCoreInstance:
		inst, err := scope.resolveCoreInstance(ctx, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return inst, nil
	case ast.SortFunc:
		def, err := scope.resolveFunctionDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return def.resolveFunction(ctx, scope)
	case ast.SortType:
		def, err := scope.resolveComponentModelTypeDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return def.resolveType(ctx, scope)
	case ast.SortComponent:
		def, err := scope.resolveComponentDefinition(0, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return def.resolveComponent(ctx, scope)
	case ast.SortInstance:
		inst, err := scope.resolveInstance(ctx, sortIdx.Idx)
		if err != nil {
			return nil, err
		}
		return inst, nil
	default:
		return nil, fmt.Errorf("unsupported sort: %v", sortIdx.Sort)
	}
}
