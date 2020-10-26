package kafka

import (
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func kafkaTopicConfigDataSource() *schema.Resource {
	return &schema.Resource{
		Read:   topicConfigRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the topic.",
			},
			"partitions": {
				Type:         schema.TypeInt,
				Computed:     true,
				Description:  "Number of partitions.",
			},
			"replication_factor": {
				Type:         schema.TypeInt,
				Computed:     true,
				Description:  "Number of replicas.",
			},
			"config": {
				Type:        schema.TypeMap,
				Computed:     true,
				Description: "A map of string k/v attributes.",
				Elem:        schema.TypeString,
			},
		},
	}
}

func topicConfigRead(d *schema.ResourceData, meta interface{}) error {
	name := d.Get("name").(string)
	client := meta.(*LazyClient)
	topic, err := client.ReadTopic(name)

	if err != nil {
		log.Printf("[ERROR] Error getting topics %s from Kafka", err)
		_, ok := err.(TopicMissingError)
		if ok {
			d.SetId("")
			return nil
		}
		return err
	}

	log.Printf("[DEBUG] Setting the state from Kafka %v", topic)
	d.SetId(name)
	d.Set("partitions", topic.Partitions)
	d.Set("replication_factor", topic.ReplicationFactor)
	d.Set("config", topic.Config)

	return nil
}

