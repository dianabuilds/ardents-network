# Ardents Private Messaging Protocol V1

## 1. Status And Scope

This document is the implementation contract for private Ardents product
traffic over the canonical Waku foundation. It refines, and does not replace:

- `canonical-network-foundation.md`;
- `network-privacy-requirements.md`;
- `persistent-state-security.md`;
- the applicable identity, policy, discovery, publication, data, and network
  product requirements.

Protocol name: `ardents-private/1`.

The contract covers Waku Relay, Store, Filter, and Lightpush. TCP and TCP-WSS
remain carrier families below Waku and do not change this protocol. QUIC is not
enabled by this protocol.

## 2. Security Boundary And Non-Claims

The confidentiality boundary is a private channel capability. A holder of the
current channel secret can derive its selector and decrypt its envelopes.
Endpoint knowledge, Waku peer identity, carrier topic access, or retained
ciphertext alone cannot do so.

The protocol hides from non-holders:

- message class and domain operation;
- principal, owner, service, conversation, blob, request, and record meaning;
- payload schema and plaintext;
- filter/lightpush interest meaning.

It does not claim to hide:

- participation in Waku or Ardents;
- source/destination peer observations available to a directly connected peer;
- timing, volume, carrier topic, opaque-selector equality, padded size bucket,
  Store retention time, or network topology;
- meaning from an authorized or compromised current capability holder.

Padding reduces exact-size leakage but is not traffic-flow anonymity. Defense
profiles, rate controls, and future cover traffic may reduce correlation; they
must never be represented as complete anonymity.

## 3. Capability Model

### 3.1 Channel Capability

A channel capability is a signed, recipient-bound grant containing:

| Field | Rule |
| --- | --- |
| `format_version` | exactly `1` |
| `channel_id` | random 128-bit local identifier; never placed on Waku |
| `generation` | unsigned 32-bit key generation, starting at `1` |
| `channel_secret` | 32 cryptographically random bytes |
| `grant_id` | random 128-bit identifier |
| `issuer_principal` | Ardents principal authorized to grant this channel |
| `subject_principal` | the only local principal allowed to use this grant |
| `permissions` | subset of `subscribe`, `publish`, `store_fetch`, `filter`, `lightpush`, `delegate` |
| `scope` | one of the scopes in section 3.2 |
| `not_before` / `not_after` | inclusive UTC validity interval |
| `issuer_signature` | Ed25519 signature over deterministic grant bytes excluding secret delivery wrapping |

The grant signature provides authority provenance. The channel secret is a
group confidentiality secret, not proof of an individual sender. Each private
message is also signed by its Ardents identity and carries its `grant_id`
inside ciphertext so receivers can enforce subject, permission, and revocation.

### 3.2 Initial Scopes

- `realm.discovery`: node presence, service publication, withdrawal, and
  discovery-fed blob/source announcements inside one provisioned realm.
- `data.exchange`: request/response access for one data-sharing relationship or
  set, never a global all-blobs selector.
- `channel.application`: reserved for future application/conversation messages;
  it is not implemented merely by accepting arbitrary payloads.
- `realm.capability_control`: signed grant, revocation, and rotation control for
  an issuer realm. Access to this scope does not itself grant data-channel
  decryption.

Scopes are local authorization data and remain encrypted during delivery. One
network-visible selector is derived per channel generation; message classes are
multiplexed inside that lane rather than encoded into separate readable topics.

### 3.3 Issuance And Delivery

The realm's first `channel.issue` trusted Principal and discovery capability are provisioned through
a stopped-node, operator-authenticated secret import. A peer endpoint alone is
never sufficient bootstrap authority.

The supported `local-multinode` deployment implements this workflow with a
real deployment-local issuer. The issuer private key and channel authority
state are retained in a dedicated Docker volume, separate from node data. Each
stopped node receives a unique subject-bound signed grant in its encrypted
Identity-owned channel grant store plus distinct store/replay keys. Re-running the
workflow is idempotent and reuses the same authority and grant identities. The
local issuer is not trusted by production profiles and is never exported as a
public or Ardents-operated realm authority.

After a recipient already has an authorized private control channel, online
grant delivery over that channel must additionally use HPKE as specified by RFC
9180 with a dedicated recipient X25519 encryption key attested by the
recipient's Ed25519 Ardents identity. The required suite is:

