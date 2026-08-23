---
status: accepted
date: 2026-08-20
---

# ADR-0021 — Use password-derived Authority Custody

## Context

Stage 7 must protect live Authority Vault records and portable Recovery Bundles
consistently on desktop and headless Ubuntu and Windows. Platform-account
keychains would introduce different availability, recovery, dependency, and
prompt semantics, while automatic root unlock is unnecessary for ordinary
runtime work performed through bounded credentials.

## Decision

Both Vault records and Recovery Bundles use canonical
`ardents-authority-envelope-v1` with distinct authenticated purposes and
separately entered passwords. The envelope derives a 32-byte key with Argon2id
v1.3 through `golang.org/x/crypto v0.55.0` using `256 MiB`, `t=3`, `p=4`, a fresh
16-byte salt, and then encrypts with Go 1.26 AES-256-GCM and a fresh random
nonce. A v1 Vault contains at most 1024 independently atomic encrypted records
and 1 GiB under one Vault password; it has no in-place password rotation.

The Vault is locked after every restart and every custody operation requires
explicit bounded secret input. Repair, update, rollback, and ordinary uninstall
preserve encrypted Vaults and monotonic floors; explicit purge removes only
declared owned copies and does not claim secure deletion. Restore enters
`authority-locked` export-only state and cannot reactivate Authority until a
strictly higher authenticated successor is accepted with a fresh runtime
Instance Key and separately reissued Grants.

DPAPI, Secret Service, automatic unlock, password escrow, cloud/help-desk
recovery, first-party cryptography, cgo, and `unsafe` are not selected for
Authority Custody.

## Consequences

- Losing a Vault or Bundle password is unrecoverable by Ardents; unattended root
  signing is unavailable in Stage 7.
- Password strength bounds offline-guess resistance. Endpoint compromise,
  snapshots, swap/crash dumps, guaranteed zeroization, and secure deletion
  remain outside the claim.
- Current Docker/Windows measurements accept the format and development profile
  but do not satisfy weakest-native-host performance or unavailable durability
  qualification. A falsifying later result reopens this decision rather than
  changing the fixed band after measurement.
- Any future OS keychain, hardware token, dedicated signer, automatic unlock,
  multi-device custody, or password rotation is a new consequential decision.

## Compliance

- [R-053](../research/records/r-053-stage-7-authority-recovery.md) records the
  exact dependency, format, resource, restore, and reconciliation evidence.
- The [release, update, and Authority Custody reference](../technical/release-update-custody.md)
  owns the maintained envelope and lifecycle boundary.
