package componentmodel

import (
	"context"
	"fmt"
	"reflect"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

type Value interface {
	isValue()
}

type ValueType interface {
	Type
	isValueType()
	supportsValue(v Value) bool
	alignment() int
	elementSize() int
	flatTypes() []api.ValueType
	liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error)
	load(llc *LiftLoadContext, offset uint32) (Value, error)
	lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error)
	store(llc *LiftLoadContext, offset uint32, val Value) error
}

type primitiveValueType[T ValueType, V Value] struct{}

func (primitiveValueType[T, V]) isValueType() {}

func (t primitiveValueType[T, V]) supportsValue(v Value) bool {
	_, ok := v.(V)
	return ok
}
func (primitiveValueType[T, V]) equalsType(other Type) bool {
	_, ok := other.(T)
	return ok
}

type Bool bool

func (v Bool) isValue() {}

type BoolType struct {
	primitiveValueType[BoolType, Bool]
}

func (t BoolType) alignment() int             { return 1 }
func (t BoolType) elementSize() int           { return 1 }
func (t BoolType) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t BoolType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return Bool(itr() != 0), nil
}

func (t BoolType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	b, ok := llc.memory.ReadByte(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read byte at offset %d", offset)
	}
	if b != 0 {
		return Bool(true), nil
	}
	return Bool(false), nil
}

func (t BoolType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	boolVal := val.(Bool)
	var flat uint64
	if boolVal {
		flat = 1
	} else {
		flat = 0
	}
	return []uint64{flat}, nil
}

func (t BoolType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	boolVal := val.(Bool)
	var b byte
	if boolVal {
		b = 1
	} else {
		b = 0
	}
	ok := llc.memory.WriteByte(offset, b)
	if !ok {
		return fmt.Errorf("failed to write byte at offset %d", offset)
	}
	return nil
}

type U8 uint8

func (v U8) isValue() {}

type U8Type struct {
	primitiveValueType[U8Type, U8]
}

func (t U8Type) alignment() int             { return 1 }
func (t U8Type) elementSize() int           { return 1 }
func (t U8Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t U8Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return U8(itr()), nil
}

func (t U8Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	b, ok := llc.memory.ReadByte(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read byte at offset %d", offset)
	}
	return U8(b), nil
}

func (t U8Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	u8Val := val.(U8)
	return []uint64{uint64(u8Val)}, nil
}

func (t U8Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	u8Val := val.(U8)
	ok := llc.memory.WriteByte(offset, byte(u8Val))
	if !ok {
		return fmt.Errorf("failed to write byte at offset %d", offset)
	}
	return nil
}

type U16 uint16

func (v U16) isValue() {}

type U16Type struct {
	primitiveValueType[U16Type, U16]
}

func (t U16Type) alignment() int             { return 2 }
func (t U16Type) elementSize() int           { return 2 }
func (t U16Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t U16Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return U16(itr()), nil
}

func (t U16Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint16Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint16 at offset %d", offset)
	}
	return U16(val), nil
}

func (t U16Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	u16Val := val.(U16)
	return []uint64{uint64(u16Val)}, nil
}

func (t U16Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	ok := llc.memory.WriteUint16Le(offset, uint16(val.(U16)))
	if !ok {
		return fmt.Errorf("failed to write uint16 at offset %d", offset)
	}
	return nil
}

type U32 uint32

func (v U32) isValue() {}

type U32Type struct {
	primitiveValueType[U32Type, U32]
}

func (t U32Type) alignment() int             { return 4 }
func (t U32Type) elementSize() int           { return 4 }
func (t U32Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t U32Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return U32(itr()), nil
}

func (t U32Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint32 at offset %d", offset)
	}
	return U32(val), nil
}

func (t U32Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	u32Val := val.(U32)
	return []uint64{uint64(u32Val)}, nil
}

func (t U32Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	ok := llc.memory.WriteUint32Le(offset, uint32(val.(U32)))
	if !ok {
		return fmt.Errorf("failed to write uint32 at offset %d", offset)
	}
	return nil
}

type U64 uint64

func (v U64) isValue() {}

type U64Type struct {
	primitiveValueType[U64Type, U64]
}

func (t U64Type) alignment() int             { return 8 }
func (t U64Type) elementSize() int           { return 8 }
func (t U64Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI64} }

func (t U64Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return U64(itr()), nil
}

func (t U64Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint64Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint64 at offset %d", offset)
	}
	return U64(val), nil
}

func (t U64Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	u64Val := val.(U64)
	return []uint64{uint64(u64Val)}, nil
}

func (t U64Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	ok := llc.memory.WriteUint64Le(offset, uint64(val.(U64)))
	if !ok {
		return fmt.Errorf("failed to write uint64 at offset %d", offset)
	}
	return nil
}

type S8 int8

func (v S8) isValue() {}

type S8Type struct {
	primitiveValueType[S8Type, S8]
}

func (t S8Type) alignment() int             { return 1 }
func (t S8Type) elementSize() int           { return 1 }
func (t S8Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t S8Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return S8(itr()), nil
}

func (t S8Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	b, ok := llc.memory.ReadByte(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read byte at offset %d", offset)
	}
	return S8(int8(b)), nil
}

func (t S8Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	s8Val := val.(S8)
	return []uint64{uint64(int64(s8Val))}, nil
}

