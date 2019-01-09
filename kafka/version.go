package kafka

import (
	"log"

	"github.com/Shopify/sarama"
)

func versionFromString(version string) sarama.KafkaVersion {
	defaultVersion := sarama.V1_0_0_0

	knownVersions := map[string]sarama.KafkaVersion{
		"1.0.0": sarama.V1_0_0_0,
		"1.1.0": sarama.V1_1_0_0,
		"1.1.1": sarama.V1_1_1_0,
		"2.0.0": sarama.V2_0_0_0,
		"2.0.1": sarama.V2_0_1_0,
		"2.1.0": sarama.V2_1_0_0,
	}

	if v, ok := knownVersions[version]; ok {
		return v
	}

	log.Printf("[WARN] Unable to convert %s to sarama version; using default %s", version, defaultVersion)
	return defaultVersion
}
