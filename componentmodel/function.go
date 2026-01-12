package componentmodel

import (
	"context"
	"fmt"
)

type functionDefinition interface {
	resolveFunction(ctx context.Context, scope instanceScope) (*Function, error)
}

type functionAliasDefinition struct {
	instanceIdx uint32
	funcName    string
}

func (d *functionAliasDefinition) resolveFunction(ctx context.Context, scope instanceScope) (*Function, error) {
	inst, err := scope.resolveInstance(ctx, d.instanceIdx)
	if err != nil {
		return nil, err
	}
	fnAny, ok := inst.exports[d.funcName]
	if !ok {
		return nil, fmt.Errorf("function %s not found in instance %d", d.funcName, d.instanceIdx)
	}
	fn, ok := fnAny.(*Function)
	if !ok {
		return nil, fmt.Errorf("export %s in instance %d is not a function", d.funcName, d.instanceIdx)
	}
	return fn, nil
}

type functionImportDefinition struct {
	name string
}

func (d *functionImportDefinition) resolveFunction(ctx context.Context, scope instanceScope) (*Function, error) {
	val, err := scope.resolveArgument(d.name)
	if err != nil {
		return nil, err
	}
	fn, ok := val.(*Function)
	if !ok {
		return nil, fmt.Errorf("import %s is not a function", d.name)
	}
	return fn, nil
}

// Function represents a component function
type Function struct {
	typ      *FunctionType
	instance *Instance
	invoke   func(ctx context.Context, params []Value) Value
}

func NewFunction(
	instance *Instance,
	typ *FunctionType,
	invoke func(ctx context.Context, params []Value) Value,
) *Function {
	return &Function{
		typ:      typ,
		instance: instance,
		invoke:   invoke,
	}
}

func (f *Function) Invoke(ctx context.Context, params ...Value) Value {
	return f.invoke(ctx, params)
}

type FunctionType struct {
	ParamTypes []ValueType
	ResultType ValueType
}

func (ft *FunctionType) equalsType(other Type) bool {
	otherFt, ok := other.(*FunctionType)
	if !ok {
		return false
	}
	if len(ft.ParamTypes) != len(otherFt.ParamTypes) {
		return false
	}
	for i, pt := range ft.ParamTypes {
		if !pt.equalsType(otherFt.ParamTypes[i]) {
			return false
		}
	}
	if ft.ResultType == nil && otherFt.ResultType == nil {
		return true
	}
	if (ft.ResultType == nil) != (otherFt.ResultType == nil) {
		return false
	}
	return ft.ResultType.equalsType(otherFt.ResultType)
}
