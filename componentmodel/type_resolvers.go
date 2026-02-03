package componentmodel

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

type typeResolver interface {
	resolveType(scope *scope) (Type, error)
	typeInfo(scope *scope) *typeInfo
}

type typeInfo struct {
	isValue    bool
	isResource bool
	typeName   string
	depth      int
	size       int
}

func newTypeInfo(
	t Type,
) *typeInfo {
	vt, isValue := t.(ValueType)
	var depth int
	if isValue {
		depth = vt.typeDepth()
	}

	_, isResource := t.(*ResourceType)
	return &typeInfo{
		isValue:    isValue,
		isResource: isResource,
		typeName:   t.typeName(),
		depth:      depth,
		size:       t.typeSize(),
	}
}

type typeResolverDefinition struct {
	typeResolver typeResolver
}

func newTypeResolverDefinition(
	typeResolver typeResolver,
) *typeResolverDefinition {
	return &typeResolverDefinition{
		typeResolver: typeResolver,
	}
}

func (d *typeResolverDefinition) isDefinition() {}

func (d *typeResolverDefinition) createType(scope *scope) (Type, error) {
	t, err := d.typeResolver.resolveType(scope)
	if err != nil {
		return nil, err
	}
	if t.typeDepth() > maxValueTypeDepth {
		return nil, fmt.Errorf("type nesting is too deep")
	}
	return t, nil
}

func (d *typeResolverDefinition) createInstance(ctx context.Context, scope *scope) (Type, error) {
	// Avoid creating a new type instance; just return the current type.
	return scope.currentType, nil
}

type indexTypeResolver[V any, T Type] struct {
	sort   sort[V, T]
	idx    uint32
	assert func(t Type) error
}

func newIndexTypeResolverOf[T Type, SV any, ST Type](
	sort sort[SV, ST],
	idx uint32,
	msg string,
) *indexTypeResolver[SV, ST] {
	return &indexTypeResolver[SV, ST]{
		sort: sort,
		idx:  idx,
		assert: func(t Type) error {
			if _, ok := t.(T); !ok {
				expectedTypeName := typeNameOf[T]()
				article := "a"
				if first := expectedTypeName[0]; first == 'a' || first == 'e' || first == 'i' || first == 'o' || first == 'u' {
					article = "an"
				}
				if msg != "" {
					return fmt.Errorf("%s", msg)
				}
				return fmt.Errorf("%s index %d is not %s %s type, found %s", sort.typeName(), idx, article, expectedTypeName, t.typeName())
			}
			return nil
		},
	}
}
func newIndexTypeResolver[V any, T Type](
	sort sort[V, T],
	idx uint32,
	assert func(t Type) error,
) *indexTypeResolver[V, T] {
	return &indexTypeResolver[V, T]{
		sort:   sort,
		idx:    idx,
		assert: assert,
	}
}

