package componentmodel

import (
	"context"
	"fmt"
	"maps"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

// Builder constructs a model from an AST
type Builder struct {
	runtime wazero.Runtime
}

// NewBuilder creates a new model builder
func NewBuilder(runtime wazero.Runtime) *Builder {
	return &Builder{
		runtime: runtime,
	}
}

// Build constructs a model Component from an AST component
func (b *Builder) Build(ctx context.Context, astComp *ast.Component) (*Component, error) {
	return b.buildComponent(ctx, astComp, nil)
}

func (b *Builder) buildComponent(ctx context.Context, astComp *ast.Component, parent *Component) (*Component, error) {
	var id string
	var parentScope *definitionScope
	if parent != nil {
		parentScope = parent.scope
		id = fmt.Sprintf("%s_%d", parent.id, defs(parent.scope, sortComponent).len())
	} else {
		id = "component_0"
	}
	comp := newComponent(id, b.runtime, parentScope)

	// Process each definition
	for _, astDef := range astComp.Definitions {
		err := b.buildDefinition(ctx, comp, astDef)
		if err != nil {
			return nil, err
		}
	}

	return comp, nil
}

func (b *Builder) buildDefinition(ctx context.Context, comp *Component, astDef ast.Definition) error {
	switch d := astDef.(type) {
	case *ast.CoreModule:
		return b.buildCoreModule(ctx, comp, d)
	case *ast.CoreInstance:
		return b.buildCoreInstance(ctx, comp, d)
	case *ast.CoreType:
		return b.buildCoreType(comp, d)
	case *ast.NestedComponent:
		return b.buildNestedComponent(ctx, comp, d)
	case *ast.Instance:
		return b.buildInstance(comp, d)
	case *ast.Alias:
		return b.buildAlias(comp, d)
	case *ast.Type:
		return b.buildType(comp, d)
	case *ast.Import:
		return b.buildImport(comp, d)
	case *ast.Export:
		return b.buildExport(comp, d)
	case *ast.Canon:
		return b.buildCanon(comp, d)
	default:
		return fmt.Errorf("unsupported definition type: %T", astDef)
	}
}

func (b *Builder) buildCoreModule(ctx context.Context, comp *Component, astMod *ast.CoreModule) error {
	// Compile the module using wazero
	/*
		TODO: delete this when either we fork wazero, or wazero supports blank import names

		fixed, err := wasm.TransformBlankImportNames(astMod.Raw)
		if err != nil {
			return fmt.Errorf("failed to transform blank import names: %w", err)
		}
	*/

	compiled, err := b.runtime.CompileModule(ctx, astMod.Raw)
	if err != nil {
		return fmt.Errorf("failed to compile core module: %w", err)
	}

	additionalExports, err := wasm.ReadExports(astMod.Raw)
	if err != nil {
		return fmt.Errorf("failed to read additional exports: %w", err)
	}

	coreModuleDefs := defs(comp.scope, sortCoreModule)
	coreModuleDefs.add(newCoreModuleStaticDefinition(
		newCoreModule(compiled, additionalExports),
	))
	return nil
}

func (b *Builder) buildCoreInstance(ctx context.Context, comp *Component, astInst *ast.CoreInstance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.CoreInstantiate:
		args := make(map[string]uint32)
		for _, astArg := range expr.Args {
			args[astArg.Name] = astArg.CoreInstanceIdx
		}
		modDef, err := defs(comp.scope, sortCoreModule).get(expr.ModuleIdx)
		if err != nil {
			return err
		}
		modType := modDef.typ()

		def := newCoreInstantiateDefinition(expr.ModuleIdx, args, newCoreInstanceType(modType.exports))
		defs(comp.scope, sortCoreInstance).add(def)
	case *ast.CoreInlineExports:
		exportTypes := make(map[string]Type)
		for _, export := range expr.Exports {
			_, typ, err := coreSortIdxDef(comp.scope, export.SortIdx)
			if err != nil {
				return err
			}
			exportTypes[export.Name] = typ
		}

		def := newCoreInlineExportsDefinition(
			expr.Exports,
			newCoreInstanceType(exportTypes),
		)
		defs(comp.scope, sortCoreInstance).add(def)
	default:
		return fmt.Errorf("invalid core instance expression type: %T", astInst.Expr)
	}
	return nil
}

