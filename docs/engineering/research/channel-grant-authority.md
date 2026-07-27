# DR-03: Production Channel Grant authority

## Metadata

- Status: accepted research recommendation; ADR-0011 accepted on 2026-07-27
- Research class: R2 deep research
- Decision owner: Identity and Security
- Research owner: Wave 3 DR-03
- Date: 2026-07-25
- Frozen baseline commit: `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Preparation revision inspected: `cbec069c37df9cf57756970a2c3a0eef8c232778`
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research: none
- Downstream consumers: DR-01 Application Messaging, DR-04 private
  multi-host reachability assumptions, DR-06 qualification
- Accepted decision: `docs/adr/0011-single-authority-channel-grant-lifecycle.md`

## Answer first

Ardents should make one deployment-owned Realm Authority Principal the sole
Channel Grant trust root for one realm. Its state is hosted by one designated
authority Node, but the Realm Authority Principal remains distinct from that
Node Principal, its Waku Peer ID, and its transport identity. Operators use
exact, Operator-only realm actions. Member Nodes exchange recipient-attested
HPKE delivery envelopes and opaque durable receipts through the protected
Operator workflow. The Application Interface never exposes authority,
selectors, channel secrets, Waku operations, or the authority topology.
Receipts are protocol attestations by the approved member Node implementation
and host, not cryptographic proof that a potentially malicious recipient really
persisted or activated state.

Every membership change creates a fresh channel generation. A removal also
creates signed grant revocations. The authority completes a cutover only after
every surviving member has acknowledged activation or the deployment owner has
fenced the non-acknowledging member. A partition therefore delays completion or
removes the unavailable survivor from service; it never converts eventual
revocation into an instantaneous claim. Discovery, data, Application, and
capability-control channels have independent IDs, secrets, generations,
membership, replay state, and audit records.

The main tradeoff is a small, explicit authority control-plane availability
dependency. Existing traffic can continue while the authority is unavailable
until its grants expire, but membership, recovery, and rotation stop. This is
preferable to Node-local issuers, which cannot provide one revocation truth.
The decision changes trust, persisted state, control artifacts, recovery, and
rollout semantics, so ADR-0011 must be accepted before implementation.

## User outcome

A deployment Operator can add or remove a Principal from a private Ardents
realm across hosts, obtain attributable protocol evidence of which generation
each approved member Node reports installed and active, recover safely after
restart or backup restore, and audit the result without revealing Channel
Grant material. A noncompliant or untrusted member is fenced rather than
treated as cryptographically proven healthy.

## Scope

### In scope

- realm trust root and authority ownership;
- Operator authorization for realm administration;
- Channel Grant issuance, recipient-bound protected delivery, install and
  activation acknowledgement;
- realm and per-channel membership;
- grant renewal, revocation, generation rotation, authority-key rotation;
- restart, interrupted rollout, backup, restore, disaster recovery;
- discovery/data/Application/control channel separation;
- bounded audit, diagnostics, cardinality, retry, and abuse behavior;
- first-release single-realm operation, explicit non-federation, and migration
  from local `ardents.local-realm/v2`;
- the authority assumptions DR-01 and DR-04 must consume.

### Out of scope

- the Application Messaging API, conversation addressing, send/receive
  semantics, delivery ordering, or message acknowledgements;
- arbitrary `Publish(topic, bytes)`;
- Waku selector, encryption, Relay, Filter, Lightpush, or Store APIs;
- Kubernetes, QUIC, WebTransport, WebRTC, remote Application transport, and
  non-Go SDKs;
- an Ardents-operated public realm authority;
- cross-realm federation, transitive trust, threshold authorities, and a
  general certificate or policy engine;
- replacing the existing private-envelope protocol with MLS;
- implementing or qualifying the selected design in this research task.

## Current product truth

All product claims below refer to the frozen product commit. No production or
protocol files changed between that commit and the inspected preparation
revision.

### Supported interfaces

| Boundary | Current interface | Product truth |
|---|---|---|
| Operator | `ardentsd init --authority-dir ... --node-dir ... --secret-dir ...` | Local stopped-node initialization only; no production authority command or RPC exists. |
| Application | none | Applications cannot issue, import, rotate, revoke, or inspect Channel Grants. |
| Internal Identity | `capability.Service.ImportGrant`, `ImportSenderGrant`, `ApplyRevocation`, `ResolveCapability`, HPKE attestation/delivery helpers | Implemented Go seams, not a supported caller journey. |
| Internal provisioning | `provision.OpenOrCreate(...).ProvisionNode(...)` | A shared local authority directory creates one issuer, one discovery channel, one data channel, and member grants. |
| Deployment | local Compose/multi-host test provisioning mounts one authority directory while Nodes are stopped | Deployment-owned test/local mechanism; not a production multi-host authority contract. |
| True external | Waku peers and deployment secret/backup storage | Neither is a Channel Grant issuer. Waku transport identity is not Principal authority. |

### Reachable journey

The only current caller-to-domain journey is:

```text
deployment invokes ardentsd init
  -> provision.OpenOrCreate(authority-dir)
  -> authority.json is created or loaded
  -> ProvisionNode creates/loads the Node Principal
  -> a subject grant is signed for the static discovery generation
  -> a subject grant is signed for the static data generation
  -> both grants and every known sender grant are imported directly
     into the stopped Node's encrypted channel-grants.db
  -> operator.json receives opaque local references
  -> runtime resolves those references for private discovery/data channels
```

The journey becomes internal-only at `provision.Authority`. It requires the
authority and Node secret/data directories to be available to one provisioning
process. It has no independent member request, protected cross-host delivery,
delivery acknowledgement, activation checkpoint, production Operator
authorization, member removal, generation rotation, or authority restore
workflow.

The HPKE path is implemented but not composed into that journey:

```text
recipient capability service
  -> creates persistent X25519 delivery key
  -> creates Principal-signed delivery-key attestation
issuer helper
  -> verifies attestation and seals one subject grant
recipient helper
  -> decrypts and imports the grant
