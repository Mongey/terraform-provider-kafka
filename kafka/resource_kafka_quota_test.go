package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_BasicQuota(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	quotaEntityName := fmt.Sprintf("quota1-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckQuotaDestroy,
		Steps: []r.TestStep{
			{
				Config: cfgs(t, bs, fmt.Sprintf(testResourceQuota1, quotaEntityName, "4000000")),
				Check:  testResourceQuota_initialCheck,
			},
		},
	})
}

func TestAcc_QuotaConfigUpdate(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	quotaEntityName := fmt.Sprintf("quota1-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckQuotaDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceQuota1, quotaEntityName, "4000000")),
				Check:  testResourceQuota_initialCheck,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceQuota1, quotaEntityName, "3000000")),
				Check:  testResourceQuota_updateCheck,
			},
		},
	})
}

func testResourceQuota_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_quota.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	entityType := instanceState.Attributes["entity_type"]
	entityName := instanceState.Attributes["entity_name"]

	client := testProvider.Meta().(*LazyClient)
	quota, err := client.DescribeQuota(entityType, entityName)
	if err != nil {
		return err
	}

	id := instanceState.ID
	qID := fmt.Sprintf("%s|%s", entityName, entityType)

	if id != qID {
		return fmt.Errorf("id doesn't match for %s, got %s", id, qID)
	}

	if len(quota.Ops) != 2 {
		return fmt.Errorf("expected configs for %s, got %v", quota.EntityName, quota.Ops)
	}

	for _, q := range quota.Ops {
		if q.Key != "consumer_byte_rate" && q.Key != "producer_byte_rate" {
			return fmt.Errorf("consumer_byte_rate and producer_byte_rate is missing got: %v", q)
		}
		if q.Key == "consumer_byte_rate" && (q.Value != 4000000 || q.Remove) {
			return fmt.Errorf("consumer_byte_rate did not get set, expected 4000000 got: %v", q)
		}
		if q.Key == "producer_byte_rate" && (q.Value != 2500000 || q.Remove) {
			return fmt.Errorf("producer_byte_rate did not get set, expected 2500000 got: %v", q)
		}
	}

	return nil
}

func testResourceQuota_updateCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_quota.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	entityType := instanceState.Attributes["entity_type"]
	entityName := instanceState.Attributes["entity_name"]

	client := testProvider.Meta().(*LazyClient)
	quota, err := client.DescribeQuota(entityType, entityName)
	if err != nil {
		return err
	}

	id := instanceState.ID
	qID := fmt.Sprintf("%s|%s", entityName, entityType)

	if id != qID {
		return fmt.Errorf("id doesn't match for %s, got %s", id, qID)
	}

	if len(quota.Ops) != 2 {
		return fmt.Errorf("expected configs for %s, got %v", quota.EntityName, quota.Ops)
	}

	for _, q := range quota.Ops {
		if q.Key != "consumer_byte_rate" && q.Key != "producer_byte_rate" {
			return fmt.Errorf("consumer_byte_rate and producer_byte_rate is missing got: %v", q)
		}
		if q.Key == "consumer_byte_rate" && (q.Value != 3000000 || q.Remove) {
			return fmt.Errorf("consumer_byte_rate did not get set, expected 3000000 got: %v", q)
		}
		if q.Key == "producer_byte_rate" && (q.Value != 2500000 || q.Remove) {
			return fmt.Errorf("producer_byte_rate did not get set, expected 2500000 got: %v", q)
		}
	}

	return nil
}

func testAccCheckQuotaDestroy(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_quota.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	entityType := instanceState.Attributes["entity_type"]
	entityName := instanceState.Attributes["entity_name"]

	client := testProvider.Meta().(*LazyClient)
	_, err := client.DescribeQuota(entityType, entityName)

	if _, ok := err.(QuotaMissingError); !ok {
		return fmt.Errorf("quota was found %v", err.Error())
	}

	return nil
}

func cfgs(t *testing.T, bs string, extraCfg string) string {
	return fmt.Sprintf(`
provider "kafka" {
	bootstrap_servers = ["%s"]
}

%s
`, bs, extraCfg)
}

const testResourceQuota1 = `
resource "kafka_quota" "test1" {
  provider = "kafka"
  entity_name               = "%s"
  entity_type               = "client-id"
  config = {
    "consumer_byte_rate" = "%s"
	"producer_byte_rate" = "2500000"
  }
}
`
