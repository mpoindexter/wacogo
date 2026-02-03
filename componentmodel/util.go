package componentmodel

import (
	"context"
	"errors"
)

func zero[T any]() T {
	var zero T
	return zero
}

func typeNameOf[T Type]() string {
	return zero[T]().typeName()
}

type staticDefinition[V any, T Type] struct {
	v V
	t T
}

func newStaticDefinition[V any, T Type](v V, t T) *staticDefinition[V, T] {
	return &staticDefinition[V, T]{v: v, t: t}
}

func (d *staticDefinition[V, T]) isDefinition() {}
func (d *staticDefinition[V, T]) createType(scope *scope) (T, error) {
	return d.t, nil
}

func (d *staticDefinition[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	return d.v, nil
}

type referenceDefinition[V any, T Type] struct {
	sort          sort[V, T]
	referencedIdx uint32
}

func newReferenceDefinition[V any, T Type](sort sort[V, T], referencedIdx uint32) *referenceDefinition[V, T] {
	return &referenceDefinition[V, T]{
		sort:          sort,
		referencedIdx: referencedIdx,
	}
}

func (d *referenceDefinition[V, T]) isDefinition() {}

func (d *referenceDefinition[V, T]) createType(scope *scope) (T, error) {
	return sortScopeFor(scope, d.sort).getType(d.referencedIdx)
}

func (d *referenceDefinition[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	return sortScopeFor(scope, d.sort).getInstance(d.referencedIdx)
}

type referenceDefinitionWithType[V any, T Type] struct {
	sort          sort[V, T]
	referencedIdx uint32
	typ           T
}

func newReferenceDefinitionWithType[V any, T Type](sort sort[V, T], referencedIdx uint32, typ T) *referenceDefinitionWithType[V, T] {
	return &referenceDefinitionWithType[V, T]{
		sort:          sort,
		referencedIdx: referencedIdx,
		typ:           typ,
	}
}

func (d *referenceDefinitionWithType[V, T]) isDefinition() {}

func (d *referenceDefinitionWithType[V, T]) createType(scope *scope) (T, error) {
	return d.typ, nil
}

func (d *referenceDefinitionWithType[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	return sortScopeFor(scope, d.sort).getInstance(d.referencedIdx)
}

var errSkipChildren = errors.New("skip children")

func walkImportTypes(imports map[string]Type, fn func(Type) error) error {
	for _, importType := range imports {
		if err := fn(importType); err != nil {
			if errors.Is(err, errSkipChildren) {
				continue
			}
			return err
		}
		switch it := importType.(type) {
		case *instanceType:
			if err := walkExportTypes(it.exports, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func walkExportTypes(exports map[string]*exportSpec, fn func(Type) error) error {
	for _, exportSpec := range exports {
		if err := fn(exportSpec.typ); err != nil {
			if errors.Is(err, errSkipChildren) {
				continue
			}
			return err
		}
		switch et := exportSpec.typ.(type) {
		case *instanceType:
			if err := walkExportTypes(et.exports, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func walkTypes(t Type, fn func(Type) error) error {
	if err := fn(t); err != nil {
		if errors.Is(err, errSkipChildren) {
			return nil
		}
		return err
	}

	switch tt := t.(type) {
	case *coreFunctionType:
		for _, param := range tt.paramTypes {
			if err := walkTypes(param, fn); err != nil {
				return err
			}
		}
		for _, result := range tt.resultTypes {
			if err := walkTypes(result, fn); err != nil {
				return err
			}
		}
	case *coreGlobalType:
		if err := walkTypes(tt.valueType, fn); err != nil {
			return err
		}
	case *coreInstanceType:
		for _, exportType := range tt.exports {
			if err := walkTypes(exportType, fn); err != nil {
				return err
			}
		}
	case *coreModuleType:
		for _, exportType := range tt.exports {
			if err := walkTypes(exportType, fn); err != nil {
				return err
			}
		}
		for _, importType := range tt.imports {
			if err := walkTypes(importType, fn); err != nil {
				return err
			}
		}

	case *coreTableType:
		if err := walkTypes(tt.elementType, fn); err != nil {
			return err
		}
	case *componentType:
		for _, importType := range tt.imports {
			if err := walkTypes(importType, fn); err != nil {
				return err
			}
		}
		for _, exportType := range tt.exports {
			if err := walkTypes(exportType, fn); err != nil {
				return err
			}
		}
	case *instanceType:
		for _, exportSpec := range tt.exports {
			if err := walkTypes(exportSpec.typ, fn); err != nil {
				return err
			}
		}
	case *FunctionType:
		for _, param := range tt.Parameters {
			if err := walkTypes(param.Type, fn); err != nil {
				return err
			}
		}
		if tt.ResultType != nil {
			if err := walkTypes(tt.ResultType, fn); err != nil {
				return err
			}
		}
	case *RecordType:
		for _, field := range tt.Fields {
			if err := walkTypes(field.Type, fn); err != nil {
				return err
			}
		}
	case *VariantType:
		for _, caseType := range tt.Cases {
			if caseType == nil {
				continue
			}
			if err := walkTypes(caseType.Type, fn); err != nil {
				return err
			}
		}
	case *ListType:
		if err := walkTypes(tt.ElementType, fn); err != nil {
			return err
		}
	case OwnType:
		if err := walkTypes(tt.ResourceType, fn); err != nil {
			return err
		}
	case BorrowType:
		if err := walkTypes(tt.ResourceType, fn); err != nil {
			return err
		}
	case interface {
		unwrap() Type
	}:
		if err := walkTypes(tt.unwrap(), fn); err != nil {
			return err
		}
	}

	return nil
}
