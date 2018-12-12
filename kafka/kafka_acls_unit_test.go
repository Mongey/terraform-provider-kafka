package kafka

import (
	"testing"

	"github.com/Shopify/sarama"
)

func TestACLToAclCreation(t *testing.T) {
	acl := stringlyTypedACL{
		ACL: ACL{
			Principal:      "principal",
			Host:           "*",
			Operation:      "Write",
			PermissionType: "Allow",
		},
		Resource: Resource{
			Type:              "Topic",
			Name:              "vault_logs",
			PatternTypeFilter: "prefixed",
		},
	}

	aclCreation, err := acl.AclCreation()
	if err != nil {
		t.Fatal(err)
	}

	if aclCreation.Resource.ResourceType != sarama.AclResourceTopic {
		t.Fatal("Invalid conversion")
	}
}
