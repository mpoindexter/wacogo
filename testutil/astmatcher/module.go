package astmatcher

import (
	"context"
	"fmt"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func CoreModuleValidator(validators ...func(wazero.CompiledModule) error) func(*ast.CoreModule) error {
	return func(module *ast.CoreModule) error {
		ctx := context.Background()
		rt := wazero.NewRuntime(ctx)
		defer rt.Close(ctx)

		mod, err := rt.CompileModule(ctx, module.Raw)
		if err != nil {
			return err
		}
		defer mod.Close(ctx)

		for _, validator := range validators {
			if err := validator(mod); err != nil {
				return err
			}
		}
		return nil
	}
}

func CoreModuleExportedFunctions(expectedExportFunctions map[string]func(def api.FunctionDefinition) error) func(wazero.CompiledModule) error {
	return func(mod wazero.CompiledModule) error {
		exports := mod.ExportedFunctions()
		if len(exports) != len(expectedExportFunctions) {
			return fmt.Errorf("expected %d exports, got %d", len(expectedExportFunctions), len(exports))
		}
		for name, def := range exports {
			expect := expectedExportFunctions[name]
			if expect == nil {
				return fmt.Errorf("unexpected export function: %s", name)
			}
			if err := expect(def); err != nil {
				return fmt.Errorf("export function %s validation failed: %w", name, err)
			}
		}
		return nil
	}
}
