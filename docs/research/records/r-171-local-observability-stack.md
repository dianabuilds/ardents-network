---
id: R-171
title: Bounded local ready-component observability stack
status: open
owner: Product Owner and Codex
started: 2026-10-01
reviewed: 2026-10-01
---

# R-171 — Which ready components can supply useful bounded local monitoring?

## Decision this unlocks

Select exact maintained versions and configuration of Prometheus, Grafana, Loki,
one collector and Alertmanager for local engineering monitoring under #394.
The Product Owner accepted this component direction. Neither exact releases nor
maintained adoption is selected yet. Avoid building a time-series database, log
search engine, chart engine or generic notification service ourselves.

## Current contract

Read [product scope](../../product/scope.md), [threat model](../../security/threat-model.md),
[local diagnostics](../../development/local-diagnostics.md), and the
[dependency register](../../development/dependencies.md). The existing monitor
selects one local process, projects safe events, separates resource samples and
retains private rotating files with visible delivery loss. The private bundle
and debug/profile interfaces are preserved. No new Network behavior, security
mode, interception, product grant, Node control or wire identity is selected.
The issue ledger owns execution state. This is the one selected active local
engineering research question; paused product research remains paused.

## Hypotheses

- **H1:** Prometheus/Grafana/Loki/Alloy/Alertmanager meet the bounded local
  metric-history, log-query and alert journey with less maintained custom code.
- **H2:** The same backends with OTel Collector meet it with a smaller or clearer
  maintained integration and equivalent privacy/failure/resource behavior.
- **H0:** Neither configuration meets the declared outcome and budgets without
  additional unsafe exposure or unsustainable maintenance; choose neither yet.

## Evaluation criteria

Define these before any container experiment:

- One explicitly selected process produces inspectable measured history and safe
  structured logs over a shared time interval. A local CLI query works without
  panel scraping. Native Node facts require actual producer evidence.
- Units, gauge/counter/rate/current/peak/limit/availability and scope are explicit.
  Missing/unpopulated fields are unavailable; counter resets and sample gaps
  break rate continuity. Quiet logs and process survival do not imply health.
- A measured threshold and source/collector loss have pending/firing/recovery
  evidence and finite silence. Durable acknowledgement and incident history are
  separate requirements unless actual components demonstrably supply them.
- Protected data: payloads, keys, credentials, Names/Targets, peer topology, raw
  profiles/argv and cross-owner context never enter the shared pipeline. Local
  operational metadata remains private. Conditions: an intact host/Docker trust
  boundary, explicit owned inputs and admitted listeners/storage permissions.
  Host/root/daemon compromise is outside this confidentiality claim. Loopback
  does not protect against other local users by itself; actual authentication
  and backend port isolation must be tested. No global correlation/wire fields.
- Probe envelope: one source, at most 256 source-derived time series, at most 16 MiB
  synthetic safe event input, 15-minute wall budget, 4 GiB combined container memory
  limits and 4 CPU combined limits. Within the declared healthy five-minute window,
  target aggregate resident memory <=1 GiB and average CPU <= one core. These are
  falsifiable local selection thresholds, not product capacity guarantees.
- Private probe artifacts are capped at 128 MiB. Inspect all backend data/index/WAL/
  temporary space, with a 2 GiB probe-state ceiling. A retention option alone is
  not an enforced filesystem quota. Record any missing quota prerequisite and
  fail/refuse the bounded environment rather than claiming a hard disk bound.
- Normal monitoring never enables expensive profiling. Isolate only selected
  container/process resources; no host PID/network, Docker socket, arbitrary host
  log discovery, production authority roots or external uploads/notifications.
- Exact source/image/configuration/tool identity, licenses, supported release/fix
  path, known advisory applicability, update/removal path and actual privileges
  are admission requirements. Do not rely on popularity or clean scans alone.
- Local probes validate engineering behavior only, not installed C0 qualification,
  anonymity, independent security review or novice-user usability.

## Evidence plan

### Primary sources

Review upstream exact releases, immutable source/license/security files,
maintenance/support policy, advisories and official configuration/API docs for
Prometheus/Alertmanager, Grafana/Loki/Alloy and OTel Collector. Record access dates
and distinguish supported branch from an independently supported old patch.
The source review below identifies probe candidates; no container build closure or maintained adoption has been admitted.

### Experiment

After source/license/support/advisory review, create one disposable probe under
experiments/r-171-local-observability/ with a README, pinned candidate images,
explicit synthetic inputs, finite resources and private evidence outside Git.
Measure component/process/container scope separately. Retain failed attempts and
exact source/image/configuration identity. Compare collectors with the same
inputs and declared environment; no unconditional speedup from different caches,
profiles, workloads or incomplete runs. Prefer a small adapter over existing safe
facts; do not implement a parallel telemetry framework.

### Failure scenarios

Exercise actual source stop, collector stop, backend outage, malformed/unknown or
oversized input, sensitive sentinel exclusion, counter reset and sample gap,
rotation/restart, storage pressure, threshold/recovery and silence expiry.
Refuse credential-free unintended access and raw artifact browsing. Prove honest
loss/unavailability rather than interpolation or inferred root cause. External
notifications and arbitrary administrative execution remain disabled.

## Findings

All source access below is dated 2026-10-01. Exact-tag release/license responses,
public GitHub advisory responses and the official Grafana security index are
retained outside Git in the local `ardents-r171-source-review` evidence directory.
Its file-hash inventory identifies the captured responses; mutable upstream pages
must be rechecked before admission. These are source observations, not runtime
measurements or a clean artifact-security verdict.

### Release and license candidates

**Sourced fact:** Official release API responses identify these stable releases:

| Component | Inspected release | Published UTC | Source license |
|---|---|---|---|
| Prometheus | v3.15.0 | 2026-09-25 | Apache-2.0 |
| Prometheus LTS alternative | v3.13.3 | 2026-09-07 | License closure still to recheck at this exact tag |
| Alertmanager | v0.34.1 | 2026-09-17 | Apache-2.0; actual LICENSE text inspected |
| Grafana | v13.2.3 | 2026-09-29 | AGPL-3.0 |
| Loki | v3.7.8 | 2026-09-17 | AGPL-3.0 |
| Alloy | v1.20.1 | 2026-09-28 | Apache-2.0 |
| OTel Collector releases | v0.162.0 | 2026-09-29 | Apache-2.0 |

