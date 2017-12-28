package kafka

import (
	"errors"
	"fmt"
	"log"

	samara "github.com/Shopify/sarama"
	"github.com/hashicorp/terraform/helper/schema"
)

// ReplicaCount returns the replication_factor for a partition
// Returns an error if it cannot determine the count, or if the number of
// replicas is different accross partitions
func ReplicaCount(c samara.Client, topic string, partitions []int32) (int, error) {
	count := -1

	for _, p := range partitions {
		replicas, err := c.Replicas(topic, p)
		if err != nil {
			return -1, errors.New("Could not get replicas for partition")
		}
		if count == -1 {
			count = len(replicas)
		}
		if count != len(replicas) {
			return count, fmt.Errorf("The replica count isn't the same across partitions %d != %d", count, len(replicas))
		}
	}
	return count, nil

}

// AvailableBrokerFromList finds a broker that we can talk to
// Returns the last know error
func AvailableBrokerFromList(brokers []string) (*samara.Broker, error) {
	var err error
	kafkaConfig := samara.NewConfig()
	kafkaConfig.Version = samara.V0_11_0_0
	fmt.Printf("Looking at %v", brokers)
	for _, b := range brokers {
		broker := samara.NewBroker(b)
		err = broker.Open(kafkaConfig)
		if err == nil {
			return broker, nil
		}
		log.Printf("[WARN] Broker @ %s cannot be reached", b)
	}

	return nil, err
}

type topicConfig struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Config            map[string]*string
}

func ConfigForTopic(topic string, brokers []string) (map[string]string, error) {
	confToSave := map[string]string{}
	request := &samara.DescribeConfigsRequest{
		Resources: []*samara.Resource{
			&samara.Resource{
				T:           samara.TopicResource,
				Name:        topic,
				ConfigNames: []string{"segment.ms"},
			},
		},
	}

	broker, err := AvailableBrokerFromList(brokers)
	if err != nil {
		return confToSave, err
	}
	cr, err := broker.DescribeConfigs(request)
	if err != nil {
		return confToSave, err
	}

	if len(cr.Resources) > 0 && len(cr.Resources[0].Configs) > 0 {
		for _, conf := range cr.Resources[0].Configs {
			if conf.Default {
				continue
			}
			log.Printf("[DEBUG] configs %s", conf.Name)
			log.Printf("[DEBUG] configs %s", conf.Value)
			confToSave[conf.Name] = conf.Value
		}
	}
	return confToSave, nil
}
func metaToTopicConfig(d *schema.ResourceData, meta interface{}) topicConfig {
	topicName := d.Get("name").(string)
	partitions := d.Get("partitions").(int)
	replicationFactor := d.Get("replication_factor").(int)
	convertedPartitions := int32(partitions)
	convertedRF := int16(replicationFactor)
	config := d.Get("config").(map[string]interface{})

	m2 := make(map[string]*string)
	for key, value := range config {
		switch value := value.(type) {
		case string:
			m2[key] = &value
		}
	}

	return topicConfig{
		Name:              topicName,
		Partitions:        convertedPartitions,
		ReplicationFactor: convertedRF,
		Config:            m2,
	}
}
