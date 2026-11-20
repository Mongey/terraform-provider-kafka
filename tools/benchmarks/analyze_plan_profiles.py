#!/usr/bin/env python3
"""Summarize separate Core/provider CPU and the union of provider read intervals."""
import json
from pathlib import Path
import sys


def interval_union(intervals):
    end = None
    total = 0
    for start, stop in sorted(intervals):
        assert stop >= start
        if end is None or start > end:
            total += stop - start
        else:
            total += max(0, stop - end)
        end = max(stop, end) if end is not None else stop
    return total / 1e9


def summarize(output, profiles):
    results = json.loads((output / "results.json").read_text())
    assert results["completed"] and not results["cleanup_errors"]
    rows = []
    for timing in results["timings"]:
        directory = profiles / timing["artifact"]
        core_files = list(directory.glob("core-*.json"))
        assert len(core_files) == 1
        core = json.loads(core_files[0].read_text())
        cpu_pids = {p.stem for p in directory.glob("*.cpu") if not p.name.startswith("core-")}
        event_pids = {p.name.split(".")[0] for p in directory.glob("*.events.json")}
        assert cpu_pids == event_pids, "incomplete provider CPU usage records"
        providers = [json.loads(path.read_text()) for path in directory.glob("*.events.json")]
        assert providers
        events = [event for provider in providers for event in provider["events"] or []]
        counts = {kind: sum(event["kind"] == kind for event in events)
                  for kind in ("kafka_topic", "kafka_acl", "configure")}
        assert counts["kafka_topic"] == (2 * timing["topics_per_cluster"] if timing["refresh"] else 0)
        assert counts["kafka_acl"] == (98 if timing["refresh"] else 0)
        active = interval_union((event["start_ns"], event["end_ns"]) for event in events)
        core_wall = (core["end_ns"] - core["start_ns"]) / 1e9
        assert 0 <= active <= core_wall <= timing["wall_s"]
        waits = [event for provider in providers for event in provider.get("kafka_waits") or []]
        wait_summary = {}
        if any("kafka_waits" in provider for provider in providers):
            descriptions = sum(e["kind"] == "response_wait" and e["api"] == 32 for e in waits)
            assert descriptions == (2 * timing["topics_per_cluster"] if timing["refresh"] else 0)
            for kind in ("broker_lock", "response_wait", "combined"):
                seconds = interval_union((e["start_ns"], e["end_ns"]) for e in waits
                                         if kind == "combined" or e["kind"] == kind)
                wait_summary[kind + "_union_s"] = seconds
            wait_summary["combined_union_fraction"] = wait_summary["combined_union_s"] / timing["wall_s"]
            wait_summary["describe_configs_responses"] = descriptions
        rows.append({**timing, "kafka_waits": wait_summary, "core_cpu_s": core["user_s"] + core["system_s"],
                     "provider_cpu_s": sum(p["user_s"] + p["system_s"] for p in providers),
                     "provider_processes": len(providers), "event_counts": counts,
                     "read_and_configure_union_s": active,
                     "read_and_configure_union_fraction": active / timing["wall_s"],
                     "outside_read_and_configure_s": timing["wall_s"] - active,
                     "core_instrumented_wall_s": core_wall})
    return {"method": "CPU seconds are cumulative process usage and overlap wall time. The union of configure/topic/ACL read callbacks is a conservative upper bound on time waiting for Kafka on this plan's refresh path; it includes provider processing and scheduling. Kafka wait unions measure time with at least one worker in the broker lock or response select, including scheduler delay. They overlap Core/provider CPU, omit request encoding and writes, and are not an exclusive critical-path decomposition. No summed goroutine waits are treated as wall time.",
            "results_path": str(output / "results.json"), "rows": rows}


if __name__ == "__main__":
    assert interval_union([(0, 10), (5, 20), (25, 30)]) == 25 / 1e9
    assert interval_union([]) == 0
    print(json.dumps(summarize(Path(sys.argv[1]), Path(sys.argv[2])), indent=2))
