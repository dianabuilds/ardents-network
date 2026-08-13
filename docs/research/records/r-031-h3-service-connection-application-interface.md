---
id: R-031
title: Can Stage 3 expose one honest Service Connection to external Applications?
status: decided
owner: Product Owner
started: 2026-08-13
reviewed: 2026-08-13
---

# R-031 — H3 Service Connection and Application Interface

## Decision this unlocks

Authorize and freeze the smallest Horizon 3 Stage 3 vertical tracer. An external
client Application and an external publisher Application use OS-local IPC to
exchange arbitrary opaque bytes over one exact-Target-authenticated Service
Connection backed by the real Stage 2 Route. A routine migration replaces the
active Instance Key and Credential generation without changing the uncompromised
Service Target or placing Service Authority in either runtime.

This record is implemented by the
[Stage 3 brief](../../development/horizon-3-stage-3-brief.md). It selects no
production IPC, credential encoding, publication store, wire protocol, carrier,
transport, Application runtime, or deployment foundation.

## Current contract

R-002 fixes external local stream integration, separate Connection, Service
Administration, and Authority Custody privileges, honest Connection Results,
per-principal Local Grants, hierarchical budgets, backpressure, and no semantic
retry. R-006 fixes one exclusive active Instance, a host-generated Instance Key,
a public bounded Credential, stable Target routine migration, and Target
replacement after Authority compromise. R-024 closes launcher-brokered
Application Principal and Local Grant restart limits. The H3 technical design
places Service Connection above Route and makes publication ready only after
fresh Introduction acknowledgement.

R-030 and the Stage 2 brief define the lower seam: four distinct endpoint-chosen
positions, current authenticated Candidate material, literal-address TLS 1.3
laboratory carrier legs, role-local knowledge, an end-to-end protected channel,
no direct/short/DNS/proxy fallback, finite deadlines, and independent evidence.

The Product Owner accepts the clean committed Stage 2 local development result
at commit `5fd806ab3b2fe6e658aaba12c1610b5087cac470` as sufficient to begin this
bounded tracer: `95/95` attempts passed in `10m13.5548419s`, image
`sha256:45c350f921fe70df2e390ff93d3a2ab941423f73fbc30f3040e095445fc9e96e`,
and retained-evidence digest
`bcfd00c4e44c501dcc31be103699c4e4474eb8773e243ec68822ac00a036dfb1`.
This is development readiness only. It is not an official Stage 1 or Stage 2
qualification result. Official Ubuntu `short`, current `churn-2h`, and
independent `unattended-24h` gates remain conjunctive prerequisites for the
integrated H3 verdict or any stronger external, security, privacy, or release
claim.

## Hypotheses

- **H1:** a deep Service Connection/Application Interface Module can admit
  launcher-brokered principals, validate publication and exclusive generation,
  authenticate the exact Target through the Stage 2 Route, and carry bounded
  opaque streams without exposing route or authority mechanics to Applications.
- **H2:** the same outcome needs a different replaceable local IPC, credential,
  publication, framing, or carrier Adapter while preserving the frozen product
  interfaces and evidence.
- **H0:** the tracer requires online Service Authority, a direct/short/ambient
  data path, copied runtime keys, hidden reconnect or Application retry, Route
  disclosure, custom cryptography, unbounded buffering, or a production
  foundation choice.

## Evaluation criteria

### Observable outcome

One exact Service Target is created by an external Authority fixture. Generation
1 and generation 2 use different host-generated Instance Keys and public bounded
Credentials; generation 2 is higher and exclusive. Each generation accepts one
new Service Connection only after current Target/Instance authentication and
fresh publication acknowledgement. Each connection carries a fresh unpredictable
`32-byte` canary and seeded `64 KiB` incompressible stream in both directions.
The independent verifier recomputes exact length, order, and SHA-256 digest.

Migration stops and unpublishes generation 1 before generation 2 publishes. The
Target remains identical. Generation 1 cannot accept new work after supersession.
The new runtime receives neither Service Authority nor the old Instance Key. A
temporary explicit unavailable interval is permitted. Same-connection migration
or recovery is Stage 4 and is not part of success.

### Process and trust topology

The controlled Docker topology contains separate processes/containers for:

1. client Application Principal;
2. client Endpoint/Application Interface broker;
3. publisher Application Principal;
4. publisher Endpoint with distinct Connection and Service Administration IPC;
5. a separately granted publication operator Application Principal;
6. an ungranted hostile sibling Application Principal;
7. an external/offline Authority fixture which alone signs Credentials;
8. Initiator, Introduction, Rendezvous, and Responder Route actors;
9. an independent verifier which observes management evidence but never proxies
   Application Data.

