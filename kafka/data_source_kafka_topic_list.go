package kafka

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"time"
)

func kafkaTopicListDataSource() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTopicListRead,
		Schema: map[string]*schema.Schema{
			"list": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "a list of kafka topics in the Kafka cluster",
				Elem: 		 &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func dataSourceTopicListRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*LazyClient)
	topicList, err := client.GetKafkaTopicList()
	if err != nil {
		return err
	}
	err = d.Set("list", topicList)
	if err != nil {
		return err
	}
	d.SetId(time.Now().UTC().String())
	return nil
}
