package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Shopify/sarama"
)

type Config struct {
	BootstrapServers        *[]string
	Timeout                 int
	CACert                  string
	ClientCert              string
	ClientCertKey           string
	ClientCertKeyPassphrase string
	TLSEnabled              bool
	SkipTLSVerify           bool
	SASLUsername            string
	SASLPassword            string
	SASLMechanism           string
}

func (c *Config) newKafkaConfig() (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_7_0_0
	kafkaConfig.ClientID = "terraform-provider-kafka"
	kafkaConfig.Admin.Timeout = time.Duration(c.Timeout) * time.Second
	kafkaConfig.Metadata.Full = true // the default, but just being clear
	kafkaConfig.Metadata.AllowAutoTopicCreation = false

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
		log.Printf("[WARN] SASL disabled username: '%s', password '%s'", c.SASLUsername, "****")
	}

	if c.TLSEnabled {
		tlsConfig, err := newTLSConfig(
			c.ClientCert,
			c.ClientCertKey,
			c.CACert,
			c.ClientCertKeyPassphrase,
		)

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

func NewTLSConfig(clientCert, clientKey, caCert, clientKeyPassphrase string) (*tls.Config, error) {
	return newTLSConfig(clientCert, clientKey, caCert, clientKeyPassphrase)
}

func parsePemOrLoadFromFile(input string) (*pem.Block, []byte, error) {
	// attempt to parse
	var inputBytes = []byte(input)
	inputBlock, _ := pem.Decode(inputBytes)

	if inputBlock == nil {
		//attempt to load from file
		log.Printf("[INFO] Attempting to load from file '%s'", input)
		var err error
		inputBytes, err = os.ReadFile(input)
		if err != nil {
			return nil, nil, err
		}
		inputBlock, _ = pem.Decode(inputBytes)
		if inputBlock == nil {
			return nil, nil, fmt.Errorf("[ERROR] Error unable to decode pem")
		}
	}
	return inputBlock, inputBytes, nil
}

func newTLSConfig(clientCert, clientKey, caCert, clientKeyPassphrase string) (*tls.Config, error) {
	tlsConfig := tls.Config{}

	if clientCert != "" && clientKey != "" {
		_, certBytes, err := parsePemOrLoadFromFile(clientCert)
		if err != nil {
			log.Printf("[ERROR] Unable to read certificate %s", err)
			return &tlsConfig, err
		}

		keyBlock, keyBytes, err := parsePemOrLoadFromFile(clientKey)
		if err != nil {
			log.Printf("[ERROR] Unable to read private key %s", err)
			return &tlsConfig, err
		}

		if x509.IsEncryptedPEMBlock(keyBlock) { //nolint:staticcheck
			log.Printf("[INFO] Using encrypted private key")
			var err error

			keyBytes, err = x509.DecryptPEMBlock(keyBlock, []byte(clientKeyPassphrase)) //nolint:staticcheck
			if err != nil {
				log.Printf("[ERROR] Error decrypting private key with passphrase %s", err)
				return &tlsConfig, err
			}
			keyBytes = pem.EncodeToMemory(&pem.Block{
				Type:  keyBlock.Type,
				Bytes: keyBytes,
			})
		}

		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			log.Printf("[ERROR] Error creating X509KeyPair %s", err)
			return &tlsConfig, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if caCert == "" {
		log.Println("[WARN] no CA file set skipping")
		return &tlsConfig, nil
	}

	caCertPool, _ := x509.SystemCertPool()
	if caCertPool == nil {
		caCertPool = x509.NewCertPool()
	}

	_, caBytes, err := parsePemOrLoadFromFile(caCert)
	if err != nil {
		log.Printf("[ERROR] Unable to read CA %s", err)
		return &tlsConfig, err
	}
	ok := caCertPool.AppendCertsFromPEM(caBytes)
	log.Printf("[TRACE] set cert pool %v", ok)
	if !ok {
		return &tlsConfig, fmt.Errorf("Couldn't add the caPem")
	}

	tlsConfig.RootCAs = caCertPool
	return &tlsConfig, nil
}

func (config *Config) copyWithMaskedSensitiveValues() Config {
	copy := Config{
		config.BootstrapServers,
		config.Timeout,
		config.CACert,
		config.ClientCert,
		"*****",
		"*****",
		config.TLSEnabled,
		config.SkipTLSVerify,
		config.SASLUsername,
		"*****",
		config.SASLMechanism,
	}
	return copy
}
