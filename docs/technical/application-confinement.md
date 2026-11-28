# Document Application confinement

Status: **selected closed Ubuntu design**, for the Product Owner's
[document workload](../product/protected-service-workload.md).
R-099's job/platform prerequisite is now resolved for this scope.
No Windows, generic browser or arbitrary-Application profile is admitted.

New Execution command and lifetime composition are portable, with a separate
native worker adapter. The unsupported-platform adapter yields only refusal
and no activation, artifact or cleanup pin; the common owner joins its exact
refused local Job before returning cleanup capacity. Actual Windows consumer
checks require no participant input/output or durable-root effects. Repeated
refused launches exceed the 64-session capacity without leaking slots or
poisoning the local generation. This verifies refusal and portable ordering,
not a Windows confinement or Application execution profile.

## Proposed migration of signed worker names

This proposal records the cutover design. Acceptance requires a decision updating [ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md) and the qualification below.

The nine-file protected group is nested in v6 Enrollment. Its descriptor is
`ardents-protected-endpoint-artifact-v1`. Enrollment, Release authorization,
the sealed-generation inventory and installed-file verification independently
bind its file names. The installed worker artifact document
`ardents-text-worker-artifact-v1` binds the executable, four systemd units and
polkit rule by installed path and digest. The chroot, Endpoint unit
dependencies and tmpfiles declaration bind related paths.

The proposed v2 inventory uses one worker executable for the document and
byte-stream profiles. Each row below must change together at cutover.

| Resource | v1 identity | Proposed v2 identity |
|---|---|---|
| Protected descriptor schema | `ardents-protected-endpoint-artifact-v1` | `ardents-protected-endpoint-artifact-v2` |
| Worker artifact file and schema | `/etc/ardents/text-worker-artifact.json`, `ardents-text-worker-artifact-v1` | `/etc/ardents/application-worker-artifact.json`, `ardents-application-worker-artifact-v2` |
| Worker archive member | `ardents-text-linux-amd64` | `ardents-application-worker-linux-amd64` |
| Worker chroot and executable | `/usr/lib/ardents/text-worker-root/ardents-text` | `/usr/lib/ardents/application-worker-root/ardents-application-worker` |
| Four worker units | `ardents-text-{reader,publisher}.socket and ardents-text-{reader,publisher}@.service` | `ardents-application-{reader,publisher}.socket and ardents-application-{reader,publisher}@.service` |
| Runtime socket directory | `/run/ardents-text` | `/run/ardents-application-worker` |
| Dynamic worker user prefixes | `ardtxt-r-`, `ardtxt-p-` | `ardapp-r-`, `ardapp-p-` |
| polkit and tmpfiles members | `50-ardents-text.rules`, `ardents-text.conf` | `50-ardents-application-worker.rules`, `ardents-application-worker.conf` |
| Endpoint systemd unit | `ardents-endpoint.service` with v1 activation dependencies and description | `ardents-endpoint.service` with v2 activation dependencies and description `Ardents protected Endpoint` |

The Endpoint executable member and the `protected-endpoint.json` member
retain their names. The signed Release target inventory, archive, Debian
payload and installed pre-start project the exact v2 name set, paths, unit
contents and file digests. Update polkit unit matches, tmpfiles and the
runtime verification of socket paths, dynamic users and cgroups against that
same inventory. Reject mixed groups even when individual file digests verify.

Authorize a forward transition from an independently verified exact v1
predecessor. Retain the original systemd unit reference through successful
stop. Require stopped original activation sockets, empty job inventory and
empty worker cgroups. Under the retained Release lease, authenticate a strictly
newer v2 generation, preserve root identity and authority floors, and stage
its complete resource set in a distinct immutable generation. Keep the
predecessor evidence until the successor's first use has been verified.

The host delivery must record the transition durably before replacing shared
unit resources. Staging prepares the new files before switching the Endpoint
unit. At activation, require the predecessor sockets and units to be
unstartable and the loaded successor to match the complete v2 inventory,
including sockets, rule and worker chroot. An interrupted cutover stays
unavailable while resources are partial. Recovery reobserves the original
stopped generation and exact staged bytes before resuming, or admits a newly
authorized forward generation. Previously committed Release floors and the
original mutable root identities survive both paths.

Acceptance requires positive and mixed-inventory controls in packaging,
portable signed-grammar tests, and an Ubuntu/systemd transition from the exact
deployed v1. The native test covers live, uncertain and tampered predecessor
refusal, interruptions during staging and activation, retained root identities
and floors, authenticated first use, the document journey, the bidirectional
byte-stream journey and joined stop.

File content uses the sole byte stream under ADR-0134. The serialized
`text_token_root` remains a local declaration field, with no content-format
meaning in the network exchange.

## Selected boundary

