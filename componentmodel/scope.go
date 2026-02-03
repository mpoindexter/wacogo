package componentmodel

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/partite-ai/wacogo/ast"
	"github.com/tetratelabs/wazero"
)

type scope struct {
	enclosingScope       *scope
	instance             *Instance
	runtime              wazero.Runtime
	arguments            map[string]*instanceArgument
	sortBoundDefinitions []any
	currentType          Type
	localResourceTypes   map[*ResourceType]struct{}
}

func newScope(enclosingScope *scope, instance *Instance, runtime wazero.Runtime, args map[string]*instanceArgument) *scope {
	boundDefs := make([]any, numSorts)
	return &scope{
		enclosingScope:       enclosingScope,
		instance:             instance,
		runtime:              runtime,
		arguments:            args,
		sortBoundDefinitions: boundDefs,
	}
}

func (s *scope) closure(count uint32) (*scope, error) {
	current := s
	for i := 0; i < int(count); i++ {
		if current.enclosingScope == nil {
			return nil, fmt.Errorf("invalid outer alias count of %d", count)
		}
		current = current.enclosingScope
	}
	return current, nil
}

func (s *scope) componentScope(argTypes map[string]Type) *scope {
	args := make(map[string]*instanceArgument)
	for name, typ := range argTypes {
		args[name] = &instanceArgument{typ: typ}
	}
	return newScope(s, nil, s.runtime, args)
}

func (s *scope) instanceScope(instance *Instance, args map[string]*instanceArgument) *scope {
	return newScope(s.enclosingScope, instance, s.runtime, args)
}

func (s *scope) resolveArgumentType(name string) (Type, error) {
	ia, ok := resolveArgumentValue(s.arguments, name)
	if !ok {
		return nil, fmt.Errorf("missing import named `%s`", name)
	}
	return ia.typ, nil
}

func (s *scope) resolveArgumentValue(ctx context.Context, name string) (any, error) {
	ia, ok := resolveArgumentValue(s.arguments, name)
	if !ok {
		return nil, fmt.Errorf("missing import named `%s`", name)
	}
	return ia.val, nil
}

type sortScope[V any, T Type] struct {
	items []*boundDefinition[V, T]
	sort  sort[V, T]
}

func (s *sortScope[V, T]) len() uint32 {
	return uint32(len(s.items))
}

func (s *sortScope[V, T]) get(idx uint32) (*boundDefinition[V, T], error) {
	if int(idx) >= len(s.items) {
		return nil, fmt.Errorf("%s index out of bounds: %d", s.sort.typeName(), idx)
	}
	return s.items[idx], nil
}

func (s *sortScope[V, T]) getType(idx uint32) (T, error) {
	d, err := s.get(idx)
	if err != nil {
		return zero[T](), err
	}
	return d.getType(), nil
}

func (s *sortScope[V, T]) getInstance(idx uint32) (V, error) {
	d, err := s.get(idx)
	if err != nil {
		return zero[V](), err
	}
	return d.getInstance()
}

func (s *sortScope[V, T]) add(item *boundDefinition[V, T]) uint32 {
	s.items = append(s.items, item)
	return uint32(len(s.items) - 1)
}

func sortScopeFor[V any, T Type](scope *scope, sort sort[V, T]) *sortScope[V, T] {
	sbd := scope.sortBoundDefinitions[int(sort)]
	if sbd == nil {
		sbd = &sortScope[V, T]{sort: sort}
		scope.sortBoundDefinitions[int(sort)] = sbd
	}
	return sbd.(*sortScope[V, T])
}

type instanceArgument struct {
	val any
	typ Type
}

func resolveArgumentValue[T any](args map[string]T, name string) (T, bool) {
	// Exact match check
	val, ok := args[name]
	if ok {
		return val, true
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
		for argName, argVal := range args {
			if strings.HasPrefix(argName, matchPrefix) {
				return argVal, true
			}
		}
	}
	return zero[T](), false
}

func sortForSortID(id uint32) genericSort {
	switch ast.Sort(id) {
	case ast.SortCoreFunc:
		return sortCoreFunction
	case ast.SortCoreMemory:
		return sortCoreMemory
	case ast.SortCoreTable:
		return sortCoreTable
	case ast.SortCoreGlobal:
		return sortCoreGlobal
	case ast.SortCoreType:
		return sortCoreType
	case ast.SortCoreModule:
		return sortCoreModule
	case ast.SortCoreInstance:
		return sortCoreInstance
	case ast.SortFunc:
		return sortFunction
	case ast.SortType:
		return sortType
	case ast.SortComponent:
		return sortComponent
	case ast.SortInstance:
		return sortInstance
	default:
		return nil
	}
}

