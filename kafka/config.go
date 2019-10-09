package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"log"

	"github.com/Shopify/sarama"
)

type Config struct {
	BootstrapServers *[]string
	Timeout          int
	CACert           string
	ClientCert       string
	ClientCertKey    string
	TLSEnabled       bool
	SkipTLSVerify    bool
	SASLUsername     string
	SASLPassword     string
	SASLMechanism    string
}

func (c *Config) newKafkaConfig() (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_1_0_0
	kafkaConfig.ClientID = "terraform-provider-kafka"

	if c.saslEnabled() {
		switch c.SASLMechanism {
		case "scram-sha512":
			kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA512)
		case "scram-sha256":
			kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA256)
		case "plain":
		default:
			log.Fatalf("[ERROR] Invalid sasl mechanism \"%s\": can only be \"scram-sha256\", \"scram-sha512\" or \"plain\"", c.SASLMechanism)
		}
		kafkaConfig.Net.SASL.Enable = true
		kafkaConfig.Net.SASL.Password = c.SASLPassword
		kafkaConfig.Net.SASL.User = c.SASLUsername
	}

	if c.TLSEnabled {
		tlsConfig, err := newTLSConfig(
			c.ClientCert,
			c.ClientCertKey,
			c.CACert)

		if err != nil {
			return kafkaConfig, err
		}

		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = tlsConfig
		kafkaConfig.Net.TLS.Config.InsecureSkipVerify = c.SkipTLSVerify
	}

	return kafkaConfig, nil
}

func (c *Config) saslEnabled() bool {
	return c.SASLUsername != "" || c.SASLPassword != ""
}

func newTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	tlsConfig := tls.Config{}

	// Load client cert
	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			// try from file
			cert, err = tls.LoadX509KeyPair(clientCert, clientKey)
			if err != nil {
				log.Fatalf("[ERROR] Error creating client pair")
				return &tlsConfig, err
			}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		log.Println("[WARN] skipping TLS client config")
	}

	if caCert == "" {
		log.Println("[WARN] no CA file set skipping")
		return &tlsConfig, nil
	}

	caCertPool := x509.NewCertPool()
	caPEM, _ := pem.Decode([]byte(caCert))
	if caPEM == nil {
		// try as file
		caCert, err := ioutil.ReadFile(caCert)
		if err != nil {
			log.Fatalf("[ERROR] unable to read CA")
			return &tlsConfig, err
		}
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		caCertPool.AppendCertsFromPEM(caPEM.Bytes)
	}

	tlsConfig.RootCAs = caCertPool
	tlsConfig.BuildNameToCertificate()
	return &tlsConfig, nil
}
