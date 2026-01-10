package host

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/partite-ai/wacogo/componentmodel"
)

type ValueTyped interface {
	ValueType(inst *Instance) componentmodel.ValueType
}

func ValueTypeFor[T any](inst *Instance) componentmodel.ValueType {
	if vt, ok := valueTypeFor(inst, reflect.TypeFor[T]()); ok {
		return vt
	}
	panic(fmt.Sprintf("ValueTypeFor: unsupported type %T", *new(T)))
}

func valueTypeFor(inst *Instance, t reflect.Type) (componentmodel.ValueType, bool) {
	switch t.Kind() {
	case reflect.Bool:
		return &componentmodel.BoolType{}, true
	case reflect.Uint8:
		return &componentmodel.U8Type{}, true
	case reflect.Uint16:
		return &componentmodel.U16Type{}, true
	case reflect.Uint32:
		return &componentmodel.U32Type{}, true
	case reflect.Uint64:
		return &componentmodel.U64Type{}, true
	case reflect.Int8:
		return &componentmodel.S8Type{}, true
	case reflect.Int16:
		return &componentmodel.S16Type{}, true
	case reflect.Int32:
		return &componentmodel.S32Type{}, true
	case reflect.Int64:
		return &componentmodel.S64Type{}, true
	case reflect.Float32:
		return &componentmodel.F32Type{}, true
	case reflect.Float64:
		return &componentmodel.F64Type{}, true
	case reflect.String:
		return &componentmodel.StringType{}, true
	}

	if t.AssignableTo(reflect.TypeFor[componentmodel.Char]()) {
		return &componentmodel.CharType{}, true
	}

	if t.AssignableTo(reflect.TypeFor[componentmodel.ByteArray]()) {
		return &componentmodel.ByteArrayType{}, true
	}

	// Resource Handle
	type handleType interface {
		ResourceType() reflect.Type
		HandleValueType(t *componentmodel.ResourceType) componentmodel.ValueType
	}

	if t.Implements(reflect.TypeFor[handleType]()) {
		ht := reflect.Zero(t).Interface().(handleType)
		if rt, ok := inst.resourceTypes[ht.ResourceType()]; ok {
			return ht.HandleValueType(rt), true
		}

		panic(fmt.Sprintf("valueTypeFor: unbound resource type %s", ht.ResourceType()))
	}

	// Generic self provided type
	if t.Implements(reflect.TypeFor[ValueTyped]()) {
		hvt := reflect.Zero(t).Interface().(ValueTyped)
		vt := hvt.ValueType(inst)
		return vt, true
	}

	// Enum type
	if t.ConvertibleTo(reflect.TypeFor[string]()) && t.Implements(reflect.TypeFor[EnumValueProvider]()) {
		enumValues := reflect.Zero(t).Interface().(EnumValueProvider).EnumValues()
		return componentmodel.EnumType(enumValues...), true
	}

	// Flags type
	if t.ConvertibleTo(reflect.TypeFor[map[string]bool]()) && t.Implements(reflect.TypeFor[FlagsValueProvider]()) {
		flagsValues := reflect.Zero(t).Interface().(FlagsValueProvider).FlagsValues()
		return &componentmodel.FlagsType{FlagNames: flagsValues}, true
	}

	// Slice type
	if t.Kind() == reflect.Slice {
		elemType, ok := valueTypeFor(inst, t.Elem())
		if !ok {
			return nil, false
		}
		return &componentmodel.ListType{ElementType: elemType}, true
	}

	return nil, false
}

func ResourceTypeFor[T any](inst *Instance, owner *Instance) *componentmodel.ResourceType {
	if rt, ok := resourceTypeFor[T](inst, owner); ok {
		return rt
	}

	panic(fmt.Sprintf("ResourceTypeFor: unsupported type %T", *new(T)))
}

func resourceTypeFor[T any](inst, owner *Instance) (*componentmodel.ResourceType, bool) {
	t := reflect.TypeFor[T]()

	rt, ok := inst.resourceTypes[t]
	if ok {
		return rt, true
	}

	rt, ok = owner.resourceTypes[t]
	if ok {
		inst.resourceTypes[t] = rt
		return rt, true
	}

	if inst != owner {
		return nil, false
	}

	var destructor func(ctx context.Context, res any)
	var rsc T
	if _, ok := any(rsc).(io.Closer); ok {
		destructor = func(ctx context.Context, res any) {
			res.(io.Closer).Close()
		}
	}
	rt = componentmodel.NewResourceType(owner.instance, t, destructor)
	inst.resourceTypes[t] = rt
	return rt, true
}