func (d *indexTypeResolver[V, T]) resolveType(scope *scope) (Type, error) {
	t, err := sortScopeFor(scope, d.sort).getType(d.idx)
	if err != nil {
		return nil, err
	}
	if err := d.assert(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (d *indexTypeResolver[V, T]) typeInfo(scope *scope) *typeInfo {
	t, err := sortScopeFor(scope, d.sort).getType(d.idx)
	if err != nil {
		return &typeInfo{
			typeName: "unknown",
		}
	}
	return newTypeInfo(t)
}

type staticTypeResolver struct {
	typ Type
}

func newStaticTypeResolver(
	typ Type,
) *staticTypeResolver {
	return &staticTypeResolver{
		typ: typ,
	}
}

func (d *staticTypeResolver) resolveType(scope *scope) (Type, error) {
	return d.typ, nil
}

func (d *staticTypeResolver) typeInfo(scope *scope) *typeInfo {
	return newTypeInfo(d.typ)
}

type listTypeResolver struct {
	elementTypeResolver typeResolver
}

func newListTypeResolver(
	elementTypeResolver typeResolver,
) (*listTypeResolver, error) {
	return &listTypeResolver{
		elementTypeResolver: elementTypeResolver,
	}, nil
}

func (d *listTypeResolver) resolveType(scope *scope) (Type, error) {
	elementType, err := d.elementTypeResolver.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve list element type: %w", err)
	}
	if _, ok := elementType.(ValueType); !ok {
		return nil, fmt.Errorf("list element type is not a value type: %T", elementType)
	}
	if _, ok := elementType.(U8Type); ok {
		return ByteArrayType{}, nil
	}
	return &ListType{
		ElementType: elementType.(ValueType),
	}, nil
}

func (d *listTypeResolver) typeInfo(scope *scope) *typeInfo {
	et := d.elementTypeResolver.typeInfo(scope)
	return &typeInfo{
		isValue:  true,
		typeName: "list",
		depth:    1 + et.depth,
		size:     1 + et.size,
	}
}

type recordTypeResolver struct {
	labels               []string
	elementTypeResolvers []typeResolver
}

func newRecordTypeResolver(
	labels []string,
	elementTypeResolvers []typeResolver,
) (*recordTypeResolver, error) {
	if len(labels) != len(elementTypeResolvers) {
		return nil, fmt.Errorf("mismatched labels and element types lengths")
	}
	if len(labels) == 0 {
		return nil, fmt.Errorf("record type must have at least one field")
	}

	return &recordTypeResolver{
		labels:               labels,
		elementTypeResolvers: elementTypeResolvers,
	}, nil
}

func (d *recordTypeResolver) resolveType(scope *scope) (Type, error) {
	fields := make([]*RecordField, len(d.elementTypeResolvers))
	for i, elemTypeResolver := range d.elementTypeResolvers {
		elemType, err := elemTypeResolver.resolveType(scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve record element type: %w", err)
		}
		elemValueType, ok := elemType.(ValueType)
		if !ok {
			return nil, fmt.Errorf("record element type is not a value type: %T", elemType)
		}
		fields[i] = &RecordField{
			Name: d.labels[i],
			Type: elemValueType,
		}
	}
	recordType := &RecordType{
		Fields: fields,
	}

	return recordType, nil
}

func (d *recordTypeResolver) typeInfo(scope *scope) *typeInfo {
	depth := 0
	size := 1
	for _, elemTypeResolver := range d.elementTypeResolvers {
		elemTypeInfo := elemTypeResolver.typeInfo(scope)
		if elemTypeInfo.depth >= depth {
			depth = elemTypeInfo.depth
		}
		size += elemTypeInfo.size
	}
	depth++
	return &typeInfo{
		isValue:  true,
		typeName: "record",
		depth:    depth,
		size:     size,
	}
}

type tupleTypeResolver struct {
	rtr *recordTypeResolver
}

func newTupleTypeResolver(
	labels []string,
	elementTypeResolvers []typeResolver,
) (*tupleTypeResolver, error) {
	rtr, err := newRecordTypeResolver(labels, elementTypeResolvers)
	if err != nil {
		return nil, err
	}
	return &tupleTypeResolver{
		rtr: rtr,
	}, nil
}

func (d *tupleTypeResolver) resolveType(scope *scope) (Type, error) {
	recordType, err := d.rtr.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tuple record type: %w", err)
	}
	return &TupleType{
		derivedValueType[*RecordType, *TupleType]{
			recordType.(*RecordType),
		},
	}, nil
}

func (d *tupleTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := d.rtr.typeInfo(scope)
	return &typeInfo{
		isValue:  true,
		typeName: "tuple",
		depth:    ti.depth,
		size:     ti.size,
	}
}

type variantTypeResolver struct {
	labels               []string
	elementTypeResolvers []typeResolver
}

func newVariantTypeResolver(
	labels []string,
	elementTypeResolvers []typeResolver,
) (*variantTypeResolver, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("variant type must have at least one case")
	}

	return &variantTypeResolver{
		labels:               labels,
		elementTypeResolvers: elementTypeResolvers,
	}, nil
}

