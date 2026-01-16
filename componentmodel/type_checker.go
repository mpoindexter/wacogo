package componentmodel

import (
	"fmt"
)

// validateResourceTypeDefinedInComponent checks that a resource type is defined in the current component
// Resource types can only use resource.new, resource.drop, and resource.rep within the component that defined them
func validateResourceTypeDefinedInComponent(resourceType *ResourceType, currentInstance *Instance) error {
	if resourceType.instance != currentInstance {
		return fmt.Errorf("resource type not defined in current component instance")
	}
	return nil
}

// validateVariantCasesNonEmpty ensures variant types have at least one case
// Per the explainer: "variants are required to have a non-empty list of cases"
func validateVariantCasesNonEmpty(variant *VariantType) error {
	if len(variant.Cases) == 0 {
		return fmt.Errorf("variant type must have at least one case")
	}
	return nil
}

// validateRecordFieldNames ensures record fields have unique names
func validateRecordFieldNames(record *RecordType) error {
	seen := make(map[string]bool)
	for _, field := range record.Fields {
		if field.Name != "" {
			if seen[field.Name] {
				return fmt.Errorf("duplicate record field name: %s", field.Name)
			}
			seen[field.Name] = true
		}
	}
	return nil
}

// validateVariantCaseNames ensures variant cases have unique names
func validateVariantCaseNames(variant *VariantType) error {
	seen := make(map[string]bool)
	for _, c := range variant.Cases {
		if seen[c.Name] {
			return fmt.Errorf("duplicate variant case name: %s", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// validateFlagNames ensures flag names are unique
func validateFlagNames(flags *FlagsType) error {
	seen := make(map[string]bool)
	for _, name := range flags.FlagNames {
		if seen[name] {
			return fmt.Errorf("duplicate flag name: %s", name)
		}
		seen[name] = true
	}
	return nil
}