func (b *Builder) buildNestedComponent(ctx context.Context, comp *Component, astNested *ast.NestedComponent) error {
	nestedComp, err := b.buildComponent(ctx, astNested.Component, comp)
	if err != nil {
		return err
	}
	defs(comp.scope, sortComponent).add(newComponentStaticDefinition(nestedComp))
	return nil
}

func (b *Builder) buildInstance(comp *Component, astInst *ast.Instance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.Instantiate:
		componentDefinition, err := defs(comp.scope, sortComponent).get(expr.ComponentIdx)
		if err != nil {
			return err
		}

		compType := componentDefinition.typ()
		def := newInstantiateDefinition(
			expr.ComponentIdx,
			expr.Args,
			newInstanceType(compType.exports),
		)
		defs(comp.scope, sortInstance).add(def)
	case *ast.InlineExports:
		exportTypes := make(map[string]Type)
		for _, export := range expr.Exports {
			_, typ, err := sortIdxDef(comp.scope, &export.SortIdx)
			if err != nil {
				return err
			}
			exportTypes[export.Name] = typ
		}
		def := newInlineExportsDefinition(expr.Exports, exportTypes)
		defs(comp.scope, sortInstance).add(def)
	default:
		return fmt.Errorf("invalid instance expression type: %T", astInst.Expr)
	}
	return nil
}

func (b *Builder) buildAlias(comp *Component, astAlias *ast.Alias) error {
	// Validate alias before building
	if err := validateAlias(comp.scope, astAlias); err != nil {
		return err
	}

	switch target := astAlias.Target.(type) {
	case *ast.CoreExportAlias:
		return b.buildCoreExportAlias(comp, astAlias.Sort, target)
	case *ast.ExportAlias:
		return b.buildExportAlias(comp, astAlias.Sort, target)
	case *ast.OuterAlias:
		return b.buildOuterAlias(comp, astAlias.Sort, target)
	default:
		return fmt.Errorf("unsupported alias target type: %T", astAlias.Target)
	}
}

func (b *Builder) buildCoreExportAlias(comp *Component, sort ast.Sort, alias *ast.CoreExportAlias) error {
	switch sort {
	case ast.SortCoreFunc:
		return addCoreExportAliasDefinitionToScope[*coreFunction, *coreFunctionType](comp.scope, sortCoreFunction, alias.InstanceIdx, alias.Name)
	case ast.SortCoreGlobal:
		return addCoreExportAliasDefinitionToScope[*coreGlobal, *coreGlobalType](comp.scope, sortCoreGlobal, alias.InstanceIdx, alias.Name)
	case ast.SortCoreMemory:
		return addCoreExportAliasDefinitionToScope[*coreMemory, *coreMemoryType](comp.scope, sortCoreMemory, alias.InstanceIdx, alias.Name)
	case ast.SortCoreType:
		return fmt.Errorf("core type export alias resolution not yet supported")
	case ast.SortCoreTable:
		return addCoreExportAliasDefinitionToScope[*coreTable, *coreTableType](comp.scope, sortCoreTable, alias.InstanceIdx, alias.Name)
	default:
		return fmt.Errorf("unsupported core export alias sort: %v", sort)
	}
}

func addCoreExportAliasDefinitionToScope[T resolvedInstance[TT], TT Type](scope *definitionScope, sort sort[T, TT], instanceIdx uint32, exportName string) error {
	instanceDef, err := defs(scope, sortCoreInstance).get(instanceIdx)
	if err != nil {
		return err
	}
	exportType, err := resolveExportType[TT](instanceDef.typ(), exportName)
	if err != nil {
		return err
	}

	defs(scope, sort).add(newCoreExportAliasDefinition[T, TT](instanceIdx, exportName, exportType))
	return nil
}

