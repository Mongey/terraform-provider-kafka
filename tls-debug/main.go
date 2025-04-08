package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Mongey/terraform-provider-kafka/kafka"
	"github.com/IBM/sarama"
)

func main() {
	err := fauxMain()
	if err != nil {
		log.Fatal(err)
	}
}

func client(broker, caLocation, clientCertLocation, clientKeyLocation string) (*sarama.Client, error) {
	brokers := []string{broker}
	caCert, err := os.ReadFile(caLocation)
	if err != nil {
		return nil, err
	}

	clientCert, err := os.ReadFile(clientCertLocation)
	if err != nil {
		return nil, err
	}

	clientKey, err := os.ReadFile(clientKeyLocation)
	if err != nil {
		return nil, err
	}

	config := &kafka.Config{
		BootstrapServers: &brokers,
		CACert:           string(caCert),
		ClientCert:       string(clientCert),
		ClientCertKey:    string(clientKey),
		SkipTLSVerify:    false,
		TLSEnabled:       true,
		Timeout:          100,
	}

	client, err := kafka.NewClient(config)
	if err != nil {
		return nil, err
	}

	c := client.SaramaClient()

	return &c, nil
}

func fauxMain() error {
	testKafka := flag.Bool("kafka-tls", false, "test-kafka")

	cert := flag.String("cert", "client.cert", "location of the cert")
	key := flag.String("key", "private.key", "location of the key")
	ca := flag.String("ca", "ca.cert", "location of the ca")
	broker := flag.String("broker", "localhost:9092", "location of the broker")

	flag.Parse()

	if *testKafka {
		client, err := client(*broker,
			*ca,
			*cert,
			*key,
		)

		if err != nil {
			return err
		}

		err = (*client).RefreshMetadata()
		if err != nil {
			return err
		}
	} else {
		caCert, err := os.ReadFile(*ca)
		if err != nil {
			return err
		}

		clientCert, err := os.ReadFile(*cert)
		if err != nil {
			return err
		}

		clientKey, err := os.ReadFile(*key)
		if err != nil {
			return err
		}
		tlsConf, err := kafka.NewTLSConfig(string(clientCert), string(clientKey), string(caCert), "test-pass")
		if err != nil {
			return err
		}
		c, err := tls.Dial("tcp", *broker, tlsConf)

		if serr, ok := err.(x509.CertificateInvalidError); ok {
			fmt.Printf("2rrrroorr %v, %d\n", serr.Cert.PermittedDNSDomains, serr.Reason)
		}
		if err != nil {
			return err
		}

		return c.Handshake()
	}

	return nil
}
