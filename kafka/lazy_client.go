package kafka

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"

	"github.com/IBM/sarama"
)

type LazyClient struct {
	once    sync.Once
	initErr error
	inner   *Client
	Config  *Config
}

func (c *LazyClient) init() error {
	var err error

	c.once.Do(func() {
		c.inner, err = NewClient(c.Config)
		c.initErr = err
	})

	if c.Config != nil {
		log.Printf("[TRACE] lazy client init %s; config, %v", c.initErr, c.Config.copyWithMaskedSensitiveValues())
	} else {
		log.Printf("[TRACE] lazy client init %s", c.initErr)
	}
	if c.initErr == sarama.ErrBrokerNotAvailable || c.initErr == sarama.ErrOutOfBrokers {
		log.Printf("[ERROR] Cannot connect to Kafka broker(s) %v", *(c.Config.BootstrapServers))
		log.Printf("[ERROR] Check if Kafka broker(s) are up and running")
		if c.Config.TLSEnabled {
			log.Printf("[ERROR] Check if Kafka broker(s) are reachable from this machine using TLS")
			tlsError := c.checkTLSConfig()
			if tlsError != nil {
				return fmt.Errorf("%w\n%s", tlsError, c.initErr)
			}
		}
	}

	return c.initErr
}

func (c *LazyClient) checkTLSConfig() error {
	kafkaConfig, err := c.Config.newKafkaConfig()
	if err != nil {
		return err
	}

	brokers := *(c.Config.BootstrapServers)
	errs := make([]error, 0, len(brokers))

	for i := 0; i < len(brokers); i++ {
		broker := brokers[i]
		tlsConf := kafkaConfig.Net.TLS.Config
		conn, err := tls.Dial("tcp", broker, tlsConf)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = conn.Handshake()
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("TLS handshake failed for all brokers: %v", errs)
	}

	return nil
}

func (c *LazyClient) CreateTopic(t Topic) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.CreateTopic(t)
}

func (c *LazyClient) ReadTopic(name string, refresh_metadata bool) (Topic, error) {
	err := c.init()
	if err != nil {
		return Topic{}, err
	}
	return c.inner.ReadTopic(name, refresh_metadata)
}

func (c *LazyClient) UpdateTopic(t Topic) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.UpdateTopic(t)
}

func (c *LazyClient) DeleteTopic(t string) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.DeleteTopic(t)
}

func (c *LazyClient) AddPartitions(t Topic) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.AddPartitions(t)
}

func (c *LazyClient) CanAlterReplicationFactor() (bool, error) {
	err := c.init()
	if err != nil {
		return false, err
	}
	return c.inner.CanAlterReplicationFactor(), nil
}

func (c *LazyClient) AlterReplicationFactor(t Topic) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.AlterReplicationFactor(t)
}

func (c *LazyClient) IsReplicationFactorUpdating(topic string) (bool, error) {
	err := c.init()
	if err != nil {
		return false, err
	}
	return c.inner.IsReplicationFactorUpdating(topic)
}

func (c *LazyClient) CreateACL(s StringlyTypedACL) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.CreateACL(s)
}

func (c *LazyClient) ListACLs() ([]*sarama.ResourceAcls, error) {
	err := c.init()
	if err != nil {
		return nil, err
	}
	return c.inner.ListACLs()
}

func (c *LazyClient) DeleteACL(s StringlyTypedACL) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.DeleteACL(s)
}

func (c *LazyClient) AlterQuota(q Quota) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.AlterQuota(q, false)
}

func (c *LazyClient) DescribeQuota(entityType string, entityName string) (*Quota, error) {
	err := c.init()
	if err != nil {
		return nil, err
	}
	return c.inner.DescribeQuota(entityType, entityName)
}

func (c *LazyClient) UpsertUserScramCredential(userScramCredential UserScramCredential) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.UpsertUserScramCredential(userScramCredential)
}

func (c *LazyClient) DescribeUserScramCredential(username string, mechanism string) (*UserScramCredential, error) {
	err := c.init()
	if err != nil {
		return nil, err
	}
	return c.inner.DescribeUserScramCredential(username, mechanism)
}

func (c *LazyClient) DeleteUserScramCredential(userScramCredential UserScramCredential) error {
	err := c.init()
	if err != nil {
		return err
	}
	return c.inner.DeleteUserScramCredential(userScramCredential)
}
