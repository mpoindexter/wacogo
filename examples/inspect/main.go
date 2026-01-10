package main

import (
	"context"
	"fmt"
	"os"

	"github.com/partite-ai/wacogo"
	"github.com/tetratelabs/wazero"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <component.wasm>\n", os.Args[0])
		os.Exit(1)
	}

	// Read component binary
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsing component (%d bytes)...\n", len(data))

	// Parse and validate the component
	component, err := wacogo.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse component: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Component parsed and validated successfully")

	// List required imports
	imports := component.ListImports()
	if len(imports) > 0 {
		fmt.Printf("\nRequired imports (%d):\n", len(imports))
		for _, imp := range imports {
			fmt.Printf("  - %s (%T)\n", imp.Name, imp.Type)
		}
	} else {
		fmt.Println("\nNo imports required")
	}

	// Create wazero runtime
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// Instantiate the component
	fmt.Println("\nInstantiating component...")
	if err := component.Instantiate(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to instantiate: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Component instantiated successfully")

	// Try to get an export (if any)
	// In a real application, you'd know the export names
	fmt.Println("\nComponent ready for use!")
}
