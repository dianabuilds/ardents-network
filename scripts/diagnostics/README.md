# Ardents local diagnostic package

A local Docker tool environment with continuous selected-source logging, a
live log panel, finite diagnostic captures and private process debug mode. Start
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

For finite captures always use a **new run name**, e.g. `/evidence/route-20260930-a`; existing capture
paths refuse rather than replacing evidence. The following examples use Linux
`diag`. On Windows pass the same arguments to `invoke.ps1 -EvidenceRoot $evidence`.

## Continuous logs and live monitoring

Select the actual local command explicitly. Monitoring starts no profile capture,
automatic product test or bug repair. The current live panel covers source and
log-delivery state; measured metric history and local alert lifecycle are still
pending. This temporary log panel is not the final Node administration design.

```sh
# No listener by default; safe JSON events go to console and retained files.
diag monitor -name node -out /state/node-monitor -- \
  /evidence/bin/ardents-node node --config /evidence/node-config.json

# Optional live panel; publish the host port on loopback only.
docker compose -f scripts/diagnostics/compose.yaml --profile monitor run --rm \
  --service-ports monitor monitor -name node -out /state/node-monitor \
  -console=false -container -listen 0.0.0.0:8094 -- \
  /evidence/bin/ardents-node node --config /evidence/node-config.json
```

On Windows use `invoke.ps1 -EvidenceRoot $evidence -Service monitor` with the same
`monitor ...` arguments. The panel is at `http://127.0.0.1:8094/`. Its status reads
the selected supervisor's bounded memory, independently of file delivery. After
source exit it keeps the terminal source result visible and continues periodic
log age pruning until the operator stops monitoring. Pruning failures remain
visible independently. Signal or optional `-timeout` bounds the monitor lifetime. The process
exit, capture timeout and file/console/status failures remain separate; a failed
sink or lost delivery makes the command fail even when the source exited zero.

The output must be a canonical absolute directory outside source, owned by the
process UID with mode 0700. Linux private `/state` is the Docker Desktop default;
do not assume Windows bind mounts enforce Unix permissions. Reuse this selected
monitor directory on restart: owned retained segments count against the same
budget, while per-session counters and row sequence restart at zero. Unknown
entries, symlinks, foreign ownership, a second writer and existing segments larger than the
selected per-file limit refuse without deleting prior files. A preserved incomplete `monitor.json.tmp` also refuses reopening;
retain and inspect the failure before explicitly cleaning its private evidence.

| Flag | Default | Meaning |
|---|---|---|
| `-segment-bytes` | 16 MiB | Maximum segment size; at least the 16 KiB input fragment limit |
| `-retain-bytes` | 1 GiB | Total owned log payload bytes; between segment size and 1 GiB |
| `-retain-files` | 65 | All log-directory files, including the empty ownership lock; 2–128 |
| `-rotate-after` | 15 min | Rotate on the next record after segment age |
| `-retain-for` | 72 h | Maximum segment age; positive and at least rotation age, at most 365 days |
| `-timeout` | 0 | Follow the selected source until signal; optional finite duration up to 24 h |
| `-raw` | false | Explicit private retention of sensitive original stdout/stderr |
| `-listen` | empty | No HTTP listener unless explicitly selected |

The independent status file and its one temporary replacement are each bounded
at 64 KiB outside the log payload budget. File queues contain at most 128 bounded
records, console queues 64; the memory tail retains at most 64 safe rows and 48 KiB of serialized rows. Record projection
can add bytes independently of raw output, so channel-loss counters must not be
summed into a unique lost-source-byte count. Age/count/byte retention expiry is
reported separately from failed delivery. Retention is not a filesystem quota;
provision finite filesystem space for logs, metadata, working space and any
supervised-command artifacts. This store does not promise power-loss durability.

Console output is safe fixed-category JSON. Structured events from stdout and
stderr use the same projection; unknown/oversized lines produce fixed notices,
never arbitrary content. Use `-console=false` when redirecting to a regular file;
the optional console requires bounded pipe/terminal writes. Raw log files are
private engineering evidence and are never served by HTTP. The panel has stream,
category and visible-text filters, view-only pause, separate producer/collector
freshness, source result and independent delivery failures. The visible tail is
not the entire retained file history. Row order is collector observation order,
not proof of cross-process causality or current Node readiness.

