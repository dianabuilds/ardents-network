# Horizon 3 Stage 3 implementation brief

Status: **authorized by accepted R-031 for bounded development evidence**

Authoritative inputs: accepted ADRs, R-002, R-006, R-023 resource and
backpressure definitions, R-024, R-030, [R-031](../research/records/r-031-h3-service-connection-application-interface.md),
the H3 technical design, product contract, threat model, package map, dependency
register, and repository rules.

## Outcome and public seams

External client and publisher Applications exchange arbitrary opaque bytes over
an exact-Target-authenticated Service Connection through scoped local IPC and the
real four-position Stage 2 Route. Applications contain no Ardents routing,
Credential, publication, Target-proof, or retry implementation. One routine
migration stops generation 1, creates a fresh generation-2 Instance Key and
higher exclusive public Credential for the same Target, republishes, and accepts
a new authenticated connection. Neither Service Authority nor the old Instance
Key enters the new runtime.

Tests observe only these pre-agreed seams:

1. local admission binds one launcher-brokered Application Principal and Local
   Grant to one finite one-use session and interface surface;
2. Service Administration publishes only with matching bounded Credential and
   non-exportable Instance Key possession and returns ready only after fresh
   Introduction acknowledgement;
3. Service Connection consumes one exact Target/current publication and asks the
   Route Module for a protected bounded stream without exposing topology;
4. the Application Interface owns connect/accept/read/write/close/cancel,
   backpressure, honest local byte counts, and R-002 Connection Results;
5. independent qualification recomputes principal, grant, Target, Credential,
   generation, stream, Route, migration, resource, evidence, and cleanup facts.

Private IPC framing, credential encoding, immutable fixture files, in-memory
publication state, TLS carrier/session certificates, goroutine arrangement, and
Compose orchestration are implementation details. Tests do not reach past these
seams.

## Domain and privilege invariants

Service Authority, Service Target, Service Instance Key, Service Instance
Credential, Publisher, Service Instance, Node identity, Route identity,
Application Principal, Local Grant, and ephemeral session capability remain
different concepts. Local Grants never leave their Endpoint. A copyable bearer,
desktop user, PID, port, or filename alone is not an Application Principal.

| Surface | May do | Must not do |
|---|---|---|
| Connection | connect/accept/read/write/close/cancel one scoped Target/Service | publish, configure, issue/export keys, inspect Route |
| Service Administration | publish/unpublish one specified Service with matching current Credential/Key possession | exchange Application Data, issue Credential, export Instance Key, reach Authority |
| Authority Custody fixture | create Authority/Target and issue higher bounded Credential for a supplied public Instance key | carry Application Data, publish online, enter Endpoint runtime |

Connection Grant does not imply administration or custody. Administration Grant
does not imply connection or custody. The hostile sibling receives no grant or
authorized IPC mount. Revocation/restart invalidates descendant sessions; no
ephemeral capability survives broker restart.

## Process and information topology

Every listed role is a separate container/process in the local gate:

| Process | Receives | Emits/learns | Forbidden |
|---|---|---|---|
| client Application | scoped Connection IPC and exact Target | opaque stream and Connection Result | Credential, publication, Nodes/Route, Authority |
| client Endpoint | client Grant/session, Target, authenticated Route input | Target-authenticated Service Connection | publisher app data semantics, Authority |
| publisher Application | scoped accepted-Connection IPC | opaque stream | administration, Credential, keys, Route |
| publisher Endpoint | separate Connection/Admin Grants, current Instance Key handle and public Credential | publication acknowledgement, Target proof, accepted stream | Authority, old key after migration |
| publication operator | scoped Administration IPC, current public Credential and generation-specific key handle | publish/unpublish result | Connection data, Authority, key export |
| hostile sibling | stolen/replayed test bearer and false principal/PID claims | rejection only | either authorized socket/surface |
| Authority fixture | Service Authority and two supplied Instance public keys | one Target and Credentials 1/2 | Endpoint IPC, Route, Application Data |
| four Route actors | exact R-030 role-local duty | adjacent identity and opaque carrier bytes | Application interface, complete plan, plaintext |
| verifier | frozen manifest and bounded observations | recomputed `pass|fail|invalid` | data forwarding or candidate authority |

Application containers have `network_mode: none`, no DNS/proxy environment,
Docker socket, host network, shared memory data path, or shared Application Data
volume. They communicate only through a principal-scoped Unix socket volume.
Endpoint and Route links use literal addresses in the declared Compose network.
The verifier is management-only and never appears in the data path.