- KEM: `DHKEM(X25519, HKDF-SHA256)`;
- KDF: `HKDF-SHA256`;
- AEAD: `ChaCha20Poly1305`.

The encryption key is distinct from the Ed25519 signing key. Go 1.26's standard
`crypto/hpke` implementation is the selected RFC 9180 implementation; online
delivery must use `hpke.DHKEM(ecdh.X25519())`, `hpke.HKDFSHA256()`, and
`hpke.ChaCha20Poly1305()`. No third-party or handwritten HPKE and no
Ed25519-to-X25519 conversion is allowed. The new key must be added to the
persistent-state inventory before online grant delivery ships.

Before an issuer seals a grant, the recipient publishes a delivery-key
attestation containing protocol version, recipient principal, Ed25519 identity
public key, X25519 HPKE public key, whole-second `not_before`/`not_after`, and an
Ed25519 signature over all fields. The attestation lifetime is at most 30 days.
The issuer derives the principal from the identity public key, verifies the
signature and validity, and requires it to equal the grant subject. A bare or
expired X25519 public key is not a valid delivery target.

HPKE does not create initial trust and does not make a public recipient inbox a
private selector. A recipient's first grant always requires explicit secure
operator provisioning or another already-authorized private relationship.
Until online grant delivery is implemented and tested, only explicit secure
operator provisioning is valid. Plaintext grant export, logging, CLI output,
and general API return values are forbidden.

### 3.4 Storage And Resolution

Capability grant/secret storage and subject binding belong to Identity, not
Network Foundation. Policy admits or denies each requested use based on scope,
permission, revocation, and operator rules. Capability material must be
encrypted at rest or held by an OS-backed secret store, separated from retained
Waku/blob payloads, backed up as secret material, and resolved by opaque local
reference. The transport facade receives only resolved operation-specific key
material and may not persist a second copy.

The channel-grant-store master key is a separately provisioned 32-byte deployment
secret. Identity derives independent store-encryption and local-reference keys
with HKDF-SHA256 domain separation. The master key is never written into
`ardents.db`; a missing/wrong key makes the authenticated capability ledger
unrecoverable and must fail closed.

Diagnostics may expose scope name, generation, validity state, issuer
fingerprint, and a non-reversible local reference. It must not expose channel
IDs, grant IDs, selectors, channel secrets, HPKE ciphertext/plaintext, or raw
recipient keys.

### 3.5 Rotation, Revocation, And Recovery

Routine selector rotation increments `generation` and derives a new generation
key. This changes network correlation identifiers but does not remove an
existing holder, because that holder still has the channel secret.

Revoking a member requires both:

1. a signed revocation of that member's `grant_id`, causing receivers to reject
   its newly signed messages; and
2. a fresh random `channel_secret` distributed under a new generation to every
   remaining member, preventing the revoked member from deriving future
   selectors or decrypting future traffic.

At activation, publishers use only the new generation. Subscribers may retain
the previous opaque selector for at most `maximum envelope lifetime + clock
skew` to drain already valid ciphertext; they never publish on it. Old retained
ciphertext cannot be made secret from a former holder who already possessed the
old key.

A lost channel secret is unrecoverable from Waku Store or blob ciphertext.
Recovery requires restoring the protected capability backup or obtaining a new
grant/new generation from an issuer. A partial grant/secret store fails closed
and may not derive a replacement secret.

## 4. Key Schedule And Opaque Selector

All integer encodings in derivation inputs are unsigned big-endian. Lengths are
explicitly prefixed with unsigned 16-bit values; concatenation without lengths
is forbidden.

For a channel capability:

```text
generation_key = HKDF-SHA256(
  input_key_material = channel_secret,
  salt = SHA-256("ardents-private/1" || channel_id),
  info = "generation-key" || uint32(generation),
  length = 32)

selector_key = HKDF-SHA256(
  input_key_material = generation_key,
  salt = nil,
  info = "selector-key",
  length = 32)

envelope_key = HKDF-SHA256(
  input_key_material = generation_key,
  salt = nil,
  info = "envelope-key",
  length = 32)

selector_token = first_20_bytes(HMAC-SHA256(
  selector_key,
  "channel-selector"))
```

The Waku pubsub carrier remains `/waku/2/default-waku/proto`. The Waku content
topic is exactly:

