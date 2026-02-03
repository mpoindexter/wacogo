package p2

import (
	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type Option[T any] = host.Option[T]

func OptionSome[T any](t T) Option[T] {
	return host.OptionSome(t)
}
func OptionNone[T any]() Option[T] {
	return host.OptionNone[T]()
}

type Result[O, E any] = host.Result[O, E]

func ResultOk[E, O any](value O) Result[O, E] {
	return host.ResultOk[E](value)
}

func ResultErr[O, E any](err E) Result[O, E] {
	return host.ResultErr[O](err)
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

type Tuple2[A, B any] host.Tuple[struct {
	A host.TupleField[Tuple2[A, B], A]
	B host.TupleField[Tuple2[A, B], B]
}]

func NewTuple2[A, B any](a A, b B) Tuple2[A, B] {
	tpl := host.NewTuple[Tuple2[A, B]]()
	tpl.Fields.A.Set(tpl, a)
	tpl.Fields.B.Set(tpl, b)
	return tpl.Tuple()
}

func (t Tuple2[A, B]) A() A {
	return t.Fields.A.Get(t)
}

func (t Tuple2[A, B]) B() B {
	return t.Fields.B.Get(t)
}

type Tuple3[A, B, C any] host.Tuple[struct {
	A host.TupleField[Tuple3[A, B, C], A]
	B host.TupleField[Tuple3[A, B, C], B]
	C host.TupleField[Tuple3[A, B, C], C]
}]

func NewTuple3[A, B, C any](a A, b B, c C) Tuple3[A, B, C] {
	tpl := host.NewTuple[Tuple3[A, B, C]]()
	tpl.Fields.A.Set(tpl, a)
	tpl.Fields.B.Set(tpl, b)
	tpl.Fields.C.Set(tpl, c)
	return tpl.Tuple()
}

func (t Tuple3[A, B, C]) A() A {
	return t.Fields.A.Get(t)
}

func (t Tuple3[A, B, C]) B() B {
	return t.Fields.B.Get(t)
}

func (t Tuple3[A, B, C]) C() C {
	return t.Fields.C.Get(t)
}
