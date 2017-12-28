package kafka

import (
	"errors"
	"fmt"
	"log"
	"time"

	samara "github.com/Shopify/sarama"
	"github.com/hashicorp/terraform/helper/schema"
)

func kafkaTopicResource() *schema.Resource {
	return &schema.Resource{
		Create: topicCreate,
		Delete: topicDelete,
		Read:   topicRead,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the topic",
			},
			"partitions": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "number of partitions",
			},
			"replication_factor": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "number of replicas",
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
	t := metaToTopicConfig(d, meta)
	c := meta.(*Client)
	broker, err := AvailableBrokerFromList(c.config.Brokers)

	if err != nil {
		return err
	}

	req := &samara.CreateTopicsRequest{
		TopicDetails: map[string]*samara.TopicDetail{
			t.Name: {
				NumPartitions:     t.Partitions,
				ReplicationFactor: t.ReplicationFactor,
				ConfigEntries:     t.Config,
			},
		},
		Timeout: 1000 * time.Millisecond,
	}
	res, err := broker.CreateTopics(req)

	if err == nil {
		for _, e := range res.TopicErrors {
			if e.Err != samara.ErrNoError {
				return fmt.Errorf("%s", e.Err)
			}
		}
		log.Printf("[INFO] Created topic %s in Kafka", t.Name)
		d.SetId(t.Name)
	}
	return err
}

func topicUpdate(d *schema.ResourceData, meta interface{}) error {
	return errors.New("Updates NYI")
}

func topicDelete(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*Client)
	t := metaToTopicConfig(d, meta)

	broker, err := AvailableBrokerFromList(c.config.Brokers)

	if err != nil {
		return err
	}

	req := &samara.DeleteTopicsRequest{
		Topics:  []string{t.Name},
		Timeout: 1000 * time.Millisecond,
	}
	res, err := broker.DeleteTopics(req)

	if err == nil {
		for k, e := range res.TopicErrorCodes {
			if e != samara.ErrNoError {
				return fmt.Errorf("%s : %s", k, e)
			}
		}
	} else {
		log.Printf("[ERROR] Error deleting topic %s from Kafka", err)
		return err
	}

	log.Printf("[INFO] Deleted topic %s from Kafka", t.Name)

	// delete from state
	d.SetId("")
	return err
}

func topicRead(d *schema.ResourceData, meta interface{}) error {
	name := d.Id()
	client := meta.(*Client)
	c := client.client
	topics, err := c.Topics()

	if err != nil {
		log.Printf("[ERROR] Error getting topics %s from Kafka", err)
		return err
	}

	for _, t := range topics {
		log.Printf("[DEBUG] Reading Topic %s from Kafka", t)
		if name == t {
			log.Printf("[DEBUG] FOUND %s from Kafka", t)
			d.Set("name", t)
			p, err := c.Partitions(t)
			if err == nil {
				log.Printf("[DEBUG] Partitions %v from Kafka", p)
				d.Set("partitions", len(p))

				r, err := ReplicaCount(c, name, p)
				if err == nil {
					log.Printf("[DEBUG] ReplicationFactor %d from Kafka", r)
					d.Set("replication_factor", r)
				}
				configToSave, err := ConfigForTopic(t, client.config.Brokers)
				if err != nil {
					log.Printf("[ERROR] Could not get config for topic %s: %s", t, err)
					return err
				}

				d.Set("config", configToSave)
			}

			return nil
		}
	}

	d.SetId("")
	return nil
}
