package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_LazyInit(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := "localhost:90"
	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config:   cfg(t, bs, fmt.Sprintf(testResourceTopic_noConfig, topicName)),
				PlanOnly: true,
			},
		},
	})
}
