package componentmodel

import (
	"github.com/tetratelabs/wazero/api"
)

type coreMemory struct {
	module api.Module
	name   string
	memory api.Memory
}

func newCoreMemory(module api.Module, name string, memory api.Memory) *coreMemory {
	return &coreMemory{
		module: module,
		name:   name,
		memory: memory,
	}
}

func (m *coreMemory) typ() *coreMemoryType {
	min := m.memory.Definition().Min()
	var max *uint32
	if defMax, ok := m.memory.Definition().Max(); ok {
		max = &defMax
	}
	return newCoreMemoryType(min, max)
}

type coreMemoryType struct {
	min uint32
	max *uint32
}

func newCoreMemoryType(min uint32, max *uint32) *coreMemoryType {
	return &coreMemoryType{
		min: min,
		max: max,
	}
}

func (t *coreMemoryType) typ() Type {
	return t
}

func (t *coreMemoryType) assignableFrom(other Type) bool {
	otherMem, ok := other.(*coreMemoryType)
	if !ok {
		return false
	}
	if t.min > otherMem.min {
		return false
	}
	// TODO: should we compare max values?
	return true
}
