package spectest

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"strings"
	"testing"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
	"github.com/partite-ai/wacogo/internal/wasmtools"
	"github.com/partite-ai/wacogo/parser"
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero"
)

//go:embed compiled/**
var testData embed.FS

//go:embed host_mod.wasm
var hostModuleBinary []byte

func TestSpec(t *testing.T) {
	fs.WalkDir(testData, "compiled", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatalf("Failed to walk to %s: %v", filePath, err)
		}
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(filePath, "wast.json") {
			t.Run(path.Dir(filePath)[len("compiled/"):], func(t *testing.T) {
				runSpecTest(t, testData, filePath)
			})
		}
		return nil
	})
}

func makeHostCoreModule(ctx context.Context, runtime wazero.Runtime) (wazero.CompiledModule, *wasm.Externs) {
	compiled, err := runtime.CompileModule(ctx, hostModuleBinary)
	if err != nil {
		panic(fmt.Errorf("failed to compile core module: %w", err))
	}

	additionalExterns, err := wasm.ReadExterns(hostModuleBinary)
	if err != nil {
		panic(fmt.Errorf("failed to read additional exports: %w", err))
	}

	return compiled, additionalExterns
}

func makeHostInstance(ctx context.Context, runtime wazero.Runtime) *componentmodel.Instance {
	hiNested := host.NewInstance()
	hiNested.MustAddFunction("return-four", func() componentmodel.U32 {
		return 4
	})

	hostModule, hostModuleExterns := makeHostCoreModule(ctx, runtime)

	ib := componentmodel.NewInstanceBuilder().AddInstanceExport("nested", hiNested.Instance()).AddCoreModuleExport(
		"simple-module", hostModule, hostModuleExterns,
	)
	hi := host.NewInstanceWithBuilder(ib)
	hi.MustAddFunction("return-three", func() componentmodel.U32 {
		return componentmodel.U32(3)
	})

	var resource1TypeState resourceState
	hi.AddTypeExport("resource1", host.ResourceTypeFor[*resource1Type](hi, hi))
	hi.AddTypeExport("resource1-again", host.ResourceTypeFor[*resource1Type](hi, hi))
	hi.MustAddFunction("[constructor]resource1", func(r componentmodel.U32) host.Own[*resource1Type] {
		return host.NewOwn(&resource1Type{
			state: &resource1TypeState,
			value: uint32(r),
		})
	})
	hi.MustAddFunction("[static]resource1.assert", func(rsc host.Own[*resource1Type], rep componentmodel.U32) {
		if uint32(rsc.Resource().value) != uint32(rep) {
			panic(fmt.Errorf("resource1 assertion failed: expected %d, got %d", rep, rsc.Resource().value))
		}
	})
	hi.MustAddFunction("[static]resource1.last-drop", func() componentmodel.U32 {
		return componentmodel.U32(resource1TypeState.lastDroppedValue)
	})
	hi.MustAddFunction("[static]resource1.drops", func() componentmodel.U32 {
		return componentmodel.U32(resource1TypeState.dropCount)
	})
	hi.MustAddFunction("[method]resource1.simple", func(self host.Borrow[*resource1Type], rep uint32) {
		if self.Resource().value != rep {
			panic(fmt.Errorf("resource1.simple assertion failed: expected %d, got %d", rep, self.Resource().value))
		}
	})
	hi.MustAddFunction("[method]resource1.take-borrow", func(self host.Borrow[*resource1Type], other host.Borrow[*resource1Type]) {

	})
	hi.MustAddFunction("[method]resource1.take-own", func(self host.Borrow[*resource1Type], other host.Own[*resource1Type]) {
		fmt.Print()
	})

	hi.AddTypeExport("resource2", host.ResourceTypeFor[*resource2Type](hi, hi))

	return hi.Instance()
}

