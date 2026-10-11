---
status: accepted
date: 2026-08-27
supersedes: none
---

# ADR-0050 — Keep closed-alpha release seeds in separate local custody

## Context

The first bounded H4 alpha needs a real TUF Release input and separately rooted
H4-6A disclosure/component inputs. `internal/release` deliberately verifies
those inputs but cannot sign or administer a repository. Test fixtures create
ephemeral keys and are prohibited from becoming release authority. The Product
Owner selected an external local Windows workspace as the first alpha custodian;
the project VPS is not a key store.

The existing Authority Vault uses a fixed Argon2id/AES-GCM profile, but its
records are specifically Name Authority material and it may expose only bounded
Namespace signing operations. Reusing it for TUF or H4-6A keys would collapse
unrelated authority and violate both modules' interfaces.

## Decision

`internal/release/custody` owns one local
`ardents-release-seed-envelope-v1` record. Its only initial interface is
`Initialize`: at an existing Product Owner-owned directory it asks a trusted
interactive adapter for a new passphrase and confirmation, creates exactly ten
independent Ed25519 private keys, and atomically publishes one encrypted record.
It returns only the ciphertext digest and the ten public keys.

The record uses the accepted 256 MiB, three-pass, four-lane Argon2id v1.3
profile, a fresh 16-byte salt, AES-256-GCM with a fresh random nonce, and
canonical header bytes as AEAD associated data. The fixed role inventory is five
TUF top-level keys for the maintained 3-of-5 Release profile, one disclosure
key, three H4-6A component keys, and one H4-4 corpus-authority key. It never
replaces an existing record, exposes a decrypted seed/password/derived key, or
offers an arbitrary signing callback.

`cmd/ardents-release-custody initialize` is the only current terminal adapter.
It accepts the record root as a path and reads the passphrase only from a real
no-echo terminal. It rejects arguments, environment variables, configuration
files, and shared stdin as secret sources.

## Consequences

- The Product Owner can initialize real alpha signing material locally without
  placing a key in the repository, GitHub, CI, bundle, or VPS.
- Release Decision, Endpoint, Update, and the H4-6A reader remain verifier or
  runtime owners; none gain key access.
- Initializing seeds does not itself create signed metadata, publish an
  artifact, invite a participant, or claim independent/threshold control.
- A future exact metadata-signing operation must be a separate bounded module
  and operation; it cannot widen this custody interface into a generic signer.
- Losing the passphrase makes this one-person provisional alpha custody
  unrecoverable. A later multi-device, hardware-backed, unattended, or threshold
  release operation requires a new decision and qualification.

## Compliance

- [R-119](../research/records/r-119-closed-alpha-release-signing-operation.md)
- [ADR-0015](0015-separate-release-decision-from-local-activation.md)
- [ADR-0021](0021-use-password-derived-authority-custody.md)
- `internal/release/custody`
- `cmd/ardents-release-custody`
