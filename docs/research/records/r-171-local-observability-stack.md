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
during the full-disk interval, nor a pending sample for this new storage rule.
Those predeclared evidence items remain missing; passing implemented assertions
is not full storage-pressure admission. The probe does not crash Loki while
writes lack WAL persistence, exhaust extended retries, prove a host/daemon
restart, or establish future log durability from HTTP acceptance. Source events
and limits are synthetic; no actual Node/Application inputs were admitted.
The extra authenticated backend scrape selects only the three named WAL metrics,
with a thirty-two-sample budget. No arbitrary exporter or raw-file discovery was
added. Complete component selection, advisory dispositions, real-source integration
and required completed-slice gates/review remain open under #394/#377.