package host

import (
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Variant[T VariantImpl] struct {
	value variantAccessor
}

type variantImpl struct {
	value variantAccessor
}

type variantAccessor interface {
	hostValue() (string, func(converter converter) any)
	modelValue(cc *callContext) componentmodel.Value
}

type hostVariantAccessor struct {
	label     string
	value     any
	converter converter
}

func (hva *hostVariantAccessor) hostValue() (string, func(converter converter) any) {
	return hva.label, func(converter converter) any { return hva.value }
}

func (hva *hostVariantAccessor) modelValue(cc *callContext) componentmodel.Value {
	if hva.value == nil {
		return &componentmodel.Variant{
			CaseLabel: hva.label,
			Value:     nil,
		}
	}
	return &componentmodel.Variant{
		CaseLabel: hva.label,
		Value:     hva.converter.fromHost(cc, hva.value),
	}
}

type modelVariantAccessor struct {
	cc      *callContext
	variant *componentmodel.Variant
}

func (mva *modelVariantAccessor) hostValue() (string, func(converter converter) any) {
	return mva.variant.CaseLabel, func(converter converter) any {
		return converter.toHost(mva.cc, mva.variant.Value)
	}
}

func (mva *modelVariantAccessor) modelValue(cc *callContext) componentmodel.Value {
	return mva.variant
}

func VariantType(
	hi *Instance,
	cases ...*VariantCaseDef,
) *componentmodel.VariantType {
	variantCases := make([]*componentmodel.VariantCase, len(cases))
	for i, c := range cases {
		variantCases[i] = &componentmodel.VariantCase{
			Name: c.caseLabel,
			Type: c.valueType(hi),
		}
	}
	return &componentmodel.VariantType{
		Cases: variantCases,
	}
}

type VariantCaseDef struct {
	caseLabel string
	valueType func(hi *Instance) componentmodel.ValueType
}

type VariantImpl interface {
	~struct{ value variantAccessor }
	ValueTyped
}

func VariantCase[
	V VariantImpl,
	C func() V,
](
	constr C,
) *VariantCaseDef {
	v := (struct{ value variantAccessor })(constr())
	caseLabel, _ := v.value.hostValue()
	return &VariantCaseDef{
		caseLabel: caseLabel,
		valueType: func(hi *Instance) componentmodel.ValueType {
			return nil
		},
	}
}

func VariantCaseValue[
	V VariantImpl,
	T any,
	C func(T) V,
](
	constr C,
) *VariantCaseDef {
	var empty T
	v := (struct{ value variantAccessor })(constr(empty))
	caseLabel, _ := v.value.hostValue()
	return &VariantCaseDef{
		caseLabel: caseLabel,
		valueType: func(hi *Instance) componentmodel.ValueType {
			return ValueTypeFor[T](hi)
		},
	}
}

func VariantConstructValue[
	V VariantImpl,
	T any,
](
	caseLabel string,
	value T,
) V {
	converter := converterFor(reflect.TypeFor[T]())
	vx := Variant[V]{
		value: &hostVariantAccessor{
			label:     caseLabel,
			value:     value,
			converter: converter,
		},
	}
	return (V)(vx)
}

func VariantConstruct[
	V VariantImpl,
](
	caseLabel string,
) V {
	vx := Variant[V]{
		value: &hostVariantAccessor{
			label: caseLabel,
			value: nil,
		},
	}
	return (V)(vx)
}

func VariantCast[
	T any,
	V VariantImpl,
](
	v V,
	caseLabelPredicate string,
) (T, bool) {
	vv := (struct{ value variantAccessor })(v)

	converter := converterFor(reflect.TypeFor[T]())
	caseLabel, getValue := vv.value.hostValue()

	if caseLabel != caseLabelPredicate {
		var zero T
		return zero, false
	}

	return getValue(converter).(T), true
}

func VariantTest[
	V VariantImpl,
](
	v V,
	caseLabel string,
) bool {
	vv := (struct{ value variantAccessor })(v)
	label, _ := vv.value.hostValue()
	return label == caseLabel
}
