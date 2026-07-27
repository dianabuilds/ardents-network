# ADR 0011: Single-authority Channel Grant lifecycle

- Status: Proposed
- Date: 2026-07-25
- Decision owners: Identity, Security, Operations
- Research: `docs/engineering/research/channel-grant-authority.md`

## Context

Ardents already has signed subject-bound Channel Grants, purpose-scoped issuer
trust, encrypted Identity-owned grant storage, retained sender grants and
revocations, recipient-attested RFC 9180 HPKE delivery primitives, generation-
derived opaque selectors, and authorization-before-replay admission.

The supported local provisioning path is not a production authority. One
process with direct access to a shared authority directory and stopped Node
directories creates a deployment-local issuer, static generation-1 discovery
and data secrets, and imports every member grant. It has no authenticated
production Operator surface, cross-host delivery acknowledgement, member
removal, generation activation, interrupted-operation recovery, authority
backup freshness proof, or federation rule.

Production authority changes the trust root, persisted truth, control
artifacts, revocation completion, backup/restore and migration contracts. Those
decisions must precede implementation and DR-01 Application Messaging.

## Decision

### Trust and ownership

One deployment-owned Realm Authority Principal is the sole Channel Grant issuer
for one v1 realm. A designated authority Node hosts the authority module and
its transactional ledger, but the Realm Authority Principal is distinct from
the authority Node Principal, every member Principal, Waku Peer ID, and
transport identity.

The authority is reachable only through exact procedures on the protected
Operator Interface. Existing Actor/Effective Principal, Access Grant, Product
Policy, idempotency and audit rules apply. Version 1 accepts only a directly
authenticated Operator Principal for authority procedures: `Actor` equals
`Effective`, and Delegation is rejected as Operator call authority, consistent
with ADR-0002's Principal-to-Application-only Delegation and interface-bound
admission contract. Audit retains both Actor and Effective attribution even
though they are equal. A future delegated authority procedure requires a new
identity/access decision and Application/Operator interface review.

The authority action catalogue is:

| Action | Resource | Delegation |
|---|---|---|
| `realm.channel.membership.change` | exact `realm/<RealmID>/channel/<ChannelID>` | rejected; direct Operator only |
| `realm.channel.generation.rotate` | exact `realm/<RealmID>/channel/<ChannelID>` | rejected; direct Operator only |
| `realm.channel.delivery.issue` | exact `realm/<RealmID>/operation/<OperationID>/delivery/<DeliveryID>` | rejected; direct Operator only |
| `realm.channel.delivery.acknowledge` | exact `realm/<RealmID>/operation/<OperationID>/delivery/<DeliveryID>` | rejected; direct Operator only |
| `realm.channel.activation.commit` | exact `realm/<RealmID>/operation/<OperationID>` | rejected; direct Operator only |
| `realm.channel.audit.read` | exact `realm/<RealmID>` or `realm/<RealmID>/channel/<ChannelID>` | rejected; direct Operator only |
| `realm.channel.recovery.execute` | exact `realm/<RealmID>` | rejected; non-delegable in Product Policy |
| `realm.authority.rotate` | exact `realm/<RealmID>` | rejected; non-delegable in Product Policy |

This direct-only column supersedes the provisional one-hop Delegation column in
the linked research packet; it does not change that packet's action names or
resource separation. Admission intersects the authenticated Operator's current
Access Grants, exact action/resource and Product Policy before mutation or
secret-bearing output. Sibling actions and parent, child or wildcard resources
do not imply authority. The Application Interface exposes no authority
procedure and no channel secret, selector, Waku operation, authority topology or
authority cryptographic material.

The first release has one active authority Principal and one realm per
authority instance. It has no federation, transitive trust, threshold authority
or Ardents-operated public issuer. Planned authority rotation requires a
predecessor- and successor-signed transition, increments authority epoch and
rotates all channels. Loss of the old authority key creates a new realm rather
than silently assigning a successor.

### Channel separation and membership

Discovery, data, Application and capability-control channels have distinct
ChannelIDs, random secrets, generations, membership, replay state and audit
records. Discovery membership grants no data or Application membership.
Capability-control access grants no payload-channel decryption.