Docker isolation and scoped mounts instantiate the launcher-brokered principal
for this development tracer. They do not qualify Windows, installers, arbitrary
same-user desktop processes, or production IPC.

## Laboratory adapters

- Target is a domain-separated SHA-256 commitment to an Ed25519 Service Authority
  public key for the tracer; the Authority signs a bounded canonical Credential.
- Instance possession is proven by an Ed25519-authenticated TLS 1.3 endpoint
  certificate/session whose public key exactly matches that Credential.
- Unix-domain sockets plus a bounded binary framing protocol are the local IPC.
- Publication is bounded current-generation memory plus fresh acknowledged
  Introduction readiness; no database or public resolution protocol is selected.
- Route carrier remains the Stage 2 literal-address TLS 1.3 laboratory adapter.

These use reviewed standard-library primitives and are replaceable. No custom
cryptographic primitive is implemented. They are not production wire, IPC,
credential, publication, storage, or transport decisions; no ADR is created.

## Frozen workload and bounds

- one Target, generations `1` then `2`, one active publication, one connection
  per generation, and no concurrent multihoming;
- one fresh `32-byte` connection canary and seeded `64 KiB` incompressible stream
  in each direction per connection; verifier compares literal expected digests;
- exact four Route positions, one dial per leg, zero retry/reconnect/fallback;
- `4 KiB` local control frames, `16 KiB` chunks, `64 KiB` logical stream per
  direction, and `64 KiB` maximum queued/pending bytes per direction;
- Endpoint → Local Grant/Application → Service/Isolation Context → connection/
  operation budgets; children never enlarge parents;
- per Endpoint: at most `8` accepted IPC connections, `2` Service Connections,
  `24` IPC/Service goroutines, `48` owned FDs, `16` timers, and `8` socket/control
  files; at most six live ephemeral sessions across the fixture;
- `5 s` admission/publication/Route setup step, `15 s` complete transfer,
  `2 s` backpressure observation, `5 s` close and shutdown;
- no persistent Application Data, session resumption, retry queue, Credential
  cache, reusable bearer file, private-key evidence, or post-restart live state.

A slow consumer must backpressure a local write before memory grows past the cap.
No accepted byte is silently dropped. A write count means locally accepted bytes
only. After partial write, timeout, cancellation, or connection loss, remote
completion is unknown. Clean close is not semantic success. Ardents performs no
Application-operation replay or hidden new connection.

## Credential and migration contract

A Credential binds exact Target, Instance public key, exclusive generation,
not-before, not-after, network ID, and the publication/connection capability.
Validation is fail closed for malformed encoding/signature, wrong Target/key,
Credential-only possession, not-yet-valid/expired validity, wrong network or
capability, lower/stale generation, and conflicting same generation.

Publication success requires:

1. authorized Administration Principal and fresh one-use session;
2. exact service-scoped Grant;
3. valid matching Credential and Instance Key possession;
4. higher/current exclusive generation and no conflict;
5. local listener and Route/Introduction readiness;
6. a fresh Introduction publication acknowledgement bound to Target, generation,
   validity, and current publisher process.

Connection success is emitted only after exact Target recomputation, Credential
validation, current exclusive generation, Instance-key TLS proof, and complete
Stage 2 Route authentication. Local listener or publication lookup alone is
never success.

Migration unpublishes and terminates generation 1, erases its runtime key handle,
creates generation 2 on the new runtime, advances the exclusive generation,
publishes the same Target, and opens a new connection. Generation 1 rejects new
work after supersession. A bounded unavailable gap is valid. Existing-connection
handoff, Route recovery, and multihoming are Stage 4/non-goals.

## Result and evidence

Connection Results use only R-002 product classes: invalid destination; local
authorization/policy/resource denial; evidenced Service unavailable; evidenced
Route unavailable; Target authentication failure; local timeout/cancellation;
authenticated established; clean close; abrupt loss; indeterminate failure.
No Node, hop, carrier, socket, generation-store, or guessed remote cause is
Application-visible.

Each attempt evidence binds schema, source commit, image identity, manifest
digest, network/Target/public Authority commitment, Credentials 1/2 without
private material, principal/Grant/surface commitments, one-use session outcomes,
publication acknowledgement, exact four Route positions by commitments, process
and container identities, Target/Instance authentication, expected/observed
stream lengths/digests, local write counts, cancellation/backpressure results,
migration facts, resource high-water values, negative results, cleanup, verifier
output, and evidence digest.