var hostFunctions = map[string]any{
	"host-echo-u32": componentmodel.NewFunction(
		&componentmodel.FunctionType{
			Parameters: []*componentmodel.FunctionParameter{
				{Name: "value", Type: componentmodel.U32Type{}},
			},
			ResultType: componentmodel.U32Type{},
		},
		func(ctx context.Context, params []componentmodel.Value) (componentmodel.Value, error) {
			val := params[0].(componentmodel.U32)
			return val, nil
		},
	),
	"host-return-two": componentmodel.NewFunction(
		&componentmodel.FunctionType{
			Parameters: []*componentmodel.FunctionParameter{},
			ResultType: componentmodel.U32Type{},
		},
		func(ctx context.Context, params []componentmodel.Value) (componentmodel.Value, error) {
			return componentmodel.U32(2), nil
		},
	),
}

func runSpecTest(t *testing.T, fsys fs.FS, jsonPath string) {
	f, err := fsys.Open(jsonPath)
	if err != nil {
		t.Fatal("Failed to open spec test JSON:", err)
	}
	defer f.Close()

	contents, err := io.ReadAll(f)
	if err != nil {
		t.Fatal("Failed to read spec test JSON:", err)
	}

	var testCases specTestCases
	if err := json.Unmarshal(contents, &testCases); err != nil {
		t.Fatal("Failed to unmarshal spec test JSON:", err)
	}

	runtime := wazero.NewRuntime(t.Context())
	defer runtime.Close(t.Context())

	tc := &testContext{
		fs:         fsys,
		path:       path.Dir(jsonPath),
		runtime:    runtime,
		components: make(map[string]*componentmodel.Component),
		instances:  make(map[string]*componentmodel.Instance),
	}

	for _, cmd := range testCases.Commands {
		t.Run(fmt.Sprintf("line:%d", cmd.line), func(t *testing.T) {
			cmd.command.execute(t, tc)
		})
	}
}

func buildComponent(ctx context.Context, runtime wazero.Runtime, src []byte) (*componentmodel.Component, error) {
	if err := wasmtools.ValidateWasm(ctx, src); err != nil {
		return nil, fmt.Errorf("Failed to validate component: %v", err)
	}

	p := parser.NewParser(bytes.NewReader(src))
	comp, err := p.ParseComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to parse component: %w", err)
	}

	// Build the model
	builder := componentmodel.NewBuilder(runtime)
	return builder.Build(ctx, comp)
}

type specTestCases struct {
	SourceFileName string                    `json:"source_file_name"`
	Commands       []*specTestCommandWrapper `json:"commands"`
}

type specTestCommandWrapper struct {
	line    int
	command specTestCommand
}

