package componentmodel

import (
	"context"
	"fmt"
	"maps"

	"github.com/partite-ai/wacogo/ast"
)

type componentModelTypeDefinition interface {
	resolveType(ctx context.Context, scope instanceScope) (Type, error)
}

type Type interface {
	equalsType(other Type) bool
}

type componentImportExportDeclType interface {
	matchesImportExport(other any) bool
}

type componentImportExportDeclTypeCoreType struct {
	typ coreType
}

func (c *componentImportExportDeclTypeCoreType) matchesImportExport(other any) bool {
	otherCoreType, ok := other.(coreType)
	if !ok {
		return false
	}

	return c.typ.isCompatible(otherCoreType)
}

type componentImportExportDeclTypeComponentModelType struct {
	typ Type
}

func (c *componentImportExportDeclTypeComponentModelType) matchesImportExport(other any) bool {
	otherCompType, ok := other.(Type)
	if !ok {
		return false
	}
	return c.typ.equalsType(otherCompType)
}

type componentTypeDefinition struct {
	imports map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error)
	exports map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error)
}

func (d *componentTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	imports := make(map[string]componentImportExportDeclType)
	for name, importDef := range d.imports {
		importType, err := importDef(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve component type import %s: %w", name, err)
		}
		imports[name] = importType
	}
	exports := make(map[string]componentImportExportDeclType)
	for name, exportDef := range d.exports {
		exportType, err := exportDef(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve component type export %s: %w", name, err)
		}
		exports[name] = exportType
	}
	return &componentType{
		imports: imports,
		exports: exports,
	}, nil
}

type componentType struct {
	scope   definitionScope
	imports map[string]componentImportExportDeclType
	exports map[string]componentImportExportDeclType
}

func (ct *componentType) equalsType(other Type) bool {
	otherCt, ok := other.(*componentType)
	if !ok {
		return false
	}
	return maps.Equal(ct.imports, otherCt.imports) && maps.Equal(ct.exports, otherCt.exports)
}

type instanceTypeDefinition struct {
	exports map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error)
}

func (d *instanceTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	exports := make(map[string]componentImportExportDeclType)
	for name, exportDef := range d.exports {
		exportType, err := exportDef(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve instance type export %s: %w", name, err)
		}
		exports[name] = exportType
	}
	return &instanceType{
		exports: exports,
	}, nil
}

type instanceType struct {
	exports map[string]componentImportExportDeclType
}

