package kafka

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func kafkaTopicsDataSource() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTopicsRead,
		Schema: map[string]*schema.Schema{
			"topics": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "A map of topic names.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						// "name": {
						// 	Type:        schema.TypeString,
						// 	Required:    true,
						// 	Description: "A topic name",
						// },
					},
				},
			},
		},
	}
}

func dataSourceTopicsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*LazyClient)
	topicsList, err := client.ReadTopicsList(true)

	if err != nil {
		log.Printf("[ERROR] Error getting list of topics from Kafka: %s", err)
		return err
	}

	log.Printf("[DEBUG] Setting the state from Kafka")
	errSet := errSetter{d: d}
	fmt.Println(topicsList)

	t := make([]interface{}, 2)

	errSet.Set("topics", t)

	return errSet.err
}
