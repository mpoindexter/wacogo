package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/parser"
	"github.com/partite-ai/wacogo/testutil"
	"github.com/tetratelabs/wazero"
)

func main() {
	wastFileName := flag.String("file", "", "the wast file to execute")
	flag.Parse()
	if wastFileName == nil || *wastFileName == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -file <file.wast>\n", os.Args[0])
		os.Exit(1)
	}

	f, err := os.Open(*wastFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	contents, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	parser := NewParser(string(contents))
	exprs, err := parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		return
	}

	ctx := context.Background()

	var lastComp *componentmodel.Component
	for _, expr := range exprs {
		if expr.Type != ExprList || len(expr.Children) == 0 || !expr.Children[0].IsAtom() {
			fmt.Println("unhandled expr, expected lists at the top level")
			continue
		}
		switch expr.Children[0].Value {
		case "component":
			fmt.Println("Processing component expression")
			c, err := buildComponent(ctx, expr)
			if err != nil {
				log.Fatalf("Failed to build component %v: %v", expr.Dump(), err)
			}
			lastComp = c
		case "assert_return":
			fmt.Println("Processing assert_return expression")
			inst, err := lastComp.Instantiate(ctx, nil)
			if err != nil {
				log.Fatalf("Failed to instantiate component: %v", err)
			}
			err = assertReturn(ctx, inst, expr)
			if err != nil {
				log.Fatalf("assert_return failed: %v", err)
			}
		case "assert_invalid":
			fmt.Println("Processing assert_invalid expression")
			if len(expr.Children) < 3 {
				log.Fatalf("assert_invalid requires at least 2 arguments")
			}
			c, err := buildComponent(ctx, expr.Children[1])
			if err == nil {
				_, err = c.Instantiate(ctx, nil)
			}
			if err == nil {
				log.Fatalf("Expected assert_invalid to fail, but it succeeded")
			} else {
				expectedMessage := expr.Children[2].Value
				fmt.Printf("assert_invalid passed with error: %v\n", err)
				if !strings.Contains(err.Error(), expectedMessage) {
					log.Fatalf("Expected error to contain %q, but got: %v", expectedMessage, err)
				}
			}
		case "assert_trap":
			fmt.Println("Processing assert_trap expression")
			if len(expr.Children) < 3 {
				log.Fatalf("assert_trap requires at least 2 arguments")
			}
			inst, err := lastComp.Instantiate(ctx, nil)
			if err != nil {
				log.Fatalf("Failed to instantiate component: %v", err)
			}
			actionExpr := expr.Children[1]
			_, err = invokeAction(ctx, inst, actionExpr)
			if err == nil {
				log.Fatalf("Expected assert_trap to fail, but it succeeded")
			} else {
				expectedMessage := expr.Children[2].Value
				fmt.Printf("assert_trap passed with error: %v\n", err)
				if !strings.Contains(err.Error(), expectedMessage) {
					log.Fatalf("Expected error to contain %q, but got: %v", expectedMessage, err)
				}
			}
		default:
			fmt.Printf("Unhandled top-level expression: %s\n", expr.Children[0].Value)
		}
	}
}

func buildComponent(ctx context.Context, expr *SExpr) (*componentmodel.Component, error) {
	componentBinary, err := testutil.Wat2Wasm(ctx, expr.Dump())
	if err != nil {
		return nil, fmt.Errorf("failed to convert component WAT to WASM: %w", err)
	}

	runtime := wazero.NewRuntime(ctx)

	p := parser.NewParser(bytes.NewReader(componentBinary))
	comp, err := p.ParseComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to parse component: %w", err)
	}

	// Build the model
	builder := componentmodel.NewBuilder(runtime)
	return builder.Build(ctx, comp)
}