func (t S8Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	s8Val := val.(S8)
	ok := llc.memory.WriteByte(offset, byte(s8Val))
	if !ok {
		return fmt.Errorf("failed to write byte at offset %d", offset)
	}
	return nil
}

type S16 int16

func (v S16) isValue() {}

type S16Type struct {
	primitiveValueType[S16Type, S16]
}

func (t S16Type) alignment() int             { return 2 }
func (t S16Type) elementSize() int           { return 2 }
func (t S16Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t S16Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return S16(itr()), nil
}

func (t S16Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint16Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint16 at offset %d", offset)
	}
	return S16(int16(val)), nil
}

func (t S16Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	s16Val := val.(S16)
	return []uint64{uint64(int64(s16Val))}, nil
}

func (t S16Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	s16Val := val.(S16)
	ok := llc.memory.WriteUint16Le(offset, uint16(s16Val))
	if !ok {
		return fmt.Errorf("failed to write uint16 at offset %d", offset)
	}
	return nil
}

type S32 int32

func (v S32) isValue() {}

type S32Type struct {
	primitiveValueType[S32Type, S32]
}

func (t S32Type) alignment() int             { return 4 }
func (t S32Type) elementSize() int           { return 4 }
func (t S32Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t S32Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return S32(itr()), nil
}

func (t S32Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint32 at offset %d", offset)
	}
	return S32(int32(val)), nil
}

func (t S32Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	s32Val := val.(S32)
	return []uint64{uint64(int64(s32Val))}, nil
}

func (t S32Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	s32Val := val.(S32)
	ok := llc.memory.WriteUint32Le(offset, uint32(s32Val))
	if !ok {
		return fmt.Errorf("failed to write uint32 at offset %d", offset)
	}
	return nil
}

type S64 int64

func (v S64) isValue() {}

type S64Type struct {
	primitiveValueType[S64Type, S64]
}

func (t S64Type) alignment() int             { return 8 }
func (t S64Type) elementSize() int           { return 8 }
func (t S64Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI64} }

func (t S64Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return S64(itr()), nil
}

func (t S64Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint64Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint64 at offset %d", offset)
	}
	return S64(int64(val)), nil
}

func (t S64Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	s64Val := val.(S64)
	return []uint64{uint64(s64Val)}, nil
}

func (t S64Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	s64Val := val.(S64)
	ok := llc.memory.WriteUint64Le(offset, uint64(s64Val))
	if !ok {
		return fmt.Errorf("failed to write uint64 at offset %d", offset)
	}
	return nil
}

type F32 float32

func (v F32) isValue() {}

type F32Type struct {
	primitiveValueType[F32Type, F32]
}

func (t F32Type) alignment() int             { return 4 }
func (t F32Type) elementSize() int           { return 4 }
func (t F32Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeF32} }

func (t F32Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	v := api.DecodeF32(itr())
	return F32(v), nil
}

func (t F32Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	v, ok := llc.memory.ReadFloat32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read float32 at offset %d", offset)
	}
	return F32(v), nil
}

func (t F32Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	f32Val := val.(F32)
	flat := api.EncodeF32(float32(f32Val))
	return []uint64{flat}, nil
}

func (t F32Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	f32Val := val.(F32)
	ok := llc.memory.WriteFloat32Le(offset, float32(f32Val))
	if !ok {
		return fmt.Errorf("failed to write float32 at offset %d", offset)
	}
	return nil
}

type F64 float64

func (v F64) isValue() {}

type F64Type struct {
	primitiveValueType[F64Type, F64]
}

func (t F64Type) alignment() int             { return 8 }
func (t F64Type) elementSize() int           { return 8 }
func (t F64Type) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeF64} }

func (t F64Type) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	v := api.DecodeF64(itr())
	return F64(v), nil
}

func (t F64Type) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	v, ok := llc.memory.ReadFloat64Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read float64 at offset %d", offset)
	}
	return F64(v), nil
}

func (t F64Type) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	f64Val := val.(F64)
	flat := api.EncodeF64(float64(f64Val))
	return []uint64{flat}, nil
}

func (t F64Type) store(llc *LiftLoadContext, offset uint32, val Value) error {
	f64Val := val.(F64)
	ok := llc.memory.WriteFloat64Le(offset, float64(f64Val))
	if !ok {
		return fmt.Errorf("failed to write float64 at offset %d", offset)
	}
	return nil
}

type Char rune

func (v Char) isValue() {}

type CharType struct {
	primitiveValueType[CharType, Char]
}

func (t CharType) alignment() int             { return 4 }
func (t CharType) elementSize() int           { return 4 }
func (t CharType) flatTypes() []api.ValueType { return []api.ValueType{api.ValueTypeI32} }

func (t CharType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	return Char(itr()), nil
}

func (t CharType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	val, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read uint32 at offset %d", offset)
	}
	return Char(val), nil
}

func (t CharType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	charVal := val.(Char)
	return []uint64{uint64(charVal)}, nil
}

func (t CharType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	charVal := val.(Char)
	ok := llc.memory.WriteUint32Le(offset, uint32(charVal))
	if !ok {
		return fmt.Errorf("failed to write uint32 at offset %d", offset)
	}
	return nil
}

type String string

func (v String) isValue() {}

type StringType struct {
	primitiveValueType[StringType, String]
}

func (t StringType) alignment() int   { return 4 }
func (t StringType) elementSize() int { return 8 }
func (t StringType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
}

