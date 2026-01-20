package p2

import (
	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type Option[T any] host.Variant[Option[T]]

func (Option[T]) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCase[Option[T]](OptionNone),
		host.VariantCaseValue(OptionSome[T]),
	)
}

func OptionSome[T any](t T) Option[T] {
	return host.VariantConstructValue[Option[T]](
		"some",
		t,
	)
}

func (v Option[T]) Some() (T, bool) {
	return host.VariantCast[T](v, "some")
}

func OptionNone[T any]() Option[T] {
	return host.VariantConstruct[Option[T]](
		"none",
	)
}

func (v Option[T]) None() bool {
	return host.VariantTest(v, "none")
}

type Void struct{}

func (Void) ValueType(inst *host.Instance) componentmodel.ValueType {
	return nil
}

func (Void) ToHost(v componentmodel.Value) any {
	return nil
}

func (Void) FromHost(v any) componentmodel.Value {
	return nil
}

type Result[O, E any] host.Variant[Result[O, E]]

func (Result[O, E]) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCaseValue(ResultOk[E, O]),
		host.VariantCaseValue(ResultErr[O, E]),
	)
}

func ResultOk[E, O any](value O) Result[O, E] {
	return host.VariantConstructValue[Result[O, E]](
		"ok",
		value,
	)
}

func (v Result[O, E]) Ok() (O, bool) {
	return host.VariantCast[O](v, "ok")
}

func ResultErr[O, E any](err E) Result[O, E] {
	return host.VariantConstructValue[Result[O, E]](
		"error",
		err,
	)
}

func (v Result[O, E]) Err() (E, bool) {
	return host.VariantCast[E](v, "error")
}

type Tuple2[A, B any] host.Record[Tuple2[A, B]]

func (Tuple2[A, B]) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[Tuple2[A, B]](
		inst,
		NewTuple2[A, B],
	)
}

func NewTuple2[A, B any](a A, b B) Tuple2[A, B] {
	return host.RecordConstruct[Tuple2[A, B]](
		host.RecordField("", a),
		host.RecordField("", b),
	)
}

func (t Tuple2[A, B]) A() A {
	return host.RecordFieldGetIndex[A](t, 0)
}

func (t Tuple2[A, B]) B() B {
	return host.RecordFieldGetIndex[B](t, 1)
}

type Tuple3[A, B, C any] host.Record[Tuple3[A, B, C]]

func (Tuple3[A, B, C]) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[Tuple3[A, B, C]](
		inst,
		NewTuple3[A, B, C],
	)
}

func NewTuple3[A, B, C any](a A, b B, c C) Tuple3[A, B, C] {
	return host.RecordConstruct[Tuple3[A, B, C]](
		host.RecordField("", a),
		host.RecordField("", b),
		host.RecordField("", c),
	)
}

func (t Tuple3[A, B, C]) A() A {
	return host.RecordFieldGetIndex[A](t, 0)
}

func (t Tuple3[A, B, C]) B() B {
	return host.RecordFieldGetIndex[B](t, 1)
}

func (t Tuple3[A, B, C]) C() C {
	return host.RecordFieldGetIndex[C](t, 2)
}
