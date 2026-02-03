package componentmodel

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

// Builder constructs a model from an AST
type Builder struct {
	runtime            wazero.Runtime
	componentIDCounter uint32
	canonIDCounter     uint32
}

// NewBuilder creates a new model builder
func NewBuilder(runtime wazero.Runtime) *Builder {
	return &Builder{
		runtime: runtime,
	}
}

// Build constructs a model Component from an AST component
func (b *Builder) Build(ctx context.Context, astComp *ast.Component) (*Component, error) {
	comp, err := b.buildComponent(ctx, astComp, nil)
	if err != nil {
		return nil, err
	}

	return comp, nil
}

func (b *Builder) buildComponent(ctx context.Context, astComp *ast.Component, parent *buildContext) (*Component, error) {
	id := fmt.Sprintf("component_%d", b.componentIDCounter)
	b.componentIDCounter++

	definitions := newDefinitions()
	imports := make(map[string]typeResolver)
	exports := make(map[string]componentExport)

	var parentScope *scope
	if parent != nil {
		parentScope = parent.scope
	}
	componentScope := newScope(parentScope, nil, b.runtime, make(map[string]*instanceArgument))

	bc := &buildContext{
		defs:    definitions,
		scope:   componentScope,
		imports: imports,
		exports: exports,
	}
	// Process each definition
	for _, astDef := range astComp.Definitions {
		err := b.buildDefinition(ctx, bc, astDef)
		if err != nil {
			return nil, err
		}
	}

	comp, err := newComponent(id, b.runtime, definitions, componentScope, imports, exports)
	if err != nil {
		return nil, err
	}

	importTypes := make(map[string]Type)
	for name, tr := range imports {
		typ, err := tr.resolveType(componentScope)
		if err != nil {
			return nil, err
		}
		importTypes[name] = typ
	}

	exportTypes := make(map[string]Type)
	for name, export := range exports {
		typ, err := export.typ(componentScope)
		if err != nil {
			return nil, err
		}
		exportTypes[name] = typ
	}

	return comp, nil
}

func (b *Builder) buildDefinition(ctx context.Context, bc *buildContext, astDef ast.Definition) error {
	switch d := astDef.(type) {
	case *ast.CoreModule:
		return b.buildCoreModule(ctx, bc, d)
	case *ast.CoreInstance:
		return b.buildCoreInstance(ctx, bc, d)
	case *ast.CoreType:
		return b.buildCoreType(bc, d)
	case *ast.NestedComponent:
		return b.buildNestedComponent(ctx, bc, d)
	case *ast.Instance:
		return b.buildInstance(bc, d)
	case *ast.Alias:
		return b.buildAlias(bc, d)
	case *ast.Type:
		return b.buildType(bc, d)
	case *ast.Import:
		return b.buildImport(bc, d)
	case *ast.Export:
		return b.buildExport(bc, d)
	case *ast.Canon:
		return b.buildCanon(bc, d)
	default:
		return fmt.Errorf("unsupported definition type: %T", astDef)
	}
}

func (b *Builder) buildCoreModule(ctx context.Context, bc *buildContext, astMod *ast.CoreModule) error {
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

	additionalExterns, err := wasm.ReadExterns(astMod.Raw)
	if err != nil {
		return fmt.Errorf("failed to read additional exports: %w", err)
	}

	coreModule := newCoreModule(compiled, additionalExterns)
	return addDefinitionToBuildContext(bc, sortCoreModule, newStaticDefinition(
		coreModule,
		coreModule.typ(),
	))
}

func (b *Builder) buildCoreInstance(ctx context.Context, bc *buildContext, astInst *ast.CoreInstance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.CoreInstantiate:

		def := newCoreInstantiateDefinition(expr)
		return addDefinitionToBuildContext(bc, sortCoreInstance, def)
	case *ast.CoreInlineExports:
		def := newCoreInlineExportsDefinition(expr)
		return addDefinitionToBuildContext(bc, sortCoreInstance, def)
	default:
		return fmt.Errorf("invalid core instance expression type: %T", astInst.Expr)
	}
}

func (b *Builder) buildNestedComponent(ctx context.Context, bc *buildContext, astNested *ast.NestedComponent) error {
	nestedComp, err := b.buildComponent(ctx, astNested.Component, bc)
	if err != nil {
		return err
	}
	return addDefinitionToBuildContext(bc, sortComponent, newComponentDefinition(nestedComp))
}

