package kafka

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	samara "github.com/Shopify/sarama"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/samuel/go-zookeeper/zk"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"address": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_BROKERS", nil),
				Description: "URL of the root of the target Vault server.",
			},
		},

		ConfigureFunc: providerConfigure,
		ResourcesMap: map[string]*schema.Resource{
			"kafka_topic": kafkaTopicResource(),
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := &KafkaConfig{
		KafkaAddress: "localhost:9092",
	}

	client := NewClient(config)

	return client, nil
}

type KafkaConfig struct {
	KafkaAddress string
	Timeout      int
}

type KafkaClient struct {
	client samara.Client
}

type Broker struct {
	Id        string
	JMXPort   int    `json:"jmx_port"`
	Timestamp int    `json:"timestamp"`
	Host      string `json:"host"`
	Version   int    `json:"int"`
	Port      int    `json:"port"`
}

func NewClient(config *KafkaConfig) *KafkaClient {
	kafkaConfig := samara.NewConfig()
	kafkaConfig.Version = samara.V0_11_0_0
	c, err := samara.NewClient([]string{config.KafkaAddress}, kafkaConfig)
	f, err := os.OpenFile("test.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		panic(err)
	}

	// don't forget to close it
	defer f.Close()
	logger := log.New(f, "[DEBUG] ", log.LstdFlags)
	samara.Logger = logger
	logger.Println("should be ok")

	if err != nil {
		fmt.Println("Error connecting to kafka")
		panic(err)
	}

	return &KafkaClient{
		client: c,
	}
}

func Brokers(c *zk.Conn) ([]Broker, error) {
	brokers := []Broker{}
	brokerIds, _, err := c.Children("/brokers/ids")

	if err != nil {
		fmt.Println("aaa")
		return brokers, err
	}

	for _, brokerId := range brokerIds {
		broker := Broker{
			Id: brokerId,
		}
		broker, _ = Get(c, broker)
		brokers = append(brokers, broker)
	}

	return brokers, nil
}

func Get(c *zk.Conn, b Broker) (Broker, error) {
	brokerInfo, _, err := c.Get("/brokers/ids/" + b.Id)

	if err != nil {
		return b, err
	}

	err = json.Unmarshal(brokerInfo, &b)

	return b, err
}
