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