func (b *Builder) buildInstance(bc *buildContext, astInst *ast.Instance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.Instantiate:

		def := newInstantiateDefinition(expr)
		return addDefinitionToBuildContext(bc, sortInstance, def)
	case *ast.InlineExports:
		exportNames := make([]string, 0, len(expr.Exports))
		for _, export := range expr.Exports {
			exportNames = append(exportNames, export.Name)
		}
		def := newInlineExportsDefinition(expr.Exports)
		return addDefinitionToBuildContext(bc, sortInstance, def)
	default:
		return fmt.Errorf("invalid instance expression type: %T", astInst.Expr)
	}
}

func (b *Builder) buildAlias(bc *buildContext, astAlias *ast.Alias) error {
	switch target := astAlias.Target.(type) {
	case *ast.CoreExportAlias:
		return b.buildCoreExportAlias(bc, astAlias.Sort, target)
	case *ast.ExportAlias:
		return b.buildExportAlias(bc, astAlias.Sort, target)
	case *ast.OuterAlias:
		return b.buildOuterAlias(bc, astAlias.Sort, target)
	default:
		return fmt.Errorf("unsupported alias target type: %T", astAlias.Target)
	}
}

func (b *Builder) buildCoreExportAlias(bc *buildContext, sort ast.Sort, alias *ast.CoreExportAlias) error {
	switch sort {
	case ast.SortCoreFunc:
		return addCoreExportAliasDefinitionToScope(bc, sortCoreFunction, alias.InstanceIdx, alias.Name)
	case ast.SortCoreGlobal:
		return addCoreExportAliasDefinitionToScope(bc, sortCoreGlobal, alias.InstanceIdx, alias.Name)
	case ast.SortCoreMemory:
		return addCoreExportAliasDefinitionToScope(bc, sortCoreMemory, alias.InstanceIdx, alias.Name)
	case ast.SortCoreType:
		return fmt.Errorf("core type export alias resolution not yet supported")
	case ast.SortCoreTable:
		return addCoreExportAliasDefinitionToScope(bc, sortCoreTable, alias.InstanceIdx, alias.Name)
	default:
		return fmt.Errorf("unsupported core export alias sort: %v", sort)
	}
}

func addCoreExportAliasDefinitionToScope[T comparable, TT Type](bc *buildContext, sort sort[T, TT], instanceIdx uint32, exportName string) error {
	return addDefinitionToBuildContext(bc, sort, newCoreExportAliasDefinition(instanceIdx, exportName, sort))
}

func (b *Builder) buildExportAlias(bc *buildContext, sort ast.Sort, alias *ast.ExportAlias) error {
	switch sort {
	case ast.SortCoreFunc:
		return addExportAliasDefinitionToScope(bc, sortCoreFunction, alias.InstanceIdx, alias.Name)
	case ast.SortCoreTable:
		return addExportAliasDefinitionToScope(bc, sortCoreTable, alias.InstanceIdx, alias.Name)
	case ast.SortCoreMemory:
		return addExportAliasDefinitionToScope(bc, sortCoreMemory, alias.InstanceIdx, alias.Name)
	case ast.SortCoreGlobal:
		return addExportAliasDefinitionToScope(bc, sortCoreGlobal, alias.InstanceIdx, alias.Name)
	case ast.SortCoreType:
		return addExportAliasDefinitionToScope(bc, sortCoreType, alias.InstanceIdx, alias.Name)
	case ast.SortCoreModule:
		return addExportAliasDefinitionToScope(bc, sortCoreModule, alias.InstanceIdx, alias.Name)
	case ast.SortCoreInstance:
		return addExportAliasDefinitionToScope(bc, sortCoreInstance, alias.InstanceIdx, alias.Name)
	case ast.SortFunc:
		return addExportAliasDefinitionToScope(bc, sortFunction, alias.InstanceIdx, alias.Name)
	case ast.SortType:
		return addExportAliasDefinitionToScope(bc, sortType, alias.InstanceIdx, alias.Name)
	case ast.SortComponent:
		return addExportAliasDefinitionToScope(bc, sortComponent, alias.InstanceIdx, alias.Name)
	case ast.SortInstance:
		return addExportAliasDefinitionToScope(bc, sortInstance, alias.InstanceIdx, alias.Name)
	default:
		return fmt.Errorf("unsupported export alias sort: %v", sort)
	}
}

func addExportAliasDefinitionToScope[T comparable, TT Type](bc *buildContext, sort sort[T, TT], instanceIdx uint32, exportName string) error {
	return addDefinitionToBuildContext(bc, sort, newInstanceExportAliasDefinition(instanceIdx, exportName, sort))
}

