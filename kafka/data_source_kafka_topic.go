package kafka

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func kafkaTopicDataSource() *schema.Resource {
	return &schema.Resource{
		Read:   topicRead,
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


