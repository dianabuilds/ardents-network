# Release, Update, and Authority Custody

Status: **current maintained technical contract.** The Stage 8 technical
refactor is complete in this bounded scope. This document does not claim a
supported installer, automatic unlock, platform qualification, or a complete
operator journey.

## Custody ownership

`internal/custody` exclusively unlocks and uses Authority root material. Its
canonical `ardents-authority-envelope-v1` Vault and Recovery Bundle formats use
the accepted fixed Argon2id/AES-GCM profile from ADR-0021. Callers receive only
bounded public receipts, never a password, derived key, root, or generic
signing capability.

An active Name Authority signs either one exact sealed transition or the
ordered pair that `naming/namespace/authority.Prepare` derives from an unsigned
existing-Name Intent. The pair must have the active public key, the active
predecessor generation/revision, and a successor Record exactly one revision
later. Admission still binds the static Intent digest; only Namespace `Submit`
writes the pending journal.

## Recovery lifecycle

```text
active encrypted Vault record
  -> explicit Bundle export and isolated test restore
  -> restore into separate encrypted authority-locked quarantine record
  -> fresh current Namespace witness, strictly higher than the recovered state
  -> new encrypted active successor + durable floor
  -> first sealed signature
```

The witness is opaque and can originate only from an already verified current
Namespace materialization. It identifies exactly one active Name Authority;
absent, ambiguous, inactive, stale, equal, or wrong-authority state fails
closed. Activation advances local watermarks and creates no runtime Instance
Key or Local Grant. The original quarantine record remains export-only and
cannot sign.

## Stage 8 Custody disposition

Stage 8 completes the Custody ownership transfer and its bounded encrypted
Vault/Bundle/signing contract. It deliberately does not select a supported
Windows or Ubuntu storage profile, crash/power-loss or permissions
qualification, a qualified Application isolation profile, or a complete
Custody operator lifecycle. Those are future product/platform work and need a
new scope decision and the applicable ADR analysis; they are not deferred
implementation defects in the current Module.

## Explicit limits

- `ardents name control` consumes the retained complete signed control wire;
  it is an operator input boundary, not a second Authority signing route.
- Update has no Vault/root input and its D0 test proves it does not mutate
  Vault or floor bytes.
- The retained `custody_notice` key is not live custody state. It is the exact
  H3 evidence text in the frozen `ardents-release-decision-v1`,
  `ardents-update-result-v1`, and update-manifest V0 formats. Go projections
  call it `EvidenceNotice`; the old byte name remains only in the unobserved
  V0 provenance fixtures. The V2 fixture cutover omits the field; an independent C4
  verifier retains the exact V0 vectors, and no V2 compatibility writer or
  Vault-status interpretation is permitted.
- Windows/Ubuntu crash, permissions, and power-loss qualification are future
  product/platform work, not current support claims.
- R-044 threshold recovery already replaces the effective Name Authority in
  Namespace; its completed Record rejects a signature from the former key.
  However, an active Vault has only opaque environment/network/root/authority
  commitments, not the Name needed to discover that replacement. It therefore
  cannot safely demote itself merely from a generic current-state view.
  A future opaque replacement proof needs its own format decision before any
  D08 migration. Broker Grant
  revocation remains a separate local-admission transition.
- Supported lifecycle/installer work is future product scope.

## Evidence

- [ADR-0021](../adr/0021-use-password-derived-authority-custody.md)
- `internal/custody/vault_operation_test.go`
- `internal/custody/vault_namespace_signing_test.go`

## Release and Update ownership

internal/release is the sole owner of release trust roots and non-decreasing
release floors. It verifies the selected TUF-compatible metadata profile,
consecutive root rotation, exact target identity, protocol/build state, and
the captured reference time. Its output is a bounded public Decision and
opaque authorization. It does not download artifacts, run a repository,
maintain an ambient cache, sign metadata, select a mirror, or expose its floor
storage.

ADR-0050 separately assigns first closed-alpha release seeds to
`internal/release/custody`. That local Product Owner boundary can initialize
exactly one password-encrypted fixed-role seed record and return a public
receipt. It cannot decrypt material for callers, sign metadata, administer a
TUF repository, publish an artifact, or configure a VPS. A concrete signing
topology and its evidence remain a later gate.

A fresh executable cannot use its own Release Decision code to authenticate
itself before first execution. `RootBytes` and the first executable therefore
have caller-owned provenance established by the selected Distribution Profile;
the Release Module fails closed on their content but does not invent that
initial provenance. Once one trusted executable has committed the root and
floors, this external enrollment input authorizes no successor: later
executable authorization remains exclusively a Release Decision followed by
the Update boundary.

For the first closed H4 alpha cohort, the selected Distribution Profile is one
**Alpha Enrollment Pin** sent through an already authenticated Product Owner
contact independently of GitHub and the download path. It binds one exact
cohort, release, platform, and manifest SHA-256. The participant compares that
digest before parsing the manifest and uses its exact inventory to verify the
executable, descriptor, initial root, complete metadata, and declared static
companions before execution. On first run the Endpoint must submit those same
bytes and its own executable to Release Decision and durably establish the
accepted root/floors before reporting network readiness. The pin is neither a
Release Module input nor successor authority; it authorizes only this first
bundle and makes no public or independent release-control claim.

internal/update consumes only that opaque authorization. It owns the bounded
offline technical transaction: immutable staging, rollback reservation,
stopped-runtime Adapter calls, atomic activation, self-test, journal,
idempotent recovery, terminal inspection, and the caller-owned schema
copy-on-write boundary. It never receives a Vault, password, Authority root,
or generic Custody writer. Its D0 behavior fixture proves that a real encrypted
Vault and floor stay unchanged during the transaction.

The current Update Module is a technical tracer, not a supported installer or
automatic updater. It selects no platform packaging, bootstrap, system
registration, unattended activation, repair, or uninstall behavior. A
supported lifecycle and its platform durability evidence remain separate
decisions.

## Cross-module invariants

- Release floors and trusted roots never decrease.
- Update cannot authorize itself or interpret Authority/Custody state.
- Custody never supplies secrets to Release or Update.
- A failed or interrupted tracer transaction returns an explicit bounded
  outcome; it must not silently activate a candidate or erase the predecessor.
- The retired V0 evidence field remains only in independent provenance vectors;
  current V2 tracer fixtures do not create a compatibility writer.

## Verification

Focused Release, Update, and Custody behavior tests cover metadata rejection,
root/floor progression, interruption/recovery, rollback, residue, encrypted
Vault non-mutation, export/restore, reconciliation, and sealed Name-control
signing. Run the normal repository gate during development and the full check
before integration. Platform crash, power-loss, permissions, and supported
lifecycle qualification are not satisfied by these tests.