func typeForCoreSortIdx(scope *scope, sortIdx ast.CoreSortIdx) (Type, error) {
	switch sortIdx.Sort {
	case ast.CoreSortFunc:
		return sortScopeFor(scope, sortCoreFunction).getType(sortIdx.Idx)
	case ast.CoreSortGlobal:
		return sortScopeFor(scope, sortCoreGlobal).getType(sortIdx.Idx)
	case ast.CoreSortMemory:
		return sortScopeFor(scope, sortCoreMemory).getType(sortIdx.Idx)
	case ast.CoreSortTable:
		return sortScopeFor(scope, sortCoreTable).getType(sortIdx.Idx)
	case ast.CoreSortType:
		return sortScopeFor(scope, sortCoreType).getType(sortIdx.Idx)
	case ast.CoreSortInstance:
		return sortScopeFor(scope, sortCoreInstance).getType(sortIdx.Idx)
	case ast.CoreSortModule:
		return sortScopeFor(scope, sortCoreModule).getType(sortIdx.Idx)
	default:
		return nil, fmt.Errorf("unsupported core export sort: %v", sortIdx.Sort)
	}
}

func typeForSortIdx(scope *scope, sortIdx *ast.SortIdx) (Type, error) {
	switch sortIdx.Sort {
	case ast.SortCoreFunc, ast.SortCoreGlobal, ast.SortCoreMemory, ast.SortCoreTable, ast.SortCoreType, ast.SortCoreInstance, ast.SortCoreModule:
		return typeForCoreSortIdx(scope, ast.CoreSortIdx{
			Sort: ast.CoreSort(sortIdx.Sort),
			Idx:  sortIdx.Idx,
		})
	case ast.SortFunc:
		return sortScopeFor(scope, sortFunction).getType(sortIdx.Idx)
	case ast.SortType:
		return sortScopeFor(scope, sortType).getType(sortIdx.Idx)
	case ast.SortComponent:
		return sortScopeFor(scope, sortComponent).getType(sortIdx.Idx)
	case ast.SortInstance:
		return sortScopeFor(scope, sortInstance).getType(sortIdx.Idx)
	default:
		return nil, fmt.Errorf("unsupported sort: %v", sortIdx.Sort)
	}
}

func instanceForSortIdx(scope *scope, sortIdx *ast.SortIdx) (any, error) {
	switch sortIdx.Sort {
	case ast.SortCoreFunc:
		return sortScopeFor(scope, sortCoreFunction).getInstance(sortIdx.Idx)
	case ast.SortCoreGlobal:
		return sortScopeFor(scope, sortCoreGlobal).getInstance(sortIdx.Idx)
	case ast.SortCoreMemory:
		return sortScopeFor(scope, sortCoreMemory).getInstance(sortIdx.Idx)
	case ast.SortCoreTable:
		return sortScopeFor(scope, sortCoreTable).getInstance(sortIdx.Idx)
	case ast.SortCoreType:
		return sortScopeFor(scope, sortCoreType).getInstance(sortIdx.Idx)
	case ast.SortCoreInstance:
		return sortScopeFor(scope, sortCoreInstance).getInstance(sortIdx.Idx)
	case ast.SortCoreModule:
		return sortScopeFor(scope, sortCoreModule).getInstance(sortIdx.Idx)
	case ast.SortFunc:
		return sortScopeFor(scope, sortFunction).getInstance(sortIdx.Idx)
	case ast.SortType:
		return sortScopeFor(scope, sortType).getInstance(sortIdx.Idx)
	case ast.SortComponent:
		return sortScopeFor(scope, sortComponent).getInstance(sortIdx.Idx)
	case ast.SortInstance:
		return sortScopeFor(scope, sortInstance).getInstance(sortIdx.Idx)
	default:
		return nil, fmt.Errorf("unsupported sort: %v", sortIdx.Sort)
	}
}

type importPlaceholderType struct{}

func (t importPlaceholderType) isType() {}

func (t importPlaceholderType) typeSize() int {
	return 0
}

func (t importPlaceholderType) typeDepth() int {
	return 0
}

func (t importPlaceholderType) checkType(other Type, typeChecker typeChecker) error {
	return fmt.Errorf("cannot check type compatibility with import placeholder type")
}

func (t importPlaceholderType) typeName() string {
	return "import_placeholder"
}
