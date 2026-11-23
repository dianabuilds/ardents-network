# Protected Service workload

The nearest closed release uses only TCP/TLS under
[ADR-0122](../adr/0122-select-tcp-only-closed-release.md). This supersedes the
dual-Carrier qualification clauses below for that release. QUIC-selected work
must refuse before effects, never fall back. Every Application, authority,
confinement, resource, recovery and joined-cleanup obligation remains required
on TCP/TLS; historical dual-Carrier receipts do not qualify changed artifacts.

Status: **selected design scope**, from the Product Owner's two explicit
answers on 2026-09-07. This describes the successor to implement and test;
it does not qualify existing C0 or authorize a public deployment.
The [common architecture](../technical/common-privacy-architecture.md) remains
the single protection construction.

## Outcome and supported scope

One owner publishes an immutable UTF-8 document through a Service. A User
opens its explicit Target Link, receives the
document, and reads its safely rendered text. The User chooses a Service,
not peers, a route, a proxy, padding or a security level.

The first delivery is a closed test network with explicitly provisioned Nodes,
State authority, time witness, issuers and finite resource permissions.
These are actual verification inputs under the closed authority contract,
not evidence of autonomous public admission or independent operators.
Public autonomy remains R-149; no public proof verifier is simulated.
The Product Owner selected Target Link first on 2026-09-07. Canonical Service
Names are a separate next stage with their real authority/close producer under
[ADR-0054](../adr/0054-separate-alpha-transition-contracts.md).
An alpha alias or provisioned lookup table is not canonical naming.

The selected platform family is Ubuntu LTS x86-64 on both endpoints.
The admitted Endpoint/worker engineering profiles are Ubuntu 22.04 LTS
(systemd 249) and Ubuntu 24.04 LTS (systemd 255), both on cgroup v2;
security-updated package revisions enter each candidate's inventory. The
platform contract is a capability floor (Ubuntu 22.04+, fail-closed) rather
than a frozen version whitelist, accepted under
[ADR-0126](../adr/0126-admit-platform-by-capability-floor.md). Infrastructure
(Source/Forwarding) runtime code still pins Ubuntu 24.04/systemd 255 until the
gated #643 migration adopts the same floor there.
A WSL run can test mechanisms but cannot qualify the complete Ubuntu host.

The protected job includes both the reader and Publisher Application process
trees. Endpoint, Node, custody, installation and update keep separate owners.
A Service receives its intended request and may return malicious content.
The operating system, installed launcher and relevant Endpoint verification
must survive for process confinement to hold.

## Participant interface for arbitrary data

[ADR-0132](../adr/0132-admit-protected-bidirectional-application-streams.md)
selects this interface. Installed qualification is pending.
The [command reference](../reference/commands.md) lists current commands.

| Person | Installs | Selected action |
|---|---|---|
| Sender or receiver using an Application | The protected Endpoint distribution and a locally authorized Application delivered for its exact confinement profile | The Application opens the recipient's Service through the local Application Interface. The sender and the receiving Service exchange ordered bytes using their Application's protocol. |
| Participant using the command line | The protected Endpoint distribution with a confined byte-stream Application | `ardents stream connect APPLICATION_SOCKET --link-file PRIVATE_LINK_FILE` reads bytes from standard input and writes response bytes to standard output. The participant supplies the Link in a private local input, keeping it out of process arguments and reserving standard input for data. |
| Service operator exposing a byte-stream Application | The protected Endpoint distribution and the signed, confined Service worker for the chosen Application | Run `ardents stream publish ADMINISTRATION_SOCKET` to start the locally authorized fixed worker and attach the operator's standard input and output for one exchange. `ardents link ADMINISTRATION_SOCKET` and explicit withdrawal use the same Service lifecycle as the document Application. |

The stream command keeps diagnostics on standard error and reports a
complete exchange only after authenticated terminal completion. Standard output
may already contain partial bytes on interruption. EOF on standard input closes
the local send direction while receiving may continue. Retrying interrupted
Application effects requires explicit Application or participant action.
Applications own framing, format and presentation for messages, images, audio
and other data. The Endpoint owns the destination, authentication, admission
and Connection lifetime. A worker receives only its scoped attachment. Selected
Application and worker identities must bind to signed local policy before any
effect. Remote input cannot select a command, executable or local file path.

