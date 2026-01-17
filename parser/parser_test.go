package parser

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/partite-ai/wacogo/ast"
	"github.com/partite-ai/wacogo/testutil"
	"github.com/partite-ai/wacogo/testutil/astmatcher"
	"github.com/tetratelabs/wazero/api"
)

// TestCase represents a single parser test case
type TestCase struct {
	Name            string
	WAT             string
	ExpectedMatcher astmatcher.Matcher // Matcher to validate the AST
	ExpectedErr     string             // If set, expect an error containing this substring
}

// RunParserTests runs a suite of parser test cases
func RunParserTests(t *testing.T, tests []TestCase) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ctx := context.Background()

			// Convert WAT to binary
			wasmBinary, err := testutil.Wat2Wasm(ctx, tt.WAT)
			if err != nil {
				t.Fatalf("failed to convert WAT to binary: %v", err)
			}

			// Parse the binary
			parser := NewParser(bytes.NewReader(wasmBinary))
			component, err := parser.ParseComponent()

			// Check expectations
			if tt.ExpectedErr != "" {
				// Expecting an error
				if err == nil {
					t.Fatalf("expected error containing %q, but got nil", tt.ExpectedErr)
				}
				if !contains(err.Error(), tt.ExpectedErr) {
					t.Fatalf("expected error containing %q, but got: %v", tt.ExpectedErr, err)
				}
				return
			}

			// Not expecting an error
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate with matcher if provided
			if tt.ExpectedMatcher != nil {
				if err := tt.ExpectedMatcher(component); err != nil {
					t.Errorf("matcher validation failed: %v", err)
				}
			}
		})
	}
}

// contains checks if s contains substring substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestBasicComponent tests basic component parsing
func TestBasicComponent(t *testing.T) {
	tests := []TestCase{
		{
			Name:            "empty component",
			WAT:             `(component)`,
			ExpectedMatcher: astmatcher.EmptyComponent(),
		},
		{
			Name: "component with two core modules",
			WAT: `(component
  (core module
    (func (export "a") (result i32) i32.const 0)
    (func (export "b") (result i64) i64.const 0)
  )
  (core module
    (func (export "c") (result f32) f32.const 0)
    (func (export "d") (result f64) f64.const 0)
  )
)`,
			ExpectedMatcher: astmatcher.MatchComponent().WithDefinitions(
				astmatcher.MatchCoreModule(
					astmatcher.CoreModuleValidator(
						astmatcher.CoreModuleExportedFunctions(map[string]func(def api.FunctionDefinition) error{
							"a": func(def api.FunctionDefinition) error {
								paramTypes := def.ParamTypes()
								if len(paramTypes) != 0 {
									return fmt.Errorf("expected no parameters, got %v", paramTypes)
								}
								resultTypes := def.ResultTypes()
								if len(resultTypes) != 1 || resultTypes[0] != api.ValueTypeI32 {
									return fmt.Errorf("expected result type i32, got %v", resultTypes)
								}
								return nil
							},
							"b": func(def api.FunctionDefinition) error {
								paramTypes := def.ParamTypes()
								if len(paramTypes) != 0 {
									return fmt.Errorf("expected no parameters, got %v", paramTypes)
								}
								resultTypes := def.ResultTypes()
								if len(resultTypes) != 1 || resultTypes[0] != api.ValueTypeI64 {
									return fmt.Errorf("expected result type i64, got %v", resultTypes)
								}
								return nil
							},
						}),
					),
				).Match,
				astmatcher.MatchCoreModule(
					astmatcher.CoreModuleValidator(
						astmatcher.CoreModuleExportedFunctions(map[string]func(def api.FunctionDefinition) error{
							"c": func(def api.FunctionDefinition) error {
								paramTypes := def.ParamTypes()
								if len(paramTypes) != 0 {
									return fmt.Errorf("expected no parameters, got %v", paramTypes)
								}
								resultTypes := def.ResultTypes()
								if len(resultTypes) != 1 || resultTypes[0] != api.ValueTypeF32 {
									return fmt.Errorf("expected result type f32, got %v", resultTypes)
								}
								return nil
							},
							"d": func(def api.FunctionDefinition) error {
								paramTypes := def.ParamTypes()
								if len(paramTypes) != 0 {
									return fmt.Errorf("expected no parameters, got %v", paramTypes)
								}
								resultTypes := def.ResultTypes()
								if len(resultTypes) != 1 || resultTypes[0] != api.ValueTypeF64 {
									return fmt.Errorf("expected result type f64, got %v", resultTypes)
								}
								return nil
							},
						}),
					),
				).Match,
			).Match,
		},
		{
			Name: "root level export prohibited",
			WAT: `
			(component
    		(component (export "a"))
  		)
  		`,
			ExpectedErr: "exporting a component from the root component is not supported",
		},
		{
			Name: "type parsing",
			WAT: `
			(component $example:gocomponent
				(type (;0;)
					(component
						(type (;0;)
							(instance
								(export (;0;) "person" (type (sub resource)))
								(type (;1;) (own 0))
								(type (;2;) (func (param "name" string) (result 1)))
								(export (;0;) "[constructor]person" (func (type 2)))
								(type (;3;) (borrow 0))
								(type (;4;) (func (param "self" 3) (result string)))
								(export (;1;) "[method]person.get-name" (func (type 4)))
							)
						)
						(export (;0;) "example:gocomponent/people" (instance (type 0)))
					)
				)
				(export (;1;) "people" (type 0))
			)`,
			ExpectedMatcher: astmatcher.MatchComponent(
				func(c *ast.Component) error {
					dt := c.Definitions[0].(*ast.Type).DefType
					if _, ok := dt.(*ast.ComponentType); !ok {
						return fmt.Errorf("expected component type, got %T", dt)
					}
					return nil
				},
			).Match,
		},
		// Add more test cases as parser implementation progresses
		// Example with expected AST structure:
		// {
		// 	Name: "component with single core module",
		// 	WAT: `(component
		//   (core module
		//     (func (export "a") (result i32) i32.const 0)
		//   )
		// )`,
		// 	ExpectedAST: &ast.Component{
		// 		Definitions: []ast.Definition{
		// 			&ast.CoreModule{
		// 				Exports: []ast.CoreExport{
		// 					{
		// 						Name: "a",
		// 						Desc: &ast.CoreFuncExport{
		// 							Idx: 0,
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
	}

	RunParserTests(t, tests)
}

// TestParserErrors tests error cases
func TestParserErrors(t *testing.T) {
	tests := []TestCase{
		{
			Name:        "invalid version - core module instead of component",
			WAT:         `(module)`, // This is a core module, not a component
			ExpectedErr: "invalid version",
		},
	}

	RunParserTests(t, tests)
}
