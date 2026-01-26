package componentmodel

import (
	"github.com/partite-ai/wacogo/wasm"
	"github.com/tetratelabs/wazero/api"
)

type coreTable struct {
	module api.Module
	name   string
	def    *wasm.TableType
}

func newCoreTable(module api.Module, name string, def *wasm.TableType) *coreTable {
	return &coreTable{
		module: module,
		name:   name,
		def:    def,
	}
}

func (t *coreTable) typ() *coreTableType {
	et, _ := coreTypeWasmConstTypeFromWasmParser(t.def.ElemType)
	min := t.def.Limits.Min
	var max *uint32
	if t.def.Limits.HasMax {
		max = &t.def.Limits.Max
	}
	return newCoreTableType(et, min, max)
}

type coreTableType struct {
	elementType Type
	min         uint32
	max         *uint32
}

func newCoreTableType(elementType Type, min uint32, max *uint32) *coreTableType {
	return &coreTableType{
		elementType: elementType,
		min:         min,
		max:         max,
	}
}

func (t *coreTableType) typ() Type {
	return t
}

func (t *coreTableType) assignableFrom(other Type) bool {
	otherTable, ok := other.(*coreTableType)
	if !ok {
		return false
	}
	if !t.elementType.assignableFrom(otherTable.elementType) {
		return false
	}
	if t.min != otherTable.min {
		return false
	}
	if (t.max == nil) != (otherTable.max == nil) {
		return false
	}
	if t.max != nil && otherTable.max != nil && *t.max != *otherTable.max {
		return false
	}
	return true
}
