---
status: accepted
date: 2026-09-26
---

# ADR-0096 — Reject the closed-alpha Entry issuance and Initiator-verification candidate surface

## Context

The cite-or-die sweep (ADR-0091) retains one deadcode-allowlist group per
unwired candidate surface. The group "unwired closed-alpha tracer" listed
five shared symbols — `internal/entry.Issue`, `internal/entry.Verify`,
`appendIssueUint16`, `appendIssueUint64`, and `validIssueInput` — with the
retirement condition: "Connect it through an accepted C0 command contract
or remove it in the ADR that rejects the candidate."

The audit found no acceptable candidate command:

- `Issue` had zero callers outside its own package test; it was a second,
  independent builder of the same canonical Invite v2 bytes that the
  test-local fixture builder produces.
- Exported `Verify` had zero callers outside package tests. It was a thin
  wrapper that mapped the private, live `validateInvite` classification
  onto an `Authorization` value for an Initiator-side Entry port.
- The receiving engine that would consume such an `Authorization` is
  absent: the package map pins "It exposes no Initiator receiving
  admission engine", and the Initiator Entry-admission adapter and its
  receiving engine were retired in earlier closure audits. No accepted C0
  command contract references Invite issuance or Initiator-side
  verification.
- The c0 reconstruction plan (F-08 row) keeps only "the internal verifier
  required to read retained Invite records until their data disposition is
  closed". That verifier is `validateInvite`, reached in production solely
  through `owner.validate` on the Import and reopen revalidation paths.

`Verification.MinimumReservation` and the `Insufficient` class existed only
for the exported `Verify` port: the sole production construction of
`Verification` (in `validation.go`) never set a reservation, so the branch
was unreachable outside tests.

## Decision

Reject the candidate surface and remove the tracer:

1. `internal/entry/issue.go` is deleted in full (`Issue`, `IssueInput`,
   `appendIssueUint16`, `appendIssueUint64`, `validIssueInput`).
2. The exported `Verify` wrapper and the `Authorization` struct are
   deleted from `internal/entry`.
3. `Verification.MinimumReservation`, its `validateInvite` branch, and the
   `Insufficient` class constant are deleted with the port they served.
4. `validateInvite`, `Verification`, and the remaining closed `Class`
   enumeration are retained unchanged for the live Import/reopen path, per
   the F-08 retention rule for reading retained Invite records. The Invite
   v2 canonical grammar in `invite.go` (magic, framing, `signatureInput`,
   decode) is the retained persisted-record schema and is untouched; this
   ADR changes no stored data contract.
5. Tests follow the production split: the four GAP-5 reservation tests
   (`verification_test.go`) die with the rejected policy — recipient-side
   Import never enforced a reservation floor, so no retained behavior loses
   coverage; the two classification tests are rewritten to drive
   `validateInvite` directly
   (`TestValidateInviteReturnsOnlyCurrentInitiatorCandidate`,
   `TestValidateInviteReturnsConflictingRoleWhenConflictCallbackReturnsTrue`);
   `TestIssueProducesAStateReferencedInvite` is deleted. The cross-package
   comment in `internal/network/duty/source_collision_chain_test.go` now
   cites `validateInvite` and the renamed test.
6. The guard `internal/architecture/entry_issuance_retirement_test.go`
   pins the absence of `issue.go`/`verification_test.go`, the rejected
   exported names, and the retained internal verifier wiring.
7. The allowlist group "unwired closed-alpha tracer" is dissolved: common
   symbols 367 → 362, windows-amd64 571 → 566, linux-amd64 368 → 363.

## Consequences

- Entry no longer pretends to own canonical issuance; the package map row
  drops the "canonical issuance" phrase. Invite bytes now exist in
  production only as retained records read by `validateInvite`.
- If a future accepted C0 command ever needs Invite issuance, it arrives
  with its own contract and ADR rather than inheriting an unwired tracer.
- The F-08 Invite-root disposition (writer retirement for
  `cmd/ardents entry recipient/import` and the attempt/contact journal
  schema) remains a separate open card; this ADR does not touch it.
