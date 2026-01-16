package componentmodel

import (
	"fmt"

	"github.com/partite-ai/wacogo/ast"
)

// validateCanonLift validates a canon lift definition
func validateCanonLift(scope *componentDefinitionScope, def *ast.CanonLift) error {
	// Validate core function index is in range
	if int(def.CoreFuncIdx) >= len(scope.coreFunctions) {
		return fmt.Errorf("canon lift: core function index %d out of range", def.CoreFuncIdx)
	}

	// Validate function type index
	if int(def.FunctionTypeIdx) >= len(scope.componentModelTypes) {
		return fmt.Errorf("canon lift: function type index %d out of range", def.FunctionTypeIdx)
	}

	// Validate canon options
	var hasMemory, hasRealloc bool
	for _, opt := range def.Options {
		switch opt := opt.(type) {
		case *ast.MemoryOpt:
			if hasMemory {
				return fmt.Errorf("canon lift: duplicate memory option")
			}
			hasMemory = true
			if int(opt.MemoryIdx) >= len(scope.coreMemories) {
				return fmt.Errorf("canon lift: memory index %d out of range", opt.MemoryIdx)
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lift: duplicate realloc option")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= len(scope.coreFunctions) {
				return fmt.Errorf("canon lift: realloc function index %d out of range", opt.FuncIdx)
			}
		case *ast.PostReturnOpt:
			if int(opt.FuncIdx) >= len(scope.coreFunctions) {
				return fmt.Errorf("canon lift: post-return function index %d out of range", opt.FuncIdx)
			}
		}
	}

	return nil
}

// validateCanonLower validates a canon lower definition
func validateCanonLower(scope *componentDefinitionScope, def *ast.CanonLower) error {
	// Validate function index is in range
	if int(def.FuncIdx) >= len(scope.functions) {
		return fmt.Errorf("canon lower: function index %d out of range", def.FuncIdx)
	}

	// Validate canon options
	var hasMemory, hasRealloc bool
	for _, opt := range def.Options {
		switch opt := opt.(type) {
		case *ast.MemoryOpt:
			if hasMemory {
				return fmt.Errorf("canon lower: duplicate memory option")
			}
			hasMemory = true
			if int(opt.MemoryIdx) >= len(scope.coreMemories) {
				return fmt.Errorf("canon lower: memory index %d out of range", opt.MemoryIdx)
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lower: duplicate realloc option")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= len(scope.coreFunctions) {
				return fmt.Errorf("canon lower: realloc function index %d out of range", opt.FuncIdx)
			}
		}
	}

	return nil
}

