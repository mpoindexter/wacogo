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

func (f *Function) Invoke(ctx context.Context, params ...Value) (Value, error) {
	return f.invoke(ctx, params)
}

type FunctionParameter struct {
	Name string
	Type ValueType
}

type FunctionType struct {
	Parameters         []*FunctionParameter
	ResultType         ValueType
	skipParamNameCheck bool
}

func (ft *FunctionType) isType() {}

func (ft *FunctionType) typeName() string {
	return "func"
}

func (ft *FunctionType) checkType(other Type, typeChecker typeChecker) error {
	oft, err := assertTypeKindIsSame(ft, other)
	if err != nil {
		return err
	}
	if len(ft.Parameters) != len(oft.Parameters) {
		return fmt.Errorf("type mismatch: expected %d parameters, found %d", len(ft.Parameters), len(oft.Parameters))
	}
	skipNameCheck := ft.skipParamNameCheck || oft.skipParamNameCheck
	for i, param := range ft.Parameters {
		if !skipNameCheck && param.Name != oft.Parameters[i].Name {
			return fmt.Errorf("type mismatch for parameter %d: expected parameter named `%s`, found `%s`", i, param.Name, oft.Parameters[i].Name)
		}
		if err := typeChecker.checkTypeCompatible(param.Type, oft.Parameters[i].Type); err != nil {
			return fmt.Errorf("type mismatch in function parameter `%s`: %w", param.Name, err)
		}
	}
	if ft.ResultType == nil && oft.ResultType == nil {
		return nil
	}
	if ft.ResultType == nil && oft.ResultType != nil {
		return fmt.Errorf("expected no result, found a result")
	} else if oft.ResultType == nil && ft.ResultType != nil {
		return fmt.Errorf("expected a result, found no result")
	}

	if err := typeChecker.checkTypeCompatible(ft.ResultType, oft.ResultType); err != nil {
		return fmt.Errorf("type mismatch with result type: %w", err)
	}
	return nil
}

func (ft *FunctionType) typeSize() int {
	size := 1 // for the function itself
	for _, param := range ft.Parameters {
		size += param.Type.typeSize()
	}
	if ft.ResultType != nil {
		size += ft.ResultType.typeSize()
	}
	return size
}

func (ft *FunctionType) typeDepth() int {
	maxDepth := 0
	for _, param := range ft.Parameters {
		if d := param.Type.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	if ft.ResultType != nil {
		if d := ft.ResultType.typeDepth(); d > maxDepth {
			maxDepth = d
		}
	}
	return 1 + maxDepth
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
}

func newFunctionTypeResolver(
	paramTypeResolvers []*parameterTypeResolver,
	resultTypeResolver typeResolver,
) *functionTypeResolver {

	return &functionTypeResolver{
		paramTypeResolvers: paramTypeResolvers,
		resultTypeResolver: resultTypeResolver,
	}
}

func (d *functionTypeResolver) resolveType(scope *scope) (Type, error) {
	paramDefns := make([]*FunctionParameter, len(d.paramTypeResolvers))
	for i, paramTypeResolver := range d.paramTypeResolvers {
		paramType, err := paramTypeResolver.typeResolver.resolveType(scope)
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
	resultType, err := d.resultTypeResolver.resolveType(scope)
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

func (d *functionTypeResolver) typeInfo(sc *scope) *typeInfo {
	size := 1
	for _, paramTypeResolver := range d.paramTypeResolvers {
		size += paramTypeResolver.typeResolver.typeInfo(sc).size
	}
	if d.resultTypeResolver != nil {
		size += d.resultTypeResolver.typeInfo(sc).size
	}
	return &typeInfo{
		typeName: "function",
		size:     size,
	}
}
