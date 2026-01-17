package componentmodel

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

type instantiation struct {
	definitionScope
	parent        instanceScope
	instance      *Instance
	args          map[string]any
	wazeroRuntime wazero.Runtime

	coreInstances []*coreInstance
	instances     []*Instance
	types         map[any]Type
	typeDepth     int
}

var _ instanceScope = (*instantiation)(nil)

func newInstantiation(parent instanceScope, component *Component, args map[string]any) *instantiation {
	return &instantiation{
		parent:          parent,
		definitionScope: &component.scope,
		instance:        newInstance(),
		args:            args,
		wazeroRuntime:   component.runtime,
		coreInstances:   make([]*coreInstance, len(component.scope.coreInstances)),
		instances:       make([]*Instance, len(component.scope.instances)),
		types:           make(map[any]Type, len(component.scope.componentModelTypes)),
	}
}

func (i *instantiation) currentInstance() *Instance {
	return i.instance
}

func (i *instantiation) resolveArgument(name string) (any, error) {
	val, ok := i.args[name]
	if !ok {
		return nil, fmt.Errorf("argument %s not found", name)
	}
	return val, nil
}

func (i *instantiation) runtime() wazero.Runtime {
	return i.wazeroRuntime
}

func (i *instantiation) resolveInstance(ctx context.Context, idx uint32) (*Instance, error) {
	if int(idx) >= len(i.instances) {
		return nil, fmt.Errorf("invalid instance index: %d", idx)
	}
	if i.instances[idx] == nil {
		def, err := i.definitionScope.resolveInstanceDefinition(0, idx)
		if err != nil {
			return nil, err
		}
		inst, err := def.resolveInstance(ctx, i)
		if err != nil {
			return nil, err
		}
		i.instances[idx] = inst
	}
	return i.instances[idx], nil
}

func (i *instantiation) resolveCoreInstance(ctx context.Context, idx uint32) (*coreInstance, error) {
	if int(idx) >= len(i.coreInstances) {
		return nil, fmt.Errorf("invalid core instance index: %d", idx)
	}
	if i.coreInstances[idx] == nil {
		def, err := i.definitionScope.resolveCoreInstanceDefinition(0, idx)
		if err != nil {
			return nil, err
		}
		inst, err := def.resolveCoreInstance(ctx, i)
		if err != nil {
			return nil, err
		}
		i.coreInstances[idx] = inst
	}
	return i.coreInstances[idx], nil
}

func (i *instantiation) resolveType(ctx context.Context, def componentModelTypeDefinition) (Type, error) {
	i.typeDepth++
	defer func() {
		i.typeDepth--
	}()
	if i.typeDepth > maxTypeRecursionDepth {
		return nil, fmt.Errorf("type nesting is too deep")
	}
	// call before memoization to handle recursive type depth checking
	typ, err := def.resolveType(ctx, i)
	if err != nil {
		return nil, err
	}

	if memoTyp, ok := i.types[def]; ok {
		return memoTyp, nil
	}

	i.types[def] = typ
	return typ, nil
}
