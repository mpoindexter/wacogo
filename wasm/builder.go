package wasm

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/api"
)

type Builder struct {
	sections []Section
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) AddSection(section Section) {
	b.sections = append(b.sections, section)
}

func (b *Builder) Build() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6D}) // WASM Magic Number
	buf.Write([]byte{0x01, 0x00, 0x00, 0x00}) // WASM Version 1
	for _, section := range b.sections {
		if err := section.writeSection(&buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

type Section interface {
	writeSection(w writer) error
}

type writer interface {
	io.Writer
	io.ByteWriter
}

type ImportSection struct {
	Imports []*Import
}

func (is *ImportSection) writeSection(w writer) error {
	var contents bytes.Buffer
	writeLEB128(&contents, uint32(len(is.Imports)))
	for _, imp := range is.Imports {
		if err := writeLEB128(&contents, uint32(len(imp.Module))); err != nil {
			return err
		}
		if _, err := contents.Write([]byte(imp.Module)); err != nil {
			return err
		}

		if err := writeLEB128(&contents, uint32(len(imp.Name))); err != nil {
			return err
		}
		if _, err := contents.Write([]byte(imp.Name)); err != nil {
			return err
		}

		if err := imp.ImportDesc.writeImportDesc(&contents); err != nil {
			return err
		}
	}

	if err := w.WriteByte(2); err != nil {
		return err
	}
	if err := writeLEB128(w, uint32(contents.Len())); err != nil {
		return err
	}
	if _, err := w.Write(contents.Bytes()); err != nil {
		return err
	}
	return nil
}

type Import struct {
	Module     string
	Name       string
	ImportDesc ImportDesc
}

type ImportDesc interface {
	writeImportDesc(writer) error
}

type FuncType struct {
	TypeIdx uint32
}

func (f *FuncType) writeImportDesc(w writer) error {
	if err := w.WriteByte(0); err != nil {
		return err
	}
	if err := writeLEB128(w, f.TypeIdx); err != nil {
		return err
	}
	return nil
}

type TableType struct {
	ElemType RefType
	Limits   *Limits
}

func (t *TableType) writeImportDesc(w writer) error {
	if err := w.WriteByte(1); err != nil {
		return err
	}

	if err := t.ElemType.writeType(w); err != nil {
		return err
	}

	if err := t.Limits.writeLimits(w); err != nil {
		return err
	}

	return nil
}

type Limits struct {
	Min, Max uint32
	HasMax   bool
}

func (l *Limits) writeLimits(w writer) error {
	if !l.HasMax {
		if err := w.WriteByte(0); err != nil {
			return err
		}

		if err := writeLEB128(w, l.Min); err != nil {
			return err
		}
	} else {
		if err := w.WriteByte(1); err != nil {
			return err
		}

		if err := writeLEB128(w, l.Min); err != nil {
			return err
		}

		if err := writeLEB128(w, l.Max); err != nil {
			return err
		}
	}

	return nil
}

type Type interface {
	writeType(w writer) error
}

type ValueType interface {
	Type
	isValueType()
}

type I32 struct{}

func (i I32) isValueType() {}

func (i I32) writeType(w writer) error {
	return w.WriteByte(0x7F)
}

type I64 struct{}

func (i I64) isValueType() {}

func (i I64) writeType(w writer) error {
	return w.WriteByte(0x7E)
}

type F32 struct{}

func (f F32) isValueType() {}

func (f F32) writeType(w writer) error {
	return w.WriteByte(0x7D)
}

type F64 struct{}

func (f F64) isValueType() {}

func (f F64) writeType(w writer) error {
	return w.WriteByte(0x7C)
}

type V128 struct{}

func (v V128) isValueType() {}

func (v V128) writeType(w writer) error {
	return w.WriteByte(0x7B)
}

type RefType interface {
	Type
	isRefType()
}

type FuncRef struct{}

func (f FuncRef) isRefType()   {}
func (f FuncRef) isValueType() {}

func (f FuncRef) writeType(w writer) error {
	return w.WriteByte(0x70)
}

type ExternRef struct{}

func (e ExternRef) isRefType()   {}
func (e ExternRef) isValueType() {}

func (e ExternRef) writeType(w writer) error {
	return w.WriteByte(0x6F)
}

type MemoryType struct {
	Min, Max uint32
	HasMax   bool
}

func (m *MemoryType) writeImportDesc(w writer) error {
	if err := w.WriteByte(2); err != nil {
		return err
	}

	if !m.HasMax {
		if err := w.WriteByte(0); err != nil {
			return err
		}

		if err := writeLEB128(w, m.Min); err != nil {
			return err
		}
	} else {
		if err := w.WriteByte(1); err != nil {
			return err
		}

		if err := writeLEB128(w, m.Min); err != nil {
			return err
		}

		if err := writeLEB128(w, m.Max); err != nil {
			return err
		}
	}

	return nil
}

type GlobalType struct {
	ValType ValueType
	Mutable bool
}

func (g *GlobalType) writeImportDesc(w writer) error {
	if err := w.WriteByte(3); err != nil {
		return err
	}

	if err := g.ValType.writeType(w); err != nil {
		return err
	}

	var mutableByte byte
	if g.Mutable {
		mutableByte = 1
	} else {
		mutableByte = 0
	}

	if err := w.WriteByte(mutableByte); err != nil {
		return err
	}

	return nil
}

func writeLEB128(w writer, value uint32) error {
	for {
		b := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		if value == 0 {
			break
		}
	}
	return nil
}

type ExportSection struct {
	Exports []*Export
}

func (es *ExportSection) writeSection(w writer) error {
	var contents bytes.Buffer
	writeLEB128(&contents, uint32(len(es.Exports)))
	for _, exp := range es.Exports {
		if err := writeLEB128(&contents, uint32(len(exp.Name))); err != nil {
			return err
		}
		if _, err := contents.Write([]byte(exp.Name)); err != nil {
			return err
		}
		if err := exp.ExportDesc.writeExportDesc(&contents); err != nil {
			return err
		}
	}

	if err := w.WriteByte(7); err != nil {
		return err
	}
	if err := writeLEB128(w, uint32(contents.Len())); err != nil {
		return err
	}
	if _, err := w.Write(contents.Bytes()); err != nil {
		return err
	}
	return nil
}

type Export struct {
	Name       string
	ExportDesc ExportDesc
}

type ExportDesc interface {
	writeExportDesc(w writer) error
}

type FuncExport struct {
	Idx uint32
}

func (f *FuncExport) writeExportDesc(w writer) error {
	if err := w.WriteByte(0); err != nil {
		return err
	}
	if err := writeLEB128(w, f.Idx); err != nil {
		return err
	}
	return nil
}

type TableExport struct {
	Idx uint32
}

func (t *TableExport) writeExportDesc(w writer) error {
	if err := w.WriteByte(1); err != nil {
		return err
	}
	if err := writeLEB128(w, t.Idx); err != nil {
		return err
	}
	return nil
}

type MemoryExport struct {
	Idx uint32
}

func (m *MemoryExport) writeExportDesc(w writer) error {
	if err := w.WriteByte(2); err != nil {
		return err
	}
	if err := writeLEB128(w, m.Idx); err != nil {
		return err
	}
	return nil
}

type GlobalExport struct {
	Idx uint32
}

func (g *GlobalExport) writeExportDesc(w writer) error {
	if err := w.WriteByte(3); err != nil {
		return err
	}
	if err := writeLEB128(w, g.Idx); err != nil {
		return err
	}
	return nil
}

type TypeSection struct {
	Types []*FuncTypeDef
}

func (ts *TypeSection) AddFuncDef(d api.FunctionDefinition) error {
	funcType := &FuncTypeDef{}
	for _, paramType := range d.ParamTypes() {
		vt, err := WazeroValueTypeToValueType(paramType)
		if err != nil {
			return err
		}
		funcType.ParamTypes = append(funcType.ParamTypes, vt)
	}

	for _, resultType := range d.ResultTypes() {
		vt, err := WazeroValueTypeToValueType(resultType)
		if err != nil {
			return err
		}
		funcType.ResultTypes = append(funcType.ResultTypes, vt)
	}

	ts.Types = append(ts.Types, funcType)
	return nil
}

func (ts *TypeSection) writeSection(w writer) error {
	var contents bytes.Buffer
	if err := writeLEB128(&contents, uint32(len(ts.Types))); err != nil {
		return err
	}
	for _, t := range ts.Types {
		if err := t.writeType(&contents); err != nil {
			return err
		}
	}

	if err := w.WriteByte(1); err != nil {
		return err
	}
	if err := writeLEB128(w, uint32(contents.Len())); err != nil {
		return err
	}
	if _, err := w.Write(contents.Bytes()); err != nil {
		return err
	}
	return nil
}

type FuncSection struct {
	FuncTypeIndices []uint32
}

func (fs *FuncSection) writeSection(w writer) error {
	var contents bytes.Buffer
	if err := writeLEB128(&contents, uint32(len(fs.FuncTypeIndices))); err != nil {
		return err
	}
	for _, typeIdx := range fs.FuncTypeIndices {
		if err := writeLEB128(&contents, typeIdx); err != nil {
			return err
		}
	}

	if err := w.WriteByte(3); err != nil {
		return err
	}
	if err := writeLEB128(w, uint32(contents.Len())); err != nil {
		return err
	}
	if _, err := w.Write(contents.Bytes()); err != nil {
		return err
	}
	return nil
}

type FuncTypeDef struct {
	ParamTypes  []ValueType
	ResultTypes []ValueType
}

func (f *FuncTypeDef) writeType(w writer) error {
	if err := w.WriteByte(0x60); err != nil {
		return err
	}

	if err := writeLEB128(w, uint32(len(f.ParamTypes))); err != nil {
		return err
	}
	for _, paramType := range f.ParamTypes {
		if err := paramType.writeType(w); err != nil {
			return err
		}
	}

	if err := writeLEB128(w, uint32(len(f.ResultTypes))); err != nil {
		return err
	}
	for _, resultType := range f.ResultTypes {
		if err := resultType.writeType(w); err != nil {
			return err
		}
	}

	return nil
}

func WazeroValueTypeToValueType(vt api.ValueType) (ValueType, error) {
	switch vt {
	case api.ValueTypeI32:
		return I32{}, nil
	case api.ValueTypeI64:
		return I64{}, nil
	case api.ValueTypeF32:
		return F32{}, nil
	case api.ValueTypeF64:
		return F64{}, nil
	case api.ValueTypeExternref:
		return ExternRef{}, nil
	default:
		return nil, fmt.Errorf("unsupported wazero value type: %v", vt)
	}
}
