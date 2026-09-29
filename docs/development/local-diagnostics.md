# Local Docker diagnostics

This owner defines the engineering diagnostic environment, its Interface and
limits. Commands and reproducible recipes live in
[`scripts/diagnostics/README.md`](../../scripts/diagnostics/README.md).
It does not qualify installed Ubuntu workers, replace `make check`, or select
additional Network behavior. Product, security and technical owners retain
Route/currentness/authority, workload and confinement requirements.

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

Collector limits: 16 KiB per input line; 256 recent lifecycle/pressure transitions and one latest periodic resource sample; 16 MiB per
raw stream; 4 MiB per event/sample file; 1024 observed group processes; one-second
sampling; maximum caller duration 24 h. Saturation continues pipe draining and
increments independent loss counters. A disk/write failure is retained and makes
the outcome incomplete. No backend exporter queue exists. Each run is finite;
there is no unattended retention service. Arbitrary supervised commands and Go
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
The ordinary runner uses UID 10001. The selected gate profile permits the
existing package-e2e sudo prerequisite within a disposable container.
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
signals include owner-typed opening-stage duration, retained queue/credit and
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
