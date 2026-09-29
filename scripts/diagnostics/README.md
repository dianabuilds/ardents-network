# Ardents local diagnostic package

A finite local Docker tool environment plus private process debug mode. Start
with [the owner and observability inventory](../../docs/development/local-diagnostics.md).
Generated evidence, profiles, state and caches always remain outside Git.

## Build and check

From repository root:

```sh
docker build -f scripts/diagnostics/Dockerfile -t ardents-diagnostics:local .
```

Build explicitly installs reviewed Go tools through `make tools-install
DIAGNOSTIC_TOOLS=1`. It downloads build dependencies; diagnostic runs never
implicitly install/upgrade tools. The image contains Go 1.26.8, Staticcheck
2025.1.1, govulncheck 1.1.4, deadcode 0.48.0, Delve 1.27.2, errcheck 1.20.0, strace, ss/tc,
tcpdump and Graphviz. `/opt/ardents-diagnostics/inventory.txt` retains actual
compiler, tool module and Debian package inventory. No product dependency is
added to go.mod. Docker uses the file-specific `.dockerignore`; the ordinary
artifact build context remains unchanged.

PowerShell (create the evidence directory with access restricted to your account):

```powershell
$evidence = Join-Path $env:TEMP 'ardents-diagnostics-evidence'
New-Item -ItemType Directory -Force -Path $evidence | Out-Null
./scripts/diagnostics/invoke.ps1 -EvidenceRoot $evidence -Build doctor
```

Linux:

```sh
umask 077
mkdir -p /tmp/ardents-diagnostics-evidence
export ARDENTS_DIAGNOSTIC_SOURCE="$PWD"
export ARDENTS_DIAGNOSTIC_EVIDENCE=/tmp/ardents-diagnostics-evidence
export ARDENTS_DIAGNOSTIC_SOURCE_SHA="$(git rev-parse HEAD)"
export ARDENTS_DIAGNOSTIC_IMAGE="$(docker image inspect ardents-diagnostics:local --format '{{.Id}}')"
alias diag='docker compose -f scripts/diagnostics/compose.yaml run --rm runner'
diag doctor
```

Always use a **new run name**, e.g. `/evidence/route-20260930-a`; existing capture
paths refuse rather than replacing evidence. The following examples use Linux
`diag`. On Windows pass the same arguments to `invoke.ps1 -EvidenceRoot $evidence`.

## Static, race and profiles

```sh
diag static -out /evidence/static-a -timeout 15m
# Both ordinary Linux and text_worker_installed Staticcheck, plus vet and unchecked-error/type-assertion analysis.
# Full dependency/reachability gates need explicit online access:
docker compose -f scripts/diagnostics/compose.yaml --profile online run --rm online \
  run -raw -out /evidence/vulnerability-a -timeout 15m -- govulncheck ./...

diag test -package ./internal/diagnostics/process -run '^Test' \
  -race -out /evidence/process-race-a -timeout 2m
# One package, explicit pattern; performance profiling is separate from race.
diag test -package ./internal/route -run '^$' -bench '^Benchmark' \
  -profile -raw -out /evidence/route-profile-a -timeout 5m
diag analyze -dir /evidence/route-profile-a
```

Test profile files: `cpu.pprof`, `heap.pprof`, `block.pprof`, `mutex.pprof`,
`runtime.trace`, and the exact `test.bin`. These are sensitive development
artifacts. `analyze` runs offline pprof top tables. Inspect further with:

```sh
docker compose -f scripts/diagnostics/compose.yaml run --rm --entrypoint go runner \
  tool pprof -list 'Closed' /evidence/route-profile-a/cpu.pprof
docker compose -f scripts/diagnostics/compose.yaml run --rm --entrypoint go runner \
  tool trace -pprof=sched /evidence/route-profile-a/runtime.trace
# Redirect that binary output to a private sched.pprof, then use go tool pprof.
```

For an interactive trace/pprof UI, publish only a chosen host loopback port,
select a read-only evidence mount, and run `go tool trace -http=0.0.0.0:8091`
or `go tool pprof -http=0.0.0.0:8091` inside that disposable container. Never
publish profiles to an external service or wildcard host address.

