package kafka

import (
	"fmt"
	"strings"
	"testing"
)

func ParseCompositeQuotaID(id string) ([]CompositeQuotaEntity, error) {
	parts := strings.Split(id, "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid composite quota ID %q: expected 2 type:name segments separated by '|', got %d", id, len(parts))
	}
	entities := make([]CompositeQuotaEntity, 2)
	seenTypes := map[string]bool{}
	for i, p := range parts {
		colonIdx := strings.Index(p, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid composite quota ID segment %q: expected type:name format", p)
		}
		entityType := p[:colonIdx]
		entityName := p[colonIdx+1:]
		if entityName == entityDefault {
			entityName = ""
		}
		if entityType != "user" && entityType != "client-id" {
			return nil, fmt.Errorf("invalid entity type %q: must be 'user' or 'client-id'", entityType)
		}
		if seenTypes[entityType] {
			return nil, fmt.Errorf("invalid composite quota ID %q: duplicate entity type %q", id, entityType)
		}
		seenTypes[entityType] = true
		entities[i] = CompositeQuotaEntity{Type: entityType, Name: entityName}
	}
	return entities, nil
}

// ID must be stable regardless of entity order — Kafka returns entities in unspecified order.
func TestCompositeQuotaID_SortedDeterministically(t *testing.T) {
	userFirst := CompositeQuota{Entities: []CompositeQuotaEntity{{Type: "user"}, {Type: "client-id", Name: "my-service"}}}
	clientFirst := CompositeQuota{Entities: []CompositeQuotaEntity{{Type: "client-id", Name: "my-service"}, {Type: "user"}}}
	if userFirst.ID() != clientFirst.ID() {
		t.Errorf("ID() must be order-independent: %q != %q", userFirst.ID(), clientFirst.ID())
	}
	want := "client-id:my-service|user:entity-default"
	if got := userFirst.ID(); got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
}

func TestParseCompositeQuotaID_RoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		entities []CompositeQuotaEntity
	}{
		{"named client + default user", []CompositeQuotaEntity{{Type: "user"}, {Type: "client-id", Name: "my-service"}}},
		{"both named", []CompositeQuotaEntity{{Type: "user", Name: "alice"}, {Type: "client-id", Name: "alice-producer"}}},
		{"both default", []CompositeQuotaEntity{{Type: "user"}, {Type: "client-id"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := CompositeQuota{Entities: tc.entities}
			id := q.ID()
			parsed, err := ParseCompositeQuotaID(id)
			if err != nil {
				t.Fatalf("ParseCompositeQuotaID(%q) error: %v", id, err)
			}
			reparsed := CompositeQuota{Entities: parsed}
			if reparsed.ID() != id {
				t.Errorf("round-trip ID mismatch: original %q, reparsed %q", id, reparsed.ID())
			}
		})
	}
}

func TestParseCompositeQuotaID_InvalidInputs(t *testing.T) {
	cases := []string{
		"",
		"no-colon",
		"client-id:foo",
		"client-id:foo|no-colon",
		"client-id:foo|user:bar|extra:baz",
		"user:alice|user:bob",
		"client-id:foo|client-id:bar",
		"ip:1.2.3.4|user:alice",
	}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			if _, err := ParseCompositeQuotaID(id); err == nil {
				t.Errorf("ParseCompositeQuotaID(%q) expected error, got nil", id)
			}
		})
	}
}

// "type:" (empty name after colon) is an alias for entity-default.
func TestParseCompositeQuotaID_EmptyColonAlias(t *testing.T) {
	entities, err := ParseCompositeQuotaID("user:|client-id:my-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entities {
		if e.Type == "user" && e.Name != "" {
			t.Errorf("expected empty name for user entity, got %q", e.Name)
		}
	}
}

// SDK stores TypeMap values as strings after a read-apply cycle; both float64 and string must be accepted.
func TestConfigOpsFromMap_StringFallback(t *testing.T) {
	ops, err := configOpsFromMap(map[string]interface{}{"producer_byte_rate": "1048576"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Value != 1048576 {
		t.Errorf("unexpected ops: %v", ops)
	}
}

func TestConfigOpsFromMap_InvalidString(t *testing.T) {
	if _, err := configOpsFromMap(map[string]interface{}{"producer_byte_rate": "not-a-number"}, false); err == nil {
		t.Error("expected error for non-numeric string, got nil")
	}
}

// strconv.ParseFloat must reject trailing garbage that fmt.Sscanf would silently accept.
func TestConfigOpsFromMap_TrailingGarbage(t *testing.T) {
	if _, err := configOpsFromMap(map[string]interface{}{"producer_byte_rate": "1000abc"}, false); err == nil {
		t.Error("expected error for string with trailing garbage, got nil")
	}
}

func TestValidateCompositeEntityName_RejectsInvalidChars(t *testing.T) {
	for _, name := range []string{"bad|name", "bad:name"} {
		if _, errs := validateCompositeEntityName(name, "name"); len(errs) == 0 {
			t.Errorf("expected error for name %q", name)
		}
	}
}

func TestValidateCompositeEntityName_RejectsSentinel(t *testing.T) {
	if _, errs := validateCompositeEntityName(entityDefault, "name"); len(errs) == 0 {
		t.Error("expected error for name equal to entityDefault sentinel")
	}
}

// omitted name (Optional field absent) must not panic.
func TestCompositeEntityHash_NilNameNoPanic(t *testing.T) {
	_ = compositeEntityHash(map[string]interface{}{"type": "user"})
}
