package host

import (
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Record[R RecordImpl] struct {
	data recordAccessor
}

type recordAccessor interface {
	getField(index int) any
	toRecord(*callContext) componentmodel.Record
}

type hostRecordAccessor struct {
	values []*RecordFieldValue
}

func (hra *hostRecordAccessor) getField(index int) any {
	return hra.values[index]
}

func (hra *hostRecordAccessor) toRecord(cc *callContext) componentmodel.Record {
	fieldValues := make([]componentmodel.Value, len(hra.values))
	for i, v := range hra.values {
		fieldValues[i] = v.converter.fromHost(cc, v.value)
	}
	return componentmodel.NewRecord(fieldValues...)
}

func (hra *hostRecordAccessor) constructType(inst *Instance) (*componentmodel.RecordType, error) {
	t := make([]*componentmodel.RecordField, len(hra.values))
	for i, f := range hra.values {
		if f.err != nil {
			return nil, f.err
		}
		t[i] = &componentmodel.RecordField{
			Name: f.fieldName,
			Type: f.fieldType(inst),
		}
	}
	return &componentmodel.RecordType{
		Fields: t,
	}, nil
}

type componentRecordAccessor struct {
	cc     *callContext
	record componentmodel.Record
}

func (cra *componentRecordAccessor) getField(index int) any {
	mv := cra.record.Field(index)
	converter := converterFor(reflect.TypeOf(mv))
	value := converter.toHost(cra.cc, mv)
	return value
}

func (cra *componentRecordAccessor) toRecord(cc *callContext) componentmodel.Record {
	return cra.record
}

type recordImpl struct {
	data recordAccessor
}

type RecordImpl interface {
	~struct {
		data recordAccessor
	}
	ValueTyped
}

func RecordType[
	R RecordImpl,
](
	hi *Instance,
	constr any,
) *componentmodel.RecordType {
	rv := reflect.ValueOf(constr)
	if rv.Kind() != reflect.Func {
		panic("RecordType constructor must be a function")
	}
	numIn := rv.Type().NumIn()
	params := make([]reflect.Value, numIn)
	for i := range numIn {
		inType := rv.Type().In(i)
		params[i] = reflect.Zero(inType)
	}

	out := rv.Call(params)
	if len(out) != 1 {
		panic("RecordType constructor must return a single value")
	}
	recordImpl := out[0].Interface()
	ri, ok := recordImpl.(R)
	if !ok {
		panic("RecordType constructor returned wrong type")
	}
	rec, err := (Record[R])(ri).data.(*hostRecordAccessor).constructType(hi)
	if err != nil {
		panic(fmt.Sprintf("RecordType: %v", err))
	}
	return rec
}

type RecordFieldValue struct {
	fieldName string
	fieldType func(hi *Instance) componentmodel.ValueType
	value     any
	converter converter
	err       error
}

func RecordField[
	T any,
](name string, value T) *RecordFieldValue {
	converter := converterFor(reflect.TypeFor[T]())
	if converter == nil {
		return &RecordFieldValue{
			err: fmt.Errorf("RecordField: unsupported field type %T for field %s", value, name),
		}
	}
	return &RecordFieldValue{
		fieldName: name,
		fieldType: func(hi *Instance) componentmodel.ValueType {
			return ValueTypeFor[T](hi)
		},
		value:     value,
		converter: converter,
	}
}

func RecordConstruct[
	R RecordImpl,
](fields ...*RecordFieldValue) R {

	return R{
		data: &hostRecordAccessor{
			values: fields,
		},
	}

}

func RecordFieldGetIndex[
	T any,
	R RecordImpl,
](r R, index int) T {
	return (Record[R])(r).data.getField(index).(T)
}
