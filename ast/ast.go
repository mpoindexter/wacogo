package ast

import "fmt"

// Component represents the top-level component structure
type Component struct {
	Definitions []Definition
}

// Definition is the interface for all component-level definitions
type Definition interface {
	isDefinition()
}

// CoreModule represents a core WebAssembly module
type CoreModule struct {
	Raw []byte // Raw bytes of the module
}

func (*CoreModule) isDefinition() {}

// CoreInstance represents a core module instance
type CoreInstance struct {
	Expr CoreInstanceExpr
}

func (*CoreInstance) isDefinition() {}

// CoreInstanceExpr defines how a core instance is created
type CoreInstanceExpr interface {
	isCoreInstanceExpr()
}

// CoreInstantiate creates an instance by instantiating a module
type CoreInstantiate struct {
	ModuleIdx uint32
	Args      []CoreInstantiateArg
}

func (*CoreInstantiate) isCoreInstanceExpr() {}

// CoreInlineExports creates an instance from direct exports
type CoreInlineExports struct {
	Exports []CoreInlineExport
}

func (*CoreInlineExports) isCoreInstanceExpr() {}

// CoreInstantiateArg provides arguments for instantiation
type CoreInstantiateArg struct {
	Name            string
	CoreInstanceIdx uint32
}

// CoreSort represents the different kinds of core definitions
type CoreSort int

const (
	CoreSortFunc CoreSort = iota
	CoreSortTable
	CoreSortMemory
	CoreSortGlobal
	CoreSortType
	CoreSortModule
	CoreSortInstance
)

// CoreSortIdx references a definition by sort and index
type CoreSortIdx struct {
	Sort CoreSort
	Idx  uint32
}

// CoreInlineExport represents an inline export definition
type CoreInlineExport struct {
	Name    string
	SortIdx CoreSortIdx
}

// NestedComponent represents a nested component definition
type NestedComponent struct {
	Component *Component
}

func (*NestedComponent) isDefinition() {}

// Instance represents a component instance
type Instance struct {
	Expr InstanceExpr
}

func (*Instance) isDefinition() {}

// InstanceExpr defines how a component instance is created
type InstanceExpr interface {
	isInstanceExpr()
}

// Instantiate creates an instance by instantiating a component
type Instantiate struct {
	ComponentIdx uint32
	Args         []InstantiateArg
}

func (*Instantiate) isInstanceExpr() {}

// InlineExports creates an instance from direct exports
type InlineExports struct {
	Exports []InlineExport
}

func (*InlineExports) isInstanceExpr() {}

// InstantiateArg provides arguments for component instantiation
type InstantiateArg struct {
	Name    string
	SortIdx *SortIdx
}

// SortIdx references a component-level definition
type SortIdx struct {
	Sort Sort
	Idx  uint32
}

// Sort represents the different kinds of component-level definitions
type Sort int

const (
	// Core sorts (injected from CoreSort)
	SortCoreFunc Sort = iota
	SortCoreTable
	SortCoreMemory
	SortCoreGlobal
	SortCoreType
	SortCoreModule
	SortCoreInstance
	// Component sorts
	SortFunc
	SortType
	SortComponent
	SortInstance
)

func (s Sort) String() string {
	switch s {
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
		return fmt.Sprintf("unknown - %v", int(s))
	}
}

// InlineExport represents an inline export definition
type InlineExport struct {
	Name    string
	SortIdx SortIdx
}

// Alias projects definitions from other components
type Alias struct {
	Target AliasTarget
	Sort   Sort
}

func (*Alias) isDefinition() {}

// AliasTarget defines where the alias comes from
type AliasTarget interface {
	isAliasTarget()
}

// ExportAlias aliases an export from an instance
type ExportAlias struct {
	InstanceIdx uint32
	Name        string
}

func (*ExportAlias) isAliasTarget() {}