func assertReturn(ctx context.Context, inst *componentmodel.Instance, expr *SExpr) error {
	if len(expr.Children) < 3 {
		return fmt.Errorf("assert_return requires at least 2 arguments")
	}

	// First child is the action
	actionExpr := expr.Children[1]
	result, err := invokeAction(ctx, inst, actionExpr)
	if err != nil {
		return fmt.Errorf("action failed: %w", err)
	}

	// Remaining children are expected results
	expectedExprs := expr.Children[2:]
	if len(expectedExprs) != 1 {
		return fmt.Errorf("only single expected result is supported")
	}
	expectedExpr := expectedExprs[0]

	expectedValue, err := parseConst(expectedExpr)
	if err != nil {
		return fmt.Errorf("failed to parse expected value: %w", err)
	}

	if result != expectedValue {
		return fmt.Errorf("assertion failed: expected %v, got %v", expectedValue, result)
	}

	fmt.Println("assert_return passed")

	return nil
}

func invokeAction(ctx context.Context, inst *componentmodel.Instance, expr *SExpr) (componentmodel.Value, error) {
	if len(expr.Children) < 2 {
		return nil, fmt.Errorf("action requires at least a function name")
	}

	if expr.Children[0].Value != "invoke" {
		return nil, fmt.Errorf("unsupported action: %v", expr.Value)
	}

	funcNameExpr := expr.Children[1]
	funcName := funcNameExpr.Value

	var args []componentmodel.Value
	for _, argExpr := range expr.Children[2:] {
		arg, err := parseConst(argExpr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %w", err)
		}
		args = append(args, arg)
	}

	export, ok := inst.Export(funcName)
	if !ok {
		return nil, fmt.Errorf("function %s not found in exports", funcName)
	}
	fn, ok := export.(*componentmodel.Function)
	if !ok {
		return nil, fmt.Errorf("export %s is not a function", funcName)
	}
	result, err := fn.Invoke(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke function %s: %w", funcName, err)
	}

	return result, nil
}

func parseConst(expr *SExpr) (componentmodel.Value, error) {
	if len(expr.Children) != 2 {
		return nil, fmt.Errorf("const expression must have exactly one argument")
	}

	switch expr.Children[0].Value {
	case "u8.const":
		valExpr := expr.Children[1]
		val, err := parseUintConst(valExpr.Value, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid u8.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.U8(uint8(val)), nil
	case "s8.const":
		valExpr := expr.Children[1]
		val, err := parseIntConst(valExpr.Value, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid s8.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.S8(int8(val)), nil
	case "u16.const":
		valExpr := expr.Children[1]
		val, err := parseUintConst(valExpr.Value, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid u16.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.U16(uint16(val)), nil
	case "s16.const":
		valExpr := expr.Children[1]
		val, err := parseIntConst(valExpr.Value, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid s16.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.S16(int16(val)), nil
	case "u32.const":
		valExpr := expr.Children[1]
		val, err := parseUintConst(valExpr.Value, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid u32.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.U32(uint32(val)), nil
	case "s32.const":
		valExpr := expr.Children[1]
		val, err := parseIntConst(valExpr.Value, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid s32.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.S32(int32(val)), nil
	case "str.const":
		valExpr := expr.Children[1]
		return componentmodel.String(valExpr.Value), nil
	case "char.const":
		valExpr := expr.Children[1]
		runes := []rune(valExpr.Value)
		if len(runes) != 1 {
			return nil, fmt.Errorf("char.const must be a single character")
		}
		return componentmodel.Char(runes[0]), nil
	case "bool.const":
		valExpr := expr.Children[1]
		val, err := strconv.ParseBool(valExpr.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid bool.const value %v: %w", valExpr.Value, err)
		}
		return componentmodel.Bool(val), nil
	default:
		return nil, fmt.Errorf("unsupported const type: %v", expr.Children[0].Value)

	}
}

func parseIntConst(v string, bitSize int) (int64, error) {
	if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
		val, err := strconv.ParseInt(v[2:], 16, bitSize)
		if err != nil {
			return 0, fmt.Errorf("invalid hex int.const value %v: %w", v, err)
		}
		return val, nil
	}
	val, err := strconv.ParseInt(v, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid int.const value %v: %w", v, err)
	}
	return val, nil
}

func parseUintConst(v string, bitSize int) (uint64, error) {
	if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
		val, err := strconv.ParseUint(v[2:], 16, bitSize)
		if err != nil {
			return 0, fmt.Errorf("invalid hex int.const value %v: %w", v, err)
		}
		return val, nil
	}
	val, err := strconv.ParseUint(v, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid int.const value %v: %w", v, err)
	}
	return val, nil
}