Every membership addition or removal creates a fresh random channel secret and
next generation. Removal additionally creates signed revocations for every
removed current grant. Routine rotation also creates a fresh secret and next
generation without changing membership. A new member receives no old
generation. A removed member cannot receive the new generation, but Ardents
does not claim to erase old ciphertext or secrets it already held.

Renewal uses new grant IDs, a fresh random channel secret and the next
generation without changing membership. The renewed subject grant, complete
bounded sender-grant snapshot, revocations, installed/active receipt
attestations and activation checkpoint advance through the same durable
delivery, activation and external-checkpoint operation. The prior grant remains
authoritative only until its expiry or the new generation activates, whichever
comes first; no same-generation sender-snapshot update is supported.

After activation, publishers use only the current generation. Subscribers may
keep the immediately previous generation receive-only for the existing bounded
private-envelope drain window. Generation activation never rolls back; repair
rolls forward.

### Delivery and acknowledgement

Member Nodes produce a Principal-signed, finite delivery-key attestation for
their persistent X25519 key. The authority seals a complete, bounded,
authority-signed `GenerationBundle` using the existing RFC 9180
X25519/HKDF-SHA256/ChaCha20Poly1305 suite. The bundle contains the subject
grant, bounded sender-grant snapshot, revocations, authority sequence,
activation contract, drain deadline and an independent receipt key. Plaintext
grants are never returned, logged, exported or copied between hosts.

The approved member implementation atomically validates and installs the
complete bundle before it returns an opaque receipt MAC. It emits an `active`
receipt only after the runtime adopts the signed activation checkpoint and
makes the prior generation receive-only. Receipts are bound to delivery ID,
envelope digest, authority sequence, channel and generation.

The MAC cryptographically proves only that its producer possessed the delivered
`receipt_key` and binds the producer's asserted phase. It does not prove durable
installation or runtime behavior against a compromised or deliberately
noncompliant recipient, because that recipient can compute either receipt after
decrypting the bundle. Installed/active receipts are therefore protocol
attestations trusted only from the approved released member implementation on a
deployment-controlled host. A modified, compromised, inconsistent or otherwise
unapproved member must be fenced even when its receipt MAC verifies.

An authority operation progresses durably through request, preparation,
delivery, installation, activation commit, activation, external checkpoint
retention and completion. It may abort only before activation commit. Retry
uses one request/operation identity; restart resumes the durable phase. Failure
to retain a post-activation checkpoint leaves the operation active but
incomplete and blocks another security mutation; it never rolls back.

Removal cannot be instantaneous across a partition. The authority reports a
membership change complete only after every surviving approved member host has
returned an active attestation or the deployment owner has provided explicit
fencing evidence for a non-acknowledging or untrusted member. A member without
the current activation checkpoint is not private-channel ready and must update
before publication, Store query or domain delivery resumes. Receipt retry
cannot repair a broken member-host trust assumption.

### Persistence, audit and recovery

The authority ledger is the sole membership/generation truth. One transaction
commits the mutation, idempotency result, delivery/receipt state, audit outbox,
monotonic authority sequence and hash-chain head. Raw authority/channel/receipt
secrets are encrypted under separately provisioned authority-store keys; the
Realm Authority signing key is held through a narrow external signer seam.

The authority consistency group contains the authority ledger/audit outbox,
store key, signing key or signer recovery material, realm configuration,
software/schema version, signed checkpoint and backup manifest.

A separately administered monotonic checkpoint repository is the anti-rollback
trust root. A signature authenticates a checkpoint but cannot distinguish a
valid old checkpoint from the latest one. The repository therefore resides
outside the authority database/archive/credential fault domain and provides:

- one unique latest head per RealmID;
- immutable retention of every accepted predecessor;
- atomic compare-and-append from the exact expected sequence to its successor;
- read of the unique current head;
- create-if-absent only for a new random RealmID during stopped genesis.

It provides no delete, replace, truncate, blind-put or last-writer-wins path.
Repository credentials and administration are separate from authority hosts,
databases, archives and their operators; immutable/WORM retention and deletion
protection are required. Its availability and compromise fault domain is
outside the complete authority-backup fault domain. Compromise that can rewrite
both head and retained history defeats anti-rollback, so suspected compromise
requires a new realm.
Every security mutation reaches completion only after its signed checkpoint is
accepted by compare-and-append. Repository unavailability after activation
leaves the operation active but `checkpointing` and blocks another security
mutation. A missing, lower, forked, non-monotonic or unexpected head/CAS result
moves the authority to recovery-required and also blocks mutation.