func (t StringType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	ptr := uint32(itr())
	length := uint32(itr())
	return t.readString(llc, ptr, length)
}

func (t StringType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	ptr, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read string pointer at offset %d", offset)
	}
	length, ok := llc.memory.ReadUint32Le(offset + 4)
	if !ok {
		return nil, fmt.Errorf("failed to read string length at offset %d", offset+4)
	}
	return t.readString(llc, ptr, length)
}

func (t StringType) readString(llc *LiftLoadContext, ptr uint32, length uint32) (String, error) {
	switch llc.stringEncoding {
	case stringEncodingUTF8:
		bytes, ok := llc.memory.Read(ptr, length)
		if !ok {
			return "", fmt.Errorf("failed to read string bytes at ptr %d with length %d", ptr, length)
		}
		return String(bytes), nil
	case stringEncodingUTF16:
		bytes, ok := llc.memory.Read(ptr, length*2)
		if !ok {
			return "", fmt.Errorf("failed to read string bytes at ptr %d with length %d", ptr, length*2)
		}
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
		decoded, err := decoder.Bytes(bytes)
		if err != nil {
			return "", fmt.Errorf("failed to decode utf16 string: %w", err)
		}
		return String(decoded), nil
	case stringEncodingLatin1UTF16:
		if (length & (1 << 31)) != 0 {
			// UTF-16 encoded
			readLength := 2 * (length & 0x7FFFFFFF)
			bytes, ok := llc.memory.Read(ptr, readLength)
			if !ok {
				return "", fmt.Errorf("failed to read string bytes at ptr %d with length %d", ptr, readLength)
			}
			decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
			decoded, err := decoder.Bytes(bytes)
			if err != nil {
				return "", fmt.Errorf("failed to decode utf16 string: %w", err)
			}
			return String(decoded), nil
		} else {
			// Latin-1 encoded
			bytes, ok := llc.memory.Read(ptr, length)
			if !ok {
				return "", fmt.Errorf("failed to read string bytes at ptr %d with length %d", ptr, length)
			}
			decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(bytes)
			if err != nil {
				return "", fmt.Errorf("failed to decode latin1 string: %w", err)
			}
			return String(decoded), nil
		}
	default:
		return "", fmt.Errorf("unsupported string encoding: %d", llc.stringEncoding)
	}
}

func (t StringType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	ptr, len, err := t.writeString(llc, val.(String))
	if err != nil {
		return nil, err
	}
	return []uint64{uint64(ptr), uint64(len)}, nil
}

func (t StringType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	strVal := val.(String)
	ptr, len, err := t.writeString(llc, strVal)
	if err != nil {
		return err
	}
	ok := llc.memory.WriteUint32Le(offset, ptr)
	if !ok {
		return fmt.Errorf("failed to write string pointer at offset %d", offset)
	}
	ok = llc.memory.WriteUint32Le(offset+4, len)
	if !ok {
		return fmt.Errorf("failed to write string length at offset %d", offset+4)
	}
	return nil
}

func (t StringType) writeString(llc *LiftLoadContext, str String) (uint32, uint32, error) {
	switch llc.stringEncoding {
	case stringEncodingUTF8:
		bytes := []byte(str)
		ptr := llc.realloc(0, 0, 1, uint32(len(bytes)))
		ok := llc.memory.Write(ptr, bytes)
		if !ok {
			return 0, 0, fmt.Errorf("failed to write string bytes at ptr %d with length %d", ptr, len(bytes))
		}
		return ptr, uint32(len(bytes)), nil
	case stringEncodingUTF16, stringEncodingLatin1UTF16:
		encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
		encoded, err := encoder.Bytes([]byte(str))
		if err != nil {
			return 0, 0, fmt.Errorf("failed to encode utf16 string: %w", err)
		}
		ptr := llc.realloc(0, 0, 2, uint32(len(encoded)))
		ok := llc.memory.Write(ptr, encoded)
		if !ok {
			return 0, 0, fmt.Errorf("failed to write string bytes at ptr %d with length %d", ptr, len(encoded))
		}
		return ptr, uint32(len(encoded) / 2), nil
	default:
		return 0, 0, fmt.Errorf("unsupported string encoding: %d", llc.stringEncoding)
	}
}

type Record struct {
	fields []Value
}

func NewRecord(fields ...Value) Record {
	return Record{
		fields: fields,
	}
}

func (r Record) Field(i int) Value {
	return r.fields[i]
}

func (r Record) isValue() {}

type RecordField struct {
	Name string
	Type ValueType
}

type RecordType struct {
	Fields []*RecordField
}

func (t *RecordType) isValueType() {}

func (t *RecordType) supportsValue(v Value) bool {
	recordVal, ok := v.(Record)
	if !ok {
		return false
	}
	if len(t.Fields) != len(recordVal.fields) {
		return false
	}
	for i, f := range t.Fields {
		if !f.Type.supportsValue(recordVal.fields[i]) {
			return false
		}
	}
	return true
}

func (t *RecordType) equalsType(other Type) bool {
	otherRecordType, ok := other.(*RecordType)
	if !ok {
		return false
	}
	if len(t.Fields) != len(otherRecordType.Fields) {
		return false
	}
	for i, f := range t.Fields {
		otherField := otherRecordType.Fields[i]
		if f.Name != otherField.Name {
			return false
		}
		if !f.Type.equalsType(otherField.Type) {
			return false
		}
	}
	return true
}

