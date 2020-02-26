package kafka

import (
	"sync"

	"github.com/Shopify/sarama"
)

type LazyClient struct {
	once   sync.Once
	inner  *Client
	Config *Config
}

func (c *LazyClient) init() error {
	var err error
	c.once.Do(func() {
		c.inner, err = NewClient(c.Config)
	})

	return err
}

func (c *LazyClient) CreateTopic(t Topic) error {
	c.init()
	return c.inner.CreateTopic(t)
}

func (c *LazyClient) ReadTopic(name string) (Topic, error) {
	c.init()
	return c.inner.ReadTopic(name)
}

func (c *LazyClient) UpdateTopic(t Topic) error {
	c.init()
	return c.inner.UpdateTopic(t)
}

func (c *LazyClient) DeleteTopic(t string) error {
	c.init()
	return c.inner.DeleteTopic(t)
}

func (c *LazyClient) AddPartitions(t Topic) error {
	c.init()
	return c.inner.AddPartitions(t)
}

func (c *LazyClient) CreateACL(s StringlyTypedACL) error {
	c.init()
	return c.inner.CreateACL(s)
}

func (c *LazyClient) ListACLs() ([]*sarama.ResourceAcls, error) {
	c.init()
	return c.inner.ListACLs()
}

func (c *LazyClient) DeleteACL(s StringlyTypedACL) error {
	c.init()
	return c.inner.DeleteACL(s)
}
