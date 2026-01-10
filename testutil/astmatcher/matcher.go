package astmatcher

import (
	"fmt"

	"github.com/partite-ai/wacogo/ast"
)

// Matcher is a function that validates an AST node and returns an error if validation fails
type Matcher func(interface{}) error

// ComponentMatcher builds matchers for ast.Component
type ComponentMatcher struct {
	validators          []func(*ast.Component) error
	definitionMatchers  []Matcher
	validateDefinitions bool
}

// MatchComponent creates a new ComponentMatcher with optional validators
func MatchComponent(validators ...func(*ast.Component) error) *ComponentMatcher {
	return &ComponentMatcher{
		validators: validators,
	}
}

// WithDefinitions adds definition matchers to validate child definitions
func (m *ComponentMatcher) WithDefinitions(matchers ...Matcher) *ComponentMatcher {
	m.definitionMatchers = matchers
	m.validateDefinitions = true
	return m
}

// Match validates the component against all matchers
func (m *ComponentMatcher) Match(node interface{}) error {
	component, ok := node.(*ast.Component)
	if !ok {
		return fmt.Errorf("expected *ast.Component, got %T", node)
	}

	// Run custom validators
	for i, validator := range m.validators {
		if err := validator(component); err != nil {
			return fmt.Errorf("component validator %d failed: %w", i, err)
		}
	}

	// Validate definitions if matchers are provided
	if m.validateDefinitions {
		if len(component.Definitions) != len(m.definitionMatchers) {
			return fmt.Errorf("definition count mismatch: expected %d, got %d",
				len(m.definitionMatchers), len(component.Definitions))
		}

		for i, matcher := range m.definitionMatchers {
			if err := matcher(component.Definitions[i]); err != nil {
				return fmt.Errorf("definition %d: %w", i, err)
			}
		}
	}

	return nil
}

// CoreModuleMatcher builds matchers for ast.CoreModule
type CoreModuleMatcher struct {
	validators []func(*ast.CoreModule) error
}

// MatchCoreModule creates a new CoreModuleMatcher with optional validators
func MatchCoreModule(validators ...func(*ast.CoreModule) error) *CoreModuleMatcher {
	return &CoreModuleMatcher{
		validators: validators,
	}
}

// WithRawSize validates the size of the raw bytes
func (m *CoreModuleMatcher) WithRawSize(minSize int) *CoreModuleMatcher {
	m.validators = append(m.validators, func(module *ast.CoreModule) error {
		if len(module.Raw) < minSize {
			return fmt.Errorf("core module raw size too small: expected at least %d, got %d", minSize, len(module.Raw))
		}
		return nil
	})
	return m
}

func (m *CoreModuleMatcher) WithValidator(validator func(module *ast.CoreModule) error) *CoreModuleMatcher {
	m.validators = append(m.validators, validator)
	return m
}

// Match validates the core module against all matchers
func (m *CoreModuleMatcher) Match(node interface{}) error {
	module, ok := node.(*ast.CoreModule)
	if !ok {
		return fmt.Errorf("expected *ast.CoreModule, got %T", node)
	}

	// Run custom validators
	for i, validator := range m.validators {
		if err := validator(module); err != nil {
			return fmt.Errorf("core module validator %d failed: %w", i, err)
		}
	}

	return nil
}

// NestedComponentMatcher builds matchers for ast.NestedComponent
type NestedComponentMatcher struct {
	validators       []func(*ast.NestedComponent) error
	componentMatcher Matcher
}

// MatchNestedComponent creates a new NestedComponentMatcher with optional validators
func MatchNestedComponent(validators ...func(*ast.NestedComponent) error) *NestedComponentMatcher {
	return &NestedComponentMatcher{
		validators: validators,
	}
}

// WithComponent adds a component matcher
func (m *NestedComponentMatcher) WithComponent(matcher Matcher) *NestedComponentMatcher {
	m.componentMatcher = matcher
	return m
}

// Match validates the nested component against all matchers
func (m *NestedComponentMatcher) Match(node interface{}) error {
	nested, ok := node.(*ast.NestedComponent)
	if !ok {
		return fmt.Errorf("expected *ast.NestedComponent, got %T", node)
	}

	// Run custom validators
	for i, validator := range m.validators {
		if err := validator(nested); err != nil {
			return fmt.Errorf("nested component validator %d failed: %w", i, err)
		}
	}

	// Validate component if matcher is provided
	if m.componentMatcher != nil {
		if err := m.componentMatcher(nested.Component); err != nil {
			return fmt.Errorf("nested component: %w", err)
		}
	}

	return nil
}
