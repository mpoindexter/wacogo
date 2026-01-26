package componentmodel

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero/api"
)

// validateCanonLift validates a canon lift definition
func validateCanonLift(scope *definitionScope, def *ast.CanonLift) error {
	// Validate core function index is in range
	if int(def.CoreFuncIdx) >= defs(scope, sortCoreFunction).len() {
		return fmt.Errorf("canon lift: core function index %d out of range", def.CoreFuncIdx)
	}

	coreFnDef, err := defs(scope, sortCoreFunction).get(def.CoreFuncIdx)
	if err != nil {
		return fmt.Errorf("canon lift: failed to get core function at index %d: %v", def.CoreFuncIdx, err)
	}

	coreFnType := coreFnDef.typ()

	// Validate function type index
	if int(def.FunctionTypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon lift: function type index out of bounds: %d", def.FunctionTypeIdx)
	}

	fnDef, err := defs(scope, sortType).get(def.FunctionTypeIdx)
	if err != nil {
		return fmt.Errorf("canon lift: failed to get function type at index %d: %v", def.FunctionTypeIdx, err)
	}

	fnType, ok := fnDef.typ().(*FunctionType)
	if !ok {
		return fmt.Errorf("canon lift: type at index %d is not a function type", def.FunctionTypeIdx)
	}

	wazeroParamTypes, wazeroResultTypes, paramsFlat, returnFlat := liftedCoreFunctionTypesFromFunctionType(fnType)
	coreParamTypes := make([]Type, len(wazeroParamTypes))
	for i, p := range wazeroParamTypes {
		coreParamTypes[i] = coreTypeWasmConstTypeFromWazero(p)
	}
	coreResultTypes := make([]Type, len(wazeroResultTypes))
	for i, r := range wazeroResultTypes {
		coreResultTypes[i] = coreTypeWasmConstTypeFromWazero(r)
	}

	// Validate that the lifted core function types match the function type
	if !slices.Equal(coreFnType.paramTypes, coreParamTypes) {
		return fmt.Errorf("lowered parameter types `%s` do not match parameter types `%s`", formatCoreTypesSignature(coreParamTypes), formatCoreTypesSignature(coreFnType.paramTypes))
	}

	if !slices.Equal(coreFnType.resultTypes, coreResultTypes) {
		return fmt.Errorf("lowered result types `%s` do not match result types `%s`", formatCoreTypesSignature(coreResultTypes), formatCoreTypesSignature(coreFnType.resultTypes))
	}

	// Check if memory/realloc are needed and provided
	needsMemory := !paramsFlat || !returnFlat
	if slices.ContainsFunc(fnType.Parameters, parameterNeedsMemory) {
		needsMemory = true
	}
	if fnType.ResultType != nil && typeNeedsMemory(fnType.ResultType) {
		needsMemory = true
	}

	needsRealloc := slices.ContainsFunc(fnType.Parameters, parameterNeedsRealloc) || !paramsFlat

	// Validate canon options
	var hasMemory, hasRealloc, hasPostReturn bool
	for _, opt := range def.Options {
		switch opt := opt.(type) {
		case *ast.MemoryOpt:
			if hasMemory {
				return fmt.Errorf("canon lift: `memory` is specified more than once")
			}
			hasMemory = true
			if int(opt.MemoryIdx) >= defs(scope, sortCoreMemory).len() {
				return fmt.Errorf("canon lift: memory index out of bounds")
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lift: canonical option `realloc` is specified more than once")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lift: realloc function index %d out of range", opt.FuncIdx)
			}
			fnDef, err := defs(scope, sortCoreFunction).get(opt.FuncIdx)
			if err != nil {
				return fmt.Errorf("canon lift: failed to get realloc function at index %d: %v", opt.FuncIdx, err)
			}
			fnTyp := fnDef.typ()
			if !slices.Equal(fnTyp.paramTypes, []Type{
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
			}) || !slices.Equal(fnTyp.resultTypes, []Type{
				coreTypeWasmConstType(api.ValueTypeI32),
			}) {
				return fmt.Errorf("canonical option `realloc` uses a core function with an incorrect signature")
			}
		case *ast.PostReturnOpt:
			if hasPostReturn {
				return fmt.Errorf("canon lift: canonical option `post-return` is specified more than once")
			}
			hasPostReturn = true
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lift: post-return function index %d out of range", opt.FuncIdx)
			}

			postReturnCoreFnDef, err := defs(scope, sortCoreFunction).get(opt.FuncIdx)
			if err != nil {
				return fmt.Errorf("canon lift: failed to get post-return function at index %d: %v", opt.FuncIdx, err)
			}
			postReturnCoreFnType := postReturnCoreFnDef.typ()

			if !slices.Equal(postReturnCoreFnType.paramTypes, coreResultTypes) || !slices.Equal(postReturnCoreFnType.resultTypes, []Type{}) {
				return fmt.Errorf("canonical option `post-return` uses a core function with an incorrect signature")
			}
		}
	}

	if err := validateCanonicalABIOptions(hasMemory, hasRealloc, needsMemory, needsRealloc); err != nil {
		return fmt.Errorf("canon lift validation failed: %w", err)
	}

	return nil
}

