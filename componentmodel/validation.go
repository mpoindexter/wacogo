package componentmodel

import (
	"fmt"
	"iter"
	"strings"

	"github.com/partite-ai/wacogo/ast"
)

// validateCanonLift validates a canon lift definition
func validateCanonLift(scope *definitionScope, def *ast.CanonLift) error {
	// Validate core function index is in range
	if int(def.CoreFuncIdx) >= defs(scope, sortCoreFunction).len() {
		return fmt.Errorf("canon lift: core function index %d out of range", def.CoreFuncIdx)
	}

	// Validate function type index
	if int(def.FunctionTypeIdx) >= defs(scope, sortType).len() {
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
			if int(opt.MemoryIdx) >= defs(scope, sortCoreMemory).len() {
				return fmt.Errorf("canon lift: memory index %d out of range", opt.MemoryIdx)
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lift: duplicate realloc option")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lift: realloc function index %d out of range", opt.FuncIdx)
			}
		case *ast.PostReturnOpt:
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lift: post-return function index %d out of range", opt.FuncIdx)
			}
		}
	}

	return nil
}

// validateCanonLower validates a canon lower definition
func validateCanonLower(scope *definitionScope, def *ast.CanonLower) error {
	// Validate function index is in range
	if int(def.FuncIdx) >= defs(scope, sortFunction).len() {
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
			if int(opt.MemoryIdx) >= defs(scope, sortCoreMemory).len() {
				return fmt.Errorf("canon lower: memory index %d out of range", opt.MemoryIdx)
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lower: duplicate realloc option")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lower: realloc function index %d out of range", opt.FuncIdx)
			}
		}
	}

	return nil
}

// validateCanonResourceNew validates a canon resource.new definition
func validateCanonResourceNew(scope *definitionScope, def *ast.CanonResourceNew) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.new: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceDrop validates a canon resource.drop definition
func validateCanonResourceDrop(scope *definitionScope, def *ast.CanonResourceDrop) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.drop: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceRep validates a canon resource.rep definition
func validateCanonResourceRep(scope *definitionScope, def *ast.CanonResourceRep) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.rep: type index %d out of range", def.TypeIdx)
	}

	return nil
}

// validateAlias validates an alias definition
func validateAlias(scope *definitionScope, alias *ast.Alias) error {
	switch target := alias.Target.(type) {
	case *ast.ExportAlias:
		// Validate instance index
		if int(target.InstanceIdx) >= defs(scope, sortInstance).len() {
			return fmt.Errorf("alias: invalid instance index %d", target.InstanceIdx)
		}
	case *ast.CoreExportAlias:
		// Validate core instance index
		if int(target.InstanceIdx) >= defs(scope, sortCoreInstance).len() {
			return fmt.Errorf("alias: invalid core instance index %d", target.InstanceIdx)
		}
	case *ast.OuterAlias:
		// Validate outer reference
		// Count must be > 0 and the outer scope must exist
		if target.Count == 0 {
			// Referencing current scope - validate index exists
			switch alias.Sort {
			case ast.SortType:
				if int(target.Idx) >= defs(scope, sortType).len() {
					return fmt.Errorf("alias: outer type index %d out of range", target.Idx)
				}
			case ast.SortCoreModule:
				if int(target.Idx) >= defs(scope, sortCoreModule).len() {
					return fmt.Errorf("alias: outer core module index %d out of range", target.Idx)
				}
			case ast.SortComponent:
				if int(target.Idx) >= defs(scope, sortComponent).len() {
					return fmt.Errorf("alias: outer component index %d out of range", target.Idx)
				}
			}
		}
		// For count > 0, we'd need to walk up the scope chain, which requires runtime context
	}

	return nil
}

// validateImport validates an import definition
func validateImport(scope *definitionScope, imp *ast.Import) error {
	switch desc := imp.Desc.(type) {
	case *ast.SortExternDesc:
		// Validate type index if present
		if int(desc.TypeIdx) >= defs(scope, sortType).len() && desc.Sort != ast.SortCoreModule {
			return fmt.Errorf("import %s: type index %d out of range", imp.ImportName, desc.TypeIdx)
		}
		if int(desc.TypeIdx) >= defs(scope, sortCoreType).len() && desc.Sort == ast.SortCoreModule {
			return fmt.Errorf("import %s: core type index %d out of range", imp.ImportName, desc.TypeIdx)
		}
	case *ast.TypeExternDesc:
		// Type imports are always valid as they introduce new types
	}

	return nil
}

