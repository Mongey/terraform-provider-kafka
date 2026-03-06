package kafka

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// MapEq compares two maps, and checks that the keys and values are the same
func MapEq(result, expected map[string]*string) error {
	if len(result) != len(expected) {
		return fmt.Errorf("%v != %v", result, expected)
	}

	for expectedK, expectedV := range expected {
		if resultV, ok := result[expectedK]; ok {
			if resultV == nil && expectedV == nil {
				continue
			}
			if *resultV != *expectedV {
				return fmt.Errorf("result[%s]: %s != expected[%s]: %s", expectedK, *resultV, expectedK, *expectedV)
			}

		} else {
			return fmt.Errorf("result[%s] should exist", expectedK)
		}
	}
	return nil
}

func nonEmptyAndTrimmed(bootstrapServers []string) []string {
	// Clone to avoid modifying the original slice
	result := slices.Clone(bootstrapServers)

	// Trim all strings in place
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}

	// Remove empty strings
	result = slices.DeleteFunc(result, func(s string) bool {
		return s == ""
	})

	return result
}

// TODO: can I just get rid of this?
func strPtrMapToStrMap(c map[string]*string) map[string]string {
	foo := map[string]string{}
	for k, v := range c {
		foo[k] = *v
	}
	return foo
}

func validateDiagFunc(validateFunc func(interface{}, string) ([]string, []error)) schema.SchemaValidateDiagFunc {
	return func(i interface{}, path cty.Path) diag.Diagnostics {
		warnings, errs := validateFunc(i, fmt.Sprintf("%+v", path))
		var diags diag.Diagnostics
		for _, warning := range warnings {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  warning,
			})
		}
		for _, err := range errs {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  err.Error(),
			})
		}
		return diags
	}
}

func intEitherNegativeOneOrAtLeastOne() schema.SchemaValidateFunc {
	minVal := 1
	return func(i interface{}, k string) (warnings []string, errors []error) {
		v, ok := i.(int)
		if !ok {
			errors = append(errors, fmt.Errorf("expected type of %s to be integer", k))
			return warnings, errors
		}

		if v != 1 || v < minVal {
			errors = append(errors, fmt.Errorf("expected %s to be either -1 or at least (%d), got %d", k, minVal, v))
			return warnings, errors
		}

		return warnings, errors
	}
}
