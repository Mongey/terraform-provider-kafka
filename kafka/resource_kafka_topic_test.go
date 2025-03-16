package kafka

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/IBM/sarama"
)

func TestAcc_BasicTopic(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]
	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_noConfig, topicName)),
				Check:  testResourceTopic_noConfigCheck,
			},
		},
	})
}

func TestAcc_TopicConfigUpdate(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_initialConfig, topicName)),
				Check:  testResourceTopic_initialCheck,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updateConfig, topicName)),
				Check:  testResourceTopic_updateCheck,
			},
		},
	})
}

func TestAcc_TopicUpdatePartitions(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_initialConfig, topicName)),
				Check:  testResourceTopic_initialCheck,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updatePartitions, topicName)),
				Check:  testResourceTopic_updatePartitionsCheck,
			},
		},
	})
}

func TestAcc_TopicAlterReplicationFactor(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	keyEncoder := sarama.StringEncoder("same key -> same partition -> same ordering")
	messages := []*sarama.ProducerMessage{
		{
			Topic: topicName,
			Key:   keyEncoder,
			Value: sarama.StringEncoder("Krusty"),
		},
		{
			Topic: topicName,
			Key:   keyEncoder,
			Value: sarama.StringEncoder("Krab"),
		},
		{
			Topic: topicName,
			Key:   keyEncoder,
			Value: sarama.StringEncoder("Pizza"),
		},
	}

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updateRF, topicName, 1, 7)),
				Check: r.ComposeTestCheckFunc(
					testResourceTopic_produceMessages(messages),
					testResourceTopic_initialCheck),
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updateRF, topicName, 3, 7)),
				Check: r.ComposeTestCheckFunc(
					testResourceTopic_updateRFCheck,
					testResourceTopic_checkSameMessages(messages)),
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updateRF, topicName, 2, 8)),
				Check: r.ComposeTestCheckFunc(
					testResourceTopic_updateRFCheck,
					testResourceTopic_checkSameMessages(messages)),
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_updateRF, topicName, 1, 10)),
				Check: r.ComposeTestCheckFunc(
					testResourceTopic_updateRFCheck,
					testResourceTopic_checkSameMessages(messages)),
			},
		},
	})
}

func TestAcc_TopicForceDelete(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set, skipping acceptance test")
		return
	}

	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		PreventPostDestroyRefresh: true,
		CheckDestroy:      testAccCheckTopicDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_initialConfig, topicName)),
				Check: r.ComposeTestCheckFunc(
					testResourceTopic_initialCheck,
				),
			},
			{
				// Stop Kafka to simulate a cluster being unavailable
				PreConfig: func() {
					client := testProvider.Meta().(*LazyClient)
					// Save the original bootstrap servers for restoration after
					originalServers := client.config.BootstrapServers
					
					// Set invalid bootstrap servers to simulate unavailable cluster
					invalidServers := []string{"unavailable-host:9092"}
					client.config.BootstrapServers = &invalidServers
					
					// Restore after test
					t.Cleanup(func() {
						client.config.BootstrapServers = originalServers
					})
				},
				Config: cfg(t, bs, fmt.Sprintf(testResourceTopic_forceDeleteConfig, topicName)),
				Check: r.ComposeTestCheckFunc(
					// The topic shouldn't exist in Terraform state anymore
					// because it was force deleted
					func(s *terraform.State) error {
						_, ok := s.RootModule().Resources["kafka_topic.test"]
						if ok {
							return fmt.Errorf("kafka_topic.test still exists in state after force delete")
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccCheckTopicDestroy(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
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

	client := testProvider.Meta().(*LazyClient)
	_, err := client.ReadTopic(name, true)

	if _, ok := err.(TopicMissingError); !ok {
		return err
	}

	return nil
}

func testResourceTopic_noConfigCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
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

	client := testProvider.Meta().(*LazyClient)
	topic, err := client.ReadTopic(name, true)

	if err != nil {
		return err
	}

	if len(topic.Config) != 0 {
		return fmt.Errorf("expected no configs for %s, got %v", name, topic.Config)
	}

	return nil
}

func testResourceTopic_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
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

	client := testProvider.Meta().(*LazyClient)
	topic, err := client.ReadTopic(name, true)
	if err != nil {
		return err
	}

	if v, ok := topic.Config["retention.ms"]; ok && *v != "11111" {
		return fmt.Errorf("retention.ms did not get set got: %v", topic.Config)
	}

	if v, ok := topic.Config["segment.ms"]; ok && *v != "22222" {
		return fmt.Errorf("segment.ms !=  %v", topic)
	}

	return nil
}

func testResourceTopic_produceMessages(messages []*sarama.ProducerMessage) r.TestCheckFunc {
	return func(s *terraform.State) error {
		c := testProvider.Meta().(*LazyClient)

		config := c.inner.config
		kafkaConfig, err := config.newKafkaConfig()
		if err != nil {
			return err
		}
		kafkaConfig.Producer.Return.Errors = true
		kafkaConfig.Producer.Return.Successes = true
		kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
		kafkaConfig.Producer.Timeout = 90 * time.Second

		producer, err := sarama.NewSyncProducer(*config.BootstrapServers, kafkaConfig)
		if err != nil {
			return err
		}

		defer func() {
			if err := producer.Close(); err != nil {
				log.Fatalln(err)
			}
		}()

		if err := producer.SendMessages(messages); err != nil {
			return err
		}

		return nil
	}
}

func testResourceTopic_updateCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*LazyClient)
	name := instanceState.ID

	if name != instanceState.Attributes["name"] {
		return fmt.Errorf("id doesn't match name")
	}

	topic, err := client.ReadTopic(name, true)
	if err != nil {
		return err
	}

	if v, ok := topic.Config["segment.ms"]; ok && *v != "33333" {
		return fmt.Errorf("segment.ms did not get updated, got: %v", topic.Config)
	}
	if v, ok := topic.Config["segment.bytes"]; ok && *v != "44444" {
		return fmt.Errorf("segment.bytes did not get updated, got: %s, expected 44444", *v)
	}

	if v, ok := topic.Config["retention.ms"]; ok || v != nil {
		return fmt.Errorf("retention.ms did not get removed, got: %v", topic.Config)
	}

	return nil
}

