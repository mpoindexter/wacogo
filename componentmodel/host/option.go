package host

import "github.com/partite-ai/wacogo/componentmodel"

type Option[T any] Variant[Option[T]]

func OptionSome[T any](t T) Option[T] {
	return VariantConstructValue[Option[T]](
		"some",
		t,
	)
}

func (v Option[T]) Some() (T, bool) {
	return VariantCast[T](v, "some")
}

func OptionNone[T any]() Option[T] {
	return VariantConstruct[Option[T]](
		"none",
	)
}

func (v Option[T]) None() bool {
	return VariantTest(v, "none")
}

func (Option[T]) ValueType(inst *Instance) componentmodel.ValueType {
	return componentmodel.NewOptionType(ValueTypeFor[T](inst))
}
