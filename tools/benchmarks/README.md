# Topic refresh benchmarks

[Findings for issue #696](FINDINGS.md) describe the retained changes, measured
speedup, logging experiments, and limits of the evidence.

## Real Terraform and Kafka

```sh
DOCKER_CONTEXT=orbstack python3 tools/benchmarks/terraform_benchmark.py \
  --terraform /path/to/terraform-1.17.1 \
  --provider-binary /path/to/baseline-provider \
  --candidate-provider-binary /path/to/candidate-provider \
  --topics 1200 --parallelism 10 --runs 10 --output /tmp/unique-empty-directory
```

Requires Docker Compose, Go, Python 3, and Terraform. Omit the provider binary to
build this checkout. Every variant gets a private provider mirror, working
directory, CLI config, and the same applied local state. The runner strips
inherited Terraform, Kafka, and AWS configuration. It starts two independent
three-broker Kafka Native 3.8.0 clusters and removes its own containers and
volumes on success or failure. Ports 19092/29092/39092 and 19093/29093/39093 must
be available. `DOCKER_CONTEXT=orbstack` is specific to the measurement host;
select your own Docker context when needed.

The real [Terraform fixture](terraform/main.tf) has two provider aliases, distinct
configs per topic and alias, one partition, replication factor three, and 49 ACLs
per cluster. Default cohorts are 593 topics per cluster, matching the issue's
1,284 resources, and 1,200 topics per cluster, totaling 2,498 resources. Default
parallelism values are 1, 10, 30, and 50.

The runner applies once, copies state to candidates, warms each variant, then
alternates ordinary saved plans and `-refresh=false` controls. It rotates variant
order each trial. The timed interval covers the complete `terraform plan`
subprocess; JSON export and validation are outside it. Every plan must refresh
exactly the expected resources, contain the exact topic configs, metadata and
ACLs, preserve the same canonical state hash, and propose no changes.

Separate diagnostic plans enable provider TRACE and broker request logging.
They check that every topic was described exactly once and that config responses
have no errors. Refresh-disabled controls must perform zero config reads.
Results include binary fingerprints/build information, host information, saved
plans, stdout, traces, per-broker request logs, and `results.json`. Interrupted
runs retain completed samples but are marked incomplete.

Use `--topics 60 --parallelism 10 --runs 1` for a smoke test. A smoke test is not
performance evidence. Compare implementations with at least five alternating
trials and report medians, ranges, correctness, and host load.

### Controlled response latency

`--broker-response-delay-ms 5` configures a modelled 5ms response delay with
Linux `tc netem`. A pinned Alpine helper with `NET_ADMIN` modifies each private
broker network namespace. The filter targets the external listener's source
port 9092, leaving replication/controller ports unaffected. Queue/filter
readback, packet counts, and zero drops are checked. Teardown removes the
namespaces and queues. This is a configured delay, not a measurement of MSK RTT.

### SDK logging comparison

Pass the same binary for both variants and add
`--candidate-sdk-log-level WARN`. Only candidate commands receive
`TF_LOG_SDK=WARN`; baseline commands retain SDK defaults. Diagnostics still
request provider TRACE, verify complete topic reads, and require zero SDK log
records for the successful WARN plan. Timed commands do not request Terraform
trace logging. Production logging defaults are unchanged.

## Profiling

Build `go build -o /tmp/profile-provider ./tools/benchmarks/profile-provider`,
pass it as the provider binary, and set `BENCH_PROFILE_DIR` outside the source
tree. `BENCH_PROFILE_CPU_ONLY=1` avoids detailed blocking/mutex profiles, whose
instrumentation can heavily distort timing. Profiling is separate from accepted
performance timings. Use the same profiled binary for default/WARN comparisons.

The diagnostic entrypoint writes CPU profiles and process CPU usage for each
provider process, plus timestamps around configure/topic/ACL read callbacks.
The union of these intervals bounds synchronous Kafka-facing callback time but
also includes provider processing and scheduling. CPU seconds overlap across
processes, and CPU profiling adds shutdown time while flushing samples.

For Core, copy [the helper](profile-core.go.txt) into the exact Terraform
v1.17.1 source as `bench_profile.go`, add `defer benchProfile()()` at the start of
`realMain`, and build. The same environment variable then records Core CPU in
separate command folders. Use the official Terraform binary for elapsed-time
comparisons; the modified build is diagnostic only.

For narrower client wait instrumentation, run
`python3 tools/benchmarks/build_wait_profile.py /tmp/unique-wait-build` and use
its `provider` binary. It copies Sarama to a temporary directory and builds a Go
overlay that timestamps broker mutex acquisition and response-select waits.
Production source and the dependency cache remain untouched. These intervals
include scheduling delay, omit request encoding/writes, and overlap other
workers' CPU. They are not Kafka server execution time or an exclusive plan
critical-path breakdown.

`python3 tools/benchmarks/analyze_plan_profiles.py OUTPUT PROFILE_DIRECTORY`
checks complete provider usage records, exact callback/config-response counts,
and interval unions. Do not add cumulative goroutine waits to wall time.

## Focused Go harnesses

`go test ./kafka -run 'TestTopicRefreshHarness|TestRefreshBrokerRouting'` verifies
mock-protocol request coverage, legacy/post-write controller routing, fallback,
and authorization errors. `go test ./kafka -run '^$' -bench BenchmarkTopicRefresh`
is a synthetic microbenchmark, not real Kafka or whole Terraform evidence.

`DOCKER_CONTEXT=orbstack tools/benchmarks/run-real-kafka.sh -race` starts a
separate disposable real Kafka cluster and runs functional update/readback and
Go refresh microbenchmarks. Production routing is compared with a controller-only
control. This uses the first cluster's ports, so run it separately from the
Terraform harness.

Run `python3 -m unittest discover -s tools/benchmarks -p 'test_*.py'` for harness
self-checks. Local PLAINTEXT, empty topics and local state do not reproduce MSK
IAM/TLS, production network/server load, or S3 backend costs.