func (t *RecordType) alignment() int {
	align := 1
	for _, f := range t.Fields {
		a := f.Type.alignment()
		if a > align {
			align = a
		}
	}
	return align
}

func (t *RecordType) elementSize() int {
	size := 0
	for _, f := range t.Fields {
		size = alignTo(size, f.Type.alignment())
		size += f.Type.elementSize()
	}
	return size
}

func (t *RecordType) flatTypes() []api.ValueType {
	var flats []api.ValueType
	for _, f := range t.Fields {
		flats = append(flats, f.Type.flatTypes()...)
	}
	return flats
}

func (t *RecordType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	values := make([]Value, len(t.Fields))
	for i, f := range t.Fields {
		val, err := f.Type.liftFlat(llc, itr)
		if err != nil {
			return nil, fmt.Errorf("failed to lift field %s: %w", f.Name, err)
		}
		values[i] = val
	}
	return Record{
		fields: values,
	}, nil
}

func (t *RecordType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	values := make([]Value, len(t.Fields))
	currentOffset := offset
	for i, f := range t.Fields {
		currentOffset = uint32(alignTo(int(currentOffset), f.Type.alignment()))
		val, err := f.Type.load(llc, currentOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to load field %s: %w", f.Name, err)
		}
		values[i] = val
		currentOffset += uint32(f.Type.elementSize())
	}
	return Record{
		fields: values,
	}, nil
}

func (t *RecordType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	recordVal := val.(Record)
	var flats []uint64
	for i, f := range t.Fields {
		fieldFlats, err := f.Type.lowerFlat(llc, recordVal.fields[i])
		if err != nil {
			return nil, fmt.Errorf("failed to lower field %s: %w", f.Name, err)
		}
		flats = append(flats, fieldFlats...)
	}
	return flats, nil
}

func (t *RecordType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	recordVal := val.(Record)
	currentOffset := offset
	for i, f := range t.Fields {
		currentOffset = uint32(alignTo(int(currentOffset), f.Type.alignment()))
		err := f.Type.store(llc, currentOffset, recordVal.fields[i])
		if err != nil {
			return fmt.Errorf("failed to store field %s: %w", f.Name, err)
		}
		currentOffset += uint32(f.Type.elementSize())
	}
	return nil
}

type Variant struct {
	CaseLabel string
	Value     Value
}

func (v *Variant) isValue() {}

type VariantCase struct {
	Name string
	Type ValueType
}

type VariantType struct {
	Cases []*VariantCase
}

func (t *VariantType) isValueType() {}

func (t *VariantType) supportsValue(v Value) bool {
	variantVal, ok := v.(*Variant)
	if !ok {
		return false
	}

	for _, c := range t.Cases {
		if c.Name == variantVal.CaseLabel {
			return c.Type.supportsValue(variantVal.Value)
		}
	}
	return false
}

func (t *VariantType) equalsType(other Type) bool {
	otherVariantType, ok := other.(*VariantType)
	if !ok {
		return false
	}
	if len(t.Cases) != len(otherVariantType.Cases) {
		return false
	}
	for i, c := range t.Cases {
		otherCase := otherVariantType.Cases[i]
		if c.Name != otherCase.Name {
			return false
		}
		if (c.Type == nil) != (otherCase.Type == nil) {
			return false
		}
		if c.Type != nil && !c.Type.equalsType(otherCase.Type) {
			return false
		}
	}
	return true
}

func (t *VariantType) alignment() int {
	align := t.discriminantSize()
	caseAlign := t.maxCaseAligment()
	if caseAlign > align {
		align = caseAlign
	}
	return align
}

func (t *VariantType) discriminantSize() int {
	numCases := len(t.Cases)
	if numCases <= 256 {
		return 1
	} else if numCases <= 65536 {
		return 2
	} else {
		return 4
	}
}

func (t *VariantType) maxCaseAligment() int {
	align := 1
	for _, c := range t.Cases {
		if c.Type == nil {
			continue
		}
		a := c.Type.alignment()
		if a > align {
			align = a
		}
	}
	return align
}

func (t *VariantType) elementSize() int {
	s := t.discriminantSize()
	s = alignTo(s, t.maxCaseAligment())
	cs := 0
	for _, c := range t.Cases {
		if c.Type == nil {
			continue
		}
		elementSize := c.Type.elementSize()
		if elementSize > cs {
			cs = elementSize
		}
	}
	s += cs
	return alignTo(s, t.alignment())
}

func (t *VariantType) flatTypes() []api.ValueType {
	var flats []api.ValueType
	for _, c := range t.Cases {
		if c.Type == nil {
			continue
		}
		caseFlats := c.Type.flatTypes()
		for i, ft := range caseFlats {
			if i >= len(flats) {
				flats = append(flats, ft)
			} else {
				flats[i] = joinFlatTypes(flats[i], ft)
			}
		}
	}
	return append([]api.ValueType{api.ValueTypeI32}, flats...)
}

