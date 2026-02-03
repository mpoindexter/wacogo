package host

import (
	"reflect"
	"strings"
	"sync"

	"github.com/partite-ai/wacogo/componentmodel"
)

var recordMetadataCache sync.Map

type Record[RF any] struct {
	*recordImpl[RF]
}

type RecordType interface {
	isRecord()
}

type ConstructableRecord[RF any] interface {
	~struct {
		*recordImpl[RF]
	}
	RecordType
	impl() *recordImpl[RF]
}

type SettableRecord[R ConstructableRecord[RF], RF any] struct {
	*recordImpl[RF]
}

func (sr SettableRecord[R, RF]) Record() R {
	return R{recordImpl: sr.recordImpl}
}

func (sr SettableRecord[R, RF]) settableRecord(R) {}

type recordImpl[RF any] struct {
	Fields         *RF
	recordAccessor recordAccessor
}

func (ri *recordImpl[RF]) init(target any, cc *callContext, rec componentmodel.Record) {
	md := recordMetadataFor[RF]()
	target.(*Record[RF]).recordImpl = &recordImpl[RF]{
		Fields: md.fields.(*RF),
		recordAccessor: &componentRecordAccessor{
			md:     md,
			cc:     cc,
			record: rec,
		},
	}
}

func (ri *recordImpl[RF]) getField(index int) any {
	return ri.recordAccessor.getField(index)
}

func (ri *recordImpl[RF]) setField(index int, value any) {
	ri.recordAccessor.setField(index, value)
}

func (ri *recordImpl[RF]) toRecord(cc *callContext) componentmodel.Record {
	return ri.recordAccessor.toRecord(cc)
}

func (ri *recordImpl[RF]) recordMeta() *recordMetadata {
	return recordMetadataFor[RF]()
}

func (ri *recordImpl[RF]) impl() *recordImpl[RF] {
	return ri
}

func (*recordImpl[RF]) isRecord() {}

type RecordField[R RecordType, V any] struct {
	fieldIndex int
}

func (rf *RecordField[R, V]) Get(instance R) V {
	getter := any(instance).(interface {
		getField(int) any
	})
	v := getter.getField(rf.fieldIndex)
	return v.(V)
}

type FieldSettableRecord[R RecordType] interface {
	settableRecord(R)
}

func (rf *RecordField[R, V]) Set(instance FieldSettableRecord[R], value V) {
	setter := any(instance).(interface {
		setField(int, any)
	})
	setter.setField(rf.fieldIndex, value)
}

func (rf *RecordField[R, V]) initField(idx int, name string) *fieldMetadata {
	rf.fieldIndex = idx
	return &fieldMetadata{
		name:      name,
		converter: converterFor(reflect.TypeFor[V]()),
		createFieldType: func(hi *Instance) componentmodel.ValueType {
			return ValueTypeFor[V](hi)
		},
		zeroValue: func() any {
			var zero V
			return zero
		},
	}
}

func NewRecord[R ConstructableRecord[RF], RF any]() SettableRecord[R, RF] {
	md := recordMetadataFor[RF]()
	vs := make([]any, len(md.fieldMetas))
	for i, fm := range md.fieldMetas {
		vs[i] = fm.zeroValue()
	}
	r := Record[RF]{
		recordImpl: &recordImpl[RF]{
			Fields: md.fields.(*RF),
			recordAccessor: &hostRecordAccessor{
				md:     md,
				values: vs,
			},
		},
	}

	return SettableRecord[R, RF]{r.impl()}
}

type recordAccessor interface {
	getField(index int) any
	setField(index int, value any)
	toRecord(*callContext) componentmodel.Record
}

type hostRecordAccessor struct {
	md     *recordMetadata
	values []any
}

func (hra *hostRecordAccessor) getField(index int) any {
	return hra.values[index]
}

func (hra *hostRecordAccessor) setField(index int, value any) {
	hra.values[index] = value
}

func (hra *hostRecordAccessor) toRecord(cc *callContext) componentmodel.Record {
	fieldValues := make([]componentmodel.Value, len(hra.values))
	for i, v := range hra.values {
		fieldValues[i] = hra.md.fieldMetas[i].converter.fromHost(cc, v)
	}
	return componentmodel.NewRecord(fieldValues...)
}

type componentRecordAccessor struct {
	cc     *callContext
	record componentmodel.Record
	md     *recordMetadata
}

func (cra *componentRecordAccessor) getField(index int) any {
	mv := cra.record.Field(index)
	converter := cra.md.fieldMetas[index].converter
	value := converter.toHost(cra.cc, mv)
	return value
}

func (cra *componentRecordAccessor) setField(index int, value any) {
	panic("componentRecordAccessor does not support setField")
}

func (cra *componentRecordAccessor) toRecord(cc *callContext) componentmodel.Record {
	return cra.record
}

type fieldMetadata struct {
	name            string
	converter       converter
	createFieldType func(hi *Instance) componentmodel.ValueType
	zeroValue       func() any
}

type recordMetadata struct {
	fieldMetas []*fieldMetadata
	fields     any
}

func recordMetadataFor[RF any]() *recordMetadata {
	typ := reflect.TypeFor[RF]()
	cacheEntry, ok := recordMetadataCache.Load(typ)
	if ok {
		md := cacheEntry.(*recordMetadata)
		return md
	}

	var md recordMetadata
	var fields RF
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, ok := field.Tag.Lookup("cm")
		if !ok {
			name = strings.ToLower(field.Name)
		}
		initer, ok := reflect.ValueOf(&fields).Elem().Field(i).Addr().Interface().(interface {
			initField(int, string) *fieldMetadata
		})
		if ok {
			md.fieldMetas = append(md.fieldMetas, initer.initField(i, name))
		}
	}
	md.fields = &fields
	recordMetadataCache.Store(typ, &md)
	return &md
}
