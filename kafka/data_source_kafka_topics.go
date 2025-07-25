package kafka

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func kafkaTopicsDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTopicsRead,
		Description: "Provides a list of all Kafka topics in the cluster.",
		Schema: map[string]*schema.Schema{
			"list": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "A list containing all the topics.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topic_name": {
							Type:        schema.TypeString,
							Computed:    true,
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
				},
			},
		},
	}
}

func dataSourceTopicsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*LazyClient)
	topicList, err := client.GetKafkaTopics()
	if err != nil {
		return diag.FromErr(err)
	}

	topics := flattenTopicsData(&topicList)
	if err := d.Set("list", topics); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprint(len(topics)))
	return diags
}

func flattenTopicsData(topicList *[]Topic) []any {
	if topicList == nil {
		return make([]any, 0)
	}
	
	topics := make([]any, 0, len(*topicList))
	for _, topic := range *topicList {
		topics = append(topics, map[string]any{
			"topic_name":         topic.Name,
			"replication_factor": topic.ReplicationFactor,
			"partitions":         topic.Partitions,
			"config":             topic.Config,
		})
	}
	return topics
}
