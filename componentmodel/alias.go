package componentmodel

import (
	"context"
	"fmt"
)

type coreExportAliasDefinition[T resolvedInstance[TT], TT Type] struct {
	instanceIdx uint32
	exportName  string
	exportType  TT
}

func newCoreExportAliasDefinition[T resolvedInstance[TT], TT Type](
	instanceIdx uint32,
	exportName string,
	exportType TT,
) *coreExportAliasDefinition[T, TT] {
	return &coreExportAliasDefinition[T, TT]{
		instanceIdx: instanceIdx,
		exportName:  exportName,
		exportType:  exportType,
	}
}

func (d *coreExportAliasDefinition[T, TT]) typ() TT {
	return d.exportType
}

func (d *coreExportAliasDefinition[T, TT]) resolve(ctx context.Context, scope *instanceScope) (T, error) {
	var zero T
	inst, err := resolve(ctx, scope, sortCoreInstance, d.instanceIdx)
	if err != nil {
		return zero, err
	}
	exportVal, typ, err := inst.getExport(d.exportName)
	if err != nil {
		return zero, err
	}

	if !d.exportType.assignableFrom(typ) {
		return zero, fmt.Errorf("export %s in instance %d is not of expected type", d.exportName, d.instanceIdx)
	}

	typedVal, ok := exportVal.(T)
	if !ok {
		return zero, fmt.Errorf("export %s in instance %d is not of expected type", d.exportName, d.instanceIdx)
	}
	return typedVal, nil
}

type exportAliasDefinition[T any, TT Type] struct {
	instanceIdx uint32
	exportName  string
	sort        sort[T, TT]
	exportType  TT
}

func newExportAliasDefinition[T any, TT Type](
	instanceIdx uint32,
	exportName string,
	sort sort[T, TT],
	exportType TT,
) *exportAliasDefinition[T, TT] {
	return &exportAliasDefinition[T, TT]{
		instanceIdx: instanceIdx,
		exportName:  exportName,
		sort:        sort,
		exportType:  exportType,
	}
}

func (d *exportAliasDefinition[T, TT]) typ() TT {
	return d.exportType
}

func (d *exportAliasDefinition[T, TT]) resolve(ctx context.Context, scope *instanceScope) (T, error) {
	var zero T
	inst, err := resolve(ctx, scope, sortInstance, d.instanceIdx)
	if err != nil {
		return zero, err
	}
	exportVal, _, err := inst.getExport(d.exportName)
	if err != nil {
		return zero, err
	}
	typedVal, ok := exportVal.(T)
	if !ok {
		return zero, fmt.Errorf("export %s in instance %d is not a %s", d.exportName, d.instanceIdx, d.sort.typeName())
	}
	return typedVal, nil
}

type outerAliasDefinition[T resolvedInstance[TT], TT Type] struct {
	outerIdx   uint32
	sort       sort[T, TT]
	idx        uint32
	exportType TT
}

func newOuterAliasDefinition[T resolvedInstance[TT], TT Type](
	outerIdx uint32,
	sort sort[T, TT],
	idx uint32,
	exportType TT,
) *outerAliasDefinition[T, TT] {
	return &outerAliasDefinition[T, TT]{
		outerIdx:   outerIdx,
		sort:       sort,
		idx:        idx,
		exportType: exportType,
	}
}

func (d *outerAliasDefinition[T, TT]) typ() TT {
	return d.exportType
}

func (d *outerAliasDefinition[T, TT]) resolve(ctx context.Context, scope *instanceScope) (T, error) {
	targetScope := scope
	for i := uint32(0); i < d.outerIdx; i++ {
		if targetScope.parent == nil {
			return *new(T), fmt.Errorf("no outer instance scope at index %d", d.outerIdx)
		}
		targetScope = targetScope.parent
	}
	return resolve(ctx, targetScope, d.sort, d.idx)
}
