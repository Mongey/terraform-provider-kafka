package kafka

import (
	"fmt"
	"log"
	"strings"

	"github.com/IBM/sarama"
)

type QuotaMissingError struct {
	msg string
}

func (e QuotaMissingError) Error() string { return e.msg }

type QuotaOp struct {
	Key    string
	Value  float64
	Remove bool
}

type Quota struct {
	EntityType string
	EntityName string
	Ops        []QuotaOp
}

func (a Quota) String() string {
	return strings.Join([]string{a.EntityName, a.EntityType}, "|")
}

func (a Quota) ID() string {
	return strings.Join([]string{a.EntityName, a.EntityType}, "|")
}

func (c *Client) AlterQuota(quota Quota, validateOnly bool) error {
	log.Printf("[INFO] Alter quota")
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}

	entity := sarama.QuotaEntityComponent{
		EntityType: sarama.QuotaEntityType(quota.EntityType),
		MatchType:  sarama.QuotaMatchExact,
		Name:       quota.EntityName,
	}

	configs := quota.Ops

	ops := []sarama.ClientQuotasOp{}
	for _, o := range configs {
		ops = append(ops, sarama.ClientQuotasOp{
			Key:    o.Key,
			Value:  o.Value,
			Remove: o.Remove,
		})
	}

	entry := sarama.AlterClientQuotasEntry{
		Entity: []sarama.QuotaEntityComponent{entity},
		Ops:    ops,
	}

	request := &sarama.AlterClientQuotasRequest{
		Entries:      []sarama.AlterClientQuotasEntry{entry},
		ValidateOnly: validateOnly,
	}

	log.Printf("[TRACE] Alter Quota Request %v", request)
	quotaR, err := broker.AlterClientQuotas(request)
	if err != nil {
		return err
	}

	log.Printf("[TRACE] ThrottleTime: %d", quotaR.ThrottleTime)

	for _, entry := range quotaR.Entries {
		if entry.ErrorCode != sarama.ErrNoError {
			return entry.ErrorCode
		}
	}

	return nil
}

func (c *Client) DescribeQuota(entityType string, entityName string) (*Quota, error) {
	log.Printf("[INFO] Describing Quota")
	broker, err := c.client.Controller()
	if err != nil {
		return nil, err
	}

	entity := sarama.QuotaFilterComponent{
		EntityType: sarama.QuotaEntityType(entityType),
		MatchType:  sarama.QuotaMatchExact,
		Match:      entityName,
	}

	request := &sarama.DescribeClientQuotasRequest{
		Components: []sarama.QuotaFilterComponent{entity},
		Strict:     true,
	}

	log.Printf("[TRACE] Describe Quota Request %v", request)
	quotaR, err := broker.DescribeClientQuotas(request)
	if err != nil {
		return nil, err
	}

	log.Printf("[TRACE] ThrottleTime: %d", quotaR.ThrottleTime)

	if err == nil {
		if quotaR.ErrorCode != sarama.ErrNoError {
			return nil, fmt.Errorf("Error describing quota %s", quotaR.ErrorCode)
		}
	}

	if len(quotaR.Entries) < 1 {
		return nil, QuotaMissingError{msg: fmt.Sprintf("%s could not be found", entityName)}
	}

	res := []Quota{}
	for _, e := range quotaR.Entries {
		ops := []QuotaOp{}
		for k, v := range e.Values {
			ops = append(ops, QuotaOp{
				Key:    k,
				Value:  v,
				Remove: false,
			})
		}
		for _, entry := range e.Entity {
			q := Quota{
				EntityType: string(entry.EntityType),
				EntityName: entry.Name,
				Ops:        ops,
			}
			res = append(res, q)
		}
	}

	return &res[0], err
}