func (b *Builder) buildOuterAlias(bc *buildContext, sort ast.Sort, alias *ast.OuterAlias) error {
	// outer aliases are restricted to only refer to immutable definitions: non-resource types, modules and components
	switch sort {
	case ast.SortType:
		return addDefinitionToBuildContext(bc, sortType, newOuterAliasDefinition(alias.Count, sortType, alias.Idx, false))
	case ast.SortCoreType:
		return addDefinitionToBuildContext(bc, sortCoreType, newOuterAliasDefinition(alias.Count, sortCoreType, alias.Idx, false))
	case ast.SortCoreModule:
		return addDefinitionToBuildContext(bc, sortCoreModule, newOuterAliasDefinition(alias.Count, sortCoreModule, alias.Idx, false))
	case ast.SortComponent:
		return addDefinitionToBuildContext(bc, sortComponent, newOuterAliasDefinition(alias.Count, sortComponent, alias.Idx, false))
	default:
		return fmt.Errorf("unsupported outer alias sort: %v", sort)
	}
}

func (b *Builder) buildType(bc *buildContext, astType *ast.Type) error {
	resolver, err := astDefTypeToTypeResolver(bc.defs, astType.DefType, true)
	if err != nil {
		return err
	}
	return addDefinitionToBuildContext(bc, sortType, newTypeResolverDefinition(resolver))
}

func (b *Builder) buildImport(bc *buildContext, astImport *ast.Import) error {
	bc.scope.arguments[astImport.ImportName] = &instanceArgument{
		typ: importPlaceholderType{},
	}

	switch desc := astImport.Desc.(type) {
	case *ast.SortExternDesc:
		switch desc.Sort {
		case ast.SortCoreModule:
			tr := newIndexTypeResolverOf[*coreModuleType](sortCoreType, desc.TypeIdx, "")
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortCoreModule, newImportDefinition(
				sortCoreModule, astImport.ImportName, tr,
			))
		case ast.SortFunc:
			tr := newIndexTypeResolverOf[*FunctionType](sortType, desc.TypeIdx, fmt.Sprintf("type index %d is not a function type", desc.TypeIdx))
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortFunction, newImportDefinition(
				sortFunction, astImport.ImportName, tr,
			))
		case ast.SortType:
			tr := newIndexTypeResolverOf[Type](sortType, desc.TypeIdx, "")
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortType, newImportDefinition(
				sortType, astImport.ImportName, tr,
			))
		case ast.SortComponent:
			tr := newIndexTypeResolverOf[*componentType](sortType, desc.TypeIdx, "")
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortComponent, newImportDefinition(
				sortComponent, astImport.ImportName, tr,
			))
		case ast.SortInstance:
			tr := newIndexTypeResolverOf[*instanceType](sortType, desc.TypeIdx, "")
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortInstance, newImportDefinition(
				sortInstance, astImport.ImportName, tr,
			))
		default:
			return fmt.Errorf("unsupported import sort: %v", desc.Sort)
		}
	case *ast.TypeExternDesc:
		var importType typeResolver
		switch b := desc.Bound.(type) {
		case *ast.EqBound:
			tr := newIndexTypeResolverOf[Type](sortType, b.TypeIdx, "")
			bc.imports[astImport.ImportName] = tr
			return addDefinitionToBuildContext(bc, sortType, newImportDefinition(
				sortType, astImport.ImportName, tr,
			))
		case *ast.SubResourceBound:
			importType = newStaticTypeResolver(&ResourceType{instance: resourceTypeBoundMarker})
		default:
			return fmt.Errorf("unsupported type bound in type import: %T", b)
		}
		bc.imports[astImport.ImportName] = importType
		return addDefinitionToBuildContext(bc, sortType, newImportDefinition(
			sortType, astImport.ImportName, importType,
		))
	default:
		return fmt.Errorf("unsupported import description type: %T", astImport.Desc)
	}
}

func (b *Builder) buildExport(bc *buildContext, astExport *ast.Export) error {

	var exportType typeResolver
	if astExport.ExternDesc != nil {
		var err error
		_, exportTypeResolver, err := astExternDescToTypeResolver(astExport.ExternDesc)
		if err != nil {
			return fmt.Errorf("failed to resolve export type: %w", err)
		}
		exportType = exportTypeResolver
	}

	switch astExport.SortIdx.Sort {
	case ast.SortCoreModule:
		return addExportToComponent(bc, sortCoreModule, astExport.ExportName, astExport.SortIdx.Idx, exportType)
	case ast.SortFunc:
		return addExportToComponent(bc, sortFunction, astExport.ExportName, astExport.SortIdx.Idx, exportType)
	case ast.SortType:
		return addExportToComponent(bc, sortType, astExport.ExportName, astExport.SortIdx.Idx, exportType)
	case ast.SortComponent:
		return addExportToComponent(bc, sortComponent, astExport.ExportName, astExport.SortIdx.Idx, exportType)
	case ast.SortInstance:
		return addExportToComponent(bc, sortInstance, astExport.ExportName, astExport.SortIdx.Idx, exportType)
	}
	return fmt.Errorf("unsupported export sort: %v", astExport.SortIdx.Sort)
}

