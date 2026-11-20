package kafka

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
	uuid "github.com/hashicorp/go-uuid"
)

// This harness only targets the disposable cluster in tools/benchmarks/compose.yaml.
func newRealRefreshClient(t testing.TB, want []Topic) *Client {
	t.Helper()
	if os.Getenv("KAFKA_REAL_BENCH") != "1" {
		t.Skip("run tools/benchmarks/run-real-kafka.sh to start the real Kafka benchmark")
	}
	servers := []string{"127.0.0.1:19092", "127.0.0.1:29092", "127.0.0.1:39092"}
	var c *Client
	deadline := time.Now().Add(90 * time.Second)
	for {
		var err error
		c, err = NewClient(&Config{BootstrapServers: &servers, KafkaVersion: "3.8.0", Timeout: 30})
		if err == nil && len(c.client.Brokers()) == 3 {
			break
		}
		if err == nil {
			err = fmt.Errorf("expected three registered brokers, got %d", len(c.client.Brokers()))
		}
		if c != nil {
			closeRealRefreshClient(t, c)
		}
		if time.Now().After(deadline) {
			t.Fatalf("Kafka did not become ready: %v", err)
		}
		time.Sleep(time.Second)
	}
	// Exclude connection establishment from refresh timings.
	for _, broker := range c.client.Brokers() {
		if broker.ID() < 1 || broker.ID() > 3 {
			closeRealRefreshClient(t, c)
			t.Fatalf("unexpected benchmark broker ID %d", broker.ID())
		}
		if err := broker.Open(c.kafkaConfig); err != nil && err != sarama.ErrAlreadyConnected {
			closeRealRefreshClient(t, c)
			t.Fatal(err)
		}
		if connected, err := broker.Connected(); !connected || err != nil {
			closeRealRefreshClient(t, c)
			t.Fatalf("connect broker %d: connected=%v, error=%v", broker.ID(), connected, err)
		}
	}
	if len(want) > 0 {
		names := make([]string, len(want))
		for i := range want {
			names[i] = want[i].Name
		}
		// A fresh connection can reach a broker whose metadata still predates creation.
		for {
			if err := c.client.RefreshMetadata(names...); err == nil {
				if err := c.extractTopics(); err != nil {
					closeRealRefreshClient(t, c)
					t.Fatal(err)
				}
				ready := true
				for _, name := range names {
					if _, ok := c.topics[name]; !ok {
						ready = false
						break
					}
				}
				if ready {
					break
				}
			}
			if time.Now().After(deadline) {
				closeRealRefreshClient(t, c)
				t.Fatal("benchmark topics did not become visible in client metadata")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return c
}

func closeRealRefreshClient(t testing.TB, c *Client) {
	t.Helper()
	if err := c.client.Close(); err != nil {
		t.Error(err)
	}
}

func realRefreshFixture(t testing.TB, count int) []Topic {
	t.Helper()
	c := newRealRefreshClient(t, nil)
	t.Cleanup(func() { closeRealRefreshClient(t, c) })
	id, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	controller, err := c.client.Controller()
	if err != nil {
		t.Fatal(err)
	}
	var created []string
	t.Cleanup(func() {
		if len(created) == 0 {
			return
		}
		response, err := controller.DeleteTopics(&sarama.DeleteTopicsRequest{Version: 3, Topics: created, Timeout: 30 * time.Second})
		if err != nil {
			t.Errorf("delete benchmark topics: %v", err)
			return
		}
		for name, code := range response.TopicErrorCodes {
			if code != sarama.ErrNoError {
				t.Errorf("delete %s: %v", name, code)
			}
		}
	})
	request := &sarama.CreateTopicsRequest{Version: 3, Timeout: 60 * time.Second, TopicDetails: map[string]*sarama.TopicDetail{}}
	want := make([]Topic, count)
	for i := range want {
		retention, policy := strconv.Itoa(60000+i), "delete"
		if i%2 == 0 {
			policy = "compact"
		}
		want[i] = Topic{Name: fmt.Sprintf("refresh-bench-%s-%04d", id, i), Partitions: 1, ReplicationFactor: 3,
			Config: map[string]*string{"retention.ms": &retention, "cleanup.policy": &policy}}
		request.TopicDetails[want[i].Name] = &sarama.TopicDetail{NumPartitions: 1, ReplicationFactor: 3, ConfigEntries: want[i].Config}
	}
	response, err := controller.CreateTopics(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range want {
		result, ok := response.TopicErrors[topic.Name]
		if !ok || result.Err != sarama.ErrNoError {
			t.Errorf("create %s: %v", topic.Name, result)
		} else {
			created = append(created, topic.Name)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	// Wait for actual replication, not just the create acknowledgement.
	deadline := time.Now().Add(90 * time.Second)
	for {
		if err := c.client.RefreshMetadata(created...); err != nil {
			t.Fatal(err)
		}
		ready := true
		for _, name := range created {
			isr, err := c.client.InSyncReplicas(name, 0)
			if err != nil || len(isr) != 3 {
				ready = false
				break
			}
		}
		if ready {
			return want
		}
		if time.Now().After(deadline) {
			t.Fatal("benchmark topics did not reach three in-sync replicas")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Keep a controller-only control after production refreshes start spreading reads.
type realRefreshControllerOnlyClient struct {
	sarama.Client
	broker *sarama.Broker
}

func (c *realRefreshControllerOnlyClient) Brokers() []*sarama.Broker {
	return []*sarama.Broker{c.broker}
}
func forceRealRefreshController(t testing.TB, c *Client) int32 {
	t.Helper()
	broker, err := c.client.Controller()
	if err != nil {
		t.Fatal(err)
	}
	c.client = &realRefreshControllerOnlyClient{Client: c.client, broker: broker}
	return broker.ID()
}

type realRefreshControllerClient struct {
	sarama.Client
	requests [3]atomic.Int64
}

func (c *realRefreshControllerClient) Controller() (*sarama.Broker, error) {
	broker, err := c.Client.Controller()
	if err == nil {
		c.requests[broker.ID()-1].Add(1)
	}
	return broker, err
}

func realRefreshPass(t testing.TB, c *Client, want []Topic, workers int) {
	t.Helper()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for i := 0; i < workers; i++ {
		wg.Go(func() {
			for i := range jobs {
				got, err := c.ReadTopic(want[i].Name, false)
				if err != nil || !reflect.DeepEqual(got, want[i]) {
					t.Errorf("ReadTopic(%s): got %#v, error %v; want %#v", want[i].Name, got, err, want[i])
				}
			}
		})
	}
	for i := range want {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func realRefreshRequests(c *Client) [3]int64 {
	var counts [3]int64
	for i := range counts {
		meter := c.kafkaConfig.MetricRegistry.Get(fmt.Sprintf("protocol-requests-rate-32-for-broker-%d", i+1))
		if meter != nil {
			counts[i] = meter.(interface{ Count() int64 }).Count()
		}
	}
	return counts
}

func TestRealTopicRefresh(t *testing.T) {
	want := realRefreshFixture(t, 12)
	for _, controllerOnly := range []bool{false, true} {
		c := newRealRefreshClient(t, want)
		var selected int32
		if controllerOnly {
			selected = forceRealRefreshController(t, c)
		}
		realRefreshPass(t, c, want, 10)
		counts := realRefreshRequests(c)
		t.Logf("controller-only=%v DescribeConfigs by broker=%v", controllerOnly, counts)
		if counts[0]+counts[1]+counts[2] != int64(len(want)) {
			t.Errorf("wrong DescribeConfigs total: %v", counts)
		}
		for i, count := range counts {
			if controllerOnly && int32(i+1) != selected && count != 0 {
				t.Errorf("controller control escaped to broker %d", i+1)
			}
		}
		closeRealRefreshClient(t, c)
	}
	c := newRealRefreshClient(t, want)
	defer closeRealRefreshClient(t, c)
	realRefreshPass(t, c, want[:1], 1)
	retention, policy := "123456", "compact"
	updated := want[0]
	updated.Config = map[string]*string{"retention.ms": &retention, "cleanup.policy": &policy}
	if err := c.UpdateTopic(updated); err != nil {
		t.Fatal(err)
	}
	before := realRefreshRequests(c)
	routing := &realRefreshControllerClient{Client: c.client}
	c.client = routing
	lazy := &LazyClient{inner: c, Config: c.config}
	lazy.once.Do(func() {})
	// Kafka propagates config updates asynchronously; use the production waiter.
	if err := waitForTopicRefresh(context.Background(), lazy, updated.Name, updated); err != nil {
		t.Fatal(err)
	}
	got, err := c.ReadTopic(updated.Name, true)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatalf("post-write read: got %#v, error %v; want %#v", got, err, updated)
	}
	for i, count := range realRefreshRequests(c) {
		// Metadata refresh can change Sarama's controller destination in KRaft.
		if got, want := count-before[i], routing.requests[i].Load(); got != want {
			t.Errorf("post-write broker %d: %d config requests, %d controller lookups", i+1, got, want)
		}
	}
}

func BenchmarkRealTopicRefresh(b *testing.B) {
	output := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(output) })
	want := realRefreshFixture(b, 593)
	for _, topics := range []int{60, 593} {
		for _, workers := range []int{1, 10, 50} {
			for _, mode := range []string{"provider", "controller-control"} {
				b.Run(fmt.Sprintf("topics=%d/workers=%d/%s", topics, workers, mode), func(b *testing.B) {
					b.StopTimer()
					var totals [3]int64
					for i := 0; i < b.N; i++ {
						c := newRealRefreshClient(b, want)
						if mode == "controller-control" {
							forceRealRefreshController(b, c)
						}
						before := realRefreshRequests(c)
						b.StartTimer()
						realRefreshPass(b, c, want[:topics], workers)
						b.StopTimer()
						after := realRefreshRequests(c)
						for j := range totals {
							totals[j] += after[j] - before[j]
						}
						closeRealRefreshClient(b, c)
					}
					for i, total := range totals {
						b.ReportMetric(float64(total)/float64(b.N), fmt.Sprintf("broker%d-requests/refresh", i+1))
					}
				})
			}
		}
	}
}