func (b *Builder) buildExportAlias(comp *Component, sort ast.Sort, alias *ast.ExportAlias) error {
	switch sort {
	case ast.SortCoreFunc:
		return addExportAliasDefinitionToScope[*coreFunction, *coreFunctionType](comp.scope, sortCoreFunction, alias.InstanceIdx, alias.Name)
	case ast.SortCoreTable:
		return addExportAliasDefinitionToScope[*coreTable, *coreTableType](comp.scope, sortCoreTable, alias.InstanceIdx, alias.Name)
	case ast.SortCoreMemory:
		return addExportAliasDefinitionToScope[*coreMemory, *coreMemoryType](comp.scope, sortCoreMemory, alias.InstanceIdx, alias.Name)
	case ast.SortCoreGlobal:
		return addExportAliasDefinitionToScope[*coreGlobal, *coreGlobalType](comp.scope, sortCoreGlobal, alias.InstanceIdx, alias.Name)
	case ast.SortCoreType:
		return addExportAliasDefinitionToScope[Type, Type](comp.scope, sortCoreType, alias.InstanceIdx, alias.Name)
	case ast.SortCoreModule:
		return addExportAliasDefinitionToScope[*coreModule, *coreModuleType](comp.scope, sortCoreModule, alias.InstanceIdx, alias.Name)
	case ast.SortCoreInstance:
		return addExportAliasDefinitionToScope[*coreInstance, *coreInstanceType](comp.scope, sortCoreInstance, alias.InstanceIdx, alias.Name)
	case ast.SortFunc:
		return addExportAliasDefinitionToScope[*Function, *FunctionType](comp.scope, sortFunction, alias.InstanceIdx, alias.Name)
	case ast.SortType:
		return addExportAliasDefinitionToScope[Type, Type](comp.scope, sortType, alias.InstanceIdx, alias.Name)
	case ast.SortComponent:
		return addExportAliasDefinitionToScope[*Component, *componentType](comp.scope, sortComponent, alias.InstanceIdx, alias.Name)
	case ast.SortInstance:
		return addExportAliasDefinitionToScope[*Instance, *instanceType](comp.scope, sortInstance, alias.InstanceIdx, alias.Name)
	default:
		return fmt.Errorf("unsupported export alias sort: %v", sort)
	}
}

func addExportAliasDefinitionToScope[T any, TT Type](scope *definitionScope, sort sort[T, TT], instanceIdx uint32, exportName string) error {
	instanceDef, err := defs(scope, sortInstance).get(instanceIdx)
	if err != nil {
		return err
	}
	exportType, err := resolveExportType[TT](instanceDef.typ(), exportName)
	if err != nil {
		return err
	}

	defs(scope, sort).add(newExportAliasDefinition(instanceIdx, exportName, sort, exportType))
	return nil
}

func (b *Builder) buildOuterAlias(comp *Component, sort ast.Sort, alias *ast.OuterAlias) error {
	// outer aliases are restricted to only refer to immutable definitions: non-resource types, modules and components
	switch sort {
	case ast.SortType:
		target, err := addOuterAliasDefinitionToScope(comp.scope, sortType, alias.Count, alias.Idx)
		if err != nil {
			return err
		}
		switch target.typ().(type) {
		case *subResourceType:
			return fmt.Errorf("cannot create outer alias to resource type")
		case *ResourceType:
			return fmt.Errorf("cannot create outer alias to resource type")
		default:
			return nil
		}
	case ast.SortCoreType:
		_, err := addOuterAliasDefinitionToScope(comp.scope, sortCoreType, alias.Count, alias.Idx)
		return err
	case ast.SortCoreModule:
		_, err := addOuterAliasDefinitionToScope(comp.scope, sortCoreModule, alias.Count, alias.Idx)
		return err
	case ast.SortComponent:
		_, err := addOuterAliasDefinitionToScope(comp.scope, sortComponent, alias.Count, alias.Idx)
		return err
	default:
		return fmt.Errorf("unsupported outer alias sort: %v", sort)
	}
}

