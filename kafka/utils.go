package kafka

import (
	"fmt"
	"strings"
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
	wellFormed := make([]string, 0)

	for _, bs := range bootstrapServers {
		trimmed := strings.TrimSpace(bs)
		if trimmed != "" {
			wellFormed = append(wellFormed, trimmed)
		}
	}

	return wellFormed
}

// TODO: can I just get rid of this?
func strPtrMapToStrMap(c map[string]*string) map[string]string {
	foo := map[string]string{}
	for k, v := range c {
		foo[k] = *v
	}
	return foo
}
