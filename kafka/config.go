package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"time"

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
	kafkaConfig.Admin.Timeout = time.Duration(c.Timeout) * time.Second

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
		kafkaConfig.Net.SASL.Handshake = true
	} else {
		log.Printf("[WARN] SASL disabled username: '%s', password '%s'", c.SASLUsername, c.SASLPassword)
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

func NewTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	return newTLSConfig(clientCert, clientKey, caCert)
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
				log.Printf("[ERROR] Error creating client pair \ncert:\n%s\n key\n%s\n", clientCert, clientKey)
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

	caCertPool, _ := x509.SystemCertPool()
	if caCertPool == nil {
		caCertPool = x509.NewCertPool()
	}
	caPEM, _ := pem.Decode([]byte(caCert))
	log.Println("[INFO] adding rootybou")
	if caPEM == nil {
		log.Println("[WARN] no caPem, checking from file")
		// try as file
		caCert, err := ioutil.ReadFile(caCert)
		if err != nil {
			log.Println("[ERROR] unable to read CA")
			return &tlsConfig, err
		}
		log.Println("[WARN] Adding pem from file")
		caCertPool.AppendCertsFromPEM(caCert)
	} else {
		ok := caCertPool.AppendCertsFromPEM([]byte(caCert))
		fmt.Printf("set cert pool %v", ok)
		if !ok {
			return &tlsConfig, fmt.Errorf("Couldn't add the caPem")
		}
	}

	tlsConfig.RootCAs = caCertPool
	tlsConfig.BuildNameToCertificate()
	return &tlsConfig, nil
}