func (d *variantTypeResolver) resolveType(scope *scope) (Type, error) {
	cases := make([]*VariantCase, len(d.elementTypeResolvers))
	for i, caseTypeResolver := range d.elementTypeResolvers {
		var caseValueType ValueType
		if caseTypeResolver != nil {
			caseType, err := caseTypeResolver.resolveType(scope)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve variant case type: %w", err)
			}
			cvt, ok := caseType.(ValueType)
			if !ok {
				return nil, fmt.Errorf("variant case type is not a value type: %T", caseType)
			}
			caseValueType = cvt
		}
		cases[i] = &VariantCase{
			Name: d.labels[i],
			Type: caseValueType,
		}
	}
	variantType := &VariantType{
		Cases: cases,
	}

	return variantType, nil
}

func (d *variantTypeResolver) typeInfo(scope *scope) *typeInfo {
	depth := 0
	size := 1
	for _, caseTypeResolver := range d.elementTypeResolvers {
		if caseTypeResolver == nil {
			continue
		}
		caseTypeInfo := caseTypeResolver.typeInfo(scope)
		if caseTypeInfo.depth >= depth {
			depth = caseTypeInfo.depth
		}
		size += caseTypeInfo.size
	}
	depth++
	return &typeInfo{
		isValue:  true,
		typeName: "variant",
		depth:    depth,
		size:     size,
	}
}

type enumTypeResolver struct {
	vtr *variantTypeResolver
}

func newEnumTypeResolver(
	labels []string,
	elementTypeResolvers []typeResolver,
) (*enumTypeResolver, error) {

	if len(labels) == 0 {
		return nil, fmt.Errorf("enum type must have at least one element")
	}

	return &enumTypeResolver{
		&variantTypeResolver{
			labels:               labels,
			elementTypeResolvers: elementTypeResolvers,
		},
	}, nil
}

func (d *enumTypeResolver) resolveType(scope *scope) (Type, error) {
	variantType, err := d.vtr.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve enum variant type: %w", err)
	}
	return &EnumType{
		derivedValueType[*VariantType, *EnumType]{
			variantType.(*VariantType),
		},
	}, nil
}

func (d *enumTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := d.vtr.typeInfo(scope)
	return &typeInfo{
		isValue:  true,
		typeName: "enum",
		depth:    ti.depth,
		size:     ti.size,
	}
}

type optionTypeResolver struct {
	vtr *variantTypeResolver
}

func newOptionTypeResolver(
	elementTypeResolver typeResolver,
) (*optionTypeResolver, error) {
	vtr, err := newVariantTypeResolver(
		[]string{"none", "some"},
		[]typeResolver{nil, elementTypeResolver},
	)
	if err != nil {
		return nil, err
	}
	return &optionTypeResolver{
		vtr: vtr,
	}, nil
}

func (d *optionTypeResolver) resolveType(scope *scope) (Type, error) {
	variantType, err := d.vtr.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve option variant type: %w", err)
	}
	return &OptionType{
		derivedValueType[*VariantType, *OptionType]{
			variantType.(*VariantType),
		},
	}, nil
}

func (d *optionTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := d.vtr.typeInfo(scope)
	return &typeInfo{
		isValue:  true,
		typeName: "option",
		depth:    ti.depth,
		size:     ti.size,
	}
}

type resultTypeResolver struct {
	vtr *variantTypeResolver
}

func newResultTypeResolver(
	okTypeResolver typeResolver,
	errTypeResolver typeResolver,
) (*resultTypeResolver, error) {
	vtr, err := newVariantTypeResolver(
		[]string{"ok", "error"},
		[]typeResolver{okTypeResolver, errTypeResolver},
	)
	if err != nil {
		return nil, err
	}
	return &resultTypeResolver{
		vtr: vtr,
	}, nil
}

func (d *resultTypeResolver) resolveType(scope *scope) (Type, error) {
	variantType, err := d.vtr.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve result variant type: %w", err)
	}
	return &ResultType{
		derivedValueType[*VariantType, *ResultType]{
			variantType.(*VariantType),
		},
	}, nil
}

func (d *resultTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := d.vtr.typeInfo(scope)
	return &typeInfo{
		isValue:  true,
		typeName: "result",
		depth:    ti.depth,
		size:     ti.size,
	}
}

type resourceTypeBoundResolver struct {
}

