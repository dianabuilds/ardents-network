# Text-Service Application confinement

Status: **selected closed Ubuntu design**, for the Product Owner's
[text-document workload](../product/protected-service-workload.md).
R-099's job/platform prerequisite is now resolved for this scope.
No Windows, generic browser or arbitrary-Application profile is admitted.

## Selected boundary

Use the distribution-maintained systemd system manager on Ubuntu 24.04 LTS,
with systemd 255, cgroup v2 and kernel namespace/seccomp support. The Endpoint
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
dependency on it. The launch boundary verifies the root-owned Ubuntu 24.04 identity, amd64
architecture and actual system-manager version 255, then verifies that the caller is that active
service's MainPID under the dedicated Endpoint user/group, with
`RemainAfterExit=no`, `ExitType=main` and `RestartMode=normal`. Unknown values
or semantics that keep the parent active after its main process exits refuse
before worker activation. This manager-owned lifetime dependency covers an
Endpoint exit that cannot run its Go cleanup; socket EOF is insufficient.

The trusted Endpoint connects the socket and verifies the installed unit,
executable and sandbox-root identity against the selected local artifact.
The system manager applies confinement before ExecStart. Only then can the
fixed worker return its local HELLO. A per-job random nonce and the exact
accepted socket instance bind the Broker's Application Principal and Local
Grant. Worker HELLO alone is never proof of isolation on an unverified unit.

## Required installed properties

The unit must include these effective properties; unknown or unavailable
required settings make the platform unavailable before Grant delivery:

- RootDirectory points to the immutable root owned by root. It contains only
  the verified static worker executable and empty required runtime mount points.
  WorkingDirectory=/ is explicit.
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
the accepted local stream plus harmless null descriptors. AF_UNIX alone
does not authorize access to an unrestricted host IPC peer; the private root,
private network namespace and socket permissions enforce that exclusion.
The protocol never receives SCM_RIGHTS and grants no file-descriptor exchange.

## Data and authority flow

The trusted UI/Endpoint boundary accepts only a selected destination or an
owner-selected document snapshot. It performs syntactic checks and local
authorization before destination-dependent network work. The new versioned
Connection Interface admits Target Link explicitly and reserves a refused Name
tag for the later canonical composition; it never accepts
a path, executable, peer list, private key or administration operation.

The Publisher's owner chooses the input file. Import once using a no-follow
regular-file check, a 4 MiB read limit, UTF-8 validation and a stable-read
check; copy to a bounded immutable in-memory snapshot. Transmit that snapshot
over the scoped local Publisher attachment. The worker sees neither the
original host path nor the host file tree. Publication authority and Instance
keys stay in Endpoint/Custody; receipt of a snapshot is not permission to sign
or publish a Service.

The Linux trusted importer walks the selected absolute path through no-follow
parent directory descriptors and opens the final file nonblocking before the
regular-file check. It rejects oversized or invalid UTF-8 input and compares a
second bounded read with the first, as well as file identity, size, mode,
owner/link count and modification/change timestamps. Timestamp equality alone
is insufficient on a filesystem with coarse change times. The returned bytes
are a private volatile copy; subsequent host-file edits do not update a
publication. This observed stable read is not an atomic filesystem snapshot
against a compromised local owner that controls concurrent writes.

The trusted UI may explicitly request its committed publication's Target Link
with `ardents-text link <administration-socket>`. This read-only operation sends
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
exact UTF-8 bytes (0 through 4 MiB), then directional EOF. There is no path,
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

The selected [installed lifecycle profile](../../tests/qualification/text-worker-lifecycle/README.md)
checks both worker roles through this actual launch and cleanup owner. It pins
the binary and temporary Endpoint unit independently, captures the exact
systemd invocation and requires both executed role tests to pass after the
Endpoint process terminates. A zero exit status without the required tests is
refused. Local owner authorization is an explicit fixture; the profile neither
substitutes for protected command adoption nor completes the hostile-worker
or end-to-end qualification matrix.

