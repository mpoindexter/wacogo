package host

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/partite-ai/wacogo/model"
)

type ValueTyped interface {
	ValueType(inst *Instance) model.ValueType
}

func ValueTypeFor[T any](inst *Instance) model.ValueType {
	if vt, ok := valueTypeFor(inst, reflect.TypeFor[T]()); ok {
		return vt
	}
	panic(fmt.Sprintf("ValueTypeFor: unsupported type %T", *new(T)))
}

func valueTypeFor(inst *Instance, t reflect.Type) (model.ValueType, bool) {
	switch t.Kind() {
	case reflect.Bool:
		return &model.BoolType{}, true
	case reflect.Uint8:
		return &model.U8Type{}, true
	case reflect.Uint16:
		return &model.U16Type{}, true
	case reflect.Uint32:
		return &model.U32Type{}, true
	case reflect.Uint64:
		return &model.U64Type{}, true
	case reflect.Int8:
		return &model.S8Type{}, true
	case reflect.Int16:
		return &model.S16Type{}, true
	case reflect.Int32:
		return &model.S32Type{}, true
	case reflect.Int64:
		return &model.S64Type{}, true
	case reflect.Float32:
		return &model.F32Type{}, true
	case reflect.Float64:
		return &model.F64Type{}, true
	case reflect.String:
		return &model.StringType{}, true
	}

	if t.AssignableTo(reflect.TypeFor[model.Char]()) {
		return &model.CharType{}, true
	}

	if t.AssignableTo(reflect.TypeFor[model.ByteArray]()) {
		return &model.ByteArrayType{}, true
	}

	// Resource Handle
	type handleType interface {
		ResourceType() reflect.Type
		HandleValueType(t *model.ResourceType) model.ValueType
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
		return model.EnumType(enumValues...), true
	}

	// Flags type
	if t.ConvertibleTo(reflect.TypeFor[map[string]bool]()) && t.Implements(reflect.TypeFor[FlagsValueProvider]()) {
		flagsValues := reflect.Zero(t).Interface().(FlagsValueProvider).FlagsValues()
		return &model.FlagsType{FlagNames: flagsValues}, true
	}

	// Slice type
	if t.Kind() == reflect.Slice {
		elemType, ok := valueTypeFor(inst, t.Elem())
		if !ok {
			return nil, false
		}
		return &model.ListType{ElementType: elemType}, true
	}

	return nil, false
}

func ResourceTypeFor[T any](inst *Instance, owner *Instance) *model.ResourceType {
	if rt, ok := resourceTypeFor[T](inst, owner); ok {
		return rt
	}

	panic(fmt.Sprintf("ResourceTypeFor: unsupported type %T", *new(T)))
}

func resourceTypeFor[T any](inst, owner *Instance) (*model.ResourceType, bool) {
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
	rt = model.NewResourceType(owner.instance, t, destructor)
	inst.resourceTypes[t] = rt
	return rt, true
}