func (it *instanceType) equalsType(other Type) bool {
	otherIt, ok := other.(*instanceType)
	if !ok {
		return false
	}
	if len(it.exports) != len(otherIt.exports) {
		return false
	}

	for name, exportType := range it.exports {
		switch exportType := exportType.(type) {
		case Type:
			otherExportType, ok := otherIt.exports[name].(Type)
			if !ok || !exportType.equalsType(otherExportType) {
				return false
			}
		case coreType:
			otherExportType, ok := otherIt.exports[name].(coreType)
			if !ok || !exportType.isCompatible(otherExportType) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

type typeSubResourceDefinition struct{}

func (d *typeSubResourceDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	return &typeSubResource{}, nil
}

type typeSubResource struct{}

func (t *typeSubResource) equalsType(other Type) bool {
	_, ok := other.(*typeSubResource)
	return ok
}

type typeAliasDefinition struct {
	instanceIdx uint32
	exportName  string
}

func (d *typeAliasDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	inst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}

	// Look up the export
	typeExport, ok := inst.exports[d.exportName]
	if !ok {
		return nil, fmt.Errorf("type export not found: %s", d.exportName)
	}

	return ensureType(typeExport)
}

type typeStaticDefinition struct {
	typ Type
}

func (d *typeStaticDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	return d.typ, nil
}

type typeImportDefinition struct {
	name string
}

func (d *typeImportDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	val, err := scope.resolveArgument(d.name)
	if err != nil {
		return nil, err
	}
	return ensureType(val)
}

type listTypeDefinition struct {
	elementTypeDef componentModelTypeDefinition
}

func (d *listTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	elementType, err := d.elementTypeDef.resolveType(ctx, scope)
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

type recordTypeDefinition struct {
	labels          []string
	elementTypeDefs []componentModelTypeDefinition
}

func (d *recordTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	fields := make([]*RecordField, len(d.elementTypeDefs))
	for i, elemTypeDef := range d.elementTypeDefs {
		elemType, err := elemTypeDef.resolveType(ctx, scope)
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
	return &RecordType{
		Fields: fields,
	}, nil
}

type variantTypeDefinition struct {
	labels          []string
	elementTypeDefs []componentModelTypeDefinition
}

func (d *variantTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	cases := make([]*VariantCase, len(d.elementTypeDefs))
	for i, caseTypeDef := range d.elementTypeDefs {
		var caseValueType ValueType
		if caseTypeDef != nil {
			caseType, err := caseTypeDef.resolveType(ctx, scope)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve variant case type: %w", err)
			}
			caseValueType, ok := caseType.(ValueType)
			if !ok {
				return nil, fmt.Errorf("variant case type is not a value type: %T", caseType)
			}
			caseType = caseValueType
		}
		cases[i] = &VariantCase{
			Name: d.labels[i],
			Type: caseValueType,
		}
	}
	return &VariantType{
		Cases: cases,
	}, nil
}

type resourceTypeDefinition struct {
	destructorFnIndex *uint32
}

func (d *resourceTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	instance := scope.currentInstance()

	var dtor *Function
	if d.destructorFnIndex != nil {
		fnDef, err := scope.resolveFunctionDefinition(0, *d.destructorFnIndex)
		if err != nil {
			return nil, err
		}

		fn, err := fnDef.resolveFunction(ctx, scope)
		if err != nil {
			return nil, err
		}
		dtor = fn
	}

	return &ResourceType{
		instance: instance,
		destructor: func(ctx context.Context, res any) {
			if dtor != nil {
				dtor.invoke(ctx, []Value{U32(res.(ExternalResourceRep))})
			}
		},
	}, nil
}

type ownTypeDefinition struct {
	resourceTypeDef componentModelTypeDefinition
}

func (d *ownTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	resType, err := d.resourceTypeDef.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve own resource type: %w", err)
	}
	resValueType, ok := resType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("own resource type is not a resource type: %T", resType)
	}
	return OwnType[ExternalResourceRep]{
		ResourceType: resValueType,
	}, nil
}

type borrowTypeDefinition struct {
	resourceTypeDef componentModelTypeDefinition
}

func (d *borrowTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	resType, err := d.resourceTypeDef.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve borrow resource type: %w", err)
	}
	resValueType, ok := resType.(*ResourceType)
	if !ok {
		return nil, fmt.Errorf("borrow resource type is not a resource type: %T", resType)
	}
	return BorrowType[ExternalResourceRep]{
		ResourceType: resValueType,
	}, nil
}

type funcTypeDefinition struct {
	paramTypeDefs []componentModelTypeDefinition
	resultTypeDef componentModelTypeDefinition
}

func (d *funcTypeDefinition) resolveType(ctx context.Context, scope instanceScope) (Type, error) {
	paramTypes := make([]ValueType, len(d.paramTypeDefs))
	for i, paramTypeDef := range d.paramTypeDefs {
		paramType, err := paramTypeDef.resolveType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve function param type: %w", err)
		}
		paramValueType, ok := paramType.(ValueType)
		if !ok {
			return nil, fmt.Errorf("function param type is not a value type: %T", paramType)
		}
		paramTypes[i] = paramValueType
	}
	resultType, err := d.resultTypeDef.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function result type: %w", err)
	}
	resultTypes, ok := resultType.(ValueType)
	if !ok {
		return nil, fmt.Errorf("function result type is not a value type: %T", resultType)
	}
	return &FunctionType{
		ParamTypes: paramTypes,
		ResultType: resultTypes,
	}, nil
}

