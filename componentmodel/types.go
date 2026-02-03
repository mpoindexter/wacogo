package componentmodel

import (
	"context"
	"fmt"
	"strconv"

	"github.com/partite-ai/wacogo/ast"
)

const maxTypeSize = 1000000

type Type interface {
	assertType
	typeName() string
	typeSize() int
	typeDepth() int
	checkType(other Type, typeChecker typeChecker) error
}

// Used to aid in code comprehension: as we add methods to the Type interface, we
// can use this to find things that _should_ be part of the Type interface, but
// aren't yet.
type assertType interface {
	isType()
}

type exportType interface {
	Type
	exportType(name string) (Type, bool)
}

type exporter interface {
	comparable
	getExport(string) (any, error)
}

type typeOnlyDefinition[V any, T Type] struct {
	typeResolver typeResolver
}

func newTypeOnlyDefinition[V any, T Type](typeResolver typeResolver) *typeOnlyDefinition[V, T] {
	return &typeOnlyDefinition[V, T]{
		typeResolver: typeResolver,
	}
}

func (d *typeOnlyDefinition[V, T]) isDefinition() {}

func (d *typeOnlyDefinition[V, T]) createType(scope *scope) (T, error) {
	t, err := d.typeResolver.resolveType(scope)
	if err != nil {
		return zero[T](), err
	}

	castType, ok := t.(T)
	if !ok {
		return zero[T](), fmt.Errorf("type mismatch: expected %s type, found %s", castType.typeName(), t.typeName())
	}
	return castType, nil
}

func (d *typeOnlyDefinition[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	return zero[V](), fmt.Errorf("unexpected use of type only definition")
}

func astDefTypeToTypeResolver(defs *definitions, defType ast.DefType, allowResources bool) (typeResolver, error) {
	switch def := defType.(type) {
	case *ast.TypeIdx:
		return newIndexTypeResolver(sortType, def.Idx, func(t Type) error {
			_, isValueType := t.(ValueType)
			if !isValueType {
				return fmt.Errorf("type index %d is not a defined type", def.Idx)
			}
			return nil
		}), nil
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
			fieldTypeDef, err := astDefTypeToTypeResolver(defs, field.Type, allowResources)
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
				caseTypeDef, err := astDefTypeToTypeResolver(defs, caseDef.Type, allowResources)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve variant case type: %w", err)
				}
				elementTypeResolvers[i] = caseTypeDef
			}
		}
		return newVariantTypeResolver(labels, elementTypeResolvers)
	case *ast.ListType:
		elemTypeDef, err := astDefTypeToTypeResolver(defs, def.Element, allowResources)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve list element type: %w", err)
		}

		return newListTypeResolver(elemTypeDef)
	case *ast.TupleType:
		labels := make([]string, len(def.Types))
		elementTypeResolvers := make([]typeResolver, len(def.Types))
		for i := range def.Types {
			labels[i] = strconv.Itoa(i)
			elemTypeResolver, err := astDefTypeToTypeResolver(defs, def.Types[i], allowResources)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve tuple element type: %w", err)
			}
			elementTypeResolvers[i] = elemTypeResolver
		}
		if len(def.Types) == 0 {
			return nil, fmt.Errorf("tuple type must have at least one type")
		}
		return newTupleTypeResolver(labels, elementTypeResolvers)
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
		return newStaticTypeResolver(flagsType), nil
	case *ast.EnumType:
		labels := make([]string, len(def.Labels))
		elementTypeResolvers := make([]typeResolver, len(def.Labels))
		copy(labels, def.Labels)
		if len(def.Labels) == 0 {
			return nil, fmt.Errorf("enum type must have at least one variant")
		}
		return newEnumTypeResolver(labels, elementTypeResolvers)
	case *ast.OptionType:
		elemTypeResolver, err := astDefTypeToTypeResolver(defs, def.Type, allowResources)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve option element type: %w", err)
		}
		return newOptionTypeResolver(elemTypeResolver)
	case *ast.ResultType:
		var okTypeResolver typeResolver
		if def.Ok != nil {
			var err error
			okTypeResolver, err = astDefTypeToTypeResolver(defs, def.Ok, allowResources)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result ok type: %w", err)
			}
		}
		var errTypeResolver typeResolver
		if def.Error != nil {
			var err error
			errTypeResolver, err = astDefTypeToTypeResolver(defs, def.Error, allowResources)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result err type: %w", err)
			}
		}
		return newResultTypeResolver(okTypeResolver, errTypeResolver)
	case *ast.OwnType:
		return newOwnTypeResolver(
			newIndexTypeResolverOf[*ResourceType](sortType, def.TypeIdx, ""),
		), nil
	case *ast.BorrowType:
		return newBorrowTypeResolver(
			newIndexTypeResolverOf[*ResourceType](sortType, def.TypeIdx, ""),
		), nil
	case *ast.ResourceType:
		if !allowResources {
			return nil, fmt.Errorf("resources can only be defined within a concrete component")
		}
		return newResourceTypeResolver(def.Dtor), nil
	case *ast.FuncType:
		paramTypeResolvers := make([]*parameterTypeResolver, len(def.Params))
		paramNames := make([]string, 0, len(def.Params))
		for i, paramDef := range def.Params {
			if paramDef.Label == "" {
				return nil, fmt.Errorf("function parameter name cannot be empty: %d", i)
			}
			paramNames = append(paramNames, paramDef.Label)
			paramTypeResolver, err := astDefTypeToTypeResolver(defs, paramDef.Type, allowResources)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve function param type: %w", err)
			}
			paramTypeResolvers[i] = newParameterTypeResolver(paramDef.Label, paramTypeResolver)
		}

		if def.Results == nil {
			return newFunctionTypeResolver(paramTypeResolvers, nil), nil
		}

		resultTypeResolver, err := astDefTypeToTypeResolver(defs, def.Results, allowResources)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve function result type: %w", err)
		}
		return newFunctionTypeResolver(paramTypeResolvers, resultTypeResolver), nil
	case *ast.ComponentType:
		componentDefs := newDefinitions()
		importTypes := make(map[string]typeResolver)
		exports := make(map[string]componentExport)

		for _, decl := range def.Declarations {
			switch decl := decl.(type) {
			// importdecl
			case *ast.ImportDecl:
				gs, typResolver, err := astExternDescToTypeResolver(decl.Desc)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component import declaration: %w", err)
				}

				importTypeResolver, err := gs.addImport(componentDefs, decl.ImportName, typResolver)
				if err != nil {
					return nil, fmt.Errorf("failed to add import to component: %w", err)
				}
				importTypes[decl.ImportName] = importTypeResolver
			case ast.InstanceDecl:
				err := addInstanceDeclToTypeScope(componentDefs, decl, exports)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component instance declaration: %w", err)
				}
			}
		}

		return newComponentTypeResolver(componentDefs, importTypes, exports), nil
	case *ast.InstanceType:
		instanceDefs := newDefinitions()
		exports := make(map[string]componentExport)

		for _, decl := range def.Declarations {
			err := addInstanceDeclToTypeScope(instanceDefs, decl, exports)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve instance declaration: %w", err)
			}
		}
		return newInstanceTypeResolver(instanceDefs, exports), nil
	default:
		return nil, fmt.Errorf("unsupported type definition: %T", defType)
	}
}

