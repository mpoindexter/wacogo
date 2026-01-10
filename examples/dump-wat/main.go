package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/partite-ai/wacogo/parser"
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
	p := parser.NewParser(bytes.NewReader(data))
	component, err := p.ParseComponent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse component: %v\n", err)
		os.Exit(1)
	}
	wat := component.ToWAT()
	fmt.Println(wat)
}