HTTP admits at most four open connections, bounded headers/timeouts, loopback
hosts and same-origin browser requests. `-container` permits container wildcard
binding only with an explicitly loopback-published host port. The listener exposes
safe local timing/numeric metadata to other local clients able to reach it; it is
not an authority to access product payloads, keys or another participant's data.

### Engineering verification of sink failures

Use the ordinary diagnostic tests for rotation, restart, foreign-file/symlink
refusal, structured stdout/stderr projection, blocked console, source timeout,
and independent live HTTP health. A real full-disk fixture additionally needs a
selected finite filesystem; a failed setup is an invalid environment.

```sh
# Rebuild the current image first. This fixture contains no product data.
docker run -d --name ardents-monitor-full-disk --init --read-only \
  --cap-drop ALL --security-opt no-new-privileges:true --user 10001:10001 \
  --memory 256m --cpus 1 --pids-limit 32 \
  --tmpfs /tmp:rw,size=16m,mode=1777 --tmpfs /quota:rw,size=64k,mode=1777 \
  -p 127.0.0.1:8095:8095 ardents-diagnostics:local \
  monitor -name full-disk-fixture -out /quota/source -console=false -raw \
  -segment-bytes 16384 -retain-bytes 1048576 -retain-files 64 \
  -container -listen 0.0.0.0:8095 -- \
  python3 -c "for n in range(40000): print('internal-fixture-' + 'x'*240, flush=True)"
```

Inspect `/status` while the source finishes: exit 0 and 10,320,000 stdout bytes;
file failure and lost bytes; continued live response and joined sinks. Confirm
`df -B1 /quota` reports 65,536 bytes used and zero available. Save the projected
status and file inventory outside Git before stopping. After `docker stop -t 8
ardents-monitor-full-disk`, inspect its terminal container state: the monitor
must exit nonzero and retain ENOSPC, even though the fixture exited zero. A full
disk may prevent the status file itself from being saved; the in-memory live
status is deliberately independent. This fixture is engineering verification,
not an operator-facing monitoring check or a product readiness qualification.

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

## Diagnostic assistant

The same explanation is available in the panel and CLI:

~~~sh
diag report -dir /evidence/check-a
diag report -dir /evidence/check-b -compare /evidence/check-a -json
~~~

An explanation distinguishes command failure, timeout, caller interruption,
source change, lost observations and missing evidence. It identifies the first
available explicit owner failure by its local ordinal in the projected
events.ndjson history, keeps cleanup separately, and supplies fixed manual
command templates with prerequisites, budgets and sensitivity. It does not
execute suggestions. Collector order is not causal order; owner UTC is retained
separately when available. The full bounded event file is inspected rather than
only the panel's transition tail.

A report command exits zero when it successfully explains complete evidence,
even if that evidence records a command FAIL. Read its explicit status/exit_code;
report success is never a test, readiness or qualification PASS. Unavailable,
invalid or incomplete evidence returns a nonzero report exit after the safe
explanation. A missing resource sample can be legitimate for a short command;
a missing sample file or mismatch with a terminal summary is incomplete.

Old captures remain readable. Unknown workload, environment, compiler/image
or tool versions are named gaps. New test captures declare race/profiling, and
the collector records its command mode without exposing arguments. Comparison
shows known differences and observed supervisor duration, preserves the earlier
FAIL, and gives no acceleration verdict: existing receipts do not declare the
full workload and environment. Cold compilation and collection overhead remain
inside supervisor duration; use timings for test execution.

The normal dashboard adds this report for its explicitly selected run. To
compare two runs in the panel, stop the existing dashboard occupying port 8090
and explicitly select both directories at startup:

~~~sh
docker compose -f scripts/diagnostics/compose.yaml --profile view stop dashboard
docker compose -f scripts/diagnostics/compose.yaml --profile view run --rm --service-ports dashboard \
  serve -container -listen 0.0.0.0:8090 \
  -dir /evidence/check-b -compare /evidence/check-a
~~~

/report accepts no caller-selected file paths or actions. All file access is
confined to the startup directories, rejects symlinks/special files, and is
bounded. The panel's existing summary and sample routes also re-project stored
records before output; unknown strings/metric fields cannot become visible.
The server remains read-only, with the existing host-loopback publication.

The panel groups outcome, missing observations, suggested checks, RSS/cgroup/PSI,
conditions and comparison. Transition filtering covers only the last 32 events.
Pause suspends panel polling; it does not stop the collector or product.
Manual refresh and copyable command templates do not execute commands.

