---
status: accepted
date: 2026-08-27
supersedes: none
---

# ADR-0051 — Confirm local release-seed custody through a public receipt only

## Context

ADR-0050 creates the first-alpha encrypted seed record and reports its public
role keys at initialization. That one-time receipt can be lost before the
release/control companions are recorded. R-120 needs a way to confirm that the
Product Owner still holds the intended encrypted record without treating the
record as a generic local signer or exporting its private material.

## Decision

`internal/release/custody` adds one `Inspect` operation. The Product Owner
supplies an existing root already protected as owner-only by the local
platform; this Go boundary does not certify Windows ACL ownership. It accepts a
trusted interactive secret adapter and, after the record authenticates and its
fixed role inventory validates, returns the same public receipt as
initialization: ciphertext SHA-256 and the ten fixed Ed25519 public keys. It
writes nothing and rejects a missing, non-regular, or symlinked record as
observed before reading, malformed envelope, invalid passphrase, or invalid
fixed inventory.

`cmd/ardents-release-custody inspect --root ABSOLUTE_OWNER_ONLY_DIRECTORY` is
the only adapter. It uses the same local password dialog on Windows and no-echo
terminal adapter elsewhere. The command has no output path, signing request,
metadata input, arbitrary byte input, network input, upload target, or private
key export.

## Consequences

- The Product Owner can retain and independently record the public TUF,
  disclosure, component, and corpus-authority companions for the already
  created record.
- The operation confirms local custody but creates no TUF metadata, H4-6A
  statement, alpha corpus, bundle, publication, participant invitation, or
  release acceptance claim.
- A future fixed alpha-input construction/signing operation remains a separate
  R-120/ADR decision and must still require recorded Network State and
  two-builder release evidence.

## Compliance

- [ADR-0050](0050-separate-local-release-seed-custody.md)
- [R-120](../research/records/r-120-bounded-alpha-input-signing.md)
- `internal/release/custody`
- `cmd/ardents-release-custody`
