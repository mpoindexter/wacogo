package componentmodel

import "fmt"

const maxTableSize = 1 << 28

type table[T any] struct {
	entries []tableEntry[T]
	free    []uint32
}

func newTable[T any]() *table[T] {
	return &table[T]{
		entries: []tableEntry[T]{
			{
				set: false,
			},
		},
	}
}

func (t *table[T]) add(entry T) uint32 {
	if len(t.free) > 0 {
		idx := t.free[len(t.free)-1]
		t.free = t.free[:len(t.free)-1]
		t.entries[idx] = tableEntry[T]{
			value: entry,
			set:   true,
		}
		return idx
	}
	idx := uint32(len(t.entries))
	if idx >= maxTableSize {
		panic("table size exceeded")
	}
	t.entries = append(t.entries, tableEntry[T]{
		value: entry,
		set:   true,
	})
	return uint32(idx)
}

func (t *table[T]) get(idx uint32) T {
	if idx >= uint32(len(t.entries)) {
		panic("invalid table index")
	}
	entry := t.entries[idx]
	if !entry.set {
		panic(fmt.Sprintf("unknown handle index %d", idx))
	}
	return entry.value
}

func (t *table[T]) remove(idx uint32) T {
	if idx >= uint32(len(t.entries)) {
		panic("invalid table index")
	}
	entry := t.entries[idx]
	if !entry.set {
		panic(fmt.Sprintf("unknown handle index %d", idx))
	}
	v := entry.value
	var zero T
	t.entries[idx] = tableEntry[T]{set: false, value: zero}
	t.free = append(t.free, idx)

	return v
}

type tableEntry[T any] struct {
	value T
	set   bool
}
