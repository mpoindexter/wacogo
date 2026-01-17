package parser

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/partite-ai/wacogo/ast"
)

// Parser reads and parses WebAssembly Component Model binary format
type Parser struct {
	reader *bufio.Reader
}

// NewParser creates a new parser for the given reader
func NewParser(r io.Reader) *Parser {
	return &Parser{
		reader: bufio.NewReader(r),
	}
}

// ParseComponent parses a complete component from the binary data
func (p *Parser) ParseComponent() (*ast.Component, error) {
	// Parse preamble
	if err := p.parsePreamble(); err != nil {
		return nil, fmt.Errorf("failed to parse preamble: %w", err)
	}

	component := &ast.Component{
		Definitions: []ast.Definition{},
	}

	// Parse sections
	for {
		sectionID, err := p.peekByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to peek section ID: %w", err)
		}

		definitions, err := p.parseSection()
		if err != nil {
			return nil, fmt.Errorf("failed to parse section %d: %w", sectionID, err)
		}

		component.Definitions = append(component.Definitions, definitions...)
	}

	return component, nil
}

// parsePreamble parses the component preamble (magic, version, layer)
func (p *Parser) parsePreamble() error {
	// Magic: 0x00 0x61 0x73 0x6D
	magic := []byte{0x00, 0x61, 0x73, 0x6D}
	for i, b := range magic {
		got, err := p.readByte()
		if err != nil {
			return fmt.Errorf("failed to read magic byte %d: %w", i, err)
		}
		if got != b {
			return fmt.Errorf("invalid magic byte %d: expected 0x%02x, got 0x%02x", i, b, got)
		}
	}

	// Version: 0x0d 0x00
	version := []byte{0x0d, 0x00}
	for i, b := range version {
		got, err := p.readByte()
		if err != nil {
			return fmt.Errorf("failed to read version byte %d: %w", i, err)
		}
		if got != b {
			return fmt.Errorf("invalid version byte %d: expected 0x%02x, got 0x%02x", i, b, got)
		}
	}

	// Layer: 0x01 0x00
	layer := []byte{0x01, 0x00}
	for i, b := range layer {
		got, err := p.readByte()
		if err != nil {
			return fmt.Errorf("failed to read layer byte %d: %w", i, err)
		}
		if got != b {
			return fmt.Errorf("invalid layer byte %d: expected 0x%02x, got 0x%02x", i, b, got)
		}
	}

	return nil
}

// parseSection parses a single section and returns any definitions
func (p *Parser) parseSection() ([]ast.Definition, error) {
	sectionID, err := p.readByte()
	if err != nil {
		return nil, err
	}

	size, err := p.readU32()
	if err != nil {
		return nil, fmt.Errorf("failed to read section size: %w", err)
	}

	// Read the entire section into a buffer to ensure we don't read beyond section boundaries
	sectionData, err := p.readBytes(int(size))
	if err != nil {
		return nil, fmt.Errorf("failed to read section data: %w", err)
	}

	// Create a new parser for this section with the limited data
	sectionParser := NewParser(bytes.NewReader(sectionData))

	var definitions []ast.Definition

	switch sectionID {
	case 0:
		// Custom section - already read and can skip
		return definitions, nil
	case 1:
		// Core module section
		defs, err := sectionParser.parseCoreModuleSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 2:
		// Core instance section
		defs, err := sectionParser.parseCoreInstanceSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 3:
		// Core type section
		defs, err := sectionParser.parseCoreTypeSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 4:
		// Component section
		defs, err := sectionParser.parseComponentSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 5:
		// Instance section
		defs, err := sectionParser.parseInstanceSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 6:
		// Alias section
		defs, err := sectionParser.parseAliasSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 7:
		// Type section
		defs, err := sectionParser.parseTypeSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 8:
		// Canon section
		defs, err := sectionParser.parseCanonSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 9:
		// Start section
		def, err := sectionParser.parseStartSection()
		if err != nil {
			return nil, err
		}
		if def != nil {
			definitions = append(definitions, def)
		}
	case 10:
		// Import section
		defs, err := sectionParser.parseImportSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	case 11:
		// Export section
		defs, err := sectionParser.parseExportSection()
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	default:
		return nil, fmt.Errorf("unknown section ID: %d", sectionID)
	}

	return definitions, nil
}

// Section parsing stubs - to be implemented later

func (p *Parser) parseCoreModuleSection() ([]ast.Definition, error) {
	// Section 1 contains a single complete core module (not a vector)
	// Multiple modules means multiple section 1 entries
	module, err := p.parseCoreModule()
	if err != nil {
		return nil, err
	}
	return []ast.Definition{module}, nil
}

func (p *Parser) parseCoreModule() (*ast.CoreModule, error) {
	// The section data IS the complete core module (including magic bytes)
	// Read all remaining data from the parser (which is limited to section size)
	moduleBytes, err := io.ReadAll(p.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read module bytes: %w", err)
	}

	module := &ast.CoreModule{
		Raw: moduleBytes,
	}

	return module, nil
}

