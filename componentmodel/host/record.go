package host

import (
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Record[R RecordImpl] struct {
	data *recordData
}

type recordData struct {
	record          componentmodel.Record
	typeConstructor func(inst *Instance) (*componentmodel.RecordType, error)
}

type recordImpl struct {
	data *recordData
}

type RecordImpl interface {
	~struct {
		data *recordData
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
	rec, err := (Record[R])(ri).data.typeConstructor(hi)
	if err != nil {
		panic(fmt.Sprintf("RecordType: %v", err))
	}
	return rec
}

type RecordFieldValue struct {
	fieldName string
	fieldType func(hi *Instance) componentmodel.ValueType
	value     componentmodel.Value
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
	mv := converter.fromHost(value)
	return &RecordFieldValue{
		fieldName: name,
		fieldType: func(hi *Instance) componentmodel.ValueType {
			return ValueTypeFor[T](hi)
		},
		value: mv,
	}
}

func RecordConstruct[
	R RecordImpl,
](fields ...*RecordFieldValue) R {
	fieldValues := make([]componentmodel.Value, len(fields))
	for i, f := range fields {
		fieldValues[i] = f.value
	}

	return R{
		data: &recordData{
			record: componentmodel.NewRecord(fieldValues...),
			typeConstructor: func(inst *Instance) (*componentmodel.RecordType, error) {
				t := make([]*componentmodel.RecordField, len(fields))
				for i, f := range fields {
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
			},
		},
	}
}

func RecordFieldGetIndex[
	T any,
	R RecordImpl,
](r R, index int) T {
	record := (Record[R])(r).data.record
	converter := converterFor(reflect.TypeFor[T]())
	v := record.Field(index)
	return converter.toHost(v).(T)
}
