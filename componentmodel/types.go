package componentmodel

import (
	"context"
	"fmt"
	"maps"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero/api"
)

type Type interface {
	typ() Type
	assignableFrom(other Type) bool
}

type exportType interface {
	Type
	exportType(name string) (Type, bool)
}

type typeResolver interface {
	typ() Type
	resolveType(ctx context.Context, scope *instanceScope) (Type, error)
}

type staticTypeResolver struct {
	staticType Type
}

func newStaticTypeResolver(typ Type) *staticTypeResolver {
	return &staticTypeResolver{
		staticType: typ,
	}
}

func (t *staticTypeResolver) typ() Type {
	return t.staticType
}

func (t *staticTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	return t.staticType, nil
}

type dynamicTypeResolver struct {
	typeIdx    uint32
	staticType Type
}

func newDynamicTypeResolver(staticType Type, typeIdx uint32) *dynamicTypeResolver {
	return &dynamicTypeResolver{
		typeIdx:    typeIdx,
		staticType: staticType,
	}
}

func (t *dynamicTypeResolver) typ() Type {
	return t.staticType
}

func (t *dynamicTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	return resolve(ctx, scope, sortType, t.typeIdx)
}

type typeResolverDefinition struct {
	typeResolver typeResolver
}

func newTypeResolverDefinition(typeResolver typeResolver) *typeResolverDefinition {
	return &typeResolverDefinition{
		typeResolver: typeResolver,
	}
}

func (d *typeResolverDefinition) typ() Type {
	return d.typeResolver.typ()
}

func (d *typeResolverDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	return d.typeResolver.resolveType(ctx, scope)
}

type typeOnlyDefinition[T resolvedInstance[TT], TT Type] struct {
	staticType TT
}

func newTypeOnlyDefinition[T resolvedInstance[TT], TT Type](staticType TT) *typeOnlyDefinition[T, TT] {
	return &typeOnlyDefinition[T, TT]{
		staticType: staticType,
	}
}

func (d *typeOnlyDefinition[T, TT]) typ() TT {
	return d.staticType
}

func (d *typeOnlyDefinition[T, TT]) resolve(ctx context.Context, scope *instanceScope) (T, error) {
	var zero T
	return zero, fmt.Errorf("unexpected use of type only definition")
}

type staticTypeDefinition struct {
	staticType Type
}

func newStaticTypeDefinition(staticType Type) *staticTypeDefinition {
	return &staticTypeDefinition{
		staticType: staticType,
	}
}

func (d *staticTypeDefinition) typ() Type {
	return d.staticType
}

func (d *staticTypeDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	return d.staticType, nil
}

type subResourceType struct{}

func (srt *subResourceType) typ() Type {
	return srt
}

func (srt *subResourceType) assignableFrom(other Type) bool {
	switch ot := other.(type) {
	case *subResourceType:
		return srt == ot
	case *ResourceType:
		return true
	}
	return false
}

type typeStaticDefinition struct {
	staticType Type
}

func (d *typeStaticDefinition) typ() Type {
	return d.staticType
}

func (d *typeStaticDefinition) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	return d.staticType, nil
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

func (d *listTypeResolver) typ() Type {
	return &ListType{
		ElementType: d.elementTypeResolver.typ().(ValueType),
	}
}

