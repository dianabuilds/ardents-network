# Release, Update, and Authority Custody

Status: **M12 custody tracer in progress.** This document records the current
Module contracts; it does not claim a supported installer, automatic unlock,
platform qualification, or a complete operator journey.

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

## Explicit limits

- `cmd/ardents-name` still consumes the retained complete signed control wire;
  moving the operator/Gateway intake through custody belongs to later
  composition work.
- Update has no Vault/root input and its D0 test proves it does not mutate
  Vault or floor bytes.
- The retained `custody_notice` key is not live custody state. It is the exact
  H3 evidence text in the frozen `ardents-release-decision-v1`,
  `ardents-update-result-v1`, and update-manifest V0 formats. Go projections
  call it `EvidenceNotice`; the old byte name remains only as the C2 R-064
  tracer writer/reader through M13. M13 must remove that writer and field
  together, or retain the vector as C4 provenance; it must not add a v2
  compatibility writer or treat the text as Vault status.
- Windows/Ubuntu crash, permissions, and power-loss qualification remain open.
- R-044 threshold recovery already replaces the effective Name Authority in
  Namespace; its completed Record rejects a signature from the former key.
  However, an active Vault has only opaque environment/network/root/authority
  commitments, not the Name needed to discover that replacement. It therefore
  cannot safely demote itself merely from a generic current-state view.
  [R-086](../research/records/r-086-custody-authority-revocation.md) owns a
  possible opaque replacement proof and any D08 migration. Broker Grant
  revocation remains a separate local-admission transition.
- Supported lifecycle/installer work remains open.

## Evidence

- [ADR-0021](../adr/0021-use-password-derived-authority-custody.md)
- [R-053](../research/records/r-053-stage-7-authority-recovery.md)
- `internal/custody/vault_operation_test.go`
- `internal/custody/vault_namespace_signing_test.go`
