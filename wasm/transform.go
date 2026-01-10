package wasm

import (
	"bytes"
	"fmt"
	"io"
)

// TransformBlankImportNames transforms a WebAssembly core module by replacing
// any imports with blank names to "$$BLANK$$".
// Takes a WebAssembly binary module as input and returns the transformed module.
func TransformBlankImportNames(moduleBytes []byte) ([]byte, error) {
	_, err := readModuleHeader(moduleBytes)
	if err != nil {
		return nil, err
	}

	// Create output buffer with preamble
	output := bytes.NewBuffer(make([]byte, 0, len(moduleBytes)+1024))
	output.Write(moduleBytes[0:8]) // Copy magic and version

	// Parse and transform sections
	pos := 8
	for pos < len(moduleBytes) {
		if pos >= len(moduleBytes) {
			break
		}

		// Read section ID
		sectionID := moduleBytes[pos]
		pos++

		// Read section size (LEB128)
		sectionSize, bytesRead, err := readLEB128(moduleBytes[pos:])
		if err != nil {
			return nil, fmt.Errorf("failed to read section size: %w", err)
		}
		pos += bytesRead

		// Check bounds
		if pos+int(sectionSize) > len(moduleBytes) {
			return nil, fmt.Errorf("section size exceeds module bounds")
		}

		// Extract section data
		sectionData := moduleBytes[pos : pos+int(sectionSize)]
		pos += int(sectionSize)

		// Transform import section (section ID 2)
		if sectionID == 2 {
			transformedData, err := transformImportSection(sectionData)
			if err != nil {
				return nil, fmt.Errorf("failed to transform import section: %w", err)
			}

			// Write section ID
			output.WriteByte(sectionID)

			// Write new section size
			writeLEB128(output, uint32(len(transformedData)))

			// Write transformed data
			output.Write(transformedData)
		} else {
			// Copy section as-is
			output.WriteByte(sectionID)
			writeLEB128(output, uint32(len(sectionData)))
			output.Write(sectionData)
		}
	}

	return output.Bytes(), nil
}

// transformImportSection transforms the import section by replacing blank import names
func transformImportSection(sectionData []byte) ([]byte, error) {
	reader := bytes.NewReader(sectionData)
	output := bytes.NewBuffer(make([]byte, 0, len(sectionData)+1024))

	// Read vector count
	count, err := readLEB128FromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read import count: %w", err)
	}

	// Write count
	writeLEB128(output, count)

	// Process each import
	for i := uint32(0); i < count; i++ {
		// Read module name
		moduleName, err := readName(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read module name for import %d: %w", i, err)
		}

		// Read field name
		fieldName, err := readName(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read field name for import %d: %w", i, err)
		}

		// Read import descriptor type and data
		descType, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read descriptor type for import %d: %w", i, err)
		}

		var descData []byte
		switch descType {
		case 0x00: // function
			// Read typeidx (u32)
			idx, err := readLEB128FromReader(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read type index: %w", err)
			}
			var t bytes.Buffer
			writeLEB128(&t, idx)
			descData = t.Bytes()
		case 0x01: // table
			// Read table type: reftype then limits
			limitsData, _, err := readTableType(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read table type: %w", err)
			}
			descData = limitsData
		case 0x02: // memory
			// Read memory type: limits
			limitsData, _, err := readLimits(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read memory type: %w", err)
			}
			descData = limitsData
		case 0x03: // global
			// Read global type: valtype then mut
			globalData, err := readGlobalType(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read global type: %w", err)
			}
			descData = globalData
		default:
			return nil, fmt.Errorf("unknown import descriptor type: 0x%02x", descType)
		}

		if moduleName == "" {
			moduleName = "$$BLANK$$"
		}

		// Write module name
		writeName(output, moduleName)

		// Write field name (possibly replaced)
		writeName(output, fieldName)

		// Write descriptor
		output.WriteByte(descType)
		output.Write(descData)
	}

	return output.Bytes(), nil
}