// validateExport validates an export definition
func validateExport(scope *definitionScope, exp *ast.Export) error {
	// Validate the sort index
	switch exp.SortIdx.Sort {
	case ast.SortFunc:
		if int(exp.SortIdx.Idx) >= defs(scope, sortFunction).len() {
			return fmt.Errorf("export %s: function index out of bounds", exp.ExportName)
		}
	case ast.SortType:
		if int(exp.SortIdx.Idx) >= defs(scope, sortType).len() {
			return fmt.Errorf("export %s: type index out of bounds", exp.ExportName)
		}
	case ast.SortInstance:
		if int(exp.SortIdx.Idx) >= defs(scope, sortInstance).len() {
			return fmt.Errorf("export %s: instance index out of bounds", exp.ExportName)
		}
	case ast.SortComponent:
		if int(exp.SortIdx.Idx) >= defs(scope, sortComponent).len() {
			return fmt.Errorf("export %s: component index out of bounds", exp.ExportName)
		}
	case ast.SortCoreModule:
		if int(exp.SortIdx.Idx) >= defs(scope, sortCoreModule).len() {
			return fmt.Errorf("export %s: module index out of bounds", exp.ExportName)
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

func parameterNeedsMemory(param *FunctionParameter) bool {
	return typeNeedsMemory(param.Type)
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

func parameterNeedsRealloc(param *FunctionParameter) bool {
	return typeNeedsRealloc(param.Type)
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

func validateExportNameStronglyUnique(exportNames iter.Seq[string], name string) error {
	for en := range exportNames {
		if !stronglyUnique(en, name) {
			return fmt.Errorf("export name `%s` conflicts with previous name `%s`", name, en)
		}
	}
	return nil
}

func validateImportNameStronglyUnique(importNames iter.Seq[string], name string) error {
	for in := range importNames {
		if !stronglyUnique(in, name) {
			return fmt.Errorf("import name `%s` conflicts with previous name `%s`", name, in)
		}
	}
	return nil
}

func stronglyUnique(a, b string) bool {
	/*
			To determine whether two names (defined as sequences of Unicode Scalar Values) are strongly-unique:

		If one name is l and the other name is [constructor]l (for the same label l), they are strongly-unique.
		If one name is l and the other name is [*]l.l (for the same label l and any annotation * with a dotted l.l name), they are not strongly-unique.
		Otherwise:
		Lowercase all the acronyms (uppercase letters) in both names.
		Strip any [...] annotation prefix from both names.
		The names are strongly-unique if the resulting strings are unequal.
	*/

	stripAnnotation := func(s string) string {
		if strings.HasPrefix(s, "[") {
			if idx := strings.Index(s, "]"); idx != -1 {
				return s[idx+1:]
			}
		}
		return s
	}

	normalize := func(s string) string {
		var result strings.Builder
		runes := []rune(s)
		i := 0
		for i < len(runes) {
			r := runes[i]
			if r >= 'A' && r <= 'Z' {
				// Start of an acronym
				for i < len(runes) && runes[i] >= 'A' && runes[i] <= 'Z' {
					result.WriteRune(runes[i] + ('a' - 'A'))
					i++
				}
			} else {
				result.WriteRune(r)
				i++
			}
		}
		return result.String()
	}

	// Check special cases
	if a == b {
		return false
	}
	if a == "[constructor]"+b || b == "[constructor]"+a {
		return true
	}
	if strings.HasPrefix(a, "[") && strings.Contains(a, "].") {
		suffix := a[strings.Index(a, "].")+2:]
		if b == suffix {
			return false
		}
	}
	if strings.HasPrefix(b, "[") && strings.Contains(b, "].") {
		suffix := b[strings.Index(b, "].")+2:]
		if a == suffix {
			return false
		}
	}

	a = stripAnnotation(a)
	b = stripAnnotation(b)

	// Normalize and compare
	normA := normalize(a)
	normB := normalize(b)
	return normA != normB
}
