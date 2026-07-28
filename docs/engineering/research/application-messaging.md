# DR-01: Application Messaging

## Metadata

- Status: accepted research recommendation; ADR-0015 Accepted
- Research class: R2 deep research
- Decision owner: Application Interface / Identity and Security / Messaging
- Research owner: Wave 3 DR-01
- Date: 2026-07-25
- Frozen baseline commit:
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research: none; ADR-0011 and ADR-0015 are accepted
- Downstream consumers: Application Messaging implementation and DR-06

## Answer first

Add one deep Application Messaging module built around opaque, authority-backed
conversations. Applications list existing conversations, durably enqueue
messages, pull a bounded inbox, acknowledge consumption, and inspect bounded
delivery state. They never select Waku topics, channel generations, transport
providers, Store queries, encryption, replay, or retry.

Conversation creation, membership change, delivery-Node rebinding, and closure
are Operator-only in v1 and consume the single-authority DR-03 lifecycle. Each
membership change activates a fresh Application-channel generation; rebind
uses a same-membership generation rotation and close suppresses renewal until
the last valid generation expires. Send success means durable local outbox
acceptance retained against a separate monotonic Messaging checkpoint;
recipient delivery is at-least-once until durable inbox admission, receipt,
expiry, revocation, or fencing. No receipt claims that an Application or person
processed a message.

This adds a new persisted conversation/inbox/outbox aggregate and a versioned
Application-message wire class. The cost is justified because topic- or
mailbox-centric SDKs push authority, group fan-out, replay, and recovery races
onto every caller. Write Proposed ADR-0015 before implementation and keep
`Q=no`.

## User outcome

An authorized Application sends a small private message or immutable Content
References to an existing one-to-one or group conversation, receives messages
after restart or temporary disconnection, acknowledges local consumption, and
sees truthful bounded delivery progress without learning Waku or Channel Grant
internals.

## Scope

### In scope

- opaque one-to-one and group conversations;
- Operator-owned lifecycle and DR-03-backed membership;
- local durable send acceptance and per-recipient delivery state;
- bounded pull/long-poll receive and durable subscription cursors;
- transport deduplication, local ordering, expiry, retry, and backpressure;
- inline payloads and immutable Content References;
- restart, backup/restore, revocation, rotation, audit, privacy, and abuse;
- additive local Application protocol and Go SDK.

### Out of scope

- `Publish(topic, bytes)` or arbitrary broadcast;
- caller-selected recipients, Waku selectors, Channel Grants, generations,
  Relay, Filter, Lightpush, Store, encryption, or transport retry;
- Application-created conversations or membership administration in v1;
- exactly-once effects, global/causal order, read receipts, typing, presence,
  edits, reactions, search, push notifications, federation, or MLS;
- remote Application transport, non-Go SDKs, large inline payloads, and release
  qualification.

## Current product truth

### Supported interfaces

| Surface | Frozen-baseline contract | Truth |
|---|---|---|
| Application | protected local Identity and Content RPCs plus Go SDK | no Messaging service, method, SDK package, or conversation |
| Operator | protected identity, network, data, and diagnostics procedures | no production realm/conversation authority procedure |
| Internal | `internal/messaging` private envelopes, selectors, durable replay and Channel Grant resolution | implemented primitives, not a product Messaging aggregate |
| Network | Waku Relay/Filter/Lightpush/Store through internal adapters | fixed private discovery/data use; not exposed to Applications |
| Deployment | local provisioning and host-local backup contracts | partial authority inputs only; no production DR-03 authority |
| True external | remote realm Nodes and network path | availability is not controlled by a sending Application |

### Reachable journey

The current Application journey ends at Identity or Content handlers. Internal
data/discovery code can resolve Channel Grant material, seal a signed private
envelope, transport it through Waku, authorize it before durable replay
admission, and retain bounded Store state. No Application call constructs a
message, owns an outbox, addresses a group, receives an inbox entry, or observes
delivery.

The selected journey is:

