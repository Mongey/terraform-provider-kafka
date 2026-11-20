package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/Mongey/terraform-provider-kafka/kafka"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	if dir := os.Getenv("BENCH_PROFILE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			panic(err)
		}
		file, err := os.Create(filepath.Join(dir, fmt.Sprintf("%d.cpu", os.Getpid())))
		if err != nil {
			panic(err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			panic(err)
		}
		if os.Getenv("BENCH_PROFILE_CPU_ONLY") != "1" {
			runtime.SetBlockProfileRate(1)
			runtime.SetMutexProfileFraction(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			if err := file.Close(); err != nil {
				panic(err)
			}
			if os.Getenv("BENCH_PROFILE_CPU_ONLY") == "1" {
				return
			}
			for _, kind := range []string{"block", "mutex", "heap"} {
				file, err := os.Create(filepath.Join(dir, fmt.Sprintf("%d.%s", os.Getpid(), kind)))
				if err != nil {
					panic(err)
				}
				if err := pprof.Lookup(kind).WriteTo(file, 0); err != nil {
					panic(err)
				}
				if err := file.Close(); err != nil {
					panic(err)
				}
			}
		}()
	}
	if dir := os.Getenv("BENCH_PROFILE_DIR"); dir != "" {
		defer writeReadEvents(dir)
	}
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: profiledProvider})
}

// Record overlapping read intervals so their union, rather than their sum, bounds Kafka time.
type readEvent struct {
	Kind  string `json:"kind"`
	Start int64  `json:"start_ns"`
	End   int64  `json:"end_ns"`
}

var readEvents struct {
	sync.Mutex
	Events []readEvent
}

func recordRead(kind string, start time.Time) {
	event := readEvent{kind, start.UnixNano(), time.Now().UnixNano()}
	readEvents.Lock()
	readEvents.Events = append(readEvents.Events, event)
	readEvents.Unlock()
}

//nolint:staticcheck // Profile the same legacy configure callback used by kafka.Provider.
func profiledProvider() *schema.Provider {
	provider := kafka.Provider()
	if os.Getenv("BENCH_PROFILE_DIR") == "" {
		return provider
	}
	configure := provider.ConfigureFunc
	provider.ConfigureFunc = func(data *schema.ResourceData) (interface{}, error) {
		defer recordRead("configure", time.Now())
		return configure(data)
	}
	for _, kind := range []string{"kafka_topic", "kafka_acl"} {
		read := provider.ResourcesMap[kind].ReadContext
		provider.ResourcesMap[kind].ReadContext = func(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
			defer recordRead(kind, time.Now())
			return read(ctx, data, meta)
		}
	}
	return provider
}
func writeReadEvents(dir string) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		panic(err)
	}
	readEvents.Lock()
	defer readEvents.Unlock()
	data, err := json.Marshal(map[string]interface{}{"events": readEvents.Events, "user_s": float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6, "system_s": float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.events.json", os.Getpid())), data, 0600); err != nil {
		panic(err)
	}
}