Docker process, mount, user, PID namespace, and network isolation act as the
launcher-brokered/OS-isolated Application Principal boundary for this development
tracer. A scoped Unix-domain-socket directory is mounted only into its authorized
Application and Endpoint. An ephemeral one-use session capability is additionally
bound to one surface, principal, parent Grant, broker start identity, and finite
deadline. The capability alone is insufficient without the OS-isolated channel.
This proves neither Windows IPC nor hostile same-desktop-user isolation.

### Information and privilege rules

- The client Application receives one Connection socket, one ephemeral session
  capability, and the exact Target. It receives no Credential, publication,
  Instance Key, Authority, Candidate, Route Plan, Node identity, or retry state.
- The publisher Application receives one accepted Connection socket and opaque
  bytes. It receives no administration or custody material.
- The publication operator receives only its Service Administration socket, one
  service-scoped session capability, generation-specific Instance Key handle,
  and public Credential. It cannot export keys or call Connection/Custody work.
- The publisher Endpoint temporarily owns only the active Instance Key and public
  Credential needed for publication and Target authentication. It never owns
  Service Authority.
- The Authority fixture owns Service Authority, issues public Credentials for
  supplied Instance public keys, and exits before online transfer. It exposes no
  socket to ordinary Applications or Endpoint runtimes.
- Route actors retain the R-030 role-local view. No Application receives their
  addresses or identities. The verifier has observation authority only.

### Deep Module seams

The external seam is one local admission and Service Connection interface:
admit a principal against one Local Grant, publish with matching bounded
Credential/Instance possession, connect or accept one exact Target, exchange a
bounded reliable ordered stream, close/cancel, and return a classified result.
Its implementation hides IPC framing, session capability lifecycle, credential
encoding, current publication state, target proof, Route plan, carrier framing,
backpressure, cleanup, and evidence projection.

The existing Route Module is deepened only enough to carry a caller-provided
bounded bidirectional stream through the same four positions. It still owns
Route selection and protected carrier work, exposes no topology to Applications,
and has no dependency on Service fixtures, Compose, qualification, or local
grants. Candidate and qualification code do not enter the Application Interface.

### Resource and failure contract

The development profile is deliberately small and is not a V1 capacity claim:

- one Endpoint pair, one Target, one active generation, one controlled migration,
  three Application Principals, three persistent Local Grant policies, at most
  six live ephemeral sessions, and one Service Connection at a time;
- exactly two successful connections total, one per generation; one dial per
  Route leg, no retry, reconnect, alternate destination, or fallback;
- `4 KiB` maximum local control frame, `16 KiB` data chunk, `64 KiB` logical
  per-direction stream, `64 KiB` maximum pending data per direction, and no
  persistent Application Data queue;
- Endpoint → Local Grant/Application → Service/Isolation Context → connection/
  operation ownership; child creation never increases an ancestor allowance;
- at most `8` accepted local IPC connections, `2` Service Connections, `24`
  Service/IPC goroutines, `48` owned file descriptors, `16` timers, and `8`
  temporary socket/control files per Endpoint process;
- `5 s` local admission/publication/setup step, `15 s` total connection
  operation, `2 s` backpressure observation, and `5 s` close/shutdown deadline;
- a slow consumer blocks local writes before the pending-data cap grows; accepted
  bytes are never silently dropped; a local write count proves only local
  acceptance, not remote read or processing;
- after partial write, timeout, cancellation, or loss, remote Application
  completion is unknown; clean close is not an Application receipt;
- every terminal path releases sessions, sockets, Route actors, processes,
  goroutines, queues, keys, publications, timers, and temporary files.

Connection Result is limited to R-002 classes. The tracer may report invalid
destination, local authorization/policy/resource denial, evidenced Service or
Route unavailability, Target authentication failure, timeout/cancellation,
established, clean close, abrupt loss, or indeterminate failure. It does not
fabricate Node, Route, publication-store, carrier, or remote-Application causes.

### Security and privacy statement

Protected information is Application Data, Service Authority and Instance Keys,
the exact Target-to-endpoint binding from ordinary Route actors, and each
Application's local authority. The adversaries exercised are an ungranted sibling
container, malformed/slow local peer, copied public Credential, stale Instance,
wrong Target/key/network/capability, and one ordinary Route actor under R-030's
role-local protocol view. Conditions are the frozen Docker isolation, fresh
fixture, exact four-position Route, standard-library signature/TLS primitives,
and complete evidence. Measurement is black-box rejection, exact byte/digest
agreement, role/process views, resource high-water facts, and cleanup.

Limitations: same-host management, project-controlled keys and Nodes, Docker
isolation, and laboratory adapters do not prove anonymity, operator independence,
decentralization, Windows support, hostile same-user isolation, cross-host
performance, production IPC, or release safety. Encryption is not described as
anonymity.

## Evidence plan

### Primary sources