```text
Application session + exact Access Grant
  -> Application Messaging handler
  -> conversation membership/version admission
  -> transactional idempotency/message/outbox persistence
  -> internal generation/selector/envelope/transport plan
  -> recipient authorization + replay/dedupe + inbox transaction
  -> recipient-Node received attestation
  -> sender delivery projection
```

The first four and last steps are new. Waku and private-envelope mechanics
remain behind the module boundary.

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
| Application Messaging is absent | `api/ardents/application/v1`, `sdk/go/client` | only Identity and Content contracts exist | I=no, R=no, O=no, Q=no |
| private envelopes are signed/encrypted and redact their string form | `internal/messaging/seal.go`, `open.go`, `envelope_types.go` | envelope and compatibility tests | implemented internal primitive |
| authorization precedes replay admission | ADR-0003; `internal/messaging/open.go`, `replay.go` | replay/authorization tests | implemented primitive |
| Channel Grant loss/revocation fails closed | `internal/messaging/channel.go`, `status.go` | channel/status tests | partial local implementation |
| Content objects are immutable and owner-qualified | ADR-0004; Application Content contract | Content unit/contract tests | implemented and reachable |
| production authority lifecycle exists | DR-03 and accepted ADR-0011 | accepted design only | not implemented or qualified |

Historical or local primitive evidence does not qualify the new Messaging
surface.

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
| Application | Actor Principal via local Credential/Session | exact conversation action; optional one-hop Delegation | client idempotency keys, payload, cursor | process to protected local Application socket |
| conversation member | Effective Principal | active membership and exact send/receive resource | membership visibility and inbox | Application admission to Messaging |
| delivery Node | Node Principal plus Waku Peer ID | hosts one member delivery binding; transport is not Ardents authority | inbox/outbox, Waku key, replay state | trusted Node process to hostile network |
| Realm Authority | DR-03 Realm Authority Principal | issues/rotates/revokes Application-channel generations | authority ledger and checkpoint | protected Operator workflow |
| Operator | Operator Principal | exact conversation lifecycle and fencing actions | membership intent and audit | protected Operator interface |
| malicious member/peer | Principal and/or Waku Peer ID | only current explicit grants | retained ciphertext and resource budgets | untrusted protocol input |

Credential, Session, Access Grant, Delegation, Channel Grant, Node Principal,
Waku Peer ID, conversation ID, and message ID remain distinct.

## Invariants

- Application admission authenticates Actor, derives Effective Principal,
  intersects Delegation, authorizes the exact action/resource, checks active
  membership/version and quotas, then persists.
- Conversation and message identifiers disclose no Principal, selector,
  channel generation, topology, or delivery address.
- Membership mutation is Operator-only and becomes active only through DR-03
  fresh-generation activation or explicit fencing.
- A recipient persists authorized dedupe/inbox truth before issuing `received`.
- Node receipt proves only an approved Node's signed assertion, never human or
  Application processing.
- Accepted send truth survives restart; internal retry never requires caller
  resubmission with a new idempotency key.
- All members, conversations, messages, bytes, references, subscriptions,
  receipts, retries, pages, waits, audit records, and labels are bounded.
- Revoked/expired authority never becomes valid because of cached routing,
  retained ciphertext, Store replay, restore, or clock failure.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
| Application Messaging aggregate | in-process | Application/Messaging | Ardents | yes |
| DR-03 Realm Authority | remote-owned | deployment authority | authority availability/mutation | no |
| private-envelope/replay module | in-process | Messaging/Security | Ardents | yes |
| Waku transport and bounded Store | local-substitutable adapter plus remote-owned peers | Network Foundation/peer Nodes | per candidate and Node | adapter yes, peers no |
| Application Content | in-process plus existing provider plan | Content | Content errors and authority | references optional |
| clocks and durable storage | local-substitutable | deployment | host/operator | yes within supported contract |

## Alternative designs

### Alternative A — topic-centric Application SDK

- External interface: `Publish(topic, bytes)` and `Subscribe(topic)`.
- Internal seam: thin Waku wrapper.
- State ownership: Applications reconstruct groups and retry.
- Authority model: callers select or indirectly learn Channel Grant selectors.
- Failure/recovery: transport-centric and ambiguous after restart.
- Compatibility/migration: Waku changes leak into the SDK.
- Operational cost: low implementation cost, high caller/security cost.