// validateCanonLower validates a canon lower definition
func validateCanonLower(scope *definitionScope, def *ast.CanonLower) error {
	// Validate function index is in range
	if int(def.FuncIdx) >= defs(scope, sortFunction).len() {
		return fmt.Errorf("canon lower: function index %d out of range", def.FuncIdx)
	}

	fnDef, err := defs(scope, sortFunction).get(def.FuncIdx)
	if err != nil {
		return fmt.Errorf("canon lower: failed to get function at index %d: %v", def.FuncIdx, err)
	}

	fnTyp := fnDef.typ()

	_, _, paramsFlat, returnFlat := loweredCoreFunctionTypesFromFunctionType(fnTyp)

	// Check if memory/realloc are needed and provided
	needsMemory := !paramsFlat || !returnFlat
	if slices.ContainsFunc(fnTyp.Parameters, parameterNeedsMemory) {
		needsMemory = true
	}
	if fnTyp.ResultType != nil && typeNeedsMemory(fnTyp.ResultType) {
		needsMemory = true
	}

	needsRealloc := false
	if fnTyp.ResultType != nil && typeNeedsRealloc(fnTyp.ResultType) {
		needsRealloc = true
	}

	encoding := ""

	// Validate canon options
	var hasMemory, hasRealloc bool
	for _, opt := range def.Options {
		switch opt := opt.(type) {
		case *ast.MemoryOpt:
			if hasMemory {
				return fmt.Errorf("canon lower: `memory` is specified more than once")
			}
			hasMemory = true
			if int(opt.MemoryIdx) >= defs(scope, sortCoreMemory).len() {
				return fmt.Errorf("canon lower: memory index out of bounds")
			}
		case *ast.ReallocOpt:
			if hasRealloc {
				return fmt.Errorf("canon lower: canonical option `realloc` is specified more than once")
			}
			hasRealloc = true
			if int(opt.FuncIdx) >= defs(scope, sortCoreFunction).len() {
				return fmt.Errorf("canon lower: realloc function index %d out of range", opt.FuncIdx)
			}
			fnDef, err := defs(scope, sortCoreFunction).get(opt.FuncIdx)
			if err != nil {
				return fmt.Errorf("canon lower: failed to get realloc function at index %d: %v", opt.FuncIdx, err)
			}
			fnTyp := fnDef.typ()
			if !slices.Equal(fnTyp.paramTypes, []Type{
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
				coreTypeWasmConstType(api.ValueTypeI32),
			}) || !slices.Equal(fnTyp.resultTypes, []Type{
				coreTypeWasmConstType(api.ValueTypeI32),
			}) {
				return fmt.Errorf("canonical option `realloc` uses a core function with an incorrect signature")
			}
		case *ast.StringEncodingOpt:
			var optEncoding string
			switch opt.Encoding {
			case ast.StringEncodingUTF8:
				optEncoding = "utf8"
			case ast.StringEncodingUTF16:
				optEncoding = "utf16"
			case ast.StringEncodingLatin1UTF16:
				optEncoding = "latin1-utf16"
			default:
				return fmt.Errorf("canon lower: unknown string encoding option %v", opt.Encoding)
			}
			if encoding != "" {
				return fmt.Errorf("canonical encoding option `%s` conflicts with option `%s`", encoding, optEncoding)
			}
			encoding = optEncoding
		case *ast.PostReturnOpt:
			return fmt.Errorf("canonical option `post-return` cannot be specified for lowerings")
		}
	}

	if err := validateCanonicalABIOptions(hasMemory, hasRealloc, needsMemory, needsRealloc); err != nil {
		return fmt.Errorf("canon lower validation failed: %w", err)
	}

	return nil
}