// CoreExportAlias aliases a core export from a core instance
type CoreExportAlias struct {
	InstanceIdx uint32
	Name        string
}

func (*CoreExportAlias) isAliasTarget() {}

// OuterAlias aliases a definition from an outer component
type OuterAlias struct {
	Count uint32 // Number of enclosing components to skip
	Idx   uint32 // Index in the target's index space
}

func (*OuterAlias) isAliasTarget() {}

// Type represents a type definition
type Type struct {
	DefType DefType
}

func (*Type) isDefinition() {}

// DefType is the interface for type definitions
type DefType interface {
	isDefType()
}

// DefValType represents value types that can be defined
type DefValType interface {
	DefType
	isValType()
}

// TypeIdx references a type by index
type TypeIdx struct {
	Idx uint32
}

func (*TypeIdx) isValType() {}
func (*TypeIdx) isDefType() {}

// Primitive value types
type BoolType struct{}
type S8Type struct{}
type U8Type struct{}
type S16Type struct{}
type U16Type struct{}
type S32Type struct{}
type U32Type struct{}
type S64Type struct{}
type U64Type struct{}
type F32Type struct{}
type F64Type struct{}
type CharType struct{}
type StringType struct{}

func (*BoolType) isValType()   {}
func (*BoolType) isDefType()   {}
func (*S8Type) isValType()     {}
func (*S8Type) isDefType()     {}
func (*U8Type) isValType()     {}
func (*U8Type) isDefType()     {}
func (*S16Type) isValType()    {}
func (*S16Type) isDefType()    {}
func (*U16Type) isValType()    {}
func (*U16Type) isDefType()    {}
func (*S32Type) isValType()    {}
func (*S32Type) isDefType()    {}
func (*U32Type) isValType()    {}
func (*U32Type) isDefType()    {}
func (*S64Type) isValType()    {}
func (*S64Type) isDefType()    {}
func (*U64Type) isValType()    {}
func (*U64Type) isDefType()    {}
func (*F32Type) isValType()    {}
func (*F32Type) isDefType()    {}
func (*F64Type) isValType()    {}
func (*F64Type) isDefType()    {}
func (*CharType) isValType()   {}
func (*CharType) isDefType()   {}
func (*StringType) isValType() {}
func (*StringType) isDefType() {}

// RecordType represents a record (struct-like) type
type RecordType struct {
	Fields []RecordField
}

func (*RecordType) isValType() {}
func (*RecordType) isDefType() {}

// RecordField represents a field in a record
type RecordField struct {
	Label string
	Type  DefValType
}

// VariantType represents a variant (tagged union) type
type VariantType struct {
	Cases []VariantCase
}

func (*VariantType) isValType() {}
func (*VariantType) isDefType() {}

// VariantCase represents a case in a variant
type VariantCase struct {
	Label string
	Type  DefValType // nil for cases without payload
}

// ListType represents a list type
type ListType struct {
	Element DefValType
}

func (*ListType) isValType() {}
func (*ListType) isDefType() {}

// TupleType represents a tuple type
type TupleType struct {
	Types []DefValType
}

func (*TupleType) isValType() {}
func (*TupleType) isDefType() {}

// FlagsType represents a flags type
type FlagsType struct {
	Labels []string
}

func (*FlagsType) isValType() {}
func (*FlagsType) isDefType() {}

// EnumType represents an enum type
type EnumType struct {
	Labels []string
}

func (*EnumType) isValType() {}
func (*EnumType) isDefType() {}

// OptionType represents an optional type
type OptionType struct {
	Type DefValType
}

func (*OptionType) isValType() {}
func (*OptionType) isDefType() {}

// ResultType represents a result type (for error handling)
type ResultType struct {
	Ok    DefValType // nil if no ok value
	Error DefValType // nil if no error value
}

func (*ResultType) isValType() {}
func (*ResultType) isDefType() {}

