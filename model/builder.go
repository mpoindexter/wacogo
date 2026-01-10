package model

import (
	"context"
	"fmt"

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
	var parentScope definitionScope
	if parent != nil {
		parentScope = &parent.scope
		id = fmt.Sprintf("%s_%d", parent.id, len(parent.scope.components))
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
	fixed, err := wasm.TransformBlankImportNames(astMod.Raw)
	if err != nil {
		return fmt.Errorf("failed to transform blank import names: %w", err)
	}
	tableTypes, err := wasm.ReadTableExports(fixed)
	if err != nil {
		return fmt.Errorf("failed to read table exports: %w", err)
	}
	compiled, err := b.runtime.CompileModule(ctx, fixed)
	if err != nil {
		return fmt.Errorf("failed to compile core module: %w", err)
	}

	comp.scope.coreModules = append(comp.scope.coreModules, &coreModuleStaticDefinition{
		coreModule: &coreModule{
			module:     compiled,
			tableTypes: tableTypes,
		},
	})
	return nil
}

func (b *Builder) buildCoreInstance(ctx context.Context, comp *Component, astInst *ast.CoreInstance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.CoreInstantiate:
		args := make(map[string]uint32)
		for _, astArg := range expr.Args {
			args[astArg.Name] = astArg.CoreInstanceIdx
		}
		def := &coreInstantiateDefinition{
			moduleIndex: expr.ModuleIdx,
			args:        args,
		}
		comp.scope.coreInstances = append(comp.scope.coreInstances, def)
		def.instanceIdx = uint32(len(comp.scope.coreInstances) - 1)
	case *ast.CoreInlineExports:
		def := &coreInlineExportsDefinition{
			exports: expr.Exports,
		}
		comp.scope.coreInstances = append(comp.scope.coreInstances, def)
		def.instanceIdx = uint32(len(comp.scope.coreInstances) - 1)
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

	comp.scope.components = append(comp.scope.components, &componentStaticDefinition{component: nestedComp})
	return nil
}

func (b *Builder) buildInstance(comp *Component, astInst *ast.Instance) error {
	switch expr := astInst.Expr.(type) {
	case *ast.Instantiate:
		def := &instantiateDefinition{
			componentIdx: expr.ComponentIdx,
			args:         expr.Args,
		}
		comp.scope.instances = append(comp.scope.instances, def)
		def.instanceIdx = uint32(len(comp.scope.instances) - 1)
	case *ast.InlineExports:
		def := &inlineExportsDefinition{
			exports: expr.Exports,
		}
		comp.scope.instances = append(comp.scope.instances, def)
		def.instanceIdx = uint32(len(comp.scope.instances) - 1)
	default:
		return fmt.Errorf("invalid instance expression type: %T", astInst.Expr)
	}
	return nil
}

func (b *Builder) buildAlias(comp *Component, astAlias *ast.Alias) error {
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
		comp.scope.coreFunctions = append(comp.scope.coreFunctions, &coreFunctionExportDefinition{
			instanceIdx: alias.InstanceIdx,
			funcName:    alias.Name,
		})
		return nil
	case ast.SortCoreGlobal:
		comp.scope.coreGlobals = append(comp.scope.coreGlobals, &coreInstanceGlobalExportDefinition{
			instanceIdx: alias.InstanceIdx,
			globalName:  alias.Name,
		})
		return nil
	case ast.SortCoreMemory:
		comp.scope.coreMemories = append(comp.scope.coreMemories, &coreInstanceMemoryExportDefinition{
			instanceIdx: alias.InstanceIdx,
			memoryName:  alias.Name,
		})
		return nil
	case ast.SortCoreType:
		return fmt.Errorf("core type export alias resolution not yet supported")
	case ast.SortCoreTable:
		comp.scope.coreTables = append(comp.scope.coreTables, &coreInstanceTableExportDefinition{
			instanceIdx: alias.InstanceIdx,
			tableName:   alias.Name,
		})
		return nil
	default:
		return fmt.Errorf("unsupported core export alias sort: %v", sort)
	}
}