The separately pinned [hostile-tree profile](../../tests/qualification/text-worker-tree/README.md)
uses an adversarial test artifact under those same installed restrictions. It
requires a live child and grandchild that ignore SIGTERM and retain the local
attachment, then proves original cgroup cleanup and continued snapshot service
from a distinct Publisher sibling. Artifact selection is a root qualification
installation, never a caller option. This is distinct from the ordinary worker
artifact and from complete abrupt-crash, escape and P6/P7 qualification.
It also observes parent exit on attachment EOF and manager-owned descendant
cleanup before invoking Endpoint Close for the remaining local job teardown.

The separately pinned [escape-matrix profile](../../tests/qualification/text-worker-escape/README.md)
uses a different root-installed worker artifact to attempt host TCP/UDP, DNS,
file and Unix-IPC access, namespace creation and uid escalation before the
ordinary worker handshake. The Endpoint driver owns the host sentinels and
listeners, so both a successful handshake and no observed contact are required
for each role. Effective syscall-filter inspection retains the `bpf` denial:
the denied syscall itself may terminate a worker and therefore cannot be a
reliable in-process pass receipt. This remains bounded installed P6/P7 evidence,
not a claim of whole-host or general Application confinement.

The local context owns a reservation from the start of worker launch through
joined cleanup, even if cancellation precedes readiness. Endpoint shutdown
revokes every retained context before waiting for any one worker. Pending
cleanup still occupies the finite context budget after its Broker lease is
released. A cleanup failure closes text-job admission for this Endpoint
generation, including previously idle contexts and late completions; creating
another context cannot recover authority while an old cgroup may remain live.
The original cleanup error survives context removal and repeated Endpoint
close. The worker lifetime owner pins cleanup before INIT, closes the exact
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

The new local Connection framing is version 2 with magic AAI3, retaining the
current bounded stream/EOF/terminal semantics. Its first request has a u8
destination tag (Name=1 reserved, Target Link=2), u16 length and 1 through 512 UTF-8
bytes. The complete request is AAI3[4], tag[1], length[2], destination[length].
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
all boundaries, terminal outcomes and incompatible old requests.

## Worker exchange

AAI3 is the trusted local caller-to-Endpoint interface. The worker receives an
already authorized stream through a separate, fixed text-worker exchange; it
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

After complete Service response validation the reader emits RESULT=6 on ID 2:
status u8 and content-length u32, then bounded BYTES and EOF for that result.
Endpoint accepts at most 4 MiB and independently checks length, UTF-8, current
job identity and terminal outcome. Only the trusted presentation function may
render these bytes, with its own escaping; arbitrary worker stdout is never
sent to a terminal. This does not authenticate a compromised worker's account
of content: the worker may lie about its allowed job but cannot gain ambient
network or host authority through this interface. No second remote request is
created by result processing. Cancellation closes all streams and joins the
whole worker cgroup; a replacement worker gets a new job and nonce.

The trusted command's read client consumes AAI3 and the fixed text exchange.
It waits for the complete UTF-8 response and clean terminal result, joins the
local stream, and independently escapes controls before presentation. The UI
reopens and identity-checks only its exact inherited input/output as
separate pollable descriptions, so cancellation interrupts blocked terminal or
pipe I/O without changing the invoking shell's shared descriptor flags. It
initializes UI signal handling only after dispatch excludes the fixed worker
entrypoints. This is a real local client; it does not supply a launch receipt,
grant, authenticated State, or a replacement for protected Endpoint composition.

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
| internal/application/interfacev2/connection | Version-2 local Connection grammar, typed destination, bounded stream and conformance; standard library only |
| internal/application/textdocument | Fixed text request/response, bounded snapshots and safe presentation; the version-2 Connection contract and standard library |
| internal/endpoint | Verified installed-unit/socket identity, local job lifecycle, opaque launch receipt, Broker composition and existing Network/Service orchestration; use existing owned imports and the two selected Application contracts |
| cmd/ardents-text | Thin trusted text UI and fixed worker entrypoints calling textdocument; standard library and the selected text/Connection owners |
| Existing installation/Release owners | Verified worker/root/unit artifacts, separate service identities and controlled install/update/remove; no unverified execution |

These are selected future package additions under ADR-0081.
The factual package map changes only with real Implementation, behavior tests,
doc.go, exact imports and callers. There is no speculative launcher framework
or library for arbitrary child programs.

## Evidence and qualification boundary

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