func addExportToComponent[V any, T Type](bc *buildContext, sort sort[V, T], exportName string, idx uint32, expectedTypeResolver typeResolver) error {
	if idx >= sortDefsFor(bc.defs, sort).len() {
		return fmt.Errorf("export `%s`: index out of bounds", exportName)
	}
	actualType, err := sortScopeFor(bc.scope, sort).getType(idx)
	if err != nil {
		return err
	}

	forceTypeReplace := expectedTypeResolver != nil
	if it, ok := any(actualType).(*instanceType); ok {
		actualType = any(it.clone()).(T)
		forceTypeReplace = true
	}

	if forceTypeReplace {
		exportedType := actualType
		if expectedTypeResolver != nil {
			expectedType, err := expectedTypeResolver.resolveType(bc.scope)
			if err != nil {
				return err
			}

			typeChecker := newTypeChecker()
			if err := typeChecker.checkTypeCompatible(expectedType, actualType); err != nil {
				return fmt.Errorf("error in export `%s`: ascribed type of export is not compatible: %w", exportName, err)
			}
			exportedType = expectedType.(T)
		}

		idx, err = addDefinitionToBuildContextGetIndex(bc, sort, newReferenceDefinitionWithType(sort, idx, exportedType))
		if err != nil {
			return err
		}
	} else {
		var err error
		idx, err = addDefinitionToBuildContextGetIndex(bc, sort, newReferenceDefinition(sort, idx))
		if err != nil {
			return err
		}
	}
	bc.exports[exportName] = newDefComponentExport(sort, idx)
	return nil
}

func (b *Builder) buildCanon(bc *buildContext, astCanon *ast.Canon) error {
	switch def := astCanon.Def.(type) {
	case *ast.CanonLift:
		b.canonIDCounter++
		fnDef, err := canonLift(b.canonIDCounter, def)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortFunction, fnDef)
	case *ast.CanonLower:
		b.canonIDCounter++
		fnDef, err := canonLower(b.canonIDCounter, def)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreFunction, fnDef)
	case *ast.CanonResourceNew:
		b.canonIDCounter++
		fnDef, err := canonResourceNew(b.canonIDCounter, def)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreFunction, fnDef)
	case *ast.CanonResourceDrop:
		b.canonIDCounter++
		fnDef, err := canonResourceDrop(b.canonIDCounter, def)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreFunction, fnDef)
	case *ast.CanonResourceRep:
		b.canonIDCounter++
		fnDef, err := canonResourceRep(b.canonIDCounter, def)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreFunction, fnDef)
	default:
		return fmt.Errorf("unsupported canon def: %T", def)
	}
}

func (b *Builder) buildCoreType(bc *buildContext, core *ast.CoreType) error {
	switch defType := core.DefType.(type) {
	case *ast.CoreRecType:
		recType, err := astRecTypeToTypeResolver(bc.defs, defType)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreType, newTypeResolverDefinition(recType))
	case *ast.CoreModuleType:
		modTypeDef, err := astModuleTypeToCoreModuleTypeResolver(bc.defs, defType)
		if err != nil {
			return err
		}
		return addDefinitionToBuildContext(bc, sortCoreType, modTypeDef)
	default:
		return fmt.Errorf("unsupported core type definition: %T", core.DefType)
	}
}

type buildContext struct {
	defs    *definitions
	scope   *scope
	imports map[string]typeResolver
	exports map[string]componentExport
}

func addDefinitionToBuildContext[V any, T Type](bc *buildContext, sort sort[V, T], def definition[V, T]) error {
	sortDefsFor(bc.defs, sort).add(def)
	return bc.defs.binders[len(bc.defs.binders)-1].bindType(bc.scope)
}

func addDefinitionToBuildContextGetIndex[V any, T Type](bc *buildContext, sort sort[V, T], def definition[V, T]) (uint32, error) {
	idx := sortDefsFor(bc.defs, sort).add(def)
	return idx, bc.defs.binders[len(bc.defs.binders)-1].bindType(bc.scope)
}