```text
/ardents/1/<lowercase-base32-no-padding(selector_token)>/proto
```

The token is 32 characters. No owner, principal, operation, service, blob,
request ID, message class, or capability ID may be appended. Filter and
Lightpush use this same complete opaque content topic. Selector derivation is
deterministic only for an authorized holder of the current channel secret.

## 5. Outer Envelope

The network-visible envelope is fixed binary framing, not JSON or protobuf.
Multi-byte integers use big-endian order.

| Offset order | Size | Field | Rule |
| --- | ---: | --- | --- |
| 1 | 4 | magic | ASCII `ARDP` |
| 2 | 1 | version | `1` |
| 3 | 1 | cipher suite | `1` = XChaCha20-Poly1305 + HKDF-SHA256 |
| 4 | 2 | flags | zero in v1; unknown bits reject |
| 5 | 4 | generation | capability generation |
| 6 | 8 | issued at | UTC Unix seconds |
| 7 | 8 | expires at | UTC Unix seconds |
| 8 | 16 | message ID | cryptographically random, unique within generation |
| 9 | 24 | nonce | cryptographically random XChaCha20 nonce |
| 10 | 4 | ciphertext length | exact following byte count |
| 11 | variable | ciphertext | protected inner message plus 16-byte AEAD tag |

The fixed header is 72 bytes including ciphertext length. The message ID and
nonce are independently generated; neither is derived from time, sender,
payload, request ID, or selector.

Encryption uses `golang.org/x/crypto/chacha20poly1305.NewX(envelope_key)`. No
custom cipher construction is allowed.

Associated data is the exact concatenation of:

1. bytes from magic through ciphertext length;
2. a uint16 length and UTF-8 pubsub carrier topic;
3. a uint16 length and UTF-8 opaque content topic.

This binds the envelope to version, generation, lifetime, ID, nonce, size, and
both Waku routing labels. Moving ciphertext between selectors or carrier topics
must fail authentication.

## 6. Inner Message

The decrypted inner message uses deterministic protobuf serialization with the
following logical fields:

```text
PrivateMessageV1 {
  uint32 protocol_version = 1;        // value must equal 1
  MessageClass message_class = 2;
  bytes grant_id = 3;                 // exactly 16 bytes
  string sender_principal = 4;
  bytes sender_public_key = 5;        // Ed25519, exactly 32 bytes
  uint32 payload_version = 6;
  bytes payload = 7;
  bytes signature = 8;                // Ed25519, exactly 64 bytes
  bytes padding = 9;
}
```

The signature input is SHA-256 of the domain separator
`ardents-private-message-signature/1`, the complete outer header through
ciphertext length with the length set to zero for signing, and deterministic
protobuf bytes with `signature` and `padding` empty. The sender principal must
derive from `sender_public_key`; the grant must be valid for that principal,
class/scope, publish permission, generation, and issued-at time.

Grant validity is evaluated at signed `issued_at`, while revocation is also
evaluated at the receiver's observed time. Once a signed revocation is
effective, a sender cannot bypass it by backdating a newly created envelope;
first delivery of that grant is rejected even if its claimed `issued_at`
precedes revocation.

Initial `MessageClass` values:

| Value | Class | Owning semantics | Required scope | Default lifetime |
| ---: | --- | --- | --- | ---: |
| 1 | `DISCOVERY_RECORD` | Discovery record carrying node presence, service publication, withdrawal, or blob/source announcement; Publication owns local intent/outcome | `realm.discovery` | 15 minutes |
| 2 | `BLOB_FETCH_REQUEST` | Data Substrate transfer request | `data.exchange` | 2 minutes |
| 3 | `BLOB_FETCH_RESPONSE` | Data Substrate success or terminal denial response | `data.exchange` | 2 minutes |
| 4 | `CAPABILITY_CONTROL` | signed grant/revocation/rotation control | `realm.capability_control` | 15 minutes |
| 5 | `BLOB_REPLICA_CONTROL` | Data Substrate capacity observation, reserve, accept/reject, commit, and commit acknowledgement | `data.exchange` | 2 minutes |

`BLOB_FETCH_REQUEST` and `BLOB_FETCH_RESPONSE` carry a signed optional
`resource_kind`. The absent/default value means `blob` for the original small
Blob path; `manifest` carries a deterministic encrypted-content Manifest over
the same capability-scoped private selector. The discriminator and complete
Manifest body are included in the inner node-identity signature.