// readTableType reads a table type: reftype then limits
func readTableType(r io.ByteReader) ([]byte, *TableType, error) {
	// Read reftype (encoded as a type, which can be multi-byte)
	reftypeData, typ, err := readType(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read reftype: %w", err)
	}

	// Read limits
	limitsData, limits, err := readLimits(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read limits: %w", err)
	}

	// Combine reftype and limits
	result := make([]byte, 0, len(reftypeData)+len(limitsData))
	result = append(result, reftypeData...)
	result = append(result, limitsData...)
	return result, &TableType{
		ElemType: typ.(RefType),
		Limits:   limits,
	}, nil
}

// readLimits reads limits: flag byte, min (u32), optional max (u32)
func readLimits(r io.ByteReader) ([]byte, *Limits, error) {
	// Read flag
	flag, err := r.ReadByte()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read limits flag: %w", err)
	}

	var limits Limits

	// Read min
	min, err := readLEB128FromReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read min: %w", err)
	}
	limits.Min = min

	result := make([]byte, 0, 10)
	result = append(result, flag)

	var t bytes.Buffer
	writeLEB128(&t, min)
	result = append(result, t.Bytes()...)

	// Read max if flag is 0x01
	if flag == 0x01 {
		limits.HasMax = true
		max, err := readLEB128FromReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read max: %w", err)
		}
		limits.Max = max
		var t bytes.Buffer
		writeLEB128(&t, max)
		result = append(result, t.Bytes()...)
	}

	return result, &limits, nil
}

// readGlobalType reads a global type: valtype then mut
func readGlobalType(r io.ByteReader) ([]byte, error) {
	// Read valtype (can be multi-byte)
	valtypeData, _, err := readType(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read valtype: %w", err)
	}

	// Read mut
	mut, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read mut: %w", err)
	}

	result := make([]byte, 0, len(valtypeData)+1)
	result = append(result, valtypeData...)
	result = append(result, mut)
	return result, nil
}

func readType(r io.ByteReader) ([]byte, Type, error) {

	// Read first byte
	firstByte, err := r.ReadByte()
	if err != nil {
		return nil, nil, err
	}
	switch firstByte {
	case 0x7F:
		return []byte{0x7F}, I32{}, nil
	case 0x7E:
		return []byte{0x7E}, I64{}, nil
	case 0x7D:
		return []byte{0x7D}, F32{}, nil
	case 0x7C:
		return []byte{0x7C}, F64{}, nil
	case 0x7B:
		return []byte{0x7B}, V128{}, nil
	case 0x70: // funcref
		return []byte{0x70}, FuncRef{}, nil
	case 0x6F: // externref
		return []byte{0x6F}, ExternRef{}, nil
	default:
		return nil, nil, fmt.Errorf("unknown type byte: 0x%02x", firstByte)
	}
}

// readName reads a name (length-prefixed string) from a reader
func readName(r io.ByteReader) (string, error) {
	length, err := readLEB128FromReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to read name length: %w", err)
	}

	bytes := make([]byte, length)
	for i := uint32(0); i < length; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return "", fmt.Errorf("failed to read name byte: %w", err)
		}
		bytes[i] = b
	}

	return string(bytes), nil
}

// writeName writes a name (length-prefixed string) to a buffer
func writeName(buf *bytes.Buffer, name string) {
	writeLEB128(buf, uint32(len(name)))
	buf.WriteString(name)
}

// readLEB128 reads an unsigned LEB128-encoded value from a byte slice
func readLEB128(data []byte) (uint32, int, error) {
	var result uint32
	var shift uint
	var bytesRead int

	for bytesRead = 0; bytesRead < len(data); bytesRead++ {
		b := data[bytesRead]
		result |= uint32(b&0x7F) << shift

		if b&0x80 == 0 {
			return result, bytesRead + 1, nil
		}

		shift += 7
		if shift >= 35 {
			return 0, 0, fmt.Errorf("LEB128 value too large")
		}
	}

	return 0, 0, fmt.Errorf("unexpected end of LEB128 data")
}

// readLEB128FromReader reads an unsigned LEB128-encoded value from a reader
func readLEB128FromReader(r io.ByteReader) (uint32, error) {
	var result uint32
	var shift uint

	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		result |= uint32(b&0x7F) << shift

		if b&0x80 == 0 {
			return result, nil
		}

		shift += 7
		if shift >= 35 {
			return 0, fmt.Errorf("LEB128 value too large")
		}
	}
}
