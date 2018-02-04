package kafka

import "testing"

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
