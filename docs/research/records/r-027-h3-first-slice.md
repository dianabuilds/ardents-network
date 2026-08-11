---
id: R-027
title: What is the smallest first Horizon 3 vertical slice?
status: decided
owner: product research
started: 2026-08-11
reviewed: 2026-08-11
---

# R-027 — First Horizon 3 slice

**Current disposition:** the standalone bootstrap-only recommendation and
implementation order are superseded and withdrawn by accepted R-029 because
they end without a real product state consumer. The source, Epoch, persistence,
freshness, conflict, and evidence mechanics are accepted only as the bootstrap
appendix to integrated Stage 1.

Where this record still says H3-A/H3-B/H3-C, recommends bootstrap-first order, or
requires its standalone soak schedule, that text is historical and
non-normative. R-029 controls Stage 1 scope, order, and evidence duration.

## Decision this unlocks

Decide what an implementation agent may build first after Carrier Lab and the
Named Unlisted Site tracer, without treating the whole Closed Test Network
horizon as one task. The result must advance a real product journey, retain
replaceable seams, and produce evidence strong enough to decide whether the next
H3 slice is rational.

This record originally recommended **H3-A: bounded authenticated Network
Epoch/bootstrap**. [R-029](r-029-h3-authenticated-node-lifecycle.md) supersedes
that order and adopts its bounded mechanics only inside authenticated state plus
a real Node lifecycle. R-029 supplies the implementation authority for
integrated Stage 1 only.

## Current contract

The following are already fixed:

