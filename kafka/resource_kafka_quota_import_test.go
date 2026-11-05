package kafka

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestImportQuota(t *testing.T) {
	r := kafkaQuotaResource()
	if r.Importer == nil {
		t.Fatal("kafka_quota must support import")
	}

	cases := []struct {
		importID   string
		entityType string
		entityName string
		id         string
	}{
		{"client-id:my-client", "client-id", "my-client", "my-client|client-id"},
		{"user:alice", "user", "alice", "alice|user"},
		{"user:", "user", "", "entity-default|user"},
		{"ip:2001:db8::1", "ip", "2001:db8::1", "2001:db8::1|ip"},
	}

	for _, tc := range cases {
		t.Run(tc.importID, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{})
			d.SetId(tc.importID)

			got, err := r.Importer.StateContext(context.Background(), d, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(got))
			}
			if v := got[0].Get("entity_type").(string); v != tc.entityType {
				t.Errorf("entity_type = %q, want %q", v, tc.entityType)
			}
			if v := got[0].Get("entity_name").(string); v != tc.entityName {
				t.Errorf("entity_name = %q, want %q", v, tc.entityName)
			}
			if got[0].Id() != tc.id {
				t.Errorf("id = %q, want %q", got[0].Id(), tc.id)
			}
		})
	}

	for _, bad := range []string{"my-client", ":my-client", "group:x"} {
		t.Run("invalid "+bad, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{})
			d.SetId(bad)
			if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
				t.Fatalf("expected an error for %q", bad)
			}
		})
	}
}