- `pass`: complete valid evidence and every function, least-privilege,
  authentication, stream, migration, resource, forbidden-path, and cleanup
  conjunct passes;
- `fail`: evidence is complete and reliable but candidate behavior violates a
  frozen conjunct;
- `invalid`: fixture, manifest, process observation, secret handling, evidence
  integrity, verifier independence, or cleanup cannot support a judgment.

Missing, mutated, contradictory, unbound, secret-bearing, or incomplete evidence
is `invalid`. Complete wrong bytes, privilege acceptance, wrong authentication,
forbidden path, stale generation acceptance, silent drop, resource breach, or
candidate leak is `fail`.

## TDD vertical slices

Work one red→green behavior at a time through the public seams:

1. correct principal + Connection Grant receives one fresh Connection session;
   wrong principal, surface, replay, substitution, and restart reuse reject;
2. correct Administration Grant publishes only its service with matching
   Credential/Key; connection access cannot administer and neither can access
   custody/export;
3. exact Target/current Instance succeeds; wrong Target/key, Credential alone,
   invalid validity/network/capability, stale/lower/conflicting generation, and
   malformed publication reject;
4. publication is not ready before fresh Introduction acknowledgement;
5. external Applications exchange exact arbitrary streams in both directions
   through all Route positions without Ardents logic;
6. direct/short/loopback/shared-file/DNS/proxy/ambient and Application-visible
   Route shortcuts are structurally absent and negatively tested;
7. malformed, oversized, partial, and slow IPC/control frames fail closed;
8. slow consumer triggers bounded backpressure; cancellation before transfer and
   after partial local write returns honest counts/class and cleans up;
9. generation 1→2 preserves Target, changes key/Credential/generation, rejects
   old new-work, and never copies Authority/old key;
10. evidence mutation/missing/contradiction is `invalid`; complete candidate
    violation is `fail`; all terminal ownership is empty.

Keep every Stage 1/2 suite. Run the smallest affected test after each slice,
`make quick-check` throughout, and `make check` before integration with the
repository-approved Go 1.26.5 toolchain.

## Real Docker development gate

One public command creates fresh external fixture/evidence roots, validates
clean committed source, builds/pins the image, launches the complete Compose
topology, repeats full generation-1→2 attempts for `10–30 minutes`, invokes the
separate verifier, owns teardown, removes the private fixture, and writes the
outer digest. Fixture/evidence roots are new, disjoint, outside Git, non-symlink,
and owner-only. Source commit and image ID are immutable inputs.

The verifier is a separate command/container and returns exactly
`pass|fail|invalid`. It imports no candidate validator in production. Raw
Authority/Instance private keys and reusable sessions are removed before the
terminal verdict. Docker containers, network, sockets, listeners, processes,
goroutines, queues, publications, sessions, and private fixture root must be
empty. Any cleanup failure is `invalid`.

## Non-goals and stop conditions

No Service Names, general Private Resolution, Bridge, blocked entry, same-
connection recovery, churn/capacity, installer/update/rollback, production vault,
Authority Recovery UI, production transport/wire/IPC/storage, consensus,
blockchain, mandatory SDK, Ardents message/HTTP semantics, datagrams, retention,
exactly-once, multihoming, safe same-Target Authority-compromise recovery,
Windows qualification, anonymity, decentralization, or public release claim.

Stop and return to R-031 if implementation needs online Service Authority,
production foundation selection, custom cryptography, H2 lab imports, direct or
short forwarding, Route disclosure, hidden retry/reconnect/fallback, unbounded
work, Stage 4 recovery, or weakening any Stage 1/2 gate.

## Definition of Done

- R-031, this brief, status/gate documents, and code state the same bounded
  development authorization and limitations;
- Applications see only scoped IPC stream/results and contain no Ardents logic;
- Connection, Administration, and Custody privileges remain non-collapsing;
- exact Target/current Instance proof and fresh publication acknowledgement
  precede success;
- exact opaque bidirectional bytes traverse the real Stage 2 Route;
- routine migration preserves Target and replaces key/Credential/generation;
- backpressure, partial-write, cancellation, negative credentials/principals/
  routes/evidence, and cleanup are bounded and tested;
- a clean committed HEAD passes `make check`, then a real `10–30 minute` local
  Docker campaign passes independent verification and digest/cleanup recheck;
- final Standards and Spec reviews have no actionable findings;
- logical docs, implementation, and qualification commits are retained;
- final report separates facts, measurements, limitations, deferred official
  gates, and exact prerequisites for Stage 4.
