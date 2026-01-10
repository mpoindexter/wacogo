package model

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
)

type Instance struct {
	exports map[string]any
	tables  tables
}

func (i *Instance) Export(name string) (any, bool) {
	val, ok := i.exports[name]
	return val, ok
}

func newInstance() *Instance {
	return &Instance{
		exports: make(map[string]any),
		tables:  newTables(),
	}
}

func NewInstanceOf(exports map[string]any) *Instance {
	return &Instance{
		exports: exports,
		tables:  newTables(),
	}
}

type instanceDefinition interface {
	resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error)
}

type instantiateDefinition struct {
	componentIdx uint32
	instanceIdx  uint32
	args         []ast.InstantiateArg
}

func (d *instantiateDefinition) resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error) {
	args := make(map[string]any)
	for _, astArg := range d.args {
		val, err := resolveSortIdx(ctx, scope, astArg.SortIdx)
		if err != nil {
			return nil, err
		}
		args[astArg.Name] = val
	}

	childCompDef, err := scope.resolveComponentDefinition(0, d.componentIdx)
	if err != nil {
		return nil, err
	}
	childComp, err := childCompDef.resolveComponent(ctx, scope)
	if err != nil {
		return nil, err
	}
	childInst, err := childComp.instantiate(ctx, args, scope)
	if err != nil {
		return nil, err
	}
	return childInst, nil
}

type inlineExportsDefinition struct {
	exports     []ast.InlineExport
	instanceIdx uint32
}

func (d *inlineExportsDefinition) resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error) {
	instance := newInstance()

	for _, export := range d.exports {
		val, err := resolveSortIdx(ctx, scope, &export.SortIdx)
		if err != nil {
			return nil, err
		}
		instance.exports[export.Name] = val
	}

	return instance, nil
}

type instanceAliasDefinition struct {
	instanceIdx uint32
	exportName  string
}

func (d *instanceAliasDefinition) resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error) {
	inst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}
	aliasInstAny, ok := inst.exports[d.exportName]
	if !ok {
		return nil, fmt.Errorf("instance export %s not found in instance %d", d.exportName, d.instanceIdx)
	}
	aliasInst, ok := aliasInstAny.(*Instance)
	if !ok {
		return nil, fmt.Errorf("export %s in instance %d is not an instance", d.exportName, d.instanceIdx)
	}
	return aliasInst, nil
}

type instanceStaticDefinition struct {
	instance *Instance
}

func (d *instanceStaticDefinition) resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error) {
	return d.instance, nil
}

type instanceImportDefinition struct {
	name string
}

func (d *instanceImportDefinition) resolveInstance(ctx context.Context, scope instanceScope) (*Instance, error) {
	val, err := scope.resolveArgument(d.name)
	if err != nil {
		return nil, err
	}
	inst, ok := val.(*Instance)
	if !ok {
		return nil, fmt.Errorf("import %s is not an instance", d.name)
	}
	return inst, nil
}
