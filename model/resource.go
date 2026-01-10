package model

type resourceHandle[T any] struct {
	typ      *ResourceType
	rep      T
	own      bool
	numLends int
}

func newResourceHandle[T any](typ *ResourceType, rep T, own bool) *resourceHandle[T] {
	return &resourceHandle[T]{
		typ: typ,
		rep: rep,
		own: own,
	}
}
