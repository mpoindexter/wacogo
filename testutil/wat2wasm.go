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

	module, err := runtime.InstantiateWithConfig(ctx, wat2wasmBinary, cnf)
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