// OwnType represents an owned handle to a resource
type OwnType struct {
	TypeIdx uint32
}

func (*OwnType) isValType() {}
func (*OwnType) isDefType() {}

// BorrowType represents a borrowed handle to a resource
type BorrowType struct {
	TypeIdx uint32
}

func (*BorrowType) isValType() {}
func (*BorrowType) isDefType() {}

// ResourceType represents a resource type definition
type ResourceType struct {
	Rep  CoreValType // Representation type (currently always i32)
	Dtor *uint32     // Optional destructor function index
}

func (*ResourceType) isDefType() {}

// CoreNumType represents core WebAssembly number types
type CoreNumType int

const (
	CoreNumTypeI32 CoreNumType = iota
	CoreNumTypeI64
	CoreNumTypeF32
	CoreNumTypeF64
)

// CoreVecType represents core WebAssembly vector types (SIMD)
type CoreVecType int

const (
	CoreVecTypeV128 CoreVecType = iota
)

// CoreHeapType represents core WebAssembly heap types
type CoreHeapType interface {
	isCoreHeapType()
}

// CoreAbsHeapType represents abstract heap types
type CoreAbsHeapType int

const (
	CoreAbsHeapTypeFunc CoreAbsHeapType = iota
	CoreAbsHeapTypeNoFunc
	CoreAbsHeapTypeExtern
	CoreAbsHeapTypeNoExtern
	CoreAbsHeapTypeAny
	CoreAbsHeapTypeEq
	CoreAbsHeapTypeI31
	CoreAbsHeapTypeStruct
	CoreAbsHeapTypeArray
	CoreAbsHeapTypeNone
	CoreAbsHeapTypeExn
	CoreAbsHeapTypeNoExn
)

func (CoreAbsHeapType) isCoreHeapType() {}

// CoreConcreteHeapType represents a concrete heap type (type index)
type CoreConcreteHeapType struct {
	TypeIdx uint32
}

func (*CoreConcreteHeapType) isCoreHeapType() {}

// CoreRefType represents core WebAssembly reference types
type CoreRefType struct {
	Nullable bool
	HeapType CoreHeapType
}

// CoreValType represents core WebAssembly value types
type CoreValType interface {
	isCoreValType()
}

func (CoreNumType) isCoreValType()  {}
func (CoreVecType) isCoreValType()  {}
func (*CoreRefType) isCoreValType() {}

// FuncType represents a component function type
type FuncType struct {
	Params  []FuncParam
	Results DefValType // nil if no result
}

func (*FuncType) isDefType() {}

// FuncParam represents a function parameter
type FuncParam struct {
	Label string
	Type  DefValType
}

// ComponentType represents a component type definition
type ComponentType struct {
	Declarations []ComponentDecl
}

func (*ComponentType) isDefType() {}

// ComponentDecl is the interface for component declarations
type ComponentDecl interface {
	isComponentDecl()
}

// InstanceType represents an instance type definition
type InstanceType struct {
	Declarations []InstanceDecl
}

func (*InstanceType) isDefType() {}

// InstanceDecl is the interface for instance declarations
type InstanceDecl interface {
	isInstanceDecl()
}

// TypeDecl represents a type declaration
type TypeDecl struct {
	Type *Type
}

func (*TypeDecl) isInstanceDecl()  {}
func (*TypeDecl) isComponentDecl() {}

// CoreTypeDecl represents a core type declaration
type CoreTypeDecl struct {
	Type *CoreType
}

func (*CoreTypeDecl) isInstanceDecl()   {}
func (*CoreTypeDecl) isComponentDecl()  {}
func (*CoreTypeDecl) isCoreModuleDecl() {}

// CoreType represents a core WebAssembly type
type CoreType struct {
	DefType CoreDefType
}

func (*CoreType) isDefinition() {}

// CoreResultType represents a sequence of value types
type CoreResultType struct {
	Types []CoreValType
}

