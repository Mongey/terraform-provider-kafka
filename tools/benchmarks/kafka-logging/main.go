// kafka-logging toggles request logging on the disposable benchmark brokers.
package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

func main() {
	bootstrap := flag.String("bootstrap", "", "benchmark bootstrap servers")
	level := flag.String("level", "INFO", "INFO or DEBUG")
	flag.Parse()
	if (*level != "INFO" && *level != "DEBUG") || *bootstrap == "" {
		log.Fatal("provide --bootstrap and --level=INFO or DEBUG")
	}
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_8_0_0
	cfg.ClientID = "terraform-refresh-harness"
	cfg.Metadata.RefreshFrequency = 0
	cfg.Net.ReadTimeout = 30 * time.Second
	c, err := sarama.NewClient(strings.Split(*bootstrap, ","), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if len(c.Brokers()) != 3 {
		log.Fatal("expected three benchmark brokers")
	}
	for _, broker := range c.Brokers() {
		if err := broker.Open(cfg); err != nil && err != sarama.ErrAlreadyConnected {
			log.Fatal(err)
		}
		res, err := broker.IncrementalAlterConfigs(&sarama.IncrementalAlterConfigsRequest{
			Resources: []*sarama.IncrementalAlterConfigsResource{{
				Type: sarama.BrokerLoggerResource, Name: strconv.Itoa(int(broker.ID())),
				ConfigEntries: map[string]sarama.IncrementalAlterConfigsEntry{
					"kafka.request.logger": {Operation: sarama.IncrementalAlterConfigsOperationSet, Value: level},
				},
			}},
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(res.Resources) != 1 || res.Resources[0].ErrorCode != 0 {
			log.Fatalf("broker %d rejected logger change: %+v", broker.ID(), res.Resources)
		}
		got, err := broker.DescribeConfigs(&sarama.DescribeConfigsRequest{Version: 2,
			Resources: []*sarama.ConfigResource{{Type: sarama.BrokerLoggerResource, Name: strconv.Itoa(int(broker.ID()))}},
		})
		if err != nil {
			log.Fatal(err)
		}
		verified := false
		for _, resource := range got.Resources {
			if resource.ErrorCode != 0 {
				log.Fatal(resource.ErrorMsg)
			}
			for _, entry := range resource.Configs {
				if entry.Name == "kafka.request.logger" && entry.Value == *level {
					verified = true
				}
			}
		}
		if !verified {
			log.Fatalf("broker %d logger readback failed", broker.ID())
		}
		fmt.Printf("broker %d request logging=%s verified\n", broker.ID(), *level)
	}
}
