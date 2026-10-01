# R-171 local observability experiment

Question and predeclared falsification criteria:
[research owner](../../docs/research/records/r-171-local-observability-stack.md).

Hypotheses: a single Alloy pipeline or a single OTel Collector pipeline supplies
safe searchable logs and correct metric/alert history with less maintained code.
Neither is accepted if privacy, failure, resource or maintenance requirements fail.

## Current boundary: inspect exact image candidates

These files are disposable research tooling, not a maintained monitoring stack.
The Linux/amd64 manifest digests were read from the official repositories on
2026-10-01. Tags are provenance; pulls use immutable platform manifests.
`images.json` is not an admission receipt. Source licenses do not establish image
closure licensing/security. Preserve failed attempts and do not launch candidates
with product/private inputs before their closure and configuration are reviewed.

From this directory on the selected Windows Docker Desktop host:

```powershell
make tools-install EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-images-a
```

Use a new absolute directory outside Git for every attempt. The explicit install
command downloads public images into Docker's image cache, records pull outcomes,
checks platform/digest identity and saves config digests, defaults and sizes. It
creates no containers, listener, volume or collector. No automatic tool download
on a test path. Preinstalled Docker, PowerShell and make are prerequisites.

Docker image-cache footprint is distinct from the predeclared probe-state and
private evidence budgets; admission must inventory it as installation overhead.
No raw Node logs, profiles, credentials, authority roots or Docker socket enter
this inspection. Upstream identity is not a guarantee against supply-chain compromise.

## Next measured boundary

Complete exact image binary/OS/transitive licensing and advisory review. Then run
one finite synthetic source with backend ports isolated, real authenticated query
access, external egress blocked, reporting disabled, explicit memory/CPU/process
budgets and sized state. Compare one collector at a time with equivalent input.
A volatile tmpfs first probe cannot prove persistent retention or crash durability.

Capture searchable events, metric units/type/scope/reset/gaps, threshold and
source/collector-loss pending/firing/recovery, silence expiry, rotation/restart,
backend outage, storage pressure, rejection/loss and resource measurements.
Use safe synthetic sentinels; no actual Application data. A process staying alive
or a static dashboard is not the required result. Keep originals outside Git and
record measured results and disposition in R-171.

## Result and disposition

Initial isolated runtime attempts are now recorded below. Image installation and the initial exact-binary scan completed. Findings remain under applicability review; JSON exit zero is not security acceptance. Prometheus was changed from the first 3.13.3 input to 3.15.0 after comparing actual scan fix floors with the newer exact-tag dependency manifest. Initial receipts and scans remain preserved. The inspected source-release candidate OTel Contrib 0.162.0
was not found at either documented-family Docker Hub or GHCR paths during initial
manifest inspection. Preserve that negative observation; do not silently substitute
a different release or treat it as rejection of all OTel Collector configurations.
The next step is to verify publication/source-build alternatives against the same
support, closure and experiment requirements. The five pinned images cover the
Alloy candidate only. No new Ardents package, SDK, protocol or administration
power is selected by this experiment.
## Synthetic runtime inputs and isolated pipeline

`fixture.py` owns one finite synthetic process: 600-second lifetime, a 60-second
queue-pressure interval, structured events and three explicitly typed metrics.
It serves only /metrics, exits on SIGTERM/SIGINT and joins its HTTP worker.
Fixture queue values are declared test inputs, not measured Node workload.
The planned input volume is private and sized; only its events file is available
to the collector. No arbitrary log root or host process namespace is admitted.

`prometheus.yml` scrapes explicit fixture and collector addresses; `alerts.yml`
contains pending windows for fixture threshold, scrape loss and collector loss.
`alertmanager.yml` has no notification integration. These are not durable
acknowledgement or incident-history features. Current syntax and all three rules
were checked with the exact candidate promtool, network disabled.

`config.alloy` tails only the synthetic file, drops the filename label and writes
to local Loki with verified client TLS. `loki.yml` requires a verified client
certificate and confines gRPC to its container loopback. Setting auth_enabled to
false selects one tenant; it does not disable the configured transport-level
client authentication. Certificates, permissions, listener isolation, egress,
configuration validation and real failure behavior still require the runtime
harness. No default insecure launcher is provided. Alloy launch must explicitly
set a non-root UID, disable usage reporting and support bundles, and use a finite
sized state directory. Neither config is a privacy or lifecycle acceptance proof.
### Verified preparation and fixture (2026-10-01)

The fixture was run in a container with no network or host ports, 128 MiB memory,
0.25 CPU, a 16-PID cap and private 16 MiB tmpfs, as UID/GID 10001. Typed metric
queries observed the event counter increase from 17 to 97; neither query occurred
during queue pressure. Structured logs recorded thirty pressure events. SIGTERM
joined the HTTP worker and exited zero. This is not backend delivery evidence.

All four configuration validators accepted the current files. The Prometheus
rule evaluator passed assertions for pending, firing and recovery for each of
three rules, including separate fixture and collector scopes. It requires a
writable bounded test-store /tmp even with a read-only root. On the selected
PowerShell host, pass dotted Loki CLI flags as quoted strings. Original failed
preparations are retained alongside corrected results. The full five-service
pipeline, private TLS credential preparation and Grafana view remain unverified.

### Explicit synthetic pipeline probe

After explicit image installation, from this experiment directory:

```powershell
make probe EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-pipeline-new RUN_NAME=r171-pipeline-new
```

Use a fresh external evidence directory and unique run name. The launcher creates
private role credentials offline, starts only the pinned backends on an internal
Docker network without host ports, then starts a finite synthetic source. No Node
or Application inputs enter this probe. Grafana backend plugins use a bounded
private /tmp for Unix sockets; only Prometheus and Loki bundled datasource
backends are enabled. The Grafana PID limit includes their Go threads.

