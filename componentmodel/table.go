package componentmodel

import "reflect"

type tables map[reflect.Type]any

func newTables() tables {
	return make(tables)
}

func getTable[T any](inst *Instance) *Table[T] {
	typ := reflect.TypeFor[T]()
	table, ok := inst.tables[typ]
	if !ok {
		newTable := newTable[T]()
		inst.tables[typ] = newTable
		return newTable
	}
	return table.(*Table[T])
}

const maxTableSize = 1 << 28

type Table[T any] struct {
	entries []tableEntry[T]
	free    []uint32
}

func newTable[T any]() *Table[T] {
	return &Table[T]{
		entries: []tableEntry[T]{
			{
				set: false,
			},
		},
	}
}

func (t *Table[T]) Add(entry T) uint32 {
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

func (t *Table[T]) Get(idx uint32) T {
	if idx >= uint32(len(t.entries)) {
		panic("invalid table index")
	}
	entry := t.entries[idx]
	if !entry.set {
		panic("table entry not set")
	}
	return entry.value
}

func (t *Table[T]) Remove(idx uint32) T {
	if idx >= uint32(len(t.entries)) {
		panic("invalid table index")
	}
	entry := t.entries[idx]
	if !entry.set {
		panic("table entry not set")
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
