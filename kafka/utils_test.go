package kafka

import (
	"slices"
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

func TestMapEq_subsetSemantics(t *testing.T) {
	a, b, c := "a", "b", "c"
	one, two, three := "1", "2", "3"

	tests := []struct {
		name     string
		result   map[string]*string
		expected map[string]*string
		wantErr  bool
	}{
		{
			name: "extra keys in result ignored",
			result: map[string]*string{
				"k1": &a, "k2": &b, "extra": &c,
			},
			expected: map[string]*string{
				"k1": &a, "k2": &b,
			},
			wantErr: false,
		},
		{
			name: "missing key in result",
			result: map[string]*string{
				"k1": &a,
			},
			expected: map[string]*string{
				"k1": &a, "k2": &b,
			},
			wantErr: true,
		},
		{
			name: "value mismatch",
			result: map[string]*string{
				"k1": &one, "k2": &two,
			},
			expected: map[string]*string{
				"k1": &one, "k2": &three,
			},
			wantErr: true,
		},
		{
			name: "identical maps",
			result: map[string]*string{
				"k1": &one, "k2": &two,
			},
			expected: map[string]*string{
				"k1": &one, "k2": &two,
			},
			wantErr: false,
		},
		{
			name:     "both nil maps",
			result:   nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "empty expected nil result",
			result:   nil,
			expected: map[string]*string{},
			wantErr:  false,
		},
		{
			name:     "nil expected empty result",
			result:   map[string]*string{},
			expected: nil,
			wantErr:  false,
		},
		{
			name: "nil pointer values both sides",
			result: map[string]*string{
				"k": nil,
			},
			expected: map[string]*string{
				"k": nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapEq(tt.result, tt.expected)
			if tt.wantErr {
				if err == nil {
					t.Fatal("MapEq: expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("MapEq: %v", err)
			}
		})
	}
}

func TestNonEmptyAndTrimmed(t *testing.T) {
	input := []string{"Hello ", "", " World"}
	expected := []string{"Hello", "World"}
	output := nonEmptyAndTrimmed(input)

	if !slices.Equal(output, expected) {
		t.Errorf("%v != %v", output, expected)
	}
}