- [scope](../../product/scope.md#horizon-3--closed-test-network) permits only
  one separately scoped H3 vertical slice at a time;
- [ADR-0004](../../adr/0004-authenticated-epochs-and-separated-control-roots.md)
  separates authorization of shared state from distribution;
- [R-009](r-009-hostile-bootstrap-and-bridge-entry.md) selects an expiring,
  content-addressed, threshold-authenticated Network Epoch whose identical bytes
  may arrive through several bounded channels;
- the [operating model](../../product/operating-model.md) fixes Time Confidence,
  Direct Source Exposure, Candidate View, Common Readiness, and Work Safety
  semantics. H3-A exercises only controlled Time Confidence, conservative source
  exposure, and a bounded work lease. Its synthetic candidate fixture is not the
  product Candidate View; H3-A cannot claim canonical Common or capability
  readiness because transparent input, full audit, Release Safety, qualified
  Route Domains, and Gate E are absent;
- [ADR-0009](../../adr/0009-go-project-foundation.md) selects Go, but no
  production transport, storage engine, consensus, wire format, deployment
  system, or route implementation;
- the actual team is one Product Owner and Codex. Project-controlled hosts,
  keys, and synthetic family fixtures cannot prove independent operation.

## Decision question

What is the smallest H3 slice that establishes persistent multi-host
authenticated bootstrap state, publishes an independently
verified bounded readiness result, and can fail before Node admission, routing,
recovery, naming, packaging, or public-network work is allowed to expand?

## Hypotheses

- **H1 — Epoch/bootstrap first:** a bounded synthetic candidate fixture and
  finite sources can establish a durable bootstrap boundary without selecting production
  foundations, then publish externally recomputable readiness events and one
  terminal campaign verdict.
- **H2 — Node lifecycle first:** persistent role processes, Node Records,
  admission, and withdrawal can be built before shared state distribution.
- **H3 — Recovery/capacity first:** topology maintenance, reconnect, overload,
  and useful Node capacity can be tested before persistent authenticated state.
- **H0:** none is narrow and vertical enough; H3 must stop for redesign.

## Evaluation criteria

The first slice must:

1. establish the narrow authenticated bootstrap boundary that H3-B must consume
   before it can honestly replace H2's preconfigured application topology;
2. end in an observable durable bootstrap/readiness outcome, not merely parse
   metadata;
3. use a fixed small topology and finite state, retries, resources, and time;
4. exercise rollback, fork, expiry, restart, source failure, and cleanup;
5. avoid a production protocol, DHT, database, orchestrator, or public API
   decision;
6. remain executable and maintainable by the one-to-one team;
7. expose a falsifiable stop result before the next slice is considered;
8. make no claim that synthetic keys or hosts are independent operator families.

## Options

| Option | Product value | Main problem | Result |
|---|---|---|---|
| H1: Epoch/bootstrap | Establishes the authority/distribution boundary every later lifecycle decision needs; H2 replacement remains conditional on H3-B consuming it. | Time, fork, persistence, and source exposure must be bounded honestly. | **Recommend first.** |
| H2: Node lifecycle | Makes contribution and withdrawal tangible. | Admission needs an authoritative current membership state; building it first creates a temporary manifest/control plane that H1 later replaces. | Sequence after H1. |
| H3: recovery/capacity | Exercises hostile operation early. | There is no stable role work unit, membership state, or lifecycle against which recovery and capacity can be measured. | Sequence after H2. |
| H0: redesign | Avoids premature implementation. | Necessary only if the H1 contract cannot stay bounded or requires a production foundation. | Stop condition, not current choice. |

**Inference:** H1 is the only option that is both end-to-end within the bootstrap
journey and foundational without pretending to select a production network
stack. It establishes an authenticated state boundary that H3-B can consume.
It does not yet replace H2 or prove the canonical product Candidate View; later
slices consume evidence rather than inventing incompatible temporary truth.

## Findings

- **Sourced fact:** accepted scope permits only one H3 vertical slice at a time.
- **Sourced fact:** accepted Network Epoch architecture already separates state
  authority from byte distribution and requires explicit expiry, rollback, and
  fork behavior.
- **Sourced fact:** R-023 has no role-specific infrastructure capacity floor
  until a measurable role/work unit exists.
- **Assumption:** the exact four isolated project-controlled Ubuntu VMs defined
  below and a controlled clock preflight are available for H3-A evidence.
- **Inference:** Epoch/bootstrap must precede Node admission because admission
  otherwise invents a temporary authoritative membership source.
- **Inference:** the one-to-one project can validate mechanics and boundedness,
  but not operator, signer, source, builder, or auditor independence.

## Horizon 3 program map

The whole horizon may be designed now but is implemented sequentially:

1. **H3-A — authenticated Epoch/bootstrap:** this record.
2. **H3-B — Node lifecycle and first state consumer:** separate role processes,
   Node Record publication, probation, eligibility, stop-new-work, drain,
   withdrawal, revocation, non-overlapping Role Domain Assignment, and the first
   runtime consumer of authenticated state. Its own research must decide how the
   H3-A fixture is replaced by or mapped into the canonical Candidate View;
   H3-A does not pre-authorize that design.
3. **H3-C — recovery and capacity:** topology maintenance, purpose-specific
   reconnect, ordinary and overlapping failures, overload, backpressure, and
   role-specific useful-work floors.
4. **H3-D — Bridges and camouflage:** blocked-entry acquisition and transport
   replacement as a separate threat-model slice.
5. **H3-E — permissionless Namespace:** claims, leases, recovery, and qualified
   Private Resolution; not an extension of Epoch membership.
6. **H3-F — install/update/rollback:** production-shaped lifecycle research,
   separate from Network Epoch authority.
7. **H3-G — Windows and Application isolation:** platform integration after the
   network contract and process boundaries stabilize.

No success in one item authorizes any later item automatically.

## Proposed H3-A claim

> In one visibly centralized, project-controlled, persistent Ubuntu test
> network, a clean or restarted Endpoint obtains one identical
> project-authorized experimental Epoch fixture through a finite source plan, rejects
> malformed, tampered, lower-than-durable-epoch-floor, observed-fork, expired,
> incompatible, and wrong-network state, persists its security floors and
> conservative H3 Attempted Source History across restart, and emits an ordered
> readiness-event stream whose positive transitions a separate harness
> process recomputes from retained state and evidence, followed by one terminal
> campaign verdict.

This claim protects the accepted experimental Epoch and fixture bytes against a
malicious distributor, unavailable source, stale
cache, process restart, and replay of a lower Epoch while the durable epoch floor
remains intact under the declared controlled-clock condition. It does **not**
protect against rollback of the complete state directory, capture of the project
test-key threshold, concealed common control, a Broad Traffic Observer, endpoint
compromise, or the absence of independent operators. It is measured by the
verification/source/restart matrix and deterministic verdict below.

## Honest operator boundary

Every real host, signer, distributor, and Endpoint is under one actual
administrative organization. Therefore:

- all real participants are one project-control family for claims;
- distinct keys and processes prove only key/process separation;
- synthetic family labels may exercise fixture filtering and Role Domain
  invariants but
  are marked `synthetic / unqualified` in every manifest and verdict;
- no runtime option may bypass a failed family check by saying “ignore family”;
- H3-A cannot satisfy public source-family, Route-family, signer-custodian,
  builder, or auditor independence gates.

There is a deliberate mechanical contradiction in a one-operator fixture: the
same honest operator family cannot be both source-only and eligible in four Route
Domains. H3-A may exercise those invariants with synthetic fixtures, but it does
not create the product Candidate View, materialize a Route, or prove that any
candidate set is usable by a Node. That first consumer boundary belongs to H3-B.

## Exact slice boundary

### Required topology

- exactly four non-overcommitted Ubuntu 26.04 LTS `x86-64` VMs in the canonical
  evidence topology:
  - `E`: `2 vCPU`, `2 GiB`, `100 Mbit/s`, Endpoint H3-S process tree;
  - `S1` and `S2`: each `2 vCPU`, `2 GiB`, `100 Mbit/s`, one source-only
    distributor H3-S process tree;
  - `H`: `4 vCPU`, `8 GiB`, `1 Gbit/s`, harness, two open-loop load generators,
    central evidence sink, and the offline signer invocation before a run;
- one offline signer operation on `H` with no listener and no route duty;
- static participant and source sets frozen in the pre-run manifest;
- distinct process identity, key material, state directory, listener, cgroup,
  and local diagnostic boundary for every online role;
- no two full H3 process profiles silently charged to the same parent resource
  budget;
- one ordered readiness-event stream, mechanically recomputed verification
  records on `H`, and one terminal campaign verdict.

All candidate data links use the frozen controlled envelope: symmetric
`100 Mbit/s`, `80 ms` added emulated RTT, independent `0.1%` programmed loss per
direction, and measured post-impairment `RTT p95 - p50 <= 10 ms`. Before
impairment, every qualifying pair must have RTT p50 `<= 5 ms` and
`p95 - p50 <= 2 ms`; afterward p50 must be in `[80 ms, 86 ms]`. A separate
management plane is unreachable from candidate processes and carries only
supervisor/evidence traffic. Harness-owned
local collectors on E/S1/S2 use the host reserve outside candidate cgroups and
stream to H. Any consolidated local reproduction is non-qualifying.

The persistent multi-host H3 roles use pre-provisioned Ubuntu and host-level
systemd/cgroup-v2 supervision as a **test fixture**, not a selected production
installer or orchestrator. Kubernetes, Nomad, a cluster database, and a public
control service are excluded.

### Included

- installed test trust root, network identity, lab-only source trust map, and
  distributor client trust map
  binding each permitted source transport key to one source identity, synthetic
  family marker, opaque endpoint handle, sensitive literal endpoint/SAN, and the
  fixed mutual-TLS profile; client entries bind E plus the two harness load
  identities without reusing any key;
- a synthetic `2-of-3` set of ordinary Ed25519 test signatures, solely to
  exercise threshold mechanics and transition failure;
- one finite pre-generated corpus of exactly `192` sequential content-addressed
  experimental Epoch envelopes, each carrying a bounded synthetic candidate
  fixture and capped at `32 KiB` for normal workload; runtime retains one active
  and at most one pending Epoch, not the corpus. Separate hostile cases exercise
  the general `1 MiB` envelope cap;
- package/bootstrap input, last-known-good cache, two authenticated source-only
  mirrors, and explicit offline-file import;
- a finite source sequence, conservative H3 Attempted Source History, durable
  epoch/time floors, observed-fork state, and atomic activation;
- one signer-policy transition fixture;
- controlled Time Confidence, expiry, restart, source exhaustion, and actual
  host/process shutdown;
- the R-028 H3-S resource, overload, diagnostics, and evidence contract;
- ordered retained readiness events, mechanically recomputed verification
  records, and one terminal campaign verdict.

### Explicitly excluded

- permissionless Node publication or admission, public join API, incentives,
  reputation, Sybil resistance, or independent operator qualification;
- dynamic Candidate Materialization, DHT, gossip, peer exchange, discovery
  expansion, transparency log, or full auditor implementation;
- Candidate-View-to-runtime materialization, a persistent Node process, Route,
  Named Site, Private Resolution, or Application Data canary;
- Bridges, censorship UX, transport camouflage, NAT traversal, relay discovery,
  or public DNS;
- permissionless Service Names, lease/recovery, naming governance, or a new
  Private Resolution implementation;
- production updater, installer, rollback, Authority Recovery, Windows, SDK,
  browser, mobile, or general Application sandbox;
- production encoding, wire protocol, storage engine, consensus protocol,
  routing library, or cryptographic primitive;
- public anonymity, availability, decentralization, censorship-resistance,
  performance, or security qualification.

The **synthetic candidate fixture** below is a lab-only authenticated array used
to exercise exact-byte parsing, assignment bounds, and source/candidate
separation. It deliberately has no transparent input-log root or cutoff, global
eligibility/capacity/concentration summaries, inclusion/rejection proofs,
Candidate Materialization, or full auditor. It is therefore never called or
counted as the product Candidate View.

## Network Epoch experiment contract

### Envelope and body

Signatures bind the exact received body bytes, never a reserialized object:

```text
content-digest = SHA-256("ardents-h3-epoch-content-v1\0" || body-bytes)
signature-input = "ardents-h3-epoch-signature-v1\0" ||
                  uint16be(length(network-identity-bytes)) ||
                  network-identity-bytes ||
                  content-digest
```

Each ordinary Ed25519 signature covers that exact `signature-input`. The
envelope carries `content-digest`, signer-policy version, and a finite set of
distinct key-id/signature pairs; its policy version must equal the authenticated
body and the current **verification policy**. That policy is installed P1 for
packaged genesis/ordinary P1 successors, the active predecessor's policy for an
ordinary same-policy successor, or the committed next policy for its activating
successor. Threshold means M valid signatures from N accepted keys. Every
signature carried in the envelope must itself be known, unique, correctly
ordered, and valid; an extra invalid or unknown signature rejects the complete
envelope even when two other signatures pass. H3-A implements no aggregate or
novel threshold cryptography.

The lab encoding is fixed for this experiment and cannot escape the H3 Module.
`body-bytes` are UTF-8, minified JSON with no BOM or insignificant whitespace.
Top-level fields occur exactly in this order:

```text
schema, network_identity, epoch_number, previous_digest,
valid_after, fresh_until, valid_until, signer_policy,
next_signer_policy, protocol_profiles, candidate_fixture, sources
```

`next_signer_policy` is either `{}` or a complete fixed-order transition object;
it is never omitted or `null`. Identifiers, public keys, digests, and opaque
endpoint handles are fixed-length lowercase hexadecimal. Times are base-10 Unix
seconds and counts/capacities are base-10 unsigned integers, with no sign,
fraction, exponent, or leading zero except literal `0`. Strings use their
shortest JSON escaping. Unknown/duplicate/out-of-order fields, floats, `null`,
invalid UTF-8, trailing bytes, or a byte sequence different from the canonical
generator output are rejected.

The scalar schema is also fixed; an agent must not infer lengths or ranges:

| Field | Exact H3-A representation |
|---|---|
| `schema` | ASCII string `ardents-h3-epoch-body-v1`. |
| `network_identity` | `64` lowercase hex characters encoding `32` bytes. `network-identity-bytes` in the signature preimage is this decoded value, so its encoded length prefix is always `uint16be(32)`. |
| `epoch_number` | JSON integer in `1..9223372036854775807`. Epoch `1` is the only packaged genesis and alone may use an all-zero `previous_digest`; every later value is exactly its accepted predecessor plus one. |
| Digests, signer `key_id`, `node_id`, and `source_id` | `64` lowercase hex characters encoding `32` bytes. A `key_id` is `SHA-256("ardents-h3-key-id-v1\0" || raw-ed25519-public-key)`. |
| Ed25519 public key | `64` lowercase hex characters encoding the ordinary `32`-byte public key. |
| `family` and `endpoint_handle` | `32` lowercase hex characters encoding an opaque `16`-byte fixture value. |
| Policy `version` | JSON integer in `1..4294967295`; the installed P1 policy is version `1`. |
| Policy `threshold` and key count | Literal threshold `2` and exactly `3` distinct keys in H3-A. The envelope contains `2` or `3` distinct, ordered signatures from that exact policy. |
| Time fields | JSON integer Unix seconds in `1..9223372036854775807`, additionally constrained by the temporal rules below and the frozen manifest. |
| `protocol_profiles` | `1..8` unique strings, sorted by ASCII bytes, each matching `[a-z0-9][a-z0-9._-]{0,63}`; the set must equal the pre-run manifest's required H3-A profiles. |
| Candidate `role` | Exactly one of `initiator`, `rendezvous`, `responder`, or `introduction`. |
| Candidate `capacity` | Literal `1`. It means one eligible synthetic fixture slot and is deliberately not a measured Node-capacity unit. |
| Fixture/source counts | `candidate_fixture.count` equals the `1..64` record-array length; `sources` contains exactly the two installed source records. |

All string values are therefore ASCII in H3-A. Decoding a wrong length, mixed or
uppercase hex, forbidden profile character, an unknown role, an out-of-range
integer, a duplicate key/record/source/handle, or a count mismatch fails before
signature-policy, chain, or freshness selection. The body `signer_policy` must
byte-for-byte describe the installed or previously authorized policy; signed
bytes cannot nominate their own untrusted verification keys.

Signer keys and envelope signatures are ordered by decoded key id; protocol
identifiers by ASCII bytes; Candidate records by decoded Node identity; source
records by decoded source identity. Nested objects also use their schema order.
Those orders are exact: signer policy is `version, threshold, keys`; each key is
`key_id, public_key`; a non-empty next policy is
`activate_at, version, threshold, keys`; the synthetic candidate fixture is
`digest, count, records`; each Candidate record is
`node_id, family, role, endpoint_handle, capacity, not_before,
no_new_work_after, not_after`; each source record is
`source_id, family, endpoint_handle, transport_key_digest`.
The synthetic candidate-fixture digest is
`SHA-256("ardents-h3-candidate-fixture-v1\0" || records-array-bytes)`, where
`records-array-bytes` is the exact canonical JSON byte span of the `records`
array and excludes the digest field that contains it.

The binary envelope is exactly:

```text
8-byte magic "ARDH3E1\0"
uint32be signer-policy-version
uint32be body-length
32-byte content-digest
uint8 signature-count (maximum 3)
signature-count * (32-byte key-id || 64-byte Ed25519 signature)
body-length body-bytes
```

There are no extensions or trailing bytes. Passing H3-A selects neither JSON nor
this envelope as a public wire format.

The body contains only:

- schema and network identity;
- monotonic epoch number and previous accepted digest;
- `valid-after`, `fresh-until`, and `valid-until`;
- signer-policy version and a bounded authenticated transition when present;
- exact allowed experimental protocol/profile identifiers;
- synthetic candidate-fixture digest, count, and at most `64` static test records;
- for each record: Node public identity, synthetic family marker, fixed role
  eligibility, opaque synthetic endpoint handle, finite capacity declaration,
  and assignment
  `not-before`, `no-new-work-after`, and `not-after`;
- source identity, family, opaque endpoint handle, and transport-key digest
  declarations needed by the experiment. These are checked against the installed
  source trust map; they cannot authenticate the distributor that supplied their
  own bytes.

The current digest is computed over the complete body bytes and appears only in
the envelope and local persistent state. It is deliberately absent from the body
it hashes, so the exact-byte contract has no self-reference.

The complete envelope is capped at `1 MiB`. It contains no Service Name, Service
Target, Application Data, route, query, User identity, history, diagnostics, or
secret key. The cap and `64`-record limit are H3-A experiment constants, not
public-network scalability claims.

Every Candidate record has a unique Node public identity and endpoint handle,
exactly one synthetic
Role Domain, one finite positive capacity, and strict
`not-before < no-new-work-after < not-after <= valid-until` bounds. A source-only
identity cannot appear in the candidate fixture. Duplicate identities/endpoint handles,
unknown roles, zero/overflow capacity, equal/reversed assignment times, or an
assignment beginning before `valid-after` are invalid. These are H3-A fixture
invariants, not a public admission protocol.

### Freshness state machine

```text
unverified
  -> staged         valid signature/chain, before valid-after
  -> fresh          valid-after <= trusted time < fresh-until
  -> stale          fresh-until <= trusted time < valid-until
  -> expired        trusted time >= valid-until

any state -> conflicting | incompatible | invalid
```

- `fresh` may start new network work;
- `stale` starts no new work, while already established work may continue only
  to its earlier Work Safety Lease or `valid-until`;
- `expired`, `conflicting`, `incompatible`, and `invalid` remove readiness and
  start no work;
- no grace interval silently extends `valid-until`;
- two threshold-valid bodies with the same epoch number and different digests
  enter persistent `conflicting`; they are never merged or selected by source
  preference;
- H3-A may stop in conflict. Automatic public fork recovery is not invented in
  this slice.

The limitation is explicit: a distributor without threshold keys cannot alter
authenticated experimental Epoch/body/candidate-fixture bytes, but if the signer
threshold has already authorized two forks, a distributor can selectively
withhold one. H3-A detects equivocation only after both digests are observed;
before then it cannot prove that an unseen fork does not exist.

Every body must satisfy the strict temporal invariant
`valid-after < fresh-until < valid-until`. Equal or reversed boundaries are
malformed and rejected before a freshness state is assigned.

One committed generation contains one `active` Epoch and at most one higher
`pending` Epoch. A future-valid Epoch is staged only in `pending`; it never
replaces a still-fresh active Epoch or changes readiness. At `valid-after`, one
serialized timer transition revalidates its chain/time bounds and atomically
commits it as active. Restart reconstructs the same timer from authenticated
time and the durable epoch/time floors. A second future Epoch while `pending` is full is
an explicit bounded unsupported result in H3-A. Tests cover fresh N plus future
N+1, restart before/after activation, and no readiness gap caused merely by
staging.

Tor's directory documents demonstrate useful separation between multiply signed
state, caches, content digests, and `valid-after / fresh-until / valid-until`, but
Tor clients may continue using a reasonably live expired consensus. Ardents
reuses the time vocabulary and distribution separation, not that additional
expiry grace. Sources: [Tor directory outline](https://spec.torproject.org/dir-spec/outline.html)
and [client operation](https://spec.torproject.org/dir-spec/client-operation.html),
accessed 2026-08-11.

### Lab source protocol and observed-head discovery

The sensitive source trust map resolves each signed opaque source endpoint
handle to one literal `IP:port`, source identity/family, test CA, expected DNS
SAN, and Ed25519 leaf-public-key digest. Transport is mutual TLS 1.3 over TCP;
both certificate chains and leaf pins must pass, client and source keys are
separate from Epoch signer keys, and no DNS resolution occurs.

The source transport digest is
`SHA-256("ardents-h3-source-transport-key-v1\0" || raw-ed25519-public-key)`.
The distributor verifies every test client certificate against the installed
client CA and the exact pinned raw Ed25519 key/role in its client trust map. E is
the only Endpoint acquisition identity. For workload cells, S1 additionally
permits only harness load identity L1 and S2 only L2; both are
frozen in the client trust map and use certificates/keys distinct from E, each
source, every signer, and each other. L1/L2 have no source, candidate, Node,
Route, or control role, and their requests are attributed separately from E in
every verdict. No private key is copied between hosts.

Every connection performs a full TLS 1.3 handshake: client session caches and
server session tickets are disabled, early data is forbidden, and the selected
cipher/certificate details are retained. The request
`network-id-digest` is
`SHA-256("ardents-h3-network-id-v1\0" || network-identity-bytes)`. These pins and
the literal address/SAN mapping are installed inputs; no value learned from an
Epoch response can authenticate the connection carrying it.

Each TLS connection carries exactly one request and one response. Integers are
unsigned big-endian:

```text
request  = "ARDH3Q1\0" || uint8 opcode || 32-byte network-id-digest ||
           32-byte object-digest
response = "ARDH3S1\0" || uint8 status || 32-byte object-digest ||
           uint32 payload-length || payload
```

Opcode `1` is `LATEST_OBSERVED` and requires an all-zero request object digest;
opcode `2` is `BY_DIGEST`. Status bytes are exactly `0=OK`, `1=NOT_FOUND`,
`2=BUSY`, `3=BAD_REQUEST`, and `4=INTERNAL`; only `OK` has a non-zero digest and
Epoch-envelope payload whose embedded `content-digest` exactly equals the header
`object-digest`. Thus the protocol object digest is the already defined
`SHA-256("ardents-h3-epoch-content-v1\0" || body-bytes)`, not a second hash of
the envelope. `BY_DIGEST` may return only that requested digest; the
highest-locally-available rule applies only to `LATEST_OBSERVED`. Every non-`OK` response
has an all-zero digest and zero payload length. No extensions, trailing bytes,
or payload beyond the `1 MiB` complete-envelope cap are accepted. One source may
return only its highest locally available object; it does not define global
current state.

Every clean acquisition sends `LATEST_OBSERVED` to the complete finite source
wave and waits until every configured source has a terminal result or the `15 s`
wave deadline before activating a newly observed Epoch. At most two requests are
concurrent. H3-A accepts at most one monotonic step above the durable active
Epoch, but an exact current or exact pending digest is also a valid
revalidation/classification result. A lower digest/number or a gap above the
direct successor is explicit `rollback`/`chain unavailable`, not an unbounded
history fetch.
When a response header reveals a digest but its body fails to arrive, one
`BY_DIGEST` attempt may request that exact object from the other source. Each
source receives at most one request per opcode per durable cycle.

The selected object is the highest **observed** threshold-valid member of
`{exact current, exact pending, direct successor}`, not a claim of global latest
state. Same-number/different-digest objects observed
before the wave barrier persist conflict. A fast valid A and delayed conflicting
B cannot create readiness between responses. If N and N+1 are both fresh, N+1
wins only when it chains to durable N. If the only reachable distributor hides
N+1 and returns still-fresh N, H3-A cannot detect the withholding; the manifest
and verdict say `latest completeness unproven`.

### Finite acquisition algorithm

```text
load installed roots, network identity, source trust map, and packaged baseline
read and verify active/pending Epoch, epoch/time floors, conflict, attempted-source history,
and any durable acquisition cycle
try bounded local package/cache candidates
resume a durable unfinished cycle without repeating a started attempt, or
persist a new cycle id, selector, randomized source order, attempts, and backoff
for the complete direct-source wave, at most two requests concurrently:
    reject pre-contact identity/family collision
    atomically mark attempt started and add conservative source history
    authenticate the source against the installed trust map and mutual TLS profile
    enforce connect/read/total deadlines and response-size cap
    validate exact bytes through the complete Epoch pipeline
    record one terminal source result
wait for the complete wave barrier; persist any observed conflict
choose only the highest observed valid exact-current/exact-pending/direct-successor result
atomically commit one complete security-state generation
publish H3-A Epoch Ready (unqualified) only after persistence succeeds
otherwise return one explicit bounded failure
```

Rules:

- local package and last-known-good state are validated before use and do not
  become current merely because they exist;
- automatic direct network sources are a manifest-frozen finite set. Each source
  receives at most one `LATEST_OBSERVED` and one `BY_DIGEST` request in one
  durable acquisition cycle;
- before any direct dial, serialize a new complete security-state generation
  that marks the attempt `started` and adds the installed source identity/family
  to the non-decreasing H3 Attempted Source History. A crash may therefore
  overcount an attempted contact but cannot forget
  one that reached the network;
- `started` consumes that source/opcode attempt across restart. On recovery it
  becomes terminal `interrupted`; the same durable cycle never redials it;
- a cycle durably records id, purpose, selector/digest, source permutation,
  `not-started/started/terminal` attempts, start/deadline, failure count, and
  `next-automatic-at`. Only one cycle exists at a time;
- after a failed automatic cycle, increment
  `consecutive_failure_count` with saturation at `9223372036854775807`; derive
  and persist `backoff_level = min(consecutive_failure_count - 1, 5)`. Select
  `base` without a shift or multiplication from the exact table
  `{0:60 s, 1:120 s, 2:240 s, 3:480 s, 4:960 s, 5:1800 s}`, then persist a
  uniform locally random delay in `[base/2, base]`; its seed/draw is recorded
  before contact. A newly accepted fresh Epoch or a complete successful
  revalidation of the exact current fresh digest resets both values to zero.
  Manifest-scheduled load
  cycles remain at their fixed cadence but cannot overlap or erase attempt state;
- transport authentication uses only the installed lab source trust map. Epoch
  source declarations are authenticated content used for cross-checking and
  later collision rules, never a bootstrap trust root;
- no more than two direct requests run concurrently;
- each request has `1 s` TCP-connect, `2 s` cumulative TLS, `3 s` first-frame,
  `1 s` maximum read/write-idle, and `5 s` hard total deadlines. Response
  streaming uses chunks no larger than `64 KiB`; the complete wave has a `15 s`
  hard terminal deadline;
- randomized jitter/order is drawn locally and fixed before observed results;
  a test run records its seed before execution;
- after a digest is observed, retrieval fallback requests only that digest. A
  source's `LATEST_OBSERVED` answer may be stale-but-fresh and therefore exposes
  the documented selective-withholding limitation; it never changes network
  identity or bypasses the wave barrier;
- DNS, clearnet HTTP fallback, alternate Namespace, or unbounded address
  discovery is forbidden;
- success requires authenticated protocol completion, not TCP connection;
- cancellation, authentication mismatch, wrong network, incompatibility, local
  resource exhaustion, timeout, and complete source exhaustion are distinct
  bounded results;
- a source identity, family, or endpoint handle already present in the retained
  active/pending candidate fixture is rejected before contact; a collision exposed
  only by a newly authenticated Epoch body is persisted and fails explicitly;
- H3 Attempted Source History and epoch/time floors survive process and host
  restart and are not reset by an Application or Isolation Context. This
  conservative history never retires entries in H3-A; it proves no-forgetting,
  not the lease/derived-work retirement of product Direct Source Exposure.

Deterministic aggregate error precedence is: persistent-state/clock failure
before contact; then observed conflict; commit failure; cancellation; local
resource failure; highest observed valid result; otherwise source authentication,
framing/size, ordered Epoch-validation, timeout, and unavailable terminal
classes. Every lower-priority observation remains in evidence. A refresh failure
does not erase a still-fresh active Epoch; a clean start with no active Epoch
remains not ready.

Tor's directory client uses randomized bounded retry timing and retries the same
object at another cache; its guard design also persists a bounded sampled set to
avoid endless rotation. These are design references, not copied protocol:
[directory retry](https://spec.torproject.org/dir-spec/client-operation.html) and
[guard algorithm](https://spec.torproject.org/guard-spec/algorithm.html),
accessed 2026-08-11.

### Time Confidence in H3-A

H3-A does not pretend to solve hostile clean-clock bootstrap. Its multi-host
fixture has a pre-run externally recorded wall-clock offset of at most `2 s` and
then uses:

- `epoch_floor = {highest accepted epoch_number, its content_digest}`;
- `trusted_time_floor = highest whole Unix second durably used for a security or
  readiness decision`;
- one process-lifetime anchor `{observed wall second, monotonic timestamp}`;
- the authenticated Epoch bounds and the harness's external clock observation.

At process start and before every readiness transition, the harness observation
must be within `2 s` of the host wall clock. A restart wall clock more than `2 s`
below `trusted_time_floor` yields `clock uncertain`; within the allowed error,
`trusted_now = max(host wall time, trusted_time_floor)`. During one process,
`trusted_now` is the maximum of current wall time and anchor wall time plus
monotonic elapsed time, so a backward wall adjustment cannot extend freshness
and a forward jump can immediately make state stale or expired.

Before accepting an Epoch, activating pending state, emitting readiness, starting
a direct-source cycle, and graceful shutdown, the state owner durably advances
`trusted_time_floor` to the whole second used by that decision. While positive
readiness exists, a fixed schedule also commits a floor checkpoint no less often
than once every `30 s`; it may coalesce with another state transition but, if it
does not, it creates a new immutable generation and readiness event. A checkpoint
more than `250 ms` late removes readiness and fails the cell. The state owner
advances `epoch_floor` only after a complete threshold/chain/time validation and
never changes the stored digest for the same number.

A crash can therefore lose up to `30 s` of uncommitted monotonic progress, not
merely a sub-second interval. It never lowers the last committed floor; after
restart no readiness is possible until the external `<=2 s` clock condition is
re-established and a new floor is committed. Every second during an active run,
the candidate reclassifies current state from the monotonic anchor while the
external collector checks clock offset; an offset breach removes qualifying
readiness and invalidates the evidence cell. The resulting additional checkpoint
events are included in the pre-run evidence-volume projection.

If the controlled-clock condition is absent, no new work starts. A forward jump
beyond `valid-until` must expire immediately; resetting the wall clock afterward
does not lower `trusted_time_floor`. Separate authenticated-time sources and
public source-family independence remain later work. Tests include offset
boundary `-2/0/+2 s`, larger rollback, forward jump, restart, pre-`valid-after`,
stale, and expired state.

### Persistence and crash consistency

H3-A uses one serialized immutable-generation protocol, not several
independently renamed security files and not a selected database:

Startup has exactly three cases. A completely absent configured state root under
the preflighted owned parent is `virgin` and may initialize only from packaged
genesis Epoch `1`. An existing root with a valid `current` pointer is normal. An
existing root that is empty, partial, or lacks a valid pointer is corrupt and
fails closed. Deleting the complete root can therefore impersonate virgin state;
that is part of the declared full-directory deletion/rollback non-protection,
not hidden protection.

1. one state owner serializes every active/pending Epoch, source order, attempted
   history, durable acquisition cycle/attempt/backoff, conflict, signer,
   epoch/time floors, exact active/pending envelope bytes and digests, resource state/profile, readiness sequence,
   status/reason/issued time, and verdict-input transition into one private,
   versioned, deterministic `state` blob no larger than `2560 KiB`;
2. its generation id is
   `SHA-256("ardents-h3-state-generation-v1\0" || state-bytes)`. The owner writes
   the **complete** blob into a new same-filesystem temporary generation
   directory, flushes the blob and directory, and renames the directory to the
   `64`-hex generation id;
3. it writes and flushes one new `current` pointer file, atomically renames that
   single pointer, and flushes the state-directory parent. The pointer contains
   exactly the `64`-hex generation id plus one LF;
4. only the generation named by `current` is active. A complete orphan created
   before the pointer commit is ignored and reported; outside the exact virgin
   case, a missing/corrupt pointer fails closed rather than selecting an arbitrary
   older generation;
5. every generation and the pointer are revalidated at startup. A transition
   may advance attempted-source history, `epoch_floor`, and `trusted_time_floor`
   but never lower or rewrite any of them.

After the pointer commit and before event emission, the owner durably creates one
content-addressed **verification capsule** on the same preflighted filesystem.
It contains the exact event bytes, a hard link to that immutable state blob, and
an exact copy of the committed pointer bytes. The state blob itself contains the
exact raw active/pending Epoch envelopes. Its identity is:

```text
capsule-digest = SHA-256("ardents-h3-verification-capsule-v1\0" ||
    uint32be(event-byte-length) || event-bytes ||
    uint32be(state-byte-length) || state-bytes ||
    uint32be(pointer-byte-length) || pointer-bytes)
```

The link keeps state bytes available even after the state directory unlinks an
old generation. The external collector copies and flushes the capsule, H
verifies it, and only a verification record binding the same `capsule_digest`
plus a digest-matching external spool acknowledgement allows local removal.

The spool reserves at most `16` capsules and `48 MiB`: `15` ordinary slots plus
one terminal slot. An ordinary transition reserves its capsule slot/bytes before
commit. If that reserve is unavailable, the terminal slot is used for one
`not_ready/evidence_failure` generation and new work stops; if the terminal
capsule itself cannot be committed/emitted, the `2 s` emergency fail-stop
applies. Capsule count/bytes and acknowledgement identity survive restart. No
unverified generation is deleted merely to recover space.

This makes the pointer the only atomic activation boundary. Parallel source
results cannot race because they submit transitions to the same state owner.
Corruption, disk full, permission loss, and crashes before/after every flush,
rename, and pointer commit have explicit non-ready or last-committed outcomes.
No secret, generated evidence, cache, or live capture is committed to the
repository.

The state directory is not an external monotonic anchor. Restoring a complete,
internally consistent older directory while its Epoch is still fresh may be
indistinguishable from legitimate old state. The harness must demonstrate and
record this limitation; H3-A claims only crash consistency and rejection of a
lower incoming/replayed Epoch while the committed epoch floor survives. Release
rollback protection belongs to H3-F.

If this cannot remain small and auditable, storage becomes its own research
question; H3-A does not silently add bbolt, SQLite, or distributed storage.

### Signer transition fixture

The `2-of-3` H3 policy exists only to test mechanics:

1. installed policy `P1` authorizes Epoch N;
2. an Epoch signed by `P1` commits the complete next policy `P2` and transition
   activation bound;
3. the first `P2` Epoch chains to the accepted transition;
4. old-only, new-too-early, mixed-below-threshold, unknown-policy, replayed, and
   rollback variants fail;
5. the signer operation is offline with respect to the H3 network and shares no
   distributor or route listener.

The transition rules are exact for this slice. A non-empty P2 has
`version = P1.version + 1`, threshold `2`, three unique keys distinct from the
three P1 fixture keys, and
`Epoch N.valid_after < P2.activate_at < Epoch N.fresh_until`. Once accepted Epoch
N commits P2, its direct successor must be exactly N+1, chain to N, reproduce P2
as its `signer_policy`, have `valid_after >= P2.activate_at`, carry only P2
signatures, and use `{}` for `next_signer_policy` in this single-transition
fixture. A P2 successor before `activate_at`, any P1-only successor after the
committed transition, mixed P1/P2 threshold counting, skipped version/number,
or a body policy different from the previously committed policy is invalid. P1
remains the verification policy for N itself; committing P2 never rewrites
already accepted bytes.

Three synthetic keys are not three custodians. Public `3-of-5` custody remains a
later external gate.

## H3-A terminal outcome

Readiness is an ordered event stream, not a mutable boolean or a timeless
receipt. For every committed security/readiness state generation, the state
owner increments one durable `readiness_sequence`, records the
status/reason/issued time in that immutable generation, commits it, and emits the
deterministic event bytes.
A crash after commit may re-emit the identical event; `(state_generation_digest,
readiness_sequence)` deduplicates it.

The event is canonical minified UTF-8 JSON, at most `4096` bytes, with no BOM,
unknown/duplicate/out-of-order field, insignificant whitespace, or trailing
byte. Fields occur exactly in this order:

```text
schema, sequence, status, reason, issued_at, valid_until,
network_identity, epoch_number, epoch_digest, candidate_fixture_digest,
candidate_fixture_count, state_generation_digest, epoch_floor_number,
epoch_floor_digest, trusted_time_floor, signer_policy_version,
retained_manifest_digest, sensitive_manifest_digest, resource_profile
```

`schema` is `ardents-h3-readiness-event-v1`; sequence/time/count/version values
use the canonical unsigned-integer syntax of the Epoch body; digests/network identity use
its exact `32`-byte lowercase-hex representation; and `resource_profile` is the
literal `h3-s-v1`. `status` is exactly `ready` or `not_ready`. A ready event uses
reason `fresh_normal` and `valid_until = Epoch.fresh_until`. A not-ready event
uses the first applicable reason in this exact precedence order:
`persistence_failure`, `evidence_failure`, `cleanup_failure`, `shutdown`,
`drain`, `protect`, `clock_uncertain`,
`conflicting`, `invalid`, `incompatible`, `expired`, `stale`, `staged`,
`no_state`; its `valid_until = issued_at`. When there is no active Epoch,
`epoch_number`, `signer_policy_version`, and `candidate_fixture_count` are zero
and the Epoch/fixture digests are all-zero; otherwise they describe the retained
active state even in a negative event. Sequence is in
`1..9223372036854775807` and never resets across restart.

The external event digest is:

```text
event-digest = SHA-256("ardents-h3-readiness-event-v1\0" || event-bytes)
```

Only a generation whose active Epoch is fresh and whose resource state is
`NORMAL` emits `ready`. A restart that re-establishes readiness, any new active
Epoch, every committed acquisition/security transition, and every
readiness-affecting resource transition emit the next event. A later generation
may remain `ready` when the active state is unchanged, but `PROTECT`,
stale/expired time, conflict, shutdown, or cleanup failure must emit
`not_ready`. The highest sequence is the only current external status; earlier
positive events remain historical evidence and never override it.

An invalid, unavailable, or timed-out refresh candidate does not relabel a
still-fresh active Epoch; its committed attempt transition may emit another
`ready/fresh_normal` event while retaining the exact failure as evidence. Only a
condition that invalidates overall active readiness selects a negative reason.

Even the highest event is necessary but not sufficient for **current external
readiness**. At observation time all of these must hold together: its matching H
record says `verified_ready`; event sequences since the last verified event are
contiguous; the live readable `current` pointer still names the event's exact
state generation; the process and cgroup still exist under `h3-s-v1`; the latest
external service-liveness sample is no older than `500 ms`; current external
clock/resource observations remain qualified and `NORMAL`; and trusted time is
before `valid_until`. Local collectors sample process/service/cgroup liveness at
least every `250 ms`. Missing/unreadable/newer/unverified state, a dead process,
a stale observation, or any failed conjunct is not ready.

When persistence, evidence, or cleanup fails but a negative generation remains
committable, E commits and emits its exact negative reason. If either durable
commit or emission is impossible, E synchronously removes in-memory readiness,
closes new-work admission and listeners, and enters the R-028 emergency fail-stop:
voluntary exit or supervisor termination completes within `2 s`. H then observes
the dead service no later than the next `500 ms` liveness limit. An old positive
artifact can consequently appear externally current for at most `2.5 s`, but E
accepts no new work during that bounded interval. This is an explicit H3-A
limitation, not a claim of instantaneous revocation.

For every event, H runs a verifier mode in a separate process. Its inputs are
the exact durable verification capsule (event bytes, immutable state blob with
the raw Epoch envelope, and committed pointer snapshot), retained manifest,
access-controlled sensitive manifest, and
the external clock/resource samples immediately bracketing `issued_at`. Both
samples must identify the same process/cgroup/profile, be at most `2 s` from the
event, and satisfy the R-028 clock/resource rules. H obtains the content-addressed
artifacts from the collector and never consumes E's readiness boolean.

The verifier emits canonical JSON `ardents-h3-verification-v1`, also capped at
`4096` bytes, with fields in this order:

```text
schema, event_digest, capsule_digest, verdict, reason, verified_at, epoch_digest,
state_generation_digest, before_sample_digest, after_sample_digest,
retained_manifest_digest, sensitive_manifest_digest
```

`verdict` is exactly `verified_ready`, `verified_not_ready`, or
`verification_failed`. A positive cell requires `verified_ready`; missing,
non-bracketing, mismatched, stale, or over-limit evidence is
`verification_failed`. `capsule_digest` is the lowercase-hex encoding of the
exact digest above and H recomputes it before any state/event check. Reusing
side-effect-free parser/hash/signature functions
is allowed; the separation proves recomputability without trusting E's boolean,
not implementation, auditor, operator, or operator-family independence.
Candidate processes cannot reach the management plane, alter the collector, or
declare H's verdict.

Verifier reason is exactly one of `match`, `missing_event`, `sequence_gap`,
`capsule_invalid`, `event_invalid`, `epoch_invalid`, `state_mismatch`, `manifest_mismatch`,
`clock_unqualified`, `resource_unqualified`, `persistence_unavailable`,
`sample_gap`, or `expired`. The record digest and external-sample digest are:

```text
verification-record-digest =
    SHA-256("ardents-h3-verification-v1\0" || verification-record-bytes)
external-sample-digest =
    SHA-256("ardents-h3-external-sample-v1\0" || canonical-sample-bytes)
```

The pre-run evidence schema freezes the canonical sample fields/order before any
candidate starts.

One campaign may contain many ordered event/verification pairs. Its terminal
canonical JSON record is at most `4096` bytes and has fields in this exact order:

```text
schema, retained_manifest_digest, sensitive_manifest_digest,
first_sequence, last_sequence, pair_count, pairs_digest, evidence_root,
machine_result
```

`schema` is `ardents-h3-campaign-verdict-v1`; `machine_result` is exactly `pass`,
`fail`, or `invalid`; and the other digest/integer encodings follow the event
rules. `pairs_digest` is
`SHA-256("ardents-h3-event-verification-pairs-v1\0" || pairs)`, where `pairs` is
the sequence-ordered concatenation of `uint64be(sequence) || 32-byte
event-digest || 32-byte verification-record-digest`. The evidence root and
ordered pair count are recomputed before the campaign record is accepted.
`pair_count` may be zero only when the campaign reaches a pre-first-event
terminal result. It can be `invalid` for a harness/preflight failure or `fail`
for candidate behavior such as a crash or security violation; the ordinary
result rules below decide which. In either case `first_sequence = 0`,
`last_sequence = 0`, and `pairs_digest =
SHA-256("ardents-h3-event-verification-pairs-v1\0")`. Otherwise `pair_count >= 1`,
the first/last values equal the bounds of the contiguous ordered pairs, and both
sequence values follow the event range.
`evidence_root` is
`SHA-256("ardents-h3-evidence-root-v1\0" || entries)`, where `entries` excludes
the terminal campaign record and is the ASCII-path-ordered concatenation of
`uint32be(path-byte-length) || path-bytes || uint64be(file-byte-length) ||
SHA-256(file-bytes)` for every retained evidence file. It is distinct from every
readiness event.

The calculator returns `invalid` when the candidate cannot be judged because a
preflight, generator timing, impairment, observer budget, sample, manifest, or
evidence-integrity gate failed. It returns `fail` when the run is valid and
complete but any candidate security, expected fault behavior, performance,
resource, shutdown, privacy, or cleanup conjunct fails. It returns `pass` only
when every required applicable cell and campaign conjunct passes and no
invalidation exists. An injected fault cell passes only through its predeclared
protective oracle, never by ordinary success.

Every percentile, mean, ratio, missing-sample, probe-window, and rounding
decision uses the common deterministic H3-A calculus in R-028; no Route
Qualification or implementation-default statistic is inherited silently.

`advance`, `redesign`, and `stop` are subsequent Product Owner dispositions, not
machine verdicts. A machine `pass` is necessary but does not itself promote
H3-B; `invalid` reruns the same frozen question after a harness repair; `fail`
requires Product Owner choice between bounded H3-A redesign and stopping this
direction.

This is an observable bootstrap result, not Common Readiness, Target Connect
Readiness, Route Qualification, or proof of a product Candidate View. H3-B owns
the first runtime consumer, persistent Node processes, and a Named
Site/Application Data tracer; its research must first resolve the transition
from this fixture to canonical View/materialization semantics. H3-A neither
imports nor modifies H2 route, OHTTP, Name, Target, or credential behavior.

## Module and deployment seams

The implementation should have one deep H3-A lab Module behind a thin command.
It owns Epoch acceptance, finite source sequencing, crash-safe local state,
scenario orchestration, fault injection, evidence, and deterministic verdict as
one cohesive experiment. Source delivery, clock, persistence, resource
observation, and external-evidence boundaries are replaceable Adapters hidden by
that Module.

The H3-A Module does not import or modify `namedsite`, `nativecircuit`, or their
OHTTP/route closure. It starts no H2 command and gains no hidden Docker,
application-data, or control-plane path merely to make the slice look vertical.

Do not create separate packages merely for types, interfaces, utilities,
configuration, metrics, or storage. Exact package and command names are chosen
only when the agent adds real implementation, tests, a non-test caller, and the
package-map entry in the same change. The resource governor begins as an
unexported part of the H3-A Module; it becomes a separate Module only after a
second real role and independently coherent Interface exist.

Pre-provisioned Ubuntu plus systemd/cgroup v2 is the first multi-host fixture
candidate because it exposes real process, socket, restart, and resource
boundaries without adding a cluster control plane. Docker Compose remains
acceptable for non-qualifying local reproduction. Neither is a production
deployment decision.

## Dependency candidates

| Need | H3-A candidate | Decision boundary |
|---|---|---|
| Hash and signatures | `crypto/sha256`, `crypto/ed25519` | Ordinary independent test signatures only; no custom primitive or aggregate threshold scheme. |
| Cancellation/deadlines | `context`, `time`, `net` | Standard library first; every operation has an owner and terminal deadline. |
| Source authentication | `crypto/tls`, `crypto/x509`; lab-only mutual TLS 1.3 over TCP with installed roots and leaf-key pins | Static literal endpoints, separate transport/signer keys, no DNS or production wire selection. |
| Encoding | strict lab-only exact-byte envelope | Replaceable Adapter; no public format selection. |
| Persistence | serialized immutable generations plus one atomic pointer | No database until evidence shows a real transaction/query need. |
| Logging | `log/slog` with fixed low-cardinality fields | No Name, Target, route, IP, key, or source string in metric labels. |
| Rate/concurrency | bounded standard-library counters/channels | Evaluate `golang.org/x/time/rate` or `x/sync/semaphore` only if a recorded dependency review shows the simple owned implementation is less safe. |
| Resource scopes | owned H3-A reservations | go-libp2p Resource Manager is a design reference only; it is not selected without libp2p. |
| Peer/network discovery | none | `LATEST_OBSERVED` queries only the installed finite sources; no DHT, mDNS, DNS discovery, default peers, AutoNAT, relay, or hole punching. |
| Orchestration | pre-provisioned systemd/cgroup-v2 fixture | No Kubernetes, Nomad, or production installer/updater. |

If libp2p is later compared, every default bootstrap peer, DHT, mDNS, relay,
AutoNAT, hole punching, listener, and metric surface must be explicitly off
unless that separate experiment selects it. A DHT may eventually expand contacts
after Epoch acceptance; it can never define initial truth.

## Evidence plan

### Primary sources

Accessed 2026-08-11:

- [Tor directory architecture and validity](https://spec.torproject.org/dir-spec/outline.html);
- [Tor client source selection and retry](https://spec.torproject.org/dir-spec/client-operation.html);
- [Tor persistent bounded guards](https://spec.torproject.org/guard-spec/algorithm.html);
- [I2P network database](https://beta.i2p.net/en/docs/overview/network-database/);
- [libp2p Kademlia specification](https://github.com/libp2p/specs/blob/master/kad-dht/README.md);
- [go-libp2p resource manager](https://pkg.go.dev/github.com/libp2p/go-libp2p/p2p/host/resource-manager).

### Pre-run manifests

The pre-code evidence contract is frozen by this record and R-028: required
contents, owners, cadence, bounds, privacy split, digest/root formulas, gates,
and result calculus may not change during implementation. Exact scalar fields,
field order, and golden bytes for the two strict canonical minified-JSON
manifests and external sample records are a lab serialization detail, not a
product protocol decision. The implementation candidate must freeze and test
that serialization before its first candidate scenario; changing it creates a
new candidate identity and invalidates earlier evidence.

The sensitive manifest is finalized first; its digest is included in the
retained manifest. Neither manifest contains its own digest:

```text
sensitive-manifest-digest =
    SHA-256("ardents-h3-sensitive-manifest-v1\0" || sensitive-manifest-bytes)
retained-manifest-digest =
    SHA-256("ardents-h3-retained-manifest-v1\0" || retained-manifest-bytes)
```

The **retained reproducibility manifest** contains:

- source tree, binary, Go toolchain, dependencies, host image/kernel, and
  supervisor versions;
- synthetic host/role identifiers, CPU/RAM/link limits, cgroup contract, OS
  limits, and clock-preflight result;
- test network identity, public key digests, signer policy, Epoch bytes/digests,
  synthetic candidate fixture, source/role identifiers and family markers, and
  validity bounds;
- exact source permutation seed, fault schedule, attempt order, deadlines,
  resource profile, readiness-event/verification/campaign schemas,
  request/response artifact digests,
  and expected result classes;
- evidence schema, collector, calculator, and cleanup versions.

The separately access-controlled **sensitive execution manifest** binds those
synthetic identifiers to real host addresses, source endpoints, the controlled
clock-control details, local paths, cgroup ancestry, and credential identifiers
or paths, never private-key bytes. Its digest is recorded in
the retained manifest, but its contents are not copied into the ordinary
evidence bundle. Reproduction and verification of real topology, clock, cgroup,
path, and credential bindings require both layers and therefore remain possible
only for the owner or a reviewer explicitly given sensitive access. The retained
layer and redacted samples let another reviewer recompute the published
calculator over disclosed inputs and verify their hashes, but do not independently
attest the hidden execution mapping. H3-A makes no independent-audit claim.

The manifest cannot be rerolled after a result. Confirmed harness failures retain
their evidence and a written invalidation reason.

### Verification matrix

Every case has a predeclared terminal result:

1. valid installed/package candidate;
2. valid last-known-good routine restart;
3. each authenticated mirror as the only available `LATEST_OBSERVED` and
   `BY_DIGEST` source;
4. explicit offline-file import;
5. unavailable, slow, truncated, oversized, malformed, extra-field, and
   selective-withholding source;
6. wrong network, incompatible profile/protocol, bad digest, bad signature, and
   below-threshold envelope;
7. number below the surviving durable epoch floor, broken previous digest,
   equal/reversed Epoch or assignment
   boundaries, duplicate identity/endpoint handle, source-role overlap,
   pre-`valid-after`, stale, expired, and clock-uncertain state;
8. fast valid A plus delayed same-number conflicting B with no readiness before
   the barrier; separate unreachable-honest-source N/N+1 withholding records
   `latest completeness unproven` rather than claiming detection;
9. old/new signer transition before, during, and after activation;
10. pre-contact collision with retained candidate-fixture state plus authenticated
    source declaration mismatch in identity, family, handle, or transport-key
    digest;
11. complete source exhaustion with no DNS/direct/old-topology fallback;
12. corrupt cache, corrupt epoch/time floor, disk full, permission loss, and crash at
    each persistence boundary;
13. process/host restart after an attempt is durably `started`, proving no redial
    in that cycle and persisted exponential backoff;
14. active fresh N plus future N+1 pending, restart before/after `valid-after`,
    activation with no staging-created readiness gap, and no new work after
    `fresh-until`;
15. exact ordered readiness events after durable state transitions, separate
    harness recomputation, negative/ineligible transitions, and the terminal
    campaign verdict;
16. restoration of a complete older state directory, recorded as an unprotected
    limitation rather than false rollback success;
17. bounded shutdown and complete owned-resource cleanup.

### Repetition, latency, and soak

- `100` clean-start attempts and `100` routine-restart attempts;
- at least `99%` successful attempts in the valid normal cell;
- clean start to `H3-A Epoch Ready (unqualified)` `p95 <= 15 s`;
- routine restart to `H3-A Epoch Ready (unqualified)` `p95 <= 5 s`;
- failures count as infinite latency and are never deleted or rerolled;
- every security-negative case must reject with zero false acceptance;
- the exact R-028 distribution of `100` process/host restart cycles retains
  epoch/time floors, attempted-source history, acquisition attempts/backoff, and state;
- one `72 h` persistent soak with scheduled Epoch refresh and separate
  readiness-event verification throughout its declared campaign;
- before H3-B promotion, one independent `7 day` unattended campaign under the
  same candidate binary, protocol/resource contract, packaged genesis, and
  pre-generated corpus;
- the two soaks have distinct retained and sensitive manifests, freshly
  provisioned virgin candidate-state roots, separate evidence roots, and
  separate machine verdicts. They may reuse immutable corpus bytes but never
  delete, rewind, or restore the active root of one campaign to start the other;
- the `7 day` campaign must start no later than `7 days` after the `72 h`
  campaign ends. At corpus creation and immediately before each campaign, the
  manifest calculator proves for every scheduled Epoch activation that
  `valid_after <= scheduled_time < fresh_until < valid_until`, including the
  complete planned campaign end. If the start window is missed or any signed
  bound would fail, generate a new corpus/candidate identity and repeat both
  campaigns from `72 h`; signed absolute Unix times are never shifted or
  reinterpreted;
- any candidate-binary or frozen-contract change invalidates both soak results
  and restarts the sequence at `72 h`; a harness-only repair changes the harness
  identity and reruns every affected campaign in full;
- no unexplained growth in RSS, live heap, FD, sockets, goroutines, timers,
  queues, cache, evidence, or logs; exact limits come from R-028.

These are H3 evidence thresholds, not Route Qualification or a public SLO.

### Evidence bundle

Evidence is generated outside the repository and contains:

- immutable retained pre-run manifest, sensitive-manifest digest, and candidate
  identity; the sensitive manifest itself remains access-controlled;
- exact Epoch/envelope digests and public test-key identities, never private
  keys;
- every source attempt, validation transition, readiness-event transition,
  independent-verification result, restart, pressure transition, shutdown phase,
  and cleanup result;
- raw one-second external resource samples plus selected Go runtime samples;
- all failures, timeouts, cancellations, invalidations, and terminal results;
- deterministic `pass`/`fail`/`invalid` machine verdict and a short human
  explanation;
- a privacy review proving no Service Name, Target, route history, Entry set,
  Application Data, persistent IP graph, or secret enters the retained bundle.

The deterministic calculator must reproduce the campaign's conjunctive
`pass`/`fail`/`invalid` result. Product Owner `advance`/`redesign`/`stop`
disposition is recorded separately. A selected-success report is not evidence.

## Narrow points and risks

| Risk | Required response |
|---|---|
| H3-A becomes a production Control Plane. | Keep encoding, source transport, storage, and topology behind lab-only Adapters; no public compatibility promise. |
| A mirror personalizes the view. | Exact signatures prevent an untrusted mirror from altering bytes; observing two authorized forks creates conflict, while an unseen threshold-authorized fork remains an explicit limitation. |
| Source diversity is mistaken for anonymity. | Retain conservative H3 Attempted Source History and state both origin observation and missing release-lifecycle semantics in every verdict. |
| Synthetic keys are called decentralized. | Mark one real project family; threshold mechanics and independent custody are separate claims. |
| Wrong wall clock silently extends trust. | Controlled time preflight plus separate epoch/time floors and monotonic time; insufficient confidence blocks readiness. |
| Retry creates unbounded contact/exposure. | Persist cycle/attempt/backoff before contact; a started attempt is consumed across restart. |
| Process crash or partial write loses the last committed state. | Single-owner immutable generations, one atomic pointer, startup revalidation, and crash-boundary tests. |
| The complete state directory is restored to an internally valid older copy. | Record this as an explicit H3-A non-protection; only surviving epoch/time floors reject lower replay. External release rollback protection belongs to H3-F. |
| The synthetic candidate fixture is mistaken for the product Candidate View or scale evidence. | Cap it at `64` records, use a distinct field/domain, and state that input-log cutoff, global summaries, proofs, materialization, and auditors are absent. |
| A positive readiness event is mistaken for network success. | Name it only `H3-A Epoch Ready (unqualified)`, require the external verification/current-state conjunction, and leave canonical View semantics, Node lifecycle, Route, and Named Site evidence to H3-B research. |
| Resource control is self-reported only. | R-028 requires external cgroup/process-tree evidence and host-level limits. |
| systemd or the current lab fixture becomes production lock-in. | Treat it as a replaceable test Adapter and make no protocol/deployment ADR. |

## Falsification and stop conditions

Stop H3-A and redesign before H3-B if any one occurs:

- unsigned, below-threshold, tampered, lower-than-surviving-epoch-floor, expired,
  wrong-network, incompatible, broken-chain, or observed-conflicting state
  becomes ready;
- a distributor without threshold authorization changes accepted experimental
  Epoch/body/candidate-fixture bytes, or
  two observed same-number valid digests are not made conflicting;
- a source or signer must stay online permanently for an already-valid routine
  restart within its finite validity window;
- source attempts, H3 Attempted Source History, goroutines, timers, sockets, queues,
  cache, disk, or logs grow without a precommitted bound;
- an installed source identity/family/handle passes the candidate-fixture collision
  check or is silently accepted after a newly authenticated body exposes it;
- restart loses or lowers attempted-source history, cycle attempts/backoff,
  active/pending state, conflict, signer-policy, epoch floor, or trusted-time floor;
- stale/uncertain state starts new work or live work exceeds its terminal bound;
- complete source exhaustion falls back to DNS, direct reachability, a different
  network, or the old preconfigured topology;
- experimental Epoch readiness becomes true before durable activation or
  remains true during
  conflict, PROTECT, DRAIN, or cleanup failure;
- H's recomputed readiness verdict differs from E's event, a sequence gap is
  unexplained, or an ineligible/live-unverifiable state remains externally ready;
- the slice requires custom cryptography, a DHT, distributed consensus, a
  database, a cluster orchestrator, or a production wire foundation to pass;
- evidence cannot recompute the verdict or leaks protected product metadata;
- synthetic participants are presented as independent operators, custodians,
  sources, or auditors;
- the R-028 resource/soak/shutdown contract fails.

## Historical recommendation (superseded)

Choose H1 with medium-high confidence: make H3-A the next and only recommended
implementation slice. Its strongest counterargument is that a small synthetic
candidate fixture and controlled clock do not prove public bootstrap scalability or
hostile clean-time recovery. That limitation is acceptable only because H3-A
withholds those claims and preserves replaceable View/time Adapters.

After complete valid H3-A evidence and its machine verdict, the Product Owner
records a separate disposition:

- `advance` permits research and explicit Product Owner consideration of H3-B;
- `redesign` keeps H3-A scope but changes a recorded algorithm/Adapter and reruns
  the complete affected matrix;
- `stop` returns the project to Control Plane/product design without compensating
  scope in Node lifecycle, Bridges, naming, updates, or UI.

## Disposition

- Question state: `decided` only as an appendix to accepted R-029.
- Standalone recommendation: superseded and withdrawn. H3-A0..H3-A6 is
  historical and is not an implementation order.
- Retained decision material: the bounded source plan, Epoch verification,
  freshness, conflict, persistence, and evidence mechanics explicitly adopted
  by R-029 and the Stage 1 brief.
- Follow-up order is controlled by the accepted H3 technical design; every
  later stage still requires its own research record and Product Owner decision.
- ADR: none. This applies accepted staging and Control Plane decisions without
  selecting a hard-to-reverse production foundation.
- Code: none created by this research record.
