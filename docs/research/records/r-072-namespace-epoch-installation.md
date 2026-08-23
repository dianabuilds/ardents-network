---
id: R-072
title: How may verified Namespace Epoch inputs become one threshold-current materialization without a caller-built Record corpus?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-072 — Namespace Epoch installation

## Decision this unlocks

R-071 makes an accepted root claim an opaque `ClaimWinner`, but the existing
`Store.Commit(Epoch, [][]byte, attest)` accepts a caller-built signed corpus.
That bypasses the new fact at the root-claim installation boundary and leaves
ordinary pending successors and claim winners on incompatible paths.

## Current contract

ADR-0020 retains the threshold-attested Namespace statement as the sole
current-state publisher. R-070/ADR-0023 require ordinary Authority transitions
to come from the immutable pending prefix. R-071 requires root claim state to
come only from its verified Epoch fact. The complete Network Epoch log and the
R-045 commitment-admission verifier remain outside Namespace under R-042; this
decision must not create a Network-State dependency or a second authority.

## Hypotheses

- **H1:** Store opens an opaque installation from its verified current snapshot;
  it can add a selected durable pending prefix and a `ClaimWinner` only through
  a signing port that Namespace verifies against the exact derived Record;
  the existing threshold statement then commits that sealed candidate.
- **H2:** give callers a mutable Record map/slice and ask them to declare which
  items came from pending state or claims.
- **H0:** keep `ClaimWinner` separate and install roots through arbitrary
  `Store.Commit` records.

## Evaluation criteria

The path must preserve the existing Ed25519 Record and threshold-statement
transcripts; start only from verified current state; reject a substituted
signer result without consuming the claim fact or changing the candidate; and
recheck selected pending entries against the durable cursor at commit.  It must
not assert a new public Epoch wire protocol, global claim-capacity decision,
or successful resolution for an unpublished root Record.

## Findings

- **Inspection (2026-08-23):** after R-070 `pendingCursorFor` constrains a
  nonempty pending journal, but genesis and claim installation still reach the
  public raw `[][]byte` `Store.Commit` input.
- **Inference:** a Store-owned candidate object can preserve the current
  snapshot/pending invariant and give Custody only the exact Record it must
  sign.  Verifying that response before insertion prevents the signing port
  from changing authority lifecycle state.
- **Assumption:** this is an interim in-process Namespace Interface.  Its
  `Record` callback value is replaced with a sealed signing request during
  F031; no external transport contract is introduced here.

## Options

| Option | Fit | Disposition |
|---|---|---|
| H1 opaque installation | One local candidate joins verified winners and durable successors before the unchanged threshold commit. | choose |
| H2 mutable corpus | Preserves caller-owned lifecycle assembly and makes substitution validation optional choreography. | reject |
| H0 separate paths | Leaves F023/F028's trusted fact disconnected from current state. | reject |

## Recommendation

Accept H1 with high confidence. `BeginEpochInstallation` derives the baseline
from the verified Store snapshot. `IncludePendingThrough` selects only the
next immutable journal prefix. `MaterializeClaim` receives a `ClaimWinner` and
an in-process signer, verifies the signer returned the exact derived Record,
then consumes the winner. `Commit` uses the unchanged threshold-attestation
and Store cursor verification. The legacy raw `Store.Commit` remains temporary
test/compatibility surface until F031 removes caller-built corpora.

The strongest counterargument is that the new object does not itself prove the
complete global Epoch corpus or R-045 admission. That proof is intentionally
still the R-042 threshold close and its input-ingestion owner; this change only
stops Namespace from losing the resulting verified fact before materialization.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** No new ADR is needed: the decision preserves ADR-0017,
ADR-0020, and ADR-0023 transcripts and refines Namespace-local composition.
M5 must retain substitution, pending-prefix, threshold-commit, restart, and
unpublished-root tests. F031 must replace the interim `Record` signing callback
and raw `Store.Commit` input with the sealed Namespace Interface.
