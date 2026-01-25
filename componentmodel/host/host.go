package host

import (
	"context"
	"fmt"
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type Instance struct {
	instanceBuilder *componentmodel.InstanceBuilder
	resourceTypes   map[reflect.Type]*componentmodel.ResourceType
}

func NewInstance() *Instance {
	b := componentmodel.NewInstanceBuilder()
	return &Instance{
		instanceBuilder: b,
		resourceTypes:   make(map[reflect.Type]*componentmodel.ResourceType),
	}
}

func (hi *Instance) Instance() *componentmodel.Instance {
	return hi.instanceBuilder.Build()
}

func (hi *Instance) AddTypeExport(name string, typ componentmodel.Type) {
	hi.instanceBuilder.AddTypeExport(name, typ)
}

func (hi *Instance) AddFunction(name string, fn any) error {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("expected a function, got %s", fnType.Kind())
	}

	var paramConverters []converter
	var paramTypes []*componentmodel.FunctionParameter
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
		paramTypes = append(paramTypes, &componentmodel.FunctionParameter{
			Name: fmt.Sprintf("param%d", i),
			Type: vt,
		})
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

	hi.instanceBuilder.AddFunctionExport(name, func(instance *componentmodel.Instance) *componentmodel.Function {
		return componentmodel.NewFunction(
			&componentmodel.FunctionType{
				Parameters: paramTypes,
				ResultType: resultType,
			},
			func(ctx context.Context, params []componentmodel.Value) (componentmodel.Value, error) {
				if len(params) != len(paramConverters) {
					return nil, fmt.Errorf("expected %d parameters, got %d", len(paramConverters), len(params))
				}
				cc := &callContext{
					instance:     instance,
					hostInstance: hi,
				}
				var hostParams []reflect.Value
				for i, param := range params {
					hostParam := paramConverters[i].toHost(cc, param)
					hostParams = append(hostParams, reflect.ValueOf(hostParam))
				}
				results := reflect.ValueOf(fn).Call(hostParams)
				if len(results) == 0 {
					return nil, nil
				}
				hostResult := results[0].Interface()
				componentResult := resultConverter.fromHost(cc, hostResult)
				return componentResult, nil
			},
		)
	})
	return nil
}

func (hi *Instance) MustAddFunction(name string, fn any) {
	err := hi.AddFunction(name, fn)
	if err != nil {
		panic(err)
	}
}
