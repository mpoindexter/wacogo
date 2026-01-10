package ast

import (
	"fmt"
	"strings"
)

// ToWAT converts the Component AST to WAT (WebAssembly Text) format
func (c *Component) ToWAT() string {
	var b strings.Builder
	b.WriteString("(component")
	b.WriteString("\n")

	for _, def := range c.Definitions {
		indent(&b, 1)
		b.WriteString(defToWAT(def, 1))
		b.WriteString("\n")
	}

	b.WriteString(")")
	return b.String()
}

func indent(b *strings.Builder, level int) {
	for i := 0; i < level; i++ {
		b.WriteString("  ")
	}
}

func defToWAT(def Definition, level int) string {
	switch d := def.(type) {
	case *CoreModule:
		return coreModuleToWAT(d)
	case *CoreInstance:
		return coreInstanceToWAT(d)
	case *NestedComponent:
		return nestedComponentToWAT(d, level)
	case *Instance:
		return instanceToWAT(d)
	case *Alias:
		return aliasToWAT(d)
	case *Type:
		return typeToWAT(d, level)
	case *Import:
		return importToWAT(d, level)
	case *Export:
		return exportToWAT(d)
	case *Canon:
		return canonToWAT(d)
	default:
		return fmt.Sprintf("(; unknown definition type: %T ;)", def)
	}
}