For a local projected JSON export, open **Предпросмотр отчёта**, inspect its
contents, then choose **Скачать просмотренный JSON**. The preview is a frozen
snapshot and cannot silently change during refresh. Source/image/tool identities
are omitted by default; selecting their checkbox invalidates the preview.
Event UTC, raw logs, paths, command arguments and profiles are excluded. The
export is capped at 256 KiB, downloaded only in the browser, never uploaded.
If browser download is unavailable, **Копировать просмотренный JSON** copies
the same frozen projection, or selects it for ordinary manual copying when
clipboard permission is unavailable. A download request is not a save receipt.
Treat it as private operational metadata even after projection.

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
for the existing package-e2e root prerequisite (including KILL for its
dropped-UID child signal/join); it is distinct from the default
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

The ordinary containers use a finite 4 GiB memory budget; the full `gate`
profile uses 8 GiB for the combined race fixtures. Samples include actual
`memory.max`/`pids.max` and `memory.events:max`; hitting a memory ceiling may
cause reclaim pressure even without OOM. These are container observations,
not a diagnosis of a specific Node or a change to product resource guards.

## Observe one owned Reader Connection

Enable the existing process diagnostics with ARDENTS_DEBUG_SOCKET set to a
canonical absolute Unix socket in an existing directory owned by that process
UID with mode 0700. The first eligible Reader Open in the first ten minutes is
retained in memory, up to 64 fixed-category records. No automatic rearm, Target
selection or product retry is added. Run these commands under the socket's owner:

    diag connection -socket /private/process.sock
    diag connection -socket /private/process.sock -json
    diag snapshot -socket /private/process.sock -kind connection -out /evidence/reader.json
    diag serve -dir /evidence/run-a -connection-socket /private/process.sock

In Docker, mount the selected Linux socket directory explicitly and preserve its
owner UID and permissions. A Windows host TCP address is not a Unix socket.
The default panel does not contact any process. The selected live Reader snapshot
is separate from run-a: process/run association is unproven. The loopback panel
can be read by other local processes/users; its fixed projection still contains
private timing metadata. It is excluded from the command-report export.

Durations of nested JOIN/authentication phases overlap. A missing Context
deadline is displayed as unknown, not zero budget. Untyped cancellation-related
refusals remain failed; context_stop separately reports observed Context
cancellation/deadline. Joined is emitted after owned cleanup, not Application EOF.
Expired/incomplete observations and missing stages remain visible. These facts
neither qualify the installed worker nor prove Network readiness or root cause.

### Periodic observations and event logs

Normal monitoring routes periodic `resource-sample` observations to retained
`samples` files, separately from the event log tail and console. Status exposes
`total_samples` and `latest_sample`; these are observations, not alert decisions
or a complete metric query interface. Sample and event sequences are independent.
Non-periodic resource transitions remain events. File retention and delivery-loss
budgets remain shared and explicitly reported.

## Private reproduction evidence package

`bundle` indexes one completed private capture into a new private JSON file.
Its assembly result is separate from the source command result and capture
integrity. A failed command can have a complete package; unavailable selected
Reader evidence yields an incomplete package and a nonzero CLI result. Existing
outputs are refused and preserved. This is a local engineering index, not a
browser export; its absolute evidence root is private operational metadata.

```sh
umask 077
mkdir -m700 /state/packages
ardents-diagnostics bundle -dir /state/completed-run -out /state/packages/run.json
# Only when explicitly selecting a configured owned Reader process:
ardents-diagnostics bundle -dir /state/completed-run -out /state/packages/with-reader.json \
  -connection-socket /state/reader-private/process.sock
# Existing private artifacts are selected by basename, never auto-discovered:
ardents-diagnostics bundle -dir /state/completed-run -out /state/packages/profile.json \
  -profile cpu.pprof -trace runtime.trace -artifact test.bin -command command.json
```

Both directories must be canonical, private and owned; output must be outside
source and the selected capture. Fixed report inputs are hashed with their finite
collector read limits; optional tools.txt is included when present. The JSON does
not copy raw file contents. A file hash or overlapping times do not establish
operation association. Reader results preserve the existing explicit association
limit and missing stages. Its socket must be an owned private Unix socket in an
owned private directory. This command does not enable debug or profiling on the
process; select those through their existing explicit interfaces.

