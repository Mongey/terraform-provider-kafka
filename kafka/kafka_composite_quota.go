package kafka

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/IBM/sarama"
)

type CompositeQuotaEntity struct {
	Type string
	Name string
}

type CompositeQuota struct {
	Entities []CompositeQuotaEntity
	Ops      []QuotaOp
}

func (q CompositeQuota) String() string {
	return q.ID()
}

func (q CompositeQuota) ID() string {
	parts := make([]string, len(q.Entities))
	for i, e := range q.Entities {
		name := e.Name
		if name == "" {
			name = entityDefault
		}
		parts[i] = e.Type + ":" + name
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func configOpsFromMap(config map[string]interface{}, removeAll bool) ([]QuotaOp, error) {
	ops := make([]QuotaOp, 0, len(config))
	for key, value := range config {
		var f float64
		switch val := value.(type) {
		case float64:
			f = val
		case string:
			var err error
			f, err = strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err != nil {
				return nil, fmt.Errorf("config key %q: cannot parse %q as float64: %w", key, val, err)
			}
		default:
			return nil, fmt.Errorf("config key %q has unexpected type %T", key, value)
		}
		ops = append(ops, QuotaOp{Key: key, Value: f, Remove: removeAll})
	}
	return ops, nil
}

func (c *Client) AlterCompositeQuota(quota CompositeQuota, validateOnly bool) error {
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}

	entityComponents := make([]sarama.QuotaEntityComponent, len(quota.Entities))
	for i, e := range quota.Entities {
		if e.Name == "" {
			entityComponents[i] = sarama.QuotaEntityComponent{
				EntityType: sarama.QuotaEntityType(e.Type),
				MatchType:  sarama.QuotaMatchDefault,
			}
		} else {
			entityComponents[i] = sarama.QuotaEntityComponent{
				EntityType: sarama.QuotaEntityType(e.Type),
				MatchType:  sarama.QuotaMatchExact,
				Name:       e.Name,
			}
		}
	}

	ops := make([]sarama.ClientQuotasOp, len(quota.Ops))
	for i, o := range quota.Ops {
		ops[i] = sarama.ClientQuotasOp{Key: o.Key, Value: o.Value, Remove: o.Remove}
	}

	request := &sarama.AlterClientQuotasRequest{
		Entries:      []sarama.AlterClientQuotasEntry{{Entity: entityComponents, Ops: ops}},
		ValidateOnly: validateOnly,
	}

	resp, err := broker.AlterClientQuotas(request)
	if err != nil {
		return err
	}

	for _, entry := range resp.Entries {
		if entry.ErrorCode != sarama.ErrNoError {
			return entry.ErrorCode
		}
	}

	return nil
}

func (c *Client) DescribeCompositeQuota(entities []CompositeQuotaEntity) (*CompositeQuota, error) {
	broker, err := c.client.Controller()
	if err != nil {
		return nil, err
	}

	components := make([]sarama.QuotaFilterComponent, len(entities))
	for i, e := range entities {
		if e.Name == "" {
			components[i] = sarama.QuotaFilterComponent{
				EntityType: sarama.QuotaEntityType(e.Type),
				MatchType:  sarama.QuotaMatchDefault,
			}
		} else {
			components[i] = sarama.QuotaFilterComponent{
				EntityType: sarama.QuotaEntityType(e.Type),
				MatchType:  sarama.QuotaMatchExact,
				Match:      e.Name,
			}
		}
	}

	request := &sarama.DescribeClientQuotasRequest{
		Components: components,
		Strict:     true,
	}

	resp, err := broker.DescribeClientQuotas(request)
	if err != nil {
		return nil, err
	}

	if resp.ErrorCode != sarama.ErrNoError {
		return nil, fmt.Errorf("error describing composite quota: %s", resp.ErrorCode)
	}

	if len(resp.Entries) < 1 {
		return nil, QuotaMissingError{msg: fmt.Sprintf("composite quota %s could not be found", CompositeQuota{Entities: entities}.ID())}
	}

	e := resp.Entries[0]
	resultEntities := make([]CompositeQuotaEntity, len(e.Entity))
	for i, comp := range e.Entity {
		resultEntities[i] = CompositeQuotaEntity{Type: string(comp.EntityType), Name: comp.Name}
	}

	ops := make([]QuotaOp, 0, len(e.Values))
	for k, v := range e.Values {
		ops = append(ops, QuotaOp{Key: k, Value: v})
	}

	return &CompositeQuota{Entities: resultEntities, Ops: ops}, nil
}
