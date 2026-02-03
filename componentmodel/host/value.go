package host

import (
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Convertable interface {
	ToHost(componentmodel.Value) any
	FromHost(any) componentmodel.Value
}

type callContext struct {
	instance     *componentmodel.Instance
	hostInstance *Instance
	cleanups     []func()
}

type converter interface {
	toHost(callCtx *callContext, v componentmodel.Value) any
	fromHost(callCtx *callContext, v any) componentmodel.Value
}

type identityConverter struct {
}

func (ic identityConverter) toHost(cc *callContext, v componentmodel.Value) any {
	return v
}

func (ic identityConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	return v.(componentmodel.Value)
}

type convertableConverter struct {
	typ reflect.Type
}

func (c convertableConverter) toHost(cc *callContext, v componentmodel.Value) any {
	return reflect.Zero(c.typ).Interface().(Convertable).ToHost(v)
}

func (c convertableConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	return v.(Convertable).FromHost(v)
}

type ownHandleConverter struct {
	handleTyp    reflect.Type
	resourceType reflect.Type
}

func (hc *ownHandleConverter) toHost(cc *callContext, v componentmodel.Value) any {
	mv := v.(interface {
		Move(*componentmodel.Instance) (componentmodel.ResourceHandle, error)
	})
	tgtHandle, err := mv.Move(cc.instance)
	if err != nil {
		panic("failed to move resource handle during conversion to host")
	}
	inst := reflect.New(hc.handleTyp)
	ownImplPtr := inst.Convert(reflect.TypeFor[*ownImpl]()).Interface().(*ownImpl)
	ownImplPtr.data = &handleData{
		lease: &handleLease{
			handle: tgtHandle,
		},
	}
	return inst.Elem().Interface()
}

func (hc *ownHandleConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	inst := reflect.ValueOf(v).Convert(reflect.TypeFor[ownImpl]()).Interface().(ownImpl)
	inst.data.dropped = true
	rt := cc.hostInstance.resourceTypes[hc.resourceType]
	return componentmodel.NewResourceHandle(cc.instance, rt, inst.data.lease.resource())
}

type borrowHandleConverter struct {
	handleTyp    reflect.Type
	resourceType reflect.Type
}

func (hc *borrowHandleConverter) toHost(cc *callContext, v componentmodel.Value) any {
	mv := v.(componentmodel.ResourceHandle)
	inst := reflect.New(hc.handleTyp)
	borrowImplPtr := inst.Convert(reflect.TypeFor[*borrowImpl]()).Interface().(*borrowImpl)
	data := &handleData{
		lease: &handleLease{
			handle: mv,
		},
	}
	borrowImplPtr.data = data
	cc.cleanups = append(cc.cleanups, func() {
		data.dropped = true
		mv.Drop()
	})
	return inst.Elem().Interface()
}

func (hc *borrowHandleConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	inst := reflect.ValueOf(v).Convert(reflect.TypeFor[borrowImpl]()).Interface().(borrowImpl)
	rt := cc.hostInstance.resourceTypes[hc.resourceType]
	h := componentmodel.NewBorrowedHandle(rt, inst.data.lease.resource(), func() {
		inst.data.lease.release()
	})
	return h
}

type castConverter[M componentmodel.Value, H any] struct{}

func (castConverter[M, H]) toHost(cc *callContext, v componentmodel.Value) any {
	return reflect.ValueOf(v).Convert(reflect.TypeFor[H]()).Interface()
}

func (castConverter[M, H]) fromHost(cc *callContext, v any) componentmodel.Value {
	return reflect.ValueOf(v).Convert(reflect.TypeFor[M]()).Interface().(componentmodel.Value)
}

type recordConverter struct {
	typ reflect.Type
}

func (rc recordConverter) toHost(cc *callContext, v componentmodel.Value) any {
	rec := v.(componentmodel.Record)
	rv := reflect.New(rc.typ)
	initer := rv.Interface().(interface {
		init(target any, cc *callContext, rec componentmodel.Record)
	})
	initer.init(rv.Interface(), cc, rec)
	return rv.Elem().Interface()
}

func (rc recordConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	toRecorder := v.(interface {
		toRecord(*callContext) componentmodel.Record
	})
	return toRecorder.toRecord(cc)
}

type variantConverter struct {
	typ reflect.Type
}

func (vc variantConverter) toHost(cc *callContext, v componentmodel.Value) any {
	variant := v.(*componentmodel.Variant)
	rv := reflect.New(vc.typ)
	variantImplPtr := rv.Convert(reflect.TypeFor[*variantImpl]()).Interface().(*variantImpl)
	variantImplPtr.value = &modelVariantAccessor{
		variant: variant,
		cc:      cc,
	}
	return rv.Elem().Interface()
}

func (vc variantConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	t := rv.Convert(reflect.TypeFor[variantImpl]()).Interface().(variantImpl)
	return t.value.modelValue(cc)
}

type enumConverter struct {
	typ reflect.Type
}

func (ec enumConverter) toHost(cc *callContext, v componentmodel.Value) any {
	label := v.(*componentmodel.Variant).CaseLabel
	rv := reflect.New(ec.typ).Elem()
	rv.Set(reflect.ValueOf(label))
	return rv.Interface()
}

func (ec enumConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	t := rv.Convert(reflect.TypeFor[string]()).Interface().(string)
	return &componentmodel.Variant{
		CaseLabel: t,
	}
}

type flagsetConverter struct {
	typ reflect.Type
}

func (fc flagsetConverter) toHost(cc *callContext, v componentmodel.Value) any {
	rv := reflect.New(fc.typ).Elem()
	rv.Set(reflect.ValueOf(v).Convert(fc.typ))
	return rv.Interface()
}

func (fc flagsetConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	rv = rv.Convert(reflect.TypeFor[componentmodel.Flags]())
	return rv.Interface().(componentmodel.Value)
}

type listConverter struct {
	elemConverter converter
	typ           reflect.Type
}

func (lc *listConverter) toHost(cc *callContext, v componentmodel.Value) any {
	srv := reflect.ValueOf(v)
	length := srv.Len()
	trv := reflect.MakeSlice(lc.typ, length, length)
	for i := range length {
		elemValue := srv.Index(i)
		hostElem := lc.elemConverter.toHost(cc, elemValue.Interface().(componentmodel.Value))
		trv.Index(i).Set(reflect.ValueOf(hostElem))
	}
	return trv.Interface()
}

func (lc *listConverter) fromHost(cc *callContext, v any) componentmodel.Value {
	rv := reflect.ValueOf(v)
	length := rv.Len()
	srv := reflect.MakeSlice(reflect.TypeFor[componentmodel.List](), length, length)
	for i := range length {
		elemValue := rv.Index(i)
		modelElem := lc.elemConverter.fromHost(cc, elemValue.Interface())
		srv.Index(i).Set(reflect.ValueOf(modelElem))
	}
	return srv.Interface().(componentmodel.Value)
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

	// Owned Resource Handle
	if t.Implements(reflect.TypeFor[interface {
		isOwnHandle()
		resourceTyped
	}]()) {
		rt := reflect.Zero(t).Interface().(resourceTyped).resourceType()
		return &ownHandleConverter{
			handleTyp:    t,
			resourceType: rt,
		}
	}

	// Borrowed Resource Handle
	if t.Implements(reflect.TypeFor[interface {
		isBorrowHandle()
		resourceTyped
	}]()) {
		rt := reflect.Zero(t).Interface().(resourceTyped).resourceType()
		return &borrowHandleConverter{
			handleTyp:    t,
			resourceType: rt,
		}
	}

	// Record
	if t.ConvertibleTo(reflect.TypeFor[RecordType]()) {
		return recordConverter{
			typ: t,
		}
	}

	// Tuple
	if t.ConvertibleTo(reflect.TypeFor[TupleType]()) {
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

	if t.ConvertibleTo(reflect.TypeFor[Convertable]()) {
		return convertableConverter{
			typ: t,
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
