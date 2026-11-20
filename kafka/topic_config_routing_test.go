package kafka

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
)

type refreshRoutingClient struct {
	sarama.Client
	available       []*sarama.Broker
	controllerReads int
}

func (c *refreshRoutingClient) Brokers() []*sarama.Broker { return c.available }
func (c *refreshRoutingClient) Controller() (*sarama.Broker, error) {
	c.controllerReads++
	return c.Client.Controller()
}

func TestRefreshBrokerRouting(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		unreachable, denied, legacy, empty bool
	}{
		{name: "any-broker"}, {name: "transport-fallback", unreachable: true},
		{name: "empty-broker-list", empty: true},
		{name: "authorization", denied: true}, {name: "negotiated-legacy", legacy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTopicRefreshHarness(t, 1, sarama.V2_0_0_0, 0)
			defer h.close(t)
			var selected *sarama.Broker
			for _, broker := range h.client.client.Brokers() {
				if broker.ID() == 1 {
					selected = broker
				}
			}
			if tc.unreachable {
				selected = sarama.NewBroker("127.0.0.1:0")
				defer func() { _ = selected.Close() }()
			}
			routing := &refreshRoutingClient{Client: h.client.client, available: []*sarama.Broker{selected}}
			h.client.client = routing
			if tc.empty {
				routing.available = nil
			}
			if tc.legacy {
				h.client.supportedAPIs[32] = 0
			}
			if tc.denied {
				h.brokers[1].SetHandlerByMap(map[string]sarama.MockResponse{
					"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{Version: 2,
						Resources: []*sarama.ResourceResponse{{ErrorCode: int16(sarama.ErrTopicAuthorizationFailed), ErrorMsg: "denied"}}}),
				})
			}
			got, err := h.client.ReadTopic(h.names[0], false)
			if tc.denied {
				if !errors.Is(err, sarama.ErrTopicAuthorizationFailed) {
					t.Fatalf("authorization error lost: %v", err)
				}
			} else if err != nil || got.Config["retention.ms"] == nil || *got.Config["retention.ms"] != "5000" {
				t.Fatalf("config lost: %#v, %v", got, err)
			}
			want := 0
			if tc.legacy || tc.unreachable || tc.empty {
				want = 1
			}
			if routing.controllerReads != want {
				t.Fatalf("controller reads=%d, want %d", routing.controllerReads, want)
			}
		})
	}
}