// CoreFuncType represents a core function type
type CoreFuncType struct {
	Params  CoreResultType
	Results CoreResultType
}

// CoreStructType represents a structure type (GC proposal)
type CoreStructType struct {
	Fields []CoreFieldType
}

// CoreArrayType represents an array type (GC proposal)
type CoreArrayType struct {
	Field CoreFieldType
}

// CoreFieldType represents a field in a structure or array
type CoreFieldType struct {
	Mutable bool
	Type    CoreStorageType
}

// CoreStorageType represents a storage type for fields
type CoreStorageType interface {
	isCoreStorageType()
}

// CorePackedType represents packed types for fields
type CorePackedType int

const (
	CorePackedTypeI8 CorePackedType = iota
	CorePackedTypeI16
)

func (CoreNumType) isCoreStorageType()    {}
func (CoreVecType) isCoreStorageType()    {}
func (*CoreRefType) isCoreStorageType()   {}
func (CorePackedType) isCoreStorageType() {}

// CoreCompType represents composite types
type CoreCompType interface {
	isCoreCompType()
}

func (*CoreFuncType) isCoreCompType()   {}
func (*CoreStructType) isCoreCompType() {}
func (*CoreArrayType) isCoreCompType()  {}
func (*CoreModuleType) isCoreCompType() {}

// CoreSubType represents a subtype with optional supertypes
type CoreSubType struct {
	Final      bool
	Supertypes []uint32 // Type indices
	Type       CoreCompType
}

// CoreRecType represents a recursive type group
type CoreRecType struct {
	SubTypes []CoreSubType
}

// CoreDefType is the interface for core type definitions
type CoreDefType interface {
	isCoreDefType()
}

func (*CoreRecType) isCoreDefType() {}

// CoreModuleType represents a module type
type CoreModuleType struct {
	Declarations []CoreModuleDecl
}

func (*CoreModuleType) isCoreDefType() {}

// CoreModuleDecl is the interface for module declarations
type CoreModuleDecl interface {
	isCoreModuleDecl()
}

// CoreImportDecl represents an import declaration in a module type
type CoreImportDecl struct {
	Module string
	Name   string
	Desc   CoreImportDesc
}

func (*CoreImportDecl) isCoreModuleDecl() {}

// CoreImportDesc describes what is being imported
type CoreImportDesc interface {
	isCoreImportDesc()
}

// CoreFuncImport represents a function import
type CoreFuncImport struct {
	TypeIdx uint32
}

func (*CoreFuncImport) isCoreImportDesc() {}

// CoreTableImport represents a table import
type CoreTableImport struct {
	Type CoreTableType
}

func (*CoreTableImport) isCoreImportDesc() {}

// CoreMemoryImport represents a memory import
type CoreMemoryImport struct {
	Type CoreMemType
}

func (*CoreMemoryImport) isCoreImportDesc() {}

// CoreGlobalImport represents a global import
type CoreGlobalImport struct {
	Type CoreGlobalType
}

func (*CoreGlobalImport) isCoreImportDesc() {}

// CoreTagImport represents a tag import
type CoreTagImport struct {
	Type CoreTagType
}

func (*CoreTagImport) isCoreImportDesc() {}

// CoreExportDecl represents an export declaration in a module type
type CoreExportDecl struct {
	Name string
	Desc CoreImportDesc
}

func (*CoreExportDecl) isCoreModuleDecl() {}

// CoreLimits represents size constraints for memories and tables
type CoreLimits struct {
	Min uint32
	Max *uint32 // nil means no maximum
}

// CoreMut represents mutability
type CoreMut bool

const (
	CoreConst CoreMut = false
	CoreVar   CoreMut = true
)

// CoreGlobalType represents a global variable type
type CoreGlobalType struct {
	Mut CoreMut
	Val CoreValType
}

// CoreMemType represents a memory type
type CoreMemType struct {
	Limits CoreLimits
}

