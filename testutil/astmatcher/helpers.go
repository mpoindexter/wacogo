package astmatcher

import (
	"fmt"
)

// AnyComponent matches any component without validation
func AnyComponent() Matcher {
	return MatchComponent().Match
}

// AnyCoreModule matches any core module without validation
func AnyCoreModule() Matcher {
	return MatchCoreModule().Match
}

// AnyDefinition matches any definition without validation
func AnyDefinition() Matcher {
	return func(node interface{}) error {
		return nil
	}
}

// AnyExport matches any export without validation
func AnyExport() Matcher {
	return func(node interface{}) error {
		return nil
	}
}

// CountDefinitions validates the number of definitions without checking their content
func CountDefinitions(expectedCount int) func(node interface{}) error {
	return func(node interface{}) error {
		return MatchComponent().WithDefinitions(makeAnyMatchers(expectedCount)...).Match(node)
	}
}

// makeAnyMatchers creates a slice of matchers that accept any value
func makeAnyMatchers(count int) []Matcher {
	matchers := make([]Matcher, count)
	for i := 0; i < count; i++ {
		matchers[i] = AnyDefinition()
	}
	return matchers
}

// EmptyComponent matches a component with no definitions
func EmptyComponent() Matcher {
	return MatchComponent().WithDefinitions().Match
}

// ComponentWithDefinitionCount matches a component with a specific number of definitions
func ComponentWithDefinitionCount(count int) Matcher {
	return CountDefinitions(count)
}

// Custom validator helpers

// Not negates a matcher
func Not(matcher Matcher) Matcher {
	return func(node interface{}) error {
		err := matcher(node)
		if err == nil {
			return fmt.Errorf("expected matcher to fail, but it passed")
		}
		return nil
	}
}

// AllOf combines multiple matchers - all must pass
func AllOf(matchers ...Matcher) Matcher {
	return func(node interface{}) error {
		for i, matcher := range matchers {
			if err := matcher(node); err != nil {
				return fmt.Errorf("matcher %d failed: %w", i, err)
			}
		}
		return nil
	}
}

// AnyOf combines multiple matchers - at least one must pass
func AnyOf(matchers ...Matcher) Matcher {
	return func(node interface{}) error {
		var lastErr error
		for _, matcher := range matchers {
			if err := matcher(node); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return fmt.Errorf("all matchers failed, last error: %w", lastErr)
	}
}
