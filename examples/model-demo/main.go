package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/partite-ai/wacogo/model"
	"github.com/partite-ai/wacogo/parser"
	"github.com/partite-ai/wacogo/wasi/p2"
	"github.com/tetratelabs/wazero"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <component.wasm>\n", os.Args[0])
		os.Exit(1)
	}

	componentPath := os.Args[1]

	// Read the component file
	data, err := os.ReadFile(componentPath)
	if err != nil {
		log.Fatalf("Failed to read component: %v", err)
	}

	// Parse the component
	p := parser.NewParser(bytes.NewReader(data))
	comp, err := p.ParseComponent()
	if err != nil {
		log.Fatalf("Failed to parse component: %v", err)
	}

	// Create a wazero runtime for loading core modules
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// Build the model
	builder := model.NewBuilder(runtime)
	modelComp, err := builder.Build(ctx, comp)
	if err != nil {
		log.Fatalf("Failed to build model: %v", err)
	}

	wasiInstances, err := p2.CreateStandardWASIInstances(
		bytes.NewBuffer(nil),
		os.Stdout,
		os.Stderr,
		os.Args,
		nil,
		"",
	)

	if err != nil {
		log.Fatalf("Failed to create WASI instances: %v", err)
	}

	args := make(map[string]any)
	for k, v := range wasiInstances {
		args[k] = v
	}

	compInst, err := modelComp.Instantiate(context.Background(), args)
	if err != nil {
		log.Fatalf("Failed to instantiate model: %v", err)
	}
	fmt.Println("Component instantiated successfully:", compInst)

	greetComp, ok := compInst.Export("example:gocomponent/greet")
	if !ok {
		log.Fatalf("Export 'example:gocomponent/greet' not found")
	}

	greetFunc, ok := greetComp.(*model.Instance).Export("greet-all")
	if !ok {
		log.Fatalf("Export 'greet-all' not found in greet component")
	}

	greetFuncTyped, ok := greetFunc.(*model.Function)
	if !ok {
		log.Fatalf("'greet-all' is not a function")
	}
	fmt.Println(greetFuncTyped)
}