Query observations are saved privately and assert actual synthetic queue values,
scoped log delivery and the pressure alert reaching Alertmanager, in addition to
datasource health and anonymous-access refusal. A zero exit from an earlier
HTTP-only probe does not establish these assertions. Docker internal-network
configuration alone does not prove complete external/DNS egress denial.

Finally the launcher saves bounded service logs and removes containers/network.
The named finite tmpfs fixture volume remains; retain its identity and remove it
explicitly after evidence disposition. This first volatile probe does not prove
retention, restart durability, silence expiry, recovery history, source/collector
outage behavior, shared-interval rendering or the five-minute resource target.

### Preserved runtime corrections

Attempts A-E did not complete: child PowerShell ACL cmdlet availability, nested
read-only bind-mount preparation, plugin Unix-socket /tmp and insufficient Grafana
thread budgets were corrected. F completed its HTTP observations but contained
zero Loki records and only a pending pressure alert, with no Alertmanager receipt.
Its successful exit is not log/alert acceptance. Alloy had encountered a missing
file before source startup; periodic discovery of that exact selected file now
handles later creation. The probe now waits through the declared pending window
and rejects missing log delivery or firing receipt. Original private reports in
ardents-r171-pipeline-a through -f remain preserved. Full findings and admission
limits belong to the research record.

G passed strengthened content assertions: normal queue 0 with 12 scoped records; pressure queue 20 with 44 records and firing receipt in Alertmanager. Both Grafana datasource health checks passed and anonymous accesses were refused. This short volatile journey is not full stack admission; remaining boundaries above still apply.


The current probe also observes a twelve-second scoped silence and its expiry,
threshold clearance, separate source and collector pending/firing/clearance
journeys. It freezes/resumes the selected synthetic processes with Docker pause;
this is not a stop/restart or persisted-state test. Per-attempt query reports are
retained privately. The lifecycle-a run passed all nine transition assertions.
The 600-second finite fixture permits a future five-minute resource measurement;
no such resource-window acceptance follows from the short lifecycle result.

Post-outage catch-up now snapshots the producer counter and requires every
sequence through that watermark in Loki. The native Alloy timestamp stage uses
valid fixture event time; the probe checks the actual stored nanosecond timestamp
against the producer's RFC3339 value. Timestamp parse failure uses skip and must
not be presented as valid source time for unsupported inputs. The event-time-a
run passed catch-up without duplicates in the selected query and returned actual
Grafana metric/log frames for one explicit 180-second interval, including a
12000 ms metric gap during source freeze. API frames do not prove visual rendering.

Bundled datasource binaries require their own dependency review. The inspected
server image contains Prometheus plugin13.2.1 and Loki plugin13.2.0, both built with
Go1.26.7. Actual binary scans reported grpc and openpgp findings; see the R171
record. No admitted replacement or blanket local-only exception is claimed.
Do not overlap the stack/query probe with a public scanner configured above
0.5 CPU; shared-a's initial one-CPU scanner overlap is not resource acceptance.
### Explicit backend restart and healthy resource window

Use fresh external evidence roots and unique run names:

```powershell
make probe-restart EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-restart-new RUN_NAME=r171-restart-new
make probe-resources EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-resources-new RUN_NAME=r171-resources-new
```

The restart overlay requires Docker Compose !override support (tested 2.40.3).
Five sized tmpfs state volumes stay mounted by a read-only idle helper while
backend containers restart. This preserves RAM state across that container
restart only; daemon/host restart or final unmount loses it. No host files,
Docker socket or product authority are mounted. Cleanup removes containers and
network but preserves volume identities for explicit later disposition.

The resource profile lowers and verifies aggregate selected-container CPU caps
to 0.96 core, then samples for at least five minutes with no concurrent query
helper. Process RSS includes Grafana plugin children; snapshot CPU and sampled
RSS peaks exclude host/Docker overhead. Subsequent native queries reject source
unavailability, scrape gaps or nonzero fixture queue during the healthy window.

Restart-a preserved selected metric/log history, silence and collector checkpoint
progress. Resources-a passed 303.16 seconds/37 samples with peak RSS 902.03 MiB
and mean sampled CPU 2.223 percent of one core. Actual backend-state bytes,
storage pressure, backend outage and daemon/host durability remain unverified;
this is not component admission or real Node capacity evidence. Advisory review
also distinguishes stripped-binary scanner placeholders from actual call reachability.
The optional `probe-backend-outage` target uses the restart profile, stops only
Loki, requires actual retry/drop counters while source/collector remain up, then
starts Loki and checks catch-up through a fixed producer watermark. Failure or
missing counters terminate the probe; absent counters are never measured zero.
Before cleanup the restart profile also samples each backend's allocated file
bytes and filesystem used/capacity through the read-only mount holder. This is
a live, non-atomic sample; Grafana /tmp, fixture state and Docker logs are separate.
Backend-outage-a completed with cleanup zero: unavailable Loki, fixture/collector
both up, retry increase one and drop increase zero; recovery delivered all 141
sequences through the fixed watermark without duplicates in that query. Actual
backend-state files used 1789952 bytes; this excludes other tmpfs/log/cache space.
Extended failure, exhausted retries, storage pressure and host durability remain
open. Original evidence is external; no maintained component admission follows.
### Explicit filesystem pressure

```powershell
make probe-storage-pressure EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-storage-new RUN_NAME=r171-storage-new
```

This profile fills only its new Loki state volume with one injector-owned file
until actual ENOSPC, then observes native WAL failures and a scoped alert while
source/collector/backend scrapes remain available. The already-installed offline
injector has 640 MiB memory / 0.25 CPU and no network; aggregate limits remain
within the research envelope. It refuses an existing injection file and removes
only its validated regular single-link owned file. No other state is removed.
The fixed-volume scope is an intentional fault injection, not normal monitoring.

