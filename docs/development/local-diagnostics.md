# Local Docker diagnostics

This owner defines the engineering diagnostic environment, its Interface and
limits. Commands and reproducible recipes live in
[`scripts/diagnostics/README.md`](../../scripts/diagnostics/README.md).
It does not qualify installed Ubuntu workers, replace `make check`, or select
additional Network behavior. Product, security and technical owners retain
Route/currentness/authority, workload and confinement requirements.

## Continuous monitoring and explicit debug mode

The selected engineering direction is continuous local Node monitoring with
console and rotating file logs, metric history, local alerts and an explicit
debug mode. The finite command collector described below supplies diagnostic
evidence; its file caps and saved reports do not implement this system.

### Normal monitoring

- Select each local process or Node explicitly. Show source identity, observation
  time, age, collection failure and unavailable fields. Process survival and an
  earlier ready event cannot establish present product readiness.
- Preserve structured console output and provide a private file sink with size
  and time rotation, finite retained bytes, file count and age. Rotation applies
  only to logs. A bounded sink must keep draining producers during saturation;
  report loss, full disk, write and shutdown failures independently of that sink.
  Join producers before bounded final drain and close. Reopening after restart
  must preserve the retention budget rather than restart its accounting at zero.
- Capture metrics with units, source scope and availability. Distinguish gauges,
  cumulative counters, interval rates, current values, observed peaks and actual
  configured limits. Counter resets and missing samples break rate continuity.
  OS process, process group, container and Node duty observations remain distinct.
  Unpopulated admission fields are unavailable, even when their encoded default
  is zero. Role-specific Usage values need their owner's meaning; they are not
  automatically waiting queue length or generic workload.
- Render time series against a shared selectable time interval. Show gaps rather
  than interpolate healthy operation through missing evidence. Each plotted
  value must be inspectable with its units and observation time.
- Evaluate local alerts over measured signals with explicit windows. Retain
  pending, firing and resolved transitions, deduplicate repeated evaluations,
  support acknowledgement and expiring silence, and retain recovery history.
  Silence and acknowledgement do not change measured health. Source loss and
  collection failure are independent visible conditions. External notification
  delivery requires separate explicit configuration and authorization.

Delivery observations retain their scope. A local sink with no lost bytes does not
prove complete backend history. Queue drain and resumed delivery do not restore
records already lost. Terminal failed records, retry attempts and enqueue failures
are distinct signals. An unavailable counter cannot become measured zero; a
counter reset cannot erase the need for retained incident history.

Normal monitoring must not enable expensive profiling merely to read counters.
Its retention budgets include indexes, temporary rotation files and backend
working space; a backend retention setting alone is not a filesystem quota.

### Selected-source live log implementation

The `monitor` engineering command supervises one explicitly supplied local
command and keeps safe console delivery, private retained log segments, an
independent bounded status writer and an optional live loopback panel. Its safe
source label and actual PID identify the selected local process; no Network
readiness or cross-owner attribution is inferred. Collector heartbeat and last
producer output have separate ages. A quiet stream is not a failed Node verdict.

The panel reads bounded supervisor memory, so file or snapshot failure remains
visible without reading the failed sink. Source exit stays visible until the
operator stops monitoring; periodic log age pruning remains active through that
terminal-panel lifetime, with its own visible failure. Tail filters and pause affect the view only. Declared
log retention enforces size, age, count and bytes across restart; loss counters
are session-local, and intentional retention expiry is separate from failed
delivery. The independent status file plus its one replacement are each at most
64 KiB in addition to the log payload budget. Queues and the memory tail (at most 64 rows and 48 KiB of serialized rows)
are finite. Filesystem quota remains the operator's responsibility; no power-loss
durability is promised. A failed console stops retrying writes while continuing
to account discarded records. Snapshot I/O does not block source cancellation;
bounded sink joins retain timeout and write/close failures rather than declare
incomplete cleanup successful. Uninterruptible OS I/O can exceed an individual
worker's join bound and must remain a failed outcome.