### Alternative B — recipient mailbox fan-out

- External interface: send to a list of Principal mailboxes.
- Internal seam: mailbox and per-recipient delivery.
- State ownership: caller owns membership snapshot and partial fan-out.
- Authority model: mailbox grants do not express stable group generations.
- Failure/recovery: callers reconcile duplicate/partial group sends.
- Compatibility/migration: simple wire, repeated policy in every caller.
- Operational cost: moderate module, high group-application cost.

### Alternative C — authority-backed conversation aggregate (selected)

- External interface: five typed conversation/message operations.
- Internal seam: conversation policy, inbox/outbox, delivery and recovery.
- State ownership: Messaging owns durable message truth; DR-03 owns grants.
- Authority model: one fresh Application-channel generation per membership
  version.
- Failure/recovery: transactional acceptance, at-least-once transport, bounded
  receipts/cursors and fail-closed restore.
- Compatibility/migration: versioned aggregate and wire class hidden from SDK.
- Operational cost: largest internal module, smallest safe caller surface.

### Decision matrix

| Criterion | Weight | A | B | C | Reasoning |
|---|---:|---:|---:|---:|---|
| Module depth | 3 | 1 | 3 | 5 | C hides policy, fan-out, replay, and transport |
| Caller leverage | 3 | 1 | 2 | 5 | C supplies reusable group semantics |
| Change locality | 2 | 1 | 3 | 4 | C localizes Waku/wire migration |
| Trust-model fit | 4 | 1 | 2 | 5 | C consumes DR-03 generations directly |
| Failure clarity | 4 | 1 | 2 | 5 | C separates acceptance, receipt, and consumption |
| Migration cost | 2 | 5 | 4 | 3 | A is cheapest but leaks change |
| Operability | 4 | 1 | 2 | 5 | C has one bounded status/recovery owner |
| **Weighted total** |  | **32** | **48** | **92** | Select C |

## Selected design

### External interface sketch

```text
ListConversations(page_token, limit) -> ConversationSummary[]
SendMessage(conversation_id, client_message_id, payload, expires_at)
  -> {message_id, accepted_at, expires_at}
ReceiveMessages(subscription_id, cursor, limit, wait)
  -> {messages[], next_cursor}
AcknowledgeMessages(subscription_id, through_cursor)
GetDelivery(message_id) -> bounded aggregate recipient states
```

Payload is exactly one of inline bytes or one to sixteen Content References.
`limit <= 100`, `wait <= 30s`, and acknowledgements advance at most 100
entries. No call contains recipient, topic, selector, generation, Waku peer,
provider, encryption, or retry fields.

Operator-only procedures create/close a conversation and change/rebind its
members. A conversation has two to thirty-two members, at most one active
delivery Node per member, and a monotonically increasing membership version.

Rebind increments a separate Messaging binding version and calls the accepted
DR-03 `realm.channel.generation.rotate` action with unchanged Principal
membership and the target Principal's new delivery-key attestation. The old
Node receives no new generation. The binding changes only after the Authority
activation checkpoint and approved-host active receipts are retained; the
immediately previous generation remains receive-only for the exact signed
drain. A suspect old host is deployment-fenced before activation.

Close first retains a monotonic Messaging `closing` tombstone, returns the
pending state, denies local new sends/replay admission, terminalizes
undelivered work, and stops Channel Grant renewal. Every approved active
delivery Node must checkpoint that tombstone or be deployment-fenced; a
partitioned Node is not counted as closed. Already admitted inbox entries
remain receive/acknowledge-only for bounded retention. The conversation becomes
`closed` after that convergence, accepted outbox work is terminal, and
current/previous grants and drains expire. DR-03 gains no conversation-close
action.

### Internal seam and state machine

The `ConversationMessaging` module owns conversation projection, idempotency,
immutable message rows, per-recipient outbox/inbox entries, subscriptions,
cursors, receipt projection, expiry, bounded retry scheduling, backup checks,
redacted diagnostics, and its own monotonic checkpoint stream. It calls DR-03
for authority lifecycle and an internal carrier port for transport.

