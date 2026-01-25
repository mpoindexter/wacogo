package componentmodel

import (
	"context"
	"fmt"

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

func (f *coreFunction) typ() *coreFunctionType {
	paramTypes := make([]Type, len(f.def.ParamTypes()))
	for i, pt := range f.def.ParamTypes() {
		paramTypes[i] = coreTypeWasmConstTypeFromWazero(pt)
	}
	resultTypes := make([]Type, len(f.def.ResultTypes()))
	for i, rt := range f.def.ResultTypes() {
		resultTypes[i] = coreTypeWasmConstTypeFromWazero(rt)
	}
	return newCoreFunctionType(paramTypes, resultTypes)
}

type coreTypeFunctionDefinition struct {
	paramTypeDefs  []definition[Type, Type]
	resultTypeDefs []definition[Type, Type]
}

func newCoreTypeFunctionDefinition(paramTypeDefs []definition[Type, Type], resultTypeDefs []definition[Type, Type]) *coreTypeFunctionDefinition {
	return &coreTypeFunctionDefinition{
		paramTypeDefs:  paramTypeDefs,
		resultTypeDefs: resultTypeDefs,
	}
}

func (d *coreTypeFunctionDefinition) typ() Type {
	paramTypes := make([]Type, len(d.paramTypeDefs))
	for i, paramTypeDef := range d.paramTypeDefs {
		paramTypes[i] = paramTypeDef.typ()
	}
	resultTypes := make([]Type, len(d.resultTypeDefs))
	for i, resultTypeDef := range d.resultTypeDefs {
		resultTypes[i] = resultTypeDef.typ()
	}
	return newCoreFunctionType(paramTypes, resultTypes)
}

func (d *coreTypeFunctionDefinition) resolve(ctx context.Context, scope *instanceScope) (Type, error) {
	paramTypes := make([]Type, len(d.paramTypeDefs))
	for i, paramTypeDef := range d.paramTypeDefs {
		ct, err := paramTypeDef.resolve(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter type %d: %w", i, err)
		}
		paramTypes[i] = ct
	}
	resultTypes := make([]Type, len(d.resultTypeDefs))
	for i, resultTypeDef := range d.resultTypeDefs {
		ct, err := resultTypeDef.resolve(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve result type %d: %w", i, err)
		}
		resultTypes[i] = ct
	}
	return newCoreFunctionType(paramTypes, resultTypes), nil
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

func (c *coreFunctionType) typ() Type {
	return c
}

func (c *coreFunctionType) assignableFrom(other Type) bool {
	otherFuncType, ok := other.(*coreFunctionType)
	if !ok {
		return false
	}
	if len(c.paramTypes) != len(otherFuncType.paramTypes) {
		return false
	}
	for i, t := range c.paramTypes {
		if t != otherFuncType.paramTypes[i] {
			return false
		}
	}
	if len(c.resultTypes) != len(otherFuncType.resultTypes) {
		return false
	}
	for i, t := range c.resultTypes {
		if t != otherFuncType.resultTypes[i] {
			return false
		}
	}
	return true
}
