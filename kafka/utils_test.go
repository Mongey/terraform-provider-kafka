package kafka

import (
	"reflect"
	"testing"
)

func TestMapEq(t *testing.T) {
	foo := "foo"
	a := map[string]*string{
		"a": &foo,
	}

	moo := "foo"
	b := map[string]*string{
		"a": &moo,
	}

	err := MapEq(a, b)
	if err != nil {
		t.Fatalf("%s", err)
	}
}
func TestNonEmptyAndTrimmed(t *testing.T) {
	input := []string{"Hello ", "", " World"}
	expected := []string{"Hello", "World"}
	output := nonEmptyAndTrimmed(input)

	if !reflect.DeepEqual(output, expected) {
		t.Errorf("%v != %v", output, expected)
	}
}
