package kafka

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestMigrateKafkaAclV0toV1_no_pattern_filter(t *testing.T) {
	oldAttributes := map[string]string{
		"acl_host":            "*",
		"acl_operation":       "Create",
		"acl_permission_type": "Allow",
		"acl_principal":       "Group:3867_stg",
		"resource_name":       "3867.stg.customer.in.core",
		"resource_type":       "Topic",
	}

	newState, err := migrateKafkaAclV0toV1(&terraform.InstanceState{
		ID:         "Group:3867_stg|*|Read|Allow|Topic|3867.stg.customer.in.core",
		Attributes: oldAttributes,
	})

	if err != nil {
		t.Fatal(err)
	}

	expectedAttributes := map[string]string{
		"acl_host":                     "*",
		"acl_operation":                "Create",
		"acl_permission_type":          "Allow",
		"acl_principal":                "Group:3867_stg",
		"resource_name":                "3867.stg.customer.in.core",
		"resource_pattern_type_filter": "Literal",
		"resource_type":                "Topic",
	}
	if !reflect.DeepEqual(newState.Attributes, expectedAttributes) {
		t.Fatalf("Expected attributes:\n%#v\n\nGiven:\n%#v\n",
			expectedAttributes, newState.Attributes)
	}
}

func TestMigrateKafkaAclV0toV1_with_pattern_filter(t *testing.T) {
	oldAttributes := map[string]string{
		"acl_host":                     "*",
		"acl_operation":                "Create",
		"acl_permission_type":          "Allow",
		"acl_principal":                "Group:3867_stg",
		"resource_name":                "3867.stg.customer.in.core",
		"resource_pattern_type_filter": "Prefix",
		"resource_type":                "Topic",
	}

	newState, err := migrateKafkaAclV0toV1(&terraform.InstanceState{
		ID:         "Group:3867_stg|*|Read|Allow|Topic|3867.stg.customer.in.core",
		Attributes: oldAttributes,
	})

	if err != nil {
		t.Fatal(err)
	}

	expectedAttributes := map[string]string{
		"acl_host":                     "*",
		"acl_operation":                "Create",
		"acl_permission_type":          "Allow",
		"acl_principal":                "Group:3867_stg",
		"resource_name":                "3867.stg.customer.in.core",
		"resource_pattern_type_filter": "Prefix",
		"resource_type":                "Topic",
	}
	if !reflect.DeepEqual(newState.Attributes, expectedAttributes) {
		t.Fatalf("Expected attributes:\n%#v\n\nGiven:\n%#v\n",
			expectedAttributes, newState.Attributes)
	}
}
