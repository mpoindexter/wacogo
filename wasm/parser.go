package wasm

import (
	"bytes"
	"fmt"
)

type Exports struct {
	Tables   map[string]*TableType
	Globals  map[string]*GlobalType
	Memories map[string]*MemoryType
}

func ReadExports(mod []byte) (*Exports, error) {

	var err error
	mod, err = readModuleHeader(mod)
	if err != nil {
		return nil, err
	}

	exportedTables := make(map[string]uint32)
	exportedGlobals := make(map[string]uint32)
	exportedMemories := make(map[string]uint32)
	tableTypes := make(map[uint32]*TableType)
	memoryTypes := make(map[uint32]*MemoryType)
	globalTypes := make(map[uint32]*GlobalType)

	var tableTypeOffset uint32 = 0
	var memoryTypeOffset uint32 = 0
	var globalTypeOffset uint32 = 0

	// Parse and transform sections
	for len(mod) > 0 {

		// Read section ID
		sectionID := mod[0]
		mod = mod[1:]

		// Read section size (LEB128)
		sectionSize, bytesRead, err := readLEB128(mod)
		if err != nil {
			return nil, fmt.Errorf("failed to read section size: %w", err)
		}
		mod = mod[bytesRead:]

		// Check bounds
		if len(mod) < int(sectionSize) {
			return nil, fmt.Errorf("section size exceeds module bounds")
		}

		// Extract section data
		sectionData := mod[:int(sectionSize)]
		mod = mod[int(sectionSize):]

		switch sectionID {
		case 2: // Import section
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read import count: %w", err)
			}

			// Process each import
			for range count {
				// Read module name
				_, err := readName(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read import module name: %w", err)
				}

				// Read name
				_, err = readName(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read import name: %w", err)
				}

				// Read kind
				kind, err := reader.ReadByte()
				if err != nil {
					return nil, fmt.Errorf("failed to read import kind: %w", err)
				}

				switch kind {
				case 0x00: // function
					// Skip function type index
					_, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip imported function type index: %w", err)
					}
				case 0x01: // table
					// Read table type
					_, tableType, err := readTableType(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to read imported table type: %w", err)
					}

					tableTypes[tableTypeOffset] = tableType
					tableTypeOffset++
				case 0x02: // memory
					// Read memory type
					_, memoryType, err := readMemoryType(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to read imported memory type: %w", err)
					}

					memoryTypes[memoryTypeOffset] = memoryType
					memoryTypeOffset++
				case 0x03: // global
					// Read global type
					_, globalType, err := readGlobalType(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to read imported global type: %w", err)
					}

					globalTypes[globalTypeOffset] = globalType
					globalTypeOffset++

				default:
					return nil, fmt.Errorf("unknown import kind: %d", kind)
				}
			}
		case 4: // Table section
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read table count: %w", err)
			}

			// Process each table
			for range count {
				// Read table type
				_, tableType, err := readTableType(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read table type: %w", err)
				}

				tableTypes[tableTypeOffset] = tableType
				tableTypeOffset++
			}
		case 5: // Memory section
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read memory count: %w", err)
			}

			// Process each memory
			for range count {
				// Read memory type
				_, memoryType, err := readMemoryType(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read memory type: %w", err)
				}

				memoryTypes[memoryTypeOffset] = memoryType
				memoryTypeOffset++
			}
		case 6: // Global section
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read global count: %w", err)
			}

			// Process each global
			for range count {
				// Read global type
				_, globalType, err := readGlobalType(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read global type: %w", err)
				}

				globalTypes[globalTypeOffset] = globalType
				globalTypeOffset++

				// Skip init expr
				_, err = readConstExpression(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to skip global init expr: %w", err)
				}
			}
		case 7: // Export section
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read export count: %w", err)
			}

			// Process each export
			for range count {
				name, err := readName(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read export name: %w", err)
				}

				kind, err := reader.ReadByte()
				if err != nil {
					return nil, fmt.Errorf("failed to read export kind: %w", err)
				}

				switch kind {
				case 0x00: // function
					// Skip function index
					_, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip function export index: %w", err)
					}
				case 0x01: // table
					idx, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to read table index: %w", err)
					}
					exportedTables[name] = idx
				case 0x02: // memory
					// Skip memory index
					idx, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip memory export index: %w", err)
					}
					exportedMemories[name] = idx
				case 0x03: // global
					// Skip global index
					idx, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip global export index: %w", err)
					}
					exportedGlobals[name] = idx
				default:
					return nil, fmt.Errorf("unknown export kind: %d", kind)
				}
			}
		}
	}

	// Build final export map
	exports := &Exports{
		Tables:   make(map[string]*TableType),
		Globals:  make(map[string]*GlobalType),
		Memories: make(map[string]*MemoryType),
	}

	for name, idx := range exportedTables {
		tableType, ok := tableTypes[idx]
		if !ok {
			return nil, fmt.Errorf("table type for index %d not found", idx)
		}
		exports.Tables[name] = tableType
	}

	for name, idx := range exportedMemories {
		memoryType, ok := memoryTypes[idx]
		if !ok {
			return nil, fmt.Errorf("memory type for index %d not found", idx)
		}
		exports.Memories[name] = memoryType
	}

	for name, idx := range exportedGlobals {
		globalType, ok := globalTypes[idx]
		if !ok {
			return nil, fmt.Errorf("global type for index %d not found", idx)
		}
		exports.Globals[name] = globalType
	}

	return exports, nil
}

func readModuleHeader(moduleBytes []byte) ([]byte, error) {
	if len(moduleBytes) < 8 {
		return nil, fmt.Errorf("module too short: %d bytes", len(moduleBytes))
	}

	// Verify magic number (0x00 0x61 0x73 0x6D)
	if !bytes.Equal(moduleBytes[0:4], []byte{0x00, 0x61, 0x73, 0x6D}) {
		return nil, fmt.Errorf("invalid magic number")
	}

	// Verify version (0x01 0x00 0x00 0x00 for core modules)
	if !bytes.Equal(moduleBytes[4:8], []byte{0x01, 0x00, 0x00, 0x00}) {
		return nil, fmt.Errorf("invalid version")
	}

	return moduleBytes[8:], nil
}
