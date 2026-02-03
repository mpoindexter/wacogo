package componentmodel

import (
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

func (c *coreTableType) isType() {}

func (t *coreTableType) typeName() string {
	return "core table"
}

func (t *coreTableType) checkType(other Type, typeChecker typeChecker) error {
	ot, err := assertTypeKindIsSame(t, other)
	if err != nil {
		return err
	}
	if err := typeChecker.checkTypeCompatible(t.elementType, ot.elementType); err != nil {
		return fmt.Errorf("expected table element type %s, found %s", t.elementType.typeName(), ot.elementType.typeName())
	}
	if t.min > ot.min {
		return fmt.Errorf("type mismatch: mismatch in table limits: table minimum size %d greater than %d", t.min, ot.min)
	}

	if t.max != nil {
		if ot.max == nil {
			return fmt.Errorf("type mismatch: mismatch in table limits: expected maximum size %d, found unbounded", *t.max)
		}
		if *t.max < *ot.max {
			return fmt.Errorf("type mismatch: mismatch in table limits: table maximum size %d less than %d", *t.max, *ot.max)
		}
	}
	return nil
}

func (t *coreTableType) typeSize() int {
	return 1 + t.elementType.typeSize()
}

func (t *coreTableType) typeDepth() int {
	return 1 + t.elementType.typeDepth()
}

type coreTableTypeResolver struct {
	elementTypeResolver typeResolver
	min                 uint32
	max                 *uint32
}

func newCoreTableTypeResolver(elementTypeResolver typeResolver, min uint32, max *uint32) *coreTableTypeResolver {
	return &coreTableTypeResolver{
		elementTypeResolver: elementTypeResolver,
		min:                 min,
		max:                 max,
	}
}

func (d *coreTableTypeResolver) resolveType(scope *scope) (Type, error) {
	elementType, err := d.elementTypeResolver.resolveType(scope)
	if err != nil {
		return nil, err
	}
	return newCoreTableType(elementType, d.min, d.max), nil
}

func (d *coreTableTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := d.elementTypeResolver.typeInfo(scope)
	return &typeInfo{
		typeName: "core table",
		depth:    1 + ti.depth,
		size:     1 + ti.size,
	}
}
