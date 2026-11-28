# Protected Service workload

The nearest closed release uses only TCP/TLS under
[ADR-0122](../adr/0122-select-tcp-only-closed-release.md). A QUIC selection
refuses before effects without a transport fallback. Every Application, authority,
confinement, resource, recovery and joined-cleanup obligation remains required
on TCP/TLS; historical dual-Carrier receipts do not qualify changed artifacts.

Status: **selected closed workload** under ADR-0134.
The [common architecture](../technical/common-privacy-architecture.md) provides
one protection construction for all Application bytes. The
[C0 scope](scope.md#c0-closed-alpha-readiness-profile) owns the qualification
boundary and release claim limits.

## Outcome and supported scope

A participant uses an explicit Target Link to reach a Service. Every file,
including text, uses the same bidirectional byte stream. Endpoint selects the
protected Route. File publication serves an immutable snapshot.

[ADR-0135](../adr/0135-continue-byte-transfers-across-admitted-connections.md)
removes the fixed total-size ceiling from the product contract. One user
transfer may span independently admitted Service Connections and reports one
authenticated result. The current implementation still applies the 4 MiB
bounds described below. Publication and transfer retain storage reservations,
bounded buffers, backpressure and authority checks.

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
selects this interface. The
[installed-pair run](https://github.com/dianabuilds/ardents-network/actions/runs/45275563719)
passed the document journey and the byte-exact public CLI exchange on two
separately installed Ubuntu Endpoints. The run exercised source
`b271ee71d8d1e3ebb6081dabe84ee3121c5eabd6` with its own artifacts and closed
fixture authorities.
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

The sole Application stream uses the existing authenticated Service Connection,
ordered bytes, flow control, continuity and terminal result. Local AAI4 framing
and the authenticated byte-stream context apply to every ordinary connection.
Content type does not select a protocol or Service profile.

Each direction admits up to 4 MiB per Connection within the existing bounded
operation lifetime. Authority loss and earlier resource deadlines still stop
the exchange. Qualification must exercise arbitrary binary bytes, directional
EOF, backpressure, interruption, revocation and joined cleanup.

## Interaction

1. The Publisher selects one regular local file, at most 4 MiB. A trusted
   local importer opens it without following symlinks and produces an immutable
   owner-authorized snapshot. Reject special files, changing input and oversize input before publication. No arbitrary path is accepted
   from a network request.
2. The isolated worker receives only that snapshot and its scoped local
   Application attachment. The Endpoint owns Instance authentication,
   Introduction readiness and publication. Readiness is shown only after
   those operations succeed.
3. The reader enters a Target Link. Endpoint admits local
   authority, verifies the exact destination, and makes one ordinary protected
   Connection before transferring bytes.
4. The Service returns a bounded response and closes its send direction.
   The reader waits for EOF and authenticated terminal completion before presenting a completed result.
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
editing or implicit file watcher. Automatic execution of HTML/scripts,
external subresources, redirects, authentication accounts and a general browser
are outside this first supported job. The Network's ordered-stream contract
and its large-stream tests remain broader than this document viewer.

## Application exchange

The receiver closes its sending direction. The Publisher writes the selected
snapshot bytes directly and closes its sending direction. Empty files are valid.
The receiver enforces the byte bound and accepts success only after directional
EOF, authenticated terminal success and joined cleanup. The transfer has no
content-specific request, header, status record or encoding validation.

A local text viewer checks UTF-8 and escapes terminal controls before output.
Saving arbitrary bytes uses the ordinary stream commands and pipes.


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

The full 64 KiB file transfer has p95 at most 3 seconds on a cold
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
