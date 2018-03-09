package kafka

import (
	"fmt"
	"log"
	"time"

	"errors"

	"github.com/Shopify/sarama"
)

type TopicMissingError struct {
	msg string
}

func (e TopicMissingError) Error() string { return e.msg }

type Client struct {
	client sarama.Client
	config *Config
}

type Config struct {
	BootstrapServers *[]string
	Timeout          int
}

func NewClient(config *Config) (*Client, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V1_0_0_0

	log.Printf("[INFO] configuring bootstrap_servers %v", config)
	bootstrapServers := *(config.BootstrapServers)

	if bootstrapServers == nil {
		return nil, fmt.Errorf("No bootstrap_servers provided")
	}

	c, err := sarama.NewClient(bootstrapServers, kafkaConfig)
	if err != nil {
		fmt.Println("Error connecting to kafka")
		return nil, err
	}

	return &Client{
		client: c,
		config: config,
	}, kafkaConfig.Validate()
}

func (c *Client) DeleteTopic(t string) error {
	broker, err := c.availableBroker()

	if err != nil {
		return err
	}

	timeout := time.Duration(c.config.Timeout) * time.Second
	req := &sarama.DeleteTopicsRequest{
		Topics:  []string{t},
		Timeout: timeout,
	}
	res, err := broker.DeleteTopics(req)

	if err == nil {
		for k, e := range res.TopicErrorCodes {
			if e != sarama.ErrNoError {
				return fmt.Errorf("%s : %s", k, e)
			}
		}
	} else {
		log.Printf("[ERROR] Error deleting topic %s from Kafka\n", err)
		return err
	}

	log.Printf("[INFO] Deleted topic %s from Kafka", t)

	return nil
}

func (c *Client) UpdateTopic(topic Topic) error {
	broker, err := c.availableBroker()

	if err != nil {
		return err
	}

	r := &sarama.AlterConfigsRequest{
		Resources:    configToResources(topic),
		ValidateOnly: false,
	}

	res, err := broker.AlterConfigs(r)

	if err != nil {
		return err
	}

	if err == nil {
		for _, e := range res.Resources {
			if e.ErrorCode != int16(sarama.ErrNoError) {
				return errors.New(e.ErrorMsg)
			}
		}
	}

	return nil
}

func (c *Client) CreateTopic(t Topic) error {
	broker, err := c.availableBroker()

	if err != nil {
		log.Printf("[WARN] DERP %s", err)
		return err
	}

	timeout := time.Duration(c.config.Timeout) * time.Second
	log.Printf("[DEBUG] Timeout is %v ", timeout)
	req := &sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			t.Name: {
				NumPartitions:     t.Partitions,
				ReplicationFactor: t.ReplicationFactor,
				ConfigEntries:     t.Config,
			},
		},
		Timeout: timeout,
	}
	res, err := broker.CreateTopics(req)

	if err == nil {
		for _, e := range res.TopicErrors {
			if e.Err != sarama.ErrNoError {
				return fmt.Errorf("%s", e.Err)
			}
		}
		log.Printf("[INFO] Created topic %s in Kafka", t.Name)
	}

	return err
}

func (c *Client) AddPartitions(t Topic) error {
	broker, err := c.availableBroker()

	if err != nil {
		log.Printf("[WARN] DERP %s", err)
		return err
	}
	timeout := time.Duration(c.config.Timeout) * time.Second
	log.Printf("[DEBUG] b of size %d", 1)
	tp := map[string]*sarama.TopicPartition{
		t.Name: &sarama.TopicPartition{
			Count: t.Partitions,
		},
	}
	log.Printf("[DEBUG] b of size %d", 2)
	req := &sarama.CreatePartitionsRequest{
		TopicPartitions: tp,
		Timeout:         timeout,
		ValidateOnly:    false,
	}
	log.Printf("[INFO] Adding partitions to %s in Kafka", t.Name)
	res, err := broker.CreatePartitions(req)
	if err == nil {
		for _, e := range res.TopicPartitionErrors {
			if e.Err != sarama.ErrNoError {
				return fmt.Errorf("%s", e.Err)
			}
		}
		log.Printf("[INFO] Added partitions to %s in Kafka", t.Name)
	}

	return err
}

func (c *Client) availableBroker() (*sarama.Broker, error) {
	var err error
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V1_0_0_0

	brokers := *c.config.BootstrapServers

	log.Printf("[DEBUG] Looking for Brokers @ %v", brokers)
	for _, b := range brokers {
		broker := sarama.NewBroker(b)
		err = broker.Open(kafkaConfig)
		if err == nil {
			return broker, nil
		}
		log.Printf("[WARN] Broker @ %s cannot be reached\n", b)
	}

	return nil, fmt.Errorf("No Available Brokers @ %v", brokers)
}

func (client *Client) ReadTopic(name string) (Topic, error) {
	c := client.client

	topic := Topic{
		Name: name,
	}

	err := c.RefreshMetadata()
	topics, err := c.Topics()

	if err != nil {
		log.Printf("[ERROR] Error getting topics %s from Kafka", err)
		return topic, err
	}

	for _, t := range topics {
		log.Printf("[DEBUG] Reading Topic %s from Kafka", t)
		if name == t {
			log.Printf("[DEBUG] FOUND %s from Kafka", t)
			p, err := c.Partitions(t)
			if err == nil {
				log.Printf("[DEBUG] Partitions %v from Kafka", p)
				topic.Partitions = int32(len(p))

				r, err := ReplicaCount(c, name, p)
				if err == nil {
					log.Printf("[DEBUG] ReplicationFactor %d from Kafka", r)
					topic.ReplicationFactor = int16(r)
				}
				configToSave, err := client.topicConfig(t)
				if err != nil {
					log.Printf("[ERROR] Could not get config for topic %s: %s", t, err)
					return topic, err
				}

				log.Printf("[DEBUG] Config %v from Kafka", strPtrMapToStrMap(configToSave))
				topic.Config = configToSave
				return topic, nil
			}
		}
	}
	err = TopicMissingError{msg: fmt.Sprintf("%s could not be found", name)}
	return topic, err
}

func (c *Client) topicConfig(topic string) (map[string]*string, error) {
	conf := map[string]*string{}
	request := &sarama.DescribeConfigsRequest{
		Resources: []*sarama.ConfigResource{
			{
				Type: sarama.TopicResource,
				Name: topic,
			},
		},
	}

	broker, err := c.availableBroker()
	if err != nil {
		return conf, err
	}
	cr, err := broker.DescribeConfigs(request)

	if err != nil {
		return conf, err
	}

	if len(cr.Resources) > 0 && len(cr.Resources[0].Configs) > 0 {
		for _, tConf := range cr.Resources[0].Configs {
			if tConf.Default {
				continue
			}
			v := tConf.Value
			conf[tConf.Name] = &v
		}
	}
	return conf, nil
}
