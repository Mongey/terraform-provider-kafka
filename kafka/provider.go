package kafka

import (
	"fmt"

	samara "github.com/Shopify/sarama"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

// Provider does stuff
//
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"brokers": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_BROKERS", []string{}),
				Description: "A list of kafka brokers",
			},
		},

		ConfigureFunc: providerConfigure,
		ResourcesMap: map[string]*schema.Resource{
			"kafka_topic": kafkaTopicResource(),
		},
	}
}

// Config is the config
type Config struct {
	Brokers []string
	Timeout int
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	iBrokers := d.Get("brokers").([]interface{})
	brokers := make([]string, 0, len(iBrokers))
	for _, iBrokers := range iBrokers {
		brokers = append(brokers, iBrokers.(string))
	}

	config := &Config{
		Brokers: brokers,
	}

	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	return client, client.client.Config().Validate()
}

// Client is ok
type Client struct {
	client samara.Client
	config *Config
}

// NewClient is
func NewClient(config *Config) (*Client, error) {
	kafkaConfig := samara.NewConfig()
	kafkaConfig.Version = samara.V0_11_0_0
	c, err := samara.NewClient(config.Brokers, kafkaConfig)

	if err != nil {
		fmt.Println("Error connecting to kafka")
		return nil, err
	}

	return &Client{
		client: c,
		config: config,
	}, kafkaConfig.Validate()
}