// validateCanonResourceNew validates a canon resource.new definition
func validateCanonResourceNew(scope *definitionScope, def *ast.CanonResourceNew) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.new: type index out of bounds: %d", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceDrop validates a canon resource.drop definition
func validateCanonResourceDrop(scope *definitionScope, def *ast.CanonResourceDrop) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.drop: type index out of bounds: %d", def.TypeIdx)
	}

	return nil
}

// validateCanonResourceRep validates a canon resource.rep definition
func validateCanonResourceRep(scope *definitionScope, def *ast.CanonResourceRep) error {
	// Validate type index is in range
	if int(def.TypeIdx) >= defs(scope, sortType).len() {
		return fmt.Errorf("canon resource.rep: type index out of bounds: %d", def.TypeIdx)
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
					return fmt.Errorf("alias: outer type index out of bounds: %d", target.Idx)
				}
			case ast.SortCoreModule:
				if int(target.Idx) >= defs(scope, sortCoreModule).len() {
					return fmt.Errorf("alias: outer core module index out of bounds: %d", target.Idx)
				}
			case ast.SortComponent:
				if int(target.Idx) >= defs(scope, sortComponent).len() {
					return fmt.Errorf("alias: outer component index out of bounds: %d", target.Idx)
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
			return fmt.Errorf("import %s: type index out of bounds: %d", imp.ImportName, desc.TypeIdx)
		}
		if int(desc.TypeIdx) >= defs(scope, sortCoreType).len() && desc.Sort == ast.SortCoreModule {
			return fmt.Errorf("import %s: core type index out of bounds: %d", imp.ImportName, desc.TypeIdx)
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
		return fmt.Errorf("canonical option `memory` is required")
	}
	if needsRealloc && !hasRealloc {
		return fmt.Errorf("canonical option `realloc` is required")
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

func validateParameterNameStronglyUnique(paramNames iter.Seq[string], name string) error {
	for pn := range paramNames {
		if !stronglyUnique(pn, name) {
			return fmt.Errorf("function parameter name `%s` conflicts with previous parameter name `%s`", name, pn)
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

func formatCoreTypesSignature(params []Type) string {
	var parts []string
	for _, p := range params {
		switch pt := p.(type) {
		case coreTypeWasmConstType:
			switch pt {
			case coreTypeWasmConstTypeFromWazero(api.ValueTypeI32):
				parts = append(parts, "I32")
			case coreTypeWasmConstTypeFromWazero(api.ValueTypeI64):
				parts = append(parts, "I64")
			case coreTypeWasmConstTypeFromWazero(api.ValueTypeF32):
				parts = append(parts, "F32")
			case coreTypeWasmConstTypeFromWazero(api.ValueTypeF64):
				parts = append(parts, "F64")
			case coreTypeWasmConstTypeV128:
				parts = append(parts, "V128")
			case coreTypeWasmConstTypeFuncref:
				parts = append(parts, "funcref")
			case coreTypeWasmConstTypeFromWazero(api.ValueTypeExternref):
				parts = append(parts, "externref")
			default:
				parts = append(parts, "unknown")
			}
		default:
			parts = append(parts, "unknown")
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
