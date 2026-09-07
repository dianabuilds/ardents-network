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

Use two installed socket/service template families: reader and Publisher.
Their Unix sockets are in a root-owned directory, accessible only to the
dedicated Endpoint account. Accept=yes creates one bounded worker instance
for one local job: one reader operation or one publication lifetime. The accepted AF_UNIX descriptor is the only
Ardents attachment inherited by that worker; StandardInput=socket and
StandardOutput=socket connect the finite framed local exchange. StandardError
is null. No logging, session bus, credentials, environment strings containing
destinations, or arbitrary file-descriptor passing is available to the worker.

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
credit overflow close the job and its attachment.

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
