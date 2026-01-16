package host

import (
	"reflect"
	"runtime"

	"github.com/partite-ai/wacogo/componentmodel"
)

type resourceTyped interface {
	resourceType() reflect.Type
}

type lease interface {
	resource() any
	release()
}

type valueLease struct {
	released bool
	rsc      any
}

func (l *valueLease) resource() any {
	if l.released {
		panic("attempted to use released value lease")
	}
	return l.rsc
}

func (l *valueLease) release() {
	if l.released {
		return
	}
	l.released = true
}

type handleLease struct {
	handle   componentmodel.ResourceHandle
	released bool
}

func (l *handleLease) resource() any {
	if l.released {
		panic("attempted to use released handle lease")
	}
	return l.handle.Resource()
}

func (l *handleLease) release() {
	if l.released {
		return
	}
	l.released = true
	l.handle.Drop()
}

type borrowedLease struct {
	released bool
	d        *handleData
}

func (l *borrowedLease) resource() any {
	return l.d.lease.resource()
}

func (l *borrowedLease) release() {
	if l.released {
		return
	}
	l.released = true
	l.d.numLends--
}

type handleData struct {
	lease    lease
	numLends int
	dropped  bool
}

type Own[T any] ownImpl

type ownImpl struct {
	data *handleData
}

func NewOwn[T any](resource T) Own[T] {
	lease := &valueLease{
		rsc: resource,
	}
	return newOwn[T](lease)
}

func newOwn[T any](l lease) Own[T] {
	data := &handleData{
		lease: l,
	}
	runtime.AddCleanup(data, func(l lease) {
		l.release()
	}, l)
	return Own[T]{
		data: data,
	}
}

func (t Own[T]) Resource() T {
	if t.data.dropped {
		panic("attempted to use dropped own handle")
	}
	return t.data.lease.resource().(T)
}

func (t Own[T]) Borrow() Borrow[T] {
	if t.data.dropped {
		panic("attempted to borrow from dropped own handle")
	}
	t.data.numLends++
	return Borrow[T]{
		data: &handleData{
			lease: &borrowedLease{
				d: t.data,
			},
		},
	}
}

func (t Own[T]) Drop() {
	if t.data.numLends > 0 {
		panic("attempted to drop own handle with active borrows")
	}
	if t.data.dropped {
		return
	}
	t.data.dropped = true
	t.data.lease.release()
}

func (Own[T]) resourceType() reflect.Type {
	return reflect.TypeFor[T]()
}

func (Own[T]) handleValueType(rt *componentmodel.ResourceType) componentmodel.ValueType {
	return componentmodel.OwnType{ResourceType: rt}
}

func (Own[T]) isOwnHandle() {}

type Borrow[T any] borrowImpl

type borrowImpl struct {
	data *handleData
}

func (t Borrow[T]) Resource() T {
	if t.data.dropped {
		panic("attempted to use dropped borrow handle")
	}
	return t.data.lease.resource().(T)
}

func (Borrow[T]) resourceType() reflect.Type {
	return reflect.TypeFor[T]()
}

func (Borrow[T]) handleValueType(rt *componentmodel.ResourceType) componentmodel.ValueType {
	return componentmodel.BorrowType{ResourceType: rt}
}

func (Borrow[T]) isBorrowHandle() {}