func (t *VariantType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	discriminant := uint32(itr())
	if int(discriminant) >= len(t.Cases) {
		return nil, fmt.Errorf("invalid discriminant %d for variant with %d cases", discriminant, len(t.Cases))
	}
	var caseValue Value
	caseType := t.Cases[discriminant].Type
	if caseType != nil {
		val, err := caseType.liftFlat(llc, itr)
		if err != nil {
			return nil, fmt.Errorf("failed to lift case %s: %w", t.Cases[discriminant].Name, err)
		}
		caseValue = val
	}
	label := t.Cases[discriminant].Name
	return &Variant{
		CaseLabel: label,
		Value:     caseValue,
	}, nil
}

func (t *VariantType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	var discriminant uint32
	switch t.discriminantSize() {
	case 1:
		b, ok := llc.memory.ReadByte(offset)
		if !ok {
			return nil, fmt.Errorf("failed to read variant discriminant at offset %d", offset)
		}
		discriminant = uint32(b)
	case 2:
		val, ok := llc.memory.ReadUint16Le(offset)
		if !ok {
			return nil, fmt.Errorf("failed to read variant discriminant at offset %d", offset)
		}
		discriminant = uint32(val)
	case 4:
		val, ok := llc.memory.ReadUint32Le(offset)
		if !ok {
			return nil, fmt.Errorf("failed to read variant discriminant at offset %d", offset)
		}
		discriminant = val
	default:
		return nil, fmt.Errorf("unsupported discriminant size %d", t.discriminantSize())
	}
	if int(discriminant) >= len(t.Cases) {
		return nil, fmt.Errorf("invalid discriminant %d for variant with %d cases", discriminant, len(t.Cases))
	}
	currentOffset := offset + uint32(t.discriminantSize())
	currentOffset = uint32(alignTo(int(currentOffset), t.maxCaseAligment()))
	var caseValue Value
	caseType := t.Cases[discriminant].Type
	if caseType != nil {
		val, err := caseType.load(llc, currentOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to load case %s: %w", t.Cases[discriminant].Name, err)
		}
		caseValue = val
	}
	label := t.Cases[discriminant].Name
	return &Variant{
		CaseLabel: label,
		Value:     caseValue,
	}, nil
}

func (t *VariantType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	variantVal := val.(*Variant)
	var flats []uint64

	var caseIdx int = -1
	for i, c := range t.Cases {
		if c.Name == variantVal.CaseLabel {
			caseIdx = i
			break
		}
	}
	if caseIdx == -1 {
		return nil, fmt.Errorf("invalid case label %s for variant", variantVal.CaseLabel)
	}
	flats = append(flats, uint64(caseIdx))
	caseType := t.Cases[caseIdx].Type
	if caseType != nil {
		caseFlats, err := caseType.lowerFlat(llc, variantVal.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to lower case %s: %w", t.Cases[caseIdx].Name, err)
		}
		flats = append(flats, caseFlats...)
	}
	return flats, nil
}

func (t *VariantType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	variantVal := val.(*Variant)
	currentOffset := offset

	var caseIdx int = -1
	for i, c := range t.Cases {
		if c.Name == variantVal.CaseLabel {
			caseIdx = i
			break
		}
	}
	if caseIdx == -1 {
		return fmt.Errorf("invalid case label %s for variant", variantVal.CaseLabel)
	}
	// Store discriminant
	switch t.discriminantSize() {
	case 1:
		ok := llc.memory.WriteByte(currentOffset, byte(caseIdx))
		if !ok {
			return fmt.Errorf("failed to write variant discriminant at offset %d", currentOffset)
		}
	case 2:
		ok := llc.memory.WriteUint16Le(currentOffset, uint16(caseIdx))
		if !ok {
			return fmt.Errorf("failed to write variant discriminant at offset %d", currentOffset)
		}
	case 4:
		ok := llc.memory.WriteUint32Le(currentOffset, uint32(caseIdx))
		if !ok {
			return fmt.Errorf("failed to write variant discriminant at offset %d", currentOffset)
		}
	default:
		return fmt.Errorf("unsupported discriminant size %d", t.discriminantSize())
	}
	currentOffset += uint32(t.discriminantSize())
	currentOffset = uint32(alignTo(int(currentOffset), t.maxCaseAligment()))
	caseType := t.Cases[caseIdx].Type
	if caseType != nil {
		err := caseType.store(llc, currentOffset, variantVal.Value)
		if err != nil {
			return fmt.Errorf("failed to store case %s: %w", t.Cases[caseIdx].Name, err)
		}
	}
	return nil
}

type List []Value

func (l List) isValue() {}

type ListType struct {
	ElementType ValueType
}

func (t *ListType) isValueType() {}

func (t *ListType) supportsValue(v Value) bool {
	listVal, ok := v.(List)
	if !ok {
		return false
	}
	for i := 0; i < len(listVal); i++ {
		if !t.ElementType.supportsValue(listVal[i]) {
			return false
		}
	}
	return true
}

func (t *ListType) equalsType(other Type) bool {
	otherListType, ok := other.(*ListType)
	if !ok {
		return false
	}
	return t.ElementType.equalsType(otherListType.ElementType)
}
func (t *ListType) alignment() int   { return 4 }
func (t *ListType) elementSize() int { return 8 }
func (t *ListType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
}

func (t *ListType) loadListValues(llc *LiftLoadContext, ptr uint32, length uint32) (Value, error) {
	elements := make(List, length)
	currentOffset := ptr
	elementSize := t.ElementType.elementSize()
	for i := uint32(0); i < length; i++ {
		val, err := t.ElementType.load(llc, currentOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to load list element %d: %w", i, err)
		}
		elements[int(i)] = val
		currentOffset += uint32(elementSize)
	}
	return elements, nil
}