Prometheus scrapes only three selected WAL metrics from authenticated Loki with
a 32-sample budget. A 30-second increase window plus six-second pending period
reports observed WAL storage failures; the counter is not a count of lost logs.
Counter silence/recovery does not establish historical durability. The probe
checks native alert clearance and source-event catch-up after space recovery;
missing history or unavailable counters remain a failed result. This profile
must not be used against an existing Node or production storage volume.
Storage-pressure-a observed actual ENOSPC / zero filesystem free bytes, native
WAL failure increase two and a firing Alertmanager receipt while all three
scrape sources remained available. The injector freed only its validated file;
the alert cleared and log catch-up plus subsequent backend restart checks passed.
Collector retry/drop values during this pressure interval
were not separately retained, so full pressure admission is
still incomplete. This is not a crash-durability or real Node acceptance claim.
Correction: pressure-a raw rules reports 06 through10 already contain pending;
report11 contains firing. The new probe explicitly requires that progression and
actual registered retry/drop counter deltas, refusing absent counters, resets or
loss. Its receipt records exact installed image identities and SHA256/size for
fourteen selected source/configuration inputs at startup, then checks those same
bytes again before success. These are two observed snapshots, not atomic source
isolation or protection against an owner changing and restoring files between them.
The strengthened pressure-b probe observed pending and actual retry/drop values,
but failed later catch-up after concurrent backend/sender restart: event157 was
missing through producer watermark183. Its nonzero receipt is retained. Planned
restart now drains/stops Alloy with Loki available, requires a clean non-OOM exit
within thirty seconds, restarts backends and waits for readiness before starting
Alloy. Source production continues in its bounded selected file. Full catch-up
assertions remain required. This is a planned lifecycle change, not a crash
recovery guarantee. Input hashes are also checked and retained on failed runs.
Pressure-c passed the strengthened native pending/firing/clearance and delivery
counter checks (+4 retries during firing, +5 at clearance, zero observed drops).
Ordered sender drain exited0 without OOM; post-restart catch-up covered watermark
180 with181 unique events without query duplicates. Its source/config snapshots
matched. This qualifies this bounded planned restart probe only; pressure-b's
original loss remains evidence, and crash/extended-outage admission remains open.
### Explicit alternative collector inspection

```powershell
make tools-install COLLECTOR=otel EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-otel-public-new
```

This explicit path downloads the exact official0.162.0 public archive/SBOM,
verifies published byte counts and SHA256 values, extracts only the selected
regular binary member (512MiB cap), and inspects buildinfo/ELF with preinstalled
bounded offline tools. The target executable is not run. Archive paths are never
extracted; existing roots/output are refused. Each download has a120-second
cooperative wall check and20-second network read timeout. All artifacts remain
external; matching published hashes are not signature or admission proof.

Observed binary size407498914bytes, Go1.26.8; no INTERP/DYNAMIC program headers.
Actual binary scan has31finding records across7advisories, all dispositions
pending. The published SPDX inventory has1080packages. Neither package count nor
binary size establishes runtime RSS, reachability, security or a collector choice.
Use the research owner for exact identities and equivalent H2 probe prerequisites.
`otel.yml` is an H2 synthetic investigation configuration. Its initial version
passed offline validation; the current profile adds a native Prometheus reader
and uses the current file_log receiver name. Exact lifecycle/resource evidence
and unresolved admission requirements belong in R-171. Never use this file
against product data or treat `validate` success as an admitted monitoring stack.

### Alternative collector runtime comparison

```powershell
make tools-install COLLECTOR=otel-image ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-otel-public-a EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-otel-image-new
make probe-otel EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-otel-pipeline-new RUN_NAME=r171-otel-pipeline-new
```

The first command verifies and copies only the previously inspected exact binary
into an external scratch-image context; no base image, OS packages or tools are
fetched. The installed linux/amd64 image ID is recorded and passed directly to
Compose. The service named alloy remains only an orchestration alias in this
research profile; its actual image is OTel, and Alloy is not also started.
Persistent queue/offset state uses the same finite retained RAM volume while the
anchor exists, with the same limits and host-reboot limitation as H1.

The H2 profile uses the same fixture, alert/source/collector-loss journey,
catch-up/source timestamp, Grafana shared-interval and ordered restart checks.
Prometheus retains only fixed native OTel counter/gauge families with the same
256-sample cap. Accepted/sent/refused values count log records; exporter queue
size/capacity use serialized bytes because the queue sizer is explicitly bytes.
Accepted records are not file byte positions or proof of final delivery. Missing
failure counters remain unavailable, not assumed zero or renamed retries.
The storage-pressure flag still refuses H2 pending its native failure/loss
measurement. The backend-outage profile below uses separate OTel semantics.

Loki explicitly ignores default resource index labels and permits only job as an
index label; selected safe event fields remain structured metadata. The probe
uses the Series API to verify index labels: query_range response labels may also
include structured metadata and cannot alone prove index cardinality. Parser
drops, malformed input, extended outage, crash and state saturation remain
separate required failure evidence before this configuration can be selected.

### Native OTel backlog failure journey

```powershell
make probe-otel-backend-outage EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-otel-backend-outage-new RUN_NAME=r171-otel-backend-outage-new
```

This research profile captures accepted/sent record counters and a drained byte
queue, stops its own Loki container, then observes source/collector up1, continued
acceptance, stalled sent count, byte backlog, and the native backlog rule through
pending/firing with an Alertmanager active receipt. It restores Loki and requires
complete source-time sequence catch-up, a drained queue and alert clearance before
ordered restart/history checks. Failure-series absence is retained as null, not
zero. Counter resets/disappearance, an observed terminal send/enqueue failure or
missing producer sequence refuse the successful-recovery assertion.