func (w *specTestCommandWrapper) UnmarshalJSON(data []byte) error {
	var typeDecode struct {
		Line int    `json:"line"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeDecode); err != nil {
		return err
	}
	w.line = typeDecode.Line
	switch typeDecode.Type {
	case "module":
		var cmd specTestModuleCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	case "module_definition":
		var cmd specTestModuleCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		cmd.DefinitionOnly = true
		w.command = &cmd
	case "module_instance":
		var cmd specTestModuleInstanceCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	case "assert_malformed":
		var cmd specTestAssertMalformedCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	case "assert_invalid", "assert_unlinkable", "assert_uninstantiable":
		var cmd specTestAssertInvalidCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	case "assert_return":
		var cmd specTestAssertReturnCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	case "assert_trap":
		var cmd specTestAssertTrapCommand
		if err := json.Unmarshal(data, &cmd); err != nil {
			return err
		}
		w.command = &cmd
	default:
		return fmt.Errorf("unsupported command type: %s", typeDecode.Type)
	}
	return nil
}

type testContext struct {
	fs            fs.FS
	path          string
	runtime       wazero.Runtime
	lastComponent *componentmodel.Component
	lastInstance  *componentmodel.Instance
	components    map[string]*componentmodel.Component
	instances     map[string]*componentmodel.Instance
}

type specTestCommand interface {
	execute(t *testing.T, ctx *testContext)
}

type specTestModuleCommand struct {
	Name           string `json:"name"`
	Filename       string `json:"filename"`
	ModuleType     string `json:"module_type"`
	DefinitionOnly bool   `json:"-"`
}

func (cmd *specTestModuleCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing module command: name=%s, filename=%s, module_type=%s", cmd.Name, cmd.Filename, cmd.ModuleType)
	modPath := path.Join(tc.path, cmd.Filename)
	contents, err := fs.ReadFile(tc.fs, modPath)
	if err != nil {
		t.Fatalf("Failed to read module file %s: %v", modPath, err)
	}

	component, err := buildComponent(t.Context(), tc.runtime, contents)
	if err != nil {
		t.Fatalf("Failed to build component from file %s: %v", modPath, err)
	}
	t.Logf("Successfully built component %s from file %s", cmd.Name, modPath)

	if cmd.Name != "" {
		tc.components[cmd.Name] = component
	}
	tc.lastComponent = component

	if cmd.DefinitionOnly {
		t.Logf("Skipping instantiation for component %s as it is definition only", cmd.Name)
		return
	}

	args := make(map[string]any)
	maps.Copy(args, hostFunctions)
	args["host"] = makeHostInstance(t.Context(), tc.runtime)
	for name, val := range tc.instances {
		args[name] = val
	}
	inst, err := component.Instantiate(t.Context(), args)
	if err != nil {
		t.Fatalf("Failed to instantiate component %s: %v", cmd.Name, err)
	}
	if cmd.Name != "" {
		tc.instances[cmd.Name] = inst
	}
	tc.lastInstance = inst
	t.Logf("Successfully instantiated component %s", cmd.Name)
}

type specTestModuleInstanceCommand struct {
	Instance string `json:"instance"`
	Module   string `json:"module"`
}

func (cmd *specTestModuleInstanceCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing module instance command: instance=%s, module=%s", cmd.Instance, cmd.Module)
	comp, ok := tc.components[cmd.Module]
	if !ok {
		t.Fatalf("Component %s not found for instantiation", cmd.Module)
	}

	args := make(map[string]any)
	for name, val := range hostFunctions {
		args[name] = val
	}
	args["host"] = makeHostInstance(t.Context(), tc.runtime)
	for name, val := range tc.instances {
		args[name] = val
	}

	inst, err := comp.Instantiate(t.Context(), args)
	if err != nil {
		t.Fatalf("Failed to instantiate component %s: %v", cmd.Module, err)
	}

	if cmd.Instance != "" {
		tc.instances[cmd.Instance] = inst
	}
	tc.lastInstance = inst
	t.Logf("Successfully instantiated component %s as instance %s", cmd.Module, cmd.Instance)
}

type specTestAssertMalformedCommand struct {
	Filename   string `json:"filename"`
	ModuleType string `json:"module_type"`
	Text       string `json:"text"`
}

func (cmd *specTestAssertMalformedCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing assert_malformed command: filename=%s, module_type=%s", cmd.Filename, cmd.ModuleType)
	if cmd.ModuleType != "binary" {
		t.Skip("Only binary module_type is supported for assert_malformed")
	}
	modPath := path.Join(tc.path, cmd.Filename)
	contents, err := fs.ReadFile(tc.fs, modPath)
	if err != nil {
		t.Fatalf("Failed to read module file %s: %v", modPath, err)
	}

	_, err = parser.NewParser(bytes.NewReader(contents)).ParseComponent()
	if err == nil {
		t.Fatalf("Expected assert_malformed to fail for file %s, but it succeeded", modPath)
	} else {
		if !strings.Contains(err.Error(), cmd.Text) {
			t.Fatalf("Expected error to contain %q, but got: %q", cmd.Text, err)
		}
		t.Logf("assert_malformed passed with error: %v\n", err)
	}
}

type specTestAssertInvalidCommand struct {
	Filename   string `json:"filename"`
	ModuleType string `json:"module_type"`
	Text       string `json:"text"`
}

func (cmd *specTestAssertInvalidCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing assert_invalid command: filename=%s, module_type=%s", cmd.Filename, cmd.ModuleType)
	modPath := path.Join(tc.path, cmd.Filename)
	contents, err := fs.ReadFile(tc.fs, modPath)
	if err != nil {
		t.Fatalf("Failed to read module file %s: %v", modPath, err)
	}

	args := make(map[string]any)
	for name, val := range hostFunctions {
		args[name] = val
	}
	args["host"] = makeHostInstance(t.Context(), tc.runtime)
	for name, val := range tc.instances {
		args[name] = val
	}
	component, err := buildComponent(t.Context(), tc.runtime, contents)
	if err == nil {
		_, err = component.Instantiate(t.Context(), args)
	}

	if err == nil {
		t.Fatalf("Expected assert_invalid to fail for file %s, but it succeeded", modPath)
	} else {
		if !strings.Contains(err.Error(), cmd.Text) {
			t.Fatalf("Expected error to contain %q, but got: %q", cmd.Text, err)
		}
		t.Logf("assert_invalid passed with error: %v\n", err)
	}
}

type value struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (v *value) toComponentModelValue() (componentmodel.Value, error) {
	switch v.Type {
	case "u8":
		var u8 stringifiedNumber[uint8]
		err := json.Unmarshal(v.Value, &u8)
		if err != nil {
			return nil, fmt.Errorf("invalid u8 value %v: %w", v.Value, err)
		}
		return componentmodel.U8(u8.Value), nil
	case "s8":
		var s8 stringifiedNumber[int8]
		err := json.Unmarshal(v.Value, &s8)
		if err != nil {
			return nil, fmt.Errorf("invalid s8 value %v: %w", v.Value, err)
		}
		return componentmodel.S8(s8.Value), nil
	case "u16":
		var u16 stringifiedNumber[uint16]
		err := json.Unmarshal(v.Value, &u16)
		if err != nil {
			return nil, fmt.Errorf("invalid u16 value %v: %w", v.Value, err)
		}
		return componentmodel.U16(u16.Value), nil
	case "s16":
		var s16 stringifiedNumber[int16]
		err := json.Unmarshal(v.Value, &s16)
		if err != nil {
			return nil, fmt.Errorf("invalid s16 value %v: %w", v.Value, err)
		}
		return componentmodel.S16(s16.Value), nil
	case "u32":
		var u32 stringifiedNumber[uint32]
		err := json.Unmarshal(v.Value, &u32)
		if err != nil {
			return nil, fmt.Errorf("invalid u32 value %v: %w", v.Value, err)
		}
		return componentmodel.U32(u32.Value), nil
	case "s32":
		var s32 stringifiedNumber[int32]
		err := json.Unmarshal(v.Value, &s32)
		if err != nil {
			return nil, fmt.Errorf("invalid s32 value %v: %w", v.Value, err)
		}
		return componentmodel.S32(s32.Value), nil
	case "string":
		var str string
		err := json.Unmarshal(v.Value, &str)
		if err != nil {
			return nil, fmt.Errorf("invalid str value %v: %w", v.Value, err)
		}
		return componentmodel.String(str), nil
	case "char":
		var char string
		err := json.Unmarshal(v.Value, &char)
		if err != nil {
			return nil, fmt.Errorf("invalid char value %v: %w", v.Value, err)
		}
		runes := []rune(char)
		if len(runes) != 1 {
			return nil, fmt.Errorf("char value must be a single character")
		}
		return componentmodel.Char(runes[0]), nil
	case "bool":
		var b bool
		err := json.Unmarshal(v.Value, &b)
		if err != nil {
			return nil, fmt.Errorf("invalid bool value %v: %w", v.Value, err)
		}
		return componentmodel.Bool(b), nil
	case "list":
		var rawElements []*value
		err := json.Unmarshal(v.Value, &rawElements)
		if err != nil {
			return nil, fmt.Errorf("invalid list value %v: %w", v.Value, err)
		}
		var elements []componentmodel.Value
		for _, rawElem := range rawElements {
			elem, err := rawElem.toComponentModelValue()
			if err != nil {
				return nil, fmt.Errorf("invalid list element %v: %w", rawElem, err)
			}
			elements = append(elements, elem)
		}
		return componentmodel.List(elements), nil
	default:
		return nil, fmt.Errorf("unsupported value type: %s", v.Type)
	}
}

type action interface {
	Execute(t *testing.T, inst *componentmodel.Instance) (componentmodel.Value, error)
}

type actionWrapper struct {
	action action
}

func (w *actionWrapper) UnmarshalJSON(data []byte) error {
	var typeDecode struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeDecode); err != nil {
		return err
	}
	switch typeDecode.Type {
	case "invoke":
		var act invokeAction
		if err := json.Unmarshal(data, &act); err != nil {
			return err
		}
		w.action = &act
	default:
		return fmt.Errorf("unsupported action type: %s", typeDecode.Type)
	}
	return nil
}

type invokeAction struct {
	Field string   `json:"field"`
	Args  []*value `json:"args,omitempty"`
}

func (a *invokeAction) Execute(t *testing.T, inst *componentmodel.Instance) (componentmodel.Value, error) {
	var args []componentmodel.Value
	for _, arg := range a.Args {
		val, err := arg.toComponentModelValue()
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %w", err)
		}
		args = append(args, val)
	}

	export, ok := inst.Export(a.Field)
	if !ok {
		return nil, fmt.Errorf("function %s not found in exports", a.Field)
	}
	fn, ok := export.(*componentmodel.Function)
	if !ok {
		return nil, fmt.Errorf("export %s is not a function", a.Field)
	}
	result, err := fn.Invoke(t.Context(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke function %s: %w", a.Field, err)
	}

	return result, nil
}

type specTestAssertReturnCommand struct {
	Action   actionWrapper `json:"action"`
	Expected []*value      `json:"expected"`
}

func (cmd *specTestAssertReturnCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing assert_return command")
	if tc.lastInstance == nil {
		t.Fatalf("No instance available to execute assert_return")
	}

	result, err := cmd.Action.action.Execute(t, tc.lastInstance)
	if err != nil {
		t.Fatalf("Action execution failed: %v", err)
	}

	if len(cmd.Expected) == 0 {
		// No expected results to check
		return
	}
	if len(cmd.Expected) != 1 {
		t.Fatalf("Only single expected result is supported")
	}
	expectedValue, err := cmd.Expected[0].toComponentModelValue()
	if err != nil {
		t.Fatalf("Failed to parse expected value: %v", err)
	}

	if result != expectedValue {
		t.Fatalf("Assertion failed: expected %v, got %v", expectedValue, result)
	}

	t.Logf("assert_return passed")
}

type specTestAssertTrapCommand struct {
	Action actionWrapper `json:"action"`
	Text   string        `json:"text"`
}

func (cmd *specTestAssertTrapCommand) execute(t *testing.T, tc *testContext) {
	t.Logf("Executing assert_trap command")
	if tc.lastInstance == nil {
		t.Fatalf("No instance available to execute assert_trap")
	}

	_, err := cmd.Action.action.Execute(t, tc.lastInstance)
	if err == nil {
		t.Fatalf("Expected assert_trap to fail, but it succeeded")
	} else {
		t.Logf("assert_trap passed with error: %v\n", err)
		if !strings.Contains(err.Error(), cmd.Text) {
			t.Fatalf("Expected error to contain %q, but got: %v", cmd.Text, err)
		}
	}
}

type number interface {
	uint8 | int8 | uint16 | int16 | uint32 | int32 | uint64 | int64 | float32 | float64
}
type stringifiedNumber[T number] struct {
	Value T
}

func (sn *stringifiedNumber[T]) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("failed to unmarshal stringified number: %w", err)
	}
	var val T
	_, err := fmt.Sscan(str, &val)
	if err != nil {
		return fmt.Errorf("failed to parse stringified number %q: %w", str, err)
	}
	sn.Value = val
	return nil
}

type resourceState struct {
	dropCount        uint32
	lastDroppedValue uint32
}

type resource1Type struct {
	state *resourceState
	value uint32
}

func (r resource1Type) Close() error {
	r.state.dropCount++
	r.state.lastDroppedValue = r.value
	return nil
}

type resource2Type struct {
	state *resourceState
	value uint32
}

func (r resource2Type) Close() error {
	r.state.dropCount++
	r.state.lastDroppedValue = r.value
	return nil
}
