---
id: R-141
title: Can a Publisher be replaced before its Credential expires without two current Instances?
status: decided; C0 retains expiry-gated successor pending a separately selected Authority-currentness topology
owner: Product Owner and Codex
started: 2026-09-05
---

# R-141 — Publisher early supersession

## Decision this unlocks

Determine whether the selected recovery direction can replace a stopped,
lost, or compromised Publisher with an early higher-generation Instance for
the same Target. The result must either specify one fail-closed supersession
contract suitable for an ADR and later vertical slices, or retain expiry-gated
replacement as an explicit C0 limitation.

This question does not authorize implementation, a Credential/Publication/
Descriptor wire change, a new dependency, Authority password automation, or an
availability claim. The Product Owner selected online Authority-signed
currentness as the direction under evaluation. The completed review finds that
it requires an unselected Authority-currentness topology; it is not a
maintained contract until that topology and a superseding ADR are accepted.

## Current contract

ADR-0003 permits a learned supersession to stop Service Connection work, but
explicitly does not select instant revocation. ADR-0064 and ADR-0065 instead
make consumed or ambiguous Publisher generations terminal: restart requires a
fresh Instance and higher Credential. The current Service Authority forbids
overlapping Credential validity; the Gateway retains one highest generation and
also rejects an overlapping higher Credential.

`publication.Decode` and `reachability.Verify` accept an otherwise valid
Credential at their decision time. They do not obtain an Authority-signed
currentness or revocation fact. A previously acquired valid Descriptor can
therefore remain locally verifiable until its own and its Credential's expiry.
The current Gateway is an availability/privacy participant, not a Service
Authority.

The current custody Interface is a local encrypted-Vault operation. It has no
listener, State-selected endpoint, authenticated Authority transport, or
Gateway-to-Authority discovery fact. The only State projection for this lookup
is `DestinationResolutionGateway`; it authenticates the Gateway profile and
does not identify a per-Target Authority responder.

## Hypotheses

- **H1:** A bounded Authority-signed supersession fact, durably sequenced with
  the successor Credential and checked by every publication, descriptor, and
  connection-acceptance path, can make a new Instance current while every
  unconfirmed status path returns `unavailable`.
- **H2:** A Gateway-local early-generation replacement suffices if it stores a
  terminal predecessor marker before exposing the successor.
- **H0:** With an offline-verifiable old Credential or Descriptor, neither H1
  nor H2 can prevent a partitioned or cached verifier from accepting old
  authority. The candidate must then retain expiry gating or explicitly select
  an always-online currentness authority and its availability/trust cost.

## Evaluation criteria

- After any successful successor decision, an old Instance cannot establish a
  new accepted Service Connection through any permitted verifier path.
- A Gateway, client, or host that cannot establish currentness returns a typed
  unavailable result; it never uses a stale `good` assertion as a fallback.
- A copied old root, replayed successor artifact, lower generation, and
  rollbacked durable root cannot reactivate Instance authority.
- At most one Instance is current for the exact Target at every accepted
  decision point. The contract states its partition semantics rather than
  calling eventual convergence immediate revocation.
- The Authority's password and root material never enter arguments,
  environment, configuration, Endpoint, Gateway, or diagnostics. If the
  selected signer is online, its only online Interface is the bounded
  currentness operation specified below.
- The chosen Module Interface keeps supersession ownership local: callers do
  not assemble revocation state or independently rank generations.
- A migration/no-migration answer exists for already consumed roots and for
  public records/descriptors that predate the new contract.

## Evidence plan

### Primary sources

- ADR-0003, ADR-0036, ADR-0064, ADR-0065, the current private-reachability
  and Endpoint/Service-runtime owners, inspected 2026-09-05.