```text
conversation: proposed -> authority_pending -> active -> rotating -> active
                                      |                    |
                                      +-> recovery_required+
active -> closing -> closed

message: accepting -> accepted -> delivering -> delivered | expired | revoked
```

The outbox transaction stores message plus every recipient state before send.
The inbox transaction checks authority/replay, stores the message and advances
the local admission cursor before receipt. Crash recovery resumes durable
phases; it never reconstructs truth from Waku Store alone.

### Authority and audit semantics

Application actions are `messaging.conversation.list`,
`messaging.message.send`, `messaging.message.receive`,
`messaging.message.acknowledge`, and `messaging.delivery.get` on exact
conversation/subscription/message resources. Actor and Effective Principal are
both recorded; one-hop Delegation is allowed only by exact intersection.

Operator actions for create, membership change, delivery-Node rebind, close,
and recovery do not accept Delegation. DR-03 exclusively owns Channel Grant
issuance, generation activation, revocation, survivor attestations, fencing,
and checkpoint truth. Audit records request ID, Actor, Effective, resource,
membership version, stable outcome, counts and digests, never payload,
selector, Channel Grant or Content locator.

Membership change maps to `realm.channel.membership.change`; delivery-Node
rebind maps to `realm.channel.generation.rotate`. Close maps to a
Messaging-owned monotonic tombstone, refusal to request renewal, and expiry of
the accepted DR-03 grants/drain windows. No Messaging procedure fabricates a
DR-03 action or checkpoint.

## Delivery and data semantics

| Concern | Selected contract |
|---|---|
| Acceptance | send success is durable local outbox acceptance only |
| Addressing | opaque conversation; members come from current active version |
| Acknowledgement | Node `received` after durable inbox; Application ack after consumption |
| Deduplication | sender idempotency tuple plus recipient global MessageID |
| Ordering | stable recipient-local admission cursor only |
| Expiry | 1 minute–7 days, default 24 hours; stops undelivered work |
| Retry | bounded internal at-least-once until receipt/terminal state |
| Inline limit | 32 KiB |
| Large payload | at most 16 immutable Content References; no authority amplification |
| Backpressure | pre-accept `ResourceExhausted`; post-accept durable pending truth |
| Terminal states | delivered, partially delivered at expiry, expired, revoked, or recovery-required |

Per Principal: at most 128 conversations and sixteen subscriptions. Per Node:
at most 1,024 conversations and 128 subscriptions. Conversation membership is
at most thirty-two. Implementations must define finite inbox/outbox message and
byte caps, receipt retention, retry count/backoff/deadline, query pages, and
audit retention in versioned profiles before implementation acceptance.

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
| invalid auth/grant/member | typed unauthenticated/forbidden/not-found | no message | no | repair exact authority |
| duplicate same send | original result | idempotency row | no new delivery | none |
| duplicate changed digest | conflict | original row | no | fix caller |
| local capacity before accept | resource exhausted | no message | caller bounded backoff | expand/clean within policy |
| crash after accept | accepted/pending after restart | outbox authoritative | internal resume | inspect if stuck |
| recipient offline/full | pending until expiry | outbox and retry phase | bounded internal | restore capacity/Node |
| duplicate transport/Store replay | no duplicate inbox | replay/message row | no caller action | none |
| membership removal | old generation stops; pending removed recipient becomes revoked | checkpoint and recipient state | no stale retry | complete DR-03 cutover/fence |
| authority unavailable | send/change fails or pending traffic continues only within valid grants | last accepted checkpoint | no mutation retry loop | restore authority/repository |
| expiry | partial/expired delivery projection | terminal row retained | no | none |
| complete restart | cursors and accepted truth reload | consistency group plus both checkpoint heads | deterministic resume | restore full group |
| stale/partial restore | Messaging unavailable/recovery-required | restored copy and repository truth preserved | fail closed | restore exact group matching both heads |
| incompatible upgrade | preflight/start failure | old schema preserved | no in-place downgrade | restore backup/fallback image |

