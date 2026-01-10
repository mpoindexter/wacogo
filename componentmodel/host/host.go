package host

import (
	"context"
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Instance struct {
	exports       map[string]any
	instance      *componentmodel.Instance
	resourceTypes map[reflect.Type]*componentmodel.ResourceType
}

func NewInstance() *Instance {
	exports := make(map[string]any)
	return &Instance{
		exports:       exports,
		instance:      componentmodel.NewInstanceOf(exports),
		resourceTypes: make(map[reflect.Type]*componentmodel.ResourceType),
	}
}

func (hi *Instance) Instance() *componentmodel.Instance {
	return hi.instance
}

func (hi *Instance) AddTypeExport(name string, typ componentmodel.Type) {
	hi.exports[name] = typ
}

func (hi *Instance) AddFunction(name string, fn any) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("expected a function, got %s", fnType.Kind())
	}

	var paramConverters []converter
	var paramTypes []componentmodel.ValueType
	var resultConverter converter
	var resultType componentmodel.ValueType

	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		vt, ok := valueTypeFor(hi, paramType)
		if !ok {
			return fmt.Errorf("unsupported parameter type %s", paramType.String())
		}
		converter := converterFor(paramType)
		if converter == nil {
			return fmt.Errorf("cannot convert parameter type %s", paramType.String())
		}
		paramConverters = append(paramConverters, converter)
		paramTypes = append(paramTypes, vt)
	}

	switch fnType.NumOut() {
	case 0:
		// No result
	case 1:
		outType := fnType.Out(0)
		vt, ok := valueTypeFor(hi, outType)
		if !ok {
			return fmt.Errorf("unsupported return type %s", outType.String())
		}
		converter := converterFor(outType)
		if converter == nil {
			converterFor(outType)
			return fmt.Errorf("cannot convert return type %s", outType.String())
		}
		resultConverter = converter
		resultType = vt
	default:
		return fmt.Errorf("functions with more than one return value are not supported")
	}

	modelFn := componentmodel.NewFunction(
		hi.instance,
		&componentmodel.FunctionType{
			ParamTypes: paramTypes,
			ResultType: resultType,
		},
		func(ctx context.Context, params []componentmodel.Value) componentmodel.Value {
			if len(params) != len(paramConverters) {
				panic(fmt.Errorf("expected %d parameters, got %d", len(paramConverters), len(params)))
			}
			var hostParams []reflect.Value
			for i, param := range params {
				hostParam := paramConverters[i].toHost(param)
				hostParams = append(hostParams, reflect.ValueOf(hostParam))
			}
			results := reflect.ValueOf(fn).Call(hostParams)
			if len(results) == 0 {
				return nil
			}
			hostResult := results[0].Interface()
			componentResult := resultConverter.fromHost(hostResult)
			return componentResult
		},
	)
	hi.exports[name] = modelFn
	return nil
}

func (hi *Instance) MustAddFunction(name string, fn any) {
	err := hi.AddFunction(name, fn)
	if err != nil {
		panic(err)
	}
}