Use the distribution-maintained systemd system manager on Ubuntu 22.04 LTS
(systemd 249) or Ubuntu 24.04 LTS (systemd 255), with cgroup v2 and kernel
namespace/seccomp support. Admission is the fail-closed capability floor
(Ubuntu >= 22.04, systemd >= 249) accepted under
[ADR-0126](../adr/0126-admit-platform-by-capability-floor.md), not an exact-version
whitelist. Ubuntu 22.04's systemd always creates a redundant
v1 `name=systemd` tracking hierarchy even on a unified boot; that line carries
no controllers and is tolerated while the cgroup2 placement stays exactly
verified (#641). systemd 249 omits the encrypted/imported credential properties
and `ExitType`/`RestartMode`; their absence is tolerated as the secure default
on 249 only (#642). The Endpoint
runs as its own unprivileged service account. Every reader or Publisher worker
runs under a separate DynamicUser in a separate systemd service/cgroup and
private filesystem/network/IPC namespaces. No untrusted Application shares
the Endpoint's Unix UID, state directory, credentials or administration socket.

The privileged work is performed by installed, root-owned systemd units.
Do not introduce a general privileged Go launcher, sudo command supplied by
an Application, shell command template, container daemon or AppContainer
implementation in this profile. Installation remains an explicit operator
action using the verified artifact and root-owned configuration.

The installed policy additionally grants only account `ardents-endpoint` the
systemd `manage-units` action with verb `stop` for canonical instances of the
fixed reader/Publisher families. It grants no start, restart, reload, unit-file
editing or arbitrary helper execution. The root-owned package rule resides at
`/usr/share/polkit-1/rules.d/50-ardents-text.rules`, mode 0644, so Endpoint can
verify its bytes without access to the private administrator rule directory.
Its digest is the sixth required installed-artifact entry. A missing, unreadable
or substituted rule makes that artifact unavailable. Effective authorization
and whole-cgroup shutdown still require their installed behavior checks.

Use two installed socket/service template families: reader and Publisher.
Their Unix sockets are in a root-owned directory, accessible only to the
dedicated Endpoint account. Accept=yes creates one bounded worker instance
for one local job: one reader operation or one publication lifetime. The accepted AF_UNIX descriptor is the only
Ardents attachment inherited by that worker; StandardInput=socket and
StandardOutput=socket connect the finite framed local exchange. StandardError
is null. No logging, session bus, credentials, environment strings containing
destinations, or arbitrary file-descriptor passing is available to the worker.

Each worker unit has `BindsTo=ardents-endpoint.service` and an `After`
dependency on it. The launch boundary verifies the root-owned Ubuntu 22.04 or
24.04 identity, amd64 architecture and actual system-manager version 249 or 255,
then verifies that the caller is that active service's MainPID under the
dedicated Endpoint user/group, with `RemainAfterExit=no`, `ExitType=main` and
`RestartMode=normal` (the latter two absent and tolerated as the secure default
on 249 only, #642). Unknown values
or semantics that keep the parent active after its main process exits refuse
before worker activation. This manager-owned lifetime dependency covers an
Endpoint exit that cannot run its Go cleanup; socket EOF is insufficient.

The new Execution Endpoint-death component measures this lifetime with an
external Root controller and two genuine held operations. Original pidfd
SIGKILL cannot execute Endpoint cleanup; independent kernel/proc observers
require disappearance of both original TERM-ignoring child/grandchild trees.
The qualified actor's failed signal result remains retained. Explicit fresh
start uses new Endpoint/worker invocations and local Job commitments, with
no old permission or physical observer adopted by that new opening. Hostile
descendants retain attachment descriptors despite EOF and have no parent-death
signal; their one-minute failure watchdog cannot satisfy the controller's
fifteen-second original-join bound. The [component profile](../../tests/qualification/execution-recovery/README.md)
defines the separately pinned artifacts and Root/non-root actors. These
observations supply no logical Service recovery, product installed acceptance,
power-loss or whole-host qualification.

The trusted Endpoint connects the socket and verifies the installed unit,
executable and sandbox-root identity against the selected local artifact.
The system manager applies confinement before ExecStart. Only then can the
fixed worker return its local HELLO. A per-job random nonce and the exact
accepted socket instance bind the Broker's Application Principal and Local
Grant. One private Job lifecycle owns that nonce, consumes the verified Grant
handoff, closes a rejected late Grant, retires the invocation and publishes its
first joined cleanup result. The Context retains only the exact admission
reservation; a replacement cannot inherit any of those Job-owned values.
Worker HELLO alone is never proof of isolation on an unverified unit.

The new Execution worker verifies the effective fixed activation socket before
dial and again when observing the original invocation after READY. Its pure
fixed-unit predicate requires the exact listening identity, no drop-ins,
Endpoint dependency, owner/access, finite connection limit and no command
hooks. This observation is distinct from the pinned on-disk socket bytes and
grants no Installation or runtime authority. An isolated native control with
unchanged artifact bytes and an effective reader `MaxConnections=63` drop-in
refused before activation; restoring the original effective socket allowed the
same lifecycle consumer to pass. That control does not qualify the complete
installed participant or hostile-worker profiles.

The separate fixed qualification caller creates one private qualification Run.
Only that Run owns its Init, progress observer, report destination, sampling
cleanup and retained joined-stream set. An ordinary text Job creates none of
that state; it can retain only the exact Run supplied by the qualification
caller. A delayed completion publishes to its original Run and cannot populate
a replacement Job's report.
Its predeclared SHA-256 scheduled-byte corpus is binary rather than text: the
fixed conformance window contains every byte value, including NUL and invalid
UTF-8, and exact offset verification survives unequal fragment boundaries.
Changed bytes, wrong offsets and a one-byte-short workload fail their existing
worker/verdict boundaries. These checks do not change the installed launch,
Grant, duration, connection-count or bitrate contract and do not admit another
worker inventory.

The byte-stream profile selects a distinct INIT on the same fixed installed
worker entrypoints. The signed executable must implement that INIT and
acknowledge the original nonce before Endpoint grants the Job. The existing
document INIT bytes and fixed unit command lines retain their meaning. A
released predecessor executable that lacks the byte-stream INIT refuses the
new operation during initialization.

## Required installed properties

The unit must include these effective properties; unknown or unavailable
required settings make the platform unavailable before Grant delivery:

- RootDirectory points to the immutable root owned by root. It contains only
  the verified static worker executable and empty required runtime mount points.
  WorkingDirectory=/ is explicit.
- Each normal fixed reader and Publisher template explicitly sets
  `Slice=system.slice`. The Endpoint verifies that effective Service property
  before Grant delivery, and accepts only the resulting direct
  `/system.slice/<canonical-unit>` cgroup path. A template instance's implicit
  per-template subslice, another slice, nested path or sibling never becomes
  an accepted worker cgroup. The separate stream-qualification inventory retains
  its selected qualification-owner slice.
- DynamicUser=yes; no supplementary groups; CapabilityBoundingSet and
  AmbientCapabilities empty; NoNewPrivileges=yes.
- PrivateNetwork=yes; PrivateIPC=yes; PrivateDevices=yes; PrivateTmp=yes;
  ProtectSystem=strict; ProtectHome=yes; ProtectControlGroups=yes.
- ProtectKernelTunables=yes; ProtectKernelModules=yes; ProtectKernelLogs=yes;
  RestrictNamespaces=yes; RestrictSUIDSGID=yes; LockPersonality=yes.
- RestrictAddressFamilies=AF_UNIX; SystemCallArchitectures=native;
  MemoryDenyWriteExecute=yes; deny io_uring_setup/enter/register, bpf, ptrace,
  process_vm_readv/writev, perf_event_open, keyctl/add_key/request_key,
  userfaultfd and the mount/module/reboot/swap/raw-I/O syscall groups.
- MemoryMax=128 MiB and TasksMax=32 per worker; LimitCORE=0; no restart policy;
  KillMode=control-group; TimeoutStopSec=2 seconds. The parent Endpoint/job
  resource tree imposes its stricter aggregate limit before another worker.
- CollectMode=inactive-or-failed releases completed socket-activated instances,
  including failed ones, so their manager records do not accumulate across
  jobs. Endpoint verifies this effective Unit property before Grant delivery.
  This is manager garbage collection, not proof of cgroup cleanup or a new
  runtime management permission. Failure outcomes remain in the journal;
  unloaded Unit result statistics are not retained. See the selected
  [systemd 255 socket guidance](https://github.com/systemd/systemd/blob/v255/man/systemd.socket.xml)
  and [Unit collection semantics](https://github.com/systemd/systemd/blob/v255/man/systemd.unit.xml).
- The fixed worker disables Go containermaxprocs/updatemaxprocs at build time, and its installed unit supplies GOMAXPROCS=2 and GOMEMLIMIT=96MiB. This avoids an ambient cgroup-file descriptor opened by the runtime before the inherited-descriptor audit; the system manager still enforces MemoryMax and TasksMax.
- No host filesystem/bus/device mounts, network namespace joins, notify socket,
  credential mounts, writable executable paths or external networking handles.

RestrictAddressFamilies constrains socket creation, not already inherited
descriptors. PrivateNetwork does not close an inherited external socket.
Therefore installation/activation must enumerate descriptors and allow only
the accepted local stream plus harmless null descriptors, with one verified
manager exception: a resource-limited service on systemd 255 inherits its own
cgroup resource-control file (observed `cpu.max`) without close-on-exec, as
default manager behavior for units carrying MemoryMax/TasksMax. The audit
admits such a descriptor only when its kernel-resolved `/proc/self/fd` target
is a regular file directly inside the worker's own unified cgroup
(`/proc/self/cgroup` `0::` placement) and names a resource control from the
fixed allowlist (`cpu.max`, `cpu.weight`, `cpu.weight.nice`, `cpu.idle`,
`memory.min`, `memory.low`, `memory.high`, `memory.max`, `memory.swap.max`,
`memory.zswap.max`, `pids.max`, `io.max`, `io.weight`), then closes it before
admission completes: the fixed worker never manages its own cgroup, so no
control authority survives the audit. Each allowlisted file grants only the
service's own limit management, bounded by its parent cgroup. Migration,
controller-enablement and lifecycle surfaces (`cgroup.procs`, `cgroup.threads`,
`cgroup.subtree_control`, `cgroup.type`, `cgroup.kill`, `cgroup.freeze`,
`cgroup.events`) are not resource controls and remain foreign authority, as
does every descriptor of a foreign cgroup, a nested child cgroup, a spoofed
path prefix or a non-regular file. systemd 249 passes no such descriptor and
its audit behavior is unchanged. AF_UNIX alone does not authorize access to
an unrestricted host IPC peer; the private root, private network namespace
and socket permissions enforce that exclusion. The protocol never receives
SCM_RIGHTS and grants no file-descriptor exchange.

## Data and authority flow

The trusted UI/Endpoint boundary accepts only a selected destination or an
owner-selected document snapshot. It performs syntactic checks and local
authorization before destination-dependent network work. The new versioned
Connection Interface admits Target Link explicitly and reserves a refused Name
tag for the later canonical composition; it never accepts
a path, executable, peer list, private key or administration operation.

The Publisher's owner chooses the input file. Import once using a no-follow
regular-file check, a 4 MiB read limit and a stable-read
check; copy to a bounded immutable in-memory snapshot. Transmit that snapshot
over the scoped local Publisher attachment. The worker sees neither the
original host path nor the host file tree. Publication authority and Instance
keys stay in Endpoint/Custody; receipt of a snapshot is not permission to sign
or publish a Service.

The Linux trusted importer walks the selected absolute path through no-follow
parent directory descriptors and opens the final file nonblocking before the
regular-file check. It rejects oversized input and compares a
second bounded read with the first, as well as file identity, size, mode,
owner/link count and modification/change timestamps. Timestamp equality alone
is insufficient on a filesystem with coarse change times. The returned bytes
are a private volatile copy; subsequent host-file edits do not update a
publication. This observed stable read is not an atomic filesystem snapshot
against a compromised local owner that controls concurrent writes.

The trusted UI may explicitly request its committed publication's Target Link
with `ardents link <administration-socket>`. This read-only operation sends
ASCII `link\n`[5] then directional EOF. An implementing Administration owner
returns ASCII `link\n`[5], a big-endian u16 length, exactly 1 through 512 printable
ASCII destination bytes, then directional EOF. Unsupported, uncommitted,
withdrawn or unavailable owners return `unavailable\n`. No Target or authority
is supplied in this request. Endpoint consumes fresh Administration authority,
checks the retained run, registration acknowledgement and current Publication
binding, and projects its canonical Link. The UI presents only that bounded
response through owned interruptible terminal/pipe output; it writes no history,
background event or ordinary diagnostic containing the destination. Retrieval
cannot publish, refresh, retry or resurrect a Service and promises no future
availability. Existing Publish, snapshot and Withdraw response bytes are unchanged.
The trusted publication client uses the existing private Administration socket.
Its bounded extension is ASCII `snapshot\n`[9], a big-endian u32 byte length,
exact file bytes (0 through 4 MiB), then directional EOF. There is no path,
Target, key, Principal or Grant in this request. The transport admits at most
one snapshot allocation/transition at a time and retains the existing finite
receive deadline. Withdrawal remains a separate operation. Only a publication
owner implementing the snapshot operation may commit it; `published\n` means
that owner returned success, while unsupported owners, malformed input,
overlap or failed publication return `unavailable\n`. Never translate this
request into bodyless Publish. The protected Endpoint Administration owner implements this operation through actual worker qualification and Descriptor acknowledgement. Command/transport fixture tests do not prove that installed composition or worker confinement; the complete installed command journey remains a separate qualification boundary. The
transport clears its borrowed bytes after the owner returns, so retention
requires an owner-held copy.

The reader's worker receives only its admitted Connection and bounded
application bytes. It has no destination authority or ambient network client.
The trusted presentation boundary independently escapes control/bidi bytes
before writing to a human terminal; a compromised worker cannot bypass that
escaping by emitting its own terminal sequence. No response launches a browser
or follows a URL. Data sent to the intended Service is still intended output.

Keep request data, snapshots and reader buffers in volatile memory. A worker
may have private bounded temporary storage but no persistence across jobs.
No document content enters journals, metrics, error strings or crash reports.
The Endpoint's ordinary floor/admission persistence does not store documents.

## Lifecycle and consumer seam

Startup: local authorization -> verified socket/unit activation -> confined
worker HELLO -> Principal-bound Local Grant -> existing Endpoint-owned
resolution/Route/Service work -> one bounded job result -> revoke attachment
and join worker cleanup. At every asynchronous completion check the current
job nonce; a late result cannot attach to a replacement job.

Cancellation and Local Grant revoke immediately stop remote admission and
close the worker's attachment, then terminate the complete cgroup. Withdrawal
retains only the separately authorized finite publication drain. A worker
fork inherits all restrictions; exit of its initial PID cannot leave helpers
alive. Failure to join cleanup is a failure result, not permission to reuse a
Principal, socket, private root or Grant.

Endpoint cleanup retains the original cgroup v2 `cgroup.events` descriptor
before readiness. It rechecks the exact systemd InvocationID and fixed
control-group stop policy, uses noninteractive `systemctl stop` with a finite
join deadline, and verifies that the pinned subtree has no live processes.
Cleanup reads typed properties through the original InvocationID bus path.
The [systemd 255 path resolver](https://github.com/systemd/systemd/blob/v255/src/core/manager.c#L3013-L3052)
can reload a collected unit's configuration through its name path, but an
unknown invocation path remains unavailable. Neither a reloaded name nor an
empty InvocationID grants original identity or Stop permission.
If manager properties are unavailable or identify another invocation, cleanup
keeps observing the original pinned cgroup through that deadline without
stopping the replacement. A changed invocation remains a cleanup failure even
after the original cgroup empties; manager unavailability can be accepted only
when the original pinned cgroup itself is observed removed.
The [kernel populated field](https://docs.kernel.org/admin-guide/cgroup-v2.html#un-populated-notification)
includes descendants. Removal is recognized only by `ENODEV` on that already
verified core events file, including its seek operation; a missing pathname,
zero MainPID, unchanged inode
link count, unknown observation or failed stop does not prove cleanup. The
[kernfs read and seek paths](https://github.com/torvalds/linux/blob/v6.8/fs/kernfs/file.c)
and the fixed cgroup events callback explain that removal observation. Keep
the first cleanup failure on repeated close. A cleanup owner alone does not
prove installed stop permission or qualify a Principal/Grant; installation
must establish that permission separately before protected admission.

Socket activation may expose a loaded, inactive/dead service while its start
job is queued. Endpoint waits only when the same manager inventory tuple has
a nonzero start job and its matching canonical job path. It retains that one
new candidate and the original baseline through the existing launch deadline;
a second candidate, disappearance or substitution refuses the launch. This
observation grants no readiness: active/running, exact invocation/process,
effective confinement, artifact, accepted peer and INIT checks remain required.
See the selected [socket job enqueue path](https://github.com/systemd/systemd/blob/v255/src/core/socket.c#L2176-L2207)
and [atomic inventory fields](https://github.com/systemd/systemd/blob/v255/src/core/dbus-manager.c#L829-L855).

The [installed lifecycle profile](../../tests/qualification/worker-lifecycle/README.md)
checks twelve original qualified Execution launches on each Text surface. It
pins the runtime binary and temporary Endpoint unit independently and verifies
joined cgroup and process cleanup before checking manager collection of each
original worker unit. Each launch has an exact receipt under the retained
Endpoint invocation. The profile uses fixture local authorization and supplies
component evidence for worker activation and cleanup.

The [Execution escape profile](../../tests/qualification/execution-escape/README.md)
pins a root-installed artifact that probes host TCP/UDP, DNS, file and Unix-IPC
access, namespace creation and UID escalation before its ordinary handshake.
Independent listeners check positive controls and detect contact on both local
surfaces. The installed property verifier requires effective `bpf` denial. A
process terminated by a forbidden syscall cannot record its own result, so the
profile relies on effective policy inspection for that control. Its P6/P7
result covers the pinned component and host environment.

The local context owns a reservation from the start of worker launch through
joined cleanup, even if cancellation precedes readiness. Endpoint shutdown
revokes every retained context before waiting for any one worker. Pending
cleanup still occupies the finite context budget after its Broker lease is
released. A cleanup failure closes text-job admission for this Endpoint
generation, including previously idle contexts and late completions; creating
another context cannot recover authority while an old cgroup may remain live.
The Job owner publishes one immutable cleanup result: the original cleanup
error survives context removal, repeated completion and repeated Endpoint
close, while successful joined cleanup alone releases the Context reservation.
Within one Context, shutdown likewise stops every extracted child owner and
every Context-owned flight before joining any of them. Those owners detach
their own state; the Context does not clear their implementation fields. The
Job joins last, and the Context remains in the Endpoint shutdown tree until
that final join, durable Publication retirement and the stored result complete.
Repeated Context Close returns that same joined result.
The worker lifetime owner pins cleanup before INIT, closes the exact
attachment on cancellation, joins initialization and cgroup cleanup, and
publishes one immutable completion. This ownership is not a qualified launch
receipt and does not supply installed stop permission or a worker Grant.

The local launch composition reserves the existing context's job, serializes
activation inventory, verifies the installed artifact and fixed socket, and
pins cleanup before verifying the artifact again and sending INIT. After
readiness it rechecks the exact invocation and artifact before creating a
fresh private worker Principal and Connection Grant. That Grant delegates only
the scoped byte exchange: Publisher administration remains with the separately
authorized context. The worker receives no capability, key or destination
selection input. An ambiguous activation terminalizes text admission rather
than allowing a replacement to inherit it.

The reader and Publisher consumers use this private Grant lease. Cleanup
interrupts their attachment and joins their Service-stream I/O before reporting
the job finished. Reader results pass both joined cleanup and an exact
last-job/current-context check; a result cannot return after a replacement has
started and finished. Retirement closes the worker Grant, while successful
worker cleanup leaves the separately authorized context available for a later
explicit job. This local composition still requires adoption by the protected
participant runtime and the complete installed-host acceptance matrix.

The maintained Broker keeps generic capability mechanics. A qualified-launch
receipt is an opaque local owner object, never a bool supplied by an arbitrary
Application. Profile 3 accepts only Principals created through the verified
text-worker launch for its normal job. The separately pinned network-qualification
inventory may admit only its exact fixed stream-test worker through the same
boundary, as specified by the qualification owner; no caller can enable it. Existing generic/unqualified callers remain compatible
with current C0 but cannot enter the adopted protected profile.

The new local Connection framing is version 2 with magic AAI4, retaining the
current bounded stream/EOF/terminal semantics. Its first request has a u8
destination tag (Name=1 reserved, Target Link=2), u16 length and 1 through 512 UTF-8
bytes. The complete request is AAI4[4], tag[1], length[2], destination[length].
The current capability is bound to the server-side Principal/attachment;
it is not a new wire field or an export to the worker. Apply the existing canonical
Target Link parser for tag 2. Tag 1 returns not-selected before any network
effect until the canonical naming stage is admitted; do not sniff or fall back.
Server acceptance is one byte (1 accepted, 0 refused). Data frames have a
big-endian u32 length and 1 through 16,384 bytes; zero length closes the sender's
input direction. The terminal marker is 0xffffffff, followed by u16 class
length, u16 reason length, UTF-8 class[1..128] and reason[0..512]. Reasons contain
no destination or private bytes. The current Stream close/Done/order semantics
and typed terminal classes remain unchanged. The old AAI2 request is not
accepted as a protected job. Conformance vectors cover accepted Link and refused Name tags,
all boundaries, terminal outcomes and incompatible old requests. The local AAI4
public-Interface oracle preserves each of the 256 byte values exactly once across
unequal request and response fragments, observes the response only after the
caller closes its input direction, and opens the owner exactly once. It exercises
Connection transport behavior only; Administration remains with the trusted
Endpoint owner and is neither inferred from bytes nor delegated to the caller.

## Worker exchange

AAI4 is the trusted local caller-to-Endpoint interface. The worker receives an
already authorized stream through a local worker attachment; it
cannot ask Endpoint to choose a destination, open a file or issue a Grant.

After installed-unit verification, Endpoint sends INIT: ASCII ARDTWP01[8],
mode u8 (reader=1, Publisher=2), fresh job-nonce[32], snapshot-length u32,
SHA-256(snapshot)[32], then snapshot bytes. Integers are big-endian. A reader
has zero snapshot length; a Publisher has at most 4 MiB. The worker returns
ARDTWR01[8], the exact nonce[32] and snapshot digest[32] only after bounded
validation. This is a readiness acknowledgement under the verified launch,
never evidence establishing the launch's own authority.

Subsequent frames are kind u8, stream-ID u32, length u32 and exact payload.
Endpoint allocates increasing odd IDs for already admitted Service streams;
the Publisher has at most 256, and stores one immutable snapshot for the
publication, not one copy/worker per incoming Connection. Reader mode permits
one Service stream (ID 1) and one result stream (ID 2). No ID is reused.
OPEN=1 has empty payload and is Endpoint-only; BYTES=2 has 1..16,384 bytes;
CREDIT=3 has one positive u32; EOF=4 has no payload; CLOSE=5 has one bounded
terminal class byte. Each direction has 64 KiB credit, and aggregate queued
worker bytes are at most 8 MiB. Grant credit only when the actual consumer has
released space. Unknown kinds, wrong directions, unsolicited streams or
credit overflow close the job and its attachment. A malformed document request is scoped to its admitted Service stream: emit one non-clean CLOSE and retain the Publisher snapshot and other streams. Stop replenishing that rejected stream; discard only already credited in-flight input until Endpoint closes it.

The verified Endpoint composition, not worker INIT or worker frames, assigns
the Service stream's checked directional byte contract. Every ordinary file
uses the same 4 MiB bound in each direction; the fixed qualification caller
keeps 64 MiB in each direction. The
reader/publisher mapping is one composition contract, and a replacement Route
Attachment continues its existing counters rather than acquiring fresh limits.

After complete Service response validation the reader emits RESULT=6 on ID 2:
status u8 and content-length u32, then bounded BYTES and EOF for that result.
Endpoint accepts at most 4 MiB and independently checks length, current
job identity and terminal outcome. Only the trusted presentation function may
render these bytes, with its own escaping; arbitrary worker stdout is never
sent to a terminal. This does not authenticate a compromised worker's account
of content: the worker may lie about its allowed job but cannot gain ambient
network or host authority through this interface. No second remote request is
created by result processing. Cancellation closes all streams and joins the
whole worker cgroup; a replacement worker gets a new job and nonce.

The trusted command's read client consumes AAI4 and the bounded file byte stream.
It waits for the complete UTF-8 response and clean terminal result, joins the
local stream, and independently escapes controls before presentation. The UI
reopens and identity-checks only its exact inherited input/output as
separate pollable descriptions, so cancellation interrupts blocked terminal or
pipe I/O without changing the invoking shell's shared descriptor flags. It
initializes UI signal handling only after dispatch excludes the fixed worker
entrypoints. This is a real local client; it does not supply a launch receipt,
grant, authenticated State, or a replacement for protected Endpoint composition.

When AAI4 refuses setup, the local client retains its bounded outcome class in
`connection.SetupRefusalError`. The trusted command maps local cancellation and
timeout to its existing stable diagnostics and presents every other setup
outcome as unavailable. It does not present the refusal reason, destination,
or Endpoint detail; cancellation or timeout takes precedence when cleanup
joins it with another setup failure.

The trusted UI imports the owner-selected file under the local owner's
permissions and submits only its bounded snapshot through the existing
owner-authorized Administration boundary. Endpoint's separate service account
does not gain arbitrary access to the owner's home. Other Applications and
reused worker UIDs cannot access its private sockets/state. Compromise of the
trusted desktop owner, Endpoint service account, root or kernel is outside the
surviving-boundary claim; testing a sibling worker does not prove otherwise.

## Planned ownership, not empty packages

| Owner | Responsibility / permitted imports when implemented |
|---|---|
| internal/application/endpointstream | Version-2 local Connection grammar, typed destination, bounded stream and conformance; standard library only |
| internal/application/document | Bounded document exchange, worker framing and attachment lifetime. |
| internal/application/document/localread | Read the Target Link, join the document exchange and render escaped UTF-8 on the caller’s terminal. |
| internal/domain/endpoint | Verified installed-unit/socket identity, local job lifecycle, opaque launch receipt, Broker composition and existing Network/Service orchestration; use existing owned imports and the two selected Application contracts |
| cmd/ardents-next | The read command opens terminal descriptors and transfers ownership to localread. Publication and Link retrieval use the Administration client. |
| internal/application/document/worker | The installed `ardents-text` artifact accepts only fixed worker entrypoints and audits the inherited descriptors. |
| Existing installation/Release owners | Verified worker/root/unit artifacts, separate service identities and controlled install/update/remove; no unverified execution |

These are selected future package additions under ADR-0081.
The factual package map changes only with real Implementation, behavior tests,
doc.go, exact imports and callers. There is no speculative launcher framework
or library for arbitrary child programs.

## Evidence and qualification boundary

The new Execution preparation component also ran both Connection and
Administration surfaces with the genuine fixed Text artifact in an isolated
Ubuntu24/systemd255 manager on WSL. The new runtime consumer activated and
initialized each worker, reobserved its original artifact/invocation, joined
its pinned cleanup, and returned only completed-current preparation provenance.
The post-run worker inventory was empty. Independent original proc/cgroup
descriptors established fixture-manager descendant termination and cgroup
removal after Stop returned 0; the manager retained a stop-timeout failure and
required SIGKILL, so clean shutdown is unclaimed. The source and evidence are recorded with
the [Execution owner](https://github.com/dianabuilds/ardents-network/issues/507).
This is self-reviewed component evidence, not an accepting installed participant,
live Service operation, hostile-tree/escape/network/recovery result or
whole-host/privacy qualification. The initial guest overlay-permission failure
remains separate failed environment evidence.

A later source-matched Execution lifecycle component run exercised joined
preparation and a live single-use operation on both local surfaces. Retirement
denied effects and interrupted the operation while original invocation close
waited for operation join; successful physical cleanup supplied exact last-Job
provenance without restoring byte permission. A control injected an error from
an already joined borrower while genuine reader and Publisher workers were
live: both local surfaces refused synchronously, each original worker joined,
late successful sibling completion refused, and repeated generation close
retained the first error. The component and Endpoint terminal result passed;
the fixture manager's pinned original processes/cgroup physically joined after
Stop returned 0 but again retained timeout/SIGKILL. This remains WSL component
self-review, with no accepting startup, Service bytes or qualification-matrix
claim.

The separate installed Execution Route component exercised the production
live holder inside the original non-root Endpoint MainPID through genuine new
signed State, permission/Stock, bootstrap and admitted issuing owners on TCP/TLS
and QUIC. Bootstrap retirement preceded fresh token spending; Route borrowers
and the original worker joined, and Hosting reopened without reservations.
Its first TCP trial expired during the fixture's pre-operation real-hour wait;
moving the consumer bound after that preparation let both Carriers pass without
changing the operation deadline. Both original fixture managers physically
joined their pinned processes/cgroups while retaining stop-timeout/SIGKILL.
Signing and infrastructure principals are test fixtures. This proves neither
installed accepting startup, private Service operations, worker-loss behavior
during Route I/O nor the complete qualification/regression matrix.

The later attachment observer was exercised with genuine installed workers on
both local surfaces, including a native race-instrumented lifecycle binary.
Unexpected attachment loss interrupted the exact operation, withheld completion
until operation join and retained a failed invocation result; clean physical
cleanup allowed fresh preparation without making that result current. The
race fixture's cleanup helper refused after its original manager had already
hit the outer runtime limit; terminal timeout/SIGKILL was observed, but no
pinned join receipt was obtained for that fixture.
The production Route holder then passed both Carriers on the changed runtime.
A further causal scenario independently stopped its sole original reader while
an already spent Prefix remained open: the holder interrupted and joined
without console EOF or caller cancellation, performed no later issuance and
reopened Hosting without reservations. Both positive and loss subcases passed.
Their fixture managers joined pinned original processes/cgroups while retaining
timeout/SIGKILL. These are self-reviewed WSL components; loss during active
remote I/O, durable spend replay/reopen, accepting Installation startup and the
full qualification matrices remain distinct evidence obligations.

A separate active-I/O control now delays the genuine ordinary signed RESULT
after actual admission/spend and issuer signing, then stops the independently
observed original reader. Both Carriers refuse completed output, join the holder
and receiving owners, return Hosting reservations and reopen the holder budget.
The positive, open-Prefix loss and active-I/O loss cases passed in canonical and
native race-instrumented consumers. The input/output fixture actor also joins.
This demonstrates worker-loss behavior at that RESULT boundary; it establishes
neither independent durable spent-token replay after reopen nor State/caller
loss at that boundary, accepting installed startup or complete qualification.

The original cleanup mechanism now reports its first definite physical failure
to the bound launch Job before continuing descendant observation. A controlled
failed-Stop test requires that report before the next pinned read and rejects a
second report; generation rules separately establish synchronous sibling denial.
Unknown manager observation can still resolve to verified original cgroup removal
without falsely reporting cleanup failure. Linux Execution race tests and fresh
genuine installed lifecycle/Route positive and loss profiles passed on the
resulting source. The controlled failed-Stop schedule is module evidence, not
an injected native manager failure or complete installed qualification.

The installed Route component subsequently reopened every genuine receiving
spend owner after physical borrower join. All accepted tokens still passed
signature/current-authority verification and refused specifically as already
spent, with their new real Hosting reservations returned exactly once. The
joined, open-Prefix loss and signed-RESULT loss cases passed on both Carriers;
worker loss did not refund those retained histories. The fresh isolated manager
physically joined its original pinned processes/cgroup with retained outer
timeout/SIGKILL. This is executor self-review, not accepting startup, State/caller
loss during RESULT, private Service work or complete qualification evidence.
The initial native race trial retained an actual QUIC CREDIT physical-write
failure after deliberate worker loss. Requiring clean receiving shutdown in
that negative case contradicted the retained physical-error contract. The
corrected oracle requires Done and an identical repeated Close result; only
exact native peer-retirement causes may be retained there, with every other
failure still refusing the case. Three source-matched native race repetitions
passed, two observing and retaining that same failed receiving retirement.
No production error is suppressed, and positive retirement remains clean.

The [Execution tree profile](../../tests/qualification/execution-tree/README.md)
uses a separately pinned adversarial worker and fixture local authorization.
Original cgroup and proc descriptors observe the exact hostile lineage,
worker identity and hardening. The independent Publisher keeps its invocation
through victim cleanup, then returns its immutable snapshot over the actual
Text worker protocol with a local stream fixture. Attachment EOF must retire the
fresh original tree while the caller still holds the operation join. The
runtime then retains the original failure, and the manager collects the
retired units. This profile covers local lifetime and snapshot progress.

The new Execution escape component separately root-pins the adversarial Text
artifact and runs both local surfaces through actual qualified launch and one
exact operation. Independent host TCP/UDP/DNS/Unix listeners observe positive
controls before launch and after original physical join; any other contact
fails. A world-readable host-only file prevents UID permissions from supplying
its refusal oracle. Original kernel event and proc-directory descriptors prove
worker cleanup. A wrong independent artifact digest refuses before Endpoint
activation. The component passed on isolated Ubuntu24/systemd255 under WSL;
the outer manager physically joined its original scope/processes while retaining
timeout/SIGKILL. Its temporary root was removed after that join. This is executor
self-review, with fixture local authority and no Service, logical recovery,
independent security or whole-host qualification. See the
[Execution escape profile](../../tests/qualification/execution-escape/README.md).

The disposable systemd probe ran in Ubuntu 24.04.4 / systemd
255.4-1ubuntu8.14 on WSL's 6.6.87.2 kernel. Fourteen parent/child attempts had
working unconfined positive controls and were denied by the confined profile.
The permitted stdin/stdout byte marker survived. The worker had no effective
capabilities, NoNewPrivs=1 and seccomp filtering enabled.

This is mechanism evidence. The implemented socket activation, exact unit
inventory, descriptor audit, hostile Application Interface, snapshot importer,
revocation and complete installed-artifact journey still need the
[qualification tests](../development/privacy-qualification.md). In particular,
a root owner or kernel that defeats these restrictions is not contained.
Do not claim protection from privileged host or microarchitectural compromise.