func astTypeToTypeDefinition(scope definitionScope, defType ast.DefType) (componentModelTypeDefinition, error) {
	switch def := defType.(type) {
	case *ast.TypeIdx:
		return scope.resolveComponentModelTypeDefinition(0, def.Idx)
	case *ast.BoolType:
		return &typeStaticDefinition{
			typ: BoolType{},
		}, nil
	case *ast.U8Type:
		return &typeStaticDefinition{
			typ: U8Type{},
		}, nil
	case *ast.S8Type:
		return &typeStaticDefinition{
			typ: S8Type{},
		}, nil
	case *ast.U16Type:
		return &typeStaticDefinition{
			typ: U16Type{},
		}, nil
	case *ast.S16Type:
		return &typeStaticDefinition{
			typ: S16Type{},
		}, nil
	case *ast.U32Type:
		return &typeStaticDefinition{
			typ: U32Type{},
		}, nil
	case *ast.S32Type:
		return &typeStaticDefinition{
			typ: S32Type{},
		}, nil
	case *ast.U64Type:
		return &typeStaticDefinition{
			typ: U64Type{},
		}, nil
	case *ast.S64Type:
		return &typeStaticDefinition{
			typ: S64Type{},
		}, nil
	case *ast.F32Type:
		return &typeStaticDefinition{
			typ: F32Type{},
		}, nil
	case *ast.F64Type:
		return &typeStaticDefinition{
			typ: F64Type{},
		}, nil
	case *ast.CharType:
		return &typeStaticDefinition{
			typ: CharType{},
		}, nil
	case *ast.StringType:
		return &typeStaticDefinition{
			typ: StringType{},
		}, nil
	case *ast.RecordType:
		labels := make([]string, len(def.Fields))
		elementTypeDefs := make([]componentModelTypeDefinition, len(def.Fields))
		for i, field := range def.Fields {
			labels[i] = field.Label
			fieldTypeDef, err := astTypeToTypeDefinition(scope, field.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve record field type: %w", err)
			}
			elementTypeDefs[i] = fieldTypeDef
		}
		return &recordTypeDefinition{
			labels:          labels,
			elementTypeDefs: elementTypeDefs,
		}, nil
	case *ast.VariantType:
		labels := make([]string, len(def.Cases))
		elementTypeDefs := make([]componentModelTypeDefinition, len(def.Cases))
		for i, caseDef := range def.Cases {
			labels[i] = caseDef.Label
			if caseDef.Type != nil {
				caseTypeDef, err := astTypeToTypeDefinition(scope, caseDef.Type)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve variant case type: %w", err)
				}
				elementTypeDefs[i] = caseTypeDef
			}
		}
		return &variantTypeDefinition{
			labels:          labels,
			elementTypeDefs: elementTypeDefs,
		}, nil
	case *ast.ListType:
		elemTypeDef, err := astTypeToTypeDefinition(scope, def.Element)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve list element type: %w", err)
		}

		return &listTypeDefinition{
			elementTypeDef: elemTypeDef,
		}, nil
	case *ast.TupleType:
		labels := make([]string, len(def.Types))
		elementTypeDefs := make([]componentModelTypeDefinition, len(def.Types))
		for i := range def.Types {
			labels[i] = ""
			elemTypeDef, err := astTypeToTypeDefinition(scope, def.Types[i])
			if err != nil {
				return nil, fmt.Errorf("failed to resolve tuple element type: %w", err)
			}
			elementTypeDefs[i] = elemTypeDef
		}
		return &recordTypeDefinition{
			labels:          labels,
			elementTypeDefs: elementTypeDefs,
		}, nil
	case *ast.FlagsType:
		return &typeStaticDefinition{
			typ: &FlagsType{
				FlagNames: def.Labels,
			},
		}, nil
	case *ast.EnumType:
		labels := make([]string, len(def.Labels))
		elementTypeDefs := make([]componentModelTypeDefinition, len(def.Labels))
		copy(labels, def.Labels)
		return &variantTypeDefinition{
			labels:          labels,
			elementTypeDefs: elementTypeDefs,
		}, nil

	case *ast.OptionType:
		elemTypeDef, err := astTypeToTypeDefinition(scope, def.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve option element type: %w", err)
		}
		return &variantTypeDefinition{
			labels:          []string{"none", "some"},
			elementTypeDefs: []componentModelTypeDefinition{nil, elemTypeDef},
		}, nil
	case *ast.ResultType:
		var okTypeDef componentModelTypeDefinition
		if def.Ok != nil {
			var err error
			okTypeDef, err = astTypeToTypeDefinition(scope, def.Ok)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result ok type: %w", err)
			}
		}
		var errTypeDef componentModelTypeDefinition
		if def.Error != nil {
			var err error
			errTypeDef, err = astTypeToTypeDefinition(scope, def.Error)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve result err type: %w", err)
			}
		}
		return &variantTypeDefinition{
			labels:          []string{"ok", "error"},
			elementTypeDefs: []componentModelTypeDefinition{okTypeDef, errTypeDef},
		}, nil
	case *ast.OwnType:
		innerTypeDef, err := scope.resolveComponentModelTypeDefinition(0, def.TypeIdx)
		if err != nil {
			return nil, err
		}
		return &ownTypeDefinition{
			resourceTypeDef: innerTypeDef,
		}, nil
	case *ast.BorrowType:
		innerTypeDef, err := scope.resolveComponentModelTypeDefinition(0, def.TypeIdx)
		if err != nil {
			return nil, err
		}
		return &borrowTypeDefinition{
			resourceTypeDef: innerTypeDef,
		}, nil
	case *ast.ResourceType:
		return &resourceTypeDefinition{
			destructorFnIndex: def.Dtor,
		}, nil
	case *ast.FuncType:
		paramTypeDefs := make([]componentModelTypeDefinition, len(def.Params))
		for i, paramDef := range def.Params {
			paramTypeDef, err := astTypeToTypeDefinition(scope, paramDef.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve function param type: %w", err)
			}
			paramTypeDefs[i] = paramTypeDef
		}
		resultTypeDef, err := astTypeToTypeDefinition(scope, def.Results)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve function result type: %w", err)
		}
		return &funcTypeDefinition{
			paramTypeDefs: paramTypeDefs,
			resultTypeDef: resultTypeDef,
		}, nil
	case *ast.ComponentType:
		typeScope := componentDefinitionScope{
			parent: scope,
		}
		imports := make(map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error))
		exports := make(map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error))

		for _, decl := range def.Declarations {
			switch decl := decl.(type) {
			case *ast.TypeDecl:
				typeDef, err := astTypeToTypeDefinition(&typeScope, decl.Type.DefType)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component type declaration: %w", err)
				}
				typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
			case *ast.CoreTypeDecl:
				switch defType := decl.Type.DefType.(type) {
				case *ast.CoreRecType:
					recType, err := astRecTypeToCoreTypeDefinition(&typeScope, defType)
					if err != nil {
						return nil, err
					}
					typeScope.coreTypes = append(typeScope.coreTypes, recType)
				case *ast.CoreModuleType:
					return nil, fmt.Errorf("core module type not supported in component type declarations")
				default:
					return nil, fmt.Errorf("unsupported core type definition: %T", defType)
				}
			case *ast.AliasDecl:
				switch target := decl.Alias.Target.(type) {
				case *ast.ExportAlias:
					switch decl.Alias.Sort {
					case ast.SortInstance:
						typeScope.instances = append(typeScope.instances, &instanceAliasDefinition{
							instanceIdx: target.InstanceIdx,
							exportName:  target.Name,
						})
					case ast.SortType:
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, &typeAliasDefinition{
							instanceIdx: target.InstanceIdx,
							exportName:  target.Name,
						})
					default:
						return nil, fmt.Errorf("unsupported alias sort in component type declarations: %v", decl.Alias.Sort)
					}
				case *ast.CoreExportAlias:
					return nil, fmt.Errorf("core export alias not supported in component type declarations")
				case *ast.OuterAlias:
					switch decl.Alias.Sort {
					case ast.SortInstance:
						instanceDef, err := typeScope.resolveInstanceDefinition(target.Count, target.Idx)
						if err != nil {
							return nil, err
						}
						typeScope.instances = append(typeScope.instances, instanceDef)
					case ast.SortType:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(target.Count, target.Idx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
					default:
						return nil, fmt.Errorf("unsupported outer alias sort in component type declarations: %v", decl.Alias.Sort)
					}
				default:
					return nil, fmt.Errorf("unsupported component alias target: %T", target)
				}
			case *ast.ImportDecl:
				switch desc := decl.Desc.(type) {
				case *ast.SortExternDesc:
					switch desc.Sort {
					case ast.SortCoreModule:
						typeDef, err := scope.resolveCoreTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.coreTypes = append(typeScope.coreTypes, typeDef)
						imports[decl.ImportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							coreType, err := typeDef.resolveCoreType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeCoreType{typ: coreType}, nil
						}
					case ast.SortComponent, ast.SortFunc, ast.SortInstance:
						typeDef, err := scope.resolveComponentModelTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						imports[decl.ImportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported import sort in component type declarations: %v", desc.Sort)
					}
				case *ast.TypeExternDesc:
					switch bound := desc.Bound.(type) {
					case *ast.EqBound:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(0, bound.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						imports[decl.ImportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					case *ast.SubResourceBound:
						typeDef := &typeSubResourceDefinition{}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						imports[decl.ImportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported type extern desc bound in component type declarations: %T", bound)
					}
				}
			case *ast.ExportDecl:
				switch desc := decl.Desc.(type) {
				case *ast.SortExternDesc:
					switch desc.Sort {
					case ast.SortCoreModule:
						typeDef, err := typeScope.resolveCoreTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.coreTypes = append(typeScope.coreTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							coreType, err := typeDef.resolveCoreType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeCoreType{typ: coreType}, nil
						}
					case ast.SortComponent, ast.SortFunc, ast.SortInstance:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported export sort in component type declarations: %v", desc.Sort)
					}
				case *ast.TypeExternDesc:
					switch bound := desc.Bound.(type) {
					case *ast.EqBound:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(0, bound.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					case *ast.SubResourceBound:
						typeDef := &typeSubResourceDefinition{}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported type extern desc bound in component type declarations: %T", bound)
					}
				}
			}
		}
		return &componentTypeDefinition{
			imports: imports,
			exports: exports,
		}, nil
	case *ast.InstanceType:
		typeScope := componentDefinitionScope{
			parent: scope,
		}
		exports := make(map[string]func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error))

		for _, decl := range def.Declarations {
			switch decl := decl.(type) {
			case *ast.TypeDecl:
				typeDef, err := astTypeToTypeDefinition(&typeScope, decl.Type.DefType)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve component type declaration: %w", err)
				}
				typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
			case *ast.CoreTypeDecl:
				switch defType := decl.Type.DefType.(type) {
				case *ast.CoreRecType:
					recType, err := astRecTypeToCoreTypeDefinition(&typeScope, defType)
					if err != nil {
						return nil, err
					}
					typeScope.coreTypes = append(typeScope.coreTypes, recType)
				case *ast.CoreModuleType:
					return nil, fmt.Errorf("core module type not supported in component type declarations")
				default:
					return nil, fmt.Errorf("unsupported core type definition: %T", defType)
				}
			case *ast.AliasDecl:
				switch target := decl.Alias.Target.(type) {
				case *ast.ExportAlias:
					switch decl.Alias.Sort {
					case ast.SortInstance:
						typeScope.instances = append(typeScope.instances, &instanceAliasDefinition{
							instanceIdx: target.InstanceIdx,
							exportName:  target.Name,
						})
					case ast.SortType:
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, &typeAliasDefinition{
							instanceIdx: target.InstanceIdx,
							exportName:  target.Name,
						})
					default:
						return nil, fmt.Errorf("unsupported alias sort in component type declarations: %v", decl.Alias.Sort)
					}
				case *ast.CoreExportAlias:
					return nil, fmt.Errorf("core export alias not supported in component type declarations")
				case *ast.OuterAlias:
					switch decl.Alias.Sort {
					case ast.SortInstance:
						instanceDef, err := typeScope.resolveInstanceDefinition(target.Count, target.Idx)
						if err != nil {
							return nil, err
						}
						typeScope.instances = append(typeScope.instances, instanceDef)
					case ast.SortType:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(target.Count, target.Idx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
					default:
						return nil, fmt.Errorf("unsupported outer alias sort in component type declarations: %v", decl.Alias.Sort)
					}
				default:
					return nil, fmt.Errorf("unsupported component alias target: %T", target)
				}

			case *ast.ExportDecl:
				switch desc := decl.Desc.(type) {
				case *ast.SortExternDesc:
					switch desc.Sort {
					case ast.SortCoreModule:
						typeDef, err := typeScope.resolveCoreTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.coreTypes = append(typeScope.coreTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							coreType, err := typeDef.resolveCoreType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeCoreType{typ: coreType}, nil
						}
					case ast.SortComponent, ast.SortFunc, ast.SortInstance:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(0, desc.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported export sort in component type declarations: %v", desc.Sort)
					}
				case *ast.TypeExternDesc:
					switch bound := desc.Bound.(type) {
					case *ast.EqBound:
						typeDef, err := typeScope.resolveComponentModelTypeDefinition(0, bound.TypeIdx)
						if err != nil {
							return nil, err
						}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					case *ast.SubResourceBound:
						typeDef := &typeSubResourceDefinition{}
						typeScope.componentModelTypes = append(typeScope.componentModelTypes, typeDef)
						exports[decl.ExportName] = func(ctx context.Context, scope instanceScope) (componentImportExportDeclType, error) {
							compType, err := typeDef.resolveType(ctx, scope)
							if err != nil {
								return nil, err
							}
							return &componentImportExportDeclTypeComponentModelType{typ: compType}, nil
						}
					default:
						return nil, fmt.Errorf("unsupported type extern desc bound in component type declarations: %T", bound)
					}
				}
			}
		}
		return &instanceTypeDefinition{
			exports: exports,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type definition: %T", defType)

	}
}

func ensureType(val any) (Type, error) {
	switch v := val.(type) {
	case Type:
		return v, nil
	default:
		return nil, fmt.Errorf("value is not a type: %T", val)
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