Select at most eight artifact basenames (ASCII letters/digits/dot/underscore/hyphen,
128 characters maximum), each up to 64 MiB; inventory totals are capped at 128 MiB
including report inputs. Missing, oversized, non-private, linked or observed
changing selected files refuse assembly. Existing fixed report inputs cannot be
selected again. Opaque `-artifact` entries have kind `private-file` and `validation: not-checked`.
Explicit `-profile` and `-trace` use prebuilt standard Go offline parsers
and record `pprof`/`go-trace` plus `validation: passed` or `failed`. No filename
extension supplies that claim. Validation failure returns nonzero and retains a
partial package; parser acceptance does not bind it to a command/executable or
prove useful profile coverage. Raw data stays private in the original capture.

To retain a useful offline table for the debugging implementer, explicitly add
`-profile-top` alongside `-profile`. For example:

```sh
ardents-diagnostics bundle -dir /state/completed-run -out /state/packages/top.json \
  -profile cpu.pprof -profile heap.pprof -profile-top
```

Read each selected artifact's `private_pprof_top` locally. Standard pprof reports
the default sample type, total, and flat/cumulative costs for up to ten rows.
The table is bounded to 16 KiB per profile and may contain sensitive embedded
symbols, build metadata and comments. It is private debug evidence; never upload
it to the monitoring log stream or treat it as safe browser content. Without the
flag, parser output remains discarded. The flag requires an explicit pprof
selection; it does not retain trace-derived profiles, execute the selected
program or enable source/remote symbolization. Empty, invalid UTF-8 or oversized
tables fail selection and retain an incomplete package, without successful
partial text. Existing package-size and time limits still apply.

Check profile type and total before interpreting the table. A quiet CPU profile
may have insufficient samples; parser acceptance and a top function do not prove
a cause of failure. Selected profile/build/operation association remains unproven,
and the original command FAIL is retained independently.

Assembly uses `-timeout` (default 10s, positive and at most 30s); cancellation is
propagated to input hashing, Reader and parser requests. Each parser request has
at most 2s, bounded output and joined process-group cancellation.
Run inside the selected diagnostic container with finite memory/CPU and writable
cache prerequisites. The deadline is cooperative for filesystem I/O; an output
file retained after an I/O/deadline error is not a successful command result.

For the debugging implementer, inspect `run.exit_code`, `run.complete`, `run.gaps`,
`connection_state`, `connection.missing_stages` when present, and each artifact's
kind/validation independently. Keep the original command FAIL. `assembly: complete`
is an index-assembly result, not a passing command or a causal diagnosis. Consult
selected private command receipts manually; their arguments may be sensitive.
A hypothesis such as a cleanup failure is not proven by a nearby resource peak.

Select the exact saved argv receipt with `-command command.json` when the
original collector was explicitly run with `-raw`. This shares the eight-artifact
limit but permits only one command receipt, at most 16 KiB/256 argv string entries.
Null/non-string/NUL arguments, empty executable and invalid UTF-8 are refused.
`reproduction.state` is `available` only when that typed receipt was read;
`argument_count` includes the executable. Arguments remain in the private source
file. `unselected` does not mean the original command is known. A corrupt selected
receipt yields `unavailable`, incomplete assembly and a nonzero CLI result.
Command/run association and a fully replayable environment are not established.

For example, inspect the package locally with `python3 -m json.tool` and manually
read the selected command artifact under its private root. This inspection runs
no tests, commands or bug search. Keep the original FAIL and missing-stage gaps
while forming and verifying a separate hypothesis.

Final gates and bounded review remain required before integrating this slice. Rebuild the diagnostic
image from the current source before expecting its CLI to include `bundle`.

### Standard parser installation

The Docker build includes prebuilt Go 1.26.8 `pprof` and `trace` binaries and
records their build metadata. Native Linux diagnostic tests and bundle parsing
require an explicit preparation step:

```sh
make tools-install DIAGNOSTIC_PARSERS_ONLY=1
```

The destination is `GOBIN` when set, otherwise `GOPATH/bin`; it must be writable
at installation and on PATH during use. This command builds only the selected
standard-library parsers, without third-party tool downloads. Clearing or changing
the Go build cache after installation does not trigger compilation during bundle
parsing. The two-second per-parser and whole-assembly limits remain unchanged.
Missing tools cause failed validation; no automatic installation or passing skip.
### Selected-process memory metrics