func (b *Builder) buildExportAlias(comp *Component, sort ast.Sort, alias *ast.ExportAlias) error {
	switch sort {
	case ast.SortFunc:
		comp.scope.functions = append(comp.scope.functions, &functionAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			funcName:    alias.Name,
		})
		return nil
	case ast.SortType:
		comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, &typeAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			exportName:  alias.Name,
		})
		return nil
	case ast.SortComponent:
		comp.scope.components = append(comp.scope.components, &componentAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			exportName:  alias.Name,
		})
		return nil
	case ast.SortCoreModule:
		comp.scope.coreModules = append(comp.scope.coreModules, &coreModuleInstanceExportAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			exportName:  alias.Name,
		})
		return nil
	case ast.SortInstance:
		comp.scope.instances = append(comp.scope.instances, &instanceAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			exportName:  alias.Name,
		})
		return nil
	case ast.SortCoreInstance:
		comp.scope.coreInstances = append(comp.scope.coreInstances, &coreInstanceAliasDefinition{
			instanceIdx: alias.InstanceIdx,
			exportName:  alias.Name,
		})
		return nil
	default:
		return fmt.Errorf("unsupported export alias sort: %v", sort)
	}
}

func (b *Builder) buildOuterAlias(comp *Component, sort ast.Sort, alias *ast.OuterAlias) error {
	switch sort {
	case ast.SortType:
		typeDef, err := comp.scope.resolveComponentModelTypeDefinition(alias.Count, alias.Idx)
		if err != nil {
			return err
		}
		comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, typeDef)
		return nil
	case ast.SortCoreModule:
		coreModuleDef, err := comp.scope.resolveCoreModuleDefinition(alias.Count, alias.Idx)
		if err != nil {
			return err
		}
		comp.scope.coreModules = append(comp.scope.coreModules, coreModuleDef)
		return nil
	case ast.SortComponent:
		componentDef, err := comp.scope.resolveComponentDefinition(alias.Count, alias.Idx)
		if err != nil {
			return err
		}
		comp.scope.components = append(comp.scope.components, componentDef)
		return nil
	default:
		return fmt.Errorf("unsupported outer alias sort: %v", sort)
	}
}

func (b *Builder) buildType(comp *Component, astType *ast.Type) error {
	def, err := astTypeToTypeDefinition(&comp.scope, astType.DefType)
	if err != nil {
		return err
	}
	comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, def)
	return nil
}

func (b *Builder) buildImport(comp *Component, astImport *ast.Import) error {
	switch desc := astImport.Desc.(type) {
	case *ast.SortExternDesc:
		switch desc.Sort {
		case ast.SortCoreModule:
			comp.scope.coreModules = append(comp.scope.coreModules, &coreModuleImportDefinition{
				name: astImport.ImportName,
			})
			return nil
		case ast.SortFunc:
			comp.scope.functions = append(comp.scope.functions, &functionImportDefinition{
				name: astImport.ImportName,
			})
			return nil
		case ast.SortType:
			comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, &typeImportDefinition{
				name: astImport.ImportName,
			})
			return nil
		case ast.SortComponent:
			comp.scope.components = append(comp.scope.components, &componentImportDefinition{
				name: astImport.ImportName,
			})
			return nil
		case ast.SortInstance:
			comp.scope.instances = append(comp.scope.instances, &instanceImportDefinition{
				name: astImport.ImportName,
			})
			return nil
		default:
			return fmt.Errorf("unsupported import sort: %v", desc.Sort)
		}
	case *ast.TypeExternDesc:
		comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, &typeImportDefinition{
			name: astImport.ImportName,
		})
		return nil
	default:
		return fmt.Errorf("unsupported import description type: %T", astImport.Desc)
	}
}

