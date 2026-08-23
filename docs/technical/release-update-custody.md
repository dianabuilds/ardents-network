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
- Windows/Ubuntu crash, permissions, and power-loss qualification remain open.
- Revocation, full foreign-format disposition, and supported lifecycle/installer
  work remain open.

## Evidence

- [ADR-0021](../adr/0021-use-password-derived-authority-custody.md)
- [R-053](../research/records/r-053-stage-7-authority-recovery.md)
- `internal/custody/vault_operation_test.go`
- `internal/custody/vault_namespace_signing_test.go`
