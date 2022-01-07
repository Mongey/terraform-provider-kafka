package kafka

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"time"
)

func kafkaTopicsDataSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTopicsRead,
		Schema: map[string]*schema.Schema{
			"list": &schema.Schema{
				Type:        schema.TypeList,
				Computed:    true,
				Description: "A list containing all the topics.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topic_name": &schema.Schema{
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the topic.",
						},
						"partitions": &schema.Schema{
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of partitions.",
						},
						"replication_factor": &schema.Schema{
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas.",
						},
						"config": &schema.Schema{
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
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(time.Now().UTC().String())
	return diags
}

func flattenTopicsData(topicList *[]Topic) []interface{} {
	if topicList != nil {
		topics := make([]interface{}, len(*topicList), len(*topicList))
		for i, topic := range *topicList {
			topicObj := make(map[string]interface{})
			topicObj["topic_name"] = topic.Name
			topicObj["replication_factor"] = topic.ReplicationFactor
			topicObj["partitions"] = topic.Partitions
			topicObj["config"] = topic.Config
			topics[i] = topicObj
		}
		return topics
	}
	return make([]interface{}, 0)
}