Accessed 2026-08-13: accepted R-002, R-006, R-023 (resource/backpressure
definitions only), R-024, R-030, the H3 technical design, Go 1.26 `crypto/ed25519`,
`crypto/rand`, `crypto/sha256`, `crypto/tls`, `net`, `io`, and `context`
documentation, RFC 8446, and Unix-domain-socket documentation. Standard-library
cryptography is a reviewed primitive implementation; Ardents implements only
bounded canonical fixture encoding and orchestration.

### Experiment

From a clean committed HEAD, create new disjoint symlink-safe fixture and evidence
roots outside Git. A preparation command creates public configuration and volatile
private inputs, pins source commit and image identity, and starts the complete
Compose topology. Application containers use `network_mode: none` and only their
scoped IPC mounts. Route actors use only their declared literal links. Authority
Custody is a separate offline process. Run repeated generation-1→generation-2
attempts for `10–30 minutes`.

A separate verifier command/container reads one frozen manifest and bounded
evidence, independently recomputes Target/Credential/generation, grants/session
binding, process separation, stream digests/order, Route positions, migration,
resource limits, forbidden-path absence, and cleanup, then returns exactly
`pass|fail|invalid`. Remove raw private keys and reusable session material before
terminal evidence. An outer digest binds preflight, attempts, verifier output,
cleanup, and terminal verdict. Cleanup failure is `invalid`.

### Mandatory cases and falsification

The tracer is falsified if any of the following is accepted or required:

- wrong/ungranted principal, copied/replayed session, substituted PID/container,
  or post-restart session;
- Connection Grant administration/custody, Administration Grant connection/
  custody, key export, or any runtime access to Service Authority;
- wrong Target, wrong Instance Key, Credential-only publication, invalid time,
  wrong network/capability, stale/lower generation, same-generation conflict,
  malformed publication, or success before fresh publication acknowledgement;
- malformed, oversized, partial, or slow local frames; silent byte drop;
  unbounded memory under a slow reader; dishonest cancellation/partial counts;
- missing Route position, direct or shortened Client→Publisher path, loopback
  data fast path, shared data file/volume, DNS/proxy/ambient network, or an
  Application-visible Node/Route field;
- generation-2 key/credential reuse, changed Target, accepted generation-1 new
  work after supersession, copied old Instance Key, or online Authority;
- incomplete, mutated, contradictory, unbound, secret-bearing, or cleanup-less
  evidence accepted as pass.

A complete valid candidate contract violation is `fail`. Missing or unreliable
harness/evidence is `invalid`. A candidate cannot self-report pass.

## Findings

- **Measurement:** the clean committed Stage 2 local Docker development campaign
  passed `95/95` attempts over `10m13.5548419s`; independent digest and process
  binding checks matched, raw keys were absent, and cleanup completed.
- **Inference:** that result exercises the exact lower Route seam needed for a
  bounded Stage 3 development tracer, but not official Route qualification.
- **Sourced fact:** R-002 and R-006 already fix the product semantics and key
  hierarchy; Stage 3 must implement rather than redefine them.
- **Assumption:** controlled Docker isolation is sufficient to model the
  launcher-brokered Application Principal for this development result only.
- **Inference:** Unix sockets, standard-library Ed25519/TLS, immutable files, and
  bounded in-memory publication are replaceable adapters and create no
  consequential production lock-in, so no ADR is justified.

## Options

### H1 — Deep Service Connection module over the Stage 2 Route

Highest leverage: Applications learn only local admission, exact Target,
classified result, and stream semantics. Credential, publication, Route, carrier,
and cleanup complexity remain local to one implementation. It preserves the
existing product seams and can replace every laboratory adapter later.

### H2 — Application-specific SDK or direct proxying

Rejected. It either spreads routing/credential/retry behavior into Applications
or creates a direct local/network fast path that bypasses the Stage 2 Route. It
also makes tests depend on a shallow SDK rather than the authoritative interface.

### H0 — Stop at research

Required if H1 needs a production foundation decision, custom cryptography,
online root authority, hidden fallback, or Stage 4 recovery semantics.

## Recommendation

Choose H1 for the bounded tracer. Implement the smallest deep module and deepen
the existing Route carrier only for bounded streams. Keep all adapters explicitly
laboratory and replaceable. Confidence is medium: the strongest counterargument
is that Docker-mounted Unix sockets may overstate real desktop principal
isolation; the limitation is therefore explicit and Stage 7 retains cross-platform
qualification.

## Disposition

- State: `decided` by the Product Owner's handoff on 2026-08-13.
- Stage 3 bounded development is authorized from the verified Stage 2 commit.
- No ADR and no production technology selection.
- R-002/R-006/R-024 domain distinctions remain unchanged; `CONTEXT.md` needs no
  new term.
- Generated private material, sockets, captures, images, and evidence remain
  outside Git.
- Official Stage 1/2 qualification and all privacy/release claims remain deferred.
