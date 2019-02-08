package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/Shopify/sarama"
)

type Config struct {
	BootstrapServers *[]string
	Timeout          int

	CACert         *x509.Certificate
	CACertFile     string
	ClientCert     *tls.Certificate
	ClientCertFile string
	ClientCertKey  string
	TLSEnabled     bool
	SkipTLSVerify  bool
	SASLUsername   string
	SASLPassword   string
}

func (c *Config) saslEnabled() bool {
	return c.SASLUsername != "" || c.SASLPassword != ""
}

func (c *Config) newKafkaConfig() (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_1_0_0
	kafkaConfig.ClientID = "terraform-provider-kafka"

	if c.saslEnabled() {
		kafkaConfig.Net.SASL.Enable = true
		kafkaConfig.Net.SASL.Password = c.SASLPassword
		kafkaConfig.Net.SASL.User = c.SASLUsername
	}

	if c.TLSEnabled {
		tlsConfig, err := c.newTLSConfig()
		if err != nil {
			return kafkaConfig, err
		}

		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = tlsConfig
		kafkaConfig.Net.TLS.Config.InsecureSkipVerify = c.SkipTLSVerify
	}

	return kafkaConfig, nil
}

func (c *Config) newTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	cert, err := c.clientCert()
	if err != nil {
		return tlsConfig, err
	}
	if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	pool, err := c.caCertPool()
	if err != nil {
		return tlsConfig, err
	}
	if pool != nil {
		tlsConfig.RootCAs = pool
	}

	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func (c *Config) clientCert() (*tls.Certificate, error) {
	if c.ClientCert != nil {
		return c.ClientCert, nil
	}
	if c.ClientCertFile != "" && c.ClientCertKey != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientCertKey)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}

	return nil, nil
}

func (c *Config) caCertPool() (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if c.CACert != nil {
		pool.AddCert(c.CACert)
	} else if c.CACertFile == "" {
		caCert, err := ioutil.ReadFile(c.CACertFile)
		if err != nil {
			return nil, err
		}
		pool.AppendCertsFromPEM(caCert)
	}
	return pool, nil
}
