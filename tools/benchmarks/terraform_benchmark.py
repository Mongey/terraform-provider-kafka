#!/usr/bin/env python3
"""Measure real Terraform plans against two disposable Kafka clusters."""

import argparse
import base64
from collections import Counter
from datetime import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import statistics
import subprocess
import tempfile
import time
import uuid


ROOT = Path(__file__).resolve().parents[2]
NETWORK_IMAGE = "alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def run(command, *, env=None, cwd=None, output=None, timeout=900):
    start = time.perf_counter()
    if output:
        with Path(output).open("w") as stream:
            result = subprocess.run(command, env=env, cwd=cwd, stdout=stream,
                                    stderr=subprocess.STDOUT, timeout=timeout)
        detail = str(output)
    else:
        result = subprocess.run(command, env=env, cwd=cwd, capture_output=True,
                                text=True, timeout=timeout)
        detail = result.stderr[-2000:]
    if result.returncode != 0:
        raise RuntimeError(f"exit {result.returncode}: {command}; see {detail}")
    return result, time.perf_counter() - start


def validate_plan(document, topics):
    resources = document["prior_state"]["values"]["root_module"]["resources"]
    require(len(resources) == 2 * topics + 98, "wrong refreshed resource count")
    counts = Counter()
    seen = set()
    for resource in resources:
        kind, alias = resource["type"], resource["name"]
        require(alias in ("first", "second"), f"unexpected resource {resource['address']}")
        counts[kind, alias] += 1
        values = resource["values"]
        index = int(resource["index"].rsplit("-", 1)[1])
        key = (kind, alias, index)
        require(key not in seen and 0 <= index < (topics if kind == "kafka_topic" else 49),
                f"duplicate or unexpected resource: {resource['address']}")
        seen.add(key)
        name = f"refresh-topic-{index:04d}"
        if kind == "kafka_topic":
            base = 60000 if alias == "first" else 120000
            require(values["name"] == name and values["partitions"] == 1
                    and values["replication_factor"] == 3, f"wrong topic metadata: {resource['address']}")
            require(values["config"] == {"retention.ms": str(base + index),
                    "cleanup.policy": "compact" if index % 2 == 0 else "delete"},
                    f"wrong topic config or provider isolation: {resource['address']}")
        else:
            require(kind == "kafka_acl" and values["resource_name"] == name
                    and values["acl_principal"] == f"User:refresh-bench-{index}"
                    and values["acl_operation"] == "Read" and values["acl_permission_type"] == "Allow",
                    f"wrong ACL: {resource['address']}")
    for alias in ("first", "second"):
        require(counts["kafka_topic", alias] == topics and counts["kafka_acl", alias] == 49,
                f"wrong counts for provider {alias}")
    changes = document.get("resource_changes", [])
    require(len(changes) == len(resources), "missing planned resources")
    require(all(item["change"]["actions"] == ["no-op"] for item in changes),
            "plan contains unexpected changes")
    canonical = json.dumps(sorted(resources, key=lambda r: r["address"]), sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode()).hexdigest()


def variant_environment(env, variant, sdk_log_level):
    command_env = env.copy()
    if variant != "baseline" and sdk_log_level:
        command_env["TF_LOG_SDK"] = sdk_log_level
    return command_env


def provider_trace(path):
    reads, completions = [], []
    sdk_lines = 0
    provider_lines = 0
    for line in path.read_text().splitlines():
        sdk_lines += "@module=sdk." in line
        provider_lines += bool(re.search(r": \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} \[", line))
        match = re.match(r"(\d{4}-\d\d-\d\dT\S+)", line)
        if not match:
            continue
        stamp = datetime.fromisoformat(match[1].replace("Z", "+00:00")).timestamp()
        if "reading topic '" in line and "from Kafka: false" in line:
            reads.append(stamp)
        if re.search(r"Read \d+ config entries from Kafka", line):
            completions.append(stamp)
    return {"topic_reads": len(reads), "config_completions": len(completions), "sdk_log_lines": sdk_lines,
            "provider_log_lines": provider_lines, "trace_bytes": path.stat().st_size,
            "topic_read_span_s": max(completions) - min(reads) if reads and completions else 0}


