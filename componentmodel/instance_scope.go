package componentmodel

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero"
)

type instanceArgument struct {
	val any
	typ Type
}

type instanceScope struct {
	definitionScope *definitionScope
	parent          *instanceScope
	currentInstance *Instance
	runtime         wazero.Runtime
	resolutions     map[any]any
	args            map[string]*instanceArgument
}

func newInstanceScope(parent *instanceScope, definitionScope *definitionScope, currentInstance *Instance, runtime wazero.Runtime, args map[string]*instanceArgument) *instanceScope {
	return &instanceScope{
		parent:          parent,
		definitionScope: definitionScope,
		currentInstance: currentInstance,
		runtime:         runtime,
		resolutions:     make(map[any]any),
		args:            args,
	}
}

func (s *instanceScope) resolveArgument(name string) (any, Type, error) {
	// Exact match check
	val, ok := s.args[name]
	if ok {
		return val.val, val.typ, nil
	}

	var matchPrefix string
	if iface, version, ok := strings.Cut(name, "@"); ok {
		versionParts := strings.Split(version, ".")
		if len(versionParts) == 3 {
			major := versionParts[0]
			minor := versionParts[1]
			patch := versionParts[2]
			majorNum, err := strconv.ParseUint(major, 10, 64)
			if err == nil {
				if majorNum > 0 {
					matchPrefix = fmt.Sprintf("%s@%d.", iface, majorNum)
				} else {
					minorNum, err := strconv.ParseUint(minor, 10, 64)
					if err == nil {
						if minorNum > 0 {
							matchPrefix = fmt.Sprintf("%s@%d.%d.", iface, majorNum, minorNum)
						} else {
							if patchNum, _, ok := strings.Cut(patch, "-"); ok {
								matchPrefix = fmt.Sprintf("%s@%d.%d.%s", iface, majorNum, minorNum, patchNum)
							}
						}
					}
				}
			}
		}
	}

	if matchPrefix != "" {
		for argName, argVal := range s.args {
			if strings.HasPrefix(argName, matchPrefix) {
				return argVal.val, argVal.typ, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("argument %s not found", name)
}

type resolvedInstance[TT Type] interface {
	comparable
	typ() TT
}

type instanceScopeResolutions[T any] []T

func resolve[T resolvedInstance[TT], TT Type](ctx context.Context, scope *instanceScope, sort sort[T, TT], idx uint32) (T, error) {
	var zero T
	defs := defs(scope.definitionScope, sort)
	resolutions, ok := scope.resolutions[sort].(instanceScopeResolutions[T])
	if !ok {
		resolutions = make(instanceScopeResolutions[T], defs.len())
		scope.resolutions[sort] = resolutions
	}

	if idx >= uint32(len(resolutions)) {
		return zero, fmt.Errorf("instance scope resolution index %d out of bounds", idx)
	}

	resolved := resolutions[idx]
	if resolved == zero {
		def, err := defs.get(idx)
		if err != nil {
			return zero, err
		}

		resolved, err = def.resolve(ctx, scope)
		if err != nil {
			return zero, err
		}
		(resolutions)[idx] = resolved
	}
	return resolved, nil
}

func resolveSortIdx(ctx context.Context, scope *instanceScope, sortIdx *ast.SortIdx) (any, Type, error) {
	switch sortIdx.Sort {
	case ast.SortCoreFunc:
		v, err := resolve(ctx, scope, sortCoreFunction, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreGlobal:
		v, err := resolve(ctx, scope, sortCoreGlobal, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreMemory:
		v, err := resolve(ctx, scope, sortCoreMemory, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreTable:
		v, err := resolve(ctx, scope, sortCoreTable, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreType:
		v, err := resolve(ctx, scope, sortCoreType, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreInstance:
		v, err := resolve(ctx, scope, sortCoreInstance, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortCoreModule:
		v, err := resolve(ctx, scope, sortCoreModule, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortFunc:
		v, err := resolve(ctx, scope, sortFunction, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortType:
		v, err := resolve(ctx, scope, sortType, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortComponent:
		v, err := resolve(ctx, scope, sortComponent, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	case ast.SortInstance:
		v, err := resolve(ctx, scope, sortInstance, sortIdx.Idx)
		if err != nil {
			return nil, nil, err
		}
		return v, v.typ(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported sort: %v", sortIdx.Sort)
	}
}