func astExternDescToTypeResolver(desc ast.ExternDesc) (genericSort, typeResolver, error) {
	switch desc := desc.(type) {
	case *ast.SortExternDesc:
		switch desc.Sort {
		case ast.SortCoreModule:
			return sortCoreModule, newIndexTypeResolverOf[*coreModuleType](sortCoreType, desc.TypeIdx, ""), nil
		case ast.SortComponent:
			return sortComponent, newIndexTypeResolverOf[*componentType](sortType, desc.TypeIdx, ""), nil
		case ast.SortFunc:
			return sortFunction, newIndexTypeResolverOf[*FunctionType](sortType, desc.TypeIdx, fmt.Sprintf("type index %d is not a function type", desc.TypeIdx)), nil
		case ast.SortInstance:
			return sortInstance, newIndexTypeResolverOf[*instanceType](sortType, desc.TypeIdx, ""), nil
		default:
			return nil, nil, fmt.Errorf("unsupported import sort in type declaration: %v", desc.Sort)
		}
	case *ast.TypeExternDesc:
		switch bound := desc.Bound.(type) {
		case *ast.EqBound:
			return sortType, newIndexTypeResolverOf[Type](sortType, bound.TypeIdx, ""), nil
		case *ast.SubResourceBound:
			return sortType, newResourceTypeBoundResolver(), nil
		default:
			return nil, nil, fmt.Errorf("unsupported type extern desc bound in type declaration: %T", bound)
		}
	default:
		return nil, nil, fmt.Errorf("unsupported extern desc in type declaration: %T", desc)
	}
}