def broker_requests(text):
    """Kafka 3.8 RequestChannel logs a JSON object after 'Completed request:'."""
    requests = []
    decoder = json.JSONDecoder()
    for line in text.splitlines():
        if "Completed request:" not in line:
            continue
        data, _ = decoder.raw_decode(line.split("Completed request:", 1)[1].lstrip())
        if data["requestHeader"]["clientId"] == "terraform-provider-kafka":
            requests.append(data)
    return requests


def network_delay(container, milliseconds, env, output, configure=False):
    # Only broker responses on EXTERNAL are delayed; replication/controller traffic stays local.
    script = "apk add --no-cache iproute2 >/dev/null\n"
    if configure:
        script += ("tc qdisc add dev eth0 root handle 1: prio bands 3 priomap 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
                   f"tc qdisc add dev eth0 parent 1:3 handle 30: netem delay {milliseconds}ms\n"
                   "tc filter add dev eth0 protocol ip parent 1: prio 1 u32 "
                   "match ip sport 9092 0xffff flowid 1:3\n")
    script += "tc -V\ntc -s qdisc show dev eth0\ntc filter show dev eth0 parent 1:"
    run(["docker", "run", "--rm", "--cap-add=NET_ADMIN", f"--network=container:{container}",
         NETWORK_IMAGE, "sh", "-ec", script], env=env, output=output, timeout=120)
    text = output.read_text()
    delay = re.search(r"\bdelay ([\d.]+)(us|ms|s)", text)
    require(delay and abs(float(delay[1]) * {"us": .001, "ms": 1, "s": 1000}[delay[2]] - milliseconds) < .001,
            "network delay was not installed")
    if not configure:
        stats = text.split("qdisc netem", 1)[-1]
        packets = re.search(r"Sent \d+ bytes (\d+) pkt", stats)
        require(packets and int(packets[1]) > 0, "no packets passed through the network delay")
        dropped = re.search(r"\(dropped (\d+)", stats)
        require(dropped and int(dropped[1]) == 0, "network delay dropped packets")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--terraform", default="terraform")
    parser.add_argument("--provider-binary", type=Path)
    parser.add_argument("--candidate-provider-binary", type=Path, action="append")
    parser.add_argument("--candidate-sdk-log-level", choices=["WARN"],
                        help="set TF_LOG_SDK=WARN only for candidate contexts")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--topics", type=int, nargs="+", default=[593, 1200], help="topics per cluster")
    parser.add_argument("--parallelism", type=int, nargs="+", default=[1, 10, 30, 50])
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--broker-response-delay-ms", type=int, default=0)
    args = parser.parse_args()
    require(not args.candidate_sdk_log_level or args.candidate_provider_binary,
            "candidate SDK log level requires a candidate provider binary")
    require(args.broker_response_delay_ms >= 0, "negative network delay")
    require(args.runs > 0 and min(args.topics) >= 49 and min(args.parallelism) > 0, "invalid workload")
    os.umask(0o077)
    output = args.output.resolve() if args.output else Path(tempfile.mkdtemp(prefix="kafka-terraform-bench-"))
    output.mkdir(parents=True, exist_ok=True)
    require(not any(output.iterdir()), "output directory must be empty")
    print(f"Artifacts: {output}", flush=True)
    env = {k: v for k, v in os.environ.items() if not k.startswith(("TF_", "KAFKA_", "AWS_"))}
    terraform = str(Path(shutil.which(args.terraform) or args.terraform).resolve())
    version, _ = run([terraform, "version", "-json"], env=env)
    version = json.loads(version.stdout)
    mirror = output / "mirror"
    package = mirror / "registry.terraform.io/mongey/kafka/0.0.1" / version["platform"]
    package.mkdir(parents=True)
    binary = package / "terraform-provider-kafka_v0.0.1"
    if args.provider_binary:
        shutil.copy2(args.provider_binary.resolve(), binary)
    else:
        run(["go", "build", "-o", str(binary), "."], cwd=ROOT, output=output / "build.log")
    logger = output / "kafka-logging"
    run(["go", "build", "-o", str(logger), "./tools/benchmarks/kafka-logging"],
        cwd=ROOT, output=output / "logger-build.log")
    config = output / "terraform.tfrc"
    config.write_text('provider_installation {\n filesystem_mirror {\n'
                      f' path = {json.dumps(str(mirror))}\n include = ["registry.terraform.io/mongey/kafka"]\n'
                      ' }\n}\n')
    env.update(TF_CLI_CONFIG_FILE=str(config), TF_IN_AUTOMATION="1", TF_INPUT="0")
    work = output / "terraform"
    work.mkdir()
    shutil.copy2(ROOT / "tools/benchmarks/terraform/main.tf", work / "main.tf")
    revision, _ = run(["git", "rev-parse", "HEAD"], cwd=ROOT)
    results = {"completed": False, "terraform": version, "provider_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
               "harness_revision": revision.stdout.strip(),
               "harness_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
               "provider_build_info": run(["go", "version", "-m", str(binary)])[0].stdout, "timings": [], "diagnostics": [],
               "timing_instrumentation": "broker request logging at INFO; Terraform trace not requested",
               "provider_profiling": bool(env.get("BENCH_PROFILE_DIR"))}
    results["host"] = {"platform": os.uname().sysname + " " + os.uname().machine,
                       "cpu_count": os.cpu_count(), "load_average": os.getloadavg()}
    contexts = [("baseline", work, config)]
    for index, candidate_path in enumerate(args.candidate_provider_binary or [], 1):
        variant = "candidate" if index == 1 else f"candidate{index}"
        candidate = output / variant
        shutil.copytree(mirror, candidate / "mirror")
        candidate_binary = candidate / "mirror" / binary.relative_to(mirror)
        shutil.copy2(candidate_path.resolve(), candidate_binary)
        candidate_config = candidate / "terraform.tfrc"
        candidate_config.write_text(config.read_text().replace(str(mirror), str(candidate / "mirror")))
        candidate_work = candidate / "terraform"
        candidate_work.mkdir()
        shutil.copy2(work / "main.tf", candidate_work / "main.tf")
        contexts.append((variant, candidate_work, candidate_config))
        results[variant + "_sha256"] = hashlib.sha256(candidate_binary.read_bytes()).hexdigest()
        results[variant + "_build_info"] = run(["go", "version", "-m", str(candidate_binary)])[0].stdout
    results["candidate_sdk_log_level"] = args.candidate_sdk_log_level
    results["broker_response_delay_ms"] = args.broker_response_delay_ms
    results["network_image"] = NETWORK_IMAGE if args.broker_response_delay_ms else None
    cluster_ids = {}
    clusters = []

    def tf(arguments, label, trace=False):
        command_env = variant_environment(env, variant, args.candidate_sdk_log_level)
        if env.get("BENCH_PROFILE_DIR"):
            command_env["BENCH_PROFILE_DIR"] = str(Path(env["BENCH_PROFILE_DIR"]) / label)
        if trace:
            command_env.update(TF_LOG_PROVIDER="TRACE", TF_LOG_PATH=str(output / f"{label}.trace"))
        return run([terraform, *arguments], cwd=work, env=command_env, output=output / f"{label}.stdout")

    def plan(topics, parallelism, refresh, label, diagnostic=False):
        saved = output / f"{label}.plan"
        flags = ["plan", "-input=false", "-no-color", "-detailed-exitcode",
                 f"-parallelism={parallelism}", f"-out={saved}"]
        if not refresh:
            flags.append("-refresh=false")
        _, elapsed = tf(flags, label, trace=diagnostic)
        document, _ = run([terraform, "show", "-json", str(saved)], cwd=work, env=variant_environment(env, variant, args.candidate_sdk_log_level))
        (output / f"{label}.json").write_text(document.stdout)
        fingerprint = validate_plan(json.loads(document.stdout), topics)
        stdout = (output / f"{label}.stdout").read_text()
        for kind, count in (("kafka_topic", topics), ("kafka_acl", 49)):
            for alias in ("first", "second"):
                actual = len(re.findall(rf"^{kind}\.{alias}\[.*Refreshing state", stdout, re.MULTILINE))
                require(actual == (count if refresh else 0), f"wrong Terraform refresh count: {label} {kind}.{alias}: {actual}")
        return {"topics_per_cluster": topics, "resource_count": 2 * topics + 98,
                "parallelism": parallelism, "refresh": refresh, "wall_s": elapsed,
                "sdk_log_level": args.candidate_sdk_log_level if variant != "baseline" else None,
                "state_sha256": fingerprint, "artifact": label}

    def logging(level, label):
        for alias, ports, _, _ in clusters:
            run([str(logger), f"--bootstrap=127.0.0.1:{ports[0]}", f"--level={level}"],
                env=env, output=output / f"{label}-{alias}-logging.stdout")

    try:
        for alias, offset in (("first", 0), ("second", 1)):
            ports = [19092 + offset, 29092 + offset, 39092 + offset]
            cluster_env = env | {f"BENCH_KAFKA_PORT{i+1}": str(port) for i, port in enumerate(ports)}
            cluster_env["BENCH_KAFKA_CLUSTER_ID"] = base64.urlsafe_b64encode(uuid.uuid4().bytes).decode().rstrip("=")
            command = ["docker", "compose", "-p", f"kafka-tf-{os.getpid()}-{alias}",
                       "-f", str(ROOT / "tools/benchmarks/compose.yaml"),
                       "-f", str(ROOT / "tools/benchmarks/terraform-compose.yaml")]
            clusters.append((alias, ports, command, cluster_env))
            run(command + ["up", "-d", "--wait"], env=cluster_env, output=output / f"{alias}-startup.log")
            ids, _ = run(command + ["ps", "-q"], env=cluster_env)
            ids = ids.stdout.split()
            require(len(ids) == 3, "expected three containers per cluster")
            cluster_ids[alias] = ids
            deadline = time.monotonic() + 120
            while True:
                logs = [run(["docker", "logs", container], env=env)[0].stdout for container in ids]
                if all("Kafka Server started" in log for log in logs):
                    break
                require(time.monotonic() < deadline, f"cluster {alias} did not start")
                time.sleep(1)
            if args.broker_response_delay_ms:
                for index, container in enumerate(ids):
                    network_delay(container, args.broker_response_delay_ms, env,
                                  output / f"{alias}-network-{index}-setup.log", configure=True)
        image_info, _ = run(["docker", "image", "inspect", "apache/kafka-native:3.8.0"], env=env, timeout=30)
        image_info = json.loads(image_info.stdout)[0]
        results["kafka_image"] = {key: image_info[key] for key in ("Id", "RepoDigests", "Architecture", "Os")}
        server, _ = run(["docker", "info", "--format", "{{.ServerVersion}} {{.KernelVersion}} {{.Architecture}}"],
                        env=env, timeout=30)
        results["docker_server"] = server.stdout.strip()
        logging("INFO", "startup")
        for variant, work, cli_config in contexts:
            env["TF_CLI_CONFIG_FILE"] = str(cli_config)
            tf(["init", "-input=false", "-no-color"], variant + "-init")
        for topics in sorted(set(args.topics)):
            for _, context_work, _ in contexts:
                (context_work / "terraform.tfvars.json").write_text(json.dumps({"topic_count": topics}))
            variant, work, cli_config = contexts[0]
            env["TF_CLI_CONFIG_FILE"] = str(cli_config)
            print(f"Applying real Terraform fixture: {topics} topics + 49 ACLs per cluster", flush=True)
            tf(["apply", "-auto-approve", "-input=false", "-no-color", "-parallelism=50"], f"n{topics}-apply")
            warmup = plan(topics, 10, True, f"n{topics}-warmup")
            fingerprint = warmup["state_sha256"]
            baseline_work = work
            for variant, work, cli_config in contexts[1:]:
                shutil.copy2(baseline_work / "terraform.tfstate", work / "terraform.tfstate")
                env["TF_CLI_CONFIG_FILE"] = str(cli_config)
                require(plan(topics, 10, True, f"{variant}-n{topics}-warmup")["state_sha256"] == fingerprint,
                        "candidate warmup changed state")
            for trial in range(args.runs):
                order = args.parallelism[trial % len(args.parallelism):] + args.parallelism[:trial % len(args.parallelism)]
                for parallelism in order:
                    for refresh in ([True, False] if trial % 2 == 0 else [False, True]):
                        variants = contexts if trial % 2 == 0 else list(reversed(contexts))
                        for variant, work, cli_config in variants:
                            env["TF_CLI_CONFIG_FILE"] = str(cli_config)
                            label = f"{variant}-n{topics}-p{parallelism}-r{trial}-{'refresh' if refresh else 'no-refresh'}"
                            row = plan(topics, parallelism, refresh, label)
                            row["variant"] = variant
                            require(row["state_sha256"] == fingerprint, f"refreshed state changed: {label}")
                            results["timings"].append(row)
                            print(f"{label}: {row['wall_s']:.3f}s, state={fingerprint[:12]}", flush=True)
            # Separate diagnostic plans avoid charging request/trace logging to timings.
            for variant, work, cli_config in contexts:
                env["TF_CLI_CONFIG_FILE"] = str(cli_config)
                for refresh in (True, False):
                    label = f"{variant}-n{topics}-diagnostic-{'refresh' if refresh else 'no-refresh'}"
                    for alias, _, command, cluster_env in clusters:
                        for broker in range(1, 4):
                            run(command + ["exec", "-T", f"kafka{broker}", "sh", "-c", ": > /opt/kafka/logs/kafka-request.log"], env=cluster_env)
                    logging("DEBUG", label)
                    row = plan(topics, 10, refresh, label, diagnostic=True)
                    require(row["state_sha256"] == fingerprint, f"diagnostic changed state: {label}")
                    logging("INFO", label + "-finished")
                    trace = provider_trace(output / f"{label}.trace")
                    require(trace["topic_reads"] == (2 * topics if refresh else 0), f"wrong provider read count: {label}")
                    require(trace["config_completions"] == trace["topic_reads"], f"incomplete config reads: {label}")
                    if variant != "baseline" and args.candidate_sdk_log_level == "WARN":
                        require(trace["sdk_log_lines"] == 0, f"unexpected SDK logging at WARN: {label}")
                    else:
                        require(trace["sdk_log_lines"] > 0, f"explicit SDK logging missing: {label}")
                    row["provider_trace"] = trace
                    row["variant"] = variant
                    row["clusters"] = {}
                    for alias, _, command, cluster_env in clusters:
                        by_broker = {}
                        by_connection = {}
                        resources = []
                        for broker in range(1, 4):
                            log, _ = run(command + ["exec", "-T", f"kafka{broker}", "cat", "/opt/kafka/logs/kafka-request.log"], env=cluster_env)
                            (output / f"{label}-{alias}-broker{broker}.log").write_text(log.stdout)
                            requests = broker_requests(log.stdout)
                            configs = [r for r in requests if r["requestHeader"]["requestApiKey"] == 32]
                            require(all(result["errorCode"] == 0 for request in configs
                                        for result in request["response"]["results"]),
                                    f"broker returned a config error: {label} {alias} {broker}")
                            by_broker[broker] = len(configs)
                            by_connection[broker] = dict(Counter(r["connection"] for r in configs))
                            for request in configs:
                                resources.extend(r["resourceName"] for r in request["request"]["resources"])
                        require(Counter(resources) == (Counter(f"refresh-topic-{i:04d}" for i in range(topics)) if refresh else Counter()),
                                f"wire topic coverage differs: {label} {alias}")
                        row["clusters"][alias] = {"describe_configs_by_broker": by_broker,
                                               "describe_configs_by_connection": by_connection,
                                               "topic_resources": len(resources)}
                    results["diagnostics"].append(row)
                    print(f"{label}: wire counts verified {row['clusters']}", flush=True)
        if args.broker_response_delay_ms:
            for alias, ids in cluster_ids.items():
                for index, container in enumerate(ids):
                    network_delay(container, args.broker_response_delay_ms, env,
                                  output / f"{alias}-network-{index}-finished.log")
        results["completed"] = True
    except Exception as error:
        results["error"] = str(error)
        raise
    finally:
        cleanup_errors = []
        for alias, _, command, cluster_env in reversed(clusters):
            if not results["completed"]:
                try:
                    run(command + ["logs", "--no-color"], env=cluster_env,
                        output=output / f"{alias}-failure.log", timeout=30)
                except Exception as error:
                    results.setdefault("diagnostic_errors", []).append(str(error))
            try:
                run(command + ["down", "--volumes"], env=cluster_env, output=output / f"{alias}-cleanup.log")
            except Exception as error:
                cleanup_errors.append(str(error))
        results["cleanup_errors"] = cleanup_errors
        (output / "results.json").write_text(json.dumps(results, indent=2) + "\n")
        require(not cleanup_errors, f"cluster cleanup failed: {cleanup_errors}")
    for variant, _, _ in contexts:
        for topics in sorted(set(args.topics)):
            for parallelism in args.parallelism:
                for refresh in (True, False):
                    values = [row["wall_s"] for row in results["timings"] if row["topics_per_cluster"] == topics
                              and row["parallelism"] == parallelism and row["refresh"] == refresh and row["variant"] == variant]
                    print(f"{variant} n={topics} p={parallelism} refresh={refresh}: median={statistics.median(values):.3f}s "
                          f"range={min(values):.3f}-{max(values):.3f}s")


if __name__ == "__main__":
    main()