func (p *Parser) parseCoreInstanceSection() ([]ast.Definition, error) {
	var instances []ast.Definition

	err := p.readVec(func() error {
		instance, err := p.parseCoreInstance()
		if err != nil {
			return err
		}
		instances = append(instances, instance)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (p *Parser) parseCoreInstance() (*ast.CoreInstance, error) {
	expr, err := p.parseCoreInstanceExpr()
	if err != nil {
		return nil, err
	}

	return &ast.CoreInstance{
		Expr: expr,
	}, nil
}

func (p *Parser) parseCoreInstanceExpr() (ast.CoreInstanceExpr, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// instantiate module with args
		moduleIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		var args []ast.CoreInstantiateArg
		err = p.readVec(func() error {
			arg, err := p.parseCoreInstantiateArg()
			if err != nil {
				return err
			}
			args = append(args, arg)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return &ast.CoreInstantiate{
			ModuleIdx: moduleIdx,
			Args:      args,
		}, nil

	case 0x01:
		// inline exports
		var exports []ast.CoreInlineExport
		err = p.readVec(func() error {
			export, err := p.parseCoreInlineExport()
			if err != nil {
				return err
			}
			exports = append(exports, export)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return &ast.CoreInlineExports{
			Exports: exports,
		}, nil

	default:
		return nil, fmt.Errorf("invalid core instance expr discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseCoreInstantiateArg() (ast.CoreInstantiateArg, error) {
	name, err := p.readName()
	if err != nil {
		return ast.CoreInstantiateArg{}, err
	}

	// Read sort (should be 0x12 for instance)
	sortByte, err := p.readByte()
	if err != nil {
		return ast.CoreInstantiateArg{}, err
	}
	if sortByte != 0x12 {
		return ast.CoreInstantiateArg{}, fmt.Errorf("expected instance sort 0x12, got 0x%02x", sortByte)
	}

	instanceIdx, err := p.readU32()
	if err != nil {
		return ast.CoreInstantiateArg{}, err
	}

	return ast.CoreInstantiateArg{
		Name:            name,
		CoreInstanceIdx: instanceIdx,
	}, nil
}

func (p *Parser) parseCoreInlineExport() (ast.CoreInlineExport, error) {
	name, err := p.readName()
	if err != nil {
		return ast.CoreInlineExport{}, err
	}

	sortIdx, err := p.parseCoreSortIdx()
	if err != nil {
		return ast.CoreInlineExport{}, err
	}

	return ast.CoreInlineExport{
		Name:    name,
		SortIdx: sortIdx,
	}, nil
}

func (p *Parser) parseCoreSortIdx() (ast.CoreSortIdx, error) {
	sortByte, err := p.readByte()
	if err != nil {
		return ast.CoreSortIdx{}, err
	}

	var sort ast.CoreSort
	switch sortByte {
	case 0x00:
		sort = ast.CoreSortFunc
	case 0x01:
		sort = ast.CoreSortTable
	case 0x02:
		sort = ast.CoreSortMemory
	case 0x03:
		sort = ast.CoreSortGlobal
	case 0x10:
		sort = ast.CoreSortType
	case 0x11:
		sort = ast.CoreSortModule
	case 0x12:
		sort = ast.CoreSortInstance
	default:
		return ast.CoreSortIdx{}, fmt.Errorf("invalid core sort: 0x%02x", sortByte)
	}

	idx, err := p.readU32()
	if err != nil {
		return ast.CoreSortIdx{}, err
	}

	return ast.CoreSortIdx{
		Sort: sort,
		Idx:  idx,
	}, nil
}

func (p *Parser) parseCoreTypeSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		typ, err := p.parseCoreType()
		if err != nil {
			return err
		}
		definitions = append(definitions, typ)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseCoreType() (*ast.CoreType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x4E:
		if _, err := p.readByte(); err != nil {
			return nil, err
		}
		listSize, err := p.readU32()
		if err != nil {
			return nil, err
		}
		var subTypes []ast.CoreSubType
		for i := uint32(0); i < listSize; i++ {
			st, err := p.parseSubType()
			if err != nil {
				return nil, err
			}
			subTypes = append(subTypes, st)
		}
		return &ast.CoreType{
			DefType: &ast.CoreRecType{
				SubTypes: subTypes,
			},
		}, nil
	case 0x50:
		// Core module type
		if _, err := p.readByte(); err != nil {
			return nil, err
		}

		nDecls, err := p.readU32()
		if err != nil {
			return nil, err
		}

		var modType ast.CoreModuleType
		for range nDecls {
			discriminator, err := p.readByte()
			if err != nil {
				return nil, err
			}

			switch discriminator {
			case 0x00:
				// core import
				decl, err := p.parseCoreImportDecl()
				if err != nil {
					return nil, err
				}
				modType.Declarations = append(modType.Declarations, decl)
			case 0x01:
				// core type
				typ, err := p.parseCoreType()
				if err != nil {
					return nil, err
				}
				modType.Declarations = append(modType.Declarations, &ast.CoreTypeDecl{
					Type: typ,
				})
			case 0x02:
				// core alias
				sortByte, err := p.readByte()
				if err != nil {
					return nil, err
				}

				var sort ast.CoreSort
				switch sortByte {
				case 0x00:
					sort = ast.CoreSortFunc
				case 0x01:
					sort = ast.CoreSortTable
				case 0x02:
					sort = ast.CoreSortMemory
				case 0x03:
					sort = ast.CoreSortGlobal
				case 0x10:
					sort = ast.CoreSortType
				case 0x11:
					sort = ast.CoreSortModule
				case 0x12:
					sort = ast.CoreSortInstance
				default:
					return nil, fmt.Errorf("invalid core sort: 0x%02x", sortByte)
				}

				aliasType, err := p.readU32()
				if err != nil {
					return nil, err
				}

				if aliasType != 0x01 {
					return nil, fmt.Errorf("unsupported core alias type: 0x%02x", aliasType)
				}

				count, err := p.readU32()
				if err != nil {
					return nil, err
				}

				idx, err := p.readU32()
				if err != nil {
					return nil, err
				}

				modType.Declarations = append(modType.Declarations, &ast.CoreAliasDecl{
					Sort: sort,
					Target: &ast.CoreOuterAlias{
						Count: count,
						Idx:   idx,
					},
				})

			case 0x03:
				// core export
				name, err := p.readName()
				if err != nil {
					return nil, err
				}
				desc, err := p.parseCoreImportDesc()
				if err != nil {
					return nil, err
				}

				modType.Declarations = append(modType.Declarations, &ast.CoreExportDecl{
					Name: name,
					Desc: desc,
				})
			default:
				return nil, fmt.Errorf("unknown core module type decl discriminator: 0x%02x", discriminator)
			}
		}
		return &ast.CoreType{
			DefType: &modType,
		}, nil

	default:
		subtype, err := p.parseSubType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreType{
			DefType: &ast.CoreRecType{
				SubTypes: []ast.CoreSubType{
					subtype,
				},
			},
		}, nil
	}
}

func (p *Parser) parseSubType() (ast.CoreSubType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return ast.CoreSubType{}, err
	}
	switch discriminator {
	case 0x4F:
		if _, err := p.readByte(); err != nil {
			return ast.CoreSubType{}, err
		}
		nSuperTypes, err := p.readU32()
		if err != nil {
			return ast.CoreSubType{}, err
		}
		var superTypes []uint32
		for i := uint32(0); i < nSuperTypes; i++ {
			superTypeIdx, err := p.readU32()
			if err != nil {
				return ast.CoreSubType{}, err
			}
			superTypes = append(superTypes, superTypeIdx)
		}
		typ, err := p.parseCoreCompositeType()
		if err != nil {
			return ast.CoreSubType{}, err
		}
		return ast.CoreSubType{
			Final:      true,
			Supertypes: superTypes,
			Type:       typ,
		}, nil
	case 0x50:
		if _, err := p.readByte(); err != nil {
			return ast.CoreSubType{}, err
		}
		nSuperTypes, err := p.readU32()
		if err != nil {
			return ast.CoreSubType{}, err
		}
		var superTypes []uint32
		for i := uint32(0); i < nSuperTypes; i++ {
			superTypeIdx, err := p.readU32()
			if err != nil {
				return ast.CoreSubType{}, err
			}
			superTypes = append(superTypes, superTypeIdx)
		}
		typ, err := p.parseCoreCompositeType()
		if err != nil {
			return ast.CoreSubType{}, err
		}
		return ast.CoreSubType{
			Supertypes: superTypes,
			Type:       typ,
		}, nil
	default:
		typ, err := p.parseCoreCompositeType()
		if err != nil {
			return ast.CoreSubType{}, err
		}
		return ast.CoreSubType{
			Final: true,
			Type:  typ,
		}, nil
	}

}

func (p *Parser) parseCoreCompositeType() (ast.CoreCompType, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x60:
		// function type
		paramTypes, err := p.parseCoreResultType()
		if err != nil {
			return nil, err
		}
		resultTypes, err := p.parseCoreResultType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreFuncType{
			Params: ast.CoreResultType{
				Types: paramTypes,
			},
			Results: ast.CoreResultType{
				Types: resultTypes,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported core composite type discriminator: 0x%02x", discriminator)
	}

}

func (p *Parser) parseCoreResultType() ([]ast.CoreValType, error) {
	var types []ast.CoreValType

	err := p.readVec(func() error {
		valType, err := p.parseCoreValType()
		if err != nil {
			return err
		}
		types = append(types, valType)
		return nil
	})

	return types, err
}

func (p *Parser) parseCoreValType() (ast.CoreValType, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x7f:
		return ast.CoreNumTypeI32, nil
	case 0x7e:
		return ast.CoreNumTypeI64, nil
	case 0x7d:
		return ast.CoreNumTypeF32, nil
	case 0x7c:
		return ast.CoreNumTypeF64, nil
	case 0x7b:
		return ast.CoreVecTypeV128, nil
	case 0x70:
		// funcref
		return &ast.CoreRefType{
			Nullable: true,
			HeapType: ast.CoreAbsHeapTypeFunc,
		}, nil
	case 0x6f:
		// externref
		return &ast.CoreRefType{
			Nullable: true,
			HeapType: ast.CoreAbsHeapTypeExtern,
		}, nil
	default:
		return nil, fmt.Errorf("invalid core value type: 0x%02x", discriminator)
	}
}

func (p *Parser) parseCoreRefType() (*ast.CoreRefType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x63:
		if _, err := p.readByte(); err != nil {
			return nil, err
		}
		ht, err := p.parseCoreHeapType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreRefType{
			Nullable: true,
			HeapType: ht,
		}, nil

	case 0x64:
		if _, err := p.readByte(); err != nil {
			return nil, err
		}
		ht, err := p.parseCoreHeapType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreRefType{
			Nullable: false,
			HeapType: ht,
		}, nil
	default:
		ht, err := p.parseAbsCoreHeapType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreRefType{
			Nullable: false,
			HeapType: ht,
		}, nil
	}
}

func (p *Parser) parseCoreHeapType() (ast.CoreHeapType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x69, 0x6A, 0x6B, 0x6C, 0x6D, 0x6E, 0x6F, 0x70, 0x71, 0x72, 0x73, 0x74:
		if _, err := p.readByte(); err != nil {
			return nil, err
		}
		return p.translateAbsCoreHeapType(discriminator)
	default:
		idx, err := p.readS32()
		if err != nil {
			return nil, err
		}
		return &ast.CoreConcreteHeapType{
			TypeIdx: uint32(idx),
		}, nil
	}
}

func (p *Parser) parseAbsCoreHeapType() (ast.CoreAbsHeapType, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return 0, err
	}

	return p.translateAbsCoreHeapType(discriminator)
}

func (p *Parser) translateAbsCoreHeapType(b byte) (ast.CoreAbsHeapType, error) {
	switch b {
	case 0x69:
		return ast.CoreAbsHeapTypeExn, nil
	case 0x6A:
		return ast.CoreAbsHeapTypeArray, nil
	case 0x6B:
		return ast.CoreAbsHeapTypeStruct, nil
	case 0x6C:
		return ast.CoreAbsHeapTypeI31, nil
	case 0x6D:
		return ast.CoreAbsHeapTypeEq, nil
	case 0x6E:
		return ast.CoreAbsHeapTypeAny, nil
	case 0x6F:
		return ast.CoreAbsHeapTypeExtern, nil
	case 0x70:
		return ast.CoreAbsHeapTypeFunc, nil
	case 0x71:
		return ast.CoreAbsHeapTypeNone, nil
	case 0x72:
		return ast.CoreAbsHeapTypeNoExtern, nil
	case 0x73:
		return ast.CoreAbsHeapTypeNoFunc, nil
	case 0x74:
		return ast.CoreAbsHeapTypeNoExn, nil
	default:
		return 0, fmt.Errorf("invalid abstract core heap type: 0x%02x", b)
	}
}

func (p *Parser) parseCoreImportDecl() (*ast.CoreImportDecl, error) {
	modName, err := p.readName()
	if err != nil {
		return nil, err
	}

	name, err := p.readName()
	if err != nil {
		return nil, err
	}

	desc, err := p.parseCoreImportDesc()
	if err != nil {
		return nil, err
	}

	return &ast.CoreImportDecl{
		Module: modName,
		Name:   name,
		Desc:   desc,
	}, nil
}

func (p *Parser) parseCoreExportDecl() (*ast.CoreExportDecl, error) {
	name, err := p.readName()
	if err != nil {
		return nil, err
	}

	desc, err := p.parseCoreImportDesc()
	if err != nil {
		return nil, err
	}

	return &ast.CoreExportDecl{
		Name: name,
		Desc: desc,
	}, nil
}

func (p *Parser) parseCoreLimits() (ast.CoreLimits, error) {
	disc, err := p.readByte()
	if err != nil {
		return ast.CoreLimits{}, err
	}

	switch disc {
	case 0x00:
		// only min
		min, err := p.readU32()
		if err != nil {
			return ast.CoreLimits{}, err
		}
		return ast.CoreLimits{
			Min: min,
		}, nil
	case 0x01:
		// min and max
		min, err := p.readU32()
		if err != nil {
			return ast.CoreLimits{}, err
		}
		max, err := p.readU32()
		if err != nil {
			return ast.CoreLimits{}, err
		}
		return ast.CoreLimits{
			Min: min,
			Max: &max,
		}, nil
	default:
		return ast.CoreLimits{}, fmt.Errorf("invalid core limits discriminator: 0x%02x", disc)
	}
}

func (p *Parser) parseCoreImportDesc() (ast.CoreImportDesc, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// function
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.CoreFuncImport{
			TypeIdx: typeIdx,
		}, nil
	case 0x01:
		// table
		refType, err := p.parseCoreRefType()
		if err != nil {
			return nil, err
		}

		limits, err := p.parseCoreLimits()
		if err != nil {
			return nil, err
		}

		return &ast.CoreTableImport{
			Type: ast.CoreTableType{
				ElemType: refType,
				Limits:   limits,
			},
		}, nil
	case 0x02:
		// memory
		limits, err := p.parseCoreLimits()
		if err != nil {
			return nil, err
		}

		return &ast.CoreMemoryImport{
			Type: ast.CoreMemType{
				Limits: limits,
			},
		}, nil
	case 0x03:
		// global
		valType, err := p.parseCoreValType()
		if err != nil {
			return nil, err
		}

		mut, err := p.readByte()
		if err != nil {
			return nil, err
		}

		mutable := ast.CoreConst
		if mut == 0x01 {
			mutable = ast.CoreVar
		} else if mut != 0x00 {
			return nil, fmt.Errorf("invalid global mutability: 0x%02x", mut)
		}

		return &ast.CoreGlobalImport{
			Type: ast.CoreGlobalType{
				Val: valType,
				Mut: mutable,
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid core import desc discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseComponentSection() ([]ast.Definition, error) {
	// Section 4 contains a single complete component (not a vector)
	// Multiple components means multiple section 4 entries
	component, err := p.parseNestedComponent()
	if err != nil {
		return nil, err
	}
	return []ast.Definition{component}, nil
}

func (p *Parser) parseNestedComponent() (*ast.NestedComponent, error) {
	// The section data IS the complete component (including preamble)
	// Read all remaining data from the parser (which is limited to section size)
	componentData, err := io.ReadAll(p.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read nested component: %w", err)
	}

	// Create a new parser for the nested component
	nestedParser := NewParser(bytes.NewReader(componentData))

	// Recursively parse the nested component
	nestedComp, err := nestedParser.ParseComponent()
	if err != nil {
		return nil, fmt.Errorf("parsing nested component: %w", err)
	}

	return &ast.NestedComponent{
		Component: nestedComp,
	}, nil
}

func (p *Parser) parseInstanceSection() ([]ast.Definition, error) {
	var instances []ast.Definition

	err := p.readVec(func() error {
		instance, err := p.parseInstance()
		if err != nil {
			return err
		}
		instances = append(instances, instance)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (p *Parser) parseInstance() (*ast.Instance, error) {
	expr, err := p.parseInstanceExpr()
	if err != nil {
		return nil, err
	}

	return &ast.Instance{
		Expr: expr,
	}, nil
}

func (p *Parser) parseInstanceExpr() (ast.InstanceExpr, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// instantiate component with args
		componentIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		var args []ast.InstantiateArg
		err = p.readVec(func() error {
			arg, err := p.parseInstantiateArg()
			if err != nil {
				return err
			}
			args = append(args, arg)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return &ast.Instantiate{
			ComponentIdx: componentIdx,
			Args:         args,
		}, nil

	case 0x01:
		// inline exports
		var exports []ast.InlineExport
		err = p.readVec(func() error {
			export, err := p.parseInlineExport()
			if err != nil {
				return err
			}
			exports = append(exports, export)
			return nil
		})
		if err != nil {
			return nil, err
		}

		return &ast.InlineExports{
			Exports: exports,
		}, nil

	default:
		return nil, fmt.Errorf("invalid instance expr discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseInstantiateArg() (ast.InstantiateArg, error) {
	name, err := p.readName()
	if err != nil {
		return ast.InstantiateArg{}, err
	}

	sortIdx, err := p.parseSortIdx()
	if err != nil {
		return ast.InstantiateArg{}, err
	}

	return ast.InstantiateArg{
		Name: name,
		SortIdx: &ast.SortIdx{
			Sort: sortIdx.Sort,
			Idx:  sortIdx.Idx,
		},
	}, nil
}

func (p *Parser) parseInlineExport() (ast.InlineExport, error) {
	name, err := p.readExportName()
	if err != nil {
		return ast.InlineExport{}, err
	}

	sortIdx, err := p.parseSortIdx()
	if err != nil {
		return ast.InlineExport{}, err
	}

	return ast.InlineExport{
		Name:    name,
		SortIdx: sortIdx,
	}, nil
}

func (p *Parser) parseAliasSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		alias, err := p.parseAlias()
		if err != nil {
			return err
		}
		definitions = append(definitions, alias)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseAlias() (*ast.Alias, error) {
	// Read sort
	sort, err := p.parseSort()
	if err != nil {
		return nil, fmt.Errorf("failed to parse alias sort: %w", err)
	}

	// Read alias target
	target, err := p.parseAliasTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to parse alias target: %w", err)
	}

	return &ast.Alias{
		Target: target,
		Sort:   sort,
	}, nil
}

func (p *Parser) parseAliasTarget() (ast.AliasTarget, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// export i n
		instanceIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		name, err := p.readName()
		if err != nil {
			return nil, err
		}
		return &ast.ExportAlias{
			InstanceIdx: instanceIdx,
			Name:        name,
		}, nil

	case 0x01:
		// core export i n
		instanceIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		name, err := p.readName()
		if err != nil {
			return nil, err
		}
		return &ast.CoreExportAlias{
			InstanceIdx: instanceIdx,
			Name:        name,
		}, nil

	case 0x02:
		// outer ct idx
		count, err := p.readU32()
		if err != nil {
			return nil, err
		}
		idx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.OuterAlias{
			Count: count,
			Idx:   idx,
		}, nil

	default:
		return nil, fmt.Errorf("invalid alias target discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseTypeSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		typ, err := p.parseType()
		if err != nil {
			return err
		}
		definitions = append(definitions, typ)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseType() (*ast.Type, error) {
	defType, err := p.parseDefType()
	if err != nil {
		return nil, err
	}

	return &ast.Type{
		DefType: defType,
	}, nil
}

func (p *Parser) parseDefType() (ast.DefType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	// Check if it's a primitive value type or a type constructor
	// Type constructors are in the range 0x3f to 0x7f
	if discriminator >= 0x3f && discriminator <= 0x7f {
		// It's a type constructor
		return p.parseTypeConstructor()
	}

	// It's a type index
	typeIdx, err := p.readU32()
	if err != nil {
		return nil, err
	}
	return &ast.TypeIdx{Idx: typeIdx}, nil
}

func (p *Parser) parseTypeConstructor() (ast.DefType, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x7f:
		return &ast.BoolType{}, nil
	case 0x7e:
		return &ast.S8Type{}, nil
	case 0x7d:
		return &ast.U8Type{}, nil
	case 0x7c:
		return &ast.S16Type{}, nil
	case 0x7b:
		return &ast.U16Type{}, nil
	case 0x7a:
		return &ast.S32Type{}, nil
	case 0x79:
		return &ast.U32Type{}, nil
	case 0x78:
		return &ast.S64Type{}, nil
	case 0x77:
		return &ast.U64Type{}, nil
	case 0x76:
		return &ast.F32Type{}, nil
	case 0x75:
		return &ast.F64Type{}, nil
	case 0x74:
		return &ast.CharType{}, nil
	case 0x73:
		return &ast.StringType{}, nil

	case 0x72:
		// record type
		var fields []ast.RecordField
		err := p.readVec(func() error {
			label, err := p.readName()
			if err != nil {
				return err
			}
			valType, err := p.parseValType()
			if err != nil {
				return err
			}
			fields = append(fields, ast.RecordField{
				Label: label,
				Type:  valType,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.RecordType{Fields: fields}, nil

	case 0x71:
		// variant type
		var cases []ast.VariantCase
		err := p.readVec(func() error {
			label, err := p.readName()
			if err != nil {
				return err
			}
			// Check for optional type
			hasType, err := p.readByte()
			if err != nil {
				return err
			}
			var valType ast.DefValType
			if hasType == 0x01 {
				valType, err = p.parseValType()
				if err != nil {
					return err
				}
			}
			// Read the trailing 0x00
			trailing, err := p.readByte()
			if err != nil {
				return err
			}
			if trailing != 0x00 {
				return fmt.Errorf("expected trailing 0x00 in variant case, got 0x%02x", trailing)
			}
			cases = append(cases, ast.VariantCase{
				Label: label,
				Type:  valType,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.VariantType{Cases: cases}, nil

	case 0x70:
		// list type
		elemType, err := p.parseValType()
		if err != nil {
			return nil, err
		}
		return &ast.ListType{Element: elemType}, nil

	case 0x67:
		return nil, fmt.Errorf("fixed length list type (0x67) is not yet supported")
		/*
			// fixed-length list type 🔧
			elemType, err := p.parseValType()
			if err != nil {
				return nil, err
			}
			_, err = p.readU32() // length
			if err != nil {
				return nil, err
			}
			// For now, treat as regular list - TODO: extend AST to support fixed-length lists
			return &ast.ListType{Element: elemType}, nil
		*/
	case 0x6f:
		// tuple type
		var types []ast.DefValType
		err := p.readVec(func() error {
			valType, err := p.parseValType()
			if err != nil {
				return err
			}
			types = append(types, valType)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.TupleType{Types: types}, nil

	case 0x6e:
		// flags type
		var labels []string
		err := p.readVec(func() error {
			label, err := p.readName()
			if err != nil {
				return err
			}
			labels = append(labels, label)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.FlagsType{Labels: labels}, nil

	case 0x6d:
		// enum type
		var labels []string
		err := p.readVec(func() error {
			label, err := p.readName()
			if err != nil {
				return err
			}
			labels = append(labels, label)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.EnumType{Labels: labels}, nil

	case 0x6b:
		// option type
		valType, err := p.parseValType()
		if err != nil {
			return nil, err
		}

		return &ast.OptionType{Type: valType}, nil

	case 0x6a:
		// result type
		// Parse optional ok type
		hasOk, err := p.readByte()
		if err != nil {
			return nil, err
		}
		var okType ast.DefValType
		if hasOk == 0x01 {
			okType, err = p.parseValType()
			if err != nil {
				return nil, err
			}
		}
		// Parse optional error type
		hasErr, err := p.readByte()
		if err != nil {
			return nil, err
		}
		var errType ast.DefValType
		if hasErr == 0x01 {
			errType, err = p.parseValType()
			if err != nil {
				return nil, err
			}
		}
		return &ast.ResultType{Ok: okType, Error: errType}, nil

	case 0x69:
		// own type
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.OwnType{TypeIdx: typeIdx}, nil

	case 0x68:
		// borrow type
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.BorrowType{TypeIdx: typeIdx}, nil

	case 0x66:
		// stream type 🔀 (async feature)
		return nil, fmt.Errorf("stream types (0x66) are not yet supported")

	case 0x65:
		// future type 🔀 (async feature)
		return nil, fmt.Errorf("future types (0x65) are not yet supported")

	case 0x64:
		// error-context type 📝
		return nil, fmt.Errorf("error-context types (0x64) are not yet supported")

	case 0x40:
		// func type
		params, err := p.parseParamList()
		if err != nil {
			return nil, err
		}

		results, err := p.parseResultList()
		if err != nil {
			return nil, err
		}

		return &ast.FuncType{
			Params:  params,
			Results: results,
		}, nil

	case 0x43:
		// async func type 🔀
		return nil, fmt.Errorf("async function types (0x43) are not yet supported")

	case 0x41:
		// component type
		var decls []ast.ComponentDecl
		err := p.readVec(func() error {
			decl, err := p.parseComponentDecl()
			if err != nil {
				return err
			}
			decls = append(decls, decl)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.ComponentType{Declarations: decls}, nil

	case 0x42:
		// instance type
		var decls []ast.InstanceDecl
		err := p.readVec(func() error {
			decl, err := p.parseInstanceDecl()
			if err != nil {
				return err
			}
			decls = append(decls, decl)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &ast.InstanceType{Declarations: decls}, nil

	case 0x3f:
		// resource type
		repType, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if repType != 0x7f {
			return nil, fmt.Errorf("expected i32 rep type 0x7f, got 0x%02x", repType)
		}
		// Parse optional destructor
		hasDtor, err := p.readByte()
		if err != nil {
			return nil, err
		}
		var dtor *uint32
		if hasDtor == 0x01 {
			dtorIdx, err := p.readU32()
			if err != nil {
				return nil, err
			}
			dtor = &dtorIdx
		}
		return &ast.ResourceType{Dtor: dtor}, nil

	case 0x3e:
		// async resource type 🚝
		return nil, fmt.Errorf("async resource types (0x3e) are not yet supported")

	default:
		return nil, fmt.Errorf("invalid type constructor: 0x%02x", discriminator)
	}
}

func (p *Parser) parseParamList() ([]ast.FuncParam, error) {
	var params []ast.FuncParam

	err := p.readVec(func() error {
		label, err := p.readName()
		if err != nil {
			return err
		}

		valType, err := p.parseValType()
		if err != nil {
			return err
		}

		params = append(params, ast.FuncParam{
			Label: label,
			Type:  valType,
		})
		return nil
	})

	return params, err
}

func (p *Parser) parseResultList() (ast.DefValType, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// Single result
		return p.parseValType()
	case 0x01:
		// No results (followed by 0x00)
		noByte, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if noByte != 0x00 {
			return nil, fmt.Errorf("expected 0x00 after no-result indicator, got 0x%02x", noByte)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid result list discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseValType() (ast.DefValType, error) {
	discriminator, err := p.peekByte()
	if err != nil {
		return nil, err
	}

	// Check if it's a primitive value type (primvaltype)
	// primvaltype: 0x7f-0x73 and 0x64
	if (discriminator >= 0x73 && discriminator <= 0x7f) || discriminator == 0x64 {
		_, err = p.readByte()
		if err != nil {
			return nil, err
		}

		switch discriminator {
		case 0x7f:
			return &ast.BoolType{}, nil
		case 0x7e:
			return &ast.S8Type{}, nil
		case 0x7d:
			return &ast.U8Type{}, nil
		case 0x7c:
			return &ast.S16Type{}, nil
		case 0x7b:
			return &ast.U16Type{}, nil
		case 0x7a:
			return &ast.S32Type{}, nil
		case 0x79:
			return &ast.U32Type{}, nil
		case 0x78:
			return &ast.S64Type{}, nil
		case 0x77:
			return &ast.U64Type{}, nil
		case 0x76:
			return &ast.F32Type{}, nil
		case 0x75:
			return &ast.F64Type{}, nil
		case 0x74:
			return &ast.CharType{}, nil
		case 0x73:
			return &ast.StringType{}, nil
		case 0x64:
			return nil, fmt.Errorf("error-context types (0x64) are not yet supported")
		default:
			return nil, fmt.Errorf("invalid primitive value type: 0x%02x", discriminator)
		}
	}

	// Otherwise it's a type index
	idx, err := p.readU32()
	if err != nil {
		return nil, err
	}
	return &ast.TypeIdx{Idx: idx}, nil
}

func (p *Parser) parseComponentDecl() (ast.ComponentDecl, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// core type
		typ, err := p.parseCoreType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreTypeDecl{Type: &ast.CoreType{DefType: typ.DefType}}, nil

	case 0x01:
		// type
		typ, err := p.parseType()
		if err != nil {
			return nil, err
		}
		return &ast.TypeDecl{Type: typ}, nil

	case 0x02:
		// alias
		alias, err := p.parseAlias()
		if err != nil {
			return nil, err
		}
		return &ast.AliasDecl{Alias: alias}, nil

	case 0x03:
		// import
		name, err := p.readImportName()
		if err != nil {
			return nil, err
		}
		desc, err := p.parseExternDesc()
		if err != nil {
			return nil, err
		}
		return &ast.ImportDecl{
			ImportName: name,
			Desc:       desc,
		}, nil

	case 0x04:
		// export
		name, err := p.readExportName()
		if err != nil {
			return nil, err
		}
		desc, err := p.parseExternDesc()
		if err != nil {
			return nil, err
		}
		return &ast.ExportDecl{
			ExportName: name,
			Desc:       desc,
		}, nil

	default:
		return nil, fmt.Errorf("invalid component decl discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseInstanceDecl() (ast.InstanceDecl, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// core type
		typ, err := p.parseCoreType()
		if err != nil {
			return nil, err
		}
		return &ast.CoreTypeDecl{Type: &ast.CoreType{DefType: typ.DefType}}, nil

	case 0x01:
		// type
		typ, err := p.parseType()
		if err != nil {
			return nil, err
		}
		return &ast.TypeDecl{Type: typ}, nil

	case 0x02:
		// alias
		alias, err := p.parseAlias()
		if err != nil {
			return nil, err
		}
		return &ast.AliasDecl{Alias: alias}, nil

	case 0x04:
		// export
		name, err := p.readExportName()
		if err != nil {
			return nil, err
		}
		desc, err := p.parseExternDesc()
		if err != nil {
			return nil, err
		}
		return &ast.ExportDecl{
			ExportName: name,
			Desc:       desc,
		}, nil

	default:
		return nil, fmt.Errorf("invalid instance decl discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseCanonSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		canon, err := p.parseCanon()
		if err != nil {
			return err
		}
		definitions = append(definitions, canon)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseCanon() (*ast.Canon, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	var def ast.CanonDef

	switch discriminator {
	case 0x00:
		// canon lift
		sortByte, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if sortByte != 0x00 {
			return nil, fmt.Errorf("expected func sort 0x00, got 0x%02x", sortByte)
		}

		coreFuncIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		opts, err := p.parseCanonOpts()
		if err != nil {
			return nil, err
		}

		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		def = &ast.CanonLift{
			CoreFuncIdx:     coreFuncIdx,
			Options:         opts,
			FunctionTypeIdx: typeIdx,
		}

	case 0x01:
		// canon lower
		sortByte, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if sortByte != 0x00 {
			return nil, fmt.Errorf("expected func sort 0x00, got 0x%02x", sortByte)
		}

		funcIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		opts, err := p.parseCanonOpts()
		if err != nil {
			return nil, err
		}

		def = &ast.CanonLower{
			FuncIdx: funcIdx,
			Options: opts,
		}

	case 0x02:
		// canon resource.new
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		def = &ast.CanonResourceNew{
			TypeIdx: typeIdx,
		}

	case 0x03:
		// canon resource.drop
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		def = &ast.CanonResourceDrop{
			TypeIdx: typeIdx,
		}

	case 0x04:
		// canon resource.rep
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		def = &ast.CanonResourceRep{
			TypeIdx: typeIdx,
		}

	default:
		return nil, fmt.Errorf("invalid canon discriminator: 0x%02x", discriminator)
	}

	return &ast.Canon{
		Def: def,
	}, nil
}

func (p *Parser) parseCanonOpts() ([]ast.CanonOpt, error) {
	var opts []ast.CanonOpt

	err := p.readVec(func() error {
		opt, err := p.parseCanonOpt()
		if err != nil {
			return err
		}
		opts = append(opts, opt)
		return nil
	})

	return opts, err
}

func (p *Parser) parseCanonOpt() (ast.CanonOpt, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		return &ast.StringEncodingOpt{Encoding: ast.StringEncodingUTF8}, nil
	case 0x01:
		return &ast.StringEncodingOpt{Encoding: ast.StringEncodingUTF16}, nil
	case 0x02:
		return &ast.StringEncodingOpt{Encoding: ast.StringEncodingLatin1UTF16}, nil
	case 0x03:
		// memory
		memIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.MemoryOpt{MemoryIdx: memIdx}, nil
	case 0x04:
		// realloc
		funcIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.ReallocOpt{FuncIdx: funcIdx}, nil
	case 0x05:
		// post-return
		funcIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.PostReturnOpt{FuncIdx: funcIdx}, nil
	default:
		return nil, fmt.Errorf("invalid canon option discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseStartSection() (ast.Definition, error) {
	return nil, fmt.Errorf("start section not yet implemented")
}

func (p *Parser) parseImportSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		import_, err := p.parseImport()
		if err != nil {
			return err
		}
		definitions = append(definitions, import_)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseImport() (*ast.Import, error) {
	// Read import name
	name, err := p.readImportName()
	if err != nil {
		return nil, fmt.Errorf("failed to read import name: %w", err)
	}

	// Read extern desc
	externDesc, err := p.parseExternDesc()
	if err != nil {
		return nil, fmt.Errorf("failed to parse extern desc: %w", err)
	}

	return &ast.Import{
		ImportName: name,
		Desc:       externDesc,
	}, nil
}

func (p *Parser) parseExportSection() ([]ast.Definition, error) {
	var definitions []ast.Definition

	err := p.readVec(func() error {
		export, err := p.parseExport()
		if err != nil {
			return err
		}
		definitions = append(definitions, export)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return definitions, nil
}

func (p *Parser) parseExport() (*ast.Export, error) {
	// Read export name
	name, err := p.readExportName()
	if err != nil {
		return nil, fmt.Errorf("failed to read export name: %w", err)
	}

	// Read sortidx
	sortIdx, err := p.parseSortIdx()
	if err != nil {
		return nil, fmt.Errorf("failed to parse sortidx: %w for export %s", err, name)
	}

	hasExternDesc, err := p.readByte()
	if err != nil {
		return nil, fmt.Errorf("failed to peek byte: %w", err)
	}

	var externDesc ast.ExternDesc
	switch hasExternDesc {
	case 0x00:
		// No extern desc, proceed
	case 0x01:
		externDesc, err = p.parseExternDesc()
		if err != nil {
			return nil, fmt.Errorf("failed to parse extern desc: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid extern desc presence byte: 0x%02x", hasExternDesc)
	}

	// Check if exporting a component from root level (not supported)
	if sortIdx.Sort == ast.SortComponent {
		return nil, fmt.Errorf("exporting a component from the root component is not supported")
	}

	return &ast.Export{
		ExportName: name,
		SortIdx:    sortIdx,
		ExternDesc: externDesc,
	}, nil
}

func (p *Parser) parseExternDesc() (ast.ExternDesc, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// core module (type i)
		sortByte, err := p.readByte()
		if err != nil {
			return nil, err
		}
		if sortByte != 0x11 {
			return nil, fmt.Errorf("expected core module sort 0x11, got 0x%02x", sortByte)
		}
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}

		return &ast.SortExternDesc{
			Sort:    ast.SortCoreModule,
			TypeIdx: typeIdx,
		}, nil

	case 0x01:
		// func (type i)
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.SortExternDesc{
			Sort:    ast.SortFunc,
			TypeIdx: typeIdx,
		}, nil

	case 0x02:
		// value - not fully implemented yet
		// TODO: Implement value extern desc
		return nil, fmt.Errorf("value extern desc not yet implemented")

	case 0x03:
		// type (typebound)
		typeBound, err := p.parseTypeBound()
		if err != nil {
			return nil, err
		}
		return &ast.TypeExternDesc{
			Bound: typeBound,
		}, nil

	case 0x04:
		// component (type i)
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.SortExternDesc{
			Sort:    ast.SortComponent,
			TypeIdx: typeIdx,
		}, nil

	case 0x05:
		// instance (type i)
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.SortExternDesc{
			Sort:    ast.SortInstance,
			TypeIdx: typeIdx,
		}, nil

	default:
		return nil, fmt.Errorf("invalid extern desc discriminator: 0x%02x", discriminator)
	}
}

func (p *Parser) parseTypeBound() (ast.TypeBound, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case 0x00:
		// eq i
		typeIdx, err := p.readU32()
		if err != nil {
			return nil, err
		}
		return &ast.EqBound{
			TypeIdx: typeIdx,
		}, nil

	case 0x01:
		// sub resource
		return &ast.SubResourceBound{}, nil

	default:
		return nil, fmt.Errorf("invalid type bound discriminator: 0x%02x", discriminator)
	}
}

// Binary reading utilities

func (p *Parser) readByte() (byte, error) {
	return p.reader.ReadByte()
}

func (p *Parser) peekByte() (byte, error) {
	bytes, err := p.reader.Peek(1)
	if err != nil {
		return 0, err
	}
	return bytes[0], nil
}

func (p *Parser) readBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	_, err := io.ReadFull(p.reader, bytes)
	return bytes, err
}

// readU32 reads an unsigned 32-bit integer in LEB128 encoding
func (p *Parser) readU32() (uint32, error) {
	var result uint32
	var shift uint
	for {
		if shift >= 35 {
			return 0, fmt.Errorf("u32 LEB128 too long")
		}
		b, err := p.readByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, nil
}

// readS32 reads a signed 32-bit integer in LEB128 encoding
func (p *Parser) readS32() (int32, error) {
	var result int32
	var shift uint
	var b byte
	var err error
	for {
		if shift >= 35 {
			return 0, fmt.Errorf("s32 LEB128 too long")
		}
		b, err = p.readByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	// Sign extend
	if shift < 32 && (b&0x40) != 0 {
		result |= ^0 << shift
	}
	return result, nil
}

// readS64 reads a signed 64-bit integer in LEB128 encoding
func (p *Parser) readS64() (int64, error) {
	var result int64
	var shift uint
	var b byte
	var err error
	for {
		if shift >= 70 {
			return 0, fmt.Errorf("s64 LEB128 too long")
		}
		b, err = p.readByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	// Sign extend
	if shift < 64 && (b&0x40) != 0 {
		result |= ^0 << shift
	}
	return result, nil
}

// readF32 reads a 32-bit float in little-endian encoding
func (p *Parser) readF32() (float32, error) {
	bytes, err := p.readBytes(4)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(bytes)
	return float32(bits), nil
}

// readF64 reads a 64-bit float in little-endian encoding
func (p *Parser) readF64() (float64, error) {
	bytes, err := p.readBytes(8)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint64(bytes)
	return float64(bits), nil
}

// readName reads a name (length-prefixed UTF-8 string)
func (p *Parser) readName() (string, error) {
	length, err := p.readU32()
	if err != nil {
		return "", fmt.Errorf("failed to read name length: %w", err)
	}
	bytes, err := p.readBytes(int(length))
	if err != nil {
		return "", fmt.Errorf("failed to read name bytes: %w", err)
	}
	return string(bytes), nil
}

// readImportName reads an import name with discriminator byte
func (p *Parser) readImportName() (string, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return "", fmt.Errorf("failed to read import name discriminator: %w", err)
	}

	switch discriminator {
	case 0x00:
		// Simple import name
		return p.readName()
	case 0x01:
		// Import name with version suffix (not fully supported yet)
		name, err := p.readName()
		if err != nil {
			return "", err
		}
		// Skip version suffix for now
		_, err = p.readName() // version suffix
		if err != nil {
			return "", fmt.Errorf("failed to read version suffix: %w", err)
		}
		return name, nil
	default:
		return "", fmt.Errorf("invalid import name discriminator: 0x%02x", discriminator)
	}
}

// readExportName reads an export name with discriminator byte
func (p *Parser) readExportName() (string, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return "", fmt.Errorf("failed to read export name discriminator: %w", err)
	}

	switch discriminator {
	case 0x00:
		// Simple export name
		return p.readName()
	case 0x01:
		// Export name with version suffix (not fully supported yet)
		name, err := p.readName()
		if err != nil {
			return "", err
		}
		// Skip version suffix for now
		_, err = p.readName() // version suffix
		if err != nil {
			return "", fmt.Errorf("failed to read version suffix: %w", err)
		}
		return name, nil
	default:
		return "", fmt.Errorf("invalid export name discriminator: 0x%02x", discriminator)
	}
}

// readVec reads a vector (count-prefixed sequence of elements)
func (p *Parser) readVec(readElement func() error) error {
	count, err := p.readU32()
	if err != nil {
		return fmt.Errorf("failed to read vector count: %w", err)
	}
	for i := range count {
		if err := readElement(); err != nil {
			return fmt.Errorf("failed to read vector element %d: %w", i, err)
		}
	}
	return nil
}

// parseSort reads a sort discriminator and returns the Sort value
func (p *Parser) parseSort() (ast.Sort, error) {
	discriminator, err := p.readByte()
	if err != nil {
		return 0, err
	}

	switch discriminator {
	case 0x00:
		// core sort
		coreSort, err := p.readByte()
		if err != nil {
			return 0, err
		}
		switch coreSort {
		case 0x00:
			return ast.SortCoreFunc, nil
		case 0x01:
			return ast.SortCoreTable, nil
		case 0x02:
			return ast.SortCoreMemory, nil
		case 0x03:
			return ast.SortCoreGlobal, nil
		case 0x10:
			return ast.SortCoreType, nil
		case 0x11:
			return ast.SortCoreModule, nil
		case 0x12:
			return ast.SortCoreInstance, nil
		default:
			return 0, fmt.Errorf("invalid core sort: 0x%02x", coreSort)
		}
	case 0x01:
		return ast.SortFunc, nil
	case 0x03:
		return ast.SortType, nil
	case 0x04:
		return ast.SortComponent, nil
	case 0x05:
		return ast.SortInstance, nil
	default:
		return 0, fmt.Errorf("invalid sort discriminator: 0x%02x", discriminator)
	}
}

// parseSortIdx reads a sortidx (sort + index)
func (p *Parser) parseSortIdx() (ast.SortIdx, error) {
	sort, err := p.parseSort()
	if err != nil {
		return ast.SortIdx{}, err
	}

	idx, err := p.readU32()
	if err != nil {
		return ast.SortIdx{}, err
	}

	return ast.SortIdx{
		Sort: sort,
		Idx:  idx,
	}, nil
}
