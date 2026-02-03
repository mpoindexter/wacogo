package componentmodel

import (
	"context"
	"fmt"
)

type importDefinition[V any, T Type] struct {
	sort               sort[V, T]
	importName         string
	importTypeResolver typeResolver
}

func newImportDefinition[V any, T Type](sort sort[V, T], importName string, resolver typeResolver) *importDefinition[V, T] {
	return &importDefinition[V, T]{
		sort:               sort,
		importName:         importName,
		importTypeResolver: resolver,
	}
}

func (d *importDefinition[V, T]) isDefinition() {}

func (d *importDefinition[V, T]) createType(scope *scope) (T, error) {
	expectedType, err := d.importTypeResolver.resolveType(scope)
	if err != nil {
		return zero[T](), err
	}

	argType, err := scope.resolveArgumentType(d.importName)
	if err != nil {
		// If the expected type is exactly the same as the import type, we the import can be missing.
		if int(d.sort) == int(sortType) && isStaticallyKnownType(expectedType) {
			return expectedType.(T), nil
		}

		// If the expected type is an empty instance type, even transitively, we allow the import to be missing
		// and just return an empty instance type. This is ridiculous, but the spec tests call for it.
		if int(d.sort) == int(sortInstance) && isStaticallyKnownInstanceType(expectedType) {
			return expectedType.(T), nil
		}
		return zero[T](), err
	}

	_, placeholder := argType.(importPlaceholderType)

	if placeholder {
		argType = expectedType
	}

	// Every instance of an imported instance type should have unique resource placeholder identities.
	if it, ok := argType.(*instanceType); ok {
		argType = it.clone()
	}

	typeChecker := newTypeChecker()
	if err := typeChecker.checkTypeCompatible(expectedType, argType); err != nil {
		return zero[T](), fmt.Errorf("import %s is not of expected type: %w", d.importName, err)
	}

	return argType.(T), nil
}

func (d *importDefinition[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	importedValue, err := scope.resolveArgumentValue(ctx, d.importName)
	if err != nil {
		if int(d.sort) == int(sortType) && isStaticallyKnownType(scope.currentType) {
			return scope.currentType.(V), nil
		}
		if int(d.sort) == int(sortInstance) && isStaticallyKnownInstanceType(scope.currentType) {
			return synthesizeRidiculousEmptyInstance(scope.currentType.(*instanceType)).(V), nil
		}
		return zero[V](), err
	}
	typedVal, ok := importedValue.(V)
	if !ok {
		return zero[V](), fmt.Errorf("export %s in import instance is not of expected type", d.importName)
	}
	return typedVal, nil
}

func isStaticallyKnownType(t Type) bool {
	containsResource := false
	walkTypes(t, func(t Type) error {
		if containsResource {
			return errSkipChildren
		}
		if _, ok := t.(*ResourceType); ok {
			containsResource = true
		}
		return nil
	})
	return !containsResource
}

func isStaticallyKnownInstanceType(t Type) bool {
	// NOTE: If the expected type is an empty instance type, even transitively, we allow the import to be missing
	// and just return an empty instance type. This is ridiculous, but the spec tests call for it.

	switch it := t.(type) {
	case *instanceType:
		if len(it.exports) == 0 {
			return true
		}
		for _, exportSpec := range it.exports {
			switch exportSpec.sort {
			case sortType:
				if !isStaticallyKnownType(exportSpec.typ) {
					return false
				}
			case sortInstance:
				if !isStaticallyKnownInstanceType(exportSpec.typ) {
					return false
				}
			default:
				return false
			}
		}
		return true
	default:
		return false
	}
}

func synthesizeRidiculousEmptyInstance(typ *instanceType) any {
	instance := newInstance()
	for name, exportSpec := range typ.exports {
		switch exportSpec.sort {
		case sortType:
			exportInst := exportSpec.typ
			instance.exports[name] = exportInst
			continue
		case sortInstance:
			instance.exports[name] = synthesizeRidiculousEmptyInstance(exportSpec.typ.(*instanceType))
		default:
			continue
		}
	}
	return instance
}