The backlog rule means sustained nonempty queued serialized bytes, not a proof of
permanent loss or a general Node health verdict. The exact exporterhelper chain
wraps the retry sender with operation observation: send_failed counts records
whose export operation returned error after that processing, not every retry.
An enqueue failure is also not automatically permanent loss when the receiver
retries the rejected batch. Persistent-queue shutdown has an additional exception:
a shutdown-marked operation can retain its item for restart even while failure
telemetry records an operation error. Final sequence coverage and the exact error/
lifecycle context are required to diagnose loss. Extended retry exhaustion,
partial delivery, queue/state saturation and crash persistence remain open.

The ready Grafana probe view separates observation age (seconds), pending/firing
rule history and scrape availability from event messages. Events use query-time
JSON formatting; Explore retains field search and original record inspection.
Missing alert series alone do not prove recovery. The shared-interval API probe
covers these expressions, but actual browser rendering remains a separate gate.
Private generation supplies a separate random Grafana secret key via a file;
keep it with the corresponding private state across the selected restart.

### Official minimal Collector build prerequisite

```powershell
make tools-install COLLECTOR=otel-builder EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-ocb-public-new
```

This downloads one exact public OTel Builder artifact and inspects its checksum,
buildinfo and ELF metadata without executing it. Signature/admission are not
implied. `builder.yml` proposes the three official components needed by H2 and
one local file configuration provider. It has not generated or built a Collector;
a short manifest does not establish actual dependency closure or runtime fit.
Generated sources/module/cache/binary must stay outside Git. Exact tool scan and
next-build falsification criteria belong in R-171. This is not a second maintained
Go module, a new Collector implementation or permission to ingest product data.

### Build the minimal official-component investigation

```powershell
make tools-install COLLECTOR=otel-minimal ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-ocb-public-a EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-minimal-build-new
```

Requires the explicitly installed exact Builder and inspection-helper image.
Verifies Builder bytes again before execution. Uses one non-root read-only Docker
container, no capabilities,3GiB memory/2CPU/pids128,600s Builder deadline,
64MiB executable temporary directory only for the checked Builder and2GiB
non-executable module/build/scratch cache. Go1.26.8 is already installed;
GOTOOLCHAIN=local refuses automatic toolchain download. Public module downloads
use the Go proxy/checksum service; no product inputs or authority roots are mounted.
Generated source/module/sums/binary/log stay in the fresh external evidence root.
The128MiB retained-output check is a post-build assertion, not a disk quota during
compilation. Build-cache ceilings and overhead are separate from runtime monitoring
budgets. Actual component/dependency/advisory closure and H2 runtime checks are
required after assembly; successful compilation is not maintained admission.

### Minimal Collector runtime comparison

```powershell
make tools-install COLLECTOR=otel-minimal-image ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-minimal-build-d EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-minimal-image-new
make probe-otel-minimal-backend-outage EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-minimal-pipeline-new RUN_NAME=r171-minimal-pipeline-new
```

The installer accepts only the exact corrected D binary size/SHA256 and copies it
into a scratch context outside Git, rechecking the copy. No base image or package
is fetched. Minimal and full variants use separate image tags; the minimal probe
selects immutable image config ID7c345035ed2941c4df2f0e95dedcf3209297ca5ef2df8b297694584c4a0a6fc1,
records minimal_distribution=true and uses the same H2 configuration/semantics.
The internal /otelcol-contrib executable path and Compose service alloy are only
compatibility aliases of this research orchestration; actual receipt identifies
the selected minimal binary/image. They are not product runtime identities.
Build/configuration inputs extend the probe snapshot; original full-variant
commands remain available. Storage-pressure minimal H2 is still refused pending
its native loss/failure evidence. This profile is not maintained admission.

### Patched datasource artifact inspection

```powershell
make tools-install COLLECTOR=grafana-plugins EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-patched-plugins-new
```

Downloads exact public linux/amd64 plugin archives from official releases, checks
published SHA256/length, and inspects bounded ZIP entries without unpacking a
whole plugin tree. Rejects unsafe names, links/encryption, excessive inventory or
expansion. Extracts only one bounded ELF backend and selected manifest metadata;
backends are not executed, registered or admitted. Root manifest signatures are
retained but not verified. Read R-171 for actual binary scan results and pending
advisory/source/license/support/compatibility decisions. Newer release does not
automatically close known findings, and a clean scan is not security admission.

### Exact Loki source and artifact dependency reconciliation

Prepare exact-commit go.mod/go.sum from the official repository in a fresh external
primary root. Use the already checked plugin archive inspection output:

~~~powershell
make tools-install COLLECTOR=loki-source ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-loki-source-primary-a PLUGIN_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-patched-plugins-a EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-loki-source-graph-new
~~~

The explicit installation route uses the installed helper, public module proxy
and checksum service; no automatic toolchain download or plugin execution.
One non-root read-only container has2GiB/1CPU/pids64 and a1GiB temporary module
cache. Exact source sums must match primary bytes. Analysis fixes Linux/amd64,
cgo off and arrow_json_stdlib to match the inspected artifact. It verifies the
actual pinned backend hash, reads its build information without executing it,
and reconciles effective dependency versions/checksums including replacements.
Raw graph and a completion receipt stay outside Git. Graph output32MiB is a
post-command assertion, not an on-disk quota; each download/list command is
bounded180s. Package absence is source evidence, not signed/reproducible artifact
admission; see R-171 for the precise finding disposition and invalidation scope.

### Updated signed datasource compatibility profile

~~~powershell
make tools-install COLLECTOR=grafana-plugin-trees ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-patched-plugins-a EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-plugin-trees-new
powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot C:/Users/vitek/AppData/Local/Temp/ardents-r171-patched-runtime-new -RunName r171-patched-runtime-new -Collector otel -MinimalCollector -RestartProbe -PatchedPluginRoot C:/Users/vitek/AppData/Local/Temp/ardents-r171-plugin-trees-new
~~~