func testResourceTopic_updatePartitionsCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*LazyClient)

	name := instanceState.ID
	topic, err := client.ReadTopic(name, true)
	if err != nil {
		return err
	}
	if topic.Partitions != 2 {
		return fmt.Errorf("partitions did not get increated got: %d", topic.Partitions)
	}

	if v, ok := topic.Config["segment.ms"]; ok && *v != "33333" {
		return fmt.Errorf("segment.ms !=  %v", topic)
	}
	return nil
}

func testResourceTopic_updateRFCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*LazyClient)
	topicName := instanceState.Attributes["name"]

	parsed, err := strconv.ParseInt(instanceState.Attributes["replication_factor"], 10, 16)
	if err != nil {
		return err
	}
	expectedRF := int16(parsed)

	parsed, err = strconv.ParseInt(instanceState.Attributes["partitions"], 10, 32)
	if err != nil {
		return err
	}
	expectedPartitions := int32(parsed)

	topic, err := client.ReadTopic(topicName, true)
	if err != nil {
		return err
	}

	if actual := topic.ReplicationFactor; actual != expectedRF {
		return fmt.Errorf("expected replication.factor of %d, but got %d", expectedRF, actual)
	}

	if actual := topic.Partitions; actual != expectedPartitions {
		return fmt.Errorf("expected %d partitions, but got %d", expectedPartitions, actual)
	}

	return nil
}

func testResourceTopic_checkSameMessages(producedMessages []*sarama.ProducerMessage) r.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState := s.Modules[0].Resources["kafka_topic.test"]
		instanceState := resourceState.Primary
		client := testProvider.Meta().(*LazyClient)
		topicName := instanceState.Attributes["name"]

		consumer, err := sarama.NewConsumerFromClient(client.inner.client)
		if err != nil {
			return err
		}

		defer func() {
			if err := consumer.Close(); err != nil {
				log.Fatalln(err)
			}
		}()

		// all messages are produced to the same partition
		partition := producedMessages[0].Partition
		maxCount := len(producedMessages)
		consumedMessages, err := testConsumePartition(consumer, topicName, partition, maxCount)
		if err != nil {
			return err
		}

		return testSameMessages(producedMessages, consumedMessages)
	}
}

func testConsumePartition(consumer sarama.Consumer, topic string, partition int32, maxCount int) ([]*sarama.ConsumerMessage, error) {
	consumedMessages := make([]*sarama.ConsumerMessage, 0, maxCount)

	partitionConsumer, err := consumer.ConsumePartition(topic, partition, sarama.OffsetOldest)
	if err != nil {
		return consumedMessages, err
	}

	defer func() {
		if err := partitionConsumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	for len(consumedMessages) < maxCount {
		select {
		case message := <-partitionConsumer.Messages():
			consumedMessages = append(consumedMessages, message)
		case <-time.After(10 * time.Second):
			return consumedMessages, nil
		}
	}

	return consumedMessages, nil
}

func testSameMessages(produced []*sarama.ProducerMessage, consumed []*sarama.ConsumerMessage) error {
	plen := len(produced)
	clen := len(consumed)
	if plen != clen {
		return fmt.Errorf("produced and consumed message counts differ, %d != %d", plen, clen)
	}

	for i, pm := range produced {
		cm := consumed[i]
		if err := testMessageEqual(pm, cm); err != nil {
			return err
		}
	}

	return nil
}

func testMessageEqual(pm *sarama.ProducerMessage, cm *sarama.ConsumerMessage) error {
	if pm.Partition != cm.Partition {
		return fmt.Errorf("different partitions: %d != %d", pm.Partition, cm.Partition)
	}

	if pm.Offset != cm.Offset {
		return fmt.Errorf("different offsets: %d != %d", pm.Offset, cm.Offset)
	}

	pmBytes, err := pm.Value.Encode()
	if err != nil {
		return err
	}

	pmValue := string(pmBytes)
	cmValue := string(cm.Value)
	if pmValue != cmValue {
		return fmt.Errorf("different values: %v != %v", pmValue, cmValue)
	}

	return nil
}

const testResourceTopic_noConfig = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}
`

const testResourceTopic_initialConfig = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1

  config = {
    "retention.ms" = "11111"
    "segment.ms" = "22222"
  }
}
`

const testResourceTopic_updateConfig = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1

  config = {
    "segment.ms" = "33333"
    "segment.bytes" = "44444"
  }
}
`

const testResourceTopic_updatePartitions = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 2

  config = {
    "retention.ms" = "11111"
    "segment.ms" = "33333"
  }
}
`

const testResourceTopic_updateRF = `
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = %d
  partitions         = %d

  config = {
    "retention.ms" = "11111"
    "segment.ms" = "22222"
  }
}
`

const testResourceTopic_forceDeleteConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  force_delete = true
}

resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}
`
