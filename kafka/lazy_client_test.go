package kafka

import (
	"strings"
	"testing"

	"github.com/IBM/sarama"
)

func Test_LazyClientWithNoConfig(t *testing.T) {
	c := &LazyClient{}
	_, err := c.ListACLs()

	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
}

func Test_LazyClientErrors(t *testing.T) {
	c := &LazyClient{}
	_, err := c.ListACLs()
	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
	_, err = c.ListACLs()
	if err == nil {
		t.Fatalf("exepted err, got %v", err)
	}
}

func Test_LazyClientWithConfigErrors(t *testing.T) {
	config := &Config{
		BootstrapServers: &[]string{"localhost:9000"},
		Timeout:          10,
	}
	c := &LazyClient{
		Config: config,
	}
	err := c.init()

	if !strings.Contains(err.Error(), sarama.ErrOutOfBrokers.Error()) {
		t.Fatalf("expected err, got %v", err)
	}
}
