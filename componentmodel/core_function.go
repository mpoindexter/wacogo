package componentmodel

import (
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero/api"
)

type coreFunction struct {
	module api.Module
	name   string
	def    api.FunctionDefinition
}

func newCoreFunction(module api.Module, name string, def api.FunctionDefinition) *coreFunction {
	return &coreFunction{
		module: module,
		name:   name,
		def:    def,
	}
}

type coreTypeFunctionTypeResolver struct {
	paramTypes  []typeResolver
	resultTypes []typeResolver
}

func newCoreTypeFunctionTypeResolver(paramTypes []typeResolver, resultTypes []typeResolver) *coreTypeFunctionTypeResolver {
	return &coreTypeFunctionTypeResolver{
		paramTypes:  paramTypes,
		resultTypes: resultTypes,
	}
}

func (d *coreTypeFunctionTypeResolver) isDefinition() {}

func (d *coreTypeFunctionTypeResolver) resolveType(scope *scope) (Type, error) {
	paramTypes := make([]Type, len(d.paramTypes))
	for i, paramType := range d.paramTypes {
		pt, err := paramType.resolveType(scope)
		if err != nil {
			return nil, err
		}
		paramTypes[i] = pt
	}
	resultTypes := make([]Type, len(d.resultTypes))
	for i, resultType := range d.resultTypes {
		rt, err := resultType.resolveType(scope)
		if err != nil {
			return nil, err
		}
		resultTypes[i] = rt
	}
	return newCoreFunctionType(paramTypes, resultTypes), nil
}

func (d *coreTypeFunctionTypeResolver) typeInfo(scope *scope) *typeInfo {
	size := 1
	for _, pt := range d.paramTypes {
		size += pt.typeInfo(scope).size
	}
	for _, rt := range d.resultTypes {
		size += rt.typeInfo(scope).size
	}
	return &typeInfo{
		typeName: "core function",
		size:     size,
	}
}

type coreFunctionType struct {
	paramTypes  []Type
	resultTypes []Type
}

func newCoreFunctionType(paramTypes []Type, resultTypes []Type) *coreFunctionType {
	return &coreFunctionType{
		paramTypes:  paramTypes,
		resultTypes: resultTypes,
	}
}

func (c *coreFunctionType) isType() {}

func (c *coreFunctionType) typeName() string {
	return "core function"
}

func (c *coreFunctionType) checkType(other Type, typeChecker typeChecker) error {
	oc, err := assertTypeKindIsSame(c, other)
	if err != nil {
		return err
	}
	if len(c.paramTypes) != len(oc.paramTypes) {
		return fmt.Errorf("type mismatch: expected: %s, found: %s", c.String(), oc.String())
	}
	for i, t := range c.paramTypes {
		if t != oc.paramTypes[i] {
			return fmt.Errorf("type mismatch: expected: %s, found: %s", c.String(), oc.String())
		}
	}
	if len(c.resultTypes) != len(oc.resultTypes) {
		return fmt.Errorf("type mismatch: expected: %s, found: %s", c.String(), oc.String())
	}
	for i, t := range c.resultTypes {
		if t != oc.resultTypes[i] {
			return fmt.Errorf("type mismatch: expected: %s, found: %s", c.String(), oc.String())
		}
	}
	return nil
}

func (c *coreFunctionType) typeSize() int {
	size := 1 // for the function itself
	for _, pt := range c.paramTypes {
		size += pt.typeSize()
	}
	for _, rt := range c.resultTypes {
		size += rt.typeSize()
	}
	return size
}

func (c *coreFunctionType) typeDepth() int {
	maxDepth := 0
	for _, pt := range c.paramTypes {
		if d := pt.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	for _, rt := range c.resultTypes {
		if d := rt.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	return 1 + maxDepth
}

func (c *coreFunctionType) String() string {
	atoms := make([]string, 0, 3)
	atoms = append(atoms, "func")
	if len(c.paramTypes) > 0 {
		paramTypeStrs := make([]string, len(c.paramTypes))
		for i, pt := range c.paramTypes {
			paramTypeStrs[i] = pt.typeName()
		}
		atoms = append(atoms, fmt.Sprintf("(param %s)", strings.Join(paramTypeStrs, " ")))
	}
	if len(c.resultTypes) > 0 {
		resultTypeStrs := make([]string, len(c.resultTypes))
		for i, rt := range c.resultTypes {
			resultTypeStrs[i] = rt.typeName()
		}
		atoms = append(atoms, fmt.Sprintf("(result %s)", strings.Join(resultTypeStrs, " ")))
	}
	return fmt.Sprintf("(%s)", strings.Join(atoms, " "))
}
