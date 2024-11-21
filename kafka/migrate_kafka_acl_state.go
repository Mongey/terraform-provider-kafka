package kafka

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func migrateKafkaAclState(v int, is *terraform.InstanceState, meta interface{}) (*terraform.InstanceState, error) {
	switch v {
	case 0:
		log.Println("[INFO] Found Kafka ACL v0 state; migrating to v1")
		return migrateKafkaAclV0toV1(is)
	default:
		return is, fmt.Errorf("unexpected schema version: %d", v)
	}
}

func migrateKafkaAclV0toV1(is *terraform.InstanceState) (*terraform.InstanceState, error) {
	if is.Empty() {
		log.Println("[DEBUG] Empty InstanceState; nothing to migrate.")
		return is, nil
	}
	log.Printf("[DEBUG] ACL Attributes before migration: %#v", is.Attributes)

	if _, ok := is.Attributes["resource_pattern_type_filter"]; !ok {
		is.Attributes["resource_pattern_type_filter"] = "Literal"
	}

	log.Printf("[DEBUG] ACL Attributes after migration: %#v", is.Attributes)

	return is, nil
}
