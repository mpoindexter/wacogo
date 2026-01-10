package wasm

import (
	"bytes"
	"fmt"
)

func ReadTableExports(mod []byte) (map[string]*TableType, error) {
	var err error
	mod, err = readModuleHeader(mod)
	if err != nil {
		return nil, err
	}

	exportedTables := make(map[string]uint32)
	tableTypes := make(map[uint32]*TableType)

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
		case 4:
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read table count: %w", err)
			}

			// Process each table
			for i := uint32(0); i < count; i++ {
				// Read table type
				_, tableType, err := readTableType(reader)
				if err != nil {
					return nil, fmt.Errorf("failed to read table type: %w", err)
				}

				tableTypes[i] = tableType
			}
		case 7:
			reader := bytes.NewReader(sectionData)

			// Read count
			count, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read export count: %w", err)
			}

			// Process each export
			for i := uint32(0); i < count; i++ {
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
					// For simplicity, we assume the table type is known or fixed.
					// In a full implementation, we would look up the table type by index.
					exportedTables[name] = idx
				case 0x02: // memory
					// Skip memory index
					_, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip memory export index: %w", err)
					}
				case 0x03: // global
					// Skip global index
					_, err := readLEB128FromReader(reader)
					if err != nil {
						return nil, fmt.Errorf("failed to skip global export index: %w", err)
					}
				default:
					return nil, fmt.Errorf("unknown export kind: %d", kind)
				}
			}
		}
	}

	// Build final export map
	exports := make(map[string]*TableType)
	for name, idx := range exportedTables {
		tableType, ok := tableTypes[idx]
		if !ok {
			return nil, fmt.Errorf("table type for index %d not found", idx)
		}
		exports[name] = tableType
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
