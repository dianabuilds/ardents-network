---
id: R-070
title: Which authenticated artifact can bridge submitted Namespace control to current materialization?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-070 — Namespace pending successor Record

## Decision this unlocks

G2-F024 needs one durable submit-to-current lifecycle. It cannot safely join
the existing Gateway control map to `Store.Commit` while a control transition
produces an unsigned `Record` and the Store accepts only Authority-signed
Records.

## Current contract

R-047/ADR-0014 retain the exact Ed25519 Record signature and R-057/ADR-0020
make threshold-attested Epoch materialization the sole source of current
Namespace proof. R-065 assigns lifecycle decision time to Gateway. R-067 says
the current internal control, receipt, and durable-chunk encodings have no
named observer and may use C0 replacement. R-069 establishes that submission
is not current state.

## Hypotheses

- **H1:** each control submission carries an Authority-signed successor Record;
  Namespace verifies that it is exactly the transition result at its single
  decision time, persists the authenticated transition plus successor as
  pending, and installs only a threshold materialization consistent with those
  pending inputs.
- **H2:** persist an unsigned Gateway-computed Record and rely on the Gateway
  result when materializing it.
- **H3:** introduce a distinct transition-envelope leaf and teach every
  materializer/resolver to treat it as current instead of a signed Record.
- **H0:** retain only volatile submission and permit Store to install any
  independently supplied signed corpus.

## Evaluation criteria

The selected artifact must let a fresh process reproduce acceptance without a
Gateway secret; preserve the existing Record Ed25519 transcript and compact
current proof; bind predecessor, successor, operation, and decision time;
reject a substituted or unsigned successor before persistence; and ensure a
threshold current installation cannot advance from bytes absent from the
durable pending journal. A submitted result remains only `submitted` or
`denied`.

## Findings

- **Inspection (2026-08-23):** `control.Apply` verifies a predecessor
  transition but returns bytes from `EncodeRecord(updated)`, while
  `Store.Commit` accepts only `SignRecord` containers. No current code can
  turn the former into the latter without an Authority private key.
- **Inference:** persisting the Gateway-computed value would make a Gateway
  assertion indistinguishable from an Authority-signed current Record after
  restart, contradicting R-047 and R-069.
- **Inference:** a second current leaf kind would duplicate Record/lineage
  verification and widen the R-057 proof contract solely to repair local
  composition.

## Options

| Option | Fit | Security and operational consequence | Disposition |
|---|---|---|---|
| H1 signed exact successor | Keeps one existing current artifact; transition remains independently verifiable and restartable. | Custody must sign the proposed successor before private submission; Gateway verifies but never signs or invents state. | choose |
| H2 unsigned pending Record | Small apparent implementation. | A compromised Gateway can fabricate post-restart lifecycle state. | reject |
| H3 new current envelope | Can bind transition evidence. | Changes the current proof semantics and duplicates Record verification with no product benefit. | reject |
| H0 separate paths | Retains current tracer behavior. | Leaves F024 open and permits disconnected Store installation. | reject |

## Recommendation

Accept H1 with high confidence. A canonical control input includes an opaque
Authority-signed successor Record. Namespace derives the successor under its
one decision time, verifies exact equality and the signature, writes the
submission/pending journal atomically, and later accepts only selected pending
entries into the threshold materialization transaction. A crash before either
commit leaves no accepted pending entry; a crash after it reopens the same
pending chain; current moves only after authenticated installation.

The strongest counterargument is that signing a successor requires Custody to
know its complete calculated fields. This is intentional: the alternative
would let Gateway author Authority state. Constructors must therefore prepare
the exact successor transcript before Custody signs it.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** ADR-0023 records the hard-to-reverse lifecycle boundary. M5 must
implement durable pending, exact-successor, atomic install, fresh reopen, and
substitution/tamper tests; no experiment is required because the conclusion is
from current protocol/source incompatibility rather than external mechanism
selection.