The optional monitor panel serves GET /metrics as well as its existing live view
and status. Example inside the explicitly selected local diagnostic environment:

~~~sh
ardents-diagnostics monitor -name node -out /state/node-monitor \
  -listen 127.0.0.1:8094 -sample-max-age 5s -- \
  /evidence/bin/ardents-node node --config /evidence/node-config.json
# GET http://127.0.0.1:8094/metrics from the selected local context.
~~~

Use the actual producer interval to choose -sample-max-age, positive and<=1h;
the example5s is not a universal Node interval. Default0 exports supervisor and
delivery health only and explicitly reports resource_export_enabled0. No
profiling is enabled. Metrics read memory, so failed file/snapshot sinks remain
visible. The endpoint uses fixed unlabelled names under diagnostic_selected_.
Supervisor freshness3s, producer age, source survival and resource observation
are separate signals. Process survival is not product readiness. Stale/stopped
sources omit resource values; absent is not zero. Invalid observations return503
rather than a partial success. Session counter resets and source gaps still need
explicit query handling; do not interpolate through them.

See the local-diagnostics owner for scope/unit catalogue and excluded native
unpopulated RSS/admission and role-specific fields. log_lost_bytes_total and
queue/console loss are delivery failures; log_expired_bytes_total is intentional
retention expiry. Observed sink flags do not require reopening that sink.

The panel's existing Host/origin protections also cover metrics. A collector in
a separate container cannot simply scrape a foreign container Host; do not
disable this guard. Use the separate mutually authenticated collector listener
below before integrating this memory endpoint into the ready backend stack. Source
metadata remains private local engineering data; no raw file/profile access,
product authority or public administration interface is granted.

### Protected collector-only metrics

Provision a dedicated diagnostic CA, monitor server certificate and one selected
collector client certificate outside the repository. The server certificate must
cover the exact IP or DNS name verified by the collector. Do not reuse product
keys. In a native Linux private volume, prepare /state/metrics-certs as owned
0700 and server.crt, server.key, client-ca.crt as owned 0600 regular single-link
files. Windows bind mounts do not prove these Unix ownership/mode requirements.

The selected client's SHA256 pin is the hash of its DER SubjectPublicKeyInfo,
not its Common Name or the hash of its certificate. Compute it with the installed
OpenSSL tool, retaining no private key in the command output:

~~~sh
openssl x509 -in /private/collector.crt -pubkey -noout |
  openssl pkey -pubin -outform DER |
  openssl dgst -sha256

ardents-diagnostics monitor -name node -out /state/node-monitor \
  -console=false -container \
  -metrics-listen 0.0.0.0:9443 -metrics-certs /state/metrics-certs \
  -metrics-client-pin <CLIENT_SPKI_SHA256_HEX> -sample-max-age 5s -- \
  /evidence/bin/ardents-node node --config /evidence/node-config.json
~~~

Keep the listener in the selected internal Docker network; do not publish a
wildcard host port. Configure the scraper with HTTPS, the independently selected
server CA, the client certificate and private key, and server_name matching the
certificate. Leave certificate verification enabled. The three monitor flags
are required together. The endpoint grants GET /metrics only and uses TLS 1.3;
all other paths are unavailable. The ordinary optional panel remains separate.

Certificate/pin rotation requires monitor restart. Expired clients are refused,
including requests on retained TLS connections. On source exit the observation
remains available until the monitoring command is cancelled. Cancellation joins
the listener; listener failure remains visible separately from source and file
sink health. No debug capture is enabled by scraping.

These are monitor connection instructions, not a qualified ready-stack deployment
or an installed Node acceptance claim. Build the updated diagnostic tool before
using new flags; an older installed helper image does not contain this change.

The `/metrics` catalogue also exposes `diagnostic_selected_log_retained_bytes`,
`log_retained_files`, `log_retention_limit_bytes`, `log_retention_limit_files`
(all prefixed `diagnostic_selected_`) and
`diagnostic_selected_log_storage_observation_available`. These are fresh local
payload-accounting observations/configuration; file count includes the lock.
Stale or unconfigured storage has availability0 and no current quantities.
The existing `diagnostic_selected_log_expired_bytes_total` is intentional
retention expiry for the current session, separately from lost delivery.
Reopening preserves retained file/byte inventory but resets session counters.
Physical disk allocation and continuously enforced filesystem quotas are outside
these metrics.
