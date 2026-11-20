# Topic refresh performance: issue #696

Ordinary topic config reads now spread across existing broker connections instead
of sending every DescribeConfigs request to the controller. This removes a
single-connection bottleneck when responses have appreciable latency. Existing
refresh logging is unchanged.

## What the issue reports

[Issue #696](https://github.com/Mongey/terraform-provider-kafka/issues/696) compares
this provider against two MSK clusters, with 593 topics and 49 ACLs each. Its
follow-up found batching slower, while distributing individual config reads
across brokers improved actual MSK plans. The AWS provider comparison uses its
MSK control-plane API and is not a faster implementation of this provider.
Our measurements below use real Terraform and real local Kafka, not MSK/IAM.

## Retained implementation

- Ordinary source-aware topic refreshes select a random existing broker for
  DescribeConfigs. No new clients, connection pools, batching, or config cache.
- Post-write and legacy config reads retain controller routing. Broker
  connection/request failures fall back to the controller. Kafka authorization
  response errors remain errors.
- No provider configuration, Terraform state schema, dependency, or logging
  behavior changes. The provider log cleanup was reverted because it did not
  establish a plan-time improvement.

## Whole Terraform measurements

Terraform 1.17.1, Sarama 1.61.1, two real three-broker Kafka Native 3.8.0 clusters,
2,400 topics plus 98 ACLs, parallelism 10. Each variant uses the same applied
state and alternating trials. Values are whole ordinary-plan seconds.

| Condition | Controller-only baseline | Broker-spreading candidate | Interpretation |
| --- | ---: | ---: | --- |
| No added delay, 15 trials | 3.481 median | 3.763 median | No reliable local-loopback speedup |
| Configured 5ms response delay, 5 trials | 19.122 (18.587–19.305) | 8.919 (8.194–9.168) | 53.4% lower median elapsed time |
| Extended 5ms run, 15 completed pairs | 19.716 (18.925–21.306) | 9.090 (8.281–10.411) | Candidate won all 15 pairs |

The extended run was requested for 100 trials and stopped by the user after 15
complete ordinary-plan pairs. It is marked incomplete and is not a 100-trial
result. Routing comparisons preceded the logging cleanup, isolating broker
selection. The delay is configured on broker responses and is not measured MSK
RTT. Local host noise and Terraform/SDK overhead limit zero-delay conclusions.

[Five-trial results](issue-696-final-latency.json),
[network filter/counter evidence](issue-696-final-network.txt),
[extended partial results](issue-696-extended-partial.json), and
[zero-delay comparison](issue-696-terraform-ab-default.json) retain the evidence.
Saved plans validated every resource/config/ACL and identical canonical state,
with no planned changes. Separate broker logs checked each topic was described
exactly once per cluster, no config response errors, and zero config requests
with refresh disabled. Delay filters reported zero drops; task clusters were
cleaned up.

## Logging cleanup (not retained)

Ten alternating local trials on 2,498 resources measured ordinary-plan medians
of 4.343s before and 4.216s after. This did not establish an elapsed-time gain.
Provider-owned diagnostic records fell 101,324 to 4,932, a 95.1% reduction.
SDK records stayed at 64,847. See [summary](issue-696-trimmed-logs-summary.json)
and [full results](issue-696-trimmed-logs.json). This is a diagnostic-volume
improvement, not a plan speedup. These log removals and their associated test
changes were reverted; the measurements remain as historical experiment results.

## SDK WARN experiment and CPU attribution

Setting `TF_LOG_SDK=WARN` in the candidate environment removed all 64,847 SDK
records while preserving the provider's 4,932 records and all 2,400 config reads.
No SDK default was changed. These measurements used the experimental provider
with trimmed logs, before that cleanup was reverted. They describe that
experiment, not a fresh CPU profile of the final routing-only implementation.
The same provider binary and official Terraform binary were used on both sides.

Ten unprofiled ordinary-plan pairs gave medians of 6.638s default versus 6.871s
WARN, ranges 3.299–14.800s versus 3.599–21.796s, and five wins each. The paired
sign-flip two-sided p-value was 0.205. This is inconclusive on the busy host.

Five separate CPU-profile pairs measured the following median CPU usage per
ordinary plan. Every pair used less CPU with WARN for both process groups.

| Process group | SDK defaults | SDK WARN | Reduction |
| --- | ---: | ---: | ---: |
| Provider | 3.367 CPU-seconds | 2.634 CPU-seconds | 21.8% |
| Terraform Core | 6.658 CPU-seconds | 6.019 CPU-seconds | 9.6% |
| Core plus provider | 10.025 CPU-seconds | 8.653 CPU-seconds | 13.7% |

SDK SubsystemTrace fell from 43.87% to 1.34% of provider CPU samples. Core plugin
stderr handling fell from 2.60 to 0.53 aggregated sample-seconds across five
plans. With WARN, about 70% of measured Core/provider CPU belongs to Core; major
remaining paths include graph/planning, GC, provider RPC/TLS and scheduling.
Our topicConfig subtree accounts for 3.77% of provider CPU samples.

CPU seconds, sampled stack percentages and blocked time are different measures.
Concurrent CPU work and Kafka waits overlap; these figures cannot be added into
an exclusive elapsed-time breakdown or interpreted as a 14% plan speedup.
Detailed blocking instrumentation also perturbed elapsed time substantially, so
those timings were not accepted as performance comparisons.

[Unprofiled results](issue-696-sdk-warn-timing.json) and
[paired statistics](issue-696-sdk-warn-timing-summary.json),
[profiled results](issue-696-sdk-warn-profile.json),
[per-plan CPU usage](issue-696-sdk-warn-profile-summary.json),
[CPU usage summary](issue-696-sdk-warn-cpu-usage-summary.json), and
[CPU stack summary](issue-696-sdk-warn-cpu-stacks-summary.json) preserve these
checks. All 60 ordinary/control plans and eight separate diagnostics validated
identical complete state and no-op actions. Both harness runs completed without
cleanup errors.

## Alternatives not retained

Batching was not selected. Least-busy routing and a second client/connection per
broker did not establish reliable gains over random spreading on the shared
host. Forced log suppression was not shipped. The retained implementation keeps
existing connection lifecycles and normal logging controls.

[Reproduction instructions](README.md) cover the real Terraform fixture,
controlled response delay, SDK logging comparison, CPU and wait profiles, and
functional real-Kafka checks. Historical measurements were taken on 2026-11-18;
fresh pre-PR correctness checks are recorded in the PR. No MSK/IAM performance
claim or full acceptance-suite result is implied by these local measurements.

## Fresh pre-PR validation, 2026-11-20

- `env -u TF_ACC go test -race ./...`: passed, including routing, fallback,
  authorization, legacy/post-write and logging tests. A separate index snapshot
  of the routing-only commit also passed the race suite.
- `golangci-lint run`: zero issues.
- Python harness self-checks: six passed.
- `tools/benchmarks/run-real-kafka.sh -race -bench '^$' -count=1`: passed the
  real-Kafka functional refresh and update/readback check. Benchmarks were
  disabled; this is fresh correctness evidence, not a new performance result.
- Real Terraform baseline/candidate smoke: two three-broker clusters, 60 topics
  and 49 ACLs per cluster, four saved ordinary/control plans and four separate
  diagnostic plans. All 218 resources and canonical states matched with no-op
  actions. Candidate config reads were distributed 28/18/14 and 18/19/23 across
  the brokers of the two clusters; controls performed zero config reads.
  [Smoke results](issue-696-pr-smoke.json) include binary fingerprints and
  successful cleanup. One trial is not a speedup measurement.
- All task Kafka containers were removed. Full acceptance tests against MSK/IAM
  were not run. The original checkout's three pre-existing edits were untouched.


## Logging rollback, 2026-11-20

The bounded-logging commit was reverted in full, including its logging-specific
test and ACL concurrency-test rewrite. Production Kafka source matches the
routing-only commit (`cd159ac`) exactly. The routing speedup measurements above
were collected before logging was trimmed and still apply to the retained change.