func (t *ListType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	ptr := uint32(itr())
	length := uint32(itr())
	return t.loadListValues(llc, ptr, length)
}

func (t *ListType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	ptr, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read list pointer at offset %d", offset)
	}
	length, ok := llc.memory.ReadUint32Le(offset + 4)
	if !ok {
		return nil, fmt.Errorf("failed to read list length at offset %d", offset+4)
	}
	return t.loadListValues(llc, ptr, length)
}

func (t *ListType) storeListValues(llc *LiftLoadContext, val Value) (uint32, int, error) {
	listVal := val.(List)
	ptr := llc.realloc(0, 0, uint32(t.alignment()), uint32(len(listVal))*uint32(t.ElementType.elementSize()))
	writeTo := uint32(alignTo(int(ptr), t.ElementType.alignment()))
	for i := range listVal {
		err := t.ElementType.store(llc, writeTo, listVal[i])
		if err != nil {
			return 0, 0, fmt.Errorf("failed to store list element %d: %w", i, err)
		}
		writeTo += uint32(t.ElementType.elementSize())
	}
	return ptr, len(listVal), nil
}

func (t *ListType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	ptr, len, err := t.storeListValues(llc, val)
	if err != nil {
		return nil, err
	}
	return []uint64{uint64(ptr), uint64(len)}, nil
}

func (t *ListType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	listVal := val.(List)
	ptr, _, err := t.storeListValues(llc, listVal)
	if err != nil {
		return err
	}
	ok := llc.memory.WriteUint32Le(offset, ptr)
	if !ok {
		return fmt.Errorf("failed to write list pointer at offset %d", offset)
	}
	ok = llc.memory.WriteUint32Le(offset+4, uint32(len(listVal)))
	if !ok {
		return fmt.Errorf("failed to write list length at offset %d", offset+4)
	}
	return nil
}

type Flags map[string]bool

func (f Flags) isValue() {}

type FlagsType struct {
	FlagNames []string
}

func (t *FlagsType) isValueType() {}

func (t *FlagsType) supportsValue(v Value) bool {
	flagsVal, ok := v.(Flags)
	if !ok {
		return false
	}
	if len(flagsVal) != len(t.FlagNames) {
		return false
	}
	for _, name := range t.FlagNames {
		if _, exists := flagsVal[name]; !exists {
			return false
		}
	}
	return true
}

func (t *FlagsType) equalsType(other Type) bool {
	otherFlagsType, ok := other.(*FlagsType)
	if !ok {
		return false
	}
	if len(t.FlagNames) != len(otherFlagsType.FlagNames) {
		return false
	}
	for i, name := range t.FlagNames {
		if name != otherFlagsType.FlagNames[i] {
			return false
		}
	}
	return true
}
func (t *FlagsType) alignment() int   { return 4 }
func (t *FlagsType) elementSize() int { return 4 }
func (t *FlagsType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32}
}

func (t *FlagsType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	bits := uint32(itr())
	flags := make(Flags)
	for i, name := range t.FlagNames {
		flags[name] = (bits & (1 << i)) != 0
	}
	return flags, nil
}

func (t *FlagsType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	bits, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read flags bits at offset %d", offset)
	}
	flags := make(Flags)
	for i, name := range t.FlagNames {
		flags[name] = (bits & (1 << i)) != 0
	}
	return flags, nil
}

func (t *FlagsType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	flagsVal := val.(Flags)
	var bits uint32
	for i, name := range t.FlagNames {
		if flagsVal[name] {
			bits |= (1 << i)
		}
	}
	return []uint64{uint64(bits)}, nil
}

func (t *FlagsType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	flagsVal := val.(Flags)
	var bits uint32
	for i, name := range t.FlagNames {
		if flagsVal[name] {
			bits |= (1 << i)
		}
	}
	ok := llc.memory.WriteUint32Le(offset, bits)
	if !ok {
		return fmt.Errorf("failed to write flags bits at offset %d", offset)
	}
	return nil
}

type ResourceType struct {
	instance   *Instance
	repType    reflect.Type
	destructor func(ctx context.Context, res any)
}

func NewResourceType(instance *Instance, repType reflect.Type, destructor func(ctx context.Context, res any)) *ResourceType {
	return &ResourceType{
		instance:   instance,
		repType:    repType,
		destructor: destructor,
	}
}

func (t *ResourceType) equalsType(other Type) bool {
	if t == anyResourceType {
		_, ok := other.(*ResourceType)
		return ok
	}
	return t == other
}

type ResourceHandle interface {
	Value
	Resource() any
	Drop()
	Borrow() ResourceHandle

	resourceType() *ResourceType
	isBorrowed() bool
}

func NewResourceHandle(instance *Instance, typ *ResourceType, rep any) ResourceHandle {
	return &ownHandle{
		instance: instance,
		typ:      typ,
		rep:      rep,
	}
}

type ownHandle struct {
	instance *Instance
	typ      *ResourceType
	rep      any
	numLends int
	dropped  bool
}

func (h *ownHandle) isValue() {}

func (h *ownHandle) resourceType() *ResourceType {
	return h.typ
}