```

It proves a useful cryptographic primitive. It does not establish initial
trust, deliver sender authorization state, acknowledge durable installation,
activate a generation, or recover an interrupted membership operation.

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
| The local issuer persists its private Ed25519 key, discovery/data IDs and secrets, generation `1`, and member grant records in one protected JSON file. | `internal/provision/realm.go`, `realm_authority.go` | `internal/provision/realm_test.go`, `run_test.go` | implemented internally; reachable only through local init |
| Grants bind issuer, subject, permissions, scope, validity, channel, generation, secret, and grant ID; local references are opaque. | `internal/identity/capability_contracts.go`, `internal/identity/capability/*` | capability service and canonical tests | implemented |
| Subject and retained sender grants are checked against a purpose-scoped trusted issuer and revocation ledger. | `internal/identity/capability/service.go`, `sender.go` | capability service tests | implemented |
| Private-message sender authorization precedes durable replay admission. | ADR-0003; `internal/messaging/open.go` | messaging envelope and restart tests | implemented, locally evidenced; not production-qualified |
| X25519 recipient delivery keys are persistent, Ed25519-attested, and usable with RFC 9180 HPKE. | `internal/identity/capability/attestation.go`, `delivery.go`; `docs/protocols/network-privacy-protocol.md` | `TestHPKEGrantDeliveryIsRecipientBoundAndPersistent` and negative tests | implemented internally; no supported delivery interface |
| Revocation of a grant can be retained and enforced locally. | `internal/identity/capability/service.go`, `sender.go` | revoke/import/authorize tests | implemented locally; no production distribution or completion semantics |
| Generation changes alter selectors and envelope keys. | `internal/messaging/selector.go` | selector/channel tests | implemented primitive; no authority rotation workflow |
| Capability storage, keys, replay state, and Node identity have fail-closed consistency-group rules. | `docs/security/persistent-state-security.md` | node identity and persistence tests | documented/partially evidenced; authority state is not included as a production restore group |
| Local provisioning configures `<data-dir>/channel-grants.db`, while the persistent-state inventory and capability-authority consistency group describe the encrypted ledger as residing in `ardents.db` and do not enumerate that separate file. | `internal/provision/realm_storage.go`, `config.go`; `docs/security/persistent-state-security.md` | whole-volume deployment backup includes the file, but no authority-specific restore proof exists | operability/documentation mismatch to resolve before production |
| A production authority issues and revokes Channel Grants through a supported interface. | capability catalogue and evidence register | no matching interface/evidence | I=partial, R=no, O=no, Q=no |

Local tests do not promote `realm.channel-grant-authority` to `Q=yes`.

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
| Realm Authority | distinct Ed25519 Realm Authority Principal | signs Channel Grants, revocations, generation checkpoints, and planned successor transition | signing key, realm ledger, current/pending channel secrets | deployment-owned authority Node boundary |
| Realm Operator | Actor Principal authenticated on the authority Node Operator Interface | exact realm actions from Node-issued Access Grants; Effective Principal is evaluated separately | membership intent, recovery and audit operations | existing identity/access admission |
| Member Principal | Principal named by a subject-bound grant | only the permissions, scope, channel and validity in that grant | HPKE delivery key, subject grant, receipt key material | member Node capability store |
| Member Node Operator | Operator of that one Node | asks the Node to attest its delivery key and atomically install an envelope | protected import/export files and local receipt | protected Operator socket/SSH forwarding |
| Approved member implementation/host | released Node binary on a deployment-controlled host | attests through receipts what its local installer/runtime did | capability store, active checkpoint, receipt key | trusted execution/host boundary; receipts do not survive compromise of this boundary |
| Application Principal | separate Principal | may later hold `channel.application` grants; never realm administration merely because it can message | Application channel grants | Application Interface remains separate |
| Deployment owner | OS/deployment authority, not implicitly an Ardents Principal | hosts/fences authority/member Nodes and protects backups; must still authenticate as a Realm Operator for realm mutations | volumes, secret files, backup repository | host, SSH, secret manager, backup system |
| Monotonic checkpoint repository | deployment-owned service identity, not an Ardents Principal | compare-and-append of one immutable realm head; no Channel Grant authority | sequence, checkpoint digest, immutable history | anti-rollback trust root outside the authority-backup fault domain |
| Waku participant | Waku Peer ID | transport participation only | retained ciphertext and opaque selectors | Waku/libp2p boundary |
| Transport endpoint | TLS identity where WSS is used | carrier authentication only | TLS key/certificate | network ingress boundary |
| Malicious current member | valid Principal and current Channel Grant | valid use within its grant, plus attempted abuse | its own current/old secrets | inside confidentiality group, outside authority |

Principal, Credential, Access Grant, Delegation, Channel Grant, Waku Peer ID,
delivery-key attestation, and transport identity remain separate concepts.

## Invariants

- One realm has exactly one active Realm Authority Principal and one
  monotonically increasing authority epoch.
- The designated authority Node hosts the module; its Node Principal is not the
  issuer unless separately enrolled as the Realm Authority Principal.
- Authority mutations pass authentication, exact Access Grant authorization,
  Product Policy, and audit admission before state commit.
- The Application Interface contains no authority procedure.
- Discovery, data, Application, and capability-control channels never share a
  channel ID, channel secret, generation, selector, replay ledger, or
  membership merely for convenience.
- `realm.capability_control` grants no discovery, data, or Application payload
  decryption. Initial trust and recovery do not depend on a channel whose
  secret is being replaced.
- Adding or removing any channel member creates a fresh random secret and next
  generation. Routine correlation rotation does the same without changing
  membership.
- Removal also commits signed revocations for every removed current grant.
  Revocation rejects the sender; generation rotation excludes it from future
  confidentiality.
- An authority operation is not complete until every survivor on an approved
  member host returns an activation receipt or the deployment owner fences that
  survivor. The receipt is trustworthy only to the extent that the released
  member implementation and host are trusted. A suspected compromised,
  modified, or noncompliant member must be fenced even if it returns a
  syntactically valid receipt.
- A member missing the current activation checkpoint is not ready and must
  update before it resumes private publication, Store query, or domain
  delivery.
- The new generation is publish-only-current after activation. The previous
  generation may be receive-only for at most maximum envelope lifetime plus
  clock skew, as already required by the private protocol.
- Old ciphertext cannot be made secret from a former holder. Adding a member
  rotates first so it receives no prior-generation key.
- HPKE delivery is bound to a valid Principal-signed delivery-key attestation.
  The envelope contains an independent receipt key; a receipt proves
  possession of the delivered bundle only. For the approved member
  implementation, `installed` and `active` are protocol attestations emitted
  after the corresponding durable transition; the MAC cannot prove that fact
  against a holder that deliberately violates the implementation.
- A conforming member never emits receipt success before atomic
  capability-store installation or activation. Authority policy never treats a
  receipt from an unapproved or suspect host as sufficient convergence.
- Restart loads one durable authority operation state and resumes it; it never
  infers completion from process memory or retries a mutation under a new ID.
- The anti-rollback repository supports atomic compare-and-append against the
  exact previous realm sequence, immutable retention of every accepted head,
  and read of the unique latest head. It is outside the authority database,
  archive, credentials, and backup fault domain.
- Restore cannot move authority sequence backward. Repository loss,
  rollback/non-monotonic history, multiple heads, an unexpected CAS head, or
  inability to read the latest head blocks same-realm restore and every further
  security mutation. It is never reinitialized from authority state; recovery
  creates a new realm.
- Secrets, channel/grant IDs, selectors, delivery ciphertext, receipt keys,
  Principal IDs in metrics, and unbounded channel/member values never enter
  logs or metrics.
- Bounds are part of v1: one realm per authority instance; at most 256 realm
  members, 1,024 active channels, 256 members per channel, one pending
  generation and one receive-only previous generation per channel, four
  outstanding deliveries per member/channel, and a 256 KiB delivery envelope.
- Grants are valid for at most 30 days and renew no later than 24 hours before
  expiry. Delivery attestations are valid for at most 30 days. An unactivated
  operation expires after a configured deadline no longer than 24 hours.
- Audit retention is explicitly bounded by deployment policy and must retain at
  least the current checkpoint and all records needed by the newest verified
  backup. Exhaustion fails authority mutations closed; it never drops unaudited
  state.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
| identity/access admission and audit outbox | in-process | Identity and Access | authority Node | yes, contract fixture |
| realm authority state machine and ledger | in-process | Identity and Security | authority Node | yes |
| capability store, sender authorization and revocation | in-process | Identity | member Node | yes |
| HPKE sealing/opening | local-substitutable | Identity cryptography | issuing/member process | yes, deterministic vectors |
| Operator Connect handler and CLI | local-substitutable | Operator Interface | local Node | yes |
| protected Unix socket / SSH stream-local forwarding | local-substitutable | deployment | each host | yes |
| member Node import and activation | remote-owned | member deployment | member Node/deployment owner | no for E2E |
| authority Node availability | remote-owned | realm deployment | deployment owner | no |
| private control delivery after bootstrap | remote-owned, optional optimization | Network/Identity | authority and member Nodes | yes with protected file workflow |
| Waku Relay/Store/Filter/Lightpush | remote-owned or true-external by topology | Network owner/operator | network deployment | locally replaceable only in tests |
| monotonic checkpoint repository | true-external anti-rollback trust root, deployment-owned | security/backup operator independent of authority backups | repository owner; authority fails closed on unavailable, non-monotonic or ambiguous head | contract fixture only |
| time source and bounded clock agreement | true-external | host operations | deployment owner | injectable in tests |

## Alternative designs

### Alternative A: one authority module on a designated authority Node

- External interface: exact Operator procedures on the protected authority and
  member Node interfaces; CLI orchestrates attestation, encrypted delivery,
  receipt, activation, status, and audit.
- Internal seam: one transactional `RealmAuthority` state machine plus one
  atomic member-side generation installer.
- State ownership: authority Node owns membership, grants, revocations,
  operations, audit sequence and checkpoints; each member owns only received
  grants, snapshots, receipts and replay truth.
- Authority model: one Realm Authority Principal, exact realm Access Grants,
  no federation or transitive trust.
- Failure and recovery: durable resumable operations; trusted-host survivor
  attestations or fencing; monotonic-repository-verified restore; new realm on
  unprovable freshness.
- Compatibility and migration: preserves Channel Grant and private-envelope
  vocabulary; introduces versioned authority/control artifacts and a
  maintenance migration.
- Operational cost: one highly protected, backed-up authority Node and a
  reachable Operator control plane for changes.

### Alternative B: deployment CLI plus shared authority files

- External interface: extend `ardentsd init` and exchange protected directories
  or archives between hosts.
- Internal seam: keep `provision.Authority` as a file mutator.
- State ownership: the deployment filesystem owns `authority.json`; each
  provisioning run reads and rewrites it.
- Authority model: possession of the directory is authority; no authenticated
  Realm Operator Principal or exact realm actions.
- Failure and recovery: file locking and manual copying; crash and concurrent
  host updates risk divergent state or ambiguous acknowledgement.
- Compatibility and migration: closest to current local profile.
- Operational cost: initially low, but unsafe remote copying, no live
  revocation workflow, and poor audit/recovery leverage.

### Alternative C: MLS epochs for every channel

- External interface: an MLS group service and client state per channel.
- Internal seam: replace Channel Grant group-secret distribution with RFC 9420
  proposals, commits, welcomes, epochs, and ratchet-tree state.
- State ownership: every member owns evolving group state; one delivery service
  carries MLS messages.
- Authority model: group membership and external joins use MLS semantics rather
  than the existing single issuer and retained sender grants.
- Failure and recovery: asynchronous group key establishment, forward secrecy,
  and post-compromise security are stronger, but missed commits, state loss,
  resynchronization, and external join become new product state machines.
- Compatibility and migration: replaces the existing grant, generation,
  selector, sender-authorization, and persisted-state assumptions.
- Operational cost: highest; requires a separate protocol and interoperability
  program before Ardents has a supported Application Messaging surface.

### Decision matrix

Scores are 1 (poor) through 5 (strong); higher is better.

| Criterion | Weight | Alternative A | Alternative B | Alternative C | Evidence or reasoning |
|---|---:|---:|---:|---:|---|
| Module depth | 5 | 5 | 2 | 4 | A hides lifecycle behind two deep seams; B leaks files; C is deep but replaces more domains. |
| Caller leverage | 4 | 5 | 2 | 3 | A gives the Operator one resumable workflow; B requires ceremony; C exposes group protocol lifecycle. |
| Change locality | 4 | 4 | 3 | 1 | A extends Identity/Operator/deployment; C changes messaging and wire fundamentals. |
| Trust-model fit | 5 | 5 | 1 | 3 | A matches purpose-scoped Realm Authority Principal and Access Grants. |
| Failure clarity | 5 | 5 | 1 | 3 | A names the receipt host-trust boundary and requires fencing; B cannot even attribute remote activation state. |
| Migration cost | 3 | 3 | 5 | 1 | B is cheapest; A has bounded v2 import; C is a protocol replacement. |
| Operability | 5 | 5 | 2 | 2 | A has audit/checkpoint/recovery ownership; C adds per-member group state. |
| Weighted total |  | 136 | 57 | 68 | Alternative A selected. |

Alternative A is selected. Alternative B remains only the supported local
bootstrap source during migration, not the production authority. Alternative C
is rejected for v1; it may be separately researched if Ardents later requires
MLS forward-secrecy/post-compromise properties.

## Selected design

### Realm and trust model

`RealmID` is a random, versioned 128-bit identifier, independent of names,
Node Principals, Waku Peer IDs, endpoints, and the authority host. One
`RealmAuthorityRecord` binds it to:

- active Realm Authority Principal and authority epoch;
- supported control-artifact version;
- current sequence and audit-chain head;
- bounded member records;
- bounded channel records;
- at most one active authority operation per channel;
- latest verified backup checkpoint.

The authority private key is a deployment secret and should be held by an
OS-backed or hardware-backed signer where supported. The authority Node stores
the public identity and calls the signer through a narrow signing seam. The
member trust registry accepts `PurposeChannelIssue` from the active authority
Principal only. Planned authority rotation requires a predecessor-signed and
successor-signed transition, increments authority epoch, updates trust, and
rotates every channel. Loss of the old authority key cannot silently nominate a
successor; it creates a new realm.

The first release supports one authority and one realm per authority instance.
It does not federate. A Principal can independently join several realms and
store unrelated grants. A cross-realm Application relationship requires an
explicit channel and grants issued under one selected realm; no membership or
trust is inherited.

### External interface sketch

These are Operator procedures, not Application procedures. Wire request and
response types must be versioned, bounded, reject unknown fields, and return
opaque local operation/delivery IDs only.

```go
type AuthorityService interface {
    ChangeMembership(context.Context, MembershipChange) (Operation, error)
    Rotate(context.Context, RotationRequest) (Operation, error)
    Deliver(context.Context, DeliveryRequest) (DeliveryEnvelope, error)
    Acknowledge(context.Context, DeliveryReceipt) (Operation, error)
    Activate(context.Context, ActivationRequest) (Operation, error)
    Inspect(context.Context, InspectRequest) (Status, error)
}

type MemberService interface {
    PrepareDelivery(context.Context, PrepareRequest) (DeliveryAttestation, error)
    InstallDelivery(context.Context, DeliveryEnvelope) (DeliveryReceipt, error)
    ActivateGeneration(context.Context, ActivationEnvelope) (DeliveryReceipt, error)
}

type CheckpointRepository interface {
    ReadHead(context.Context, RealmID) (CheckpointHead, error)
    CompareAndAppend(
        context.Context,
        RealmID,
        expectedPreviousSequence uint64,
        next SignedCheckpoint,
    ) (CheckpointHead, error)
}
```

`Inspect` returns redacted realm/channel class, generation, operation phase,
counts, deadlines and stable reasons. It does not return membership lists
unless the caller has the separate exact audit/read action, and never returns
Channel Grant material.

The CLI composes the Node procedures over local protected sockets or SSH
stream-local forwarding. It may move an attestation, ciphertext envelope, or
receipt through a protected temporary file, but never plaintext grant
material. Temporary files are private, bounded, atomically replaced, and
deleted after import/acknowledgement.

`CheckpointRepository` is a deployment integration, not an Operator or
Application API. `CompareAndAppend` succeeds only when the retained unique head
equals the expected prior sequence and the new signed checkpoint is exactly its
successor. It atomically retains the new immutable head and prior history.
Create-if-absent is permitted only for a new random RealmID during a stopped
genesis ceremony. Delete, replace, truncate, blind put, and last-writer-wins are
not supported operations.

### Exact authority

The authority Node defines:

| Action | Resource | Delegation |
|---|---|---|
| `realm.channel.membership.change` | exact `realm/<RealmID>/channel/<ChannelID>` | one-hop intersection allowed |
| `realm.channel.generation.rotate` | exact channel | one-hop intersection allowed |
| `realm.channel.delivery.issue` | exact operation/delivery | one-hop intersection allowed |
| `realm.channel.delivery.acknowledge` | exact operation/delivery | one-hop intersection allowed |
| `realm.channel.activation.commit` | exact operation | one-hop intersection allowed |
| `realm.channel.audit.read` | realm or exact channel | one-hop intersection allowed |
| `realm.channel.recovery.execute` | exact realm | non-delegable in Product Policy |
| `realm.authority.rotate` | exact realm | non-delegable in Product Policy |

For a delegated call, the Actor is the authenticated Principal and Effective is
the delegator. Actor grants, Effective grants, the one-hop Delegation, Product
Policy, and resource all intersect. Audit retains both. A valid Delegation does
not expose these actions on the Application Interface.

Member-side install requires the Operator of that member Node plus an envelope
whose subject equals the local Principal, realm/authority are trusted,
generation is the expected successor, and operation sequence is newer than the
local checkpoint.

### Internal seam and persisted state

`RealmAuthority` owns one transaction containing:

- realm/authority epoch and sequence;
- member status: `candidate`, `active`, `suspended`, `removed`;
- channel class, membership, current/previous/pending generation;
- signed subject and sender grants plus revocations;
- operation intent, deadline, phase, Actor/Effective attribution;
- delivery ID, subject, attestation digest, envelope digest, receipt verifier,
  install/activation status and retry generation;
- audit outbox record, chain head and signed checkpoint.

Raw channel secrets, the authority key, and receipt verifier secrets are
encrypted under separately provisioned authority-store keys. The signing key is
not in the database. Member Identity storage owns imported grants, sender
snapshot, revocations, local checkpoint, receipt state, and delivery private
key. Network receives only operation-local resolved material.

`GenerationInstaller` validates and atomically commits a complete
`GenerationBundle`:

```text
version, realm_id, authority_epoch, authority_sequence,
operation_id, channel_class, channel_id, generation,
not_before, activation_deadline,
subject_grant, bounded_sender_grants, bounded_revocations,
previous_generation_drain_deadline,
receipt_key, authority_signature
```

The bundle is sealed to the member's attested X25519 key using the existing
RFC 9180 suite. `delivery_id`, suite/version and subject Principal are bound in
HPKE `info`. The recipient returns:

```text
HMAC(receipt_key,
  phase || delivery_id || envelope_digest ||
  authority_sequence || channel_id || generation)
```

The receipt MAC proves that its producer possessed the delivered receipt key
and binds its asserted phase to the delivery. It does not cryptographically
prove durable storage or runtime behavior because a compromised recipient can
use the key arbitrarily. The approved member implementation emits `installed`
only after durable atomic installation and `active` only after its runtime
switches to the activation checkpoint and marks the previous generation
receive-only. The authority accepts these as attestations from that trusted
implementation/host. A modified, compromised, or otherwise unapproved member
is fenced even if its MAC verifies. Receipts contain no secret, grant,
selector, Principal, or endpoint.

### State machine

```text
requested
  -> prepared
  -> delivering
  -> installed
  -> activation_committed
  -> activating
  -> completed

requested/prepared/delivering/installed
  -> aborted (only before activation commit)

activation_committed/activating
  -> checkpointing by active receipts or explicit fencing
  -> completed after the signed checkpoint is retained externally
  -> recovery_required on timeout; roll forward only
```

Preparation creates the pending generation and, for removal, its revocations.
Delivery retries reuse the same delivery identity and bytes when the
attestation remains valid. An explicit reissue increments the delivery retry
generation and invalidates the prior receipt verifier without changing the
authority operation.

Activation is a signed, durable authority checkpoint with an exact effective
time. After it commits, rollback to the old generation is forbidden. A failed
or partial activation is repaired by finishing delivery, fencing a member, or
rotating forward again. Completion also requires the monotonic checkpoint
repository to accept the new signed head with compare-and-append against the
exact prior sequence. An unavailable repository leaves the already active
operation in `checkpointing`; an unexpected, missing, forked, or lower head
moves the authority to `recovery_required`. Both block another security
mutation without rolling back. Startup reads and validates the external head
before resuming the persisted phase and emits no duplicate mutation or audit
identity.

### Membership and generation rules

- Initial realm creation provisions distinct discovery and capability-control
  channels. Data channels are relationship/set scoped. Application channels
  are created only by the future DR-01 owning workflow.
- Adding a member prepares a new generation and withholds current/past secrets
  from the candidate. It becomes active only after the new generation activates.
- Removing a member prepares revocations and a new random secret. The member is
  omitted from all deliveries. Remaining members install both revocations and
  new generation state.
- Routine rotation changes secret and generation but not membership.
- Renewal uses new grant IDs and a new generation so the bounded sender
  snapshot and receipt attestation advance together.
- Per-channel generations increase monotonically and never wrap. Near
  `uint32` exhaustion, the channel is replaced with a new ChannelID through a
  maintenance migration.
- Discovery membership does not imply data or Application membership. An
  Application channel never reuses realm discovery/data keys.
- Sender snapshots are complete and bounded for that channel generation.
  Unknown or removed sender grants fail closed before replay admission.

### Authority and audit semantics

Successful or denied authority procedures record:

- stable event version, realm-local sequence and random correlation ID;
- operation/delivery ID where applicable;
- Actor Principal, Effective Principal, action and exact resource;
- channel class and generation, never ChannelID or selector;
- prior/new member counts, never membership in metrics;
- stable result/reason, deadline and recovery disposition;
- previous audit hash and current record hash.

The authority mutation, idempotency result and audit outbox append are one
transaction. Diagnostics delivery is at least once under the existing outbox
pattern. The signed checkpoint binds realm, authority epoch, authority
sequence, audit head, active channel generations and backup format version.

## Delivery and data semantics

This packet concerns control delivery, not Application messages.

| Concern | Channel authority contract |
|---|---|
| Ordering | authority sequence and per-channel generation are strictly increasing; older bundles/checkpoints reject |
| Acknowledgement | separate `installed` and `active` attestations from the approved member implementation/host; MAC proves bundle possession, not honest execution |
| Deduplication | request ID, operation ID and delivery retry generation make mutations/imports idempotent |
| Expiry | grant <=30 days; attestation <=30 days; pending operation <=24 hours; envelope rejected after operation deadline |
| Limits | explicit realm/channel/member/delivery/envelope bounds in invariants |
| Backpressure | one pending generation per channel; excess mutations return stable `Conflict`/`ResourceExhausted` |
| Large payload | not applicable; bounded control bundle only; no Content Reference |
| Terminal outcomes | completed, aborted-before-activation, recovery-required, or denied; no ambiguous success |

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
| malformed request/attestation/envelope | `InvalidArgument` without sensitive detail | no mutation except bounded denial audit | do not retry unchanged input | correct input/trust |
| unauthenticated or unauthorized Operator | uniform `Unauthenticated`/`Forbidden` | denial audit where identity is known | no automatic retry | obtain exact grant |
| authority signing/store unavailable | `Unavailable`, authority not ready | prior transaction remains authoritative | bounded retry with same request ID | repair signer/store; never regenerate |
| member unreachable before activation | operation remains `delivering` | pending generation retained | retry same delivery until deadline | restore reachability or abort before activation |
| member unreachable after activation commit | operation becomes `recovery_required` | new generation remains authoritative | roll forward only | restore/update member or fence it |
| receipt lost | status remains unacknowledged | member may already have idempotent install | re-submit same envelope and receipt | no secret export |
| compromised member forges `installed` or `active` receipt without the claimed transition | MAC can verify but is not independent evidence of host behavior | authority records only the member assertion | retry cannot establish truth | fence the member, rotate without it, investigate host/release integrity |
| authority crash at any phase | retry reports durable phase | one state-machine row and audit identity | resume same operation | inspect then resume |
| member crash after install | install receipt/state reloads | bundle and checkpoint durable | activate when checkpoint arrives | restart and inspect |
| member crash after activation before receipt | runtime reloads active checkpoint | active generation durable | regenerate same receipt | re-submit receipt |
| malicious old/replayed bundle | stable rejection | local state unchanged | none | audit and investigate |
| removed member publishes old generation | sender revocation denial before replay | replay ledger not mutated | none | investigate/rotate forward if needed |
| grant approaching expiry | renewal operation | old grant remains until expiry/new activation | same-operation retry | complete renewal before 24-hour boundary |
| monotonic repository unavailable after activation | operation remains `checkpointing`; activation does not roll back | new generation and signed head remain authoritative locally | retry the identical compare-and-append | restore repository availability before another security mutation |
| repository returns missing, lower, forked, or unexpected head / CAS conflict | authority enters `recovery_required` | local state is preserved but same-realm freshness is ambiguous | no blind overwrite or head reset | investigate immutable history; if unique latest head cannot be proven, create a new realm |
| repository is lost or found inside the authority-backup fault domain | same-realm recovery and security mutations are forbidden | existing data-plane grants expire normally | repository must not be recreated from authority DB/archive | establish a new realm and re-enroll |
| authority backup restore with matching latest external checkpoint | authority starts recovery-only, verifies head, then resumes | exact backed-up sequence | no mutations before verification | run restore drill and readiness proof |
| authority backup freshness cannot be proved | old realm stays stopped | no inferred rollback | no retry that resets sequence | create new realm and re-enroll |
| authority key lost | old realm cannot issue | member grants expire normally; no successor trusted | no key regeneration under same Principal | create new realm |
| planned authority-key rotation | maintenance operation | dual-signed transition and new epoch | resume/roll forward | update all trust and rotate all channels |
| node capability store/key lost | member private channels fail closed | authority truth remains | obtain a new delivery for same active member | repair identity continuity, attest new delivery key |
| operation/audit bound exhausted | `ResourceExhausted`, readiness degraded for mutations | no unaudited mutation | retry only after retention/closure action | archive verified audit, close terminal operations |

### Backup and restore

The authority consistency group is separate from every Node group:

- authority ledger/database and audit outbox;
- authority-store encryption key;
- Realm Authority signing key or signer recovery material;
- realm trust/configuration and exact software/schema version;
- signed latest checkpoint and backup manifest.

The monotonic checkpoint repository is an anti-rollback trust root, not merely
backup storage. Its credentials, storage, administrative path and failure
domain are independent of the authority database and every archive that it
validates. It provides a unique head per RealmID, immutable prior-head
retention, and atomic compare-and-append from exact sequence `n` to `n+1`.
A checkpoint signature authenticates content but does not make an old signed
checkpoint fresh; freshness comes from the repository's monotonic head.

Every completed security mutation has successfully appended its signed
checkpoint. Authority backup may be called current only when its sequence and
digest equal the repository head. Restore is into an empty stopped authority
instance. It reads the external head first, then verifies archive integrity,
key/Principal binding, schema, audit chain, exact sequence/digest equality and
immutable predecessor continuity. Repository unavailability, loss,
non-monotonic history, multiple heads, or rollback ambiguity blocks same-realm
restore and mutation. Neither an Operator nor restore code may recreate or
overwrite the repository head from an authority archive. If a unique latest
head cannot be proved, recovery creates a new RealmID and re-enrolls members.

Member backup remains the stopped-Node consistency group in
`persistent-state-security.md`, extended to include the complete channel
generation snapshot and receipts. Partial restore never synthesizes delivery,
receipt, revocation, or generation truth.

### Migration from local realm v2

Migration is a coordinated maintenance boundary:

1. stop all Nodes and take verified authority and Node backups;
2. require one valid `ardents.local-realm/v2` authority and reconcile every
   member record with every Node capability store;
3. create `RealmID`, authority epoch/sequence, transactional authority ledger,
   audit genesis and checkpoint while preserving the issuer Principal;
4. create-if-absent the RealmID genesis head in an empty monotonic repository
   outside the v2 authority/Node backup fault domain; any pre-existing or
   non-empty head fails migration;
5. import only the existing discovery/data channel IDs, secrets, generation and
   signed grants that reconcile exactly; any mismatch fails migration;
6. mark imported local installs as migration evidence, not HPKE receipts;
7. rotate discovery/data to fresh generations through the selected delivery
   and receipt workflow before enabling production multi-host operation;
8. compare-and-append the post-rotation head, verify independent retention,
   then remove shared authority directory mounts and start new-control-plane
   software.

Old software cannot manage or read the new authority schema. Downgrade restores
the complete pre-migration stopped backup. Mixed old/new authority management
is unsupported. During a channel cutover, new software may receive current and
previous private-envelope generations only under the bounded drain rule.

## Security, privacy, and abuse analysis

### Malicious authorized member

A current member knows its channel generation secret and can decrypt that
generation, derive its selector, send signed authorized traffic, and attempt
volume abuse. Channel encryption does not protect other members from it.
Message signature, exact grant permission/scope, sender revocation, replay,
size/rate quotas, and domain authorization remain required.

After removal, the member keeps every old secret and ciphertext it already
obtained. Revocation prevents newly observed signed traffic from being admitted;
rotation prevents access to future generations. Neither claim retroactive
confidentiality.

A member that has decrypted a bundle also knows its `receipt_key`. It can
compute a valid `installed` or `active` MAC without persisting the bundle,
switching runtime state, or making the prior generation receive-only. Receipt
verification therefore authenticates a statement from the approved member
implementation/host; it is not remote attestation or proof of honest execution.
Unexpected binary provenance, host compromise, inconsistent diagnostics, or
other noncompliance invalidates that trust assumption. The deployment owner
must fence the member and rotate without it; repeated receipts cannot repair
the trust failure.

### Malicious or compromised Operator

An Operator with exact membership/rotation authority can change channel
membership but does not receive channel secrets in responses. Two-person
approval is not a v1 protocol feature and must not be implied. Recovery and
authority rotation are non-delegable Product Policy actions. Audit makes the
Actor/Effective mutation attributable but cannot prevent an already-authorized
malicious decision.

### Malicious or compromised checkpoint repository

The monotonic repository is an anti-rollback trust root. A party able to erase
or rewrite both its unique head and immutable history can make an old signed
checkpoint appear current; checkpoint signatures alone do not detect that.
The repository therefore requires separately administered credentials,
append/CAS-only permissions, immutable/WORM retention, deletion protection and
an availability/failure domain independent of authority hosts, databases,
archives and their operators.

Any suspected repository compromise, retention-policy violation, missing
history, non-monotonic response or unexplained CAS conflict invalidates
same-realm freshness. The authority preserves local evidence, stops security
mutations, and does not accept an Operator-supplied head as repair. If
independent immutable history cannot establish one latest head, recovery
creates a new realm.

### Unauthenticated attacker

Requests are bounded before cryptographic work. Unknown realm/channel/member
and policy denial are externally uniform. Attestations, envelopes and receipts
have fixed maximum sizes, versions, deadlines and idempotency identities.
HPKE failures expose one authentication error. Rate limits key on the
authenticated session/operation, not Principal labels in metrics.

### Rollback, replay, and split brain

Authority sequence, channel generation, delivery retry generation and envelope
digest reject control replay. Checkpoint signatures authenticate records but
do not distinguish a valid older record from the latest record. The independent
monotonic repository supplies that anti-rollback truth through unique-head
compare-and-append and immutable history.

Only one pending operation per channel prevents local concurrent generations.
A restored authority must match the repository's unique latest head. A stale
signed head, CAS mismatch, truncated history, repository rollback, or two
authority instances claiming the same RealmID/epoch stops both security
mutation and same-realm restore. Operators may reconcile only from intact
immutable repository history; otherwise they create a new realm.

### Cryptographic boundaries

RFC 9180 HPKE protects the delivery plaintext for the holder of the recipient
private key and binds application-supplied context. It does not supply
transport, retry, replay, acknowledgement, or durable-install semantics; those
are Ardents state-machine obligations. The existing X25519/HKDF-SHA256/
ChaCha20Poly1305 suite and Principal-signed attestation remain selected.

MLS provides asynchronous group key establishment, forward secrecy and
post-compromise security, but adopting it would replace rather than complete
the existing Channel Grant protocol. It is not a drop-in delivery wrapper.

## Observability

Bounded metrics:

- authority readiness and signer/store readiness;
- operation counts by phase, channel class and stable result;
- pending/expired delivery counts;
- generation activation age buckets;
- member/channel count buckets;
- receipt verification failures by stable reason;
- audit outbox depth and oldest age;
- checkpoint/backup age and restore-verification result;
- monotonic repository reachability, CAS outcome and head relation using
  bounded stable labels only.

Metrics never label RealmID, ChannelID, Principal, operation/delivery ID,
endpoint, selector, grant ID, or raw reason text. Diagnostics may show opaque
local operation IDs to an authorized Operator, bounded phase/deadline, class,
generation and recovery action. Health for existing private traffic does not
fail merely because authority mutation is unavailable; authority readiness and
grant-expiry risk are separate signals. A member with a missed activation
checkpoint is not ready for that channel.

Operator workflow:

1. authenticate to the authority and member Node Operator Interfaces;
2. inspect current checkpoint and ensure no channel operation is pending;
3. request the membership/rotation operation with one stable request ID;
4. collect member delivery attestations;
5. deliver HPKE envelopes and submit installed receipts;
6. commit activation, activate survivors and submit active receipts;
7. fence any explicitly excluded/non-acknowledging member;
8. verify readiness, redacted audit and the new signed checkpoint;
9. compare-and-append the checkpoint in the independent monotonic repository
   and only then verify operation completion.

No step prints or copies plaintext Channel Grants.

## Compatibility consequences

- **Wire:** existing `CapabilityGrant`, revocation, private envelope and Waku
  carrier formats remain. New authority/member Operator messages,
  `GenerationBundle`, receipts, activation and checkpoint artifacts are
  strictly versioned additions.
- **Persistence:** new transactional authority schema and member generation
  snapshot/receipt records are required. `authority.json` is not the production
  state store.
- **Configuration:** member Nodes trust exact authority Principal/epoch and
  configure opaque current/previous local references. They do not configure
  channel secrets or selectors.
- **Backup/restore:** authority is a new consistency group with an external
  anti-rollback repository whose monotonic head is outside the authority backup
  fault domain; Node consistency groups gain generation snapshot/receipt state.
- **Rollout:** authority first, then stopped member migration, then fresh
  generation delivery/activation. Old management software is fenced.
- **Downgrade:** only complete stopped pre-migration backup restore.
- **Mixed generation:** current publish plus bounded previous receive is
  allowed; mixed authority-management software is not.
- **Federation:** none. A future federation protocol needs a new ADR, wire
  version, conflict rules and migration.

## External primary sources

Retrieved 2026-07-25:

- RFC 9180, *Hybrid Public Key Encryption*, February 2022,
  https://www.rfc-editor.org/rfc/rfc9180.html. Used for HPKE recipient
  confidentiality, ciphersuite identifiers, context binding, and explicit
  non-goals around replay/loss/application transport.
- RFC 9420, *The Messaging Layer Security (MLS) Protocol*, July 2023,
  https://www.rfc-editor.org/rfc/rfc9420.html. Used only to assess the rejected
  MLS alternative and its asynchronous group/FS/PCS scope.
- Go 1.26 release notes, February 2026,
  https://go.dev/doc/go1.26. Used to confirm that `crypto/hpke` is the standard
  RFC 9180 implementation selected by the existing protocol.
- Go standard library `crypto/hpke`, version `go1.26.4`,
  https://pkg.go.dev/crypto/hpke. Used to confirm the available standard
  one-shot HPKE API and X25519 DHKEM support.

No external source substitutes for Ardents product authority decisions.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit | canonical bundle/receipt/checkpoint vectors; receipt-MAC possession versus claimed-phase distinction; exact action/resource/Delegation matrix; membership/generation state transitions; retry/idempotency; bounds; redaction; rollback rejection | Go unit tests with injected clock/random/store/signer | JUnit/JSON tied to exact commit |
| Contract | Operator and member procedures reject unknown fields and oversized inputs; no Application procedure; stable public errors; artifact compatibility vectors; checkpoint read-head and compare-and-append reject skip, replace, fork and stale expected sequence | generated Connect contracts, repository contract fixture and Go CLI/handler tests | protocol descriptor and vector manifest |
| Integration | two Nodes perform attestation, HPKE delivery, atomic install, activation, sender authorization, revocation-before-replay, restart/resume and audit outbox delivery | tagged integration with real stores/keys | JUnit plus redacted operation transcript |
| E2E | three hosts add, rotate, remove, renew and recover a member through protected Operator paths; no plaintext grants; old generation receive-only then removed | canonical Linux private-LAN environment | scenario JSON/JUnit and packet capture assertions |
| Security | malicious attestation/envelope/receipt, a bundle holder forging both receipt phases, replay, valid-but-old signed checkpoint, deleted/rolled-back/forked repository head, split authority, wrong Principal/realm/channel/generation, revoked sender, Store replay, cardinality/size/rate exhaustion, secret/log scan | Linux `-race`, adversarial and fuzz jobs | security report, fuzz seeds, redaction scan |
| Deployment | authority/member restart at every phase; compromised/noncompliant survivor receipt followed by fencing; authority outage; repository outage/CAS conflict; backup/restore latest and stale-negative; independence of repository and authority-backup failure domains; planned authority rotation; migration/downgrade drill | supported Docker/private multi-host topology | manifests, repository history/head evidence, signed checkpoint hashes, JUnit |
| Release | all above pass once without hidden retry on one clean commit; capability stays Q=no until complete matching evidence | DR-06 clean release gate | immutable evidence index with source/toolchain identity |

Additional required negative assertions:

- removed member cannot decrypt the next generation;
- newly added member cannot decrypt retained previous-generation ciphertext;
- a Node missing activation cannot become private-channel ready;
- a holder of `receipt_key` can forge both receipt phases without performing
  either transition; tests and public/operator wording never call the MAC
  cryptographic proof of durable or runtime state;
- an unapproved, compromised or inconsistent member is fenced despite a valid
  receipt before the authority reports trusted convergence;
- authority reports completion only with attestations from every approved
  survivor or explicit deployment fencing evidence;
- a valid old signed checkpoint, unavailable/deleted head, non-monotonic
  history, CAS mismatch, fork, or repository colocated in the authority-backup
  fault domain prevents same-realm restore and further security mutation;
- restore/migration code cannot create, reset, truncate or blind-write a
  repository head from an authority archive;
- stale authority restore never resurrects removed membership;
- discovery, data, Application and control selectors never collide or reuse
  key material;
- no Waku, log, metric, CLI JSON, audit record, backup manifest or test report
  contains a channel secret, selector, receipt key, or plaintext grant.

## Open questions

None that changes the selected external interface, trust root, persistence
owner, revocation completion rule, backup/restore contract, wire artifacts, or
migration.

Implementation may choose an OS-backed versus hardware-backed signer adapter
and a specific repository product. Both are behind the selected seams. The
repository choice is not open with respect to trust semantics: it must provide
independent-fault-domain immutable retention and atomic monotonic
compare-and-append exactly as specified.

## Decision-register proposals

The integrator should add, after review:

| Proposal | Decision/question | Disposition |
|---|---|---|
| W3-D03-01 | One deployment-owned Realm Authority Principal on a designated authority Node is the sole issuer for one non-federated realm. | accept via ADR-0011 |
| W3-D03-02 | Operator-only HPKE generation bundles plus installed/active receipts are the production delivery workflow; Application Interface authority is forbidden. | accept via ADR-0011 |
| W3-D03-03 | Every membership change rotates the affected channel; removal completes only after approved-host activation attestations or deployment fencing. A receipt MAC proves possession, not honest persistence/runtime behavior. | accept via ADR-0011 |
| W3-D03-04 | Discovery, data, Application and capability-control channels have separate IDs/secrets/generations/membership/replay/audit. | accept via ADR-0011 |
| W3-D03-05 | A monotonic append/CAS repository outside the authority-backup fault domain is the anti-rollback trust root. Missing, non-monotonic, ambiguous or mismatching head blocks mutation/same-realm restore; otherwise create a new realm. | accept via ADR-0011 |
| W3-D03-06 | Federation and MLS are rejected for v1. Planned authority rotation is dual-signed; lost-key recovery creates a new realm. | accept via ADR-0011 |

## Recommendation

Write ADR before implementation.

ADR-0011 records the trust, control artifacts, transactional persistence,
revocation completion, recovery and migration contract. After it is accepted,
implement the following dependency-ordered vertical slices. Do not publish
issue files until the maintainer approves their granularity.

## Vertically sliced implementation issues

### CGA-01: Create and inspect a production realm authority

- User story: as a Realm Operator, I create or reopen one realm on a designated
  authority Node and inspect its redacted trust/checkpoint status.
- Complete behavior: add exact Operator actions/resources, deep authority
  ledger, external signer seam, monotonic checkpoint repository seam and
  genesis CAS, audit genesis/outbox, bounds and `Inspect`; compose only on
  Operator Interface.
- Acceptance criteria: Actor/Effective authorization; Application absence;
  one durable realm/epoch/sequence; repository create-if-absent and exact CAS;
  key/Principal/head mismatch and corrupt state fail closed; metrics/audit
  redact secrets; restart validates the same external head.
- Blocked by: accepted ADR-0011.
- Research class after packet: implementation-ready R0 with security review.

### CGA-02: Deliver and acknowledge one initial generation

- User story: as a member Node Operator, I attest the local delivery key, import
  an HPKE generation bundle, and return a receipt without seeing plaintext
  grant material.
- Complete behavior: authority `Deliver/Acknowledge`, member
  `PrepareDelivery/InstallDelivery`, complete sender snapshot, receipt MAC,
  atomic install, idempotent retry and bounded artifacts.
- Acceptance criteria: subject/realm/epoch/sequence binding; RFC 9180 vectors;
  wrong/tampered/replayed/expired artifacts fail; crash after commit regenerates
  the same receipt; a malicious holder can forge an asserted phase and the
  contract identifies it only as a trusted-host attestation; no secret output.
- Blocked by: CGA-01.
- Research class after packet: implementation-ready R0/R1 cryptographic
  integration.

### CGA-03: Rotate a channel and attest activation across hosts

- User story: as a Realm Operator, I rotate one channel and obtain protocol
  attestations that every approved member switched while the previous
  generation is receive-only for a bounded drain.
- Complete behavior: `Rotate`, activation checkpoint/envelope, active receipt,
  current/previous/pending member state, readiness and phase-resume behavior.
- Acceptance criteria: new selector/key; never publish old after activation;
  member missing checkpoint not ready; authority crash at every transition
  resumes; suspect/noncompliant member is fenced even with a valid MAC; only one
  pending generation; roll-forward after activation.
- Blocked by: CGA-02.
- Research class after packet: implementation-ready R1 due multi-host lifecycle.

### CGA-04: Add/remove membership with revocation and fencing

- User story: as a Realm Operator, I add or remove a Principal and obtain a
  truthful terminal result under success, partition, timeout and malicious old
  traffic.
- Complete behavior: fresh generation for add/remove, signed revocations,
  candidate/active/suspended/removed state, survivor receipts, explicit
  deployment fencing evidence and uniform recovery.
- Acceptance criteria: add has no old key; removed sender fails before replay;
  removed member has no next secret; unacknowledged survivor prevents
  completion unless fenced; a receipt from an unapproved/suspect survivor never
  substitutes for fencing; exact audit attribution and bounds.
- Blocked by: CGA-03; DR-04 supplies the supported fencing/reachability
  procedure before production acceptance.
- Research class after packet: implementation-ready R1 with adversarial E2E.

### CGA-05: Renew grants and separate channel classes

- User story: as a Realm Operator, I renew discovery, relationship-scoped data,
  and future Application channels independently without cross-class authority
  or key reuse.
- Complete behavior: 30-day grants, 24-hour renewal threshold, distinct
  channel-class constructors/policy, sender snapshot renewal, class-specific
  status and audit.
- Acceptance criteria: cross-scope/cross-channel use denied; selector/key/replay
  separation proven; expiry degrades only affected channel; bounds enforced.
- Blocked by: CGA-04; DR-01 owns creation/membership policy for
  `channel.application`, not the authority lifecycle.
- Research class after packet: implementation-ready R0.

### CGA-06: Backup, restore, recovery and local-v2 migration

- User story: as a deployment Operator, I restore the latest authority or
  migrate the local realm without resurrecting revoked authority.
- Complete behavior: authority consistency-group archive, signed external
  checkpoint, independent monotonic repository adapter, exact-head/CAS and
  immutable-history verification, recovery-only startup, planned dual-signed
  authority transition, new-realm lost-key/repository-loss path, v2 importer
  and downgrade drill.
- Acceptance criteria: latest restore preserves sequence/revocations; stale or
  missing/rolled-back/forked repository head fails; old signed checkpoint is
  insufficient; partial restore never regenerates keys or repository head;
  repository/authority backups have independent failure domains; imported
  state reconciles every member; fresh post-migration rotation and CAS required;
  old manager fenced.
- Blocked by: CGA-05 and deployment backup/checkpoint adapter.
- Research class after packet: implementation-ready R1 due migration/recovery.

### CGA-07: Qualify the authority lifecycle

- User story: as a release reviewer, I receive one matching-commit evidence set
  for production authority and private multi-host use.
- Complete behavior: execute the full acceptance matrix on the support topology,
  retain evidence and reconcile the capability catalogue without premature
  promotion.
- Acceptance criteria: no hidden retries; clean exact commit; unit/contract/
  integration/E2E/security/deployment/release evidence complete; external
  source/toolchain versions retained.
- Blocked by: CGA-06, accepted DR-04 compatibility, DR-06 scope.
- Research class after packet: R3 qualification.

## Cross-stage dependencies

- **DR-01 may rely on:** one authority per realm; generic Application-channel
  authority lifecycle; fresh generation on membership change; HPKE delivery;
  installed/active receipts; no federation; explicit bounds. DR-01 still owns
  conversation identity, addressing, group policy and message semantics.
- **DR-04 must provide:** protected Operator reachability to the authority and
  each member, fencing of a non-acknowledging Node, authority/backup placement,
  independent monotonic checkpoint-repository placement/availability,
  immutable retention, clock bounds, failure-domain and upgrade order. A
  topology that cannot fence a stale or suspect member cannot claim completed
  revocation.
- **Operations must extend:** stopped consistency groups, restore verification,
  incident response and migration/downgrade procedures for authority state.
- **Identity/Access must provide:** exact actions/resources,
  Actor/Effective/Delegation attribution, idempotency and audit outbox.
- **Network privacy remains owner of:** selector/key derivation,
  private-envelope generation overlap, sender authorization and replay order.
- **DR-06 owns:** clean matching-commit qualification and any `Q` promotion.
