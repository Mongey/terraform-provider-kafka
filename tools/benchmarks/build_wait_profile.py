#!/usr/bin/env python3
"""Build a diagnostic provider with a Go overlay; leave Sarama and production source unchanged."""
import json
import shutil
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[2]
output = Path(sys.argv[1]).resolve()
output.mkdir(parents=True, exist_ok=False)
module = subprocess.check_output(["go", "list", "-f", "{{.Dir}}", "github.com/IBM/sarama"], cwd=root, text=True).strip()
shutil.copytree(module, output / "sarama")
broker = output / "sarama/broker.go"
source = broker.read_text()
old = "func (b *Broker) sendAndReceive(req protocolBody, res protocolBody) error {\n\tb.lock.Lock()"
assert source.count(old) == 1
source = source.replace(old, old.replace("\tb.lock.Lock()", '\tbenchStart := time.Now()\n\tb.lock.Lock()\n\tbenchRecordWait("broker_lock", benchStart, req.key())'))
old = "func handleResponsePromise(req protocolBody, res protocolBody, promise *responsePromise, metricRegistry metrics.Registry) error {\n\tselect {"
assert source.count(old) == 1
source = source.replace(old, old.replace("\tselect {", "\tbenchStart := time.Now()\n\tselect {"))
for case in ["case buf := <-promise.packets:", "case err := <-promise.errors:"]:
    # Only instrument this function, not other response handlers.
    start = source.index("func handleResponsePromise(")
    end = source.index("\nfunc ", start + 1)
    body = source[start:end]
    assert body.count(case) == 1
    body = body.replace(case, case + '\n\t\tbenchRecordWait("response_wait", benchStart, req.key())')
    source = source[:start] + body + source[end:]
source += '''
// Diagnostic overlay only; timestamps contain no request names, config values, or credentials.
type BenchmarkWaitEvent struct { Kind string `json:"kind"`; Start int64 `json:"start_ns"`; End int64 `json:"end_ns"`; API int16 `json:"api"` }
var benchmarkWaits struct { sync.Mutex; events []BenchmarkWaitEvent }
func benchRecordWait(kind string, start time.Time, api int16) {
 event := BenchmarkWaitEvent{kind,start.UnixNano(),time.Now().UnixNano(),api}
 benchmarkWaits.Lock(); benchmarkWaits.events = append(benchmarkWaits.events,event); benchmarkWaits.Unlock()
}
func BenchmarkWaitEvents() []BenchmarkWaitEvent {
 benchmarkWaits.Lock(); defer benchmarkWaits.Unlock()
 return append([]BenchmarkWaitEvent(nil), benchmarkWaits.events...)
}
'''
provider = root / "tools/benchmarks/profile-provider/main.go"
main = provider.read_text().replace('"context"', '"context"\n "github.com/IBM/sarama"')
old = '"events": readEvents.Events,'
assert main.count(old) == 1
main = main.replace(old, '"kafka_waits": sarama.BenchmarkWaitEvents(), '+old)
broker.chmod(0o600)
broker.write_text(source)
(output / "main.go").write_text(main)
overlay = {"Replace": {str(provider): str(output / "main.go")}}
(output / "overlay.json").write_text(json.dumps(overlay))
shutil.copy2(root / "go.mod", output / "go.mod")
shutil.copy2(root / "go.sum", output / "go.sum")
subprocess.run(["go", "mod", "edit", "-modfile", str(output / "go.mod"), "-replace", "github.com/IBM/sarama="+str(output / "sarama")], cwd=root, check=True)
subprocess.run(["go", "build", "-modfile", str(output / "go.mod"), "-overlay", str(output / "overlay.json"), "-o", str(output / "provider"), "./tools/benchmarks/profile-provider"], cwd=root, check=True)
print(output / "provider")