Staging rechecks exact public archive identities, bounded inventory/expansion,
safe paths and complete manifest file hashes. No plugin runs during staging.
The opt-in Compose override mounts those trees read-only and loads supported
external core versions. Grafana's signature verifier remains enabled using its
built-in key; outbound key retrieval remains disabled. No unsigned loading,
development mode or arbitrary host log discovery is enabled. Runtime asserts
exact versions and valid Grafana Labs signatures through authenticated plugin
settings after the shared-interval query. Start/end tree hashes must match.
The finite synthetic profile also checks alert/source/collector lifecycle,
catch-up and ordered restart. It does not prove browser rendering, real process
integration, updated resource overhead or maintained dependency admission.

### Finite rendered browser profile

Install the temporary standard-library Windows relay explicitly from this directory:

~~~powershell
make tools-install COLLECTOR=browser-relay EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-browser-relay-new
powershell -NoProfile -ExecutionPolicy RemoteSigned -File probe.ps1 -EvidenceRoot C:/Users/vitek/AppData/Local/Temp/ardents-r171-browser-new -RunName r171-browser-new -Collector otel -MinimalCollector -PatchedPluginRoot C:/Users/vitek/AppData/Local/Temp/ardents-r171-plugin-trees-a -BrowserProbe -BrowserRelayBinary C:/Users/vitek/AppData/Local/Temp/ardents-r171-browser-relay-new/browser-relay.exe
~~~

The recipe requires the exact checked relay hash recorded in probe.ps1. A changed
source/toolchain or build identity requires inspection and an explicit pin update;
a fresh build is not automatically accepted. Generated executable and evidence
stay outside Git. The relay is build-ignored experimental code, not a maintained
package or proposed monitoring service.

The profile retains native delivery checks, explicitly freezes the synthetic
source for12seconds, then verifies catch-up, shared query intervals and plugin
identity before opening a browser window. It excludes full lifecycle/restart,
backend/storage pressure and resource measurement; those have separate profiles.
Never interpret this profile's successful exit as completion of those profiles.

Open the browser-ready.txt URL and use the private generated Grafana login.
Inspect actual rendered metric gaps, legends/units, logs and expanded metadata.
Record observations separately under reports/browser-observation.json. To release
the finite window, write browser-done.txt in the same private evidence directory.
This marker is an operator release, not a machine assertion of UI correctness.

The host relay binds only127.0.0.1:8098, has a300second lifetime, at most8
concurrent streams and30second idle deadlines. Its only destination is
grafana:3000 through the explicitly named synthetic browser-tunnel container.
That helper has128MiB/.1CPU/pids64, no host mounts or capabilities and remains
on the existing internal network. The probe owns and stops the relay before
Compose teardown. No daemon configuration, public egress or unsigned plugin
loading is enabled. Loopback access still requires Grafana authentication and
the intact local host/daemon trust assumption. Relay overhead is not qualified
by earlier resource measurements.

Read R-171 for retained Docker port-publication and initial browser chunk failures,
the successful warm-cache render, and remaining cold-load/alert presentation
limitations. This remains synthetic research evidence, not real Node monitoring.

### Selected monitor metric mapping prototype

monitor_metrics.py accepts one explicitly supplied safe monitor JSON snapshot on
stdin, at most64KiB, and writes fixed unlabelled Prometheus exposition. It does
not sample, discover processes, open files or sockets, forward raw logs or admit
a monitoring backend. Require an explicit --sample-max-age in seconds from the
selected producer interval. An external caller must bind the selected job; PID,
source name, arbitrary JSON keys, command arguments and event tail never become
metric labels or HELP text.

The current common catalogue exports cgroup cumulative CPU microseconds as
seconds and cgroup current memory. Both the extended process sampler and the
closed owner-cgroup sampler populate these fields. The event does not declare
measurement coverage: extended Go/process/socket/PSI/event-counter fields may
be unpopulated defaults and are excluded, even when present in JSON. RSS,
admission, timers, queues, storage and Hosting Usage need separate verified
scope/availability semantics. Actual accepted-Node attempt-e exposed this gap;
earlier prototype checks did not qualify the closed producer path.
Supervisor freshness is independently bounded3seconds. Stale supervisor omits
the source-alive and resource metrics; a stopped process omits resource metrics.
Fresh receipt time cannot refresh an old producer timestamp. Missing fields
remain absent, actual measured zero remains zero, future/invalid timestamps and
invalid numeric values refuse. source_process_alive is survival, not readiness.
session_started_seconds exposes a supervisor reset boundary; upstream counter
and session resets plus observation gaps still require explicit query handling.
This stateless mapper alone does not implement gap-safe rate calculations.

Run behavior checks with the installed helper, no network or writable source:

~~~powershell
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 256m --cpus .5 --pids-limit 32 --mount "type=bind,source=C:/Users/vitek/.codex/worktrees/local-diagnostics/ardents-network/experiments/r-171-local-observability,target=/probe,readonly" --workdir /probe --entrypoint python3 sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad -B -m unittest -v test_monitor_metrics
~~~

The seven checks cover scope/units, unpopulated and sensitive-field exclusion,
stale supervisor, replayed producer time, stopped source, absent versus zero,
invalid observations and session identity. They use fabricated safe snapshots,
not live-process integration evidence. Real safe snapshot ingestion, scraping,
log rotation delivery, alert journeys and actual Node/debug correlation remain
the next integration boundary after exact component selection.
CLI checks additionally exercise oversized/malformed/invalid UTF-8/deeply nested stdin and verify fixed refusal without input echo; the suite now contains ten checks.

### Exact Prometheus source graph inspection

Prepare exact-commit go.mod/go.sum from upstream5241a27fe3c6983549fccc32f6e65917408c63cd
in a fresh external PrimaryRoot, and retain the pinned image binary/buildinfo
from the actual public inspection above. Then explicitly run:

~~~powershell
make tools-install COLLECTOR=prometheus-source PRIMARY_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-prometheus-source-a ARTIFACT_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-prometheus-315-scan-a EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-prometheus-graph-new
~~~

