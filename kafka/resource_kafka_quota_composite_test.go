package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_CompositeQuota_UserDefaultClientNamed(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	clientName := fmt.Sprintf("composite-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckCompositeQuotaDestroy(t),
		Steps: []r.TestStep{
			{
				Config: cfgs(t, bs, fmt.Sprintf(testResourceCompositeQuotaUserDefaultClientNamed, clientName)),
				Check:  testResourceCompositeQuota_check(t),
			},
		},
	})
}

func compositeEntitiesFromState(t *testing.T, instanceState *terraform.InstanceState) []CompositeQuotaEntity {
	t.Helper()
	entities, err := ParseCompositeQuotaID(instanceState.ID)
	if err != nil {
		t.Fatalf("compositeEntitiesFromState: invalid resource ID %q: %v", instanceState.ID, err)
	}
	return entities
}

func testResourceCompositeQuota_check(t *testing.T) r.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		resourceState := s.Modules[0].Resources["kafka_quota_composite.test1"]
		if resourceState == nil {
			return fmt.Errorf("resource not found in state")
		}
		instanceState := resourceState.Primary
		if instanceState == nil {
			return fmt.Errorf("resource has no primary instance")
		}

		entities := compositeEntitiesFromState(t, instanceState)
		client := testProvider.Meta().(*LazyClient)
		quota, err := client.DescribeCompositeQuota(entities)
		if err != nil {
			return err
		}

		expectedID := CompositeQuota{Entities: entities}.ID()
		if instanceState.ID != expectedID {
			return fmt.Errorf("id mismatch: state has %s, expected %s", instanceState.ID, expectedID)
		}

		byKey := map[string]QuotaOp{}
		for _, q := range quota.Ops {
			byKey[q.Key] = q
		}

		if q, ok := byKey["consumer_byte_rate"]; !ok || q.Value != 4000000 || q.Remove {
			return fmt.Errorf("consumer_byte_rate: expected 4000000, got %v (all ops: %v)", byKey["consumer_byte_rate"], quota.Ops)
		}
		if q, ok := byKey["producer_byte_rate"]; !ok || q.Value != 2500000 || q.Remove {
			return fmt.Errorf("producer_byte_rate: expected 2500000, got %v (all ops: %v)", byKey["producer_byte_rate"], quota.Ops)
		}

		return nil
	}
}

func testAccCheckCompositeQuotaDestroy(t *testing.T) r.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		resourceState := s.Modules[0].Resources["kafka_quota_composite.test1"]
		if resourceState == nil {
			return fmt.Errorf("resource not found in state")
		}
		instanceState := resourceState.Primary
		if instanceState == nil {
			return fmt.Errorf("resource has no primary instance")
		}

		entities := compositeEntitiesFromState(t, instanceState)
		meta := testProvider.Meta()
		if meta == nil {
			return fmt.Errorf("provider Meta() returned nil")
		}

		client := meta.(*LazyClient)
		quota, err := client.DescribeCompositeQuota(entities)
		if err != nil {
			if _, ok := err.(QuotaMissingError); ok {
				return nil
			}
			return fmt.Errorf("unexpected error checking composite quota destroy: %v", err)
		}
		if len(quota.Ops) > 0 {
			return fmt.Errorf("composite quota still has active config keys after destroy: %v", quota.Ops)
		}
		return nil
	}
}

// users/<default>/clients/<named> — quota precedence #4.
const testResourceCompositeQuotaUserDefaultClientNamed = `
resource "kafka_quota_composite" "test1" {
  entity {
    type = "user"
  }
  entity {
    type = "client-id"
    name = "%s"
  }
  config = {
    "consumer_byte_rate" = "4000000"
    "producer_byte_rate" = "2500000"
  }
}
`