func addOuterAliasDefinitionToScope[T resolvedInstance[TT], TT Type](scope *definitionScope, sort sort[T, TT], outerIdx uint32, idx uint32) (definition[T, TT], error) {
	outerDefs, err := nestedDefs(scope, sort, outerIdx)
	if err != nil {
		return nil, err
	}

	targetDef, err := outerDefs.get(idx)
	if err != nil {
		return nil, err
	}

	defs(scope, sort).add(newOuterAliasDefinition[T, TT](outerIdx, sort, idx, targetDef.typ()))
	return targetDef, nil
}

func (b *Builder) buildType(comp *Component, astType *ast.Type) error {
	def, err := astTypeToTypeResolver(comp.scope, astType.DefType)
	if err != nil {
		return err
	}
	defs(comp.scope, sortType).add(newTypeResolverDefinition(def))
	return nil
}

func (b *Builder) buildImport(comp *Component, astImport *ast.Import) error {
	// Validate import
	if err := validateImport(comp.scope, astImport); err != nil {
		return err
	}

	switch desc := astImport.Desc.(type) {
	case *ast.SortExternDesc:
		switch desc.Sort {
		case ast.SortCoreModule:
			typDef, err := defs(comp.scope, sortCoreType).get(desc.TypeIdx)
			if err != nil {
				return err
			}
			modType, ok := typDef.typ().(*coreModuleType)
			if !ok {
				return fmt.Errorf("import %s: core type index %d is not a module type", astImport.ImportName, desc.TypeIdx)
			}
			comp.importTypes[astImport.ImportName] = modType
			defs(comp.scope, sortCoreModule).add(newImportDefinition[*coreModule](
				astImport.ImportName, modType,
			))
			return nil
		case ast.SortFunc:
			importType, err := resolveTypeIdx(comp.scope, sortFunction, desc.TypeIdx)
			if err != nil {
				return err
			}
			comp.importTypes[astImport.ImportName] = importType
			defs(comp.scope, sortFunction).add(newImportDefinition[*Function](
				astImport.ImportName, importType,
			))
			return nil
		case ast.SortType:
			importType, err := resolveTypeIdx[Type](comp.scope, sortType, desc.TypeIdx)
			if err != nil {
				return err
			}
			comp.importTypes[astImport.ImportName] = importType
			defs(comp.scope, sortType).add(newImportDefinition[Type](
				astImport.ImportName, importType,
			))
			return nil
		case ast.SortComponent:
			importType, err := resolveTypeIdx(comp.scope, sortComponent, desc.TypeIdx)
			if err != nil {
				return err
			}
			comp.importTypes[astImport.ImportName] = importType
			defs(comp.scope, sortComponent).add(newImportDefinition[*Component](
				astImport.ImportName, importType,
			))
			return nil
		case ast.SortInstance:
			importType, err := resolveTypeIdx(comp.scope, sortInstance, desc.TypeIdx)
			if err != nil {
				return err
			}
			comp.importTypes[astImport.ImportName] = importType
			defs(comp.scope, sortInstance).add(newImportDefinition[*Instance](
				astImport.ImportName, importType,
			))
			return nil
		default:
			return fmt.Errorf("unsupported import sort: %v", desc.Sort)
		}
	case *ast.TypeExternDesc:
		var importType Type
		switch b := desc.Bound.(type) {
		case *ast.EqBound:
			typDef, err := defs(comp.scope, sortType).get(b.TypeIdx)
			if err != nil {
				return err
			}
			importType = typDef.typ()
		case *ast.SubResourceBound:
			importType = &subResourceType{}
		default:
			return fmt.Errorf("unsupported type bound in type import: %T", b)
		}
		comp.importTypes[astImport.ImportName] = importType
		defs(comp.scope, sortType).add(newImportDefinition[Type](
			astImport.ImportName, importType,
		))
		return nil
	default:
		return fmt.Errorf("unsupported import description type: %T", astImport.Desc)
	}
}