The installed helper downloads the exact public commit module through the Go
proxy/checksum service. GOTOOLCHAIN=local prevents automatic toolchain downloads.
GOWORK=off selects that root module; public module archives omit nested workspace
modules. Source go.mod/go.sum must match upstream bytes. Linux/amd64, cgo0,
netgo/builtinassets and ./cmd/prometheus match the inspected entrypoint/tags.
A read-only non-root container has4GiB/1CPU/pids64 and a3GiB temporary cache.
Each download/list command is bounded180seconds. Retained graph32MiB is a
post-command assertion, not a disk quota. Source and artifact mounts are read-only,
output fresh/outside Git; no target executes and no product data is mounted.
The public binary144293003bytes is separately pinned and rechecked; it is not
private runtime evidence under the128MiB monitoring envelope. Earlier workspace
and cache-budget failures remain preserved. Read R-171 for scope/toolchain,
actual receipt and unresolved artifact/source admission limits.
Recorded graph-c:1505packages, exact source manifests and effective binary dependency versions/sums matched; no OpenPGP/S3 crypto packages or Go dynamic plugin package. R-171 limits this source disposition to the exact entrypoint/tags and records toolchain/build provenance gaps; it is not maintained stack admission.

### Finite selected-snapshot HTTP exporter

monitor_export.py reuses monitor_metrics.py and reads only monitor.json from one
explicitly selected existing private directory. It opens the directory once,
refuses redirected ancestors, anchors subsequent file opens to its descriptor,
rejects symlinks/FIFO/non-regular/non-owned/non-private files and reads at most
64KiB. Atomic snapshot replacement works within the selected directory; replacing
the directory itself does not silently redirect the exporter. Raw log/profile
paths, event tails and arbitrary filenames are never HTTP resources.

Linux-only research invocation, with the selected source directory explicitly
mounted read-only and matching its existing owner UID:

~~~sh
python3 -B /probe/monitor_export.py --snapshot-dir /selected-monitor \
  --sample-max-age 5 --duration 300 --port 9102 --container
~~~

Use --container only on the explicitly isolated internal probe network; default
binding is127.0.0.1. This is not a public/host administration listener or a
maintained deployment. Windows bind mounts cannot be assumed to provide the
required Unix ownership/0700directory/0600file guarantees. The chosen age budget
must follow the actual producer interval;5seconds is an example, not a universal
Node interval. Only GET /metrics is provided. Missing/invalid files return503;
stale valid snapshots return200 with monitor_fresh0 and omitted resource/source
status values. A stopped, fresh source emits survival0 without resource values.
Neither survival nor a successful HTTP scrape proves product readiness.

The server has one active request,2second socket idle timeout,16KiB maximum
metric response and a finite positive lifetime at most600seconds. Expiry
interrupts the active socket, joins its timer and closes the server/directory.
Filesystem reads are cooperative OS operations; this does not promise an
interruptible deadline for blocked kernel/filesystem I/O. No raw input, request
body, private path or selected source label is logged. Current finite successful
expiry is tested; managed signal/host crash behavior is not qualified here.

Run test_monitor_metrics plus test_monitor_export in the installed helper as
above, adding a16MiB private /tmp tmpfs. Fifteen checks passed. In addition to
file/HTTP boundary checks, the suite launches the real installed
ardents-diagnostics monitor supervising an owned shell process that exits7.
Actual monitor snapshots traverse HTTP from survival1 to0, preserving source
exit7 and joined log sinks. No Node sample exists in this workload; sample_fresh0
and absence of Node CPU/memory metrics are asserted. This proves the supervisor
bridge, not live product Node instrumentation or Prometheus/Loki ingestion.

## Rebuild and exercise the actual monitor/Node binaries

This is a bounded engineering probe under R171, not ready-backend admission or
a healthy/installed Node qualification. It reuses the already installed diagnostic
helper and refuses implicit image/dependency downloads. From repository root:

~~~powershell
make -C experiments/r-171-local-observability tools-install COLLECTOR=monitor-tool EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-monitor-build-new
~~~

The new output must be outside Git and absent before the command. The installer
restricts its Windows ACL, records exact Go source hashes and helper identity,
then builds Linux/amd64 ardents-diagnostics and ardents-node without network
access in a read-only container. The build has a180second timeout,2GiB memory,
2CPU,pids64 and separate512MiB scratch/cache tmpfs. Source hashes must agree
before/after; both artifacts are bounded64MiB and have SHA256/build information.
Existing installed helper executables remain unchanged. Dirty source is declared,
not silently attributed to HEAD.

Run probe-monitor-binary.py in that same installed helper with --network none,
read-only root, user10001, no capabilities/no-new-privileges,256MiB,1CPU,pids32,
a private16MiB /tmp tmpfs and a45second outer timeout. Mount the complete build
output read-only at /binaries, this experiment read-only at /probe, and a fresh
account-private report root writable at /reports. The command is:

~~~sh
timeout 45 python3 /probe/probe-monitor-binary.py
~~~

The probe verifies actual binary hashes and collector CLI flags. It generates
dedicated diagnostic TLS credentials only in private Linux tmpfs, then starts
the real monitor supervising the real Node with an explicitly missing required
plan. The valid selected client must observe fresh terminal survival0 through
HTTPS, resource freshness0 and absent resource values. SIGTERM joins monitoring
and preserves the original Node exit2 as monitor exit1. No raw source output,
keys, payloads or product authority enter metrics/reports. This deliberately
failed Node cannot prove healthy resource coverage or readiness.

