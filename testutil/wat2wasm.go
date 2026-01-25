package testutil

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

//go:embed wasm-tools.wasm
var wat2wasmBinary []byte

var compileCache wazero.CompilationCache = wazero.NewCompilationCache()

func init() {
	// Precompile wat2wasm to speed up tests.
	ctx := context.Background()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(compileCache))
	defer runtime.Close(ctx)

	_, err := runtime.CompileModule(ctx, wat2wasmBinary)
	if err != nil {
		panic(fmt.Errorf("precompile wat2wasm: %w", err))
	}

	// Don't close the module here as we want to keep it in the cache.
}

// Wat2Wasm converts the given WAT to WASM binary using wazero's wat2wasm module.
func Wat2Wasm(ctx context.Context, wat string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cnf := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithStdin(strings.NewReader(wat)).
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime().
		WithArgs("wasm-tools", "parse", "-")

	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(compileCache))
	defer runtime.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	cm, err := runtime.CompileModule(ctx, wat2wasmBinary)
	if err != nil {
		return nil, fmt.Errorf("compile wat2wasm: %w", err)
	}
	defer cm.Close(ctx)

	module, err := runtime.InstantiateModule(ctx, cm, cnf)
	if err != nil {
		if exitErr, ok := err.(*sys.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode == sys.ExitCodeDeadlineExceeded {
				return nil, fmt.Errorf("wat2wasm timed out: %v", stderr.String())
			}
			return nil, fmt.Errorf("wat2wasm exited with code %d: %v", exitCode, stderr.String())
		}
		return nil, err
	}
	defer module.Close(ctx)

	wasmBinary := stdout.Bytes()

	return wasmBinary, nil
}
