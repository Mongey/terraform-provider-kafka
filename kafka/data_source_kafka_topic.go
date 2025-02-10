package kafka

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func kafkaTopicDataSource() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTopicRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the topic.",
			},
			"partitions": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of partitions.",
			},
			"replication_factor": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of replicas.",
			},
			"config": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "A map of string k/v attributes.",
				Elem:        schema.TypeString,
			},
		},
	}
}

func dataSourceTopicRead(d *schema.ResourceData, meta interface{}) error {
	// Unlike the resource topicRead, there is no pre-existing ID. We must use the 'name' to look up the resource.
	// See https://learn.hashicorp.com/tutorials/terraform/provider-create?in=terraform/providers#implement-read
	name := d.Get("name").(string)

	client := meta.(*LazyClient)
	topic, err := client.ReadTopic(name, true)
	if err != nil {
		log.Printf("[ERROR] Error getting topic %s from Kafka: %s", name, err)
		_, ok := err.(TopicMissingError)

		if ok {
			return fmt.Errorf("could not find topic '%s'", name)
		}

		return err
	}

	log.Printf("[DEBUG] Setting the state from Kafka %v", topic)
	errSet := errSetter{d: d}
	errSet.Set("name", topic.Name)
	errSet.Set("partitions", topic.Partitions)
	errSet.Set("replication_factor", topic.ReplicationFactor)
	errSet.Set("config", topic.Config)

	// Set the id to the name
	d.SetId(name)
	return errSet.err
}
