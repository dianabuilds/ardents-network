# Transit Grant acquisition

Status: **accepted contract; signer and Endpoint acquisition lifecycle implemented.**
This document owns the purpose-scoped signer, fixed encrypted outcome, durable
issuer budget/idempotency, and Endpoint at-most-once acquisition lifecycle
selected by [ADR-0062](../adr/0062-scope-online-transit-grant-signing.md).

## Authority boundary

Authenticated Network State selects one `transit-issuance` Node and binds its
Node-signed issuer profile. The current profile declares exactly one Transit
Grant signing public key. Its private key belongs only to that online duty:

```text
offline State authority --authenticates--> Epoch / issuer profile
                                            |
                                            v
Endpoint -- Entry --> Initiator -- OHTTP --> purpose-scoped issuer
                                            |
                                            v
                                  Transit Grant v1 only
```

The online signer does not receive a State/Epoch private key and its public key
is absent from the Epoch authority set. State verifiers therefore cannot accept
its signatures as State. Its only maintained signing operation accepts the
closed Transit Grant tuple and uses the existing Transit Grant v1 signature
domain. It has no arbitrary byte-signing interface.

The profile remains opaque to `internal/network/state`; the credential owner
verifies its Node signature and extracts the one State-bound Grant signer.
Endpoint, Initiator, Introduction, and Responder receive only the narrow
projection required for their own check. No caller supplies an alternate
signer, issuer URL, candidate ordering, or fallback.

## Fixed request and outcome

One request contains only:

- Request ID;
- Network ID, Epoch, and State digest;
- Descriptor-selected Introduction Node and fixed Introduction role;
- fresh attachment ID and TLS client-public-key digest; and
- whole-second Grant expiry within the current State/duty deadline.

It contains no Name, Target, Publication, Descriptor, Publisher material,
Entry Invite, stable participant identifier, complete Route, literal endpoint,
or Service Administration input. The complete plaintext is padded to the one
fixed credential message size before OHTTP encapsulation.

The versioned response has exactly four fixed-size encrypted outcomes:

| Outcome | Meaning | Payload |
|---|---|---|
| `issued` | This exact Request ID and tuple own one committed result. | One Transit Grant v1. |
| `exhausted` | The current duty has no unreserved budget unit. | None. |
| `withdrawn` | The authenticated issuer duty no longer permits new work. | None. |
| `unavailable` | The request cannot safely produce or disclose another result. | None. |

HTTP status, body length, and relay framing do not vary by an authenticated
outcome. Signing, storage, scheduling, and transport can still produce timing
variation; this contract makes no timing-privacy claim. A failure before authenticated OHTTP decoding is only
local `unavailable`; the Endpoint never infers `exhausted` or `withdrawn` from
cleartext transport behavior.

## Issuer-owned durable lifecycle

The issuer opens one exclusive owner-only root for one exact duty identity:
Network ID, Epoch/digest, issuer Node, Grant signer, assignment deadline, and
budget generation. The initialized finite budget is immutable for that duty.
Opening a missing, corrupt, substituted, rolled-back, or scope-mismatched root
fails closed.

For a valid request, the issuer serializes this transition:

1. If Request ID exists with a different request digest, return
   `unavailable`.
2. If it exists with a committed result, return the same result bytes.
3. If the duty is withdrawn, return `withdrawn`; if no unit remains, return
   `exhausted`.
4. Atomically reserve one unit and persist Request ID, request digest, and a
   fresh Grant ID before signing.
5. Sign the exact Transit Grant v1 and atomically commit its bytes.
6. On restart, finish an incomplete reservation with the same persisted Grant
   ID and deterministic Ed25519 signature; never reserve another unit.

Only accepted reservations occupy the bounded ledger, so its entry ceiling is
the duty budget. Terminal duty cleanup may erase the ledger only after its
authenticated deadline and operational evidence retention requirement. It is
not carried into a successor State duty.

## Endpoint-owned at-most-once lifecycle

The Endpoint protects at most one acquisition per in-flight operation in a
separate exclusive owner-only root within its protected state profile. The durable record contains the exact
Request ID/tuple, State-duty identity, one-use TLS private key, phase, and
terminal class; it contains no Target or Route plan.

```text
absent -> pending -> ready -> presenting -> spent
             |        |          |
             +--------+----------+-> exhausted / withdrawn / unavailable / burned
```

- `pending` may resend only the byte-identical Request ID and tuple to reconcile
  an interrupted exchange.
- `ready` owns one verified Grant/key pair and may begin exactly one receiving-
  Node presentation.
- entering `presenting` is durable. Any completion or ambiguity becomes
  `spent` or `burned`; it never returns to `ready`.
- expiry, State successor, withdrawal, invalid response, local corruption, or
  explicit cancellation erases the key and ends the attempt.
- the Endpoint does not automatically replay publish, open, withdraw, or
  Application bytes. A later explicit operation uses a fresh Request ID and
  consumes another budget unit only when the caller still has authority.

The receiving Node's existing durable Grant-ID replay ledger remains the final
at-most-once admission check. Issuer idempotency and Endpoint lifecycle do not
replace it.

## Product and failure boundary

Acquisition is an Endpoint operation below the local Application Interface.
Connection and Service Administration callers request their product operation;
they do not receive a Grant, private TLS key, Entry contact, State view, issuer
profile, or Route plan. Browser and Desktop Adapters cannot invoke custody or
administration through the Connection surface.

The honest availability bound is the intersection of current authenticated
State, one current Entry, the selected Initiator and issuer duties, remaining
global issuer budget, and the exact operation deadline. Withholding,
withdrawal, saturation, expiry, corruption, and State conflict fail visibly
with no DNS, clearnet, stale-State, alternate-Node, operator-plan, or Browser
fallback.

## Implementation and evidence order

1. Version and test the issuer profile and request/outcome codecs while keeping
   Transit Grant v1 unchanged.
2. Implement the exclusive durable issuer budget/idempotency state and its
   crash/reopen matrix.
3. Project the purpose-scoped signer through current State to Endpoint and
   receiving Node verification.
4. Endpoint pending/reconcile/present/burn state is implemented; terminal
   Application Interface diagnostics remain part of the headless composition.
5. Compose that owner into the headless publish/open/withdraw journey before
   artifact-native qualification.

Ordinary deterministic/process gates are required during implementation. VPS,
soak, hostile-load, platform-matrix, Browser, and release qualification remain
deferred until the final candidate freeze.

## Non-claims

This contract does not provide anonymity, unlinkable membership, a per-user or
Sybil-resistant quota, independent operation, censorship resistance,
availability, public enrollment, or a permissionless resource market. A
selected Initiator and issuer can collude on timing and can deny or consume the
finite project-operated alpha budget. Encryption hides the request fields from
the Initiator under the stated non-collusion condition; it does not hide that
an Endpoint used that Initiator or that the issuer served a request.