Measured2026-10-01: build-c completed0 with Go1.26.8; diagnostic binary11325110bytes
SHA2561e3d58ba087e5e1b3cf7368215ed3bdf7b42bcd006e30922c51933a3ad0a1967;
Node15633689bytes SHA256a2a288a24dc0c045e72a3aa67667050c276ecf628adaa703786739930f742048.
TLS runtime-b completed0 and retained source_exit2,monitor_exit1,
sinks_joined=true,resource_samples0,tls_terminal_observed=true.
Original build setup-a/b failures and runtime-a BOM refusal are retained separately.
These receipts do not establish Prometheus scrape, Loki delivery, final stack
admission or the parent debug goal. Next evidence must use an explicitly accepted
live Node plan and the actual backend consumers.

### Accepted Node preparation profile and retained refusal

NODE_FIXTURE=1 explicitly adds current ardents, ardents-control and the existing
qualification-network generator to the offline build. The source manifest now
includes cached and nonignored untracked Go/mod/sum inputs, bounded20000files;
the before/after check therefore also covers new implementation files.

~~~powershell
make -C experiments/r-171-local-observability tools-install COLLECTOR=monitor-tool NODE_FIXTURE=1 EVIDENCE_ROOT=C:/Users/vitek/AppData/Local/Temp/ardents-r171-node-fixture-build-new
~~~

probe-node-resource.py validates all five exact artifact hashes, generates a
new private canonical fixture, remaps only local filesystem placement, and
uses actual commands for State acceptance, issuer initialization/inspection,
profile signing/inspection/submission and Hosting initialization. It selects
one Introduction Node and two real Sources. The clock observer uses the current
qualification recipe. This local shared-container placement is not two hosts,
a provider tariff, installed confinement or an ordinary Permission/Custody
qualification. A separate OpenSSL-generated fixture admission key is declared;
no user authority or product private key is imported.

Run in the installed helper with network none, read-only root/source/binaries,
user10001, no capabilities/no-new-privileges,4GiB,2CPU,pids128,128MiB private
/tmp tmpfs and240second outer timeout. Mount binaries at /binaries, this
experiment at /probe, the current stream-network-two-host directory read-only
at /qualification and a new account-private output at /reports:

~~~sh
timeout 240 python3 /probe/probe-node-resource.py
~~~

The output includes a bounded owner-private fixture archive containing generated
test keys and seeds; never publish or ingest it into Loki/HTTP. Command results
remain private. The public-shaped receipt records phase/cleanup/availability
rather than identities. Successful live observations would retain safe projected
metrics separately. The first refusal and subsequent correction are recorded below.

Actual attempt accepted-node-a failed at accept-profile-0, before any source
start: closed profile does not match accepted State. Static owner inspection
found the generator's Domains issue60-a/issue60-b cannot map through the actual
closed-profile Role Domain join. Filed #395 with exact source/artifact provenance
and the command failure. Do not weaken the join or treat serialization checks
as healthy Node admission. Native metrics, log rotation, source stop, backend
ingestion and healthy shutdown remain unverified by this attempt.

### Accepted Node resource and shutdown correction

The canonical fixture now selects family names through the existing actual Epoch
assignment for initiator/rendezvous/responder/introduction, rather than declare
Role Domains independently of the authenticated State. Product State/profile
validators remain unchanged. Probe setup creates every declared DutyRoot before
Node startup; failure-d retained the missing spend-lock parent. The producer
diagnostic_directory and monitor -raw are explicit private captures for this
owned generated fixture, not normal monitoring defaults or HTTP/Loki input.

Final accepted-node-h (TCP/TLS) and accepted-node-i (QUIC) both completed0:
three actual State/profile owners accepted, two actual Sources started, one
actual Introduction Node produced three samples. Unsupported default-valued
Go/process/PSI fields stayed absent from /metrics. Resource samples did not
occupy the event tail; timed rotation produced12 files,9055/9454 retained bytes,
within the2MiB/128file cap. Actual Node exit0, Sources/clock exit0, intentional
monitor interruption exit1, sinks_joined=true and cleanup/snapshot/file/
retention failures=false were asserted after shutdown. No delivery loss occurred.
This is one shared local container: cgroup CPU/memory covers that container,
not an independent per-Node attribution or two-host qualification.

Build-c diagnostic SHA256:
35b96b33ec8eefcf54d9503f4e4c6f2de991f387d1578572b551cf159dc3b267.
The fixture generator SHA256:
ad351078f45a9069bda177357b43c770f003d846fec270a5d8fa2db78ff410bf.
Product Node/ardents/control hashes remained unchanged from build-a.
Failure-e exposed unsupported encoded zeros; failure-f retained an outdated
probe assertion before its corrected expectation. Failure receipts are kept.

Run the explicitly rebuilt five-artifact set with probe-node-resource.py
--carrier tcp-tls or --carrier quic. Mount the artifact root read-only at
/binaries, this experiment read-only at/probe, the current
tests/qualification/stream-network-two-host tree read-only at/qualification and
a fresh account-private external result root at/reports. Use the pinned installed
diagnostic helper, UID10001, network none, read-only root, no capabilities,
no-new-privileges,512MiB memory,2CPUs,64PIDs,64MiB private executable/tmp tmpfs
and timeout120seconds. Do not mount host sockets, host authority roots or any
private capture into the monitoring backends. This probe verifies the real source
boundary; Prometheus/Loki ingestion and full backend admission remain pending.

### Explicit real-source browser preview

start-node-preview.ps1 -EvidenceRoot <NEW_PRIVATE_EXTERNAL_ROOT>
-BinaryRoot <EXPLICIT_FIVE_ARTIFACT_BUILD> -PluginRoot <CHECKED_PLUGIN_TREE>
-RelayBinary <EXPLICIT_BUILT_BROWSER_RELAY> -RunName <UNIQUE_r171_NAME>
uses the existing pinned images and minimal OTel distribution without downloads.
Build the helper commands only through the explicit tools-install recipes above.
The selected source is one real accepted Introduction Node and two real Sources
inside one local container. No raw producer capture is enabled in this preview.

