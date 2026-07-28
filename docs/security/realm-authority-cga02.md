# Realm Authority CGA-02 Security Contract

## Scope

CGA-02 adds one recipient-bound initial-generation journey. The protected
Operator Interface exposes Authority `IssueInitialGeneration` and
`AcknowledgeInitialGeneration`, plus member-Node
`PrepareGenerationDelivery` and `InstallGenerationDelivery`. There is no
Application Interface equivalent.

The exact actions are:

- `realm.channel.delivery.issue` and
  `realm.channel.delivery.acknowledge` on
  `realm/<RealmID>/operation/<OperationID>/delivery/<DeliveryID>`;
- `realm.channel.delivery.prepare` on the exact local Principal;
- `realm.channel.delivery.install` on the exact operation/delivery resource
  carried by the authenticated envelope binding.

All four mutations require direct Actor/Effective equality. Authority issue and
acknowledgement additionally pass
`policy.disable_realm_channel_delivery`.

A clean recipient Node may start in delivery-only mode with
`privacy.delivery_enabled=true`. That mode requires the encrypted
channel-grant store, its protected key, the local subject Principal and
`channel.issue` trust, but deliberately does not require an already-installed
discovery/data grant or replay key. After initial installation, operators can
enable `privacy.required=true` with the installed channel references and replay
material.

## Cryptographic contract

The member persists a distinct X25519 delivery key in its encrypted Identity
capability ledger and returns a finite Principal-signed attestation. The
Authority signs the complete generation bundle and seals it with the RFC 9180
base-mode suite DHKEM(X25519, HKDF-SHA256), HKDF-SHA256 and
ChaCha20Poly1305. `info` commits the versioned delivery binding.

The bundle and installed receipt bind the Realm, Authority Principal and epoch,
Authority sequence, operation and delivery IDs, channel ID and class,
generation, recipient Principal, delivery-key digest, envelope digest and
expiry. The retained RFC 9180 Appendix A.2 known-answer test verifies this
exact HPKE suite independently of Ardents serialization.

The opaque receipt uses HMAC-SHA256 under the per-delivery receipt key. It is
only evidence that a holder of approved-host material produced an assertion.
A malicious holder can forge an `installed` or later phase; it is not proof of
durable persistence or honest runtime behavior. Authority state accepts only
the CGA-02 `installed` phase and rechecks every envelope binding.

## Atomicity, replay and crash recovery

Member install validates the entire signed snapshot, commits subject/sender
grants, revocations and the stable receipt in one encrypted-store transaction,
then returns the receipt. A crash after that commit replays the identical
receipt and cannot install a second generation.

Authority issue and acknowledgement commit ledger, audit outbox, idempotency,
sequence and signed checkpoint intent before compare-and-append. Restart
reconciles either declared crash boundary against the retained repository head.
The same request, operation, delivery, envelope and receipt identities are
reused; conflicting request-ID payload reuse fails closed.

## Limits and disclosure

Attestations and artifacts are versioned, reject unknown fields and expire
within 30 days. Operations expire within 24 hours. Envelopes are limited to
256 KiB; sender/revocation snapshots and ledger collections retain their
declared bounds.

The CLI moves attestation, ciphertext and receipt only through bounded private
files created without replacement. It deletes the consumed attestation after
issue and deletes delivery/receipt files after acknowledgement. Plaintext
Channel Grants, selectors, receipt keys and private endpoints are absent from
CLI output, diagnostics, logs and metrics. Status exposes only bounded counts,
phase, sequence, generation and stable reasons.

Rotation, activation, membership mutation, fencing, renewal, restore and
qualification remain outside CGA-02. Qualification remains `Q=no`.
