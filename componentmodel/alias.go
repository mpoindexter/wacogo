package componentmodel

import (
	"context"
	"fmt"
)

type exportAliasDefinition[V any, T Type, I exporter, IT exportType] struct {
	instanceIdx  uint32
	exportName   string
	sort         sort[V, T]
	instanceSort sort[I, IT]
}

func newInstanceExportAliasDefinition[T comparable, TT Type](
	instanceIdx uint32,
	exportName string,
	sort sort[T, TT],
) *exportAliasDefinition[T, TT, *Instance, *instanceType] {
	return &exportAliasDefinition[T, TT, *Instance, *instanceType]{
		instanceIdx:  instanceIdx,
		exportName:   exportName,
		sort:         sort,
		instanceSort: sortInstance,
	}
}

func newCoreExportAliasDefinition[T comparable, TT Type](
	instanceIdx uint32,
	exportName string,
	sort sort[T, TT],
) *exportAliasDefinition[T, TT, *coreInstance, *coreInstanceType] {
	return &exportAliasDefinition[T, TT, *coreInstance, *coreInstanceType]{
		instanceIdx:  instanceIdx,
		exportName:   exportName,
		sort:         sort,
		instanceSort: sortCoreInstance,
	}
}

func (d *exportAliasDefinition[V, T, I, IT]) isDefinition() {}

func (d *exportAliasDefinition[V, T, I, IT]) createType(scope *scope) (T, error) {
	instanceType, err := sortScopeFor(scope, d.instanceSort).getType(d.instanceIdx)
	if err != nil {
		return zero[T](), err
	}
	exportType, ok := instanceType.exportType(d.exportName)
	if !ok {
		return zero[T](), fmt.Errorf("%s %d has no export named `%s`", d.instanceSort.typeName(), d.instanceIdx, d.exportName)
	}

	exportTypeCast, ok := exportType.(T)
	if !ok {
		return zero[T](), fmt.Errorf("export `%s` for %s %d is not a %s", d.exportName, d.instanceSort.typeName(), d.instanceIdx, d.sort.typeName())
	}

	return exportTypeCast, nil
}

func (d *exportAliasDefinition[V, T, I, IT]) createInstance(ctx context.Context, scope *scope) (V, error) {
	inst, err := sortScopeFor(scope, d.instanceSort).getInstance(d.instanceIdx)
	if err != nil {
		return zero[V](), err
	}
	exportVal, err := inst.getExport(d.exportName)
	if err != nil {
		return zero[V](), err
	}
	typedVal, ok := exportVal.(V)
	if !ok {
		return zero[V](), fmt.Errorf("export %s in %s %d is not a %s", d.exportName, d.instanceSort.typeName(), d.instanceIdx, d.sort.typeName())
	}
	return typedVal, nil
}

type outerAliasDefinition[V any, T Type] struct {
	outerIdx      uint32
	sort          sort[V, T]
	idx           uint32
	allowResource bool
}

func newOuterAliasDefinition[V any, T Type](
	outerIdx uint32,
	sort sort[V, T],
	idx uint32,
	allowResource bool,
) *outerAliasDefinition[V, T] {
	return &outerAliasDefinition[V, T]{
		outerIdx:      outerIdx,
		sort:          sort,
		idx:           idx,
		allowResource: allowResource,
	}
}

func (d *outerAliasDefinition[V, T]) isDefinition() {}

func (d *outerAliasDefinition[V, T]) createType(scope *scope) (T, error) {
	targetScope, err := scope.closure(d.outerIdx)
	if err != nil {
		return zero[T](), err
	}

	t, err := sortScopeFor(targetScope, d.sort).getType(d.idx)
	if err != nil {
		return zero[T](), err
	}

	if !d.allowResource && int(d.outerIdx) > 0 && int(d.sort) == int(sortType) {
		err := walkTypes(any(t).(Type), func(t Type) error {
			switch t.(type) {
			case *ResourceType:
				return fmt.Errorf("alias refers to resources not defined in the current component")
			default:
				return nil
			}
		})
		if err != nil {
			return zero[T](), err
		}
	}

	return t, nil
}

func (d *outerAliasDefinition[V, T]) createInstance(ctx context.Context, scope *scope) (V, error) {
	targetScope, err := scope.closure(d.outerIdx)
	if err != nil {
		return zero[V](), err
	}
	return sortScopeFor(targetScope, d.sort).getInstance(d.idx)
}
