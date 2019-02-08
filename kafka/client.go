package kafka

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Shopify/sarama"
)

type TopicMissingError struct {
	msg string
}

func (e TopicMissingError) Error() string { return e.msg }

type Client struct {
	client        sarama.Client
	kafkaConfig   *sarama.Config
	config        *Config
	supportedAPIs map[int]int
}

func NewClient(config *Config) (*Client, error) {
	log.Printf("[INFO] configuring bootstrap_servers %v", config)
	bootstrapServers := *(config.BootstrapServers)

	if bootstrapServers == nil {
		return nil, fmt.Errorf("No bootstrap_servers provided")
	}

	kc, err := config.newKafkaConfig()
	if err != nil {
		log.Println("[ERROR] Error creating kafka client")
		return nil, err
	}

	c, err := sarama.NewClient(bootstrapServers, kc)
	sarama.Logger = log.New(os.Stdout, "[TRACE] [Sarama]", log.LstdFlags)
	if err != nil {
		log.Println("[ERROR] Error connecting to kafka")
		return nil, err
	}

	client := &Client{
		client:      c,
		config:      config,
		kafkaConfig: kc,
	}

	err = kc.Validate()
	if err != nil {
		return client, err
	}

	client.populateAPIVersions()
	return client, nil
}

func (c *Client) populateAPIVersions() {
	log.Printf("[DEBUG] retrieving supported APIs from broker")
	broker, err := c.client.Controller()
	if err != nil {
		log.Printf("[ERROR] Unable to populate supported API versions. Error retrieving controller: %s", err)
		return
	}

	resp, err := broker.ApiVersions(&sarama.ApiVersionsRequest{})
	if err != nil {
		log.Printf("[ERROR] Unable to populate supported API versions. %s", err)
		return
	}

	m := map[int]int{}
	for _, v := range resp.ApiVersions {
		log.Printf("[TRACE] API key %d. Min %d, Max %d", v.ApiKey, v.MinVersion, v.MaxVersion)
		m[int(v.ApiKey)] = int(v.MaxVersion)
	}
	c.supportedAPIs = m
}

func (c *Client) DeleteTopic(t string) error {
	broker, err := c.client.Controller()
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
	broker, err := c.client.Controller()
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
	broker, err := c.client.Controller()
	if err != nil {
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
	broker, err := c.client.Controller()
	if err != nil {
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
				partitionCount := int32(len(p))
				log.Printf("[DEBUG] %d Partitions Found: %v from Kafka", partitionCount, p)
				topic.Partitions = partitionCount

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

func (c *Client) getDescribeConfigAPIVersion() int16 {
	return int16(c.versionForKey(32, 1))
}

func (c *Client) versionForKey(apiKey, wantedMaxVersion int) int {
	if maxSupportedVersion, ok := c.supportedAPIs[apiKey]; ok {
		if maxSupportedVersion < wantedMaxVersion {
			return maxSupportedVersion
		}
		return wantedMaxVersion
	}

	return 0
}

func (c *Client) topicConfig(topic string) (map[string]*string, error) {
	conf := map[string]*string{}
	request := &sarama.DescribeConfigsRequest{
		Version: c.getDescribeConfigAPIVersion(),
		Resources: []*sarama.ConfigResource{
			{
				Type: sarama.TopicResource,
				Name: topic,
			},
		},
	}

	broker, err := c.client.Controller()
	if err != nil {
		return conf, err
	}

	cr, err := broker.DescribeConfigs(request)
	if err != nil {
		return conf, err
	}

	if len(cr.Resources) > 0 && len(cr.Resources[0].Configs) > 0 {
		for _, tConf := range cr.Resources[0].Configs {
			v := tConf.Value
			log.Printf("[INFO] Topic: %s. %s: %v. Default %v, Source %v, Version %d", topic, tConf.Name, v, tConf.Default, tConf.Source, cr.Version)
			for _, s := range tConf.Synonyms {
				log.Printf("[INFO] Syonyms: %v", s)
			}

			if isDefault(tConf, int(cr.Version)) {
				continue
			}
			conf[tConf.Name] = &v
		}
	}
	return conf, nil
}