Restore into an empty stopped authority reads the repository head first and may
resume the same realm only when archive sequence/digest and immutable
predecessor history equal that unique head. Repository loss, rollback
ambiguity, non-monotonic history, multiple heads, inability to read the head,
or placement inside the authority-backup fault domain prevents same-realm
restore. Restore code never recreates or overwrites the head from an authority
archive. If freshness cannot be proved, the old realm remains stopped and
recovery creates a new realm.

Member backup extends the stopped-Node consistency group with complete channel
generation snapshot and receipt state. Partial restore never regenerates or
infers grants, delivery, receipts, revocations or generation truth.

### Bounds and migration

V1 supports one realm per authority instance, at most 256 members, 1,024 active
channels, 256 members per channel, one pending and one receive-only previous
generation per channel, four outstanding deliveries per member/channel, and a
256 KiB delivery envelope. Grants and delivery attestations are valid for at
most 30 days; renewal begins at least 24 hours before grant expiry. Pending
operations expire within 24 hours. Audit retention is bounded and exhaustion
fails authority mutations closed.

Migration from `ardents.local-realm/v2` is a stopped maintenance operation. It
reconciles every authority/member record and Node capability store, imports the
existing issuer and exact discovery/data grant material into the new
transactional schema, and creates the signed genesis head with repository
create-if-absent in a fault domain independent of all v2 backups. A pre-existing
or non-empty head fails migration. Migration then requires a fresh generation
rotation, member attestations/fencing, and successful compare-and-append before
multi-host use. Shared authority directory mounts and old authority management
are removed. Downgrade restores the complete stopped pre-migration backup;
mixed old/new authority management is unsupported.

## Considered options

### Deployment CLI and shared authority files

Rejected for production. It is close to the local provisioning implementation
but equates filesystem possession with realm administration, requires unsafe
cross-host ceremony, has no protocol-bound member phase attestation, and makes
concurrent mutation, audit and recovery ambiguous.

### Node-local issuers with control gossip

Rejected. Several issuers cannot provide one membership/revocation truth during
partition without introducing a consensus or conflict-resolution protocol.
Issuer compromise and stale restore would have realm-wide ambiguous outcomes.

### MLS per channel

Rejected for v1. RFC 9420 supplies a substantially different asynchronous group
key and epoch protocol with forward secrecy and post-compromise security.
Adoption would replace the existing Channel Grant, retained sender-grant,
selector, envelope and persistence assumptions rather than complete the
missing production authority lifecycle.

## Consequences

- DR-01 can treat Application-channel grant lifecycle as a deep authority
  dependency while retaining ownership of addressing, group policy and
  Messaging API semantics.
- DR-04 must support protected authority/member Operator reachability, explicit
  Node fencing, authority/backup placement, an independently administered
  monotonic checkpoint repository, immutable retention and bounded clocks. A
  topology without fencing cannot claim completed revocation.
- Existing private traffic can continue during authority outage until grant
  expiry. Membership, rotation, renewal and recovery cannot.
- HPKE protects delivery but does not provide replay, retry, acknowledgement or
  durable installation; the Ardents state machine owns those semantics.
- Revocation convergence is explicit. Partitioned, stale, suspect or
  noncompliant members are unavailable/fenced rather than silently accepted.
- Receipt MACs are trusted-host protocol attestations, not cryptographic remote
  attestation. A member with the receipt key can forge its asserted phase.
- The monotonic checkpoint repository is a separate anti-rollback trust root.
  Its loss or ambiguous history requires a new realm rather than restoring an
  old signed checkpoint.
- The existing `CapabilityGrant`, revocation and private-envelope wire formats
  remain; authority control artifacts and persistence are versioned additions.
- The authority becomes a new highly protected operational component and
  backup consistency group.
- `realm.channel-grant-authority` remains `Q=no` until the complete
  matching-commit unit, contract, integration, E2E, security, deployment and
  release matrix passes.
