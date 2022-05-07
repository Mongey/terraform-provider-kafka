package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

//lintignore:AT001
func TestAcc_LazyInit(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := "localhost:90"

	_, err = overrideProvider()
	if err != nil {
		t.Fatal(err)
	}

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		Steps: []r.TestStep{
			{
				Config:             cfg(t, bs, fmt.Sprintf(test_allResources_config, topicName, topicName)),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

const test_allResources_config = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}

resource "kafka_acl" "test" {
	resource_name                = "%s"
	resource_type                = "Topic"
	resource_pattern_type_filter = "Prefixed"
	acl_principal                = "User:Alice"
	acl_host                     = "*"
	acl_operation                = "Write"
	acl_permission_type          = "Deny"
}
`
