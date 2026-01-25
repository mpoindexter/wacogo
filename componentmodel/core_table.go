package componentmodel

import (
	"context"
	"fmt"

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

type coreTypeTableDefinition struct {
	elementType definition[Type, Type]
	min         uint32
	max         *uint32
}

func newCoreTypeTableDefinition(elementType definition[Type, Type], min uint32, max *uint32) *coreTypeTableDefinition {
	return &coreTypeTableDefinition{
		elementType: elementType,
		min:         min,
		max:         max,
	}
}

func (d *coreTypeTableDefinition) typ() Type {
	return newCoreTableType(
		d.elementType.typ(),
		d.min,
		d.max,
	)
}

func (d *coreTypeTableDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	elemType, err := d.elementType.resolve(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve table element type: %w", err)
	}
	return newCoreTableType(
		elemType,
		d.min,
		d.max,
	), nil
}