func (d *listTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	elementType, err := d.elementTypeResolver.resolveType(ctx, scope)
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

type recordTypeResolver struct {
	labels               []string
	elementTypeResolvers []typeResolver
	staticType           *RecordType
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
	fields := make([]*RecordField, len(elementTypeResolvers))
	for i, elemTypeResolver := range elementTypeResolvers {
		elemTyp := elemTypeResolver.typ()
		fields[i] = &RecordField{
			Name: labels[i],
			Type: elemTyp.(ValueType),
		}
	}
	return &recordTypeResolver{
		labels:               labels,
		elementTypeResolvers: elementTypeResolvers,
		staticType: &RecordType{
			Fields: fields,
		},
	}, nil
}

func (d *recordTypeResolver) typ() Type {
	return d.staticType
}

func (d *recordTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	fields := make([]*RecordField, len(d.elementTypeResolvers))
	for i, elemTypeResolver := range d.elementTypeResolvers {
		elemType, err := elemTypeResolver.resolveType(ctx, scope)
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
	// Validate record field names are unique
	if err := validateRecordFieldNames(recordType); err != nil {
		return nil, err
	}
	return recordType, nil
}

type variantTypeResolver struct {
	labels               []string
	elementTypeResolvers []typeResolver
	staticTyp            *VariantType
}

func newVariantTypeResolver(
	labels []string,
	elementTypeResolvers []typeResolver,
) (*variantTypeResolver, error) {

	cases := make([]*VariantCase, len(elementTypeResolvers))
	for i, caseTypeResolver := range elementTypeResolvers {
		var caseValueType ValueType
		if caseTypeResolver != nil {
			caseTyp := caseTypeResolver.typ()
			caseValueType = caseTyp.(ValueType)
		}
		cases[i] = &VariantCase{
			Name: labels[i],
			Type: caseValueType,
		}
	}

	vt := &VariantType{
		Cases: cases,
	}

	// Validate variant has at least one case and case names are unique
	if err := validateVariantCasesNonEmpty(vt); err != nil {
		return nil, err
	}
	if err := validateVariantCaseNames(vt); err != nil {
		return nil, err
	}
	return &variantTypeResolver{
		labels:               labels,
		elementTypeResolvers: elementTypeResolvers,
		staticTyp:            vt,
	}, nil
}

func (d *variantTypeResolver) typ() Type {
	return d.staticTyp
}

func (d *variantTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	cases := make([]*VariantCase, len(d.elementTypeResolvers))
	for i, caseTypeResolver := range d.elementTypeResolvers {
		var caseValueType ValueType
		if caseTypeResolver != nil {
			caseType, err := caseTypeResolver.resolveType(ctx, scope)
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

func (d *resourceTypeResolver) typ() Type {
	return &d.resourceType
}

func (d *resourceTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	instance := scope.currentInstance

	var dtor api.Function
	if d.destructorFnIndex != nil {
		coreFn, err := resolve(ctx, scope, sortCoreFunction, *d.destructorFnIndex)
		if err != nil {
			return nil, err
		}
		fn := coreFn.module.ExportedFunction(coreFn.name)
		dtor = fn
	}

	return &ResourceType{
		instance: instance,
		destructor: func(ctx context.Context, res any) {
			if dtor != nil {
				dtor.Call(ctx, uint64(res.(uint32)))
			}
		},
	}, nil
}

type ownTypeResolver struct {
	resourceTypeResolver typeResolver
}

func newOwnTypeResolver(
	resourceTypeResolver typeResolver,
) (*ownTypeResolver, error) {
	containedType := resourceTypeResolver.typ()
	_, isResourceType := containedType.(*ResourceType)
	_, isSubResourceType := containedType.(*subResourceType)
	if !isResourceType && !isSubResourceType {
		return nil, fmt.Errorf("own type resource type is not a resource type: %T", containedType)
	}
	return &ownTypeResolver{
		resourceTypeResolver: resourceTypeResolver,
	}, nil
}

func (d *ownTypeResolver) typ() Type {
	return OwnType{
		ResourceType: d.resourceTypeResolver.typ(),
	}
}

func (d *ownTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	resType, err := d.resourceTypeResolver.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve own resource type: %w", err)
	}
	resValueType, ok := resType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("own resource type is not a resource type: %T", resType)
	}
	ownType := OwnType{
		ResourceType: resValueType,
	}
	return ownType, nil
}

type borrowTypeResolver struct {
	resourceTypeResolver typeResolver
}

func newBorrowTypeResolver(
	resourceTypeResolver typeResolver,
) (*borrowTypeResolver, error) {
	containedType := resourceTypeResolver.typ()
	_, isResourceType := containedType.(*ResourceType)
	_, isSubResourceType := containedType.(*subResourceType)
	if !isResourceType && !isSubResourceType {
		return nil, fmt.Errorf("borrow type resource type is not a resource type: %T", containedType)
	}
	return &borrowTypeResolver{
		resourceTypeResolver: resourceTypeResolver,
	}, nil
}

func (d *borrowTypeResolver) typ() Type {
	return BorrowType{
		ResourceType: d.resourceTypeResolver.typ(),
	}
}

func (d *borrowTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	resType, err := d.resourceTypeResolver.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve borrow resource type: %w", err)
	}
	resValueType, ok := resType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("borrow resource type is not a resource type: %T", resType)
	}
	borrowType := BorrowType{
		ResourceType: resValueType,
	}
	return borrowType, nil
}

func astTypeToTypeResolver(scope *definitionScope, defType ast.DefType) (typeResolver, error) {
	switch def := defType.(type) {
	case *ast.TypeIdx:
		targetDef, err := defs(scope, sortType).get(def.Idx)
		if err != nil {
			return nil, err
		}
		return newDynamicTypeResolver(targetDef.typ(), def.Idx), nil
	case *ast.BoolType:
		return newStaticTypeResolver(BoolType{}), nil
	case *ast.U8Type:
		return newStaticTypeResolver(U8Type{}), nil
	case *ast.S8Type:
		return newStaticTypeResolver(S8Type{}), nil
	case *ast.U16Type:
		return newStaticTypeResolver(U16Type{}), nil
	case *ast.S16Type:
		return newStaticTypeResolver(S16Type{}), nil
	case *ast.U32Type:
		return newStaticTypeResolver(U32Type{}), nil
	case *ast.S32Type:
		return newStaticTypeResolver(S32Type{}), nil
	case *ast.U64Type:
		return newStaticTypeResolver(U64Type{}), nil
	case *ast.S64Type:
		return newStaticTypeResolver(S64Type{}), nil
	case *ast.F32Type:
		return newStaticTypeResolver(F32Type{}), nil
	case *ast.F64Type:
		return newStaticTypeResolver(F64Type{}), nil
	case *ast.CharType:
		return newStaticTypeResolver(CharType{}), nil
	case *ast.StringType:
		return newStaticTypeResolver(StringType{}), nil
	case *ast.RecordType:
		labels := make([]string, len(def.Fields))
		elementTypeResolvers := make([]typeResolver, len(def.Fields))
		for i, field := range def.Fields {
			labels[i] = field.Label
			fieldTypeDef, err := astTypeToTypeResolver(scope, field.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve record field type: %w", err)
			}
			elementTypeResolvers[i] = fieldTypeDef
		}
		return newRecordTypeResolver(labels, elementTypeResolvers)
	case *ast.VariantType:
		labels := make([]string, len(def.Cases))
		elementTypeResolvers := make([]typeResolver, len(def.Cases))
		for i, caseDef := range def.Cases {
			labels[i] = caseDef.Label
			if caseDef.Type != nil {
				caseTypeDef, err := astTypeToTypeResolver(scope, caseDef.Type)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve variant case type: %w", err)
				}
				elementTypeResolvers[i] = caseTypeDef
			}
		}
		return newVariantTypeResolver(labels, elementTypeResolvers)
	case *ast.ListType:
		elemTypeDef, err := astTypeToTypeResolver(scope, def.Element)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve list element type: %w", err)
		}

		return newListTypeResolver(elemTypeDef)
	case *ast.TupleType:
		labels := make([]string, len(def.Types))
		elementTypeResolvers := make([]typeResolver, len(def.Types))
		for i := range def.Types {
			labels[i] = ""
			elemTypeResolver, err := astTypeToTypeResolver(scope, def.Types[i])
			if err != nil {
				return nil, fmt.Errorf("failed to resolve tuple element type: %w", err)
			}
			elementTypeResolvers[i] = elemTypeResolver
		}
		if len(def.Types) == 0 {
			return nil, fmt.Errorf("tuple type must have at least one type")
		}
		return newRecordTypeResolver(labels, elementTypeResolvers)
	case *ast.FlagsType:
		flagsType := &FlagsType{
			FlagNames: def.Labels,
		}
		if len(def.Labels) == 0 {
			return nil, fmt.Errorf("flags must have at least one entry")
		}
		if len(def.Labels) > 32 {
			return nil, fmt.Errorf("cannot have more than 32 flags")
		}
		// Validate flag names are unique
		if err := validateFlagNames(flagsType); err != nil {
			return nil, err
		}
		return newStaticTypeResolver(flagsType), nil
	case *ast.EnumType:
		labels := make([]string, len(def.Labels))
		elementTypeResolvers := make([]typeResolver, len(def.Labels))
		copy(labels, def.Labels)
		if len(def.Labels) == 0 {
			return nil, fmt.Errorf("enum type must have at least one variant")
		}
		return newVariantTypeResolver(labels, elementTypeResolvers)

	case *ast.OptionType:
		elemTypeResolver, err := astTypeToTypeResolver(scope, def.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve option element type: %w", err)
		}
		return newVariantTypeResolver(
			[]string{"none", "some"},
			[]typeResolver{nil, elemTypeResolver},
		)
	case *ast.ResultType:
		var okTypeResolver typeResolver
		if def.Ok != nil {
			var err error
			okTypeResolver, err = astTypeToTypeResolver(scope, def.Ok)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result ok type: %w", err)
			}
		}
		var errTypeResolver typeResolver
		if def.Error != nil {
			var err error
			errTypeResolver, err = astTypeToTypeResolver(scope, def.Error)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result err type: %w", err)
			}
		}
		return newVariantTypeResolver(
			[]string{"ok", "error"},
			[]typeResolver{okTypeResolver, errTypeResolver},
		)
	case *ast.OwnType:
		innerTypeDef, err := defs(scope, sortType).get(def.TypeIdx)
		if err != nil {
			return nil, err
		}
		return newOwnTypeResolver(
			newDynamicTypeResolver(innerTypeDef.typ(), def.TypeIdx),
		)
	case *ast.BorrowType:
		innerTypeDef, err := defs(scope, sortType).get(def.TypeIdx)
		if err != nil {
			return nil, err
		}
		return newBorrowTypeResolver(
			newDynamicTypeResolver(innerTypeDef.typ(), def.TypeIdx),
		)
	case *ast.ResourceType:
		if def.Dtor != nil {
			_, err := defs(scope, sortCoreFunction).get(*def.Dtor)
			if err != nil {
				return nil, err
			}
		}
		return newResourceTypeResolver(def.Dtor), nil
	case *ast.FuncType:
		paramTypeResolvers := make([]*parameterTypeResolver, len(def.Params))
		paramNames := make([]string, 0, len(def.Params))
		for i, paramDef := range def.Params {
			if paramDef.Label == "" {
				return nil, fmt.Errorf("function parameter name cannot be empty: %d", i)
			}
			if err := validateParameterNameStronglyUnique(func(yield func(string) bool) {
				for _, pn := range paramNames {
					if !yield(pn) {
						return
					}
				}
			}, paramDef.Label); err != nil {
				return nil, err
			}
			paramNames = append(paramNames, paramDef.Label)
			paramTypeResolver, err := astTypeToTypeResolver(scope, paramDef.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve function param type: %w", err)
			}
			paramTypeResolvers[i] = newParameterTypeResolver(paramDef.Label, paramTypeResolver)
		}

		if def.Results == nil {
			return newFunctionTypeResolver(paramTypeResolvers, nil)
		}

		resultTypeResolver, err := astTypeToTypeResolver(scope, def.Results)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve function result type: %w", err)
		}
		return newFunctionTypeResolver(paramTypeResolvers, resultTypeResolver)
	case *ast.ComponentType:
		typeScope := newDefinitionScope(scope)
		imports := make(map[string]Type)
		exports := make(map[string]Type)

		for _, decl := range def.Declarations {
			switch decl := decl.(type) {
			// importdecl
			case *ast.ImportDecl:
				typ, err := astExternDescToType(typeScope, decl.Desc, true)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component import declaration: %w", err)
				}
				if err := validateImportNameStronglyUnique(maps.Keys(imports), decl.ImportName); err != nil {
					return nil, err
				}
				imports[decl.ImportName] = typ
			case ast.InstanceDecl:
				err := addInstanceDeclToScope(typeScope, decl, exports)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component instance declaration: %w", err)
				}
			}
		}
		return newStaticTypeResolver(newComponentType(imports, exports)), nil
	case *ast.InstanceType:
		typeScope := newDefinitionScope(scope)
		exports := make(map[string]Type)

		for _, decl := range def.Declarations {
			err := addInstanceDeclToScope(typeScope, decl, exports)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve instance declaration: %w", err)
			}
		}
		return newStaticTypeResolver(newInstanceType(exports)), nil
	default:
		return nil, fmt.Errorf("unsupported type definition: %T", defType)

	}
}