Replica-control payloads additionally carry a node-identity Ed25519 signature
binding action, operation ID, source, target, public key, and body. A receiver
silently ignores a well-routed payload addressed to another node, but the named
target verifies the complete inner binding and fails closed. Capacity responses
are target-signed, operation-bound observations; they are selection hints only
and never count as reservations or committed replicas.

Network Foundation owns framing and delivery only. It does not interpret the
domain payload beyond class/scope admission. Discovery continues to validate
record signatures/freshness/trust. Data Substrate continues to validate blob,
requester, source, transfer, encryption, and retention truth.

## 7. Size, Padding, Time, And Replay

### 7.1 Size And Padding

- Maximum unpadded inner protobuf bytes excluding `padding`: 128 KiB.
- Padding buckets for the complete inner protobuf are 1, 4, 16, 64, and
  128 KiB. The sender selects the smallest bucket that fits.
- Payloads that cannot fit the 128 KiB bucket are rejected; future large-object
  transfer must use Data Substrate chunking, not oversized Waku messages.
- Maximum complete outer envelope: 132 KiB. Any larger envelope is rejected
  before decryption.
- Receivers verify that padding is all zero and the decoded message occupies
  the declared bucket; arbitrary padding is rejected.

### 7.2 Time

- `expires_at` must be greater than `issued_at`.
- Maximum protocol lifetime is 30 days; each message class applies the shorter
  default/policy limit from section 6.
- Future clock skew greater than 5 minutes is rejected.
- Expired envelopes are rejected before domain delivery but only after bounded
  framing checks.
- Implementations use UTC Unix seconds and an injectable clock in tests.

### 7.3 Replay

Authentication occurs before a message ID is admitted to the replay ledger so
unauthenticated traffic cannot poison it. The durable replay key is a local
digest of channel reference, generation, and message ID; raw selector and
capability secrets are not stored.

The digest is an HMAC-SHA256 value under an HKDF-separated 32-byte local replay
master key supplied independently from retained Waku data. The master key is
not stored in the replay ledger. A missing persistent path, missing/wrong key,
corrupt ledger, or persistence failure rejects admission fail-closed; an
in-memory fallback is forbidden.

An authenticated duplicate is rejected until its envelope expiry. Entries are
pruned after expiry. The ledger has a configured per-channel and global bound;
when a bound is reached it rejects new messages fail-closed and reports
`privacy.replay.capacity_exhausted` rather than evicting unexpired entries and
allowing replay. Store fetch and live Relay delivery share this ledger, so a
retained duplicate is not re-applied after restart.

## 8. Receive Order And Error Taxonomy

Receive processing order is fixed:

1. bound outer byte length;
2. parse fixed framing and reject unknown flags/version/suite;
3. validate selector syntax, generation, lengths, and coarse time limits;
4. resolve a local capability without exposing whether resolution succeeded;
5. authenticate/decrypt with associated data;
6. reject authenticated replay;
7. decode bounded deterministic protobuf and padding;
8. validate principal, identity signature, grant, scope, permission, validity,
   and revocation;
9. deliver to the owning domain and record terminal outcome.

Stable internal reason codes:

- `privacy.envelope.malformed`
- `privacy.envelope.oversized`
- `privacy.envelope.version_unsupported`
- `privacy.envelope.suite_unsupported`
- `privacy.envelope.flags_unsupported`
- `privacy.envelope.time_invalid`
- `privacy.envelope.expired`
- `privacy.envelope.authentication_failed`
- `privacy.envelope.replayed`
- `privacy.envelope.signature_invalid`
- `privacy.envelope.sender_unauthorized`
- `privacy.channel_grant.missing`
- `privacy.channel_grant.not_yet_valid`
- `privacy.channel_grant.expired`
- `privacy.channel_grant.revoked`
- `privacy.channel_grant.scope_denied`
- `privacy.channel_grant.issuer_untrusted`
- `privacy.channel_grant.invalid`
- `privacy.replay.capacity_exhausted`
- `privacy.migration.legacy_rejected`

Remote responses and unauthenticated diagnostics collapse selector/key/grant
failures to a generic rejection. Operator diagnostics may expose reason code,
message class only after successful decryption, count, profile, and recovery
action. They may not expose raw message bytes, selector, channel/grant IDs,
principal for rejected unauthenticated traffic, nonce, keys, signatures, or
decryption detail.

