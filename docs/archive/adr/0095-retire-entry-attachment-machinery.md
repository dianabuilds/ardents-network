---
status: accepted
date: 2026-09-26
---

# ADR-0095 — Retire the uncalled Entry attachment execution machinery and ValidateNameOrigin

## Context

The cite-or-die sweep (ADR-0091) left one shared production deadcode
allowlist group: 22 "unwired shared legacy Entry and Service Connection
leaves". ADR-0093 removed `route.OpenEntryAttachment`, the sole production
caller of `entry.owner.Acquire`. The post-retirement caller audit (F-08 in
the reconstruction plan) confirms the remaining facts: `Acquire`, `Contact`,
`RecipientCertificate`, the attempt/contact journal writers
(`beginAttempt`/`nextContact`/`finishContact`/`terminalize` and their
helpers), the `guardedConnection` deadline carrier, and the
`attachmentLease` tracking machinery have no production caller on any
platform; `service/connection.ValidateNameOrigin` likewise has none. The
retained Invite root (`entry.Open(...).Import(...)` behind the
`cmd/ardents entry recipient/import` operator commands) never touches the
attachment machinery, and the live text runtime reaches name-origin checks
through `ContinuesNameOrigin` and `ValidateRecovery` only.

The Invite-root durable state decodes strictly
(`decoder.DisallowUnknownFields()`), and existing roots may carry an
attachment journal written by pre-retirement builds. Per the F-08 boundary,
the Invite-root writer retirement and its full data disposition remain a
separate operator-contract card; this record retires only the uncalled
execution machinery, following the ADR-0094 stance: code paths retire, the
persisted data contract is retained.

## Decision

1. Delete `internal/entry/attempt.go`, `guarded_connection.go`,
   `contact.go`, and `attachment_lifecycle.go` with the Acquire-driven
   suites `attachment_lifecycle_test.go` and `carrier_opener_test.go`.
   Remove the exported `Attempt`, `Presentation`, and `CandidateOpener`
   contract types, the owner's `lifecycle`/`cancelLifecycle`/
   `acquisitions`/`attachments`/`nextAttachment` fields, the
   `RecipientCertificate` accessor, and `retireInvalidActiveLocked`.
   Remove `ValidateNameOrigin` from `internal/service/connection`.
2. Retain the legacy data contract: the `durableState.Attempt`/`Contacts`
   schema with `attemptRecord`/`contactRecord` and `validAttemptState`
   validation; `Open`'s journal recovery, which terminalizes a non-terminal
   or unclean legacy attempt as `entry-interrupted` and settles its
   replacements; and `Import`'s draining-replacement branch over a
   non-terminal legacy attempt. `validRecord` moves to `revalidate.go` with
   its live Open/close-path consumers. Removing the schema fields belongs
   to the F-08 Invite-root data disposition, not to this retirement.
3. Retire `settleClosingAttempt` as provably vacuous: `Open` terminalizes
   every non-terminal journal before the root becomes usable, and no live
   writer can create a new one, so `Close` reduces to its idempotent
   exclusive-lease release.
4. Keep `ContinuesNameOrigin` and `ValidateRecovery` with their Linux text
   runtime callers; only the uncalled validator leaf is removed.
5. Keep the recovery and draining invariants under test by writing legacy
   journals directly (`entry_test.go`, `import_recipient_test.go`), and add
   `internal/architecture/entry_attachment_retirement_test.go` guarding both
   the absence inventory and the retained data contracts.
6. Dissolve the allowlist group: production deadcode counts move to common
   367, windows-amd64 571, linux-amd64 368, exact-matched on both platforms,
   with the `-test` dead set empty.
7. The `entry.Issue`/`Verify` tracer group is explicitly out of scope; its
   candidate surface still needs an ADR that either connects it to an
   accepted command contract or rejects it.

## Consequences

`internal/entry` loses roughly 500 lines of production code and its owner
close path becomes a pure lease release. Existing Invite roots stay
readable and writable through the retained operator commands, and any
legacy attachment journal terminalizes deterministically at reopen. The
open F-08 obligations narrow to the Invite-root writer retirement, the
attempt/contact schema disposition, and the `Issue`/`Verify` candidate
decision.
