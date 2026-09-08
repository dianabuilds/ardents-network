# Private admission in the closed successor

Status: **closed-network design contract** for the
[selected workload](../product/protected-service-workload.md).
Public entitlement and autonomous issuer eligibility remain R-149.
Use the [common architecture](common-privacy-architecture.md) and
[protected forwarding contract](protected-route-protocol.md).

## Actual authority and provisioned inputs

The existing independently pinned closed Network identity, State authority and
time-witness verification remain in force. The operator provisions one current
issuer duty, a finite set of receiving duties and offline issuance permissions.
The issuer never possesses State, Name, Service Authority or Release keys.
A Source distributes evidence; it does not grant permission or accept State.

The closed State authenticates a profile digest and the exact issuer Node,
issuer Ed25519 duty key, common token keys, duty generation, time window and
receiver/resource classes. There is no client-selected issuer URL or trust root.
State, enrollment and time verification retain their existing byte formats and
root/freshness/conflict checks. State verifies the
[closed profile](protected-route-protocol.md#authenticated-closed-profile);
the credential owner consumes only its authenticated admission projection.
The signature authority and complete profile grammar are defined there.

An offline permission has a fresh random 32-byte permission ID, Network,
issuer-duty generation, a separate holder Ed25519 public key, a UTC hourly
window and a u32 count for each resource class. The operator signs its
canonical bytes using the separately authenticated closed admission authority.
The holder key is issuer-facing only, never a User/Persona/transport identity.
The permission and key stay with the Endpoint owner; no Application obtains
them. Each belongs to exactly one Isolation Context or Publisher role and one
hour. Generate independent permission IDs and holder keys for every such pair;
never reuse a holder across contexts or send an installation identifier to the
issuer. Provisioning is an explicit closed-network operation, not public signup.

A User installation receives at most 4,096 token reservations per hour in
total across its context-scoped permissions; a Publisher receives at most
16,384 across its publication roles. The offline provisioning owner allocates
these finite maxima before signing, records its issuance allocation durably,
and refuses overlapping allocations that exceed them. Its private provisioning
record can associate its own recipients: blindness does not hide that record
from this authority. It is not transmitted to issuers or receivers.
The issuer duty itself permits at most
65,536 reservations per hour across all permissions. Counters and atomic
debits are shared by all its listeners. Issuance right, rather than traffic
receipts or the count of identities, is the closed admission basis.
No amount of issued tokens grants voting weight or public network influence.

## Provisioning and key lifecycle

Use the existing closed-control, Custody, Instance and issuer command owners;
add bounded commands there, not a general signer or a new provisioning daemon.
The offline operator first supplies the independently authorized current Epoch,
Node Records and public issuer keys. The closed State signing boundary signs
only the exact verified profile for that Network/Epoch. If its required signing
authority is unavailable, provisioning is unavailable; generating another key
cannot match or replace the enrollment pin.

Custody owns a separate closed-admission authority record and signs only a
sealed permission-allocation request prepared by the credential owner. This is
an explicit new purpose under its existing encrypted, owner-controlled authority
boundary, distinct from State, Name, Service and Release records. Bind the
purpose, Network and authority public key in the protected record. The request
fixes the issuer/duty, holder, interval and class maxima; Custody exposes no
generic raw-signing API or private key. Extend its package/import ownership
only with the implementing architecture change and boundary tests.

Endpoint generates the holder key and exports only its public permission
request plus a fresh request digest for explicit offline approval. Keep holder
private keys and local context associations volatile. Worker loss does not
erase the surviving Endpoint's context; Endpoint loss does. After that loss,
a new context needs a fresh permission within the operator's remaining hourly
allocation. Neither that crash nor an unused permission refunds a signed
allocation or an issuer debit.

Issuer initialization generates dedicated class/window RSA keys in its fresh,
exclusive owner-only issuer root and exports only exact public SPKI/profile
inputs. It receives no admission-authority private key. Preserve its durable
reservation and receiving ledgers with atomic write/flush/reopen before
acknowledgement. Provision successor hourly keys before use; never replace
bytes under an already signed key/window or reuse a key across cohorts.
No password, private key or secret permission appears in arguments, environment,
logs, clipboard-style diagnostics or a public output. Use the established
interactive Custody unlock and owner-only private input/output paths.
Operator actions are explicit, finite closed-network provisioning, not public
signup, a test-only validity callback or an administrative naming mechanism.

The public issuer-key profile is `ARDCIP01`[8], Network[32], issuer Node[32],
not-before u64, not-after u64, key-count u16, then sorted window-start u64,
class u8, SPKI-length u16 and exact RSA-PSS SPKI bytes, followed by a Node
Ed25519 signature under `ardents-closed-issuer-keys-v1\0`. It contains every
class 1–3 for every hour in its one-to-six-hour interval. Its owner-only root
retains corresponding PKCS#1 RSA private keys under an immutable root marker;
reopening requires the exact Network, Node, signer and interval. State still
accepts token keys only through the subsequently State-signed `ARDCPR03`
profile, never by trusting this profile as a replacement State authority.

## Canonical signed permission

All integers are unsigned big-endian. The signing input is the ASCII domain
`ardents-issuance-permission-v1\0`, followed by Network[32],
issuer-Node[32], duty-generation u64, permission-ID[32], holder-key[32],
not-before u64, not-after u64 and three u32 maxima in class order 1,2,3.
The signature is Ed25519[64]. The complete permission is 228 bytes, excluding the constant signing domain.
Its interval is exactly one aligned UTC hour and lies inside the duty's
authenticated validity. Reject unknown classes, zero identities, overflow,
noncanonical size, wrong Network/duty or unsupported public key.

The Endpoint prepares the sealed public allocation request as `ARDPAR01`[8],
admission-authority-key[32], Network[32], issuer-Node[32], duty-generation
u64, permission-ID[32], holder-key[32], not-before u64, not-after u64, three
u32 maxima, local allocation role u8 (`User=1`, `Publisher=2`) and holder
Ed25519[64]. The holder signs the ASCII domain
`ardents-admission-allocation-v1\0` followed by every preceding field. The
complete request is exactly 269 bytes. Custody accepts only the current aligned
UTC hour, independently confirms its SHA-256 before unlock, verifies this
holder proof and returns a permission only from its separate encrypted
admission root. One atomically replaced encrypted allocation-ledger envelope
holds the current authority successor and exact request digests; its monotonic
floor is flushed only after the envelope is verified. This retains the complete
allowed hourly reservation set without turning the vault's bounded record count
into a smaller quota. An interrupted replacement is recovered only when it is
the exact next floor; a restored older envelope is refused. An identical retry
returns the same permission, while a changed body for that permission ID fails.
This is a fixed allocation operation, never a raw-signing interface.

This is a separately scoped permission signature, not a signature on a token.
The authority key is independently identified in the signed closed issuer
profile. Its compromise invalidates admission scarcity; honest receivers'
local budgets still apply. It cannot authorize State, custody or naming.

## Selected cryptographic use

Use CIRCL v1.6.5 `blindsign/blindrsa`,
`SHA384PSSDeterministic`: RFC 9474 blind RSA with the RFC 9578 type-2
token construction. A token has 354 bytes; an individual blinded request has
259 bytes; its blinded signature has 256 bytes. This is the binary construction
inside Ardents's authenticated framing, not implementation of the RFC's HTTP
challenge/discovery endpoints. HTTP headers and public DNS are not required.

RSA keys are 2,048-bit, two-prime keys with exponent 65,537, generated by
Go's `crypto/rsa.GenerateKey` and checked with `Validate` by the operator.
They are dedicated to this use. Reject nil, nonpositive/even/incorrect-size
moduli, wrong exponent, wrong algorithm parameters or malformed DER before
calling the library. Require each blinded input integer and returned signature
to be strictly between zero and the modulus, with exactly 256 bytes.

The key ID is SHA-256 of the exact admitted RFC 9578 RSA-PSS SPKI bytes.
The Ardents canonical encoding uses SHA-384 and MGF1/SHA-384 identifiers with NULL hash parameters,
explicit 48-byte salt and omitted default trailer field. The selected DER
encoding has 346 bytes for these keys and is pinned byte-for-byte in State.
A generic `x509.MarshalPKIXPublicKey` RSA encoding uses a different algorithm
identifier and MUST NOT supply this key ID. Parse and re-encode against the
selected grammar; do not accept alternative DER representations as the same
admitted key. This exact encoding is Ardents's choice: the RFC's example omits
the hash NULL parameters and consequently has a different encoded length.
Independent RFC-vector parsing must preserve the vector's original SPKI/key ID;
never normalize an already signed token into Ardents's key identifier.

The RFC challenge uses token type 2. Its issuer name is
`i-` + unpadded lowercase base32(SHA-256(issuer Node ID)) + `.invalid`.
Its single origin name uses `n-` and the receiving Node ID by the same rule.
These are public role labels inside the challenge, never DNS/SNI destinations.
Its 32-byte redemption context is SHA-256 over the domain
`ardents-admission-context-v1\0`, Network[32], profile-digest[32],
receiver-Node[32], receiver-duty-generation u64, class u8 and window-start u64.
The client constructs this exact shared challenge from authenticated facts;
the receiver accepts no personalized challenge. Each token uses a new
32-byte CSPRNG nonce. No Target, Name, join secret or local context ID appears.

Each hourly class has one common RSA key for all holders and receivers.
The full key ID and window in Ardents framing disambiguate the RFC request's
truncated key ID. There is no probing of alternative keys after failure.
The current and next hourly configurations may be published together;
acceptance is confined to the exact token window and receiver duty.

## Resource classes and admission

| Class | Permission conferred by one token | Receiver-local restrictions |
|---|---|---|
| 1 Control | One confidential role operation, at most 64 KiB of framed request/response bytes and 30 seconds | No Application Data or arbitrary forwarding destination |
| 2 Forward | One authenticated forwarding channel or data-join terminal lane, at most 32 MiB of receiver-accounted ingress plus egress and 1,800 seconds | A forwarding channel permits 256 concurrent work lanes plus two reserved control lanes; all child work remains charged to the same parent reserve |
| 3 Publication | One Introduction registration for at most 600 seconds and 1 MiB of capsule/control traffic | One slot, at most 16 pending capsules, delivery rate at most 4/s |

These are work maxima, not bytes the implementation should generate.
A fresh class-2 token can replenish the same live lane's byte reserve before
exhaustion, within its original deadline and every aggregate parent/host limit.
It cannot resurrect a closed lane, extend its time or change its peer.
Only actual work can request replenishment.

The token's hourly window bounds redemption, not an extension of a live lease.
At successful redemption fix the lease end to the earliest of admission plus
the class lifetime, the caller's deadline and current duty/profile/work-safety
terminal bounds. Later byte replenishment cannot extend that end. A token
presented outside its window is rejected even if a previously admitted lane is
still live. This separates spend-ledger expiry from the lifetime of already
authorized work. The longer class-2 lease allows a complete ten-minute network
workload after setup; the text Application's own job deadline remains 600 s.

A receiving duty has at most 1,024 live role channels and 1,024 child lanes,
64 MiB queued ciphertext and
65,536 spent-token entries per hour. Control and pre-admission work use separate
smaller counters so an Application stream cannot consume termination capacity.
Rate-limit signature verification to 128/s per duty and 4 concurrent checks;
excess gets a bounded refusal before crypto. Any stricter host allowance wins.
These finite design maxima allow the retained 256-Connection Publisher workload
and its paired Rendezvous sides; they do not prove useful Node capacity.
Allocate buffers on demand under aggregate limits, never one full queue per
declared maximum. Qualification must demonstrate the required workload.
A compromised issuer may sign arbitrarily many tokens. These receiver limits
bound honest local work; they do not guarantee fair availability or detect
overissuance. Token holders can still exhaust permitted shared capacity.

## Issuance, ambiguity and spending

One issuance operation carries the permission, a fresh 32-byte request ID,
class/window, 1 through 32 exact blinded requests and a holder signature.
The holder signs the domain `ardents-issuance-request-v1\0`,
permission-ID[32], request-ID[32], class u8, window-start u64,
count u16 and SHA-256(concatenated blinded requests).
A maximum of 32 means a count of tokens, not 32 unbounded nested requests.

The issuer first validates the permission, signature, exact body and current
duty. Under one exclusive durable ledger transaction, it either returns an
existing byte-identical committed result for the same ID/digest, or reserves
the complete batch against both permission and duty counters. A changed digest
for that ID is unavailable. Invalid requests occupy no permanent ledger slot.
A crash after reservation completes only that reservation on restart.
No partial success/refund creates additional quota.

Issuer responses have the same 16 KiB encrypted plaintext shape for issued,
exhausted, withdrawn and unavailable results. The client verifies every
finalized token before exposing it to admission. Failure ends the batch;
there is no automatic switch of issuer or key.

CIRCL's blinding State is private, opaque and not serializable. Keep it in
volatile Endpoint-owned memory. A same-process retry may reconcile the exact
pending request while that state exists. After a process crash, a pending
reservation is burned locally; do not recreate blinding randomness, serialize
library internals or reserve the same right again under another ID secretly.
Ready token stock is also volatile and scoped; losing unused stock is a finite
availability cost. The issuer's durable debit remains authoritative.

Before presenting a token, the Endpoint durably marks its SHA-256, receiver,
window/class and attempt as potentially spent. It then presents it only inside
the intended authenticated TLS channel. The receiver validates the challenge
and signature, durably inserts the token nonce/key/context tuple into its
exclusive spend ledger, reserves capacity, and only then enables forwarding.
Bind the accepted operation to that channel's TLS exporter and its local lane
nonce. A TLS exporter binding is not a new client identity.

Receiver rejection after spend, lost acknowledgement, cancellation or crash
burns that token. Duplicate presentations fail even on another connection.
Replacing a lane uses a fresh token and the existing bounded Connection
continuity rules; no Application-operation replay follows.
A failed disk write or ambiguous ledger ownership means unavailable.

Do not replicate a receiving duty behind independent ledgers. A restored
snapshot cannot resume an old duty; explicit fresh State/duty/key generation
and retained authority floors are required. Complete storage/time-authority
rollback lies outside the surviving-boundary claim. Entries may be removed
only after the authenticated acceptance window and its 60-second cleanup
margin have ended; expiry never authorizes reuse.

## Bootstrap without a token cycle

The same protected forwarding supports a narrowly typed bootstrap allowance.
It can reach only current public evidence or the one current issuer, never a
private resolver, Introduction submission, data Rendezvous or arbitrary IP.
At each hop allow at most 128 KiB and 10 seconds per bootstrap lane, 4 live
lanes per adjacent connection, 16 total per duty and 256 KiB queued work.
The duty's aggregate bootstrap output ceiling is 1 MiB/minute with a 128 KiB
burst. Fresh source addresses or keys cannot enlarge that aggregate ceiling.

The issuer requires the offline permission before expensive signing.
The client may upgrade a live target-free prefix with appropriate one-use
forwarding tokens before any private operation. Issuance success alone does
not authorize use of a bootstrap-only lane for private traffic.
This bounds honest resource consumption under a flood; it cannot ensure
availability against an adversary that monopolizes the bounded allowance.

## Retention and public extension

Permission IDs, holder proofs, request IDs and blinded requests belong only to
issuance. Spend nonces belong only to the receiving duty. Neither enters State,
agreement, telemetry or a public log. Ordinary diagnostics retain bounded
reason/counter classes without these identifiers or traffic histories.
Detailed synthetic traces belong only to the test evidence directory.

A public successor must replace offline permissions and appointed issuer/State
eligibility through an accepted R-149 contract with real scarcity/freshness
semantics. Keep the private request/Target/Connection facts out of that contract.
Changing those semantics requires a new authenticated generation and review;
the closed permission format cannot become a permanent public administrator.

Evidence and limitations are in the
[R-152 contract research](../research/records/r-152-closed-scheme-contract.md).
