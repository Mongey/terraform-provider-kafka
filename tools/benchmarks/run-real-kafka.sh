#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
compose=(docker compose -p "kafka-refresh-bench-$$" -f tools/benchmarks/compose.yaml)
cleanup() {
  status=$?
  if (( status != 0 )); then
    "${compose[@]}" logs --tail=100 >&2 || true
  fi
  "${compose[@]}" down --volumes || exit 1
  exit "$status"
}
trap cleanup EXIT
"${compose[@]}" up -d --wait --wait-timeout 240
env -u TF_ACC KAFKA_REAL_BENCH=1 go test ./kafka \
  -run '^TestRealTopicRefresh$' -bench '^BenchmarkRealTopicRefresh$' \
  -benchtime=1x -count=10 -timeout=15m "$@"
