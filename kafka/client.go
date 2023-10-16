package kafka

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

type TopicMissingError struct {
	msg string
}

func (e TopicMissingError) Error() string { return e.msg }

type void struct{}

var member void

type Client struct {
	client        sarama.Client
	kafkaConfig   *sarama.Config
	config        *Config
	supportedAPIs map[int]int
	topics        map[string]void
	topicsMutex   sync.RWMutex
}

func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("Cannot create client without kafka config")
	}

	log.Printf("[TRACE] configuring bootstrap_servers %v", config.copyWithMaskedSensitiveValues())
	if config.BootstrapServers == nil {
		return nil, fmt.Errorf("No bootstrap_servers provided")
	}

	bootstrapServers := *(config.BootstrapServers)
	if bootstrapServers == nil {
		return nil, fmt.Errorf("No bootstrap_servers provided")
	}

	log.Printf("[INFO] configuring kafka client with %v", config.copyWithMaskedSensitiveValues())

	kc, err := config.newKafkaConfig()
	if err != nil {
		log.Printf("[ERROR] Error creating kafka client %v", err)
		return nil, err
	}

	// warning: at this point sarama will attempt to connect to the kafka cluster

	maxRetry := 5

	var c sarama.Client
	for attempt := 0; attempt < maxRetry; attempt++ {
		c, err = sarama.NewClient(bootstrapServers, kc)
		if err != nil {
			log.Printf("[ERROR] [%d/%d] Error connecting to kafka %s", attempt+1, maxRetry, err)
			if attempt >= maxRetry-1 {
				return nil, err
			}

			time.Sleep(5 * time.Second)
		}
	}

	client := &Client{
		client:      c,
		config:      config,
		kafkaConfig: kc,
	}

	err = client.populateAPIVersions()
	if err != nil {
		return client, err
	}

	err = client.extractTopics()

	return client, err
}

func (c *Client) SaramaClient() sarama.Client {
	return c.client
}