The source and loopback8098 relay run for one hour. Backends retain their finite
memory/CPU/TSDB/tmpfs/log budgets and do not restart automatically. TLS server
material remains in a1MiB owner-private RAM volume; the existing tunnel holds
that mount with UID10002, which cannot read it. Collector mounts only safe
monitor storage and includes /fixture/monitor/logs/*-events.log, not raw/sample
files or private fixture roots. The generated Grafana password is account-private
at <ROOT>/private/grafana/password, login probe. Do not publish it.

After startup, run query-probe.py node-preview through the private query
certificate mount and selected internal network. A launch receipt alone does
not prove ingestion. The query must prove actual mTLS Node scrape, fresh measured
CPU/memory, unavailable extended fields, actual safe lifecycle events in Loki,
only job indexed and both Grafana datasource queries. Use the shared time picker
and Explore for details/field filters. Actual resource scope is the shared
container. An old READY record is not present readiness.

To stop only this preview, use docker compose --env-file <ROOT>/compose.env
-p <RunName> with compose.yaml,compose.otel.yaml,compose.plugins.yaml,
compose.browser.yaml and compose.node.yaml, then down --volumes. Stop only the
relay process identified in the launch receipt after checking its executable.
Generated private failure receipts survive outside Git. Removing these specific
volumes destroys this disposable preview's RAM history; it must not target any
other project. This preview does not qualify backend adoption, installed
two-host behavior, full profile/debug instrumentation or the parent goal.

The temporary loopback relay has sixteen simultaneous connection slots. This
supports two ordinary browser sessions within the tunnel's64PID limit; it is not
a general serving proxy. A cold browser must load actual module files rather
than rely on another browser's cache. The twelve-held-connections/raw health
request refusal and correction are recorded in the research owner. When replacing
an owned relay during an existing finite preview, verify its executable and
container argument, retain the original receipt, and save the replacement PID
and remaining budget separately. Cleanup must use the verified replacement PID,
not the original exited process ID. No replacement restarts or extends the source.

### Explicit private live-Node profiles

The same accepted-Node probe accepts --debug-profiles with --carrier tcp-tls
or --carrier quic. This explicit option sets ARDENTS_DEBUG_SOCKET only on the
owned Introduction Node, in a0700 private native tmpfs directory. It cannot be
combined with --preview-seconds. Normal monitoring remains unchanged.

The probe captures runtime, two-second CPU, heap, goroutine and two-second trace
through the existing private snapshot CLI. Each capture is bounded64MiB. Runtime
must contain actual positive heap allocation and goroutine observations; pprof
captures and trace scheduling output are checked offline by already installed
standard Go parsers with a two-second parser deadline. Nothing is built or fetched
during parsing. Profiles, parser tables and command outputs stay inside the
private fixture archive; only kind/size/hash/reference/validation enter the
private result receipt. Archive admission still has its64MiB total bound.

The first actual TCP/TLS debug run returned0 with joined normal shutdown and no
cleanup failures. Runtime163bytes, CPU778bytes, heap3245bytes, goroutine2249bytes
and trace191117bytes were accepted. Total private working files699077bytes.
This demonstrates capture/parse from the selected real process, not diagnosis of
a product error, sufficient CPU sampling of a quiet workload, or causality between
an operation and a profile. Use the exact selected executable and private receipt
for later offline analysis. Never mount these artifacts into Grafana or Loki.

### Native OTel retry exhaustion and retained loss

Use the separate finite synthetic loss profile:

~~~sh
make -C experiments/r-171-local-observability probe-otel-retry-exhaustion EVIDENCE_ROOT=<NEW_PRIVATE_EXTERNAL_ROOT> RUN_NAME=<UNIQUE_r171_NAME> PLUGIN_ROOT=<CHECKED_PLUGIN_TREE>
~~~

This requires the installed minimal OTel image and exact checked plugin pair.
The profile uses the existing private state-volume configuration; it does not
request or claim the separate restart qualification. Normal backend-recovery
probes retain their no-loss catch-up assertions. Loss injection stops only this
new project's Loki until the actual native terminal failed-record counter appears;
the selected exporter retry window remains30seconds. The observer has70seconds
to see failure and30seconds for resumed delivery. All original observations and
cleanup receipts remain outside Git. Source sequences are bounded1..600.

The pre-injection full catch-up establishes the synthetic history baseline.
After recovery, an empty serialized-byte queue and resumed sent counter are
insufficient: the query must also find a missing synthetic sequence and the
CollectorLogRecordsLost firing signal. The loss rule tests the positive cumulative
terminal-failure count for the current collector session. It remains firing after
queue recovery; restart may reset the counter, so it is not a durable incident
or acknowledgement store. A lazily absent failure counter is unavailable, not0.
A first positive series would be missed by a rate/increase-only rule without an
earlier sample. Terminal failed records are not retry attempts or enqueue failures.

Attempt-a observed failed_records1 and sequence127 absent after recovery.
Attempt-b also observed failed_records1, accepted190/sent189, queue_bytes0 and
sequence139 absent, with the new loss alert still firing. Both complete=true,
cleanup_exit0 and source/plugin snapshots stable. Only these owned experimental
containers were removed. This verifies the synthetic native delivery-loss path;
it does not qualify real Node rotation/storage pressure, crash durability, full
backend admission, Alertmanager notification delivery or acknowledgement history.

The actual-Node dashboard separates local supervisor lost bytes from collector
terminal failed record count and queue bytes. Their units and scopes differ;
none of these observations proves a complete global history by itself.

Both launch paths retain a native-alert-rules.txt preflight before starting
services. The pinned Prometheus parser must accept the actual selected rules;
Compose YAML validation alone cannot prove rule-file admission. The original
malformed Node-rule append was preserved as a private native refusal and fixed.
The loss experiment validates native alert/query behavior, not post-change
browser rendering of the actual-Node dashboard.