// validateCanonResourceNew validates a canon resource.new definition
func validateCanonResourceNew(scope *componentDefinitionScope, def *ast.CanonResourceNew) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= len(scope.componentModelTypes) {
		return fmt.Errorf("canon resource.new: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceDrop validates a canon resource.drop definition
func validateCanonResourceDrop(scope *componentDefinitionScope, def *ast.CanonResourceDrop) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= len(scope.componentModelTypes) {
		return fmt.Errorf("canon resource.drop: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceRep validates a canon resource.rep definition
func validateCanonResourceRep(scope *componentDefinitionScope, def *ast.CanonResourceRep) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= len(scope.componentModelTypes) {
		return fmt.Errorf("canon resource.rep: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateAlias validates an alias definition
func validateAlias(scope *componentDefinitionScope, alias *ast.Alias) error {
	switch target := alias.Target.(type) {
	case *ast.ExportAlias:
		// Validate instance index
		if int(target.InstanceIdx) >= len(scope.instances) {
			return fmt.Errorf("alias: invalid instance index %d", target.InstanceIdx)
		}
	case *ast.CoreExportAlias:
		// Validate core instance index
		if int(target.InstanceIdx) >= len(scope.coreInstances) {
			return fmt.Errorf("alias: invalid core instance index %d", target.InstanceIdx)
		}
	case *ast.OuterAlias:
		// Validate outer reference
		// Count must be > 0 and the outer scope must exist
		if target.Count == 0 {
			// Referencing current scope - validate index exists
			switch alias.Sort {
			case ast.SortType:
				if int(target.Idx) >= len(scope.componentModelTypes) {
					return fmt.Errorf("alias: outer type index %d out of range", target.Idx)
				}
			case ast.SortCoreModule:
				if int(target.Idx) >= len(scope.coreModules) {
					return fmt.Errorf("alias: outer core module index %d out of range", target.Idx)
				}
			case ast.SortComponent:
				if int(target.Idx) >= len(scope.components) {
					return fmt.Errorf("alias: outer component index %d out of range", target.Idx)
				}
			}
		}
		// For count > 0, we'd need to walk up the scope chain, which requires runtime context
	}

	return nil
}

// validateImport validates an import definition
func validateImport(scope *componentDefinitionScope, imp *ast.Import) error {
	switch desc := imp.Desc.(type) {
	case *ast.SortExternDesc:
		// Validate type index if present
		if int(desc.TypeIdx) >= len(scope.componentModelTypes) && desc.Sort != ast.SortCoreModule {
			return fmt.Errorf("import %s: type index %d out of range", imp.ImportName, desc.TypeIdx)
		}
		if int(desc.TypeIdx) >= len(scope.coreTypes) && desc.Sort == ast.SortCoreModule {
			return fmt.Errorf("import %s: core type index %d out of range", imp.ImportName, desc.TypeIdx)
		}
	case *ast.TypeExternDesc:
		// Type imports are always valid as they introduce new types
	}

	return nil
}

// validateExport validates an export definition
func validateExport(scope *componentDefinitionScope, exp *ast.Export) error {
	// Validate the sort index
	switch exp.SortIdx.Sort {
	case ast.SortFunc:
		if int(exp.SortIdx.Idx) >= len(scope.functions) {
			return fmt.Errorf("export %s: function index %d out of range", exp.ExportName, exp.SortIdx.Idx)
		}
	case ast.SortType:
		if int(exp.SortIdx.Idx) >= len(scope.componentModelTypes) {
			return fmt.Errorf("export %s: type index %d out of range", exp.ExportName, exp.SortIdx.Idx)
		}
	case ast.SortInstance:
		if int(exp.SortIdx.Idx) >= len(scope.instances) {
			return fmt.Errorf("export %s: instance index %d out of range", exp.ExportName, exp.SortIdx.Idx)
		}
	case ast.SortComponent:
		if int(exp.SortIdx.Idx) >= len(scope.components) {
			return fmt.Errorf("export %s: component index %d out of range", exp.ExportName, exp.SortIdx.Idx)
		}
	case ast.SortCoreModule:
		if int(exp.SortIdx.Idx) >= len(scope.coreModules) {
			return fmt.Errorf("export %s: core module index %d out of range", exp.ExportName, exp.SortIdx.Idx)
		}
	}

	return nil
}

// validateCanonicalABIOptions validates canonical ABI options
func validateCanonicalABIOptions(hasMemory, hasRealloc, needsMemory, needsRealloc bool) error {
	if needsMemory && !hasMemory {
		return fmt.Errorf("canonical ABI requires memory option")
	}
	if needsRealloc && !hasRealloc {
		return fmt.Errorf("canonical ABI requires realloc option")
	}
	return nil
}

// typeNeedsMemory checks if a type requires memory for canonical ABI
func typeNeedsMemory(t ValueType) bool {
	switch vt := t.(type) {
	case *RecordType:
		for _, field := range vt.Fields {
			if typeNeedsMemory(field.Type) {
				return true
			}
		}
	case *VariantType:
		for _, c := range vt.Cases {
			if c.Type != nil && typeNeedsMemory(c.Type) {
				return true
			}
		}
	case *ListType, StringType, ByteArrayType:
		return true
	}
	return false
}

// typeNeedsRealloc checks if a type requires realloc for canonical ABI
func typeNeedsRealloc(t ValueType) bool {
	// Realloc is needed for types that are allocated in memory
	switch vt := t.(type) {
	case *ListType, StringType, ByteArrayType:
		return true
	case *RecordType:
		for _, field := range vt.Fields {
			if typeNeedsRealloc(field.Type) {
				return true
			}
		}
	case *VariantType:
		for _, c := range vt.Cases {
			if c.Type != nil && typeNeedsRealloc(c.Type) {
				return true
			}
		}
	}
	return false
}
