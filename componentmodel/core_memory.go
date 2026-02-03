package componentmodel

import (
	"fmt"

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

func (c *coreMemoryType) isType() {}

func (t *coreMemoryType) typeName() string {
	return "core memory"
}

func (t *coreMemoryType) checkType(other Type, typeChecker typeChecker) error {
	ot, err := assertTypeKindIsSame(t, other)
	if err != nil {
		return err
	}
	if t.min > ot.min {
		return fmt.Errorf("type mismatch: mismatch in memory limits: memory minimum size %d is greater than %d", t.min, ot.min)
	}
	// TODO: should we compare max values?
	return nil
}

func (t *coreMemoryType) typeSize() int {
	return 1
}

func (t *coreMemoryType) typeDepth() int {
	return 1
}