## Live network process debug mode

`ardents` and `ardents-node` accept the explicit `ARDENTS_DEBUG_SOCKET`
environment variable. Without it there are no diagnostic effects. Set it
separately for each process; the absolute socket parent must already exist,
be owned by that process UID with 0700 permissions and contain no symlink.
The socket is 0600, bounded, local-only and removed by joined shutdown.
Use the native Linux `/state` named volume for sockets on Docker Desktop;
Windows bind mounts cannot be assumed to enforce Unix ownership/modes.
No changes to Carrier, Route wire, State authority or confined workers occur.

Run a maintained command/config explicitly supplied by the operator:

```sh
# Build debugging artifacts outside source (debug bytes are not Release bytes).
docker compose -f scripts/diagnostics/compose.yaml run --rm --entrypoint sh runner -c \
  'mkdir -p /evidence/bin /state/node-debug; chmod 700 /state/node-debug; go build -gcflags="all=-N -l" -buildvcs=false -o /evidence/bin/ardents-node ./cmd/ardents-node'
# Config/state fixtures must be deliberately supplied under private /evidence.
diag run -raw -out /evidence/node-run-a -timeout 5m -- \
  env ARDENTS_DEBUG_SOCKET=/state/node-debug/process.sock \
  /evidence/bin/ardents-node node --config /evidence/node-config.json
```

During that run, in another terminal sharing the private evidence mount:

```sh
diag snapshot -socket /state/node-debug/process.sock \
  -kind runtime -out /evidence/runtime-a.json
diag snapshot -socket /state/node-debug/process.sock \
  -kind cpu -seconds 5 -sensitive -out /evidence/live-cpu-a.pprof
diag snapshot -socket /state/node-debug/process.sock \
  -kind goroutine -sensitive -out /evidence/live-goroutine-a.pprof
diag snapshot -socket /state/node-debug/process.sock \
  -kind trace -seconds 3 -sensitive -out /evidence/live-trace-a.out
```

Other kinds: `heap`, `allocs`, `block`, `mutex`. Profiles/traces may reveal
runtime-sensitive information; `-sensitive` is required. The collection is
limited to one profile request, 64 MiB and 30 seconds. Block/mutex sampling
changes scheduling; reproduce latency again without profiling before drawing
performance conclusions. `/runtime` is process observation, not readiness.

## Live panel and metrics

```sh
export ARDENTS_DIAGNOSTIC_RUN=node-run-a
docker compose -f scripts/diagnostics/compose.yaml --profile view up -d dashboard
# http://127.0.0.1:8090 and http://127.0.0.1:8090/metrics
# After diagnosis:
docker compose -f scripts/diagnostics/compose.yaml --profile view stop dashboard
```

Windows: `invoke.ps1 -EvidenceRoot $evidence -Service dashboard -RunName node-run-a`.
The panel polls the safe live snapshot, recent lifecycle/failure events and
process-group RSS chart. It distinguishes running, command failure, timeout and
incomplete evidence; it exposes no raw log/profile files. Fixed metrics have
no Target, peer, request ID, permission or arbitrary error labels. Loss counters
and unrecognized input are visible, including when the product emits no events.

## Debugger, syscalls and network fault diagnosis

Use `-gcflags="all=-N -l"` only for a separate local artifact. Delve accepts
interactive commands: `break`, `continue`, `goroutines`, `stack`, `locals`,
`print`, `disassemble`; do not make its unauthenticated server public.

```sh
docker compose -f scripts/diagnostics/compose.yaml --profile debug run --rm debugger \
  exec /evidence/bin/ardents-node -- node --config /evidence/node-config.json
# Same isolated profile, syscall evidence capped by the collector:
docker compose -f scripts/diagnostics/compose.yaml --profile debug run --rm \
  --entrypoint ardents-diagnostics debugger run -raw \
  -out /evidence/syscalls-a -timeout 2m -- \
  strace -f -tt -T -e trace=network,desc,process /evidence/bin/ardents-node \
  node --config /evidence/node-config.json
```

For loss/delay tests, `network` opts into NET_ADMIN/NET_RAW in **its own**
namespace. A supplied subprocess must start the real local network fixture there:

