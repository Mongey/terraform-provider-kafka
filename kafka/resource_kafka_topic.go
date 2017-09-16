package kafka

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
)

func kafkaTopicResource() *schema.Resource {
	return &schema.Resource{
		Create: topicCreate,
		Update: topicUpdate,
		Delete: topicDelete,
		Read:   topicRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    false,
				Description: "The name of the topic",
			},
			"partitions": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "number or partitions",
			},
			"replication_factor": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "number or repls",
			},
			"config": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Description: "the config",
			},
		},
	}
}

func topicCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*KafkaClient)
	kcClient := client.client

	timeout := int32(2147483647)
	t := metaToTopicConfig(d, meta)

	err := kcClient.CreateTopic(t.Name, t.Partitions, t.ReplicationFactor, t.Config, timeout)
	if err == nil {
		d.SetId(t.Name)
	}
	return err
}
func topicUpdate(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func topicDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*KafkaClient)
	kcClient := client.client
	timeout := int32(2147483647)
	t := metaToTopicConfig(d, meta)

	err := kcClient.DeleteTopic(t.Name, timeout)

	if err != nil {
		log.Printf("[ERROR] Error Reading %s from Kafka", err)
		return err
	}

	// delete
	d.SetId("")
	return err
}

func topicRead(d *schema.ResourceData, meta interface{}) error {
	name := d.Id()
	log.Printf("[DEBUG] HI Reading %s from Kafka", name)

	client := meta.(*KafkaClient)
	kcClient := client.client

	err := kcClient.RefreshMetadata(name)
	if err != nil {
		log.Printf("hm %s", kcClient.Config().Validate())
		log.Printf("hm %s", kcClient.Brokers())
		log.Printf("hm %s", kcClient.Config())
		log.Printf("hm %s", kcClient.Config().Consumer)
		log.Printf("[ERROR] Error Refreshing from Kafka %s", err)
		return err
	}
	topics, err := kcClient.Topics()

	if err != nil {
		log.Printf("[ERROR] Error Reading %s from Kafka", err)
		return err
	}
	log.Printf("[DEBUG] NO Error Reading %s from Kafka %d", name, len(topics))

	for _, t := range topics {
		log.Printf("[DEBUG] HI Reading %s from Kafka", t)
		log.Printf("[DEBUG] checking if %s == %s", t, name)
		if name == t {
			log.Printf("[INFO] FOUND %s from Kafka", t)
			return nil
		}
	}

	// delete
	//d.SetId("")

	return nil
}

type TopicConfig struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Config            map[string]string
}

func metaToTopicConfig(d *schema.ResourceData, meta interface{}) TopicConfig {
	topicName := d.Get("name").(string)
	partitions := d.Get("partitions").(int)
	replicationFactor := d.Get("replication_factor").(int)
	convertedPartitions := int32(partitions)
	convertedRF := int16(replicationFactor)
	config := d.Get("config").(map[string]interface{})

	m2 := make(map[string]string)
	for key, value := range config {
		switch value := value.(type) {
		case string:
			m2[key] = value
		}
	}

	return TopicConfig{
		Name:              topicName,
		Partitions:        convertedPartitions,
		ReplicationFactor: convertedRF,
		Config:            m2,
	}
}
