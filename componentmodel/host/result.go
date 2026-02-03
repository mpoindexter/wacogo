package host

import "github.com/partite-ai/wacogo/componentmodel"

type Result[O, E any] Variant[Result[O, E]]

func (Result[O, E]) ValueType(inst *Instance) componentmodel.ValueType {
	return componentmodel.NewResultType(
		ValueTypeFor[O](inst),
		ValueTypeFor[E](inst),
	)
}

func ResultOk[E, O any](value O) Result[O, E] {
	return VariantConstructValue[Result[O, E]](
		"ok",
		value,
	)
}

func (v Result[O, E]) Ok() (O, bool) {
	return VariantCast[O](v, "ok")
}

func ResultErr[O, E any](err E) Result[O, E] {
	return VariantConstructValue[Result[O, E]](
		"error",
		err,
	)
}

func (v Result[O, E]) Err() (E, bool) {
	return VariantCast[E](v, "error")
}