// CoreTableType represents a table type
type CoreTableType struct {
	Limits   CoreLimits
	ElemType *CoreRefType
}

// CoreTagType represents a tag (exception) type
type CoreTagType struct {
	TypeIdx uint32 // References a function type
}

// CoreAliasDecl represents an alias in a module type
type CoreAliasDecl struct {
	Target CoreAliasTarget
	Sort   CoreSort
}

func (*CoreAliasDecl) isCoreModuleDecl() {}

// CoreAliasTarget defines where the core alias comes from
type CoreAliasTarget interface {
	isCoreAliasTarget()
}

// CoreOuterAlias aliases a definition from an outer component/module
type CoreOuterAlias struct {
	Count uint32
	Idx   uint32
}

func (*CoreOuterAlias) isCoreAliasTarget() {}

// AliasDecl represents an alias declaration
type AliasDecl struct {
	Alias *Alias
}

func (*AliasDecl) isInstanceDecl()  {}
func (*AliasDecl) isComponentDecl() {}

// ImportDecl represents an import declaration
type ImportDecl struct {
	ImportName string
	Desc       ExternDesc
}

func (*ImportDecl) isComponentDecl() {}

// ExportDecl represents an export declaration
type ExportDecl struct {
	ExportName string
	Desc       ExternDesc
}

func (*ExportDecl) isInstanceDecl()  {}
func (*ExportDecl) isComponentDecl() {}

// ExternDesc describes an import or export
type ExternDesc interface {
	isExternDesc()
}

// SortExternDesc references a definition by sort
type SortExternDesc struct {
	Sort    Sort
	TypeIdx uint32
}

func (*SortExternDesc) isExternDesc() {}

// TypeExternDesc describes a type import/export
type TypeExternDesc struct {
	Bound TypeBound
}

func (*TypeExternDesc) isExternDesc() {}

// TypeBound defines the bound for a type import/export
type TypeBound interface {
	isTypeBound()
}

// EqBound indicates structural equality with a type
type EqBound struct {
	TypeIdx uint32
}

func (*EqBound) isTypeBound() {}

// SubResourceBound indicates subtype of resource
type SubResourceBound struct{}

func (*SubResourceBound) isTypeBound() {}

// Import represents a component import
type Import struct {
	ImportName string
	Desc       ExternDesc
}

func (*Import) isDefinition() {}

// Export represents a component export
type Export struct {
	ExportName string
	SortIdx    SortIdx
	ExternDesc ExternDesc
}

func (*Export) isDefinition() {}

// Canon represents a canonical definition
type Canon struct {
	Def CanonDef
}

func (*Canon) isDefinition() {}

// CanonDef is the interface for canonical definitions
type CanonDef interface {
	isCanonDef()
}

// CanonLift wraps a core function to produce a component function
type CanonLift struct {
	CoreFuncIdx     uint32
	Options         []CanonOpt
	FunctionTypeIdx uint32
}

func (*CanonLift) isCanonDef() {}

// CanonLower wraps a component function to produce a core function
type CanonLower struct {
	FuncIdx uint32
	Options []CanonOpt
}

func (*CanonLower) isCanonDef() {}

// CanonOpt represents canonical ABI options
type CanonOpt interface {
	isCanonOpt()
}

// StringEncodingOpt specifies string encoding
type StringEncodingOpt struct {
	Encoding StringEncoding
}

func (*StringEncodingOpt) isCanonOpt() {}

// StringEncoding represents different string encodings
type StringEncoding int

const (
	StringEncodingUTF8 StringEncoding = iota
	StringEncodingUTF16
	StringEncodingLatin1UTF16
)

// MemoryOpt specifies the memory to use
type MemoryOpt struct {
	MemoryIdx uint32
}

func (*MemoryOpt) isCanonOpt() {}

// ReallocOpt specifies the realloc function
type ReallocOpt struct {
	FuncIdx uint32
}