func astExternDescToType(scope *definitionScope, desc ast.ExternDesc, addDef bool) (Type, error) {
	switch desc := desc.(type) {
	case *ast.SortExternDesc:
		switch desc.Sort {
		case ast.SortCoreModule:
			typeDef, err := defs(scope, sortCoreType).get(desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			modType, isModType := typeDef.typ().(*coreModuleType)
			if !isModType {
				return nil, fmt.Errorf("core type index %d is not a module type", desc.TypeIdx)
			}
			if addDef {
				defs(scope, sortCoreModule).add(newTypeOnlyDefinition[*coreModule](modType))
			}
			return typeDef.typ(), nil
		case ast.SortComponent:
			typeDef, err := defs(scope, sortType).get(desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			compType, err := resolveTypeIdx(scope, sortComponent, desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			if addDef {
				defs(scope, sortComponent).add(newTypeOnlyDefinition[*Component](compType))
			}
			return typeDef.typ(), nil
		case ast.SortFunc:
			typeDef, err := defs(scope, sortType).get(desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			funcTyp, err := resolveTypeIdx(scope, sortFunction, desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			if addDef {
				defs(scope, sortFunction).add(newTypeOnlyDefinition[*Function](funcTyp))
			}
			return typeDef.typ(), nil
		case ast.SortInstance:
			typeDef, err := defs(scope, sortType).get(desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			instanceTyp, err := resolveTypeIdx(scope, sortInstance, desc.TypeIdx)
			if err != nil {
				return nil, err
			}
			if addDef {
				defs(scope, sortInstance).add(newTypeOnlyDefinition[*Instance](instanceTyp))
			}
			return typeDef.typ(), nil
		default:
			return nil, fmt.Errorf("unsupported import sort in type declaration: %v", desc.Sort)
		}
	case *ast.TypeExternDesc:
		switch bound := desc.Bound.(type) {
		case *ast.EqBound:
			typeDef, err := defs(scope, sortType).get(bound.TypeIdx)
			if err != nil {
				return nil, err
			}
			if addDef {
				defs(scope, sortType).add(typeDef)
			}
			return typeDef.typ(), nil
		case *ast.SubResourceBound:
			var typ subResourceType
			if addDef {
				defs(scope, sortType).add(newStaticTypeDefinition(&typ))
			}
			return &typ, nil
		default:
			return nil, fmt.Errorf("unsupported type extern desc bound in type declaration: %T", bound)
		}
	default:
		return nil, fmt.Errorf("unsupported extern desc in type declaration: %T", desc)
	}
}

func addInstanceDeclToScope(scope *definitionScope, decl ast.InstanceDecl, exports map[string]Type) error {
	switch decl := decl.(type) {
	case *ast.CoreTypeDecl:
		switch defType := decl.Type.DefType.(type) {
		case *ast.CoreRecType:
			recType, err := astRecTypeToCoreTypeDefinition(scope, defType)
			if err != nil {
				return err
			}
			defs(scope, sortCoreType).add(recType)
			return nil
		case *ast.CoreModuleType:
			modType, err := astModuleTypeToCoreModuleTypeDefinition(scope, defType)
			if err != nil {
				return err
			}
			defs(scope, sortCoreType).add(modType)
			return nil
		default:
			return fmt.Errorf("unsupported core type definition: %T", defType)
		}

	case *ast.TypeDecl:
		typeResolver, err := astTypeToTypeResolver(scope, decl.Type.DefType)
		if err != nil {
			return fmt.Errorf("failed to resolve component type declaration: %w", err)
		}
		defs(scope, sortType).add(newStaticTypeDefinition(typeResolver.typ()))
		return nil

	case *ast.AliasDecl:
		//Validation of instancedecl (currently) only allows the type and instance sorts in alias declarators.
		switch target := decl.Alias.Target.(type) {
		case *ast.ExportAlias:
			switch decl.Alias.Sort {
			case ast.SortType:
				instanceDef, err := defs(scope, sortInstance).get(target.InstanceIdx)
				if err != nil {
					return err
				}
				exportTyp, ok := instanceDef.typ().exportType(target.Name)
				if !ok {
					return fmt.Errorf("instance %d has no export named `%s`", target.InstanceIdx, target.Name)
				}
				defs(scope, sortType).add(newExportAliasDefinition(target.InstanceIdx, target.Name, sortType, exportTyp))
				return nil
			case ast.SortInstance:
				instanceDef, err := defs(scope, sortInstance).get(target.InstanceIdx)
				if err != nil {
					return err
				}
				exportTyp, ok := instanceDef.typ().exportType(target.Name)
				if !ok {
					return fmt.Errorf("instance %d has no export named `%s`", target.InstanceIdx, target.Name)
				}
				it, ok := exportTyp.(*instanceType)
				if !ok {
					return fmt.Errorf("exported type %s is not an instance type", target.Name)
				}
				defs(scope, sortInstance).add(newExportAliasDefinition(target.InstanceIdx, target.Name, sortInstance, it))
				return nil
			default:
				return fmt.Errorf("unsupported alias sort in type declaration: %v", decl.Alias.Sort)
			}
		case *ast.CoreExportAlias:
			return fmt.Errorf("core export alias not supported in type declarations")
		case *ast.OuterAlias:
			switch decl.Alias.Sort {
			case ast.SortInstance:
				nestedDefs, err := nestedDefs(scope, sortInstance, target.Count)
				if err != nil {
					return err
				}
				instanceDef, err := nestedDefs.get(target.Idx)
				if err != nil {
					return err
				}
				defs(scope, sortInstance).add(instanceDef)
				return nil
			case ast.SortCoreInstance:
				nestedDefs, err := nestedDefs(scope, sortCoreInstance, target.Count)
				if err != nil {
					return err
				}
				instanceDef, err := nestedDefs.get(target.Idx)
				if err != nil {
					return err
				}
				defs(scope, sortCoreInstance).add(instanceDef)
				return nil
			case ast.SortType:
				nestedDefs, err := nestedDefs(scope, sortType, target.Count)
				if err != nil {
					return err
				}
				typeDef, err := nestedDefs.get(target.Idx)
				if err != nil {
					return err
				}
				defs(scope, sortType).add(typeDef)
				return nil
			case ast.SortCoreType:
				nestedDefs, err := nestedDefs(scope, sortCoreType, target.Count)
				if err != nil {
					return err
				}
				typeDef, err := nestedDefs.get(target.Idx)
				if err != nil {
					return err
				}
				defs(scope, sortCoreType).add(typeDef)
				return nil
			default:
				return fmt.Errorf("unsupported outer alias sort in component type declarations: %v", decl.Alias.Sort)
			}
		default:
			return fmt.Errorf("unsupported component alias target: %T", target)
		}
	case *ast.ExportDecl:
		typ, err := astExternDescToType(scope, decl.Desc, true)
		if err != nil {
			return fmt.Errorf("failed to resolve instance export declaration: %w", err)
		}

		if _, exists := exports[decl.ExportName]; exists {
			return fmt.Errorf("export name `%s` already defined", decl.ExportName)
		}
		if err := validateExportNameStronglyUnique(maps.Keys(exports), decl.ExportName); err != nil {
			return err
		}
		exports[decl.ExportName] = typ
		return nil
	default:
		return fmt.Errorf("unsupported instance declaration: %T", decl)
	}
}

func ResultType(ok ValueType, err ValueType) *VariantType {
	cases := make([]*VariantCase, 2)
	cases[0] = &VariantCase{
		Name: "ok",
		Type: ok,
	}
	cases[1] = &VariantCase{
		Name: "error",
		Type: err,
	}
	return &VariantType{
		Cases: cases,
	}
}

func OptionType(elem ValueType) *VariantType {
	cases := make([]*VariantCase, 2)
	cases[0] = &VariantCase{
		Name: "none",
		Type: nil,
	}
	cases[1] = &VariantCase{
		Name: "some",
		Type: elem,
	}
	return &VariantType{
		Cases: cases,
	}
}

func EnumType(labels ...string) *VariantType {
	cases := make([]*VariantCase, len(labels))
	for i, label := range labels {
		cases[i] = &VariantCase{
			Name: label,
			Type: nil,
		}
	}
	return &VariantType{
		Cases: cases,
	}
}

func TupleType(elementTypes ...ValueType) *RecordType {
	fields := make([]*RecordField, len(elementTypes))
	for i, elemType := range elementTypes {
		fields[i] = &RecordField{
			Type: elemType,
		}
	}
	return &RecordType{
		Fields: fields,
	}
}

func resolveExportType[T resolvedInstance[TT], TT Type](exporter exportType, name string, sort sort[T, TT], exporterName string) (TT, error) {
	var zero TT
	typ, ok := exporter.exportType(name)
	if !ok {
		return zero, fmt.Errorf("%s has no export named `%s`", exporterName, name)
	}
	castTyp, ok := typ.(TT)
	if !ok {
		return zero, fmt.Errorf("export `%s` for %s is not a %s", name, exporterName, sort.typeName())
	}
	return castTyp, nil
}

func resolveTypeIdx[T resolvedInstance[TT], TT Type](scope *definitionScope, sort sort[T, TT], idx uint32) (TT, error) {
	var zero TT
	typeDef, err := defs(scope, sortType).get(idx)
	if err != nil {
		return zero, fmt.Errorf("failed to resolve type index %d: %w", idx, err)
	}
	typ := typeDef.typ()
	castTyp, ok := typ.(TT)
	if !ok {
		var article string
		switch sort.typeName()[0] {
		case 'a', 'e', 'i', 'o', 'u':
			article = "an"
		default:
			article = "a"
		}
		return zero, fmt.Errorf("type index %d is not %s %s type", idx, article, sort.typeName())
	}
	return castTyp, nil
}
