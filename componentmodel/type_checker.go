package componentmodel

import (
	"fmt"
	"iter"
)

type typeChecker interface {
	checkTypeCompatible(Type, Type) error
}

type typeCheckingScope struct {
	boundIdentities map[*ResourceType]*ResourceType
}

func newTypeChecker() *typeCheckingScope {
	return &typeCheckingScope{
		boundIdentities: make(map[*ResourceType]*ResourceType),
	}
}

func (s *typeCheckingScope) checkTypeCompatible(a Type, b Type) error {
	// Bind resource type bounds to a particular identity the first time we see them
	if aRT, ok := a.(*ResourceType); ok {
		if aRT.instance == resourceTypeBoundMarker {
			ident, ok := s.boundIdentities[aRT]
			if !ok {
				ident = aRT
				if bRT, ok := b.(*ResourceType); ok {
					s.boundIdentities[aRT] = bRT
					ident = bRT
				}
			}
			a = ident
		}
	}
	if err := a.checkType(b, s); err != nil {
		return err
	}
	return nil
}

type comparableType interface {
	Type
	comparable
}

func assertTypeIdentityEqual[T comparableType](a T, b Type) error {
	typedB, ok := b.(T)
	if !ok {
		return fmt.Errorf("type mismatch: expected %s, found %s", a.typeName(), b.typeName())
	}
	if a != typedB {
		return fmt.Errorf("type mismatch: expected %s, found %s", a.typeName(), b.typeName())
	}
	return nil
}

func assertCompositeTypeElementsEqual[T Type](a T, b Type, elements func(T) iter.Seq2[string, Type], checker typeChecker) (T, error) {
	typedB, ok := b.(T)
	if !ok {
		return zero[T](), fmt.Errorf("type mismatch: expected %s, found %s", a.typeName(), b.typeName())
	}

	nextA, stopA := iter.Pull2(elements(a))
	defer stopA()

	nextB, stopB := iter.Pull2(elements(typedB))
	defer stopB()

	for {
		kA, vA, okA := nextA()
		kB, vB, okB := nextB()

		if !okA && !okB {
			break
		}
		if okA && !okB {
			return zero[T](), fmt.Errorf("type mismatch: missing element named `%s`", kA)

		}
		if !okA && okB {
			return zero[T](), fmt.Errorf("type mismatch: extra element named `%s` found", kB)
		}
		if kA != kB {
			return zero[T](), fmt.Errorf("type mismatch: expected element named `%s`, found `%s`", kA, kB)
		}
		if err := checker.checkTypeCompatible(vA, vB); err != nil {
			return zero[T](), fmt.Errorf("type mismatch in element `%s`: %w", kA, err)
		}
	}
	return typedB, nil
}

func assertTypeKindIsSame[T Type](a T, b Type) (T, error) {
	typedB, ok := b.(T)
	if !ok {
		return zero[T](), fmt.Errorf("type mismatch: expected %s, found %s", a.typeName(), b.typeName())
	}
	return typedB, nil
}