func (h *ownHandle) isBorrowed() bool {
	return h.numLends > 0
}

func (h *ownHandle) Resource() any {
	if h.dropped {
		panic("cannot use dropped resource")
	}
	return h.rep
}

func (h *ownHandle) Drop() {
	if h.dropped {
		return
	}
	h.dropped = true
	if h.typ.destructor != nil {
		if h.typ.instance == h.instance {
			h.typ.destructor(h.instance.currentContext, h.rep)
		} else {
			if err := h.typ.instance.enter(h.instance.currentContext); err != nil {
				panic(fmt.Errorf("failed to enter instance during resource destructor: %w", err))
			}
			h.typ.destructor(h.instance.currentContext, h.rep)
			if err := h.typ.instance.exit(); err != nil {
				panic(fmt.Errorf("failed to exit instance during resource destructor: %w", err))
			}
		}
	}
}

func (h *ownHandle) Borrow() ResourceHandle {
	h.numLends++
	h.instance.borrowCount++
	return &borrowedHandle{
		typ: h.typ,
		rep: h.rep,
		onDrop: func() {
			h.instance.borrowCount--
			h.numLends--
		},
	}
}

func (h *ownHandle) Move(inst *Instance) (ResourceHandle, error) {
	if h.dropped {
		return nil, fmt.Errorf("cannot move dropped resource")
	}
	if h.numLends > 0 {
		return nil, fmt.Errorf("cannot move resource with active borrows")
	}
	h.dropped = true
	return &ownHandle{
		instance: inst,
		typ:      h.typ,
		rep:      h.rep,
	}, nil
}

type OwnType struct {
	ResourceType *ResourceType
}

func (t OwnType) isValueType() {}
func (t OwnType) supportsValue(v Value) bool {
	_, ok := v.(*ownHandle)
	if !ok {
		return false
	}
	return true
}

func (t OwnType) equalsType(other Type) bool {
	otherOwnType, ok := other.(OwnType)
	if !ok {
		return false
	}
	return t.ResourceType.equalsType(otherOwnType.ResourceType)
}
func (t OwnType) alignment() int   { return 4 }
func (t OwnType) elementSize() int { return 4 }
func (t OwnType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32}
}

func (t OwnType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	v := api.DecodeU32(itr())
	return t.lift(llc, v)
}

func (t OwnType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	v, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read resource handle index at offset %d", offset)
	}
	return t.lift(llc, v)
}

func (t OwnType) lift(llc *LiftLoadContext, handleIdx uint32) (Value, error) {
	h := llc.instance.loweredHandles.remove(handleIdx)
	if h.resourceType() != t.ResourceType {
		return nil, fmt.Errorf("resource handle type mismatch during lift: expected %p, got %p", t.ResourceType, h.resourceType())
	}
	if h.isBorrowed() {
		return nil, fmt.Errorf("cannot lift owned resource while it has active borrows")
	}

	return &ownHandle{
		instance: llc.instance,
		typ:      t.ResourceType,
		rep:      h,
	}, nil
}

func (t OwnType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	idx, err := t.lower(llc, val)
	if err != nil {
		return nil, err
	}
	return []uint64{uint64(idx)}, nil
}

func (t OwnType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	idx, err := t.lower(llc, val)
	if err != nil {
		return err
	}
	if !llc.memory.WriteUint32Le(offset, idx) {
		return fmt.Errorf("failed to write resource handle index at offset %d", offset)
	}
	return nil
}

func (t OwnType) lower(llc *LiftLoadContext, v Value) (uint32, error) {
	srcHandle := v.(*ownHandle)
	tgtHandle, err := srcHandle.Move(llc.instance)
	if err != nil {
		return 0, fmt.Errorf("failed to move resource handle during lower: %w", err)
	}
	if tgtHandle.resourceType() != t.ResourceType {
		return 0, fmt.Errorf("resource handle type mismatch during lower: expected %p, got %p", t.ResourceType, tgtHandle.resourceType())
	}

	idx := llc.instance.loweredHandles.add(tgtHandle)
	return idx, nil
}

type borrowedHandle struct {
	onDrop   func()
	typ      *ResourceType
	rep      any
	numLends int
	dropped  bool
}

func NewBorrowedHandle(typ *ResourceType, rep any, onDrop func()) ResourceHandle {
	return &borrowedHandle{
		typ:    typ,
		rep:    rep,
		onDrop: onDrop,
	}
}

func (h *borrowedHandle) isValue() {}

func (h *borrowedHandle) resourceType() *ResourceType {
	return h.typ
}

func (h *borrowedHandle) isBorrowed() bool {
	return h.numLends > 0
}

func (h *borrowedHandle) Resource() any {
	if h.dropped {
		panic("cannot use dropped resource")
	}
	return h.rep
}

func (h *borrowedHandle) Drop() {
	if h.dropped {
		return
	}
	if h.numLends > 0 {
		panic("cannot drop borrowed resource with active borrows")
	}
	h.dropped = true
	if h.onDrop != nil {
		h.onDrop()
	}
}
func (h *borrowedHandle) Borrow() ResourceHandle {
	h.numLends++
	return &borrowedHandle{
		typ: h.typ,
		rep: h.rep,
		onDrop: func() {
			h.numLends--
		},
	}
}

type BorrowType struct {
	ResourceType *ResourceType
}

