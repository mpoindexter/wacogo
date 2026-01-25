package componentmodel

import (
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

func (g *coreGlobal) typ() *coreGlobalType {
	vt := coreTypeWasmConstTypeFromWazero(g.global.Type())
	_, isMutable := g.global.(api.MutableGlobal)
	return newCoreGlobalType(vt, isMutable)
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

func (t *coreGlobalType) typ() Type {
	return t
}

func (t *coreGlobalType) assignableFrom(other Type) bool {
	otherGlobal, ok := other.(*coreGlobalType)
	if !ok {
		return false
	}
	if !t.valueType.assignableFrom(otherGlobal.valueType) {
		return false
	}
	if t.mutable != otherGlobal.mutable {
		return false
	}
	return true
}
