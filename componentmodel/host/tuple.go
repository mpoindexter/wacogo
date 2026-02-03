package host

import (
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Tuple[TF any] struct {
	*tupleImpl[TF]
}

type TupleType interface {
	isTuple()
}

type ConstructableTuple[TF any] interface {
	~struct {
		*tupleImpl[TF]
	}
	TupleType
	impl() *tupleImpl[TF]
}

type SettableTuple[T TupleType, TF any] struct {
	*tupleImpl[TF]
}

func (sr SettableTuple[T, TF]) Tuple() T {
	return any(Tuple[TF]{sr.tupleImpl}).(T)
}

func (sr SettableTuple[T, TF]) settableTuple(T) {}

type tupleImpl[TF any] struct {
	Fields        *TF
	tupleAccessor recordAccessor
}

func (ti *tupleImpl[TF]) impl() *tupleImpl[TF] {
	return ti
}

func (ti *tupleImpl[TF]) recordMeta() *recordMetadata {
	return recordMetadataFor[TF]()
}

func (ti *tupleImpl[TF]) init(target any, cc *callContext, rec componentmodel.Record) {
	md := recordMetadataFor[TF]()
	target.(*Tuple[TF]).tupleImpl = &tupleImpl[TF]{
		Fields: md.fields.(*TF),
		tupleAccessor: &hostRecordAccessor{
			md:     md,
			values: make([]any, len(md.fieldMetas)),
		},
	}
}

func (ti *tupleImpl[TF]) getField(index int) any {
	return ti.tupleAccessor.getField(index)
}

func (ti *tupleImpl[TF]) setField(index int, value any) {
	ti.tupleAccessor.setField(index, value)
}

func (*tupleImpl[TF]) isTuple() {}

type TupleField[T TupleType, V any] struct {
	fieldIndex int
	name       string
	converter  converter
}

func (tf *TupleField[T, V]) Get(instance T) V {
	getter := any(instance).(interface {
		getField(int) any
	})
	v := getter.getField(tf.fieldIndex)
	return v.(V)
}

type FieldSettableTuple[T TupleType] interface {
	settableTuple(T)
}

func (tf *TupleField[T, V]) Set(instance FieldSettableTuple[T], value V) {
	setter := any(instance).(interface {
		setField(int, any)
	})
	setter.setField(tf.fieldIndex, value)
}

func (tf *TupleField[T, V]) initField(idx int, name string) *fieldMetadata {
	tf.fieldIndex = idx
	return &fieldMetadata{
		name:      name,
		converter: converterFor(reflect.TypeFor[V]()),
		zeroValue: func() any { var v V; return v },
		createFieldType: func(hi *Instance) componentmodel.ValueType {
			return ValueTypeFor[V](hi)
		},
	}
}

func NewTuple[T ConstructableTuple[TF], TF any]() SettableTuple[T, TF] {
	md := recordMetadataFor[TF]()
	t := Tuple[TF]{
		tupleImpl: &tupleImpl[TF]{
			Fields: md.fields.(*TF),
			tupleAccessor: &hostRecordAccessor{
				md:     md,
				values: make([]any, len(md.fieldMetas)),
			},
		},
	}

	return SettableTuple[T, TF]{t.impl()}
}