Messaging persistence, replay state, and both checkpoint references form a
stopped-Node consistency contract. Each Node owns one independently retained
hash-chained `MessagingStateCheckpoint/v1` stream binding Node Principal,
Messaging sequence/state digest, software/schema/profile, Realm, the exact
accepted DR-03 head, and predecessor digest. Every externally acknowledged
Messaging mutation, recipient receipt, Application acknowledgement,
binding/lifecycle cutover, and terminal delivery transition succeeds only
after exact compare-and-append. An interrupted append resumes the same prepared
successor and is never acknowledged optimistically.

The Messaging checkpoint repository is outside the stopped-Node backup fault
domain and supplies one unique head per Node, immutable predecessor history,
create-if-absent only for empty genesis, and no overwrite/repair path. Its
finite capacity is frozen in AM-01 and exhaustion fails writes closed. Restore
reads the Messaging and DR-03 repositories first and admits the same Node/Realm
only when the complete stopped backup equals both unique heads. Thus an older
Messaging backup cannot match a still-current Authority head and silently lose
accepted messages or cursor/acknowledgement truth. The Messaging repository is
freshness evidence only: it is neither membership authority nor message
storage.

Migration from the frozen baseline creates an empty versioned Messaging store
and its create-if-absent genesis head only after ADR-0011 authority genesis;
there is no legacy Application message data to import. Mixed Nodes may carry
the new class only inside a release-declared compatibility window.

## Security, privacy, and abuse analysis

- Unauthenticated and unauthorized callers fail before conversation lookup or
  payload work; absent/forbidden resources use the documented privacy-uniform
  projection.
- A malicious member may retain plaintext, withhold Application ack, submit
  duplicates, fill quotas, or lie about human processing. It cannot mint
  membership, extend expiry, select a generation, or make a Node receipt prove
  an Application side effect.
- Removing a member cannot erase retained plaintext and is not instantaneous
  during partition. New-generation completion requires attestations or
  deployment fencing under DR-03.
- Sender idempotency, recipient replay/message dedupe, payload size, member
  count, pages, subscriptions, inbox/outbox bytes, rates and retry budgets
  bound amplification.
- Content References do not grant fetch authority. Messaging never fetches
  referenced content during send admission and never logs locators.
- Public metrics exclude Principal, conversation/message ID, selector,
  endpoint, Content Reference and Waku Peer ID labels.

## Observability

Protected status reports aggregate active/rotating/recovery-required
conversations, outbox age buckets, pending/delivered/expired counts, inbox
capacity buckets, subscription lag buckets, authority generation/checkpoint
freshness, and stable reason codes. Metrics use only bounded state/outcome
labels. Operator diagnostics can page exact conversation/message audit under
separate authorization and redact payload and cryptographic material.

A stuck oldest-outbox age, authority/checkpoint unavailability, inbox capacity,
receipt timeout, rotation/fencing wait, replay-capacity exhaustion, or restore
mismatch degrades Messaging readiness. Network-only health never reports the
Application Messaging contract ready.

## Compatibility consequences

- **Wire:** additive versioned Application message class inside DR-03's
  Application channel; no public Waku fields.
- **Persistence:** new versioned conversation, idempotency, inbox/outbox,
  subscription, receipt, expiry, and audit state.
- **Configuration:** only versioned finite quota/retention profiles; no caller
  selector or transport configuration.
- **Backup/restore:** complete Messaging group plus exact independent
  Messaging and DR-03 heads; same-Authority-head rollback, partial restore,
  fork, ambiguity, or repository unavailability fails closed.
- **Rollout:** authority and compatible member support precede activation;
  ordinary mixed generation is bounded and declared.