- [RFC 5280](https://www.rfc-editor.org/rfc/rfc5280), inspected 2026-09-05,
  for certificate validity and revocation-list processing.
- [RFC 6960](https://www.rfc-editor.org/rfc/rfc6960), inspected 2026-09-05,
  for signed online status, its validity interval, and the need for a relying
  party to contact a status responder.
- Fischer, Lynch, and Paterson, [*Impossibility of Distributed Consensus with
  One Faulty Process*](https://groups.csail.mit.edu/tds/papers/Lynch/pods83-flp.pdf),
  inspected 2026-09-05, for the asynchronous failure/termination trade-off.

### Repository measurement

Trace the real Publisher start and the exact verifier/connection paths with
test keys and controlled time. Construct an old valid Descriptor, an early
successor candidate, a stale Gateway record, and a client that learned the old
Descriptor before supersession. The experiment must demonstrate either refusal
at every path or the exact path that remains able to use old authority.

The external Go-overlay diagnostic at
`C:/Users/vitek/AppData/Local/Temp/ardents-publisher-early-supersession-diagnostic`
ran on commit `7b277dce5496a8057d07295ed762b28da84e9bc7` with:

```text
go test -overlay C:\Users\vitek\AppData\Local\Temp\ardents-publisher-early-supersession-diagnostic\overlay.json ./internal/service/reachability -run '^TestEarlySupersessionDiagnostic$' -count=1 -v
```

It exited `0`. With test-only keys it manually constructed a signed overlapping
successor solely to expose verifier behavior; it did not bypass or redefine
Custody issuance. At `T+2m`, `reachability.Verify` accepted both generation 1
and the overlapping generation 2 Descriptor. The same early successor was
rejected by `reachability.Store.Publish` as `StoreInvalid` because its
Credential interval overlaps. The log contains no secret material.

### Falsification cases

- old and successor Instances both obtain accepted currentness or new work;
- a copied or rolled-back root signs after a successor;
- an old Descriptor or Credential enables a new connection after a successor;
- a partition makes a verifier silently prefer old evidence rather than return
  unavailable;
- a recovery procedure changes Target, exports a secret, requires an
  unrecorded manual repair, or depends on a hidden online operator.

## Findings

- **Measurement (2026-09-05):** the diagnostic above proves that the current
  closed Descriptor verifier has no Gateway-store or Authority-currentness
  input. Its acceptance is exactly signature, Target/Network, Credential-time,
  digest, slot-time, and Instance-signature verification.
- **Measurement (2026-09-05):** `internal/route/user_attachment.go` passes the
  descriptor returned from the fresh private lookup to `reachability.Verify`,
  and `internal/route/user_introduction.go` repeats `publication.Decode`.
  Neither Interface carries a supersession decision from the Gateway.
- **Measurement (2026-09-05):** the Gateway Store independently enforces the
  current non-overlap rule, but that local fact cannot invalidate a Descriptor
  already held outside the Store.
- **Measurement (2026-09-05):** `internal/custody` exposes only local Vault
  operations, while `internal/network/state.ResolutionView` exposes only the
  State-selected `DestinationResolutionGateway`. Repository search found no
  Authority listener, Authority endpoint, State Authority-responder projection,
  or authenticated Gateway-to-Authority carrier. The existing Gateway's
  configuration explicitly excludes Publisher origin and Service private
  material.
- **Inference:** H2 is false. Changing only Gateway publication/lookup state
  cannot establish that no permitted verifier accepts old authority after an
  early successor exists.
- **Sourced fact:** RFC 6960 models revocation as a separate signed status
  assertion with its own validity interval. A relying party that requires fresh
  status contacts the responder; certificate validity alone does not encode a
  later revocation.
- **Inference:** H1 requires every currentness-sensitive acceptance path to
  receive a fresh, authenticated status decision, or requires a prior finite
  old-descriptor quiescence interval. The former adds an online authority and
  explicit unavailable-under-partition behavior; the latter cannot be an
  immediate revocation and must bound all previously issued Descriptor lives.
- **Limitation:** FLP is not a substitute for a protocol proof here. It
  establishes why a claim of both definitive distributed handover and guaranteed
  termination through arbitrary asynchronous crash/partition needs additional
  timing or authority assumptions. The exact Ardents assumption remains to be
  selected.

## Candidate designs

1. **Authority-signed status-gated supersession.** The online Authority signs
   a fresh nonce-bound currentness result over its durable monotonic record;
   the result names one Credential generation/digest and its finite decision
   time. All publication, Descriptor, and connection-acceptance paths require
   that exact result. Unknown/freshness failure is unavailable. This can meet
   H1, but makes the Authority online for currentness and requires a versioned
   private response contract, partition tests, and a selected authenticated
   Authority-currentness topology. The current product has none of the latter;
   a Gateway cannot invent it without becoming an unselected discovery and
   authority path.
2. **Gateway-only early replacement.** The Gateway stores the successor before
   resolving it and suppresses its old descriptor. This is insufficient if a
   client can use an old Descriptor or offline Publication verifier without a
   fresh status decision; it is retained to test H2 rather than assumed safe.
3. **Expiry-gated successor.** Preserve current non-overlap. It is the known
   safe baseline but does not satisfy the requested ordinary-restart recovery.

## Online-status trust analysis

A delegated Gateway or separate response signer cannot preserve the current
Gateway limitation by itself. After rollback or compromise it can bind a fresh
request nonce to an old still-valid Authority-signed Credential, while a new
Endpoint has no independent fact that a later successor exists. Requiring that
delegate to retain a floor is an implementation control, not a cryptographic
proof against the compromised signer.

The only candidate that retains the present statement that a Gateway cannot
make an old generation current is therefore an **online Authority signer**. It
must use the Authority's already durable monotonic record and sign exactly one
fresh result for the requester nonce, Target, Network, current generation and
Publication digest, decision time, and finite expiry. A signed predecessor
result must expire no later than the Authority's scheduled successor-effective
time; otherwise it can cross the handover boundary by replay.

This changes the security and operational contract:

| Topic | Required result |
| --- | --- |
| Authority custody | The Authority root is online while responding. Interactive unlock remains local; no password appears in arguments, environment, configuration, or logs. |
| Gateway | It forwards opaque lookup work and may withhold it, but cannot manufacture a currentness result or choose a generation. |
| Endpoint and Route | They accept a Descriptor only with a fresh Authority result bound to the same private-lookup nonce; a cached Descriptor or status result alone is unavailable. |
| Handover | Custody durably records the successor and predecessor termination before responding. The Authority starts returning the successor only at an effective time after every older status result expires. The interval before that time is explicitly unavailable, not old-current fallback. |
| Existing work | A Service Connection established before the effective time may drain only within its existing finite safety bounds. New lookup, connection, or recovery work after the effective time requires successor status. |
| Partition | Authority, Gateway, or status freshness failure is `unavailable`; no peer, direct Publisher, cache, or old proof is a fallback. |
| Compromise | Online Authority compromise remains Target compromise, as for the existing Authority key. Gateway compromise remains withholding/traffic-metadata risk, not successor minting. |
| Gateway replacement | A newly State-selected Gateway does not need a copied per-Target floor: it forwards to the same Authority currentness owner. |

The Authority-status Module should expose one deep Interface: given an opaque
private lookup request already bound to Target/Network/nonce/deadline, return
one signed currentness outcome. Custody owns its state and transition ordering;
Endpoint, Gateway, and Route do not assemble records or compare generations.
An in-memory test Adapter with controlled time is sufficient for behavior
tests; it is not a second Authority implementation.

## Unselected topology precondition

A nonce-bound result cannot be precomputed or safely cached by the Gateway.
For every lookup, the Gateway therefore needs an authenticated way to deliver
the exact Target/nonce/deadline request to the Authority and return its signed
answer. Current private reachability has only:

```text
Endpoint -> Entry -> Initiator -> State-selected Gateway
Publisher -> authenticated descriptor publication -> Gateway
```

There is no Authority responder in that graph. Adding one would require a new
answer to all of the following, none of which this card is authorized to
choose: the Authority responder's reachability/discovery without a direct
Publisher source; its carrier and authentication; Target-to-responder mapping;
Authority unlock and key exposure while online; responder availability and
recovery; and the disclosure introduced when the Gateway asks it about a
Target. Treating the existing Gateway as that responder would make the Gateway
an Authority or give it an Authority signing key, contradicting ADR-0036 and
the threat model.

## Product Owner direction and proposed contract

On 2026-09-05 the Product Owner confirmed the narrower consequence of option
1: if selected, the Service Authority becomes an online signer for every
private reachability lookup, and its unavailability blocks new Publisher
connection, publication acquisition, and recovery work. The missing topology
above means this is a rejected C0 candidate, not a proposed ADR decision.

The proposed closed v2 currentness response is Authority-signed and names:

- protocol/version domain, Network ID, Target, request nonce, and the exact
  lookup deadline;
- one Credential generation and digest, one Publication/Descriptor digest, and
  the signed result class;
- Authority decision time and an expiry no later than the request deadline;
- the Authority public identity that already authenticates the Target's
  Credential chain.

The Endpoint and Route accept a Descriptor only when this exact signed result
is fresh, has the request's nonce/deadline/Target/Network bindings, names the
same Credential and Descriptor digests, and verifies under that Target's
Authority. No cached Descriptor, cached status, Gateway Store class, direct
Publisher path, or valid predecessor Credential is a fallback. This requires a
new closed private-reachability response version: v1 contains no status field
or Authority signature and cannot safely carry this assertion.

The Authority-status Module has one external Interface: resolve one bounded
request to `current(descriptor, proof)`, `unavailable`, or `conflicting`.
Its implementation owns all of the following internal state: the monotonic
generation watermark, an immutable activated descriptor digest for each
generation, an optional pre-effective successor, the predecessor's last status
expiry, and a non-decreasing security-time watermark. Gateway transports the
opaque request/response and may store or withhold a descriptor, but it cannot
select a generation or produce a proof. Route and Endpoint consume one result;
they neither rank generations nor reconstruct Authority state.

### Handover and persistence rules

1. Custody may issue a higher successor Credential before predecessor expiry,
   but issuance alone makes neither descriptor current. The successor remains
   unpublished to currentness until it has a valid, exact ready publication
   transcript.
2. The Authority validates that transcript and durably records the successor's
   exact Credential and Descriptor digests before choosing an effective time.
   It chooses that time strictly after the maximum possible predecessor-status
   lifetime (the current lookup limit is 15 seconds) and its authenticated-time
   uncertainty bound. If that bound is unavailable, the operation is
   unavailable.
3. Before the effective time, every predecessor proof expires no later than
   that time and no successor proof is emitted. At or after it, the Authority
   emits only the successor proof. A scheduled transition can be cancelled only
   before emitting any successor proof; once effective, its predecessor is
   terminal and cannot be reactivated.
4. Gateway publication of the successor is not an Authority transition. If the
   successor descriptor is absent, malformed, stale, or its live slot fails,
   the result is unavailable; the Authority never revives the predecessor as a
   liveness fallback.
5. Every state-changing write is durable before its externally visible result.
   On reopen, malformed, rolled-back, missing, or contradictory currentness
   state is unavailable. Existing pre-v2 publications have no activation
   record and must be explicitly republished under a fresh successor; they are
   not silently upgraded to current.

| Interruption point | Required recovery result |
| --- | --- |
| Successor Credential issued, no ready transcript | Predecessor remains current; successor is unavailable. |
| Ready transcript received, activation write not committed | Reopen retains the predecessor only; successor has no currentness proof. |
| Activation committed, before effective time | Reopen preserves the scheduled transition and caps predecessor proofs at the same effective time. |
| Effective time reached, Gateway/slot absent or successor process lost | Unavailable; never predecessor fallback. |
| Authority, Gateway, route, time-confidence, proof, or durable-state failure | Unavailable; no stale-good or direct fallback. |
| Authority durable rollback or copied predecessor root after effective time | Fail closed unless the monotonic currentness state is proven consistent; an old generation cannot receive a fresh proof. |

The implementation must test the transition at controlled time with a
predecessor proof valid immediately before the effective boundary, invalid at
the boundary, the successor valid there, status replay/nonce substitution,
Gateway withholding, all listed crash points, and Authority time uncertainty.
It must also prove through the real Endpoint/Route path that a direct old
Descriptor cannot establish a new Service Connection. These are acceptance
criteria for later selected implementation slices, not claims for this record.

## Provisional observation

RFC 6960 illustrates the necessary shape of revocation: status is a distinct
signed assertion with a validity interval, and a relying party that needs its
freshness contacts the responder. The current Ardents proof path intentionally
has no equivalent assertion. The FLP result does not itself decide Ardents
semantics, but it rules out claiming both a definitive distributed decision and
termination through an asynchronous crash/partition without additional model
assumptions. R-141 must therefore make any availability loss or added authority
explicit rather than relabel it as local recovery.

## Recommendation

Retain the current expiry-gated successor as the C0 recovery limitation; do not
promote B to an ADR or implementation slice. Confidence is high for this C0
decision: Gateway-only supersession is disproved, and online Authority status
has no permitted transport or discovery topology. This does not make expiry
gating a permanent product requirement. A future separately selected research
question may design and test a bounded Authority-currentness topology; only an
accepted ADR from that work may supersede ADR-0003, ADR-0036, ADR-0064, and
ADR-0065. It must preserve TLS, signature, expiry, replay, and generation-floor
gates and define the operational availability cost explicitly.

## Disposition

Decided on 2026-09-05: current C0 retains expiry-gated successor issuance and
the consumed-root restart limitation. The Product Owner's B direction was
evaluated and rejected for the present C0 topology, not as a permanent product
constraint. No ADR has been created or accepted and no implementation is
authorized. The external overlay diagnostic remains outside the repository as
disposable evidence.