func newResourceTypeBoundResolver() *resourceTypeBoundResolver {
	return &resourceTypeBoundResolver{}
}

func (d *resourceTypeBoundResolver) resolveType(scope *scope) (Type, error) {
	return &ResourceType{instance: resourceTypeBoundMarker}, nil
}

func (d *resourceTypeBoundResolver) typeInfo(scope *scope) *typeInfo {
	return &typeInfo{
		isResource: true,
		typeName:   "resource bound",
		depth:      1,
		size:       1,
	}
}

type resourceTypeResolver struct {
	destructorFnIndex *uint32
	resourceType      ResourceType
}

func newResourceTypeResolver(
	destructorFnIndex *uint32,
) *resourceTypeResolver {
	return &resourceTypeResolver{
		destructorFnIndex: destructorFnIndex,
		resourceType:      ResourceType{},
	}
}

func (d *resourceTypeResolver) resolveType(scope *scope) (Type, error) {
	instance := scope.instance

	if d.destructorFnIndex != nil {
		dtorType, err := sortScopeFor(scope, sortCoreFunction).getType(*d.destructorFnIndex)
		if err != nil {
			return nil, err
		}

		typeChecker := newTypeChecker()
		expectedType := newCoreFunctionType([]Type{coreTypeWasmConstTypeFromWazero(api.ValueTypeI32)}, nil)
		if err := typeChecker.checkTypeCompatible(expectedType, dtorType); err != nil {
			return nil, fmt.Errorf("wrong signature for a destructor: %w", err)
		}
	}

	return newResourceType(
		instance,
		func(ctx context.Context, res any) {
			if d.destructorFnIndex != nil {
				// TODO: what do if destructor fails?
				coreFn, _ := sortScopeFor(scope, sortCoreFunction).getInstance(*d.destructorFnIndex)
				fn := coreFn.module.ExportedFunction(coreFn.name)
				fn.Call(ctx, uint64(res.(uint32)))
			}
		},
	), nil
}

func (d *resourceTypeResolver) typeInfo(scope *scope) *typeInfo {
	return &typeInfo{
		isResource: true,
		typeName:   "resource",
		depth:      1,
		size:       1,
	}
}

type ownTypeResolver struct {
	resourceTypeResolver typeResolver
}

func newOwnTypeResolver(
	resourceTypeResolver typeResolver,
) *ownTypeResolver {
	return &ownTypeResolver{
		resourceTypeResolver: resourceTypeResolver,
	}
}

func (d *ownTypeResolver) resolveType(scope *scope) (Type, error) {
	resType, err := d.resourceTypeResolver.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve own resource type: %w", err)
	}
	if !d.resourceTypeResolver.typeInfo(scope).isResource {
		return nil, fmt.Errorf("own type must refer to a resource type, found %s not a resource type", d.resourceTypeResolver.typeInfo(scope).typeName)
	}
	ownType := OwnType{
		ResourceType: resType,
	}
	return ownType, nil
}

func (d *ownTypeResolver) typeInfo(scope *scope) *typeInfo {
	return &typeInfo{
		isValue:  true,
		typeName: "own",
		depth:    2,
		size:     2,
	}
}

type borrowTypeResolver struct {
	resourceTypeResolver typeResolver
}

func newBorrowTypeResolver(
	resourceTypeResolver typeResolver,
) *borrowTypeResolver {

	return &borrowTypeResolver{
		resourceTypeResolver: resourceTypeResolver,
	}
}

func (d *borrowTypeResolver) resolveType(scope *scope) (Type, error) {
	resType, err := d.resourceTypeResolver.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve borrow resource type: %w", err)
	}
	if !d.resourceTypeResolver.typeInfo(scope).isResource {
		return nil, fmt.Errorf("borrow type must refer to a resource type, found %s not a resource type", d.resourceTypeResolver.typeInfo(scope).typeName)
	}
	borrowType := BorrowType{
		ResourceType: resType,
	}
	return borrowType, nil
}

func (d *borrowTypeResolver) typeInfo(scope *scope) *typeInfo {
	return &typeInfo{
		isValue:  true,
		typeName: "borrow",
		depth:    2,
		size:     2,
	}
}
