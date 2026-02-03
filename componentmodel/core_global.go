package componentmodel

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"
)

type coreGlobal struct {
	module api.Module
	name   string
	global api.Global
}

func newCoreGlobal(module api.Module, name string, global api.Global) *coreGlobal {
	return &coreGlobal{
		module: module,
		name:   name,
		global: global,
	}
}

type coreGlobalType struct {
	valueType Type
	mutable   bool
}

func newCoreGlobalType(valueType Type, mutable bool) *coreGlobalType {
	return &coreGlobalType{
		valueType: valueType,
		mutable:   mutable,
	}
}

func (c *coreGlobalType) isType() {}

func (t *coreGlobalType) typeName() string {
	return "core global"
}

func (t *coreGlobalType) checkType(other Type, typeChecker typeChecker) error {
	ot, err := assertTypeKindIsSame(t, other)
	if err != nil {
		return err
	}
	if err := typeChecker.checkTypeCompatible(t.valueType, ot.valueType); err != nil {
		return fmt.Errorf("expected global type %s, found %s", t.valueType.typeName(), ot.valueType.typeName())
	}
	if t.mutable != ot.mutable {
		return fmt.Errorf("type mismatch: global mutability mismatch, expected %v, found %v", t.mutable, ot.mutable)
	}
	return nil
}

func (t *coreGlobalType) typeSize() int {
	return 1 + t.valueType.typeSize()
}

func (t *coreGlobalType) typeDepth() int {
	return 1 + t.valueType.typeDepth()
}

type coreGlobalTypeResolver struct {
	valueTypeResolver typeResolver
	mutable           bool
}

func newCoreGlobalTypeResolver(valueTypeResolver typeResolver, mutable bool) *coreGlobalTypeResolver {
	return &coreGlobalTypeResolver{
		valueTypeResolver: valueTypeResolver,
		mutable:           mutable,
	}
}

func (r *coreGlobalTypeResolver) resolveType(scope *scope) (Type, error) {
	valueType, err := r.valueTypeResolver.resolveType(scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve global value type: %w", err)
	}
	return newCoreGlobalType(valueType, r.mutable), nil
}

func (r *coreGlobalTypeResolver) typeInfo(scope *scope) *typeInfo {
	ti := r.valueTypeResolver.typeInfo(scope)
	return &typeInfo{
		typeName: "core global",
		size:     1 + ti.size,
		depth:    1 + ti.depth,
	}
}
