package kafka

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

type topicRefreshHarness struct {
	client  *Client
	brokers []*sarama.MockBroker
	names   []string
}

func newTopicRefreshHarness(t testing.TB, topics int, version sarama.KafkaVersion, delay time.Duration) *topicRefreshHarness {
	t.Helper()
	h := &topicRefreshHarness{}
	metadata := sarama.NewMockMetadataResponse(t).SetController(0)
	for id := int32(0); id < 3; id++ {
		broker := sarama.NewMockBroker(t, id)
		broker.SetLatency(delay)
		h.brokers = append(h.brokers, broker)
		metadata.SetBroker(broker.Addr(), id)
	}
	for i := 0; i < topics; i++ {
		name := fmt.Sprintf("refresh-%04d", i)
		h.names = append(h.names, name)
		metadata.SetLeader(name, 0, int32(i%3))
	}
	for _, broker := range h.brokers {
		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"MetadataRequest":        metadata,
			"DescribeConfigsRequest": sarama.NewMockDescribeConfigsResponse(t),
		})
	}
	cfg := sarama.NewConfig()
	cfg.Version = version
	cfg.ApiVersionsRequest = false
	cfg.Metadata.RefreshFrequency = 0
	cfg.Metadata.Retry.Max = 0
	cfg.Net.DialTimeout = 10 * time.Second
	cfg.Net.ReadTimeout = 10 * time.Second
	cfg.Net.WriteTimeout = 10 * time.Second
	sc, err := sarama.NewClient([]string{h.brokers[0].Addr()}, cfg)
	if err != nil {
		for _, broker := range h.brokers {
			broker.Close()
		}
		t.Fatal(err)
	}
	maxDescribeVersion := 0
	if version.IsAtLeast(sarama.V2_0_0_0) {
		maxDescribeVersion = 2
	}
	h.client = &Client{client: sc, kafkaConfig: cfg, config: &Config{}, supportedAPIs: map[int]int{32: maxDescribeVersion}}
	if err := h.client.extractTopics(); err != nil {
		h.close(t)
		t.Fatal(err)
	}
	// Isolate refresh from connection setup, including concurrent first-open races.
	for _, broker := range sc.Brokers() {
		if err := broker.Open(cfg); err != nil && err != sarama.ErrAlreadyConnected {
			h.close(t)
			t.Fatal(err)
		}
		if connected, err := broker.Connected(); !connected || err != nil {
			h.close(t)
			t.Fatalf("connect broker %d: connected=%v, error=%v", broker.ID(), connected, err)
		}
	}
	return h
}

func (h *topicRefreshHarness) close(t testing.TB) {
	t.Helper()
	if err := h.client.client.Close(); err != nil {
		t.Error(err)
	}
	for _, broker := range h.brokers {
		broker.Close()
	}
}

// Each worker follows the same ReadTopic path as a Terraform resource refresh.
func (h *topicRefreshHarness) refresh(t testing.TB, parallelism int, refreshMetadata bool) {
	t.Helper()
	var wg sync.WaitGroup
	jobs := make(chan string)
	for i := 0; i < parallelism; i++ {
		wg.Go(func() {
			for name := range jobs {
				got, err := h.client.ReadTopic(name, refreshMetadata)
				// The stock Sarama response supplies these two non-default entries.
				retention, password := "5000", "12345"
				want := Topic{Name: name, Partitions: 1, ReplicationFactor: 3, Config: map[string]*string{
					"retention.ms": &retention,
					"password":     &password,
				}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("ReadTopic(%q, %v) = %#v, %v; want %#v", name, refreshMetadata, got, err, want)
				}
			}
		})
	}
	for _, name := range h.names {
		jobs <- name
	}
	close(jobs)
	wg.Wait()
}

func (h *topicRefreshHarness) requestCounts(t testing.TB) (configs []int, resources, metadata int) {
	t.Helper()
	seen := make(map[string]bool)
	for _, broker := range h.brokers {
		requests := 0
		for _, entry := range broker.History() {
			switch req := entry.Request.(type) {
			case *sarama.DescribeConfigsRequest:
				if want := int16(h.client.supportedAPIs[32]); req.Version != want {
					t.Errorf("DescribeConfigs version = %d, want %d", req.Version, want)
				}
				requests++
				resources += len(req.Resources)
				for _, resource := range req.Resources {
					if resource.Type != sarama.TopicResource {
						t.Errorf("unexpected config resource type %v", resource.Type)
					}
					seen[resource.Name] = true
				}
			case *sarama.MetadataRequest:
				metadata++
			}
		}
		configs = append(configs, requests)
	}
	for _, name := range h.names {
		if !seen[name] {
			t.Errorf("topic %q was never described", name)
		}
	}
	return configs, resources, metadata
}

func TestTopicRefreshHarness(t *testing.T) {
	for _, tc := range []struct {
		name       string
		version    sarama.KafkaVersion
		afterWrite bool
	}{
		{"refresh-v2", sarama.V2_0_0_0, false},
		{"refresh-v0", sarama.V0_11_0_0, false},
		{"after-write-v2", sarama.V2_0_0_0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTopicRefreshHarness(t, 12, tc.version, 0)
			defer h.close(t)
			h.refresh(t, 10, tc.afterWrite)
			counts, resources, metadata := h.requestCounts(t)
			t.Logf("DescribeConfigs by broker=%v resources=%d Metadata=%d (includes startup)", counts, resources, metadata)
			if (tc.afterWrite || tc.version == sarama.V0_11_0_0) && (counts[1] != 0 || counts[2] != 0) {
				t.Errorf("legacy and post-write reads must use controller, got %v", counts)
			}
		})
	}
}

// One operation is a complete cold refresh, with a fresh client for every sample.
func BenchmarkTopicRefresh(b *testing.B) {
	output := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(output) })
	for _, topics := range []int{60, 593} {
		for _, parallelism := range []int{1, 10, 50} {
			b.Run(fmt.Sprintf("topics=%d/parallelism=%d", topics, parallelism), func(b *testing.B) {
				var totals [3]int
				var totalResources int
				b.StopTimer()
				for i := 0; i < b.N; i++ {
					h := newTopicRefreshHarness(b, topics, sarama.V2_0_0_0, 3*time.Millisecond)
					b.StartTimer()
					h.refresh(b, parallelism, false)
					b.StopTimer()
					counts, resources, _ := h.requestCounts(b)
					for broker, count := range counts {
						totals[broker] += count
					}
					totalResources += resources
					h.close(b)
				}
				for broker, total := range totals {
					b.ReportMetric(float64(total)/float64(b.N), fmt.Sprintf("broker%d-requests/refresh", broker))
				}
				b.ReportMetric(float64(totalResources)/float64(b.N), "resources/refresh")
			})
		}
	}
}
