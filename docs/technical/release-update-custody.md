# Release, Update, and Authority Custody

Status: **current maintained technical contract.** The bounded Custody
ownership transfer is complete in this scope. This document does not claim a
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

## Custody disposition

The maintained Custody Module owns the bounded encrypted
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
receipt. ADR-0052 historically permitted one profile-bound operation to
consume that selected record internally and create only the fixed RC1/RC2
static input directories after maintained verifier preflight. ADR-0059 retired
that completed one-off operation and both terminal routes. The maintained
custody boundary now only initializes or inspects the encrypted record; it
cannot sign, assemble, publish, upload, or execute a release.

A fresh executable cannot use its own Release Decision code to authenticate
itself before first execution. `RootBytes` and the first executable therefore
have caller-owned provenance established by the selected Distribution Profile;
the Release Module fails closed on their content but does not invent that
initial provenance. Once one trusted executable has committed the root and
floors, this external enrollment input authorizes no successor: later
executable authorization remains exclusively a Release Decision followed by
the Update boundary.

For the historical first closed-alpha cohort, the selected Distribution Profile was one
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

`internal/endpoint/replacement` consumes only the opaque Release authorization
and exact candidate bytes. It owns the selected Ubuntu foreground replacement:
direct-file admission, immutable staging beside the program, retained
predecessor, stopped exact user unit, atomic activation, no-network candidate
self-test, durable journal, explicit recovery, and separately authorized
rollback. It never receives a Vault, password, Authority root, downloader, or
generic Custody writer.

The former generic `internal/update` transaction had no production caller and
is retired. Its distinct schema-copy and adapter choreography are not part of
the selected Endpoint replacement contract. Current replacement behavior and
crash boundaries are verified at the owning Module; platform packaging,
automatic update, Windows replacement, system registration, and public release
qualification remain unselected.

## Cross-module invariants

- Release floors and trusted roots never decrease.
- Endpoint replacement cannot authorize itself or interpret Authority/Custody
  state.
- Custody never supplies secrets to Release or Endpoint replacement.
- A failed or interrupted replacement returns an explicit bounded outcome; it
  must not silently accept a candidate or erase the retained predecessor.
- The retired V0 evidence field remains only in historical provenance; no
  maintained writer recreates the retired generic transaction.

## Verification

Focused Release, Update, and Custody behavior tests cover metadata rejection,
root/floor progression, interruption/recovery, rollback, residue, encrypted
Vault non-mutation, export/restore, reconciliation, and sealed Name-control
signing. Run the normal repository gate during development and the full check
before integration. Platform crash, power-loss, permissions, and supported
lifecycle qualification are not satisfied by these tests.