func addInstanceDeclToTypeScope(defs *definitions, decl ast.InstanceDecl, exports map[string]componentExport) error {
	switch decl := decl.(type) {
	case *ast.CoreTypeDecl:
		switch defType := decl.Type.DefType.(type) {
		case *ast.CoreRecType:
			recType, err := astRecTypeToTypeResolver(defs, defType)
			if err != nil {
				return err
			}
			sortDefsFor(defs, sortCoreType).add(newTypeResolverDefinition(recType))
			return nil
		case *ast.CoreModuleType:
			modType, err := astModuleTypeToCoreModuleTypeResolver(defs, defType)
			if err != nil {
				return err
			}
			sortDefsFor(defs, sortCoreType).add(modType)
			return nil
		default:
			return fmt.Errorf("unsupported core type definition: %T", defType)
		}

	case *ast.TypeDecl:
		typeResolver, err := astDefTypeToTypeResolver(defs, decl.Type.DefType, false)
		if err != nil {
			return fmt.Errorf("failed to resolve component type declaration: %w", err)
		}
		sortDefsFor(defs, sortType).add(newTypeResolverDefinition(typeResolver))
		return nil

	case *ast.AliasDecl:
		//Validation of instancedecl (currently) only allows the type and instance sorts in alias declarators.
		switch target := decl.Alias.Target.(type) {
		case *ast.ExportAlias:
			switch decl.Alias.Sort {
			case ast.SortType:
				sortDefsFor(defs, sortType).add(newInstanceExportAliasDefinition(target.InstanceIdx, target.Name, sortType))
				return nil
			case ast.SortInstance:
				sortDefsFor(defs, sortInstance).add(newInstanceExportAliasDefinition(target.InstanceIdx, target.Name, sortInstance))
				return nil
			default:
				return fmt.Errorf("unsupported alias sort in type declaration: %v", decl.Alias.Sort)
			}
		case *ast.CoreExportAlias:
			return fmt.Errorf("core export alias not supported in type declarations")
		case *ast.OuterAlias:
			switch decl.Alias.Sort {
			case ast.SortInstance:
				sortDefsFor(defs, sortInstance).add(newOuterAliasDefinition(target.Count, sortInstance, target.Idx, true))
				return nil
			case ast.SortCoreInstance:
				sortDefsFor(defs, sortCoreInstance).add(newOuterAliasDefinition(target.Count, sortCoreInstance, target.Idx, true))
				return nil
			case ast.SortType:
				sortDefsFor(defs, sortType).add(newOuterAliasDefinition(target.Count, sortType, target.Idx, true))
				return nil
			case ast.SortCoreType:
				sortDefsFor(defs, sortCoreType).add(newOuterAliasDefinition(target.Count, sortCoreType, target.Idx, true))
				return nil
			default:
				return fmt.Errorf("unsupported outer alias sort in component type declarations: %v", decl.Alias.Sort)
			}
		default:
			return fmt.Errorf("unsupported component alias target: %T", target)
		}
	case *ast.ExportDecl:
		gs, typResolver, err := astExternDescToTypeResolver(decl.Desc)
		if err != nil {
			return fmt.Errorf("failed to resolve instance export declaration: %w", err)
		}
		export, err := gs.addTypeOnlyExportDefinition(defs, typResolver)
		if err != nil {
			return fmt.Errorf("failed to add export to instance type: %w", err)
		}
		exports[decl.ExportName] = export
		return nil
	default:
		return fmt.Errorf("unsupported instance declaration: %T", decl)
	}
}

func NewResultType(ok ValueType, err ValueType) *ResultType {
	cases := make([]*VariantCase, 2)
	cases[0] = &VariantCase{
		Name: "ok",
		Type: ok,
	}
	cases[1] = &VariantCase{
		Name: "error",
		Type: err,
	}
	return &ResultType{
		derivedValueType[*VariantType, *ResultType]{
			&VariantType{
				Cases: cases,
			},
		},
	}
}

func NewOptionType(elem ValueType) *OptionType {
	cases := []*VariantCase{
		{
			Name: "none",
			Type: nil,
		},
		{
			Name: "some",
			Type: elem,
		},
	}
	return &OptionType{
		derivedValueType[*VariantType, *OptionType]{
			&VariantType{
				Cases: cases,
			},
		},
	}
}

func NewEnumType(labels ...string) *EnumType {
	cases := make([]*VariantCase, len(labels))
	for i, label := range labels {
		cases[i] = &VariantCase{
			Name: label,
			Type: nil,
		}
	}
	return &EnumType{
		derivedValueType[*VariantType, *EnumType]{
			&VariantType{
				Cases: cases,
			},
		},
	}
}

func NewTupleType(elementTypes ...ValueType) *TupleType {
	fields := make([]*RecordField, len(elementTypes))
	for i, elemType := range elementTypes {
		fields[i] = &RecordField{
			Type: elemType,
		}
	}
	return &TupleType{
		derivedValueType[*RecordType, *TupleType]{
			&RecordType{
				Fields: fields,
			},
		},
	}
}
