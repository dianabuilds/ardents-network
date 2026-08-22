---
id: R-063
title: When does a verified release root become durable relative to executable acceptance?
status: accepted
owner: Product Owner and Codex
started: 2026-08-22
reviewed: 2026-08-22
---

# R-063 — Release-root transaction boundary

## Decision this unlocks

DA-01 and M1 need one release-recovery oracle: after a fully threshold-verified
consecutive root is available but later release metadata or executable checks
fail, restart must know whether to trust that root and which, if any, version
floors advanced. This record selects the recovery oracle; a later M1 change
still needs its own format, reader/writer, and failure-path evidence.

## Current contract

R-049 selects a bounded TUF-compatible verifier and durable non-decreasing
root/floor state. Its recommendation says a failure publishes neither a release
decision nor a partial floor transaction. The accepted Stage 7 lifecycle
specification instead requires the client to durably publish every fully
verified next root before using it to verify later metadata; a root transition
authorizes verification but never executable bytes. ADR-0015 retains
non-decreasing release floors and separates Release Decision from activation.

The current Release Decision implementation follows the lifecycle rule: it
publishes the exact initial and each consecutive verified root chain through
`CommitRoot`, preserves any already accepted timestamp/snapshot/targets floors,
and publishes new metadata floors only after complete accepted metadata;
no-update retains existing floors without a write. The Stage 8 F003 review correctly records this as a
decision conflict, not evidence that either wording may be silently chosen.

## Hypotheses

- **H1:** A verified consecutive root chain is a durable root-anchor
  transaction. Timestamp, snapshot, and targets floors form a separate atomic
  accepted-metadata transaction: later failure adds the root archive, preserves
  any prior complete metadata floors unchanged, and adds no new metadata floor.
- **H2:** Root and all metadata floors are one atomic transaction; no root
  survives a later metadata or executable rejection.
- **H0:** Neither boundary preserves the accepted root-rotation and
  fail-closed recovery contract; release-governed behavior must stop pending a
  redesigned lifecycle.

## Evaluation criteria

The choice must keep root rotation threshold-authenticated, consecutive,
environment/network bound, tamper-evident, non-decreasing, and restart-safe.
It must preserve the exact verified consecutive root archive ending at the
accepted root and never authorize executable bytes, update activation, or a
positive Release Decision from a root alone. A malformed/unauthorized root, a
root gap, or a failed durable write must change no trusted state. A rejected
timestamp, snapshot, targets, target identity, or executable policy must not
advance a new metadata floor or alter a prior complete metadata-floor set. The
chosen state must have one owner and a recoverable atomic publication rule
without relying on distributor, package, or executable signatures.

## Evidence plan

### Primary sources

- R-049 recommendation and disposition, accessed 2026-08-22.
- Stage 7 lifecycle specification §3.1 and platform-evidence B10,
  accessed 2026-08-22.
- ADR-0015, accessed 2026-08-22.
- `internal/releasedecision/{tuf_client,evaluate,store,store_persist}.go` and
  `floor_persistence_test.go`, inspected 2026-08-22.
- Stage 8 G2 release/update delta review F003, accessed 2026-08-22.

### Experiment

Run two durable-root rejection characterizations. First, submit a chain with
one fully verified successor root and below-threshold targets to a fresh
temporary floor root; reopen after rejection and capture the exact root archive
plus root, timestamp, snapshot, and targets floors. Second, repeat after an
accepted metadata-floor publication. This falsifies H1 if the exact root
archive is not durable, any new metadata floor advances, a prior metadata floor
changes, or restart changes the result. It falsifies H2 if the accepted
lifecycle evidence requires root publication before later metadata verification.

### Failure scenarios

- root threshold failure, gap, reuse, cross-environment/network root, expiry,
  or failed root write;
- later timestamp, snapshot, targets, target-identity, or executable-policy
  rejection after a valid root rotation;
- crash/tamper between root-anchor and metadata-floor publication;
- malicious distributor replaying prior metadata after a new root anchor;
- a caller interpreting root-only state as executable authorization.

## Findings

- **Sourced fact:** the lifecycle specification requires durable publication of
  each fully verified root before later metadata verification and states that a
  root transition does not authorize executable bytes.
- **Sourced fact:** R-049's recommendation uses all-or-nothing language for a
  "partial floor transaction" without distinguishing the root trust anchor
  from timestamp/snapshot/targets acceptance floors.
- **Sourced fact:** the current `Store` contract names root-only state
  explicitly; its root version/digest and exact root archive may advance while
  all three metadata floors are zero, or while a previously accepted complete
  metadata-floor set remains unchanged.
- **Measurement:** on 2026-08-22,
  `TestFloorStorePublishesRootBeforeRejectingExecutableMetadata` passed from a
  fresh temporary root. It threshold-rejected targets metadata after a valid
  root-2 rotation, retained the exact root-1/root-2 archive and root-2 digest
  with timestamp/snapshot/targets versions and digests at zero, and observed
  the same root-only state after reopen.
- **Measurement:** on 2026-08-22,
  `TestFloorStoreRetainsAcceptedMetadataFloorsAfterLaterRootOnlyRejection`
  passed. After an accepted root-1 metadata-floor set, it threshold-rejected
  targets following root-2 rotation, retained the exact root-1/root-2 archive
  and root-2 digest, preserved the prior timestamp/snapshot/targets versions
  and digests, and observed the identical floor set after reopen.
- **Measurement:** `TestEvaluateAtomicFloorPublication` and
  `TestEvaluateRestartIntegrity` passed on the same date. They confirm that a
  complete successor floor transaction is atomic and remains readable after
  reopen; these local tests do not establish a product Qualification claim.
- **Inference:** H1 reconciles both accepted semantic obligations if R-049's
  all-or-nothing rule is scoped to the executable-metadata acceptance
  transaction rather than to the independently verified root anchor.

## Options

| Option | Product and security fit | Recovery and implementation consequence | Disposition |
|---|---|---|---|
| H1: root anchor separate | Preserves authenticated root rotation and prevents a later metadata failure from erasing the root needed for future verification; never authorizes bytes. | Persist the exact root archive atomically; leave prior metadata floors untouched on rejection and commit a new complete metadata-floor set only with accepted material; no-update writes nothing; test crash and tamper at the boundary. | Accepted. |
| H2: single transaction | Makes one broad import result atomic. | Contradicts the lifecycle requirement to anchor a root before relying on it for later verification; can discard a verified successor on unrelated target failure. | Reject unless the lifecycle specification is superseded. |
| H0: stop | Avoids choosing inconsistent state semantics. | Blocks M1 and all release-governed behavior until a new lifecycle is designed. | Fallback. |

## Recommendation

Accept H1. R-049 is clarified: a failure publishes neither a release decision
nor a new partial **metadata-floor** transaction; it may retain the exact
independently verified consecutive root archive and any prior complete metadata
floors. Confidence is high because H1 matches the lifecycle specification,
Stage 7 B10 evidence rule, and both fresh-root and prior-floor restart
characterizations. The strongest objection is that widening the durable root
surface without revalidating archive/tamper recovery could preserve a malformed
intermediate chain; M1 must therefore retain a separate root-archive integrity
and restart test before any code cutover.

## Disposition

**Accepted H1 on 2026-08-22 under the Product Owner's standing Stage 8
authority.** Both bounded characterizations passed; no D06 production state
mutation was made. No ADR is required because this reconciles R-049 with the
already accepted lifecycle rule without choosing a new format or technology.
A future lifecycle or D06 format change still requires its own ADR analysis.
