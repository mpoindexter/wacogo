package host

import (
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type converter interface {
	toHost(componentmodel.Value) any
	fromHost(any) componentmodel.Value
	modelType() reflect.Type
}

type identityConverter struct {
	modelTyp reflect.Type
}

func (ic identityConverter) toHost(v componentmodel.Value) any {
	return v
}

func (ic identityConverter) fromHost(v any) componentmodel.Value {
	return v.(componentmodel.Value)
}

func (ic identityConverter) modelType() reflect.Type {
	return ic.modelTyp
}

type castConverter[M componentmodel.Value, H any] struct{}

func (cc castConverter[M, H]) toHost(v componentmodel.Value) any {
	return reflect.ValueOf(v).Convert(reflect.TypeFor[H]()).Interface()
}

func (cc castConverter[M, H]) fromHost(v any) componentmodel.Value {
	return reflect.ValueOf(v).Convert(reflect.TypeFor[M]()).Interface().(componentmodel.Value)
}

func (cc castConverter[M, H]) modelType() reflect.Type {
	return reflect.TypeFor[M]()
}

type recordConverter struct {
	typ reflect.Type
}

func (rc recordConverter) toHost(v componentmodel.Value) any {
	rec := v.(componentmodel.Record)
	rv := reflect.New(rc.typ)
	recordImplPtr := rv.Convert(reflect.TypeFor[*recordImpl]()).Interface().(*recordImpl)
	recordImplPtr.data = &recordData{
		record: rec,
	}
	return rv.Elem().Interface()
}

func (rc recordConverter) fromHost(v any) componentmodel.Value {
	ri := reflect.ValueOf(v).Convert(reflect.TypeFor[recordImpl]()).Interface().(recordImpl)
	if ri.data == nil {
		return componentmodel.Record{}
	}
	return ri.data.record
}

func (rc recordConverter) modelType() reflect.Type {
	return reflect.TypeFor[componentmodel.Record]()
}

type variantConverter struct {
	typ reflect.Type
}

func (vc variantConverter) toHost(v componentmodel.Value) any {
	variant := v.(*componentmodel.Variant)
	rv := reflect.New(vc.typ)
	variantImplPtr := rv.Convert(reflect.TypeFor[*variantImpl]()).Interface().(*variantImpl)
	variantImplPtr.value = variant
	return rv.Elem().Interface()
}

func (vc variantConverter) fromHost(v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	t := rv.Convert(reflect.TypeFor[variantImpl]()).Interface().(variantImpl)
	return t.value
}

func (vc variantConverter) modelType() reflect.Type {
	return reflect.TypeFor[*componentmodel.Variant]()
}

type enumConverter struct {
	typ reflect.Type
}

func (ec enumConverter) toHost(v componentmodel.Value) any {
	label := v.(*componentmodel.Variant).CaseLabel
	rv := reflect.New(ec.typ).Elem()
	rv.Set(reflect.ValueOf(label))
	return rv.Interface()
}

func (ec enumConverter) fromHost(v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	t := rv.Convert(reflect.TypeFor[string]()).Interface().(string)
	return &componentmodel.Variant{
		CaseLabel: t,
	}
}

func (ec enumConverter) modelType() reflect.Type {
	return reflect.TypeFor[*componentmodel.Variant]()
}

type flagsetConverter struct {
	typ reflect.Type
}

func (fc flagsetConverter) toHost(v componentmodel.Value) any {
	rv := reflect.New(fc.typ).Elem()
	rv.Set(reflect.ValueOf(v).Convert(fc.typ))
	return rv.Interface()
}

func (fc flagsetConverter) fromHost(v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	rv = rv.Convert(reflect.TypeFor[componentmodel.Flags]())
	return rv.Interface().(componentmodel.Value)
}

func (fc flagsetConverter) modelType() reflect.Type {
	return reflect.TypeFor[componentmodel.Flags]()
}

type listConverter struct {
	elemConverter converter
	typ           reflect.Type
}

func (lc *listConverter) toHost(v componentmodel.Value) any {
	srv := reflect.ValueOf(v)
	length := srv.Len()
	trv := reflect.MakeSlice(lc.typ, length, length)
	for i := 0; i < length; i++ {
		elemValue := srv.Index(i)
		hostElem := lc.elemConverter.toHost(elemValue.Interface().(componentmodel.Value))
		trv.Index(i).Set(reflect.ValueOf(hostElem))
	}
	return trv.Interface()
}

func (lc *listConverter) fromHost(v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	length := rv.Len()
	srv := reflect.MakeSlice(lc.elemConverter.modelType(), length, length)
	for i := 0; i < length; i++ {
		elemValue := rv.Index(i)
		modelElem := lc.elemConverter.fromHost(elemValue.Interface())
		srv.Index(i).Set(reflect.ValueOf(modelElem))
	}
	return srv.Interface().(componentmodel.Value)
}

func (lc listConverter) modelType() reflect.Type {
	return reflect.TypeFor[componentmodel.List]()
}

func converterFor(t reflect.Type) converter {
	switch t {
	case reflect.TypeFor[componentmodel.Bool](), reflect.TypeFor[componentmodel.U8](),
		reflect.TypeFor[componentmodel.U16](), reflect.TypeFor[componentmodel.U32](),
		reflect.TypeFor[componentmodel.U64](), reflect.TypeFor[componentmodel.S8](),
		reflect.TypeFor[componentmodel.S16](), reflect.TypeFor[componentmodel.S32](),
		reflect.TypeFor[componentmodel.S64](), reflect.TypeFor[componentmodel.F32](),
		reflect.TypeFor[componentmodel.F64](), reflect.TypeFor[componentmodel.String](),
		reflect.TypeFor[componentmodel.Char](), reflect.TypeFor[componentmodel.ByteArray]():
		return identityConverter{}
	}

	// Resource handles
	type handleType interface {
		ResourceType() reflect.Type
		HandleValueType(t *componentmodel.ResourceType) componentmodel.ValueType
	}

	if t.Implements(reflect.TypeFor[handleType]()) {
		return identityConverter{}
	}

	// Record
	if t.ConvertibleTo(reflect.TypeFor[recordImpl]()) {
		return recordConverter{
			typ: t,
		}
	}

	// Variant
	if t.ConvertibleTo(reflect.TypeFor[variantImpl]()) {
		return variantConverter{
			typ: t,
		}
	}

	// Enum
	if t.ConvertibleTo(reflect.TypeFor[string]()) && t.Implements(reflect.TypeFor[EnumValueProvider]()) {
		return enumConverter{
			typ: t,
		}
	}

	// Flags
	if t.ConvertibleTo(reflect.TypeFor[map[string]bool]()) && t.Implements(reflect.TypeFor[FlagsValueProvider]()) {
		return flagsetConverter{
			typ: t,
		}
	}

	// Lists
	if t.Kind() == reflect.Slice {
		if t.Elem().Kind() == reflect.Uint8 {
			return castConverter[componentmodel.ByteArray, []byte]{}
		}
		elemConverter := converterFor(t.Elem())
		if elemConverter != nil {
			return &listConverter{
				elemConverter: elemConverter,
				typ:           t,
			}
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return castConverter[componentmodel.Bool, bool]{}
	case reflect.Uint8:
		return castConverter[componentmodel.U8, uint8]{}
	case reflect.Uint16:
		return castConverter[componentmodel.U16, uint16]{}
	case reflect.Uint32:
		return castConverter[componentmodel.U32, uint32]{}
	case reflect.Uint64:
		return castConverter[componentmodel.U64, uint64]{}
	case reflect.Int8:
		return castConverter[componentmodel.S8, int8]{}
	case reflect.Int16:
		return castConverter[componentmodel.S16, int16]{}
	case reflect.Int32:
		return castConverter[componentmodel.S32, int32]{}
	case reflect.Int64:
		return castConverter[componentmodel.S64, int64]{}
	case reflect.Float32:
		return castConverter[componentmodel.F32, float32]{}
	case reflect.Float64:
		return castConverter[componentmodel.F64, float64]{}
	case reflect.String:
		return castConverter[componentmodel.String, string]{}
	}
	return nil
}