```sh
docker compose -f scripts/diagnostics/compose.yaml --profile network run --rm network \
  run -raw -out /evidence/delay-a -timeout 5m -- sh -c \
  'tc qdisc add dev lo root netem delay 20ms; go test ./internal/node/forwarding -run "^Test" -count=1; result=$?; tc qdisc del dev lo root; exit "$result"'
```

Run the same selected test without netem as a baseline. A perturbation failure
is diagnostic evidence, not a new product qualification. `ss -s`, `ss -tin`,
`tc -s qdisc show` and `tcpdump -Z diagnostic -i lo -c 100 -s 96 -w /evidence/private.pcap`
are available only with the required isolated privilege. Packet headers still
contain sensitive addresses/timing and ciphertext can remain identifying;
keep pcaps private and explicitly finite. No host networking or Docker socket
is exposed. Kernel/eBPF/perf findings require a declared host/kernel profile;
Docker Desktop is not a substitute for one.

## Existing specialized observations

Run `ardents diagnostics timeline` with the existing bounded stdin interface
for its exact human-readable Node/Source/Endpoint projection. The common package
recognizes those schemas for its own fixed metrics. For isolated Role state,
use the existing `make text-role-durable-state-capture` with its explicit Linux
input/output contract. For heap dumps, use `heapdump-capture`/`heapdump-role-map`
with their existing manifest/profile prerequisites. Installed worker runners
remain on actual qualifying hosts; this container cannot supply systemd
confinement or replace an installed receipt.

Read each run's `manifest.json`, `tools.txt`, `summary.json`, `events.ndjson`
and `samples.ndjson`; `live.json` is transient status. `stdout.log`, `stderr.log`
and `command.json` appear only with explicit `-raw` (static mode retains analyzer
text explicitly). Logs continue draining after saturation, and lost bytes make
the run incomplete. Preserve earlier failed runs. Evidence has no automatic
remote upload or indefinite retention; remove sensitive bundles when their local
purpose ends, and provision a filesystem quota for arbitrary commands/profiles.

## Complete gate environment

Ordinary runner/tests use UID 10001, required by real filesystem-denial tests.
The explicit `gate` profile permits sudo only inside its disposable container
for the existing package-e2e root prerequisite; it is distinct from the default
no-new-privileges runner. It also has online access for advisory checks.

```sh
docker compose -f scripts/diagnostics/compose.yaml --profile gate run --rm gate \
  run -raw -out /evidence/check-a -timeout 2h -- make check
```

Use a fresh cache volume after changing its owning UID; do not recursively
change permissions on host directories. The image includes pinned PowerShell
7.6.6 and Python 3 for actual architecture script checks. The privileged network
profile runs as root solely to apply namespace-local netem and packet capture;
run unprivileged product tests with `setpriv --reuid=10001 --regid=10001 --clear-groups`.

## Find slow tests before changing the checked workload

```sh
diag run -raw -out /evidence/endpoint-timing-a -timeout 20m -- \
  go test -json ./internal/endpoint -short -shuffle=on -count=1 -timeout=15m
diag timings -input /evidence/endpoint-timing-a/stdout.log
```

`timings` ranks package and top-level test terminal durations separately,
retains failures/skips and refuses an incomplete capture. Parent/subtest
numbers overlap; they are not additive. Cold compile time belongs to the
supervisor's total duration, rather than Go test's package execution duration.
Use an explicit affected-owner pattern while editing, then the full gate on
integration. Shared loopback/ledger workloads remain serial. Permission-hour
and declared 256-stream pacing must be preserved by any measured optimization.

The network profile also permits CHOWN/SETUID/SETGID so Debian tcpdump can
create its bounded capture and drop to the diagnostic account. These rights
remain confined to that container; the default runner grants none.

The live transition tail is independent of periodic resource samples.
The latest resource event remains separate, including DRAIN/EXIT states.
Caller interruption has its own fixed metric and never displays as success.
Source inventory rejects special files and shares the whole-run budget.
Profile temporaries are unlinked before sensitive data is written; the first
cleanup failure remains terminal.