func (b *Builder) buildExport(comp *Component, astExport *ast.Export) error {
	// Validate export
	if err := validateExport(comp.scope, astExport); err != nil {
		return err
	}

	switch astExport.SortIdx.Sort {
	case ast.SortCoreModule:
		return addExportToComponent(comp, sortCoreModule, astExport.ExportName, astExport.SortIdx.Idx)
	case ast.SortFunc:
		return addExportToComponent(comp, sortFunction, astExport.ExportName, astExport.SortIdx.Idx)
	case ast.SortType:
		return addExportToComponent(comp, sortType, astExport.ExportName, astExport.SortIdx.Idx)
	case ast.SortComponent:
		return addExportToComponent(comp, sortComponent, astExport.ExportName, astExport.SortIdx.Idx)
	case ast.SortInstance:
		return addExportToComponent(comp, sortInstance, astExport.ExportName, astExport.SortIdx.Idx)
	}
	return fmt.Errorf("unsupported export sort: %v", astExport.SortIdx.Sort)
}

func addExportToComponent[T resolvedInstance[TT], TT Type](comp *Component, sort sort[T, TT], exportName string, idx uint32) error {
	def, err := defs(comp.scope, sort).get(idx)
	if err != nil {
		return err
	}
	defs(comp.scope, sort).add(def)
	if err := validateExportNameStronglyUnique(maps.Keys(comp.exports), exportName); err != nil {
		return err
	}
	comp.exports[exportName] = newDefComponentExport(sort, idx, def.typ())
	return nil
}

func (b *Builder) buildCanon(comp *Component, astCanon *ast.Canon) error {
	switch def := astCanon.Def.(type) {
	case *ast.CanonLift:
		// Validate canon lift
		if err := validateCanonLift(comp.scope, def); err != nil {
			return err
		}
		fnDef, err := canonLift(comp, def)
		if err != nil {
			return err
		}
		defs(comp.scope, sortFunction).add(fnDef)
		return nil
	case *ast.CanonLower:
		// Validate canon lower
		if err := validateCanonLower(comp.scope, def); err != nil {
			return err
		}
		fnDef, err := canonLower(comp, def)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreFunction).add(fnDef)
		return nil
	case *ast.CanonResourceNew:
		// Validate canon resource.new
		if err := validateCanonResourceNew(comp.scope, def); err != nil {
			return err
		}
		fnDef, err := canonResourceNew(comp, def)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreFunction).add(fnDef)
		return nil
	case *ast.CanonResourceDrop:
		// Validate canon resource.drop
		if err := validateCanonResourceDrop(comp.scope, def); err != nil {
			return err
		}
		fnDef, err := canonResourceDrop(comp, def)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreFunction).add(fnDef)
		return nil
	case *ast.CanonResourceRep:
		// Validate canon resource.rep
		if err := validateCanonResourceRep(comp.scope, def); err != nil {
			return err
		}
		fnDef, err := canonResourceRep(comp, def)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreFunction).add(fnDef)
		return nil
	default:
		return fmt.Errorf("unsupported canon def: %T", def)
	}
}

func (b *Builder) buildCoreType(comp *Component, core *ast.CoreType) error {
	switch defType := core.DefType.(type) {
	case *ast.CoreRecType:
		recTypeDef, err := astRecTypeToCoreTypeDefinition(comp.scope, defType)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreType).add(recTypeDef)
		return nil
	case *ast.CoreModuleType:
		modTypeDef, err := astModuleTypeToCoreModuleTypeDefinition(comp.scope, defType)
		if err != nil {
			return err
		}
		defs(comp.scope, sortCoreType).add(modTypeDef)
		return nil
	default:
		return fmt.Errorf("unsupported core type definition: %T", core.DefType)
	}
}