func (t BorrowType) isValueType() {}
func (t BorrowType) supportsValue(v Value) bool {
	_, ok := v.(ResourceHandle)
	if !ok {
		return false
	}
	return true
}

func (t BorrowType) equalsType(other Type) bool {
	otherBorrowType, ok := other.(BorrowType)
	if !ok {
		return false
	}
	return t.ResourceType.equalsType(otherBorrowType.ResourceType)
}
func (t BorrowType) alignment() int   { return 4 }
func (t BorrowType) elementSize() int { return 4 }
func (t BorrowType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32}
}

func (t BorrowType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	v := api.DecodeU32(itr())
	return t.lift(llc, v)
}

func (t BorrowType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	v, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read resource handle index at offset %d", offset)
	}
	return t.lift(llc, v)
}

func (t BorrowType) lift(llc *LiftLoadContext, handleIdx uint32) (Value, error) {
	rh := llc.instance.loweredHandles.get(handleIdx)
	if rh.resourceType() != t.ResourceType {
		return nil, fmt.Errorf("resource handle type mismatch during lift: expected %p, got %p", t.ResourceType, rh.resourceType())
	}
	return rh, nil
}

func (t BorrowType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	idx, err := t.lower(llc, val)
	if err != nil {
		return nil, err
	}
	return []uint64{uint64(idx)}, nil
}

func (t BorrowType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	idx, err := t.lower(llc, val)
	if err != nil {
		return err
	}
	if !llc.memory.WriteUint32Le(offset, idx) {
		return fmt.Errorf("failed to write resource handle index at offset %d", offset)
	}
	return nil
}

func (t BorrowType) lower(llc *LiftLoadContext, v Value) (uint32, error) {
	borrowVal := v.(ResourceHandle)
	idx := llc.instance.loweredHandles.add(borrowVal.Borrow())
	return idx, nil
}

type ByteArray []byte

func (b ByteArray) isValue() {}

type ByteArrayType struct{}

func (t ByteArrayType) isValueType() {}

func (t ByteArrayType) supportsValue(v Value) bool {
	_, ok := v.(ByteArray)
	if !ok {
		return false
	}
	return true
}

func (t ByteArrayType) equalsType(other Type) bool {
	_, ok := other.(ByteArrayType)
	if !ok {
		return false
	}
	return true
}
func (t ByteArrayType) alignment() int   { return 4 }
func (t ByteArrayType) elementSize() int { return 8 }
func (t ByteArrayType) flatTypes() []api.ValueType {
	return []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
}

func (t ByteArrayType) liftFlat(llc *LiftLoadContext, itr func() uint64) (Value, error) {
	ptr := uint32(itr())
	length := uint32(itr())
	bytes, ok := llc.memory.Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("failed to read byte array at pointer %d with length %d", ptr, length)
	}
	return ByteArray(bytes), nil
}

func (t ByteArrayType) load(llc *LiftLoadContext, offset uint32) (Value, error) {
	ptr, ok := llc.memory.ReadUint32Le(offset)
	if !ok {
		return nil, fmt.Errorf("failed to read list pointer at offset %d", offset)
	}
	length, ok := llc.memory.ReadUint32Le(offset + 4)
	if !ok {
		return nil, fmt.Errorf("failed to read list length at offset %d", offset+4)
	}
	bytes, ok := llc.memory.Read(ptr, length)
	if !ok {
		return nil, fmt.Errorf("failed to read byte array at pointer %d with length %d", ptr, length)
	}
	return ByteArray(bytes), nil
}

func (t ByteArrayType) lowerFlat(llc *LiftLoadContext, val Value) ([]uint64, error) {
	ptr := llc.realloc(0, 0, 1, uint32(len(val.(ByteArray))))
	ok := llc.memory.Write(ptr, []byte(val.(ByteArray)))
	if !ok {
		return nil, fmt.Errorf("failed to write byte array at pointer %d", ptr)
	}
	return []uint64{uint64(ptr), uint64(len(val.(ByteArray)))}, nil
}

func (t ByteArrayType) store(llc *LiftLoadContext, offset uint32, val Value) error {
	aryVal := val.(ByteArray)
	ptr := llc.realloc(0, 0, 1, uint32(len(aryVal)))
	ok := llc.memory.Write(ptr, []byte(aryVal))
	if !ok {
		return fmt.Errorf("failed to write byte array at pointer %d", ptr)
	}
	ok = llc.memory.WriteUint32Le(offset, ptr)
	if !ok {
		return fmt.Errorf("failed to write byte array pointer at offset %d", offset)
	}
	ok = llc.memory.WriteUint32Le(offset+4, uint32(len(aryVal)))
	if !ok {
		return fmt.Errorf("failed to write byte array length at offset %d", offset+4)
	}
	return nil
}

func alignTo(value, alignment int) int {
	if alignment == 0 {
		return value
	}
	remainder := value % alignment
	if remainder == 0 {
		return value
	}
	return value + (alignment - remainder)
}

func joinFlatTypes(a, b api.ValueType) api.ValueType {
	if a == b {
		return a
	}

	if (a == api.ValueTypeI32 && b == api.ValueTypeF32) || (a == api.ValueTypeF32 && b == api.ValueTypeI32) {
		return api.ValueTypeI32
	}
	return api.ValueTypeI64
}
