package componentmodel

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

type Component struct {
	id      string
	runtime wazero.Runtime
	scope   componentDefinitionScope
	exports map[string]func(ctx context.Context, scope instanceScope) (any, error)
}

func newComponent(id string, runtime wazero.Runtime, parent definitionScope) *Component {
	return &Component{
		id:      id,
		runtime: runtime,
		scope:   componentDefinitionScope{parent: parent},
		exports: make(map[string]func(ctx context.Context, scope instanceScope) (any, error)),
	}
}

func (c *Component) Instantiate(ctx context.Context, args map[string]any) (*Instance, error) {
	return c.instantiate(ctx, args, nil)
}

func (c *Component) instantiate(ctx context.Context, args map[string]any, parentScope instanceScope) (*Instance, error) {
	instantiation := newInstantiation(parentScope, c, args)

	for i := range c.scope.coreInstances {
		_, err := instantiation.resolveCoreInstance(ctx, uint32(i))
		if err != nil {
			return nil, err
		}
	}

	for i := range c.scope.instances {
		_, err := instantiation.resolveInstance(ctx, uint32(i))
		if err != nil {
			return nil, err
		}
	}

	for exportName, exportFunc := range c.exports {
		val, err := exportFunc(ctx, instantiation)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate export %s: %v", exportName, err)
		}
		instantiation.instance.exports[exportName] = val
	}

	return instantiation.instance, nil
}

type componentDefinition interface {
	resolveComponent(ctx context.Context, scope instanceScope) (*Component, error)
}

type componentAliasDefinition struct {
	instanceIdx uint32
	exportName  string
}

func (d *componentAliasDefinition) resolveComponent(ctx context.Context, scope instanceScope) (*Component, error) {
	aliasInst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}
	compVal, ok := aliasInst.exports[d.exportName]
	if !ok {
		return nil, fmt.Errorf("export %s not found in instance %d", d.exportName, d.instanceIdx)
	}
	comp, ok := compVal.(*Component)
	if !ok {
		return nil, fmt.Errorf("export %s in instance %d is not a component", d.exportName, d.instanceIdx)
	}
	return comp, nil
}

type componentStaticDefinition struct {
	component *Component
}

func (d *componentStaticDefinition) resolveComponent(ctx context.Context, scope instanceScope) (*Component, error) {
	return d.component, nil
}

type componentImportDefinition struct {
	name            string
	expectedTypeDef componentModelTypeDefinition
}

func (d *componentImportDefinition) resolveComponent(ctx context.Context, scope instanceScope) (*Component, error) {
	val, err := scope.resolveArgument(d.name)
	if err != nil {
		return nil, err
	}
	comp, ok := val.(*Component)
	if !ok {
		return nil, fmt.Errorf("import %s is not a component", d.name)
	}

	expectedType, err := scope.resolveType(ctx, d.expectedTypeDef)
	if err != nil {
		return nil, err
	}

	componentType, ok := expectedType.(*componentType)
	if !ok {
		return nil, fmt.Errorf("expected type for component import %s is not a component type", d.name)
	}

	if err := componentType.validateComponent(comp); err != nil {
		return nil, fmt.Errorf("component import %s does not match expected type: %w", d.name, err)
	}
	return comp, nil
}
