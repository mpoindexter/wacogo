package host

import (
	"reflect"

	"github.com/partite-ai/wacogo/model"
)

type Variant[T VariantImpl] struct {
	value *model.Variant
}

type variantImpl struct {
	value *model.Variant
}

func VariantType(
	hi *Instance,
	cases ...*VariantCaseDef,
) *model.VariantType {
	variantCases := make([]*model.VariantCase, len(cases))
	for i, c := range cases {
		variantCases[i] = &model.VariantCase{
			Name: c.caseLabel,
			Type: c.valueType(hi),
		}
	}
	return &model.VariantType{
		Cases: variantCases,
	}
}

type VariantCaseDef struct {
	caseLabel string
	valueType func(hi *Instance) model.ValueType
}

type VariantImpl interface {
	~struct{ value *model.Variant }
	ValueTyped
}

func VariantCase[
	V VariantImpl,
	C func() V,
](
	constr C,
) *VariantCaseDef {
	v := (struct{ value *model.Variant })(constr())
	caseLabel := v.value.CaseLabel
	return &VariantCaseDef{
		caseLabel: caseLabel,
		valueType: func(hi *Instance) model.ValueType {
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
	v := (struct{ value *model.Variant })(constr(empty))
	caseLabel := v.value.CaseLabel
	return &VariantCaseDef{
		caseLabel: caseLabel,
		valueType: func(hi *Instance) model.ValueType {
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
		value: &model.Variant{
			CaseLabel: caseLabel,
			Value:     converter.fromHost(value),
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
		value: &model.Variant{
			CaseLabel: caseLabel,
		},
	}
	return (V)(vx)
}

func VariantCast[
	T any,
	V VariantImpl,
](
	v V,
	caseLabel string,
) (T, bool) {
	vv := (struct{ value *model.Variant })(v)

	if vv.value.CaseLabel != caseLabel {
		var zero T
		return zero, false
	}

	converter := converterFor(reflect.TypeFor[T]())
	return converter.toHost(vv.value.Value).(T), true
}

func VariantTest[
	V VariantImpl,
](
	v V,
	caseLabel string,
) bool {
	vv := (struct{ value *model.Variant })(v)
	return vv.value.CaseLabel == caseLabel
}
