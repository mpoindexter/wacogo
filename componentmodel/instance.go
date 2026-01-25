package componentmodel

import (
	"context"
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/ast"
)

type InstanceBuilder struct {
	exports  map[string]any
	types    map[string]Type
	instance *Instance
}

func NewInstanceBuilder() *InstanceBuilder {
	instance := newInstance()
	return &InstanceBuilder{
		exports:  instance.exports,
		types:    instance.exportTypes,
		instance: instance,
	}
}

func (b *InstanceBuilder) AddTypeExport(name string, typ Type) {
	b.exports[name] = typ
	b.types[name] = typ
}

func (b *InstanceBuilder) AddFunctionExport(name string, fnFactory func(instance *Instance) *Function) {
	fn := fnFactory(b.instance)
	b.exports[name] = fn
	b.types[name] = fn.typ()
}

func (b *InstanceBuilder) CreateResourceType(repType reflect.Type, destructor func(ctx context.Context, res any)) *ResourceType {
	resourceType := newResourceType(b.instance, repType, destructor)
	return resourceType
}

func (b *InstanceBuilder) Build() *Instance {
	return b.instance
}

type Instance struct {
	exports        map[string]any
	exportTypes    map[string]Type
	active         bool
	currentContext context.Context
	loweredHandles *table[ResourceHandle]
	borrowCount    uint32
}

func newInstance() *Instance {
	return &Instance{
		exports:        make(map[string]any),
		exportTypes:    make(map[string]Type),
		loweredHandles: newTable[ResourceHandle](),
	}
}

func (i *Instance) Export(name string) (any, bool) {
	val, ok := i.exports[name]
	return val, ok
}

func (i *Instance) enter(ctx context.Context) error {
	if i.active {
		return fmt.Errorf("instance is already active")
	}
	i.active = true
	i.currentContext = ctx
	return nil
}

func (i *Instance) exit() error {
	if !i.active {
		panic("instance is not active")
	}
	if i.borrowCount > 0 {
		return fmt.Errorf("cannot exit instance: there are still borrowed handles")
	}
	i.currentContext = nil
	i.active = false
	return nil
}

func (i *Instance) typ() *instanceType {
	return &instanceType{
		exports: i.exportTypes,
	}
}

func (i *Instance) getExport(name string) (any, Type, error) {
	val, ok := i.exports[name]
	if !ok {
		return nil, nil, fmt.Errorf("export %s not found", name)
	}
	typ, ok := i.exportTypes[name]
	if !ok {
		return nil, nil, fmt.Errorf("export type for %s not found", name)
	}
	return val, typ, nil
}

type instanceType struct {
	exports map[string]Type
}

func newInstanceType(exports map[string]Type) *instanceType {
	return &instanceType{
		exports: exports,
	}
}

func (it *instanceType) typ() Type {
	return it
}

func (it *instanceType) exportType(name string) (Type, error) {
	et, ok := it.exports[name]
	if !ok {
		return nil, fmt.Errorf("export %s not found", name)
	}
	return et, nil
}

func (it *instanceType) assignableFrom(other Type) bool {
	otherIt, ok := other.(*instanceType)
	if !ok {
		return false
	}

	// all exports in this type are present in the other type
	for name, exportType := range it.exports {
		otherExportType, ok := otherIt.exports[name]
		if !ok || !exportType.assignableFrom(otherExportType) {
			return false
		}
	}
	return true
}

type instantiateDefinition struct {
	componentIdx uint32
	args         []ast.InstantiateArg
	instanceType *instanceType
}

func newInstantiateDefinition(componentIdx uint32, args []ast.InstantiateArg, instanceType *instanceType) *instantiateDefinition {
	return &instantiateDefinition{
		componentIdx: componentIdx,
		args:         args,
		instanceType: instanceType,
	}
}

func (d *instantiateDefinition) typ() *instanceType {
	return d.instanceType
}

func (d *instantiateDefinition) exportType(name string) (Type, error) {
	et, ok := d.instanceType.exports[name]
	if !ok {
		return nil, fmt.Errorf("export %s not found", name)
	}
	return et, nil
}

func (d *instantiateDefinition) resolve(ctx context.Context, scope *instanceScope) (*Instance, error) {
	args := make(map[string]*instanceArgument)
	for _, astArg := range d.args {
		val, typ, err := resolveSortIdx(ctx, scope, astArg.SortIdx)
		if err != nil {
			return nil, err
		}
		args[astArg.Name] = &instanceArgument{val: val, typ: typ}
	}
	childComp, err := resolve(ctx, scope, sortComponent, d.componentIdx)
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
	exportTypes map[string]Type
}

func newInlineExportsDefinition(exports []ast.InlineExport, exportTypes map[string]Type) *inlineExportsDefinition {
	return &inlineExportsDefinition{
		exports:     exports,
		exportTypes: exportTypes,
	}
}

func (d *inlineExportsDefinition) typ() *instanceType {
	return newInstanceType(d.exportTypes)
}

func (d *inlineExportsDefinition) exportType(name string) (Type, error) {
	et, ok := d.exportTypes[name]
	if !ok {
		return nil, fmt.Errorf("export %s not found", name)
	}
	return et, nil
}

func (d *inlineExportsDefinition) resolve(ctx context.Context, scope *instanceScope) (*Instance, error) {
	instance := newInstance()

	for _, export := range d.exports {
		val, typ, err := resolveSortIdx(ctx, scope, &export.SortIdx)
		if err != nil {
			return nil, err
		}
		instance.exports[export.Name] = val
		instance.exportTypes[export.Name] = typ
	}

	return instance, nil
}