func coreModuleToWAT(m *CoreModule) string {
	var b strings.Builder
	b.WriteString("(core module")
	if len(m.Raw) > 0 {
		b.WriteString(" (binary")
		// Show first few bytes
		limit := 16
		if len(m.Raw) < limit {
			limit = len(m.Raw)
		}
		for i := 0; i < limit; i++ {
			b.WriteString(fmt.Sprintf(" %02x", m.Raw[i]))
		}
		if len(m.Raw) > limit {
			b.WriteString(" ...")
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func coreInstanceToWAT(ci *CoreInstance) string {
	var b strings.Builder
	b.WriteString("(core instance")
	b.WriteString(" ")
	b.WriteString(coreInstanceExprToWAT(ci.Expr, 0))
	b.WriteString(")")
	return b.String()
}

func coreInstanceExprToWAT(expr CoreInstanceExpr, level int) string {
	switch e := expr.(type) {
	case *CoreInstantiate:
		return coreInstantiateToWAT(e, level)
	case *CoreInlineExports:
		return coreInlineExportsToWAT(e)
	default:
		return fmt.Sprintf("(; unknown core instance expr: %T ;)", expr)
	}
}

func coreInstantiateToWAT(ci *CoreInstantiate, level int) string {
	var b strings.Builder
	b.WriteString("(instantiate ")
	b.WriteString(fmt.Sprintf("%d", ci.ModuleIdx))
	for _, arg := range ci.Args {
		b.WriteString("\n")
		indent(&b, level+2)
		b.WriteString("(with \"")
		b.WriteString(arg.Name)
		b.WriteString("\" ")
		b.WriteString(fmt.Sprintf("(instance %d)", arg.CoreInstanceIdx))
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func coreInlineExportsToWAT(e *CoreInlineExports) string {
	var b strings.Builder
	for _, export := range e.Exports {
		b.WriteString(coreInlineExportToWAT(&export))
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}

func coreInlineExportToWAT(e *CoreInlineExport) string {
	return fmt.Sprintf("(export \"%s\" (%s %d))",
		e.Name,
		coreSortToString(e.SortIdx.Sort),
		e.SortIdx.Idx)
}

func coreSortToString(sort CoreSort) string {
	switch sort {
	case CoreSortFunc:
		return "func"
	case CoreSortTable:
		return "table"
	case CoreSortMemory:
		return "memory"
	case CoreSortGlobal:
		return "global"
	case CoreSortType:
		return "type"
	case CoreSortModule:
		return "module"
	case CoreSortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown-sort-%d", sort)
	}
}

func nestedComponentToWAT(nc *NestedComponent, level int) string {
	if nc.Component == nil {
		return "(component)"
	}
	var b strings.Builder
	b.WriteString("(component")
	b.WriteString("\n")

	for _, def := range nc.Component.Definitions {
		indent(&b, level+1)
		b.WriteString(defToWAT(def, level+1))
		b.WriteString("\n")
	}

	indent(&b, level)
	b.WriteString(")")
	return b.String()
}

func instanceToWAT(inst *Instance) string {
	var b strings.Builder
	b.WriteString("(instance")
	b.WriteString(" ")
	b.WriteString(instanceExprToWAT(inst.Expr))
	b.WriteString(")")
	return b.String()
}

func instanceExprToWAT(expr InstanceExpr) string {
	switch e := expr.(type) {
	case *Instantiate:
		return instantiateToWAT(e)
	case *InlineExports:
		return inlineExportsToWAT(e)
	default:
		return fmt.Sprintf("(; unknown instance expr: %T ;)", expr)
	}
}

func instantiateToWAT(inst *Instantiate) string {
	var b strings.Builder
	b.WriteString("(instantiate ")
	b.WriteString(fmt.Sprintf("%d", inst.ComponentIdx))
	for _, arg := range inst.Args {
		b.WriteString(" (with \"")
		b.WriteString(arg.Name)
		b.WriteString("\" ")
		b.WriteString(fmt.Sprintf("(%s %d)", sortToString(arg.SortIdx.Sort), arg.SortIdx.Idx))
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func inlineExportsToWAT(e *InlineExports) string {
	var b strings.Builder
	for _, export := range e.Exports {
		b.WriteString(inlineExportToWAT(&export))
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}

func inlineExportToWAT(e *InlineExport) string {
	return fmt.Sprintf("(export \"%s\" (%s %d))",
		e.Name,
		sortToString(e.SortIdx.Sort),
		e.SortIdx.Idx)
}

func sortToString(sort Sort) string {
	switch sort {
	case SortCoreFunc:
		return "core func"
	case SortCoreTable:
		return "core table"
	case SortCoreMemory:
		return "core memory"
	case SortCoreGlobal:
		return "core global"
	case SortCoreType:
		return "core type"
	case SortCoreModule:
		return "core module"
	case SortCoreInstance:
		return "core instance"
	case SortFunc:
		return "func"
	case SortType:
		return "type"
	case SortComponent:
		return "component"
	case SortInstance:
		return "instance"
	default:
		return fmt.Sprintf("unknown-sort-%d", sort)
	}
}

func aliasToWAT(a *Alias) string {
	var b strings.Builder
	b.WriteString("(alias ")
	b.WriteString(aliasTargetToWAT(a.Target))
	b.WriteString(" (")
	b.WriteString(sortToString(a.Sort))
	b.WriteString("))")
	return b.String()
}

func aliasTargetToWAT(target AliasTarget) string {
	switch t := target.(type) {
	case *ExportAlias:
		return fmt.Sprintf("export %d \"%s\"", t.InstanceIdx, t.Name)
	case *CoreExportAlias:
		return fmt.Sprintf("core export %d \"%s\"", t.InstanceIdx, t.Name)
	case *OuterAlias:
		return fmt.Sprintf("outer %d %d", t.Count, t.Idx)
	default:
		return fmt.Sprintf("(; unknown alias target: %T ;)", target)
	}
}

func typeToWAT(t *Type, level int) string {
	var b strings.Builder
	b.WriteString("(type")
	b.WriteString(" ")
	b.WriteString(defTypeToWAT(t.DefType, level))
	b.WriteString(")")
	return b.String()
}

func defTypeToWAT(dt DefType, level int) string {
	switch t := dt.(type) {
	case *BoolType:
		return "bool"
	case *S8Type:
		return "s8"
	case *U8Type:
		return "u8"
	case *S16Type:
		return "s16"
	case *U16Type:
		return "u16"
	case *S32Type:
		return "s32"
	case *U32Type:
		return "u32"
	case *S64Type:
		return "s64"
	case *U64Type:
		return "u64"
	case *F32Type:
		return "f32"
	case *F64Type:
		return "f64"
	case *CharType:
		return "char"
	case *StringType:
		return "string"
	case *RecordType:
		return recordTypeToWAT(t)
	case *VariantType:
		return variantTypeToWAT(t)
	case *ListType:
		return listTypeToWAT(t)
	case *TupleType:
		return tupleTypeToWAT(t)
	case *FlagsType:
		return flagsTypeToWAT(t)
	case *EnumType:
		return enumTypeToWAT(t)
	case *OptionType:
		return optionTypeToWAT(t)
	case *ResultType:
		return resultTypeToWAT(t)
	case *OwnType:
		return fmt.Sprintf("(own %d)", t.TypeIdx)
	case *BorrowType:
		return fmt.Sprintf("(borrow %d)", t.TypeIdx)
	case *ResourceType:
		return resourceTypeToWAT(t)
	case *FuncType:
		return funcTypeToWAT(t)
	case *ComponentType:
		return componentTypeToWAT(t, level)
	case *InstanceType:
		return instanceTypeToWAT(t, level)
	case *TypeIdx:
		return fmt.Sprintf("%d", t.Idx)
	default:
		return fmt.Sprintf("(; unknown def type: %T ;)", dt)
	}
}

func valTypeToWAT(vt DefValType) string {
	switch t := vt.(type) {
	case *BoolType:
		return "bool"
	case *S8Type:
		return "s8"
	case *U8Type:
		return "u8"
	case *S16Type:
		return "s16"
	case *U16Type:
		return "u16"
	case *S32Type:
		return "s32"
	case *U32Type:
		return "u32"
	case *S64Type:
		return "s64"
	case *U64Type:
		return "u64"
	case *F32Type:
		return "f32"
	case *F64Type:
		return "f64"
	case *CharType:
		return "char"
	case *StringType:
		return "string"
	case *RecordType:
		return recordTypeToWAT(t)
	case *VariantType:
		return variantTypeToWAT(t)
	case *ListType:
		return listTypeToWAT(t)
	case *TupleType:
		return tupleTypeToWAT(t)
	case *FlagsType:
		return flagsTypeToWAT(t)
	case *EnumType:
		return enumTypeToWAT(t)
	case *OptionType:
		return optionTypeToWAT(t)
	case *ResultType:
		return resultTypeToWAT(t)
	case *OwnType:
		return fmt.Sprintf("(own %d)", t.TypeIdx)
	case *BorrowType:
		return fmt.Sprintf("(borrow %d)", t.TypeIdx)
	case *TypeIdx:
		return fmt.Sprintf("%d", t.Idx)
	default:
		return fmt.Sprintf("(; unknown val type: %T ;)", vt)
	}
}

func recordTypeToWAT(rt *RecordType) string {
	var b strings.Builder
	b.WriteString("(record")
	for _, field := range rt.Fields {
		b.WriteString(" (field \"")
		b.WriteString(field.Label)
		b.WriteString("\" ")
		b.WriteString(valTypeToWAT(field.Type))
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func variantTypeToWAT(vt *VariantType) string {
	var b strings.Builder
	b.WriteString("(variant")
	for _, c := range vt.Cases {
		b.WriteString(" (case \"")
		b.WriteString(c.Label)
		b.WriteString("\"")
		if c.Type != nil {
			b.WriteString(" ")
			b.WriteString(valTypeToWAT(c.Type))
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func listTypeToWAT(lt *ListType) string {
	return fmt.Sprintf("(list %s)", valTypeToWAT(lt.Element))
}

func tupleTypeToWAT(tt *TupleType) string {
	var b strings.Builder
	b.WriteString("(tuple")
	for _, t := range tt.Types {
		b.WriteString(" ")
		b.WriteString(valTypeToWAT(t))
	}
	b.WriteString(")")
	return b.String()
}

func flagsTypeToWAT(ft *FlagsType) string {
	var b strings.Builder
	b.WriteString("(flags")
	for _, label := range ft.Labels {
		b.WriteString(" \"")
		b.WriteString(label)
		b.WriteString("\"")
	}
	b.WriteString(")")
	return b.String()
}

func enumTypeToWAT(et *EnumType) string {
	var b strings.Builder
	b.WriteString("(enum")
	for _, label := range et.Labels {
		b.WriteString(" \"")
		b.WriteString(label)
		b.WriteString("\"")
	}
	b.WriteString(")")
	return b.String()
}

func optionTypeToWAT(ot *OptionType) string {
	return fmt.Sprintf("(option %s)", valTypeToWAT(ot.Type))
}

func resultTypeToWAT(rt *ResultType) string {
	var b strings.Builder
	b.WriteString("(result")
	if rt.Ok != nil {
		b.WriteString(" (ok ")
		b.WriteString(valTypeToWAT(rt.Ok))
		b.WriteString(")")
	}
	if rt.Error != nil {
		b.WriteString(" (error ")
		b.WriteString(valTypeToWAT(rt.Error))
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func resourceTypeToWAT(rt *ResourceType) string {
	var b strings.Builder
	b.WriteString("(resource (rep ")
	b.WriteString(coreValTypeToWAT(rt.Rep))
	b.WriteString(")")
	if rt.Dtor != nil {
		b.WriteString(fmt.Sprintf(" (dtor %d)", *rt.Dtor))
	}
	b.WriteString(")")
	return b.String()
}

func funcTypeToWAT(ft *FuncType) string {
	var b strings.Builder
	b.WriteString("(func")
	for _, param := range ft.Params {
		b.WriteString(" (param \"")
		b.WriteString(param.Label)
		b.WriteString("\" ")
		b.WriteString(valTypeToWAT(param.Type))
		b.WriteString(")")
	}
	if ft.Results != nil {
		b.WriteString(" (result ")
		b.WriteString(valTypeToWAT(ft.Results))
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func componentTypeToWAT(ct *ComponentType, level int) string {
	var b strings.Builder
	b.WriteString("(component")
	for _, decl := range ct.Declarations {
		b.WriteString("\n")
		indent(&b, level+1)
		b.WriteString(componentDeclToWAT(decl, level+1))
	}
	if len(ct.Declarations) > 0 {
		b.WriteString("\n")
		indent(&b, level)
	}
	b.WriteString(")")
	return b.String()
}

func instanceTypeToWAT(it *InstanceType, level int) string {
	var b strings.Builder
	b.WriteString("(instance")
	for _, decl := range it.Declarations {
		b.WriteString("\n")
		indent(&b, level+1)
		b.WriteString(instanceDeclToWAT(decl, level+1))
	}
	if len(it.Declarations) > 0 {
		b.WriteString("\n")
		indent(&b, level)
	}
	b.WriteString(")")
	return b.String()
}

func componentDeclToWAT(decl ComponentDecl, level int) string {
	switch d := decl.(type) {
	case *TypeDecl:
		return typeToWAT(d.Type, level)
	case *CoreTypeDecl:
		return coreTypeToWAT(d.Type, level)
	case *AliasDecl:
		return aliasToWAT(d.Alias)
	case *ImportDecl:
		return importDeclToWAT(d, level)
	case *ExportDecl:
		return exportDeclToWAT(d, level)
	default:
		return fmt.Sprintf("(; unknown component decl: %T ;)", decl)
	}
}

func instanceDeclToWAT(decl InstanceDecl, level int) string {
	switch d := decl.(type) {
	case *TypeDecl:
		return typeToWAT(d.Type, level)
	case *CoreTypeDecl:
		return coreTypeToWAT(d.Type, level)
	case *AliasDecl:
		return aliasToWAT(d.Alias)
	case *ExportDecl:
		return exportDeclToWAT(d, level)
	default:
		return fmt.Sprintf("(; unknown instance decl: %T ;)", decl)
	}
}

func importDeclToWAT(id *ImportDecl, level int) string {
	var b strings.Builder
	b.WriteString("(import \"")
	b.WriteString(id.ImportName)
	b.WriteString("\" ")
	b.WriteString(externDescToWAT(id.Desc, level))
	b.WriteString(")")
	return b.String()
}

func exportDeclToWAT(ed *ExportDecl, level int) string {
	var b strings.Builder
	b.WriteString("(export \"")
	b.WriteString(ed.ExportName)
	b.WriteString("\" ")
	b.WriteString(externDescToWAT(ed.Desc, level))
	b.WriteString(")")
	return b.String()
}

func externDescToWAT(desc ExternDesc, level int) string {
	switch d := desc.(type) {
	case *SortExternDesc:
		var b strings.Builder
		b.WriteString("(")
		b.WriteString(sortToString(d.Sort))
		b.WriteString(fmt.Sprintf(" (type %d)", d.TypeIdx))
		b.WriteString(")")
		return b.String()
	case *TypeExternDesc:
		var b strings.Builder
		b.WriteString("(type")
		b.WriteString(" ")
		b.WriteString(typeBoundToWAT(d.Bound))
		b.WriteString(")")
		return b.String()
	default:
		return fmt.Sprintf("(; unknown extern desc: %T ;)", desc)
	}
}

func typeBoundToWAT(bound TypeBound) string {
	switch b := bound.(type) {
	case *EqBound:
		return fmt.Sprintf("(eq %d)", b.TypeIdx)
	case *SubResourceBound:
		return "(sub resource)"
	default:
		return fmt.Sprintf("(; unknown type bound: %T ;)", bound)
	}
}

func importToWAT(imp *Import, level int) string {
	var b strings.Builder
	b.WriteString("(import \"")
	b.WriteString(imp.ImportName)
	b.WriteString("\" ")
	b.WriteString(externDescToWAT(imp.Desc, level))
	b.WriteString(")")
	return b.String()
}

func exportToWAT(exp *Export) string {
	return fmt.Sprintf("(export \"%s\" (%s %d))",
		exp.ExportName,
		sortToString(exp.SortIdx.Sort),
		exp.SortIdx.Idx)
}

func canonToWAT(c *Canon) string {
	return canonDefToWAT(c.Def)
}

func canonDefToWAT(def CanonDef) string {
	switch d := def.(type) {
	case *CanonLift:
		var b strings.Builder
		b.WriteString("(canon lift (core func ")
		b.WriteString(fmt.Sprintf("%d)", d.CoreFuncIdx))
		for _, opt := range d.Options {
			b.WriteString(" ")
			b.WriteString(canonOptToWAT(opt))
		}
		b.WriteString(")")
		return b.String()
	case *CanonLower:
		var b strings.Builder
		b.WriteString("(canon lower (func ")
		b.WriteString(fmt.Sprintf("%d)", d.FuncIdx))
		for _, opt := range d.Options {
			b.WriteString(" ")
			b.WriteString(canonOptToWAT(opt))
		}
		b.WriteString(")")
		return b.String()
	case *CanonResourceNew:
		var b strings.Builder
		b.WriteString("(canon resource.new ")
		b.WriteString(fmt.Sprintf("%d", d.TypeIdx))
		b.WriteString(")")
		return b.String()
	case *CanonResourceDrop:
		var b strings.Builder
		b.WriteString("(canon resource.drop ")
		b.WriteString(fmt.Sprintf("%d", d.TypeIdx))
		b.WriteString(")")
		return b.String()
	case *CanonResourceRep:
		var b strings.Builder
		b.WriteString("(canon resource.rep ")
		b.WriteString(fmt.Sprintf("%d", d.TypeIdx))
		b.WriteString(")")
		return b.String()
	default:
		return fmt.Sprintf("(; unknown canon def: %T ;)", def)
	}
}

func canonOptToWAT(opt CanonOpt) string {
	switch o := opt.(type) {
	case *StringEncodingOpt:
		var enc string
		switch o.Encoding {
		case StringEncodingUTF8:
			enc = "utf8"
		case StringEncodingUTF16:
			enc = "utf16"
		case StringEncodingLatin1UTF16:
			enc = "latin1+utf16"
		default:
			enc = fmt.Sprintf("unknown-%d", o.Encoding)
		}
		return fmt.Sprintf("(string-encoding=%s)", enc)
	case *MemoryOpt:
		return fmt.Sprintf("(memory %d)", o.MemoryIdx)
	case *ReallocOpt:
		return fmt.Sprintf("(realloc %d)", o.FuncIdx)
	case *PostReturnOpt:
		return fmt.Sprintf("(post-return %d)", o.FuncIdx)
	default:
		return fmt.Sprintf("(; unknown canon opt: %T ;)", opt)
	}
}

func coreTypeToWAT(ct *CoreType, level int) string {
	var b strings.Builder
	b.WriteString("(core type")
	b.WriteString(" ")
	b.WriteString(coreDefTypeToWAT(ct.DefType, level))
	b.WriteString(")")
	return b.String()
}

func coreDefTypeToWAT(dt CoreDefType, level int) string {
	switch t := dt.(type) {
	case *CoreRecType:
		return coreRecTypeToWAT(t, level)
	case *CoreModuleType:
		return coreModuleTypeToWAT(t, level)
	default:
		return fmt.Sprintf("(; unknown core def type: %T ;)", dt)
	}
}

func coreRecTypeToWAT(rt *CoreRecType, level int) string {
	var b strings.Builder
	if len(rt.SubTypes) == 1 {
		// Single subtype can be written directly
		b.WriteString(coreSubTypeToWAT(&rt.SubTypes[0], level))
	} else {
		// Multiple subtypes need (rec ...)
		b.WriteString("(rec")
		for _, st := range rt.SubTypes {
			b.WriteString("\n")
			indent(&b, level+1)
			b.WriteString(coreSubTypeToWAT(&st, level+1))
		}
		if len(rt.SubTypes) > 0 {
			b.WriteString("\n")
			indent(&b, level)
		}
		b.WriteString(")")
	}
	return b.String()
}

func coreSubTypeToWAT(st *CoreSubType, level int) string {
	var b strings.Builder
	if st.Final || len(st.Supertypes) > 0 {
		b.WriteString("(sub ")
		if st.Final {
			b.WriteString("final ")
		}
		for _, idx := range st.Supertypes {
			b.WriteString(fmt.Sprintf("%d ", idx))
		}
		b.WriteString(coreCompTypeToWAT(st.Type, level))
		b.WriteString(")")
	} else {
		b.WriteString(coreCompTypeToWAT(st.Type, level))
	}
	return b.String()
}

func coreCompTypeToWAT(ct CoreCompType, level int) string {
	switch t := ct.(type) {
	case *CoreFuncType:
		return coreFuncTypeToWAT(t)
	case *CoreStructType:
		return coreStructTypeToWAT(t)
	case *CoreArrayType:
		return coreArrayTypeToWAT(t)
	case *CoreModuleType:
		return coreModuleTypeToWAT(t, level)
	default:
		return fmt.Sprintf("(; unknown core comp type: %T ;)", ct)
	}
}

func coreFuncTypeToWAT(ft *CoreFuncType) string {
	var b strings.Builder
	b.WriteString("(func")
	if len(ft.Params.Types) > 0 {
		b.WriteString(" (param")
		for _, t := range ft.Params.Types {
			b.WriteString(" ")
			b.WriteString(coreValTypeToWAT(t))
		}
		b.WriteString(")")
	}
	if len(ft.Results.Types) > 0 {
		b.WriteString(" (result")
		for _, t := range ft.Results.Types {
			b.WriteString(" ")
			b.WriteString(coreValTypeToWAT(t))
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func coreStructTypeToWAT(st *CoreStructType) string {
	var b strings.Builder
	b.WriteString("(struct")
	for _, field := range st.Fields {
		b.WriteString(" (field ")
		if field.Mutable {
			b.WriteString("(mut ")
		}
		b.WriteString(coreStorageTypeToWAT(field.Type))
		if field.Mutable {
			b.WriteString(")")
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func coreArrayTypeToWAT(at *CoreArrayType) string {
	var b strings.Builder
	b.WriteString("(array ")
	if at.Field.Mutable {
		b.WriteString("(mut ")
	}
	b.WriteString(coreStorageTypeToWAT(at.Field.Type))
	if at.Field.Mutable {
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

func coreModuleTypeToWAT(mt *CoreModuleType, level int) string {
	var b strings.Builder
	b.WriteString("(module")
	for _, decl := range mt.Declarations {
		b.WriteString("\n")
		indent(&b, level+1)
		b.WriteString(coreModuleDeclToWAT(decl, level+1))
	}
	if len(mt.Declarations) > 0 {
		b.WriteString("\n")
		indent(&b, level)
	}
	b.WriteString(")")
	return b.String()
}

func coreModuleDeclToWAT(decl CoreModuleDecl, level int) string {
	switch d := decl.(type) {
	case *CoreImportDecl:
		return coreImportDeclToWAT(d)
	case *CoreExportDecl:
		return coreExportDeclToWAT(d)
	case *CoreAliasDecl:
		return coreAliasDeclToWAT(d)
	case *CoreTypeDecl:
		return coreTypeToWAT(d.Type, level)
	default:
		return fmt.Sprintf("(; unknown core module decl: %T ;)", decl)
	}
}

func coreImportDeclToWAT(id *CoreImportDecl) string {
	var b strings.Builder
	b.WriteString("(import \"")
	b.WriteString(id.Module)
	b.WriteString("\" \"")
	b.WriteString(id.Name)
	b.WriteString("\" ")
	b.WriteString(coreImportDescToWAT(id.Desc))
	b.WriteString(")")
	return b.String()
}

func coreImportDescToWAT(desc CoreImportDesc) string {
	switch d := desc.(type) {
	case *CoreFuncImport:
		var b strings.Builder
		b.WriteString("(func")
		b.WriteString(fmt.Sprintf(" (type %d)", d.TypeIdx))
		b.WriteString(")")
		return b.String()
	case *CoreTableImport:
		var b strings.Builder
		b.WriteString("(table")
		b.WriteString(" ")
		b.WriteString(coreTableTypeToWAT(&d.Type))
		b.WriteString(")")
		return b.String()
	case *CoreMemoryImport:
		var b strings.Builder
		b.WriteString("(memory")
		b.WriteString(" ")
		b.WriteString(coreMemTypeToWAT(&d.Type))
		b.WriteString(")")
		return b.String()
	case *CoreGlobalImport:
		var b strings.Builder
		b.WriteString("(global")
		b.WriteString(" ")
		b.WriteString(coreGlobalTypeToWAT(&d.Type))
		b.WriteString(")")
		return b.String()
	case *CoreTagImport:
		var b strings.Builder
		b.WriteString("(tag")
		b.WriteString(fmt.Sprintf(" (type %d)", d.Type.TypeIdx))
		b.WriteString(")")
		return b.String()
	default:
		return fmt.Sprintf("(; unknown core import desc: %T ;)", desc)
	}
}

func coreExportDeclToWAT(ed *CoreExportDecl) string {
	var b strings.Builder
	b.WriteString("(export \"")
	b.WriteString(ed.Name)
	b.WriteString("\" ")
	b.WriteString(coreImportDescToWAT(ed.Desc))
	b.WriteString(")")
	return b.String()
}

func coreAliasDeclToWAT(ad *CoreAliasDecl) string {
	var b strings.Builder
	b.WriteString("(alias ")
	b.WriteString(coreAliasTargetToWAT(ad.Target))
	b.WriteString(" (")
	b.WriteString(coreSortToString(ad.Sort))
	b.WriteString("))")
	return b.String()
}

func coreAliasTargetToWAT(target CoreAliasTarget) string {
	switch t := target.(type) {
	case *CoreOuterAlias:
		return fmt.Sprintf("outer %d %d", t.Count, t.Idx)
	default:
		return fmt.Sprintf("(; unknown core alias target: %T ;)", target)
	}
}

func coreValTypeToWAT(vt CoreValType) string {
	switch t := vt.(type) {
	case CoreNumType:
		switch t {
		case CoreNumTypeI32:
			return "i32"
		case CoreNumTypeI64:
			return "i64"
		case CoreNumTypeF32:
			return "f32"
		case CoreNumTypeF64:
			return "f64"
		default:
			return fmt.Sprintf("unknown-num-%d", t)
		}
	case CoreVecType:
		switch t {
		case CoreVecTypeV128:
			return "v128"
		default:
			return fmt.Sprintf("unknown-vec-%d", t)
		}
	case *CoreRefType:
		return coreRefTypeToWAT(t)
	default:
		return fmt.Sprintf("(; unknown core val type: %T ;)", vt)
	}
}

func coreRefTypeToWAT(rt *CoreRefType) string {
	var b strings.Builder
	b.WriteString("(ref ")
	if rt.Nullable {
		b.WriteString("null ")
	}
	b.WriteString(coreHeapTypeToWAT(rt.HeapType))
	b.WriteString(")")
	return b.String()
}

func coreHeapTypeToWAT(ht CoreHeapType) string {
	switch t := ht.(type) {
	case CoreAbsHeapType:
		switch t {
		case CoreAbsHeapTypeFunc:
			return "func"
		case CoreAbsHeapTypeNoFunc:
			return "nofunc"
		case CoreAbsHeapTypeExtern:
			return "extern"
		case CoreAbsHeapTypeNoExtern:
			return "noextern"
		case CoreAbsHeapTypeAny:
			return "any"
		case CoreAbsHeapTypeEq:
			return "eq"
		case CoreAbsHeapTypeI31:
			return "i31"
		case CoreAbsHeapTypeStruct:
			return "struct"
		case CoreAbsHeapTypeArray:
			return "array"
		case CoreAbsHeapTypeNone:
			return "none"
		case CoreAbsHeapTypeExn:
			return "exn"
		case CoreAbsHeapTypeNoExn:
			return "noexn"
		default:
			return fmt.Sprintf("unknown-abs-heap-%d", t)
		}
	case *CoreConcreteHeapType:
		return fmt.Sprintf("%d", t.TypeIdx)
	default:
		return fmt.Sprintf("(; unknown core heap type: %T ;)", ht)
	}
}

func coreStorageTypeToWAT(st CoreStorageType) string {
	switch t := st.(type) {
	case CoreNumType, CoreVecType, *CoreRefType:
		return coreValTypeToWAT(t.(CoreValType))
	case CorePackedType:
		switch t {
		case CorePackedTypeI8:
			return "i8"
		case CorePackedTypeI16:
			return "i16"
		default:
			return fmt.Sprintf("unknown-packed-%d", t)
		}
	default:
		return fmt.Sprintf("(; unknown core storage type: %T ;)", st)
	}
}

func coreTableTypeToWAT(tt *CoreTableType) string {
	return fmt.Sprintf("%d %s %s",
		tt.Limits.Min,
		limitMaxToWAT(tt.Limits.Max),
		coreRefTypeToWAT(tt.ElemType))
}

func coreMemTypeToWAT(mt *CoreMemType) string {
	return fmt.Sprintf("%d%s", mt.Limits.Min, limitMaxToWAT(mt.Limits.Max))
}

func limitMaxToWAT(max *uint32) string {
	if max == nil {
		return ""
	}
	return fmt.Sprintf(" %d", *max)
}

func coreGlobalTypeToWAT(gt *CoreGlobalType) string {
	if gt.Mut {
		return fmt.Sprintf("(mut %s)", coreValTypeToWAT(gt.Val))
	}
	return coreValTypeToWAT(gt.Val)
}
