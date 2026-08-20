# Stage 7 decision proposal — Use explicit password-derived Authority Custody

Status: **accepted on 2026-08-20 and recorded in
[ADR-0021](../adr/0021-use-password-derived-authority-custody.md).**

## Context

Stage 7 must protect live Authority Vault records and export portable Recovery
Bundles on Ubuntu and Windows, including headless operation and recovery after
source account/machine loss. The active team is one Product Owner and Codex; a
platform-specific keychain pair adds separate dependencies, prompts, failure
semantics, and recovery limits. Windows DPAPI is normally tied to the same user
credentials and machine. Freedesktop Secret Service remains a desktop service/
draft surface that may be absent on the required Ubuntu host.

The roots are used only through explicit Authority Custody operations. Runtime
Connection and Service Administration work uses bounded credentials and does
not need automatic root unlock.

## Proposed decision

Choose R-053 O2 and the exact
[Authority Custody specification](stage-7-authority-custody-spec.md):

1. protect both live Vault records and portable Bundles with the same versioned
   Argon2id/AES-256-GCM envelope, using distinct authenticated purposes, fresh
   salt/key/nonce, and separately entered passwords; one v1 Vault is bounded to
   1024 independently atomic records/1 GiB under one Vault password and has no
   in-place password rotation;
2. keep the Vault locked after restart and require explicit terminal or bounded
   secret-descriptor input for every custody operation;
3. select no DPAPI, Secret Service, cloud/help-desk recovery, password escrow,
   first-party cryptography, cgo, or `unsafe` for custody;
4. keep update/install/runtime ownership outside Authority Custody and retain
   encrypted Vault/floors across ordinary repair/update/uninstall; and
5. restore only into isolated `authority-locked` export-only state; require a
   strictly higher authenticated successor, fresh runtime Instance Key, and
   separately reissued Grants before active use.

## Consequences

- Vault and Bundle behavior is portable and consistent across both hosts and
  Distribution Profiles.
- The Product Owner must remember/store owner-chosen passwords independently;
  Ardents cannot recover them.
- Unattended root signing is deliberately unavailable in Stage 7. This is
  compatible with offline/protected roots and bounded online credentials.
- Password strength limits offline-guess resistance. Fixed Argon2 resource cost
  raises but cannot eliminate that risk.
- OS file permissions, process isolation, and encrypted-at-rest bytes are
  defense in depth; endpoint/Owner compromise, snapshots, swap/crash dumps, and
  guaranteed secure deletion remain outside the claim.
- Any future automatic OS keychain unlock, hardware token, dedicated signer,
  or multi-device custody is a new consequential decision, not an Adapter swap
  hidden inside this profile.

## Acceptance gate

The Product Owner accepted O2 for Stage 7 development on 2026-08-20, including
explicit-password UX/loss semantics and the recorded weakest-native-host and
durability qualification deferrals. Scheduled Ubuntu-Docker/current-Windows
KDF/RSS/format/interruption/restore/reconciliation/cleanup cells remain S7.2
evidence gates. A falsifier reopens ADR-0021; it is not hidden as a qualified
pass.