Reuse the existing authenticated Service Connection and its ordered stream,
flow control, continuity and terminal result. Reuse the local Application
Connection byte framing where its authority and lifetime match the new profile.
The [local binary conformance test](../../internal/application/connection/transport_binary_conformance_test.go)
exercises a byte exchange through local framing with a test stream. The
[scheduled qualification workload](../technical/application-confinement.md#selected-boundary)
exercises fixed binary canaries with qualification-only state. Neither supplies
a production participant job. Production admission needs an exact signed
worker profile and a real Service handler; it must refuse an unsupported
profile before delivering Application bytes. The existing document request
grammar remains scoped to document Services. The current v3 Target Link carries
only Network and Target. It can remain the destination input, but cannot prove
which Application the authenticated Instance offers. The client must request
the byte-stream profile explicitly. The Instance must authenticate its
acceptance in the logical Connection setup before either side admits a data
worker or forwards user payload. The acceptance must bind the exact Target,
Instance, authenticated Connection and chosen profile. Continuity must retain
that original binding. Use the existing transport and document grammar for
document peers. An older peer, unknown profile, missing proof or profile
mismatch must terminate the requested byte-stream operation without fallback.

Use the existing encrypted Instance proof and Continuity setup to
authenticate the profile. Both peers derive the expected Application binding
from their own admitted local profile. The initiator derives it from the
explicit operation, and the Publisher derives it from the verified Service
configuration. Commit the byte-stream selection to the logical context that
the Instance proves and Continuity retains. Preserve the exact existing
document context transcript. A mismatch therefore fails the existing context
verification without a new cleartext TLS negotiation field or a trial
Application request. The two local owners must check this binding before they
hand an authenticated stream to a data worker.

The first byte-stream profile admits up to 4 MiB in each direction per
Connection. Admission uses the original 60-second setup and exchange budget
and one active exchange per stream job. Shorter authority and resource
deadlines still apply and recovery cannot renew them. Existing peers retain
the document profile and refuse a byte-stream binding. Keep the document workload and its signed identities
unchanged until their separately designed migration is adopted. Qualify the new
workload through the same protected Route with two installed Endpoints. Exercise
non-UTF-8 bytes, directional EOF, backpressure, truncation, interrupted transfer,
revocation and joined cleanup with both positive and causal refusal cases.
Confidentiality of Application bytes and traffic-correlation exposure require
separate evidence under the [threat model](../security/threat-model.md).

## Interaction

1. The Publisher selects one regular local UTF-8 file, at most 4 MiB. A trusted
   local importer opens it without following symlinks and produces an immutable
   owner-authorized snapshot. Reject special files, changing input, invalid
   UTF-8 and oversize input before publication. No arbitrary path is accepted
   from a network request.
2. The isolated worker receives only that snapshot and its scoped local
   Application attachment. The Endpoint owns Instance authentication,
   Introduction readiness and publication. Readiness is shown only after
   those operations succeed.
3. The reader enters a Target Link. Endpoint admits local
   authority, verifies the exact destination, and makes one ordinary protected
   Connection before the reader sends the document request.
4. The Service returns a bounded response and closes its send direction.
   The reader accepts the complete length and terminal result before rendering.
   Render plain text: escape terminal control characters, including ESC,
   C0/C1 controls other than line feed and tab, and bidi control characters.
   Links are text, never automatically fetched or launched.
5. A later explicit read uses a fresh Connection. Valid destination caches may
   remain only within the same local Isolation Context and validity window.
   The reader has no persistent history, cookies, login, telemetry or private
   shared cache in this first job.
6. Withdrawal stops new admissions immediately. A previously authorized read
   may finish within the earlier of its existing deadline and a 5-second drain.
   Explicit revoke or local safety failure cancels immediately.

A document revision is a new explicit publication operation; there is no live
editing or implicit file watcher. Download/save/export, arbitrary HTML/scripts,
external subresources, redirects, authentication accounts and a general browser
are outside this first supported job. The Network's ordered-stream contract
and its large-stream tests remain broader than this document viewer.

## Application exchange

Use one request per Connection. Its wire bytes are a fixed 512-byte record:
ASCII `ARDTXT01` (8 bytes), request operation `1` (one byte), then 503 zero
bytes. Reject any other length, operation, nonzero padding or second request.

The response is `ARDTXT01`, one status byte, one unsigned big-endian u32 body
length, then that many bytes and authenticated directional EOF. Status `0`
means UTF-8 content (0 through 4 MiB); `1` means unavailable and has length
zero. No HTTP, compression, redirect, MIME sniffing, script or remote filename.
Authentication and exact Target are provided by Service Connection, not by
this simple application grammar. A 64 KiB response is the NET-32 reference;
larger documents have the same protection but a size-dependent completion time.

Never render an unbounded stream or partial unauthenticated outcome as a
completed document. A lost Connection can resume the same ordered transport
only under its bounded continuity contract; it cannot cause a second
Application request. If that continuity fails, show an interrupted read.
A new read requires an explicit User action.

## Visible failures and local lifecycle

The supported outcomes are invalid destination/input, authority/currentness
failure, unavailable Service, incompatible generation, insufficient local or
network capacity, timeout, cancelled, interrupted and successful completion.
The ordinary UI gives these useful classes; it does not display peer identities,
issuer tokens, hop counts or speculative detection of traffic correlation.

Before isolation is effective, the worker receives neither a Grant nor a
network-capable descriptor. Isolation setup failure produces an unavailable
job. There is no proxy-only accepting fallback. A crashed worker is killed
with its complete process group/cgroup and loses its attachment and volatile
context. A worker restart is a new local job, not an implicit new remote read.

Endpoint and Instance restart retain current authority and publication floors.
A published Instance whose private key was intentionally erased cannot be
resurrected. Republish requires the existing authorized successor Credential
and its non-overlap conditions; the operator sees the earliest permissible
time. This first scope does not promise automatic Publisher restart recovery.
Designing that separate authority change is not hidden inside privacy work.

## Acceptance and limits

The full 512-byte / 64 KiB exchange has p95 at most 3 seconds on a cold
destination and 1 second on a warm new Connection in the declared test envelope.
A ready Endpoint is the start condition; full bootstrap is reported separately.
The [qualification contract](../development/privacy-qualification.md) defines
the network conditions, overhead and readiness measurements.

There is no autonomous filler stream. The first scheme retains the stated
traffic-correlation risk. It must still enforce payload/Target authentication,
per-role field separation, context separation, ordinary-network escape denial,
finite resource use and explicit failures. Viewing a Service and sending its
intended request are not anonymous from that Service's application contents
by definition.

Complete endpoint/kernel or launcher compromise, all-path withholding and
broad timing/volume correlation remain stated limitations. An internal test
network and Product Owner walkthrough do not establish independent security,
market adoption, public anonymity or public availability.
