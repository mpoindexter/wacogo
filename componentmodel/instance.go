package componentmodel

import (
	"context"
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

type InstanceBuilder struct {
	exports     map[string]any
	exportSpecs map[string]*exportSpec
	instance    *Instance
}

func NewInstanceBuilder() *InstanceBuilder {
	instance := newInstance()
	return &InstanceBuilder{
		exports:     instance.exports,
		exportSpecs: instance.exportSpecs,
		instance:    instance,
	}
}

func (b *InstanceBuilder) AddTypeExport(name string, typ Type) *InstanceBuilder {
	b.exports[name] = typ
	b.exportSpecs[name] = &exportSpec{typ: typ, sort: sortType}
	return b
}

func (b *InstanceBuilder) AddFunctionExport(name string, fnFactory func(instance *Instance) *Function) *InstanceBuilder {
	fn := fnFactory(b.instance)
	fn.funcTyp.skipParamNameCheck = true
	b.exports[name] = fn
	b.exportSpecs[name] = &exportSpec{typ: fn.funcTyp, sort: sortFunction}
	return b
}

func (b *InstanceBuilder) AddInstanceExport(name string, instance *Instance) *InstanceBuilder {
	b.exports[name] = instance
	b.exportSpecs[name] = &exportSpec{typ: newInstanceType(instance.exportSpecs, func() map[string]*exportSpec {
		return instance.exportSpecs
	}), sort: sortInstance}
	return b
}

func (b *InstanceBuilder) AddCoreModuleExport(name string, mod wazero.CompiledModule, additionalExterns *wasm.Externs) *InstanceBuilder {
	coreModule := newCoreModule(mod, additionalExterns)
	b.exports[name] = coreModule
	b.exportSpecs[name] = &exportSpec{typ: coreModule.typ(), sort: sortCoreModule}
	return b
}

func (b *InstanceBuilder) CreateResourceType(repType reflect.Type, destructor func(ctx context.Context, res any)) *ResourceType {
	resourceType := newResourceType(b.instance, destructor)
	return resourceType
}

func (b *InstanceBuilder) Init(init func(*InstanceBuilder)) *InstanceBuilder {
	init(b)
	return b
}

func (b *InstanceBuilder) Build() *Instance {
	return b.instance
}

type Instance struct {
	exports        map[string]any
	exportSpecs    map[string]*exportSpec
	active         bool
	mayLeave       bool
	currentContext context.Context
	loweredHandles *table[ResourceHandle]
	borrowCount    uint32
}

func newInstance() *Instance {
	return &Instance{
		exports:        make(map[string]any),
		exportSpecs:    make(map[string]*exportSpec),
		loweredHandles: newTable[ResourceHandle](),
		mayLeave:       true,
	}
}

func (i *Instance) Export(name string) (any, bool) {
	val, ok := i.exports[name]
	return val, ok
}

func (i *Instance) enter(ctx context.Context) error {
	if i.active {
		return fmt.Errorf("cannot enter component instance: already active")
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
		return fmt.Errorf("cannot leave component instance: there are still borrowed handles")
	}
	i.currentContext = nil
	i.active = false
	return nil
}

func (i *Instance) preventLeave() func() {
	i.mayLeave = false
	return func() {
		i.mayLeave = true
	}
}

func (i *Instance) checkLeave() error {
	if !i.mayLeave {
		return fmt.Errorf("cannot leave component instance: leaving is currently prevented")
	}
	return nil
}

func (i *Instance) getExport(name string) (any, error) {
	val, ok := i.exports[name]
	if !ok {
		return nil, fmt.Errorf("export %s not found", name)
	}
	return val, nil
}

type instanceType struct {
	exports map[string]*exportSpec
	newCopy func() map[string]*exportSpec
}

func newInstanceType(exports map[string]*exportSpec, newCopy func() map[string]*exportSpec) *instanceType {
	return &instanceType{
		exports: exports,
		newCopy: newCopy,
	}
}

func (it *instanceType) clone() *instanceType {
	if it.newCopy != nil {
		return &instanceType{
			exports: it.newCopy(),
		}
	}
	return it
}

func (it *instanceType) isType() {}

func (it *instanceType) typeName() string {
	return "instance"
}

func (it *instanceType) exportType(name string) (Type, bool) {
	spec, ok := it.exports[name]
	if !ok {
		return nil, false
	}
	return spec.typ, ok
}

func (it *instanceType) checkType(other Type, typeChecker typeChecker) error {
	oit, err := assertTypeKindIsSame(it, other)
	if err != nil {
		return err
	}

	// all exports in this type are present in the other type, unless they are statically
	// known
	for name, exportSpec := range it.exports {
		otherExportSpec, ok := oit.exports[name]
		if !ok {
			if exportSpec.sort == sortType && isStaticallyKnownType(exportSpec.typ) {
				continue
			}
			return fmt.Errorf("type mismatch: missing expected export `%s` in instance type", name)
		}

		if exportSpec.sort != otherExportSpec.sort {
			return fmt.Errorf("type mismatch in instance export `%s`: expected %s, found %s", name, exportSpec.typ.typeName(), otherExportSpec.sort.typeName())
		}
		if err := typeChecker.checkTypeCompatible(exportSpec.typ, otherExportSpec.typ); err != nil {
			return fmt.Errorf("type mismatch in instance export `%s`: %w", name, err)
		}
	}
	return nil
}

func (it *instanceType) typeSize() int {
	size := 1
	for _, exportSpec := range it.exports {
		size += exportSpec.typ.typeSize()
	}
	return size
}

func (it *instanceType) typeDepth() int {
	maxDepth := 0
	for _, exportSpec := range it.exports {
		if d := exportSpec.typ.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	return 1 + maxDepth
}

type instantiateDefinition struct {
	astDef *ast.Instantiate
}

func newInstantiateDefinition(astDef *ast.Instantiate) *instantiateDefinition {
	return &instantiateDefinition{
		astDef: astDef,
	}
}

func (d *instantiateDefinition) isDefinition() {}

func (d *instantiateDefinition) createType(scope *scope) (*instanceType, error) {
	componentType, err := sortScopeFor(scope, sortComponent).getType(d.astDef.ComponentIdx)
	if err != nil {
		return nil, fmt.Errorf("unknown component: %w", err)
	}

	argTypes := make(map[string]Type)
	for _, astArg := range d.astDef.Args {
		typ, err := typeForSortIdx(scope, astArg.SortIdx)
		if err != nil {
			return nil, err
		}
		argTypes[astArg.Name] = typ
	}

	requiredArgs := componentType.imports

	typeChecker := newTypeChecker()
	for argName, expectedArgType := range requiredArgs {
		actualType, ok := resolveArgumentValue(argTypes, argName)
		if !ok {
			return nil, fmt.Errorf("missing import named `%s`", argName)
		}
		if err := typeChecker.checkTypeCompatible(expectedArgType, actualType); err != nil {
			return nil, fmt.Errorf("type mismatch for import `%s`: expected %s, found %s: %w", argName, expectedArgType.typeName(), actualType.typeName(), err)
		}
	}

	return componentType.instanceType(scope, argTypes)
}

func (d *instantiateDefinition) createInstance(ctx context.Context, scope *scope) (*Instance, error) {
	comp, err := sortScopeFor(scope, sortComponent).getInstance(d.astDef.ComponentIdx)
	if err != nil {
		return nil, err
	}
	args := make(map[string]*instanceArgument)
	for _, astArg := range d.astDef.Args {
		val, err := instanceForSortIdx(scope, astArg.SortIdx)
		if err != nil {
			return nil, err
		}
		typ, err := typeForSortIdx(scope, astArg.SortIdx)
		if err != nil {
			return nil, err
		}
		args[astArg.Name] = &instanceArgument{val: val, typ: typ}
	}

	inst, err := comp.instantiate(ctx, args)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

type inlineExportsDefinition struct {
	exports []ast.InlineExport
}

func newInlineExportsDefinition(exports []ast.InlineExport) *inlineExportsDefinition {
	return &inlineExportsDefinition{
		exports: exports,
	}
}

func (d *inlineExportsDefinition) isDefinition() {}

func (d *inlineExportsDefinition) createType(scope *scope) (*instanceType, error) {
	exportSpecs := make(map[string]*exportSpec)
	for _, export := range d.exports {
		typ, err := typeForSortIdx(scope, &export.SortIdx)
		if err != nil {
			return nil, err
		}
		exportSpecs[export.Name] = &exportSpec{typ: typ, sort: sortForSortID(uint32(export.SortIdx.Sort))}
	}

	return newInstanceType(exportSpecs, nil), nil
}

func (d *inlineExportsDefinition) createInstance(ctx context.Context, scope *scope) (*Instance, error) {
	instance := newInstance()

	for _, export := range d.exports {
		val, err := instanceForSortIdx(scope, &export.SortIdx)
		if err != nil {
			return nil, err
		}
		typ, err := typeForSortIdx(scope, &export.SortIdx)
		if err != nil {
			return nil, err
		}
		instance.exports[export.Name] = val
		instance.exportSpecs[export.Name] = &exportSpec{typ: typ, sort: sortForSortID(uint32(export.SortIdx.Sort))}
	}

	return instance, nil
}

type instanceTypeResolver struct {
	definitions *definitions
	exports     map[string]componentExport
}

func newInstanceTypeResolver(definitions *definitions, exports map[string]componentExport) *instanceTypeResolver {
	return &instanceTypeResolver{
		definitions: definitions,
		exports:     exports,
	}
}

func (r *instanceTypeResolver) resolveType(scope *scope) (Type, error) {
	makeExports := func() (map[string]*exportSpec, error) {
		instanceScope := scope.componentScope(nil)
		for _, binder := range r.definitions.binders {
			if err := binder.bindType(instanceScope); err != nil {
				return nil, err
			}
		}
		exportSpecs := make(map[string]*exportSpec)
		for name, export := range r.exports {
			typ, err := export.typ(instanceScope)
			if err != nil {
				return nil, err
			}

			if it, ok := typ.(*instanceType); ok {
				typ = it.clone()
			}

			exportSpecs[name] = &exportSpec{typ: typ, sort: export.sort()}
		}

		return exportSpecs, nil
	}

	exportSpecs, err := makeExports()
	if err != nil {
		return nil, err
	}

	it := newInstanceType(exportSpecs, func() map[string]*exportSpec {
		ets, _ := makeExports()
		return ets
	})
	if it.typeSize() > maxTypeSize {
		return nil, fmt.Errorf("effective type size exceeds the limit")
	}
	return it, nil
}

func (r *instanceTypeResolver) typeInfo(scope *scope) *typeInfo {
	size := 1
	for _, export := range r.exports {
		typ, err := export.typ(scope)
		if err != nil {
			// Handle error appropriately, possibly returning nil or logging
			continue
		}
		size += typ.typeSize()
	}
	return &typeInfo{
		typeName: "instance",
		size:     size,
	}
}
