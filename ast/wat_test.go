package ast

import (
	"strings"
	"testing"
)

func TestComponentToWAT(t *testing.T) {
	comp := &Component{
		Definitions: []Definition{
			&Type{
				DefType: &RecordType{
					Fields: []RecordField{
						{Label: "x", Type: &S32Type{}},
						{Label: "y", Type: &S32Type{}},
					},
				},
			},
			&Export{
				ExportName: "test",
				SortIdx:    SortIdx{Sort: SortType, Idx: 0},
			},
		},
	}

	wat := comp.ToWAT()

	// Check basic structure
	if !strings.Contains(wat, "(component") {
		t.Errorf("Expected component, got: %s", wat)
	}
	if !strings.Contains(wat, "(type") {
		t.Errorf("Expected type definition, got: %s", wat)
	}
	if !strings.Contains(wat, "(record") {
		t.Errorf("Expected record type, got: %s", wat)
	}
	if !strings.Contains(wat, "(export \"test\"") {
		t.Errorf("Expected export, got: %s", wat)
	}

	t.Logf("Generated WAT:\n%s", wat)
}

func TestPrimitiveTypes(t *testing.T) {
	tests := []struct {
		name     string
		defType  DefType
		expected string
	}{
		{"bool", &BoolType{}, "bool"},
		{"s8", &S8Type{}, "s8"},
		{"u8", &U8Type{}, "u8"},
		{"s16", &S16Type{}, "s16"},
		{"u16", &U16Type{}, "u16"},
		{"s32", &S32Type{}, "s32"},
		{"u32", &U32Type{}, "u32"},
		{"s64", &S64Type{}, "s64"},
		{"u64", &U64Type{}, "u64"},
		{"f32", &F32Type{}, "f32"},
		{"f64", &F64Type{}, "f64"},
		{"char", &CharType{}, "char"},
		{"string", &StringType{}, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defTypeToWAT(tt.defType, 0)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestVariantType(t *testing.T) {
	vt := &VariantType{
		Cases: []VariantCase{
			{Label: "ok", Type: &S32Type{}},
			{Label: "error", Type: &StringType{}},
			{Label: "none", Type: nil},
		},
	}

	result := variantTypeToWAT(vt)
	expected := "(variant (case \"ok\" s32) (case \"error\" string) (case \"none\"))"

	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestListType(t *testing.T) {
	lt := &ListType{Element: &U8Type{}}
	result := listTypeToWAT(lt)
	expected := "(list u8)"

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestFuncType(t *testing.T) {
	ft := &FuncType{
		Params: []FuncParam{
			{Label: "x", Type: &S32Type{}},
			{Label: "y", Type: &S32Type{}},
		},
		Results: &S32Type{},
	}

	result := funcTypeToWAT(ft)

	if !strings.Contains(result, "(func") {
		t.Errorf("Expected func type, got: %s", result)
	}
	if !strings.Contains(result, "(param \"x\" s32)") {
		t.Errorf("Expected param x, got: %s", result)
	}
	if !strings.Contains(result, "(param \"y\" s32)") {
		t.Errorf("Expected param y, got: %s", result)
	}
	if !strings.Contains(result, "(result s32)") {
		t.Errorf("Expected result, got: %s", result)
	}

	t.Logf("Function type WAT: %s", result)
}

func TestCoreModuleToWAT(t *testing.T) {
	cm := &CoreModule{
		Raw: []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
	}

	result := coreModuleToWAT(cm)

	if !strings.Contains(result, "(core module") {
		t.Errorf("Expected core module, got: %s", result)
	}
	if !strings.Contains(result, "(binary") {
		t.Errorf("Expected binary data, got: %s", result)
	}

	t.Logf("Core module WAT: %s", result)
}

func TestAliasToWAT(t *testing.T) {
	tests := []struct {
		name     string
		alias    *Alias
		contains []string
	}{
		{
			name: "export alias",
			alias: &Alias{
				Target: &ExportAlias{InstanceIdx: 0, Name: "foo"},
				Sort:   SortFunc,
			},
			contains: []string{"(alias", "export", "foo"},
		},
		{
			name: "outer alias",
			alias: &Alias{
				Target: &OuterAlias{Count: 1, Idx: 2},
				Sort:   SortType,
			},
			contains: []string{"(alias", "outer", "1", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aliasToWAT(tt.alias)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Expected %s to contain %s", result, s)
				}
			}
			t.Logf("Alias WAT: %s", result)
		})
	}
}

func TestCanonLift(t *testing.T) {
	cl := &CanonLift{
		CoreFuncIdx: 5,
		Options: []CanonOpt{
			&StringEncodingOpt{Encoding: StringEncodingUTF8},
			&MemoryOpt{MemoryIdx: 0},
		},
	}

	result := canonDefToWAT(cl)

	if !strings.Contains(result, "(canon lift") {
		t.Errorf("Expected canon lift, got: %s", result)
	}
	if !strings.Contains(result, "(core func 5)") {
		t.Errorf("Expected core func reference, got: %s", result)
	}
	if !strings.Contains(result, "string-encoding=utf8") {
		t.Errorf("Expected string encoding option, got: %s", result)
	}
	if !strings.Contains(result, "(memory 0)") {
		t.Errorf("Expected memory option, got: %s", result)
	}

	t.Logf("Canon lift WAT: %s", result)
}

func TestResultType(t *testing.T) {
	tests := []struct {
		name     string
		rt       *ResultType
		expected string
	}{
		{
			name:     "both ok and error",
			rt:       &ResultType{Ok: &S32Type{}, Error: &StringType{}},
			expected: "(result (ok s32) (error string))",
		},
		{
			name:     "only ok",
			rt:       &ResultType{Ok: &S32Type{}, Error: nil},
			expected: "(result (ok s32))",
		},
		{
			name:     "only error",
			rt:       &ResultType{Ok: nil, Error: &StringType{}},
			expected: "(result (error string))",
		},
		{
			name:     "neither",
			rt:       &ResultType{Ok: nil, Error: nil},
			expected: "(result)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resultTypeToWAT(tt.rt)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCoreFuncType(t *testing.T) {
	cft := &CoreFuncType{
		Params:  CoreResultType{Types: []CoreValType{CoreNumTypeI32, CoreNumTypeI64}},
		Results: CoreResultType{Types: []CoreValType{CoreNumTypeF32}},
	}

	result := coreFuncTypeToWAT(cft)
	expected := "(func (param i32 i64) (result f32))"

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCoreRefType(t *testing.T) {
	tests := []struct {
		name     string
		rt       *CoreRefType
		expected string
	}{
		{
			name:     "nullable func",
			rt:       &CoreRefType{Nullable: true, HeapType: CoreAbsHeapTypeFunc},
			expected: "(ref null func)",
		},
		{
			name:     "non-nullable extern",
			rt:       &CoreRefType{Nullable: false, HeapType: CoreAbsHeapTypeExtern},
			expected: "(ref extern)",
		},
		{
			name:     "concrete type",
			rt:       &CoreRefType{Nullable: true, HeapType: &CoreConcreteHeapType{TypeIdx: 5}},
			expected: "(ref null 5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coreRefTypeToWAT(tt.rt)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestComplexComponentToWAT(t *testing.T) {
	// Create a more complex component with nested structures
	comp := &Component{
		Definitions: []Definition{
			// Resource type
			&Type{
				DefType: &ResourceType{
					Rep: CoreNumTypeI32,
				},
			},
			// Nested component
			&NestedComponent{
				Component: &Component{
					Definitions: []Definition{
						&Type{
							DefType: &EnumType{
								Labels: []string{"pending", "success", "failure"},
							},
						},
					},
				},
			},
			// Import
			&Import{
				ImportName: "logger",
				Desc: &SortExternDesc{
					Sort:    SortFunc,
					TypeIdx: 0,
				},
			},
			// Canonical lift
			&Canon{
				Def: &CanonLift{
					CoreFuncIdx: 0,
					Options: []CanonOpt{
						&StringEncodingOpt{Encoding: StringEncodingUTF8},
					},
				},
			},
		},
	}

	wat := comp.ToWAT()

	// Verify structure
	if !strings.Contains(wat, "(component") {
		t.Errorf("Missing component header")
	}
	if !strings.Contains(wat, "(resource") {
		t.Errorf("Missing resource type")
	}
	if !strings.Contains(wat, "(component") {
		t.Errorf("Missing nested component")
	}
	if !strings.Contains(wat, "(enum") {
		t.Errorf("Missing enum type")
	}
	if !strings.Contains(wat, "(import \"logger\"") {
		t.Errorf("Missing import")
	}
	if !strings.Contains(wat, "(canon lift") {
		t.Errorf("Missing canon lift")
	}

	t.Logf("Complex component WAT:\n%s", wat)
}
