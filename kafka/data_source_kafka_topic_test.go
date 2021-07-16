package kafka

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func kafkaClient(provider *schema.Provider) (*LazyClient, error) {
	meta := testProvider.Meta()
	if meta == nil {
		return nil, fmt.Errorf("No client found, was Configure() called on provider?")
	}
	client := meta.(*LazyClient)

	return client, nil
}

func TestAcc_TopicData(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip(" Acceptance tests skipped unless env 'TF_ACC' set")
	}
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	// ensure the provider is called so we can extract a kafka client from it
	_, err = overrideProvider()
	if err != nil {
		t.Fatal("Could not construct client", err)
	}

	client, err := kafkaClient(testProvider)
	if err != nil {
		t.Fatal("Could not construct client")
	}

	segmentMs := "12345"
	err = client.CreateTopic(Topic{
		topicName,
		1,
		1,
		map[string]*string{
			"segment.ms": &segmentMs,
		},
	})

	if err != nil {
		t.Fatalf("Unable to setup topic that is needed for data source test: %s", err)
	}

	r.Test(t, r.TestCase{
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"kafka": func() (*schema.Provider, error) {
				return overrideProvider()
			},
		},
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testDataSourceTopic, topicName)),
				Check:  testDataSourceTopic_existingTopicCheck,
			},
		},
	})
}

func testDataSourceTopic_existingTopicCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.kafka_topic.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	name := instanceState.ID

	if name != instanceState.Attributes["name"] {
		return fmt.Errorf("id doesn't match name")
	}

	if v, ok := instanceState.Attributes["replication_factor"]; ok && v != "1" {
		return fmt.Errorf("replication_factor did not match, got: %v", instanceState.Attributes["replication_factor"])
	}
	if v, ok := instanceState.Attributes["partitions"]; ok && v != "1" {
		return fmt.Errorf("partitions did not get match, got: %v", instanceState.Attributes["partitions"])
	}
	if v, ok := instanceState.Attributes["config.segment.ms"]; ok && v != "12345" {
		return fmt.Errorf("segment.ms did not get match, got: %v", instanceState.Attributes["config.segment.ms"])
	}

	return nil
}

//lintignore:AT004
const testDataSourceTopic = `
data "kafka_topic" "test" {
  name               = "%s"
}
`