Structured stdout and stderr use the shared safe projection. Unknown or oversized
output becomes fixed notices; raw stream retention is explicit private engineering
capture, never HTTP access. Source arguments and private raw paths do not enter
the live status. Continuous metric history, local alerts and the final
monitoring/administration interface are not supplied by this log slice. Limits,
restart refusal conditions and console prerequisites belong to the command
[recipe](../../scripts/diagnostics/README.md#continuous-logs-and-live-monitoring).

### Selected-source metric endpoint

The existing optional monitor panel also exposes GET /metrics from the same
bounded supervisor memory. It opens no raw files and enables no sampling or
profiling. Fixed unlabelled signals expose supervisor age/freshness, observed
process survival, independent delivery/retention/cleanup failures and session
loss/intentional-expiry counters. Source names, PID, arguments, paths and event
tails are excluded. A three-second stale supervisor omits current source and
delivery-health values; process survival is not Node readiness. Session start
identifies a reset boundary but does not itself provide gap-safe rate queries.

Native Linux Node resource export requires an explicit positive
-sample-max-age budget (at most one hour) from the selected producer interval.
Zero, the default, disables resource values without inventing a freshness
budget. Producer and receipt times must both be fresh and within the supervisor
session; receipt time cannot refresh an old producer observation. Missing fields
remain absent, stopped/stale sources lose resource values, and invalid
observations return503 without partial successful exposition.

The fixed catalogue exports only cumulative cgroup CPU time and current cgroup
memory, which both current Node measurement paths populate. CPU microseconds
become cumulative seconds; cgroup memory includes kernel and cache charges and
is not process RSS. The resource event does not identify whether the extended
process sampler or the closed owner-cgroup sampler supplied it. Consequently
encoded zero or positive values for Go memory, descriptors, sockets, threads,
goroutines, socket memory, PSI and memory-event counters cannot establish
availability and are excluded until producer measurement coverage is explicit.
RSS/admission and role-specific timer/queue/storage/Hosting Usage values remain
excluded pending their own source scope and semantics. A fresh sample does not
mean every serialized field was measured.
This endpoint retains the panel's loopback Host/origin protections,16KiB metric
response cap and existing bounded HTTP lifecycle. An arbitrary container Host
cannot bypass those protections; separately deployed collectors require an
explicit protected integration, not a panel-protection waiver. This endpoint
does not supply a Prometheus deployment, historical store, Loki delivery, alert
consumer or installed Node qualification.

### Protected collector metrics connection

The monitor can separately expose its memory metrics to one selected diagnostic
client using -metrics-listen, -metrics-certs and -metrics-client-pin together.
This listener serves only GET /metrics; it does not expose panel status, raw
logs, profiles, certificate files or product control. The existing panel's
Host/origin guard remains unchanged.

The private canonical certificate directory must be owned by the monitor UID
with no group/other permissions. Its fixed server.crt, server.key and
client-ca.crt files must be regular, private, owned, single-link and at most
64 KiB each. Symlinks and special files are refused. Loading finishes and
closes files before opening the listener. Use dedicated diagnostic certificates,
never product transport or authority keys.

TLS 1.3 verifies the client chain against the explicitly supplied CA and
requires the SHA256 pin of the selected client's DER SubjectPublicKeyInfo.
The client must independently verify the server CA and address. Request-time
certificate validity is rechecked on retained connections. Trust and key changes
require monitor restart; no hot reload or revocation service is supplied.

Addresses are loopback unless -container explicitly permits 0.0.0.0 in the
selected diagnostic container. Four connections, header/read/write/idle
timeouts and the existing 16 KiB response limit bound the endpoint. Unexpected
listener failure is retained independently as metrics_listener_failed;
unauthenticated peer details and keys do not enter normal logs. No source
restart or extra product authority follows from collector failure.

Metrics-only monitoring retains the terminal source observation and periodic
log pruning until cancellation, as the panel does. Listener shutdown is joined
and its errors remain in the command outcome. These connection checks do not
prove Prometheus deployment, backend admission, actual Node integration or
host/power-loss durability.

### Debug mode

Debug mode adds a finite, explicitly selected capture for one local owner:
actual operation stages, context budgets and joined cleanup; goroutine stacks;
CPU, heap, block, mutex and Go execution trace profiles. Show capture duration,
progress, completion, cancellation, overhead settings and failures. Keep raw
artifacts private and summaries separate from raw profile or log access.

Debug mode must not introduce a lawful-interception capability: no content
interception endpoint, decryption-key export, impersonation, protection bypass,
or additional grant to observe another participant's Application data. Node
administration and diagnostic access must not confer those capabilities. Product
payloads, credentials and key material do not belong in normal logs or projected
monitoring output. Explicit local engineering raw captures and memory profiles
can contain sensitive data; private placement is a handling requirement, not
proof that those artifacts contain no secrets. Diagnostic changes must check
these boundaries at their actual producer and consumer seams.

Debug instrumentation must preserve authority, wire behavior, cancellation and
cleanup errors. It cannot make an incomplete operation successful, retry product
operations automatically or weaken the common Route protection baseline.

### Panel and verification

The current saved-run panel is a temporary engineering interface. Its layout
and interaction model are not the design baseline for the future monitoring or
Node administration panel; that interface requires a complete redesign around
operator tasks.

The primary views are Overview, Logs, Metrics, Alerts and Debug. Saved command
runs are supporting evidence within these views. Suggested checks, test recipes
and failure reproduction belong to developer work, not the normal monitoring
workflow. Monitoring observes selected sources and evaluates declared alert
rules; it does not autonomously search for or repair product bugs. Run comparison
belongs in a separate developer tool: name both runs and their conditions,
render units and meaningful differences, and distinguish incomparable conditions
from measured change without presenting a speedup as established. Future Node control actions
require their own existing owner and authorization boundary; monitoring does
not confer permission to change Node configuration or lifecycle.

Verify the system with a running selected source: follow live console and file
logs through rotation and restart; induce sink saturation and full disk;
inspect measured gauge/counter/reset behavior and stale-source gaps; drive an
alert through pending, firing, silence expiry and recovery; cancel a debug
capture and check joined cleanup. Retained original failures must remain
inspectable. A static dashboard or successful command report cannot substitute
for these lifecycle checks.

## Existing observations and gaps

| Owner | Available evidence | Gap / interpretation |
|---|---|---|
| Node `lifecycle_event.go` and `resource` | Bounded lifecycle transitions, pressure decisions, Go/process/cgroup/socket/queue/admission samples, Hosting used/reserved/remaining values | No global network health verdict; resource samples must not displace transitions. Assignment/generation/digests and free-form reasons are local sensitive observations. |
| Endpoint `participant_observation_linux.go` | Readiness, permission-required, connection/refresh/withdrawal failure; observer failures retained in runtime terminal result | No per-operation latency histogram or runtime queue/token gauge; unavailable evidence cannot be invented from stderr or process existence. |
| Source command events | Ready, wave accepted, terminal background/cleanup categories | Process survival does not establish current State or Source readiness. |
| `internal/diagnostics/timeline` | Existing safe streaming operator timeline, direct JSON/journal input | Human-readable projection; no resource/time series store. Continue using its existing command for exact timeline semantics. |
| qualification command | Set-wide stream/latency/throughput/resource verdicts and invalidations | Purpose-selected installed environment; Docker fixture results cannot replace its verdicts. |
| Endpoint Role/heap capture tests | Isolated role durable-state observations and Go heap dumps | Secret-bearing, explicit profiles and inputs; not routine always-on collection. |
| test/process/race/fuzz owners | Go JSON, timeout stacks, race reports and bounded mutation scenarios | Separate checked profiles; a passing subset is not a full gate. |

The local collector joins currently available event and numeric resource
observations with independent process-group RSS, threads, FDs, CPU jiffies and
TCP/UDP namespace counters, container cgroup memory/CPU/throttle/OOM and PSI
pressure snapshots. Missing optional observations remain named unavailable. It recognizes Node, Source and Endpoint schemas,
including Go test output and journal MESSAGE wrappers. Only fixed categories
and numeric field names reach the dashboard/metrics; raw identity fields,
addresses, bodies, request/assignment digests and free-form reasons are omitted.
Observed time is collector time; supplied owner UTC remains separate. It does
not create cross-role trace IDs or infer a failing role from network timing.

The explicit `ARDENTS_DEBUG_SOCKET` mode in `ardents` and `ardents-node` supplies
current Go-runtime counters and private profile capture. This is a diagnostic
process Interface, never an Application/Network Interface or an authority.
Empty input creates no listener, goroutine, profiling rates or capture files.
Configured diagnostics refuse before work on unsupported platforms or unsafe
placement. The parent must exist, be canonical without symlinks, owned by the
process UID with mode 0700; the Unix socket has mode 0600. Existing paths are
never removed to obtain admission. Cleanup removes only the socket object
owned by that generation, retains errors, and joins an active capture.
No TCP debug listener, automatic remote export or worker escape is introduced.

## Diagnostic lifecycle and bounds

1. Explicitly build/install tools and image; inventory actual image ID, tool
   modules/compiler and Debian package versions. Normal checks install nothing.
2. Mount exact source read-only, private evidence outside Git and named caches.
   Record source revision and SHA-256 of actual tree content, including local
   changes, curated environment and time budget. Command arguments are retained
   only with explicit raw capture; otherwise their exact reconstruction remains
   the caller's obligation. Do not pass secrets as command arguments.
3. Create a new evidence directory; never overwrite an earlier failure. Start
   one owned process group and continuously drain both pipes. Snapshot a bounded
   safe event tail and independent observations once per second. The source
   tree hash, not HEAD alone, identifies a modified candidate.
4. Preserve command exit, caller interruption, whole-run timeout and collection
   failure separately. At deadline send TERM to the group, allow three seconds,
   then KILL; terminate remaining descendants and bound output drain. Failure,
   timeout, saturation or incomplete observation cannot become success.
5. Analyze private profiles offline and compare repeated *declared* conditions.
   Keep race and performance runs separate. Delete sensitive evidence explicitly
   after diagnosis according to the local owner's retention decision.

Finite command collector limits: 16 KiB per input line; 256 recent lifecycle/pressure transitions and one latest periodic resource sample; 16 MiB per
raw stream; 4 MiB per event/sample file; 1024 observed group processes; one-second
sampling; maximum caller duration 24 h. Saturation continues pipe draining and
increments independent loss counters. A disk/write failure is retained and makes
the outcome incomplete. No backend exporter queue exists. Each finite command capture is bounded; its saved-run files do not rotate.
The separate `monitor` command owns the live retained log lifecycle. Arbitrary supervised commands and Go
profiles can write additional files: provision a finite filesystem quota for
long or hostile diagnostics. Log limits are not a filesystem quota.

Live Interface: at most four simultaneous clients and one profile/trace request;
fixed runtime fields; profiles capped at 64 MiB; CPU/trace 1–30 seconds; request
read budget 2 s, write 35 s, idle 2 s, shutdown 3 s. Profiling opts into block
sampling at 1 ms and mutex fraction 10 for the diagnostic process lifetime;
CPU/trace only run on request. These costs change timing. Profiles contain
addresses/stacks and potentially sensitive runtime material, so profile capture
requires explicit `-sensitive` and remains private. Private temporary files are unlinked before the first sensitive write; failed
unlink prevents capture, and the first cleanup failure is retained in the
joined terminal outcome. Failed profile writes never return HTTP success. Runtime counters report process health, not product readiness.

## Environment and privilege boundary

Default runner: no external network, read-only source/root filesystem, no Linux
capabilities, 4 GiB memory, four CPU quota, 512 tasks and finite temporary mounts.
The full gate has a separate finite 8 GiB budget: race fixtures reached the
ordinary 4 GiB ceiling without OOM. Samples include memory/pid limits and
`memory.events:max` so pressure at a hard limit is visible before OOM.
The ordinary runner uses UID 10001. The selected gate profile permits the
existing package-e2e sudo prerequisite within a disposable container. KILL
permits that root test owner to signal and join its deliberately dropped-UID
Endpoint child; it grants no host process access.
Online advisory/module access, SYS_PTRACE debugger and NET_ADMIN/NET_RAW network
fault/capture profiles are individually selected. Network capture also grants
CHOWN/SETUID/SETGID for tcpdump to drop to its diagnostic UID. No Docker socket, host PID,
host network, authority root or user credentials are mounted. Debugger/fault
profiles operate only in the disposable container namespace; do not attach a
product installation to them. The dashboard publishes only host 127.0.0.1:8090,
serves projected records, and has no raw file browsing route.

Docker Desktop shares a VM kernel/cgroup and virtual NIC. Namespace TCP/UDP
counters can include every fixture process; they are not per-peer or per-role
counters and do not measure QUIC loss or useful Application throughput. RSS of
a process group sums per-process RSS and can double-count shared pages. CPU jiffies sum the currently living group processes and may decrease when
a child exits; they are not a monotonic counter or normalized percentage.
Container cgroup CPU counters remain separate from that group sum. Short-lived processes may fall
between samples; Go goroutines require runtime observations, not OS thread
counts. Installed worker, permission-hour, real-host pressure and cross-host
privacy/availability gates retain their original prerequisites.

## Further instrumentation boundary

The package deliberately exposes missing observations as gaps. Useful future
signals outside the explicitly enabled Reader capture below include retained queue/credit and
token stock, JOIN pair lifecycle, cleanup outcome and explicit saturation.
Introduce them at their owning module with fixed categories, finite cardinality,
bounded output and adversarial behavior tests. Cross-role identifiers, targets,
peer addresses or token/body tracing need the privacy owner's explicit design;
the local collector cannot invent them or turn them into metric labels.

Recognized owner categories outside the bounded projection are represented as
unclassified/missing, increment a fixed unknown-category counter, and make
evidence incomplete. Their original value is never copied into telemetry.
For a full retained preparation-stage string use the existing private timeline
command; this panel is a bounded projection, not that exact text interface.

At terminal completion the collector rechecks source content within the same
run budget. A changed/unavailable final inventory invalidates the candidate
receipt with a fixed source-change flag. Freeze the source before an accepted
measurement; this comparison is not a transactional filesystem snapshot.

## Local diagnostic assistant

The CLI report and read-only panel use the same report builder over an explicitly
selected private run, with an optional explicitly selected comparison run.
Its Interface separates command outcome from capture validity and current
capability readiness. It returns finite facts, named missing evidence, the first
available explicitly observed failure with a local event ordinal, separate
cleanup observations and a fixed catalogue of manual next-check templates.
It neither executes recommendations nor accepts HTTP path/action selection.

The builder inspects the bounded projected event history rather than inferring
causality from the latest transition tail. Supplied owner UTC stays separate
from collector UTC/order. Malformed/truncated history or terminal count mismatch
invalidates completeness; lost history cannot establish a first overall cause.
A historical READY, a live process or a clean command exit never grants current
product readiness. Cgroup limit/OOM/throttling observations are container facts,
not attribution to one Node or proof of why an operation failed.

Manifest projection admits only exact source/image digest forms, a recognized
compiler/platform, finite time budget, fixed collector mode and explicit test
race/profiling flags. Tool versions are parsed only from fixed known inventory
formats; arbitrary inventory lines remain private. Full workload/environment
identity is absent from existing receipts and is reported as unknown.
Comparisons retain both outcomes, display known condition differences and
supervisor duration separately, and issue no speedup verdict. These records
are local observations rather than authenticated Release/qualification evidence.

Admission uses a canonical absolute directory and os.Root-confined regular-file
opens with NOFOLLOW/NONBLOCK. Manifest is at most 16 KiB, tool inventory 1 MiB,
each summary/event/sample file 4 MiB, JSON nesting 32, event/sample records 65536
and each record 16 KiB. Object keys must be ASCII; duplicate case aliases are
rejected before decoding so Unicode field aliases cannot override evidence.
At most eight failures are returned with an explicit
truncation notice. Stored fields are projected again for report, summary and
sample routes; unknown strings/maps never become report prose, HTML or labels.
Byte and record limits are not a transactional snapshot, filesystem quota or
protection against root/same-UID evidence modification. Missing terminal files
remain visible; a live snapshot older than ten seconds is stale observation,
not proof the supervised command or product exited.

Report generation success means a complete capture was explained, even when
the command failed; incomplete/unavailable report inputs give a nonzero CLI
result. Original command exit and failed comparison outcomes remain explicit.
Run instructions and exact report/compare examples belong to the README.

### Panel interaction and previewed export

The panel separates outcome, capture validity, observed failures, gaps, manual
checks, resource history and comparison. Its filter applies only to the 32-event
tail; it cannot establish a first overall failure. Pause stops browser polling,
not the collector or supervised command. Manual refresh remains available.
The RSS plot uses at most 1000 maximum-per-bucket points on observed UTC; peak
facts remain over the admitted sample history. The plot is not test timing.

Export is browser-local and bounded to 256 KiB. The operator must open a visible
JSON preview before download or copying. A browser download request is not
a save receipt; copying the same preview is the fallback. That preview is frozen across automatic refresh;
changing provenance selection invalidates it and requires another preview.
The default projection omits source/image/tool identity and absolute event UTC.
An explicit checkbox includes the already validated conditions. Both compared
outcomes, completeness, gaps, resource facts and fixed manual recommendations
remain present. Raw logs, paths, command arguments, profiles, keys, Names,
Targets and peer histories are not included. No export upload or control route
exists. Even the projected report is private operational metadata; the operator
reviews its visible contents before choosing to share it elsewhere.

### One owned Reader Connection capture

A configured process debug socket additionally exposes read-only GET /connection.
The first eligible Reader Open during the first ten minutes of that process
session claims one capture. It never rearms or selects a Target. The recorder
retains at most 64 fixed-category records in memory; expiration and dropped
records invalidate completeness without stopping product work. With diagnostics
disabled, the nil/zero trace handle returns the original Context and adds no
recording, timers, callbacks or I/O. Recording itself performs bounded memory
updates and inspects at most 64 error-tree nodes.

The production Reader route observes admission, session activation, worker launch
and operation acquisition, the actual bounded worker activation (launch gate,
readiness and Grant binding), Introduction preparation, JOIN (including the separately
observed initial TLS/native Service authentication), fixed request, document
exchange, Service/worker closure, the current-owner check when actually performed,
local response, caller-cancellation join and session release. Nested phases
overlap and their durations must not be summed. Elapsed/duration values use the
process monotonic clock. The inner worker activation observes its actual 15-second
Context and Service authentication observes its actual WorkSafety deadline Context.
The optional remaining budget is an observation of that
stage's supplied Context deadline, not all authorization, token or protocol limits;
an absent deadline is unknown.

Observation handles explicitly cross the worker's Context re-parenting using
diagnostic values only. Existing authority parents, cancellation, leases,
deadlines, outcomes and wire data are preserved. The returned readResult marks
joined completion only after deferred caller join, session release and owner
cleanup, before publishing its joined barrier. Application EOF and native retired
are not substitutes for this ownership boundary.

Error text and remote/local identities never enter the snapshot. Terminal error
classification is conservative: an untyped refusal remains failed even when its
text mentions cancellation. A separately observed context_stop records canceled
or deadline only from the supplied Context; it does not erase additional cleanup
errors or assert their cause.

CLI connection and snapshot -kind connection, and the panel's optional
connection-socket selection use one strict bounded decoder (32 KiB, exact schema,
fixed fields/categories, ordered paired durations). The socket schema is
ardents-reader-trace-v1; CLI/panel projections use the distinct
ardents-reader-observation-v1 schema with derived missing-stage and association facts. Missing/unknown/duplicate
facts cannot become joined or healthy evidence. The panel's live owner snapshot
is independent of the saved command receipt; their association is unproven.
No owner socket is inferred from a stored directory. Reader snapshots are not
included in the existing command-report export. Socket access retains its owner
Unix permissions; opting into the loopback browser panel exposes the projected
timing metadata to other local processes/users able to reach that listener.
Loopback excludes remote binding, not local adversaries or endpoint compromise.
No Network readiness, installed worker qualification or causal diagnosis follows
from a snapshot. Other Reader operations, Publisher telemetry and Node
queue/credit/token gauges remain outside this capture.

### Periodic observations and event logs

Normal monitoring routes periodic `resource-sample` observations to retained
`samples` files, separately from the event log tail and console. Status exposes
`total_samples` and `latest_sample`; these are observations, not alert decisions
or a complete metric query interface. Sample and event sequences are independent.
Non-periodic resource transitions remain events. File retention and delivery-loss
budgets remain shared and explicitly reported.

### Evidence for the debugging implementer

The implementer needs inspectable evidence from one reproduction, rather than
screen scraping or an inferred diagnosis. The existing finite report records
source revision/tree, image/tool versions where available, terminal command
outcome, first observed fixed-category failure and collection gaps. The Reader
connection observation exposes bounded stages and explicitly unproven command
association. Selected private runtime profiles remain separate artifacts.
These facilities are not yet a unified incident package.

The next tooling boundary should assemble those existing receipts with explicit
artifact references and availability/loss information. It must preserve unknown
association rather than join records by timestamp as proof of causality. Missing
operation context, error provenance or product instrumentation requires a bounded
change at the current producer owner; the dashboard cannot invent a component,
operation identifier, severity, stack or source-code location. Product payloads,
keys and credentials are not diagnostic correlation fields. Reproduction recipes
and developer checks remain engineering work, separate from normal monitoring.

### Private reproduction package foundation

The engineering `bundle` CLI assembles a new private JSON index from one explicitly
selected completed capture. It preserves the existing report's command outcome,
capture integrity and gaps separately from assembly success. The index includes
the canonical local evidence root and hashes/sizes of the four required report input
files plus the optional `tools.txt` input when present. It is private operational metadata, not a browser export or proof of causal
association. It never overwrites an existing output or alters the selected capture.

Input/output directories must be canonical, private and owned; inventoried files
must be owned private regular single-link files. Each input has a finite read
budget; assessment uses the same opened root as inventory, which is repeated
after all selected reads. Observed content/input-set changes are refused. This is not an atomic filesystem snapshot or a guarantee against a
malicious owner changing and restoring inputs between observations. Output is
bounded to 256 KiB outside the source and selected capture. A failed write may leave
an incomplete file, which is retained and cannot be silently overwritten.

An optional explicitly selected owned private Reader socket uses the existing
strict 32 KiB native decoder and safe projection. Its outcome, records, missing
stages and unproven command association are preserved. `connection_state` states
whether the selected observation could be read; `available` is not operation
success, present readiness or proof that it belongs to the selected command.
With no selection it is `unselected`. Refused/unavailable/invalid Reader evidence
produces a private partial package with `assembly: incomplete` and a fixed failure
category, while retaining valid command evidence and returning the original error.
Invalid initial capture admission or observed mutation refuses package output and
preserves the original inputs.

Up to eight explicitly selected `-artifact` basenames in the selected capture
can index private profiles, traces, the test executable or raw command receipts.
There is no directory walk, extension-based format claim, raw content copy or
artifact execution. `-artifact` explicitly selects an opaque `private-file`;
`-profile` selects `pprof`; `-trace` selects `go-trace`. Required receipts have
kind `report-input`. Selection shares the same eight-file limit. Each selected file is capped at 64 MiB; one inventory pass
is capped at 128 MiB including report inputs. Both inventory passes validate
private regular single-link ownership and observed identity/content stability.
Hash/size identify observed bytes, not a valid profile format, executable match,
operation association or a replayable reproduction. Raw artifacts may contain
sensitive data and remain private.

Explicit pprof/trace selections are parsed offline by prebuilt standard Go parsers,
using an admitted file descriptor, no selected executable or symbolization, and no remote
source or HTTP listener. The parser has a two-second deadline per selection,
64 KiB stderr budget and either 64 KiB pprof table or 64 MiB trace-to-profile
stdout budget. Output is discarded by default. Explicit `-profile-top` requires
at least one `-profile` selection and retains standard offline pprof top tables
for those selections only, at most ten rows and 16 KiB per table. The JSON
`private_pprof_top` field may contain sensitive embedded function names, build
metadata and profile comments: it stays in this private local package and is
never sent to monitoring, HTTP, Loki or a public report. No executable, source
lookup or remote symbolization is added. Trace-derived output is still discarded.
The package's 256 KiB output budget, artifact inventory, identity checks and
parser/assembly deadlines remain in force. Over-budget, empty or invalid UTF-8
top output fails selection; partial text is not presented as a successful table.
A table shows the profile's default sample type, total and flat/cumulative costs;
this does not establish sufficient sampling, operation/build association or a
cause of failure. Embedded symbols are observed profile content, not authenticated
source provenance. `validation: passed` means the
selected parser accepted the observed bytes, not semantic completeness, profile
coverage, performance diagnosis or operation/executable association. Unsuccessful
validation retains a private partial package with `validation: failed`, returning
nonzero; it does not guess whether failure was corruption, tool availability or
resource limits. Opaque artifacts have `validation: not-checked`.

Assembly has an explicit positive `-timeout` up to 30 seconds (default 10),
recorded as `assembly_budget_seconds`. SIGINT/SIGTERM and that deadline propagate
to inventories, Reader requests and parsers. Parser process groups are canceled
and waited; cancellation does not produce a successful package. Regular-file
filesystem calls remain subject to kernel/storage latency: this is a cooperative
deadline, not a hard kernel-I/O latency guarantee. A deadline during output I/O
can leave a file that must be inspected alongside the CLI error; failed writes
are never silently overwritten. The source/output named roots are checked against
the opened directories before output admission. This cannot prevent an owner
changing paths after the final observation.

Known report input schema/record validation gaps produce incomplete assembly and
a nonzero result while retaining the original interpreted command receipt.
An optional explicit `-command` selects one `command-receipt` artifact. It is a
UTF-8 JSON argv array capped at 16 KiB and 256 string entries, with a nonempty
executable entry and no null/non-string/NUL arguments. It is never executed or
copied into the package. `reproduction` records `unselected`, `available`, or
`unavailable`, a relative artifact reference and the number of argv entries
(including the executable) when validated. Availability means a readable typed
receipt, not proof of command/run association or replayability. The environment
remains `not-fully-declared`; exact flags can be read manually from the selected
private receipt. Invalid selected receipts retain a partial package with the
original command outcome and return nonzero.

Public CLI tests exercise kernel file-size write refusal in a separate process,
retained failed output/no overwrite, unsafe file/path/link/permission admission,
individual/total byte limits and cancellation of an actual in-flight Reader HTTP
request. Original captures remain preserved. Full final gates and completed
bounded review still precede integration. No product/wire
identity or interception grant is introduced by its engineering schema.

### Installed parser prerequisites

The selected Go 1.26 toolchain can build its standard pprof/trace tools lazily.
Preparation is not profile parsing and must not consume the two-second capture
parser budget. Explicit `make tools-install` builds `cmd/pprof` and `cmd/trace`
with the selected toolchain into the writable `GOBIN`, or `GOPATH/bin` when
`GOBIN` is unset. `DIAGNOSTIC_PARSERS_ONLY=1` selects only those standard tools;
it installs no third-party module. Put that directory on PATH. Missing binaries
are an invalid diagnostic environment, never a passing skip or an implicit build.
The diagnostic image and selected architecture CI install them before execution.
`doctor` and image inventory retain their compiler/build metadata. Heap-profile
and runtime-trace CLI tests use an empty Go build cache; parsing must not compile
or fetch tools. Process/output/deadline limits remain unchanged.

### Selected backend filesystem budget observation

The local observability experiment now exposes a private metadata-only
filesystem inventory for an explicitly selected Compose project. It joins only
that container's PID namespace under the same UID, with no added capabilities,
and observes fixed state, temporary and image-declared data roots. It reads no
file contents or profiles. Missing measurements remain unavailable; shared
filesystem quantities are not attributed to a process or Node. Capacity on
unbounded image-created volumes cannot satisfy a selected quota merely because
the configured database path uses another bounded filesystem.

This observer and its bounded experimental mounts are engineering preparation.
They do not select a production storage layout or establish complete disk-budget
coverage. Docker log bytes, external writable evidence and persistence remain
separate obligations. Reproducible invocation belongs to the
[experiment recipe](../../experiments/r-171-local-observability/README.md#selected-native-filesystem-inventory).

### Collector storage-refusal observation

The local backend experiment has a separate finite minimal-OTel collector-state
exhaustion profile. It uses only its owned64MiB synthetic state tmpfs and new
verified injection/activation files. Existing project containers or selected state/fixture volumes are refused before evidence creation. Generated padding and a brief selected Loki
pause require queue growth; native ENOSPC, refusal/unavailability and the actual
Prometheus/Alertmanager signal must be observed. A full filesystem alone is
insufficient evidence of collector failure. Recovery preserves the original
failure and separately checks queue drain and source sequence delivery; it does
not recreate the checkpoint database. Installed Node and host crash durability
remain outside this experimental acceptance boundary.

The monitoring rule `CollectorLogEnqueueRejected` reports a positive cumulative
native enqueue-refusal count for the current collector session. Receiver retries
may subsequently deliver the record, so this is distinct from terminal exporter
loss. It remains a warning after delivery recovery until the count is reset;
absence is unknown and restart can clear it. This is neither a durable incident
store nor acknowledgement history. Raw collector errors stay private; they do
not enter the safe projected Node event stream or public profile output.
### Private artifact storage observation

The experiment also measures logical file lengths in one explicitly selected,
account-owned external artifact root whose privacy remains an operator prerequisite, not an observation result. A bounded metadata-only double
observation refuses redirects, changes and the selected 128-MiB overrun, retaining
an aggregate private receipt. It does not read payload/profile content. This
snapshot neither enforces a continuous quota nor proves physical allocation;
Docker logs and filesystem metadata remain separate coverage. Invocation and
limits belong to the experiment's private artifact observation recipe.