Release sources: [Prometheus](https://github.com/prometheus/prometheus/releases/tag/v3.15.0),
[LTS](https://github.com/prometheus/prometheus/releases/tag/v3.13.3),
[Alertmanager](https://github.com/prometheus/alertmanager/releases/tag/v0.34.1),
[Grafana](https://github.com/grafana/grafana/releases/tag/v13.2.3),
[Loki](https://github.com/grafana/loki/releases/tag/v3.7.8),
[Alloy](https://github.com/grafana/alloy/releases/tag/v1.20.1), and
[Collector](https://github.com/open-telemetry/opentelemetry-collector-releases/releases/tag/v0.162.0).
Licenses were fetched with the exact release ref through each repository's
`/license` API. GitHub reported NOASSERTION for Alertmanager, but its LICENSE
blob equals the inspected Apache-2.0 blob in Prometheus/Collector; classifier
failure is not evidence of a different license. Source licenses do not establish
all image-layer/transitive licenses. Use unmodified OSS services through documented
interfaces; preserve notices and exact source references. Redistribution or
modification needs its own actual license-closure review.

**Sourced fact:** [Prometheus support policy](https://prometheus.io/docs/introduction/release-cycle/)
marks 3.13 supported until 2027-07-31; ordinary minor releases generally stop
receiving fixes after their six-week cycle. LTS is best effort and only promises
selected high-severity fixes, so it does not waive Ardents' all-findings rule.
[Grafana support policy](https://grafana.com/docs/grafana/latest/upgrade-guide/when-to-upgrade/)
marks 13.2 supported until 2027-05-18 and 12.4 until 2027-05-24. Supported branches
do not make superseded patches independently supported. Alloy/Loki/Collector
exact support and replacement cadence remain to be established.

**Sourced fact:** The exact-tag [Alloy README](https://github.com/grafana/alloy/blob/v1.20.1/README.md)
plans minor releases every three weeks and patches every one to two weeks, with
security releases at any time and best-effort scheduling. It gives no inspected
fixed support end date for 1.20.1. [Loki release cadence](https://grafana.com/docs/loki/latest/release-notes/cadence/)
recommends current patches, normally monthly or twice monthly, and warns that
minor releases may break compatibility. This is an ongoing update burden, not
an LTS promise. [Collector versioning](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/VERSIONING.md)
limits its one-year binary-distribution LTS commitment to major versions starting
at v1; it cannot be applied to inspected v0.162.0. Component stability must be
reviewed for the exact selected receiver/processor/exporter combination.
**Inference:** Do not prefer Collector on a falsely assumed one-year support
promise. Prefer the smaller proven pipeline after equal-input probes, and retain
one explicitly owned update/removal path whichever collector is selected.

### Exposure, loss and unavailable state

**Sourced fact:** [Loki authentication](https://grafana.com/docs/loki/latest/operations/authentication/)
does not provide an application authentication layer; tenant headers alone do
not authenticate clients. Native RequireAndVerifyClientCert mTLS is documented.
**Inference:** Probe either native verified client certificates or an authenticating
entry point; publish no raw Loki port. A Docker internal network is containment,
not individual-user authentication. Grafana login and local agent queries require
real credentials; disabling signup/anonymous access must be tested.

**Sourced fact:** [Alloy file source](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/)
reports unhealthy only for invalid configuration. A reader's offset and valid
configuration do not prove end-to-end delivery. Its default filename label exposes
the source path, and corrupt-position recovery can duplicate or discard records.
**Inference:** Drop filename labels, admit only the explicitly mounted safe event
files, preserve delivery/loss counters and test restart/rotation against actual
queried events. Collector configuration health cannot drive Node readiness.

**Sourced fact:** [Alloy CLI](https://grafana.com/docs/alloy/latest/reference/cli/run/)
has usage reporting enabled unless disabled and exposes a support-bundle endpoint
unless disabled. [Grafana analytics](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/)
and [Loki reporting](https://grafana.com/docs/loki/latest/configure/) are enabled
by default. **Inference:** Explicitly disable reporting, update/plugin polling,
support-bundle exposure and remote configuration; verify blocked external egress
rather than treating a setting as proof. No profiling pipeline in normal mode.

**Sourced fact:** [Prometheus storage](https://prometheus.io/docs/prometheus/latest/storage/)
requires working space for compaction beyond retained blocks. **Inference:**
Retention size is not the complete state quota. For the disposable first probe,
explicitly sized tmpfs can enforce finite backend working space; it is volatile,
charges memory and cannot prove durable restart retention. Durable monitoring
needs a separately verified bounded persistent filesystem. Refuse a claimed
bounded persistent launch when that prerequisite is absent. Current Docker uses
`io.containerd.snapshotter.v1`; this observation alone proves no volume quota.

### Security evidence quality

**Sourced fact:** Public repository advisory endpoints returned 28 Grafana entries
with the newest dated 2023-03-01, and no Alloy/Loki entries. The
[official Grafana advisory index](https://grafana.com/security/security-advisories/)
contains 2026 findings for all three. **Inference:** Empty repository-advisory
responses cannot admit these components; review the official index plus exact
image/software closure. Do not conflate a successful HTTP response with coverage.

**Sourced fact:** [CVE-2026-75889](https://grafana.com/security/security-advisories/cve-2026-75889/)
is fixed in Alloy >=1.19.0; the 1.20.1 source candidate is beyond that fix floor.
[CVE-2026-21729](https://grafana.com/security/security-advisories/cve-2026-21729/)
is fixed in Loki >=3.7.0; candidate 3.7.8 is beyond that floor.
Grafana [CVE-2026-13719](https://grafana.com/security/security-advisories/cve-2026-13719/),
[CVE-2026-13720](https://grafana.com/security/security-advisories/cve-2026-13720/), and
[CVE-2026-81841](https://grafana.com/security/security-advisories/cve-2026-81841/)
name 13.2.3 as a fix for the 13.2 branch. These inspected cases are not a verdict
on every listed advisory or dependency. Ambiguous malformed version ranges need
release/source verification, not automatic range inference.

**Sourced fact:** Prometheus 3.13.3 release notes identify dependency security
updates GO-2026-5841 and GO-2026-6303 and fixes for shutdown spinning, compaction
blocking, restart missing samples and corrupt-store handle leaks.
**Inference:** Test actual shutdown/restart/pressure, preserve failure evidence,
and compare latest supported patch closures before choosing 3.13.3 versus 3.15.0.

**Assumption:** The accepted component direction reduces custom backend/UI
maintenance. Resource footprint, useful log search, full alert lifecycle and
artifact security are unmeasured. Existing private debug packages establish none
of those backend outcomes.
### Exact image inspection and remaining admission boundary

**Measurement:** On the selected Docker Desktop Linux/amd64 host, registry
manifest inspection resolved immutable platform digests for the five Alloy-stack
candidates. [Experiment tooling](../../../experiments/r-171-local-observability/README.md)
records those inputs. Explicit `make tools-install` downloaded and checked all
five platform/digest identities; receipt `ardents-r171-images-b/receipt.json`
records completion, per-image IDs, defaults and sizes. No candidate service ran.
The preceding PowerShell execution-policy and helper-autoload failures remain
failed preparation attempts. The recipe now uses process-scoped RemoteSigned,
matching the inspected machine policy, and a direct .NET SHA256 implementation;
it changes no host execution policy and installs no PowerShell module.

**Measurement:** Image-reported sizes total 899,358,496 bytes; this is a per-image
sum, not unique physical Docker-cache occupancy. Four images declare non-root
users; Alloy declares none. **Inference:** The Alloy experiment must explicitly
supply a non-root UID, prove access to only selected input/config/state and retain
no capabilities. A default user declaration alone proves neither confinement nor
correct file permissions.

**Measurement:** Contrib Collector 0.162.0 manifest lookup returned not found at
both `otel/opentelemetry-collector-contrib:0.162.0` and
`ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:0.162.0`.
The official release exists. This is a publication/prerequisite observation for
these exact image paths, not proof that no valid Collector distribution exists.
Verify publishing/source-build alternatives; do not silently compare another
release or start two collectors.

**Measurement:** Exact public Go executables were copied from created but never
started, network-disabled containers; extraction containers were removed after
inspection. Hashes and `go version -m` metadata are retained in
`ardents-r171-public-binary-inputs-b`. The extraction tree totals 1,437,821,754
bytes, distinct from runtime probe state and private diagnostic captures. Include
these public installation-review inputs and the earlier failed extraction in
actual local workspace accounting; do not claim they fit the 128 MiB runtime
probe-evidence allowance. `/etc/os-release` was absent at all inspected image
paths; OS/base-image closure remains unresolved. That absence does not erase
available Go build metadata or prove a dependency-free image.

**Measurement:** All 94 selected Grafana/Grafana OSS/Loki/Alloy advisory pages
listed in the captured official index were retrieved. 85 contain a recognizable
fixed-version section; nine do not. Page extraction is not an applicability
disposition. Preserve ambiguous version ranges and inspect secondary primary
advisory/source evidence for the nine before admission. The initial bounded
`govulncheck -mode=binary -json` scan completed for all five exact copied public
binaries, without timeout, OOM or JSON decoding errors. Scanner v1.1.4 reported
DB last modified 2026-09-28T16:43:40Z. Each JSON-mode exit was zero, but reports
contain 13 distinct Go advisory IDs across the stack; this is not a clean verdict.
Raw event streams, per-component exit files and scope-aware parsed summaries are
retained in `ardents-r171-go-closure-a`. Binary symbol presence is not runtime
reachability; module-only findings are not symbol findings. No finding is waived.
OS/transitive/image licensing, full finding dispositions and verified runtime
configuration remain prerequisites.

**Sourced fact:** [Govulncheck documentation](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
says JSON output exits successfully regardless of detected vulnerabilities;
binary analysis lacks source call graphs and may report unreachable symbols.
**Inference:** The admission consumer must decode the stream, check completion,
record findings and their module/package/symbol scope, and require dispositions
rather than treating process exit zero as security acceptance.

**Measurement:** Initial Prometheus 3.13.3 binary metadata declares grpc 1.82.1
and x/crypto 0.55.0. Actual scan findings include grpc fix floors 1.83.1/1.82.2 and
x/crypto 0.56.0. The newer exact-tag
[Prometheus 3.15.0 go.mod](https://github.com/prometheus/prometheus/blob/v3.15.0/go.mod)
declares grpc 1.83.2 and x/crypto 0.56.0. **Inference:** Replace the first probe's
Prometheus input with exact official 3.15.0, then verify its actual binary closure.
The mutable tag was resolved to Linux/amd64 manifest
`sha256:86b17a25c2db1d16a61b16b3c8f336679eb19e26333d03e808da387206e40faa`.
The experiment lock records this candidate change; the original LTS scan and
image receipt remain inspectable. Choosing current rather than LTS requires
tracking the ordinary six-week release/fix cadence. It does not resolve the other
images' findings or claim that the replacement binary was already scanned.

**Sourced fact:** The nine vendor pages without a fixed-version section were
cross-checked against their CVE CNA records. The inspected affected ranges stop
in Grafana 11.x or earlier; two refer to Grafana Enterprise. Exact snapshots in
`missing-fixed-version-cna-review.json` retain products, bounds and assigners.
The 13.2.3 OSS candidate is outside those recorded ranges. This covers those
nine advisory rows, not all old GitHub advisories, image components or plugins.

**Measurement:** `make quick-check` completed successfully for the current
experimental tooling/research delta (session 78523). Actual install invocations
refuse an existing evidence directory and a repository-contained root, and create
no repository artifact on refusal. Retained image receipts were not overwritten.
### Prepared pipeline and actual isolated fixture behavior

**Measurement:** The experiment now includes a finite synthetic Python source,
Prometheus scrape/rule inputs, notification-free Alertmanager configuration,
Alloy file-to-Loki pipeline and Loki verified-client TLS configuration. Candidate
`promtool`, `amtool`, Alloy `validate` and Loki `verify-config` accepted their
respective files. This validates configuration syntax only; no TLS handshake,
backend delivery, dashboard, retention or installed Node behavior is established.

**Measurement:** The synthetic source ran as UID/GID 10001 with no Docker network,
no host ports, read-only root, all capabilities dropped, no-new-privileges, a
128 MiB memory limit, 0.25 CPU limit, 16-PID cap and private 16 MiB tmpfs. Actual
loopback /metrics queries returned typed queue/heartbeat gauges and an event
counter increasing from 17 to 97. Both queried queue values were zero, outside
the scripted pressure window; do not claim a measured pressure sample from those
queries. Its final structured stdout contained 67 normal and 30 pressure events.
SIGTERM produced exit 0 without OOM and joined the HTTP worker. This is fixture
lifecycle evidence, not product shutdown qualification or collector delivery.
Host config, two metric observations, resource snapshot, final logs and terminal
container state are retained in `ardents-r171-fixture-lifecycle-a`.

**Measurement:** Exact candidate `promtool test rules` evaluated all three alert
rules against explicit synthetic series. Assertions observed pending state,
no premature firing, expected firing and recovery for queue pressure, fixture
scrape loss and collector scrape loss. The first attempt failed because a
read-only /tmp supplied no test-store prerequisite; rerun used a private 64 MiB
tmpfs without network or elevated capabilities. The original refusal remains
failed preparation. Loki's first native CLI call failed after PowerShell split
an unquoted dotted argument; exact quoted arguments passed and both outcomes
are retained. These are corrected test-environment/argument issues, not waived
repository gates. `make quick-check` passed (session 11166).

**Inference:** The next observable boundary is the actual five-service isolated
pipeline with private per-client TLS keys, finite state/egress/process resources,
authenticated query access and provisioned shared-interval Grafana views. Rule
unit behavior does not prove Alertmanager receipt, silence expiry, recovery
history or live alert presentation. Existing dependency/image dispositions,
state durability, source/reset/gaps, outages and full privacy/resource admission
remain open before maintained integration.
### Actual isolated pipeline and corrected false-positive boundary

**Measurement:** Runtime attempts are retained outside Git in
`ardents-r171-pipeline-a` through `ardents-r171-pipeline-g`. A failed while child
PowerShell could not autoload Set-Acl; direct .NET directory ACL application fixed
the prerequisite. B failed on a nested read-only Grafana bind mount. C/D reached
backend queries but Grafana could not register datasource plugins: their Unix
socket creation failed on read-only /tmp. A private 32 MiB noexec tmpfs corrected
that boundary. E exposed the 32-PID budget exhausting Go threads across bundled
plugins. Disable unused backend plugins and use an explicit 96-PID Grafana budget;
only Loki and Prometheus registered in the subsequent measured run. No root,
capability, host PID namespace or Docker-socket access was added.

**Measurement:** F returned HTTP success and datasource health OK, but content
inspection found zero Loki records, a pending threshold alert and no Alertmanager
receipt. Its successful launcher exit is only HTTP-observation completion, not
log/alert acceptance. Alloy logged a missing selected file before fixture startup.
Enable periodic discovery of that exact path, with a one-second scan; do not
expand the mounted log root. Strengthen the query probe to reject absent scoped
records, incorrect queue/scrape values and missing firing/Alertmanager receipt.
Observe pressure after the six-second pending interval instead of declaring an
early pending sample sufficient. All originals remain preserved.

**Measurement:** G terminated with exit zero and cleanup_exit zero. Actual content
assertions observed queue 0 and 12 scoped Loki records in normal phase; queue 20,
44 records including pressure events, Prometheus `FixtureQueuePressure` firing
and an active matching Alertmanager alert in pressure phase. Both selected
Grafana datasource health endpoints returned status OK. Query-client backend TLS
was authenticated; attempts without client certificates were refused. Anonymous
Grafana dashboard access returned 401. A provisioned four-panel dashboard was
retrieved through authenticated API, not rendered in a browser. No product data
or Node health claim follows. Docker lists no surviving R171-pipeline containers.

**Measurement:** `make quick-check` session 8076 passed for the runtime harness
configuration delta before the final discovery/content-assertion correction.
The corrected discovery/content-assertion delta passed `make quick-check` (session 42756, exit zero). The original probe
invoked once from the repository root also failed because `probe` belongs to the
experiment Makefile; use `make -C experiments/r-171-local-observability probe`.

**Limit:** G is a short volatile synthetic journey, not the five-minute resource
target, complete egress/DNS proof, recovery/silence-expiry/history, source or
collector outage, retention/storage pressure, restart durability or dashboard
rendering acceptance. Exact image closure now includes separately executable
bundled datasource plugins: Grafana server binary review alone is insufficient.
The inspected image's Prometheus plugin manifest identifies 13.2.1 within the
13.2.3 server image; verify those artifacts independently rather than assigning
server version/advisory floors to all plugin executables. Maintained component
admission remains open.

### Live silence, recovery and separate observation failures

**Measurement:** `ardents-r171-lifecycle-a` passed the strengthened normal/pressure
checks and all nine lifecycle assertions, then exited zero with cleanup_exit zero.
A scoped twelve-second silence changed the existing firing threshold alert's
Alertmanager state to suppressed while both scrape sources remained available and
queue stayed twenty. The silence became expired; the same firing alert returned
to active with no silencedBy entries. After the fixture pressure interval ended,
no matching threshold alert remained in either active API result. This observes
recovery/clearance, not a durable resolved-event or acknowledgement history.

**Measurement:** Freezing only the synthetic source produced fixture up=0 while
collector up=1. Its queue series was absent, not zero-filled. The source-loss rule
was observed pending without Alertmanager receipt, then firing with active receipt.
Resuming the same process restored its series and cleared the matching alert.
Freezing only Alloy produced fixture up=1/collector up=0; the queue remained
present. Collector-loss independently passed pending, firing/active receipt and
clearance after resuming Alloy. Each polling observation is retained under its
phase/attempt number; no timeout or unsuccessful attempt was reclassified as a
passing retry. These are observed freezes of explicit synthetic processes,
not kill/restart, graceful-stop qualification or product authority operations.

**Measurement:** The fixture lifetime is now bounded at 600 seconds with a
30-90 second pressure interval. This allows silence expiry before recovery and
leaves room for the separately required healthy five-minute resource window.
The older 180-second fixture could not supply that window. No five-minute result
is claimed from this lifecycle run. Request bodies use the exact Alertmanager
v0.34.1 API schema; silence matchers cover only fixture pressure. Notification
integrations remain absent. All R171 pipeline containers were removed.

**Limit:** Full source/collector restart durability, post-outage log catch-up,
backend outage/storage pressure, resource/cardinality/egress and rendered shared
interval remain open. Raw profiles and Node/Application inputs were not collected.

### Post-outage log catch-up and actual Grafana query frames

**Measurement:** `ardents-r171-shared-a` observed every synthetic sequence through
producer watermark 117 after resuming the collector: 118 returned/unique entries,
no duplicate sequences in that query. A Grafana `/api/ds/query` request supplied
one explicit 180-second from/to interval to both selected datasource backends.
It returned 59 metric rows and 118 log rows inside that interval; metric values
included normal zero and pressure twenty. This exercises actual backend queries,
not merely datasource-health or dashboard-schema APIs. It is not browser rendering.

**Measurement:** Content review identified that default file ingestion uses
collector receipt time. The revised Alloy process parses the fixture's `at`
field and assigns RFC3339Nano event timestamps. `ardents-r171-event-time-a`
repeated all lifecycle assertions, checked each returned Loki timestamp against
its source event time within one microsecond, caught up through watermark 117
with 118 unique records/no duplicates, and passed actual common-interval Grafana
queries. The metric frame contained a 12000 ms gap during source unavailability;
no synthetic zero replaced that missing period. Exit and cleanup_exit were zero,
with no surviving R171 pipeline containers. These results prove the valid fixture
input only: malformed JSON is dropped and timestamp-parse failure uses skip;
that fallback is not a valid-event-time claim for unsupported inputs.

**Limit:** Shared-a overlapped separate public binary review initially configured
with one CPU, so the combined configured maximum with a query helper could reach
4.5 CPU rather than the predeclared four. It is not a within-envelope resource
receipt. The scan was already terminal when its configured cap was reduced to
0.5; this does not retroactively correct that overlap. Event-time-a ran after the
scanner was terminal. Future overlapping binary review must cap it at 0.5 CPU or
run sequentially. Neither run establishes the five-minute healthy resource target.

**Measurement:** `make quick-check` session 27215 exited zero after shared-query
and event-time corrections; `git diff --check` passed. Originals remain private.

### Exact bundled datasource binary findings

**Measurement:** Two public plugin binaries/manifests/licenses were copied from a
created, never-started exact Grafana candidate container. Public review inputs are
in `ardents-r171-public-plugin-inputs-a`; raw govulncheck JSON and pending finding
summary are in `ardents-r171-plugin-closure-a`. Source-image identity remains
`sha256:d84563330dc9d2fd2bc096d0fb96021b5319c75bf6ed555566709998e823dec4`.
Both binaries report Go 1.26.7; the server's version/build floor cannot substitute.

- Prometheus plugin manifest 13.2.1, 32907426 bytes, SHA256
  `2fe41b679a3d319450ded2a0cd88c0da1a36f7ed48f4932238022cc49b285eca`:
  GO-2026-6443 / CVE-2026-84445 in grpc 1.83.1, two scanner symbol-mode records (reachability unproved),
  recorded fix floor 1.83.2.
- Loki plugin manifest 13.2.0, 40472738 bytes, SHA256
  `c5df1ff330569638166e65e3c5ec16c2474a080444e955e94e66ba41fb494774`:
  GO-2026-5932, seven openpgp wildcard records (reachability unproved), no fixed version supplied.

**Measurement:** Installed govulncheck 1.1.4, Go tool 1.26.8, database
https://vuln.go.dev last-modified 2026-09-28T16:43:40Z. The actual helper image for
these probes was `sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad`;
a mutable local tag must not substitute for that observed identity. Scanner
terminated zero/no OOM with retained JSON; zero in JSON mode is not a clean result.
Symbol-mode output is not proof of linked symbols, selected-configuration exploitability or a source call graph.
Both findings remain pending; neither severity nor local-only use is an exemption.

**Sourced fact:** Prometheus datasource v13.2.2 release explicitly records a fix
for CVE-2026-84445 and its exact go.mod uses grpc 1.84.0. Latest v13.2.3 adds metric
search/UI changes but declares a development grpc version; review the actual fix
commit/closure before inferring safety from its higher semantic version. Loki
plugin v13.2.1 exact go.mod uses grpc 1.83.2 while retaining x/crypto 0.56.0;
this alone does not resolve its openpgp finding. Source assets/build/support and
actual replacement binary admission remain unproved. Sources, accessed 2026-10-01:
[Prometheus releases](https://github.com/grafana/grafana-prometheus-datasource/releases),
[v13.2.2 source](https://github.com/grafana/grafana-prometheus-datasource/blob/v13.2.2/go.mod),
[v13.2.3 source](https://github.com/grafana/grafana-prometheus-datasource/blob/v13.2.3/go.mod),
[Loki v13.2.1 source](https://github.com/grafana/grafana-loki-datasource/blob/v13.2.1/go.mod).

### Scanner precision correction and restart/resource measurements

**Sourced fact:** govulncheck v1.1.4 binary mode falls back to module-level
precision when extracted package symbols are empty. It then constructs known
vulnerable function names from the advisory, including a package wildcard when
no function list is supplied. Therefore the earlier JSON symbol-mode record
counts must not be read as measured linked vulnerable functions. Source accessed
2026-10-01: [binary analysis implementation](https://github.com/golang/vuln/blob/v1.1.4/internal/vulncheck/binary.go).

**Measurement and inference:** `elf-sections.txt` in the private plugin review
shows .gopclntab but no .symtab or debug sections for both exact plugins. The
openpgp records contain package wildcards. These observations are consistent
with the documented stripped-binary fallback; they do not prove an openpgp call
in either selected plugin. All advisory dispositions remain pending. The failed
`go tool nm` attempt required an unwritable Go cache and is retained as failed;
readelf used the installed tool without an implicit build or new installation.

**Measurement:** `ardents-r171-restart-a` completed with cleanup_exit zero.
Restarting the five backend containers preserved 59 historical metric rows,
119 log records and one synthetic silence in the selected baseline. The live
fixture continued; 22 new source events and 20 new collector reads were observed.
This supports checkpoint reuse rather than a complete historical reread, not
transactional exactly-once delivery. Post-restart catch-up returned 141 unique
records through watermark 140 without duplicates in the selected query.

**Boundary:** `compose.restart.yaml` keeps five sized named tmpfs volumes mounted
by one idle, non-networked, non-root helper with read-only mounts. Backends retain
only their own writable state. RAM state survives backend-container restart while
that mount holder lives; it does not survive Docker-daemon/host restart or the
last unmount. Compose 2.40.3-desktop.1 supports the required !override syntax.
This experiment adds no maintained product role or administration authority.

**Measurement:** `ardents-r171-resources-a` repeated the lifecycle/restart checks
and then measured a healthy 303.1557687-second window with 37 samples. Summed
selected-process RSS, including Grafana datasource children, peaked at
945844224 bytes (902.03 MiB), below the predeclared 1 GiB threshold. Mean sampled
CPU was 2.22297 percent where 100 percent is one core; the independently enforced
aggregate selected-container CPU ceiling was 0.96 core. Native metric queries
returned 150 queue rows, all zero, and both source up series remained one without
a scrape gap over the checked window. The run and cleanup exited zero.

**Limits:** RSS peak is sampled, CPU is a snapshot mean rather than an integrated
average, and Docker-daemon/host CLI overhead is excluded. Resource caps bound the
selected container set; these measurements do not establish Node workload or
capacity. Last-sample Grafana RSS was 388362240 bytes across three processes;
Alloy RSS was 226754560 bytes. Footprint is substantial for this synthetic source.
Backend-state allocated bytes were not captured before cleanup and remain a
missing receipt; configured filesystem caps alone do not prove actual retained
usage. Storage pressure, backend outage, daemon/host persistence, egress,
cardinality and rendered dashboard acceptance remain open. Containers were removed.
## Options

H1 and H2 are alternatives for one collector, never a default dual pipeline.
Record fit, exposure, operational/update dependencies and removal costs for each.
Tempo/Pyroscope and a new runtime SDK are outside this first selection unless a
named unmet requirement establishes their necessity.

## Recommendation

Choose no exact versions/configuration yet. Complete the named bounded review and
probe. Confidence in the component direction is provisional; the strongest risk
is five maintained services imposing more footprint/auth/storage/update complexity
than one human and Codex can sustain for this selected local workload.

## Disposition

Open. No maintained component adoption, new runtime dependency, protocol or
administration authority follows from this record. Promote an accepted selection
to current engineering/dependency owners and an ADR for meaningful lock-in before
maintained implementation. #394 owns handoff/admission and #377 retains the full
monitoring, debugging and later existing-authority administration goal.

Verification receipt: make quick-check session 10851 exited zero after lifecycle probe changes. Lifecycle-a external evidence inventory totals 591578 bytes, below the declared runtime-evidence ceiling; this measurement does not establish a general enforced filesystem quota.

Resource receipt inventories (2026-10-01): restart-a retained 853209 bytes and
resources-a retained 1062643 bytes of external evidence, below 128 MiB. This is
an observed inventory, not an enforced host-filesystem quota. make quick-check
session 94927 passed for the restart/resource delta; later backend-outage
changes are a separate uncompleted verification boundary.
### Bounded log-backend outage and allocated storage

**Measurement:** `ardents-r171-backend-outage-a` completed with cleanup_exit zero.
The probe stopped only Loki, observed its query interface unavailable while
fixture up=1 and collector up=1, and required actual registered retry and drop
counters. Retry increase was one, drop increase zero. Restoring Loki delivered
all 141 sequences through captured watermark 141, with 141 unique records,
zero duplicate sequences in the selected query and preserved source event time.
No missing counter was treated as measured zero. The subsequent five-backend
restart/history/checkpoint assertions also passed. This is a short graceful
backend stop/start; it does not prove crash, extended outage, overflow or lossless
delivery after exhausting the finite retry budget. No automatic repair follows.

**Measurement:** Before cleanup, read-only state-holder observations summed
1789952 bytes of allocated files and filesystem-used bytes across the five
backend state volumes. Per-owner used bytes: Prometheus 57344, Alertmanager 4096,
Loki 77824, Alloy 8192, Grafana 1642496. Actual filesystem capacities matched
512/32/512/64/128 MiB respectively. The live sample is not atomic and excludes
Grafana /tmp, fixture state, Docker service logs, Docker daemon and image cache.
It does not establish a full storage-pressure or long-retention acceptance.
Original reports and container cleanup receipts remain outside Git.

Verification: make quick-check session 95776 passed during the backend-outage
change. The actual outage run also exercised the final stricter check requiring
both retry and drop counters. Required full gates/review/integration still belong
to the completed maintained-adoption slice; no exact component is admitted here.
### Predeclared filesystem-pressure probe boundary

Before running a new storage-pressure attempt, fill only the new run's Loki
512 MiB tmpfs using one explicit synthetic regular file. The offline injector
uses the already-installed helper as UID10001, read-only root, no network,
no capabilities, 640 MiB memory and 0.25 CPU. It mounts only that selected
volume, its own report directory and read-only experiment code. No host log or
product input is mounted. The combined runtime configuration stays below the
4 GiB/4 CPU envelope; source and collector continue running.

Falsification: actual ENOSPC and filesystem free bytes must be observed, not
inferred from configured quota. Record Loki's native WAL disk-full counter and
collector retry/drop counters, keeping unavailable metrics distinct from zero.
Require a visible native pressure signal while scrape sources remain available.
Do not infer persistence from HTTP success: Loki can accept an entry that cannot
be written to WAL in the interval before its disk protection checks. Free only
the injector-owned file, verify space recovery, then query the fixed producer
watermark for missing/duplicate events. Missing history or a counter-based
loss must remain a failed durability/delivery result, never an erased retry.

Primary source accessed 2026-10-01: [Loki WAL behavior](https://grafana.com/docs/loki/latest/operations/storage/wal/).
The current docs describe 90 percent disk protection and a disk-full failure
counter, including an acknowledged-but-not-WAL-persisted window. These are
source expectations; exact candidate behavior must be measured separately.
### Actual WAL filesystem exhaustion and native alert

**Measurement:** `ardents-r171-storage-pressure-a` terminated zero with cleanup_exit
zero. The injector owned one regular file of 536846336 bytes in the new 512 MiB
Loki tmpfs; an actual write returned ENOSPC and statvfs reported zero free bytes.
After validating identity, owner, type, size and single-link count, the injector
removed only that file. Observed free space recovered to 536846336 bytes.

**Measurement:** Native `loki_ingester_wal_disk_full_failures_total` increased by
two while fixture, collector and logbackend up all remained one. The scoped
`LogStoragePressure` rule reached firing and Alertmanager active. Its expression
uses a thirty-second counter-increase window and six-second pending period.
After freeing space, the rule and active Alertmanager result cleared; the
cumulative WAL counter remained two above baseline. This observes alert clearance,
not erasure of the failure, proof of durability or an incident-history feature.

**Measurement:** Post-pressure catch-up reached captured producer watermark 133
on attempt twenty, returning 144 unique events without duplicates and with source
event time preserved. The subsequent backend restart/history checks passed;
post-restart query included every sequence through watermark 177 with 178 unique
events. Runtime reports retained 1749804 bytes outside Git. make quick-check
session 93482 passed. No surviving R171 containers remain after cleanup.

**Limit:** This profile did not separately retain collector retry/drop counters
during the full-disk interval. Pending was not asserted in the first probe; the retained raw rules observations do contain it.
Those predeclared evidence items remain missing; passing implemented assertions
is not full storage-pressure admission. The probe does not crash Loki while
writes lack WAL persistence, exhaust extended retries, prove a host/daemon
restart, or establish future log durability from HTTP acceptance. Source events
and limits are synthetic; no actual Node/Application inputs were admitted.
The extra authenticated backend scrape selects only the three named WAL metrics,
with a thirty-two-sample budget. No arbitrary exporter or raw-file discovery was
added. Complete component selection, advisory dispositions, real-source integration
and required completed-slice gates/review remain open under #394/#377.
### Correction: pending evidence and alternative collector publication

**Measurement correction:** The pressure-a raw `06-rules.json` through
`10-rules.json` already contain LogStoragePressure pending, at collector times
1790813652.7870317 through 1790813657.049377. `11-rules.json` contains firing at
1790813658.1154. The previous claim that a pending sample was missing was wrong;
raw samples existed, but the first probe did not explicitly assert that sequence.
The strengthened probe now requires pending without an Alertmanager firing
receipt before accepting firing, and collects actual retry/drop baselines and
deltas. Any unavailable counter, reset or observed drop fails that boundary.

**Sourced fact:** The official collector release v0.162.0 published 2026-09-29
12:34:07Z includes `otelcol-contrib_0.162.0_linux_amd64.tar.gz` (112285869 bytes),
API digest `sha256:fcc063749f730f8c21fe29f2d340ff174f5f1c5885bd3156fb6c985a3036fcc3`,
plus checksum, SBOM and Sigstore artifacts. Source accessed 2026-10-01:
[official release](https://github.com/open-telemetry/opentelemetry-collector-releases/releases/tag/v0.162.0).
The primary API receipt is retained in external `ardents-r171-source-review/otel-contrib-0162-release-assets.json`.
This supplies a concrete artifact-inspection path despite the earlier absent
image tags. API digest is published provenance, not downloaded-byte verification
or signature validation. No asset was installed, executed, admitted or substituted
into the Alloy pipeline by this observation. H2 remains open.
**Preserved refusal:** pressure-b reached pending/firing/recovery with actual
retry increase four during firing and five by clearance, no observed dropped
entries, and successful pre-restart catch-up. The later post-restart catch-up
failed: producer watermark183, final query203 records, sequence157 absent.
The run terminated nonzero/cleanup zero; it is not corrected by a later pass.
The prototype restarted sender and receiver concurrently with five-second stops,
leaving delivery at their shutdown boundary unqualified. Exact causal attribution
to sender versus receiver is not established by these reports.

**Concrete correction to test:** stop/drain Alloy with Loki still available,
retain and require a clean non-OOM exit within thirty seconds, restart backends,
wait for their readiness, then start Alloy. The source keeps producing bounded
safe events to its selected retained file throughout. All previous catch-up,
event-time/history/counter checks remain required. This establishes ordered
planned restart behavior only, not arbitrary crash durability or exactly-once
transport. Input stability is now recorded also for failed terminal attempts.
**Measurement after correction:** pressure-c completed with terminal zero and
cleanup_exit zero. Native WAL failure increase one, pending explicitly observed,
then firing/active receipt while fixture/collector/logbackend up stayed one.
Collector retry delta was four during firing and five at clearance; actual drop
delta remained zero. Space recovery and alert clearance passed. The sender
stopped before backend shutdown with exit0, no OOM. After ordered restart, the
query covered every event through watermark180 with181 unique records, no query
duplicates and preserved source event time on its first attempt. This supports
the corrected planned lifecycle for the selected synthetic configuration; it does
not erase pressure-b's loss or prove arbitrary crash durability/exactly-once use.

**Measurement:** pressure-c receipt includes five exact installed linux/amd64
image config/manifest identities, helper image0ecc73f2 and fourteen selected
source/configuration SHA256/size entries. Their start/end snapshots matched,
source_inputs_stable=true. Private retained evidence totals2147603bytes. These
are observed snapshots, not an atomic copy or protection against an owner
changing and restoring bytes between observations. Failure receipts now also
attempt the final identity check. quick-check54465 passed before the ordering
correction; quick-check43616 passed afterward. All probe containers were removed.
The bounded missing pending/delivery-counter evidence is now supplied; full
component selection/admission, extended/crash/storage lifecycle and actual
selected-process monitoring/debug goal acceptance remain open.
### Official alternate collector artifact inspection

**Measurement:** The official v0.162.0 SPDX-2.3 SBOM was downloaded with a3MiB
read ceiling; actual2255297bytes and SHA25609a4df608b40031664d4e9588fe6c5ba43ed99f7ea28c8a92e481f70437c4adf
match the release API digest. It lists1080packages,71 distinct license expressions,
four packages without concluded licensing. Those tool-generated conclusions are
an inventory, not independent licensing acceptance. Relevant entries include
filelogreceiver/filestorage0.162.0, grpc1.83.2, x/crypto0.57.0 and stdlibGo1.26.8.
Original data/receipt remain in external ardents-r171-otel-source-a.

**Measurement:** Explicit `make tools-install COLLECTOR=otel` fetched the pinned
112285869-byte archive and same SBOM, checked their actual digests and extracted
only the regular `otelcol-contrib` member without executing it or extracting
archive paths. Binary407498914bytes, SHA2562425bdf5f89042cd71f56cf5a66b41681d340cbe14c0b1899bcfe2a3a685a064.
Installed Go buildinfo inspection confirms go1.26.8; ELF program headers contain
no INTERP/DYNAMIC entries. This is metadata inspection, not successful startup,
libc/OS admission, signature verification or runtime resource evidence. Helper
identity0ecc73f2 is recorded. Public artifact installation overhead is separate
from the128MiB private runtime-evidence budget. Original receipts are in
ardents-r171-otel-public-a; installation session94112 completed zero.

**Measurement:** Actual binary govulncheck1.1.4 scan (database last-modified
2026-09-28T16:43:40Z) terminated0/noOOM. Its31finding records group into7advisories:
GO-2022-0635/0646 (aws-sdk-go1.55.8), GO-2026-5046/5047/5048(avro/v2 2.33.0),
GO-2026-5544(azureauthextension0.162.0), GO-2026-5932(x/crypto0.57.0).
The larger raw OSV stream includes advisories outside the actual finding set;
it is not a count of candidate vulnerabilities. No source call graph or
selected-configuration reachability was proved. All dispositions remain pending.
Raw scan/config/receipt and pending-findings.json remain in ardents-r171-otel-closure-a.
The stopped scanner container was removed. Matching digests and JSON exit0 do
not establish signature, component admission or ongoing support.

**Sourced comparison:** exact v0.162.0 filelog defaults to reading from end,
filename metadata enabled,1024concurrent files,1MiB entry limit, receiver retry
disabled and offsets only in memory without selected storage. The selected-file
probe must override these defaults and preserve source time. The exact release
manifest supplies OTLP/HTTP exporter; Loki's native OTLP path uses that exporter,
structured metadata and resource-to-index mapping. Do not reuse a removed legacy
Loki-exporter example or blindly copy default identifying index attributes.
File-storage recreation can reset state/duplicate data; compaction needs additional
bounded working space. H2 must test finite queue/offset persistence, pressure,
shutdown, timestamp/label mapping and footprint with equivalent input before a
choice. No H2 runtime or maintained configuration is admitted here.
Sources accessed2026-10-01:
[filelog v0.162.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.162.0/receiver/filelogreceiver/README.md),
[file storage v0.162.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.162.0/extension/storage/filestorage/README.md),
[exact distribution manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/v0.162.0/distributions/otelcol-contrib/manifest.yaml),
[native Loki OTLP ingestion](https://grafana.com/docs/loki/latest/send-data/otel/).

Verification: make quick-check session36848 exited zero for installer/extractor
changes. Full checks/review and component admission remain separate requirements.
### Scoped Azure authentication finding correction

**Sourced fact, accessed 2026-10-01:** GO-2026-5544 points to
[GHSA-pjv4-3c63-699f](https://github.com/open-telemetry/opentelemetry-collector-contrib/security/advisories/GHSA-pjv4-3c63-699f).
The upstream advisory describes inbound authentication comparing the supplied
bearer token against a credential token obtained using client-controlled Host;
it lists affected versions0.124.0 through0.150.0 and no patched version. Our
binary scanner nevertheless reported azureauthextension0.162.0. The advisory
range alone does not explain or waive that finding.

**Measurement/source inspection:** Downloaded source module0.162.0 checksum
`h1:sA8bT9TBgVtlyeyJM6TYjeTpE6ucJAtCg+STOcUSme8=` matches the dependency entry
in the inspected407498914-byte linux/amd64 Collector binary whose SHA256 is
2425bdf5f89042cd71f56cf5a66b41681d340cbe14c0b1899bcfe2a3a685a064.
Go's module receipt identifies upstream commit
ae8c507510f48f433ab47dd1c6b01a59d6c388b5, module tag
extension/azureauthextension/v0.162.0. This correlates the source module with the
binary's declared build information; it is not a signed reproducible build proof.

At that exact source, `Authenticate` rejects a nil verifier and delegates the
incoming bearer token to `a.verifier.Verify`; it does not read Host or call
getTokenForHost. Start constructs the OIDC provider from configured issuer and
passes the configured audience as ClientID. getTokenForHost still exists in the
outbound client path; its existence is not evidence of the old inbound defect.
[Exact source](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/extension/azureauthextension/extension.go).
Source SHA25638bfa64a17e91eaf0cf86a7faceaa5b78e9d0e070ff1106e22d2461233ab81a8.
The upstream test named TestAuthenticateRejectsReplayedBearerToken uses a mock
verifier returning audience mismatch; inspecting it is not executing a real
signature/issuer/audience/expiry validation test. Those broader claims are not
made here. No production Azure credential or token was acquired.

**Scoped disposition:** the particular Host-derived token-comparison defect is
corrected in the inspected source module corresponding to this exact binary.
Preserve the scanner finding and the upstream metadata discrepancy; do not
classify every Azure feature or the entire Collector as secure. Owner: Codex
under R-171. Covered target: this linux/amd64 artifact and its declared source;
no other build/platform/configuration is admitted. Reassess after any artifact,
module/checksum/source, advisory, inbound-auth configuration, OIDC dependency or
privilege change. The remaining six advisory IDs, full verification closure,
signature, licenses/support and actual H2 runtime remain unresolved.

Reproduction: in an external empty directory, set GOMODCACHE to a separate
external cache and GOTOOLCHAIN=local, GOENV=off, GOWORK=off; run
`go mod download -json github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension@v0.162.0`.
Compare its Sum with the actual binary's retained `go version -m` output; hash
binary and extension.go against the identities above; inspect Authenticate,
Start and the named mock test. This downloads source only, not a tool install or
module execution. Original module receipt and static assertion/hash receipt are
external ardents-r171-otel-module-review-a/azure-module-receipt.json and
source-disposition.json. The bounded assertion completed zero on2026-10-01;
it checks the reviewed method boundary, not compiler/runtime semantics.

### H2 selected-file configuration validation

The experimental `otel.yml` selects only filelog/fixture, file_storage/fixture
and otlp_http/fixture. It reads the exact synthetic events.ndjson, disables file
name/path metadata, uses one file/batch,16KiB entry cap, explicit source timestamp
and severity parsing, one persistent queue consumer and1MiB serialized queue
limit. Receiver/exporter retries each have30-second finite bounds; these are
separate retry stages, not one proven end-to-end deadline. The exporter names
only the existing private Loki OTLP endpoint with mTLS and TLS1.2 minimum.
Offsets and queue share a proposed64MiB state filesystem. Configured queue bytes
are not a proof of database/compaction working-space bounds. Parser on_error=drop
needs actual loss/invalid-input observations before admission. Internal metrics
are disabled in this validation-only configuration; delivery telemetry must be
explicitly enabled and checked before any comparative runtime probe or selection.
No inbound telemetry receiver, Azure extension, cloud exporter, external endpoint
or debug exporter is selected. This configuration list does not establish full
binary reachability/non-applicability for the remaining findings.

**Measurement:** exact binary2425bdf5 passed its `validate --config /probe/otel.yml`
command offline, UID10001, read-only root, no capabilities/no-new-privileges,
network none,16PID,768MiB/.5CPU. Its SHA256 was checked immediately before copying
to a448MiB private executable tmpfs; state tmpfs64MiB remained noexec. Terminal
exit0/noOOM. No components were started for ingestion, no TLS connection was
established and no Node/process data was read. Config validity is not runtime
footprint, delivery, timestamp, storage, privacy or component-admission evidence.

Original first attempt was refused by exec permissions on the Windows bind
mount; second copied the verified binary but the Docker tmpfs default remained
noexec and raised PermissionError. Third explicitly supplied exec for the owned
binary-copy tmpfs and passed. The initial shell also reported an accidental
nonexistent reporting command after the retained Docker error; it has no bearing
on validation success. All three original states/output are retained in external
ardents-r171-otel-config-a; none was reclassified as a pass. The runtime probe
should use an explicit installed image rather than copy407MiB on each startup;
the copy here is disposable investigation overhead, not a proposed deployment.

Verification: make quick-check session37095 terminated zero for this research/configuration delta. All three validation containers were removed. Full candidate checks, admission, equivalent H2 ingestion and dev integration remain open.

### H2 runtime path and native metric/index inspection

**Measurement:** explicit `make tools-install COLLECTOR=otel-image` verified the
previously inspected407498914-byte binary SHA2562425bdf5, copied it into an
external bounded context, then built a linux/amd64 scratch image without a base
OS/tool/package fetch or RUN instruction. Build exit0, config ID
bfb8f2802ab8f2d9d6f024264ffb30ccbc9b97faedc0fa5be2188da0967db640.
Image/context/build receipt remain external ardents-r171-otel-image-c. This is an
investigation artifact, not an admitted or signature-verified distribution.
Original attempt-a failed before creating its output because this selected
Windows PowerShell lacked Get-FileHash. Attempt-b retained a non-complete receipt
when native Docker progress on stderr was treated as a terminating ErrorRecord;
no successful build was claimed from that attempt. The corrected installer uses
.NET SHA256 and retains both build streams, checking the actual native exit code.
Those installer failures do not become passing retries or component evidence.

**Current experimental input:** OTel replaces Alloy in the same finite Compose
profile; the service name alloy is a research orchestration alias only. Its
actual pinned image ID/collector selection is retained in the probe receipt.
The file_log receiver (filelog deprecated alias removed after native warning)
uses the explicit safe synthetic file. Native Prometheus telemetry now listens
only inside the private probe network. Only fixed accepted/refused/sent/send-
failed/enqueue-failed record counter, queue gauge and process-RSS families may
pass the existing256-sample scrape cap. Native accepted/sent counters represent
records, not source file offsets or proven final durability. Persistent queue
size/capacity use serialized bytes in this selected configuration. Missing
failure series are unavailable, not zero or Alloy retry equivalents. Backend
outage/pressure flags therefore currently refuse H2 pending separate native
error/loss semantics and measurements; this is an incomplete comparison boundary.

**Preserved failure and concrete correction:** otel-pipeline-a delivered normal
and pressure records with actual accepted44/sent44/refused0, queue size0 and
capacity1048576, but its index-label assertion failed. It incorrectly treated
query_range returned labels (including per-record structured metadata) as
indexed stream identities. [Loki structured metadata](https://grafana.com/docs/loki/latest/get-started/labels/structured-metadata/)
explicitly describes their automatic extraction into query labels.
The [Series API](https://grafana.com/docs/loki/latest/reference/loki-http-api/#query-streams)
reports stored stream label sets. The corrected assertion requires precisely
one indexed series `{job="fixture"}` and separately checks the finite permitted
returned metadata keys. Loki ignores default resource index attributes,
explicitly selects job and drops other resource/scope attributes; safe event
fields remain bounded structured metadata. Source access date2026-10-01.
The original run exited nonzero/cleanup0 and remains external otel-pipeline-a.

**Measurement:** otel-pipeline-b completed0/cleanup0 with19 selected input hash/
size snapshots stable. Actual Series API returned only job=fixture; native
accepted/sent counters were observed and nonzero. Silence12s/suppression/expiry,
threshold recovery, and source/collector freeze pending/firing/recovery all
passed. Post-freeze catch-up watermark116 returned117 unique records, no query
duplicates, preserving original source timestamps within the existing1000ns
check. Shared180-second Grafana query produced metric59rows/log118rows and an
observed10000ms source gap. These are actual datasource frames, not browser
rendering or novice usability.

The sender drained/stopped cleanly0/noOOM before backend restart. Restart
retained60metric samples/118log keys/one silence; source advanced24events and
new receiver accepted21records, rejecting a full historical reread. Subsequent
catch-up through watermark144 returned144 unique records without query duplicates
and preserved source time, attempt2. This is planned restart with anchored RAM
state, not arbitrary crash/daemon/host persistence or exactly-once delivery.
Retained private evidence totals1195908bytes; allocated backend-state files
1847296bytes (live non-atomic sample excluding Grafana/tmp, fixture and Docker
service logs). All run containers were removed. Original reports/receipt are in
external ardents-r171-otel-pipeline-b. quick-check77576 terminated0 before the
separate healthy resource probe; no full candidate check/admission/dev merge.

**Primary advisory clarification during H2 measurement:** GO-2022-0635 and
GO-2022-0646 describe the legacy AWS S3 encryption client's algorithm negotiation
and CBC padding-oracle paths, requiring access to the relevant encrypted S3
objects/decryption path. They are not generic failures of every AWS HTTP request.
The Crypto client V2 recommendation is not interchangeable with finding an
aws-sdk-go-v2 module in the distribution. GO-2026-5932 specifically identifies
unmaintained x/crypto/openpgp packages; it is not a blanket claim about every
x/crypto package. These prerequisite descriptions guide the remaining source/
configuration inspection, not an already reviewed non-applicability decision.
Primary Go vulnerability DB files and upstream git blob receipts are external
ardents-r171-otel-advisory-review-a, accessed2026-10-01:
[GO-2022-0635](https://github.com/golang/vulndb/blob/master/data/osv/GO-2022-0635.json),
[GO-2022-0646](https://github.com/golang/vulndb/blob/master/data/osv/GO-2022-0646.json),
[GO-2026-5932](https://github.com/golang/vulndb/blob/master/data/osv/GO-2026-5932.json).

**Exact buildinfo clarification:** the binary declares hamba/avro/v2 v2.31.0
replaced by iskorotkov/avro/v2 v2.33.0, checksum
h1:fbscLHxRFT4QBPQp6MuYEL2Ao9o89KPAWYCHQ//5K18=; it also separately declares
iskorotkov/avro/v2 v2.33.1, checksum
h1:/tyfa5IFPNDkeB59kuyu9fyPZ3aiwv9YsMZwd39CNg0=.
Seeing the later version does not remove the earlier replacement or close its
three scanner findings. Existing raw buildinfo/scan remain the authoritative
inputs. Both relevant source/import paths and any admitted configuration must
be assessed rather than treating the distribution as containing one Avro version.

**H2 healthy-window measurement:** otel-resources-a completed0/cleanup0 with
nineteen selected source/config snapshots stable. Healthy interval302.9017528s,
37samples, sampled peak process RSS947097600bytes (903.22MiB), mean sampled CPU
2.695135135percent of one core, combined enforced ceiling0.96core. No query
helper or other R-171 container ran during that seven-container window. RSS
includes Grafana's plugin children; daemon/host CLI overhead, integrated CPU
accounting and unsampled peaks are excluded. Native collector RSS at the final
sample219152384bytes; complete-stack limits remained below4GiB/4CPU.
Full healthy history retained150zero-queue samples and fixture/collector up1
without observation gaps. Final catch-up covered watermark452 with453unique
records/no query duplicates and preserved source time on attempt1.
Allocated backend-state files2154496bytes; private evidence before the small
comparison receipt1452965bytes. Quota capacities and exclusions remain
as in H1; anchored RAM survives selected container restart only. Reports and
receipt are external ardents-r171-otel-resources-a. No surviving run containers.

**Comparison scope:** exact fixture/helper and four backend image IDs, memory
and healthy-window CPU caps match resources-a: True. Selected collector,
collector metrics and Loki's OTLP mapping differ by design. H1 sampled peak
945844224bytes/mean snapshot CPU2.222972973percent; H2's whole-stack observations
do not demonstrate a useful footprint improvement. One window per configuration
is not a statistical performance claim or proof of a general regression.
Choose from failure visibility, privacy and maintenance closure as well as these
measurements; H2 backend/pressure/loss and remaining advisory admission are open.
The final naming-only receipt correction calls the native H2 restart counter
collector_new_accepted_records; earlier captured reports retain their original
collector_new_read_lines label, which in H2 means accepted records, not byte
positions. No assertion logic or old report was changed by that correction.

### H2 native backlog and failed-operation semantics

**Exact source inspection, accessed2026-10-01:** binary buildinfo declares
exporterhelper0.162.0 checksum h1:PYOhmghgbrcmaWMUVi3MLMWFkwR9o+J5JdVqO4Vwj1Q=.
The corresponding tag's BaseExporter constructs timeout, retry, observation,
then queue wrappers. obsReportSender adds send_failed records only when its
wrapped retry operation returns an error; this is not a retry-attempt counter.
obsQueue adds enqueue_failed records on failed Offer; a downstream rejection
may still be retried by the selected receiver. Persistent queue onDone keeps a
shutdown-marked dispatched item for restart instead of deleting it. Therefore,
failed-operation records and a generic Dropping data log are not sufficient in
every lifecycle to prove permanent record loss. Actual sequence coverage,
shutdown/error context and storage outcome must be inspected.
Primary selected source bytes/git-blob receipts remain external
ardents-r171-otel-delivery-source-a:
[wrapper order](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/exporter/exporterhelper/internal/base_exporter.go),
[operation counter](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/exporter/exporterhelper/internal/obs_report_sender.go),
[retry path](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/exporter/exporterhelper/internal/retry_sender.go),
[queue offer observation](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/exporter/exporterhelper/internal/queue/obs_queue.go),
[shutdown persistence](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.162.0/exporter/exporterhelper/internal/queue/persistent_queue.go).
This tag inspection is not a signed reproducible source-to-binary proof or a
completed package/security admission review.

**Probe criteria, encoded before start:** capture drained baseline; stop owned
Loki; retain native signal/availability/Prometheus/Alertmanager observations
through backlog pending/firing, continued accepted records, stalled sent records
and positive queue bytes. Require independent fixture/collector availability,
no observed reset or terminal operation/enqueue-failure increase, then restore
Loki and require complete source-watermark/time coverage, drained queue and rule/
manager clearance. Missing failure series remain null/unavailable. Subsequent
ordered restart keeps its original history checks. This evaluates the selected
short outage; it does not establish extended outage or retry-exhaustion safety.
The existing H1 retry/drop assertions are retained for Alloy. A sustained queue
alert is scoped to log_delivery and names serialized-byte backlog, not permanent
loss or Node workload. Storage-pressure H2 remains refused pending its own probe.

**H2 short backend-outage measurement:** otel-backend-outage-a completed with
complete=true, cleanup0 and twenty selected source/config inputs stable.
During the unavailable backend interval the source and collector remained up1;
accepted136/sent127 records, queued2889 serialized bytes against1048576 capacity.
Pending and firing were observed, including Alertmanager activation, while the
last three sent-count samples stalled. send_failed/enqueue_failed series were
absent and explicitly retained as null/unavailable, not inferred zero.
After recovery accepted156/sent156 and queue0 were observed; the backlog rule
and Alertmanager cleared. Independent source coverage passed watermark155 with
156unique records, no query duplicates and original event time on attempt1.
After the subsequent ordered restart coverage passed watermark183 with183unique
records, no query duplicates and preserved source time on attempt1. The count
is the captured producer set, not an inferred zero-based cardinality.
Reports/receipt and the successful quick-check log remain external
ardents-r171-otel-backend-outage-a. All run containers were removed. This proves
only the selected short outage/recovery journey; positive terminal failure
counters, retry exhaustion, queue/state saturation and abrupt crash are still
unmeasured. No full candidate admission, product-source integration or dev merge.

### Ready Grafana diagnostic view probe

The synthetic provisioned view now separates source observation age in seconds,
measured pending/firing alert history, source/collector scrape availability and
readable severity/source/event/sequence log messages. Formatting happens at
query time; stored safe JSON and source timestamps remain unchanged. Absence of
ALERTS is not treated as a recovery receipt, and silence does not alter measured
rule states. There is no new incident store, acknowledgement claim, automatic
bug search or product authority. Existing queue and event counters retain their
explicit synthetic scope. Empty series remain unavailable and graphs do not
connect missing observations.

The shared-interval probe queries the exact formatted log, age and alert
expressions through the actual Grafana datasource API in addition to its existing
queue/gap query, requiring nonempty frames inside the same bounded interval.
This is an API/data-path assertion, not browser rendering or usability acceptance.
A fresh random Grafana secret key is created in its private role directory and
read through the file configuration provider; it is distinct from the generated
admin password and never printed. Restart reuses the same key while private
state is retained; this is not host-reboot storage qualification.
Primary configuration/query references, accessed2026-10-01:
[Grafana configuration](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/),
[Loki query-time JSON and line format](https://grafana.com/docs/loki/latest/query/log_queries/).

**Grafana view data-path measurement:** grafana-view-a completed0/cleanup0,
selected source/config snapshots stable; quick-check33593 completed0. The actual
Grafana API returned one shared180s interval: queue59rows with a12000ms source
observation gap, age59rows, six alert frames/46rows containing both pending and
firing, and120log rows. All120lines matched readable severity/source/event/
sequence format. This manual inspection used the actual frame schema Line field;
it is not a rendered screenshot or an automated field-level assertion. The
private secret-key file contains64hex characters; contents were not printed.
After ordered restart, Grafana datasource/dashboard access and metric/log/silence
history remained valid, with final producer catch-up passing. All run containers
were removed. Original API responses and receipt remain external
ardents-r171-grafana-view-a. Real-process integration, browser rendering, durable
incident acknowledgement/history and candidate admission remain unproven.

### H2 official-component minimal distribution investigation

**Question within H2:** can official OTel Collector Builder assemble only the
current file receiver, filesystem queue storage and OTLP HTTP exporter, reducing
unused dependency review without first-party Collector code? This remains an
investigation variant of the same collector comparison, not a third maintained
collector or an accepted technology selection.

**Predeclared next-build criteria:** generated Go module/source, dependency
cache and binary stay outside Git; use installed Go1.26.8 and exact upstream
components, no first-party components/replacements. Inspect actual generated
component factories, go.mod/go.sum, executable buildinfo/license/support/
advisory closure; do not infer dependency exclusion from a short manifest alone.
The candidate must accept the existing bounded H2 configuration and preserve
its privacy, delivery/error, alert, quota and resource requirements. Absence of
legacy AWS crypto, affected Avro or OpenPGP in actual build closure would reduce
specific review work, not prove overall security. Reuse runtime evidence only
where exact identity/configuration permits it; a new binary needs actual checks.

**Primary facts, accessed2026-10-01:** official OCB documentation supports
assembling upstream components from a manifest. Exact release cmd/builder/v0.162.0
was published2026-09-29T12:35:37Z. linux/amd64 asset8417442bytes has published
SHA2567c74640d726f23689d8853e0d5a55707ad8b524417ca7416c036c4ecb9ddb01f.
The checked downloaded bytes match. Buildinfo records Go1.26.8, builder module
v0.162.0+dirty, revision62cdad2ea133239380b44d20d84eb26e114779b6,
vcs.modified=true. Matching digest is not signature or reproducible-build proof.
Builder has not been executed. Exact-tag upstream license is Apache2.0.
Selected manifests/readme/license/API responses remain external
ardents-r171-ocb-source-a; downloaded artifact/metadata/receipt remain external
ardents-r171-ocb-public-a. No maintained admission is granted.

The minimal manifest registers filestorage0.162.0, filelogreceiver0.162.0,
otlphttpexporter0.162.0 and fileprovider1.68.0, matching exact upstream component
module versions. It excludes remote configuration providers and additional
receiver/exporter factories by declaration; actual dependency exclusion is still
unmeasured. Generated module identity is a disposable example.invalid path.

**Tool scan measurement:** ocb-closure-a exit2 was an invalid scanner environment:
runtime/cgo pthread_create failed under pids16, before finding completion. Raw
partial JSON/stderr/exit remain preserved. ocb-closure-b used pids64,
GOMAXPROCS2 and Go DNS, same512MiB/0.5CPU/90s/no-capability/read-only envelope;
govulncheck1.1.4 binary/symbol mode completed0 with no finding records, database
last-modified2026-09-28T16:43:40Z. This later pass does not erase the first failure
or prove support/security of the resulting Collector. Public build overhead is
separate from the monitoring runtime budgets. quick-check69678 completed0.

Primary references:
[OCB documentation](https://opentelemetry.io/docs/collector/extend/ocb/),
[exact Builder release](https://github.com/open-telemetry/opentelemetry-collector-releases/releases/tag/cmd/builder/v0.162.0),
[exact upstream distribution manifest](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/v0.162.0/distributions/otelcol-contrib/manifest.yaml),
[exact Builder readme](https://github.com/open-telemetry/opentelemetry-collector/blob/cmd/builder/v0.162.0/cmd/builder/README.md).

### Minimal official-component assembly receipts

The explicit tools-install recipe uses the checked upstream Builder and installed
Go1.26.8 in one3GiB/2CPU/pids128 non-root read-only container; no capabilities,
product input, authority root, host PID/network or Docker socket. Builder has a
600s deadline. Executable64MiB temporary storage is only for the checked Builder;
module/build/scratch cache is a separate finite2GiB non-executable tmpfs. Public
modules use the Go proxy/checksum service; toolchain auto-download is refused.
Generated module/source/sums/binary and receipts stay outside Git. Retained-output
128MiB assertion runs after compilation and is not a disk quota during compilation.
Build overhead is not silently substituted for the monitoring runtime envelope.

**Original failures retained:** minimal-build-a refused execution of /tmp/ocb
because the temporary mount lacked explicit exec; no assembly occurred.
minimal-build-b then reached official source generation and compilation but failed
ENOSPC because Go scratch defaulted to the64MiB temporary mount. That partial
generated module/factory/log/receipt is retained. The correction routes GOTMPDIR
to the finite cache mount and declares its2GiB ceiling. Generated factories
already show only filestorage/filelogreceiver/otlphttpexporter plus selected file
provider; actual completed binary/dependency closure is still required. Partial
module metadata includes Kubernetes dependencies from upstream: a tiny factory
list must not be represented as a tiny dependency set. No failures were waived.

**Completed assembly:** minimal-build-c completed0 in87.14s; Builder manifest
SHA25634e0c596710fc497f69887f7e5fb0a4d99c46e2389ea2b022e0096a2872e1f8f.
Actual executable38748322bytes, SHA256
1c3642b56275cd644e4adb986fdd371a1e7fd032f03785fc08a6baec758fa12a;
retained public outputs before buildinfo38814134bytes. Buildinfo records176dep
lines and no aws-sdk-go/Avro modules, while x/crypto0.57.0 remains. The full
contrib executable was407498914bytes. This establishes smaller artifact/closure,
not runtime RSS improvement, package-level OpenPGP exclusion or security admission.
Raw generated source/go.mod/go.sum, binary/buildinfo/log/receipt remain external
ardents-r171-minimal-build-c. govulncheck1.1.4 actual binary/symbol scan completed0
with no finding records, DB2026-09-28T16:43:40Z, external
ardents-r171-minimal-closure-a. Clean scanner output does not replace remaining
source/license/support/advisory and runtime admission evidence.

**Configuration incompatibility found before runtime:** a first validate command
was refused by Docker because an unused bind source was empty (exit125, no target
launch); invocation receipt is preserved. The corrected bounded offline launch
then executed minimal-build-c's exact binary and refused the existing H2 config:
DefaultScheme not found in providers list (exit1). The generated main registered
only fileprovider but inherited the Collector default env scheme. This is a real
minimal-distribution configuration mismatch, not a passing compile assertion.
The exact upstream Builder README documents conf_resolver.default_uri_scheme;
builder.yml now explicitly selects file, keeping local configuration only.
A fresh assembly/identity/scan/validate result is required; prior C scan cannot
silently qualify the changed D binary. Original failures remain external
ardents-r171-minimal-closure-a. No maintained runtime has been admitted.

**Corrected local resolver measurement:** minimal-build-d completed0 in104.02s,
manifest SHA256dba2cbcc49bb2d034ce8a03130d7a3146db518ab2808fe463942f3075f714775.
Actual binary38748322bytes, SHA256
f5be238d7d0a3d88d620bbcb8a8d15e652e4ba3d59cb244d58a018c06948a646.
Fresh exact-binary govulncheck1.1.4 scan completed0/no finding records, and the
existing H2 configuration validate completed0 without adding env or remote
providers. Source/build receipts are external ardents-r171-minimal-build-d;
scan/validate receipts are external ardents-r171-minimal-closure-b. This fixes the
resolver incompatibility only. Actual pipeline delivery/restart/pressure/privacy/
resource checks, package support/license closure and maintained selection remain
required; successful validate does not assert runtime behavior or dev integration.