func (*ReallocOpt) isCanonOpt() {}

// PostReturnOpt specifies the post-return function
type PostReturnOpt struct {
	FuncIdx uint32
}

func (*PostReturnOpt) isCanonOpt() {}

// CanonResourceNew creates a new resource
type CanonResourceNew struct {
	TypeIdx uint32
}

func (*CanonResourceNew) isCanonDef() {}

// CanonResourceDrop drops a resource handle
type CanonResourceDrop struct {
	TypeIdx uint32
}

func (*CanonResourceDrop) isCanonDef() {}

// CanonResourceRep gets the representation of a resource
type CanonResourceRep struct {
	TypeIdx uint32
}

func (*CanonResourceRep) isCanonDef() {}

// Core WebAssembly Module Sections

// CoreImport represents a core module import
type CoreImport struct {
	Module string
	Name   string
	Desc   CoreImportDesc
}

// CoreFunc represents a function definition in a module
type CoreFunc struct {
	TypeIdx uint32
}

// CoreTable represents a table definition
type CoreTable struct {
	Type CoreTableType
	Init CoreExpr // Initializer expression
}

// CoreMem represents a memory definition
type CoreMem struct {
	Type CoreMemType
}

// CoreGlobal represents a global variable definition
type CoreGlobal struct {
	Type CoreGlobalType
	Init CoreExpr // Initializer expression
}

// CoreTag represents a tag (exception) definition
type CoreTag struct {
	Type CoreTagType
}

// CoreStart represents a start function
type CoreStart struct {
	FuncIdx uint32
}

// CoreElem represents an element segment
type CoreElem struct {
	Type CoreRefType
	Init []CoreExpr // Element initializer expressions
	Mode CoreElemMode
}

// CoreElemMode represents element segment modes
type CoreElemMode interface {
	isCoreElemMode()
}

// CoreElemModePassive represents a passive element segment
type CoreElemModePassive struct{}

func (*CoreElemModePassive) isCoreElemMode() {}

// CoreElemModeActive represents an active element segment
type CoreElemModeActive struct {
	TableIdx uint32
	Offset   CoreExpr
}

func (*CoreElemModeActive) isCoreElemMode() {}

// CoreElemModeDeclarative represents a declarative element segment
type CoreElemModeDeclarative struct{}

func (*CoreElemModeDeclarative) isCoreElemMode() {}

// CoreData represents a data segment
type CoreData struct {
	Init []byte
	Mode CoreDataMode
}

// CoreDataMode represents data segment modes
type CoreDataMode interface {
	isCoreDataMode()
}

// CoreDataModePassive represents a passive data segment
type CoreDataModePassive struct{}

func (*CoreDataModePassive) isCoreDataMode() {}

// CoreDataModeActive represents an active data segment
type CoreDataModeActive struct {
	MemIdx uint32
	Offset CoreExpr
}

func (*CoreDataModeActive) isCoreDataMode() {}

// CoreCode represents the code (locals and body) of a function
type CoreCode struct {
	Locals []CoreLocals
	Body   CoreExpr
}

// CoreLocals represents local variable declarations
type CoreLocals struct {
	Count uint32
	Type  CoreValType
}

// CoreExpr represents a core WebAssembly expression (sequence of instructions)
type CoreExpr struct {
	Instrs []CoreInstr
}

// CoreInstr represents a core WebAssembly instruction
// This is a placeholder - actual instruction AST would be much more detailed
type CoreInstr interface {
	isCoreInstr()
}

// CoreExternType represents external types for imports and exports
type CoreExternType interface {
	isCoreExternType()
}

func (*CoreFuncType) isCoreExternType()   {}
func (*CoreTableType) isCoreExternType()  {}
func (*CoreMemType) isCoreExternType()    {}
func (*CoreGlobalType) isCoreExternType() {}
func (*CoreTagType) isCoreExternType()    {}