- **Downgrade:** no in-place schema downgrade; restore complete stopped backup.
- **SDK:** additive typed Go package; old clients see feature unavailable.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit | IDs, bounds, idempotency conflict, state machines, cursor/ack, expiry, redaction | Go unit/fake clock | JUnit/JSON |
| Contract | exact actions/resources, Delegation, payload oneof, limits, privacy-uniform errors, no Waku fields | real handler and SDK | protocol corpus |
| Integration | transactional outbox/inbox crash points, replay, receipts, Content References, restart | local Nodes/storage | tagged trace |
| E2E | two-party/group, offline recipient, partial expiry, removal/fence, rebind, subscription resume | real three-Node topology | sanitized scenario artifacts |
| Security | malicious member, forged/stale generation, replay floods, quota exhaustion, restore rollback, no secret/payload leakage | hostile peer/client lab | security report |
| Deployment | authority outage/recovery, checkpoint repo, host loss, backup/restore, mixed upgrade and rollback | DR-04 supported topology | deployment bundle |
| Release | all required gates once on one clean commit without hidden retry | independent DR-06 runners | accepted qualification snapshot |

## Open questions

No unresolved question changes the selected external interface, trust root,
state owner, wire boundary, or migration contract. Exact finite byte/message
capacities and retry schedule are versioned implementation-profile constants,
not caller-facing freedom, and must be fixed before the first implementation
slice is accepted.

## Decision-register proposals

- W3-D002: select opaque authority-backed conversations and the five-operation
  Application interface.
- Membership is Operator-only in v1 and every change activates a fresh DR-03
  Application-channel generation.
- Send success is durable local acceptance; transport is at-least-once and
  Node receipt is not Application processing proof.
- Recipient-local cursor order only; no exactly-once, total/causal ordering,
  arbitrary topics, federation, presence, or read receipts.
- Inline payload is bounded to 32 KiB; large immutable data uses Content
  References without authority amplification.

## Recommendation

Implement only from accepted ADR-0015 and its frozen AM-01 profile.

## Vertically sliced implementation issues

### AM-01 — Freeze contracts and authority bindings

- User story: an Application developer sees one bounded conversation API.
- Behavior: accept ADR-0015; add exact protocol/SDK types, action registry,
  resource binding and negative corpus with no Waku fields.
- Acceptance: contract/security tests cover auth, Delegation, privacy and
  bounds.
- Blocked by: none; ADR-0011 and ADR-0015 are accepted.
- Research class: R0.

### AM-02 — Create and rotate a conversation

- User story: an Operator creates a two-party/group conversation and changes
  membership without stale authority.
- Behavior: persist lifecycle, invoke DR-03 fresh generation, attest/fence,
  activate or enter recovery-required.
- Acceptance: crash/restart, removal, rebind and concurrent mutation tests.
- Blocked by: AM-01 and production DR-03 authority slices.
- Research class: R1.

### AM-03 — Durably send and inspect delivery

- User story: an Application retries a send safely and sees truthful progress.
- Behavior: transactional idempotency/message/outbox, bounded fan-out, expiry,
  delivery projection and Content Reference validation.
- Acceptance: same/different digest, capacity, crash and partial expiry tests.
- Blocked by: AM-02 and existing Content contract.
- Research class: R1.

### AM-04 — Admit, receive and acknowledge

- User story: a recipient resumes a bounded inbox after restart.
- Behavior: authorize-before-replay inbox transaction, Node receipt, durable
  subscription cursor, long-poll and consumption ack.
- Acceptance: replay, page/order, ack skip, restart and full-inbox cases.
- Blocked by: AM-03.
- Research class: R1.

### AM-05 — Recover and migrate Messaging truth

- User story: an Operator restores without resurrecting stale membership or
  losing accepted messages silently.
- Behavior: consistency-group backup, exact Messaging/Authority dual-head
  comparison, retry recovery, version migration and fail-closed downgrade.
- Acceptance: every crash point, stale/partial restore, authority outage and
  mixed-generation negatives.
- Blocked by: AM-02 through AM-04 and DR-04 authority/topology recovery.
- Research class: R1/R3.

### AM-06 — Qualify bounded conversations

- User story: a release reviewer can accept the exact Messaging promise.
- Behavior: run contract, security, multinode, deployment, recovery and release
  matrix on one commit.
- Acceptance: retained evidence has source/toolchain identity and no retry
  masking; only DR-06 may promote `Q`.
- Blocked by: AM-05 and DR-06 scope.
- Research class: R3.