Waku's upstream structured logs are not an Ardents diagnostics surface because
they may attach raw content topics and message fields. The embedded Waku logger
must therefore write to a compatible discard sink. Ardents-owned transport
status, health signals, publish outcomes, and bounded privacy reason counters
are the only permitted operator-facing network truth.

## 9. Waku Role Mapping And Threats

| Waku role | Protocol use | Observable to role/operator | Required control |
| --- | --- | --- | --- |
| Relay | live publish/subscribe of outer envelopes | peer participation, generic carrier, opaque selector equality, time, padded size | encrypt before publish; authenticate after receive; rate/size bounds |
| Store | retained private envelopes and offline query by opaque selector | selector equality, store time, expiry, ciphertext, query peer | no plaintext labels/payload; expiry/replay checks after fetch |
| Filter | constrained subscription to opaque content topic | subscriber peer and stable opaque interest | same capability-derived selector; never class/owner/service labels |
| Lightpush | constrained publication of outer envelope | publisher peer, opaque selector, time/size | protect before submission; no plaintext request metadata |

An unauthorized Store/Relay/Filter/Lightpush operator cannot derive channel
meaning or decrypt. A compromised current capability holder can observe and
decrypt its channel and can attempt validly encrypted abuse; identity
signature, grant permission, policy, rate control, revocation ledger, and secret
rotation limit that case. Compromise of a relay endpoint alone grants no
selector derivation authority.

## 10. Versioning And Migration

### 10.1 No Downgrade

Version `1` is the only accepted version. Unsupported versions/suites are
terminal rejections. There is no negotiation that falls back to plaintext,
readable topics, an older private version, or unsigned messages. A future
protocol version uses a separately authorized selector generation and an
explicit encrypted migration window.

### 10.2 Hard Cut From Technical Alpha

The current readable topics are legacy and forbidden after migration:

- `ardents/1/discovery-record`;
- `ardents/1/blob-request`;
- `ardents/1/blob-response` and requester/request-ID-derived suffixes.

Migration is a coordinated maintenance boundary:

1. create protected backups and stop publication/fetch workers;
2. provision and validate required capabilities on every authorized node;
3. stop old nodes; mixed old/new publication is unsupported;
4. upgrade all nodes and preserve local domain state;
5. start private-protocol nodes, re-publish fresh discovery/publication truth
   only through opaque selectors, and verify raw Waku captures;
6. keep old Waku Store records out of all queries and remove them under an
   explicit retention/purge operation.

New nodes do not subscribe to, query, decode, bridge, or dual-publish legacy
topics. If a required capability is missing, local domain truth remains local,
publication/fetch is denied, and diagnostics reports
`privacy.channel_grant.missing`; plaintext is never used to preserve availability.
Previously exposed plaintext cannot be made confidential retroactively.

Rolling interoperability with technical-alpha nodes is deliberately rejected.
Availability during migration is achieved through a scheduled cutover and
verified backups, not a privacy downgrade.

The product concept and operational vocabulary are **Channel Grant**. The
existing cryptographic `CapabilityGrant` type, `CAPABILITY_CONTROL` wire
identifier, `realm.capability_control` purpose, canonical/domain-separated
bytes, and persisted ledger format remain unchanged until a separately
versioned protocol migration.

## 11. Acceptance Requirements

Implementation is incomplete until tests prove:

- deterministic selector/HKDF vectors and cross-node interoperability;
- endpoint-only and wrong-scope callers cannot derive selectors;
- raw Relay, Store, Filter, and Lightpush observations contain none of the
  tested principal/service/blob/request/class/payload semantics;
- tamper, topic relocation, replay (including restart), expiry, future time,
  wrong key, wrong generation, revoked grant, wrong principal, unknown flags,
  version/suite, malformed protobuf/padding, and all size limits fail explicitly;
- generation rotation changes selectors; revocation secret rotation excludes
  the revoked holder; overlap subscribes but never publishes old generation;
- no logs, diagnostics, API results, or test reports reveal protected material;
- legacy readable topics and fallback paths are absent from production use;
- real multi-node Waku Relay/Store/Filter/Lightpush flows retain truthful
  readiness and delivery behavior.