func (b *Builder) buildExport(comp *Component, astExport *ast.Export) error {
	switch astExport.SortIdx.Sort {
	case ast.SortCoreModule:
		def, err := comp.scope.resolveCoreModuleDefinition(0, astExport.SortIdx.Idx)
		if err != nil {
			return err
		}
		comp.scope.coreModules = append(comp.scope.coreModules, def)
		comp.exports[astExport.ExportName] = func(ctx context.Context, scope instanceScope) (any, error) {
			return def.resolveCoreModule(ctx, scope)
		}
		return nil
	case ast.SortFunc:
		def, err := comp.scope.resolveFunctionDefinition(0, astExport.SortIdx.Idx)
		if err != nil {
			return err
		}
		comp.scope.functions = append(comp.scope.functions, def)
		comp.exports[astExport.ExportName] = func(ctx context.Context, scope instanceScope) (any, error) {
			return def.resolveFunction(ctx, scope)
		}
		return nil
	case ast.SortType:
		def := comp.scope.componentModelTypes[astExport.SortIdx.Idx]
		comp.scope.componentModelTypes = append(comp.scope.componentModelTypes, def)
		comp.exports[astExport.ExportName] = func(ctx context.Context, scope instanceScope) (any, error) {
			return def.resolveType(ctx, scope)
		}
		return nil
	case ast.SortComponent:
		def := comp.scope.components[astExport.SortIdx.Idx]
		comp.scope.components = append(comp.scope.components, def)
		comp.exports[astExport.ExportName] = func(ctx context.Context, scope instanceScope) (any, error) {
			return def.resolveComponent(ctx, scope)
		}
		return nil
	case ast.SortInstance:
		def := comp.scope.instances[astExport.SortIdx.Idx]
		comp.scope.instances = append(comp.scope.instances, def)
		instanceIdx := len(comp.scope.instances) - 1
		comp.exports[astExport.ExportName] = func(ctx context.Context, scope instanceScope) (any, error) {
			return scope.resolveInstance(ctx, uint32(instanceIdx))
		}
		return nil
	}
	return fmt.Errorf("unsupported export sort: %v", astExport.SortIdx.Sort)
}

func (b *Builder) buildCanon(comp *Component, astCanon *ast.Canon) error {
	switch def := astCanon.Def.(type) {
	case *ast.CanonLift:
		comp.scope.functions = append(comp.scope.functions, &functionCanonLift{lift: def})
		return nil
	case *ast.CanonLower:
		fnDef, err := canonLower(comp, def)
		if err != nil {
			return err
		}
		comp.scope.coreFunctions = append(comp.scope.coreFunctions, fnDef)
		return nil
	case *ast.CanonResourceNew:
		// TODO: implement resource new
		return fmt.Errorf("resource new not yet supported")
	case *ast.CanonResourceDrop:
		fnDef, err := canonResourceDrop(comp, def)
		if err != nil {
			return err
		}
		comp.scope.coreFunctions = append(comp.scope.coreFunctions, fnDef)
		return nil
	case *ast.CanonResourceRep:
		// TODO: implement resource rep
		return fmt.Errorf("resource rep not yet supported")
	default:
		return fmt.Errorf("unsupported canon def: %T", def)
	}
}

func (b *Builder) buildCoreType(comp *Component, core *ast.CoreType) error {
	switch defType := core.DefType.(type) {
	case *ast.CoreRecType:
		recTypeDef, err := astRecTypeToCoreTypeDefinition(&comp.scope, defType)
		if err != nil {
			return err
		}
		comp.scope.coreTypes = append(comp.scope.coreTypes, recTypeDef)
		return nil
	case *ast.CoreModuleType:
		modTypeDef, err := astModuleTypeToCoreModuleTypeDefinition(&comp.scope, defType)
		if err != nil {
			return err
		}
		comp.scope.coreTypes = append(comp.scope.coreTypes, modTypeDef)
		return nil
	default:
		return fmt.Errorf("unsupported core type definition: %T", core.DefType)
	}
}
