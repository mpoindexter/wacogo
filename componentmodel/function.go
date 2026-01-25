package componentmodel

import (
	"context"
	"fmt"
)

// Function represents a component function
type Function struct {
	funcTyp *FunctionType
	invoke  func(ctx context.Context, params []Value) (Value, error)
}

func NewFunction(
	typ *FunctionType,
	invoke func(ctx context.Context, params []Value) (Value, error),
) *Function {
	return &Function{
		funcTyp: typ,
		invoke:  invoke,
	}
}

func (f *Function) typ() *FunctionType {
	return f.funcTyp
}

func (f *Function) Invoke(ctx context.Context, params ...Value) (Value, error) {
	return f.invoke(ctx, params)
}

type FunctionParameter struct {
	Name string
	Type ValueType
}

type FunctionType struct {
	Parameters []*FunctionParameter
	ResultType ValueType
}

func (ft *FunctionType) typ() Type {
	return ft
}

func (ft *FunctionType) assignableFrom(other Type) bool {
	otherFt, ok := other.(*FunctionType)
	if !ok {
		return false
	}
	if len(ft.Parameters) != len(otherFt.Parameters) {
		return false
	}
	for i, param := range ft.Parameters {
		if !param.Type.assignableFrom(otherFt.Parameters[i].Type) {
			return false
		}
	}
	if ft.ResultType == nil && otherFt.ResultType == nil {
		return true
	}
	if (ft.ResultType == nil) != (otherFt.ResultType == nil) {
		return false
	}
	return ft.ResultType.assignableFrom(otherFt.ResultType)
}

type parameterTypeResolver struct {
	name         string
	typeResolver typeResolver
}

func newParameterTypeResolver(name string, typeResolver typeResolver) *parameterTypeResolver {
	return &parameterTypeResolver{
		name:         name,
		typeResolver: typeResolver,
	}
}

type functionTypeResolver struct {
	paramTypeResolvers []*parameterTypeResolver
	resultTypeResolver typeResolver
	staticType         *FunctionType
}

func newFunctionTypeResolver(
	paramTypeResolvers []*parameterTypeResolver,
	resultTypeResolver typeResolver,
) (*functionTypeResolver, error) {
	paramTypes := make([]*FunctionParameter, len(paramTypeResolvers))
	for i, paramTypeResolver := range paramTypeResolvers {
		paramType := paramTypeResolver.typeResolver.typ()
		paramValueType, ok := paramType.(ValueType)
		if !ok {
			return nil, fmt.Errorf("function param type is not a value type: %T", paramType)
		}
		paramTypes[i] = &FunctionParameter{
			Name: paramTypeResolver.name,
			Type: paramValueType,
		}
	}

	var funcType *FunctionType
	if resultTypeResolver == nil {
		funcType = &FunctionType{
			Parameters: paramTypes,
			ResultType: nil,
		}
	} else {
		resultType := resultTypeResolver.typ()
		resultTypes, ok := resultType.(ValueType)
		if !ok {
			return nil, fmt.Errorf("function result type is not a value type: %T", resultType)
		}
		funcType = &FunctionType{
			Parameters: paramTypes,
			ResultType: resultTypes,
		}
	}
	return &functionTypeResolver{
		paramTypeResolvers: paramTypeResolvers,
		resultTypeResolver: resultTypeResolver,
		staticType:         funcType,
	}, nil
}

func (d *functionTypeResolver) typ() Type {
	return d.staticType
}

func (d *functionTypeResolver) resolveType(ctx context.Context, scope *instanceScope) (Type, error) {
	paramDefns := make([]*FunctionParameter, len(d.paramTypeResolvers))
	for i, paramTypeResolver := range d.paramTypeResolvers {
		paramType, err := paramTypeResolver.typeResolver.resolveType(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve function param type: %w", err)
		}
		paramValueType, ok := paramType.(ValueType)
		if !ok {
			return nil, fmt.Errorf("function param type is not a value type: %T", paramType)
		}
		paramDefns[i] = &FunctionParameter{
			Name: paramTypeResolver.name,
			Type: paramValueType,
		}
	}
	if d.resultTypeResolver == nil {
		funcType := &FunctionType{
			Parameters: paramDefns,
			ResultType: nil,
		}
		return funcType, nil
	}
	resultType, err := d.resultTypeResolver.resolveType(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve function result type: %w", err)
	}
	resultTypes, ok := resultType.(ValueType)
	if !ok {
		return nil, fmt.Errorf("function result type is not a value type: %T", resultType)
	}
	funcType := &FunctionType{
		Parameters: paramDefns,
		ResultType: resultTypes,
	}
	return funcType, nil
}