func (c *Client) populateAPIVersions() error {
	ch := make(chan []sarama.ApiVersionsResponseKey)
	errCh := make(chan error)

	brokers := c.client.Brokers()
	kafkaConfig := c.kafkaConfig
	for _, broker := range brokers {
		go apiVersionsFromBroker(broker, kafkaConfig, ch, errCh)
	}

	clusterApiVersions := make(map[int][2]int) // valid api version intervals across all brokers
	errs := make([]error, 0)
	for i := 0; i < len(brokers); i++ {
		select {
		case brokerApiVersions := <-ch:
			updateClusterApiVersions(&clusterApiVersions, brokerApiVersions)
		case err := <-errCh:
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.New(sarama.MultiErrorFormat(errs))
	}

	c.supportedAPIs = make(map[int]int, len(clusterApiVersions))
	for apiKey, versionMinMax := range clusterApiVersions {
		versionMin := versionMinMax[0]
		versionMax := versionMinMax[1]

		if versionMax >= versionMin {
			c.supportedAPIs[apiKey] = versionMax
		}

		// versionMax will be less than versionMin only when
		// two or more brokers have disjoint version
		// intervals...which means the api is not supported
		// cluster-wide
	}

	return nil
}

func apiVersionsFromBroker(broker *sarama.Broker, config *sarama.Config, ch chan<- []sarama.ApiVersionsResponseKey, errCh chan<- error) {
	resp, err := rawApiVersionsRequest(broker, config)

	if err != nil {
		errCh <- err
	} else if sarama.KError(resp.ErrorCode) != sarama.ErrNoError {
		errCh <- errors.New(sarama.KError(resp.ErrorCode).Error())
	} else {
		ch <- resp.ApiKeys
	}
}

func rawApiVersionsRequest(broker *sarama.Broker, config *sarama.Config) (*sarama.ApiVersionsResponse, error) {
	if err := broker.Open(config); err != nil && err != sarama.ErrAlreadyConnected {
		return nil, err
	}

	defer func() {
		if err := broker.Close(); err != nil && err != sarama.ErrNotConnected {
			log.Fatal(err)
		}
	}()

	return broker.ApiVersions(&sarama.ApiVersionsRequest{})
}

func updateClusterApiVersions(clusterApiVersions *map[int][2]int, brokerApiVersions []sarama.ApiVersionsResponseKey) {
	cluster := *clusterApiVersions

	for _, apiBlock := range brokerApiVersions {
		apiKey := int(apiBlock.ApiKey)
		brokerMin := int(apiBlock.MinVersion)
		brokerMax := int(apiBlock.MaxVersion)

		clusterMinMax, exists := cluster[apiKey]
		if !exists {
			cluster[apiKey] = [2]int{brokerMin, brokerMax}
		} else {
			// shrink the cluster interval according to
			// the broker interval

			clusterMin := clusterMinMax[0]
			clusterMax := clusterMinMax[1]

			if brokerMin > clusterMin {
				clusterMinMax[0] = brokerMin
			}

			if brokerMax < clusterMax {
				clusterMinMax[1] = brokerMax
			}

			cluster[apiKey] = clusterMinMax
		}
	}
}

func (c *Client) extractTopics() error {
	topics, err := c.client.Topics()
	if err != nil {
		log.Printf("[ERROR] Error getting topics %s from Kafka", err)
		return err
	}
	log.Printf("[DEBUG] Got %d topics from Kafka", len(topics))
	c.topicsMutex.Lock()
	c.topics = make(map[string]void)
	for _, t := range topics {
		c.topics[t] = member
	}
	c.topicsMutex.Unlock()
	return nil
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
		log.Printf("[ERROR] Error deleting topic %s from Kafka: %s", t, err)
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
	log.Printf("[TRACE] Timeout is %v ", timeout)

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
	tp := map[string]*sarama.TopicPartition{
		t.Name: &sarama.TopicPartition{
			Count: t.Partitions,
		},
	}

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

func (c *Client) CanAlterReplicationFactor() bool {
	_, ok1 := c.supportedAPIs[45] // https://kafka.apache.org/protocol#The_Messages_AlterPartitionReassignments
	_, ok2 := c.supportedAPIs[46] // https://kafka.apache.org/protocol#The_Messages_ListPartitionReassignments

	return ok1 && ok2
}

func (c *Client) AlterReplicationFactor(t Topic) error {
	log.Printf("[DEBUG] Refreshing metadata for topic '%s'", t.Name)
	if err := c.client.RefreshMetadata(t.Name); err != nil {
		return err
	}

	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return err
	}

	assignment, err := c.buildAssignment(t)
	if err != nil {
		return err
	}

	return admin.AlterPartitionReassignments(t.Name, *assignment)
}

func (c *Client) buildAssignment(t Topic) (*[][]int32, error) {
	partitions, err := c.client.Partitions(t.Name)
	if err != nil {
		return nil, err
	}

	allReplicas := c.allReplicas()
	newRF := t.ReplicationFactor
	rand.Seed(time.Now().UnixNano())

	assignment := make([][]int32, len(partitions))
	for _, p := range partitions {
		oldReplicas, err := c.client.Replicas(t.Name, p)
		if err != nil {
			return &assignment, err
		}

		oldRF := int16(len(oldReplicas))
		deltaRF := newRF - oldRF
		newReplicas, err := buildNewReplicas(allReplicas, &oldReplicas, deltaRF)
		if err != nil {
			return &assignment, err
		}

		assignment[p] = *newReplicas
	}

	return &assignment, nil
}

func (c *Client) allReplicas() *[]int32 {
	brokers := c.client.Brokers()
	replicas := make([]int32, 0, len(brokers))

	for _, b := range brokers {
		id := b.ID()
		if id != -1 {
			replicas = append(replicas, id)
		}
	}

	return &replicas
}

func buildNewReplicas(allReplicas *[]int32, usedReplicas *[]int32, deltaRF int16) (*[]int32, error) {
	usedCount := int16(len(*usedReplicas))

	if deltaRF == 0 {
		return usedReplicas, nil
	} else if deltaRF < 0 {
		end := usedCount + deltaRF
		if end < 1 {
			return nil, errors.New("dropping too many replicas")
		}

		head := (*usedReplicas)[:end]
		return &head, nil
	} else {
		extraCount := int16(len(*allReplicas)) - usedCount
		if extraCount < deltaRF {
			return nil, errors.New("not enough brokers")
		}

		unusedReplicas := *findUnusedReplicas(allReplicas, usedReplicas, extraCount)
		newReplicas := *usedReplicas
		for i := int16(0); i < deltaRF; i++ {
			j := rand.Intn(len(unusedReplicas))
			newReplicas = append(newReplicas, unusedReplicas[j])
			unusedReplicas[j] = unusedReplicas[len(unusedReplicas)-1]
			unusedReplicas = unusedReplicas[:len(unusedReplicas)-1]
		}

		return &newReplicas, nil
	}
}

func findUnusedReplicas(allReplicas *[]int32, usedReplicas *[]int32, extraCount int16) *[]int32 {
	usedMap := make(map[int32]bool, len(*usedReplicas))
	for _, r := range *usedReplicas {
		usedMap[r] = true
	}

	unusedReplicas := make([]int32, 0, extraCount)
	for _, r := range *allReplicas {
		_, exists := usedMap[r]
		if !exists {
			unusedReplicas = append(unusedReplicas, r)
		}
	}

	return &unusedReplicas
}

func (c *Client) IsReplicationFactorUpdating(topic string) (bool, error) {
	log.Printf("[DEBUG] Refreshing metadata for topic '%s'", topic)
	if err := c.client.RefreshMetadata(topic); err != nil {
		return false, err
	}

	partitions, err := c.client.Partitions(topic)
	if err != nil {
		return false, err
	}

	admin, err := sarama.NewClusterAdminFromClient(c.client)
	if err != nil {
		return false, err
	}

	statusMap, err := admin.ListPartitionReassignments(topic, partitions)
	if err != nil {
		return false, err
	}

	for _, status := range statusMap[topic] {
		if isPartitionRFChanging(status) {
			return true, nil
		}
	}

	return false, nil
}

func isPartitionRFChanging(status *sarama.PartitionReplicaReassignmentsStatus) bool {
	return len(status.AddingReplicas) != 0 || len(status.RemovingReplicas) != 0
}

func (client *Client) ReadTopic(name string, refreshMetadata bool) (Topic, error) {
	c := client.client
	log.Printf("[INFO] 👋 reading topic '%s' from Kafka: %v", name, refreshMetadata)

	topic := Topic{
		Name: name,
	}

	if refreshMetadata {
		log.Printf("[DEBUG] Refreshing metadata for topic '%s'", name)
		err := c.RefreshMetadata(name)

		if err == sarama.ErrUnknownTopicOrPartition {
			err := TopicMissingError{msg: fmt.Sprintf("%s could not be found", name)}
			return topic, err
		}
		if err != nil {
			log.Printf("[ERROR] Error refreshing topic '%s' metadata %s", name, err)
			return topic, err
		}
		err = client.extractTopics()
		if err != nil {
			return topic, err
		}
	} else {
		log.Printf("[DEBUG] skipping metadata refresh for topic '%s'", name)
	}

	client.topicsMutex.RLock()
	_, topicExists := client.topics[name]
	client.topicsMutex.RUnlock()
	if topicExists {
		log.Printf("[DEBUG] Found %s from Kafka", name)
		p, err := c.Partitions(name)
		if err == nil {
			partitionCount := int32(len(p))
			log.Printf("[DEBUG] [%s] %d Partitions Found: %v from Kafka", name, partitionCount, p)
			topic.Partitions = partitionCount

			r, err := ReplicaCount(c, name, p)
			if err != nil {
				return topic, err
			}

			log.Printf("[DEBUG] [%s] ReplicationFactor %d from Kafka", name, r)
			topic.ReplicationFactor = int16(r)

			configToSave, err := client.topicConfig(name)
			if err != nil {
				log.Printf("[ERROR] [%s] Could not get config for topic %s", name, err)
				return topic, err
			}

			log.Printf("[TRACE] [%s] Config %v from Kafka", name, strPtrMapToStrMap(configToSave))
			topic.Config = configToSave
			return topic, nil
		}
	}

	err := TopicMissingError{msg: fmt.Sprintf("%s could not be found", name)}
	return topic, err
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

//topicConfig retrives the non-default config map for a topic
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
			log.Printf("[TRACE] [%s] %s: %v. Default %v, Source %v, Version %d", topic, tConf.Name, v, tConf.Default, tConf.Source, cr.Version)

			for _, s := range tConf.Synonyms {
				log.Printf("[TRACE] Syonyms: %v", s)
			}

			if isDefault(tConf, int(cr.Version)) {
				continue
			}
			conf[tConf.Name] = &v
		}
	}
	return conf, nil
}

func (c *Client) getDescribeAclsRequestAPIVersion() int16 {
	return int16(c.versionForKey(29, 1))
}
func (c *Client) getCreateAclsRequestAPIVersion() int16 {
	return int16(c.versionForKey(30, 1))
}

func (c *Client) getDeleteAclsRequestAPIVersion() int16 {
	return int16(c.versionForKey(31, 1))
}

func (c *Client) getDescribeConfigAPIVersion() int16 {
	return int16(c.versionForKey(32, 1))
}
