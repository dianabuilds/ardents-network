---
status: accepted
date: 2026-11-19
supersedes: ADR-0110 (decision 1 and the ACA1 maintenance clause of decision 5); ADR-0038 (maintained reader clause)
---

# ADR-0127 — Retire ACA1 inspection and the persisted-portable enrollment diagnosis

Status: **accepted** by the Product Owner on 2026-11-19 in the active
conversation (M2 rescope decisions #A/#B/#C, implemented under #647). There
are no supported clients of the retired surfaces; the Product Owner's reframe
— no clients, free to choose what suits the product — is the explicit
retirement decision for the persisted portable document.

## Context

The working product runs on the successor tree. The ACA1 disclosure-inspection
stack (`internal/alphacontrol` and `internal/alphacontrol/inspection`,
`inspect-bundle`/`inspect-transitions`) and the persisted
`ardents-alpha-enrollment-input-v1` diagnosis bridge keep a parallel old
Release/State/enrollment verification path production-alive beside the
successor owners that already verify the same authority facts. The catalog was
disclosure-only: it never granted State, Release, or runtime authority
(ADR-0038, ADR-0120). The v1 document is a persisted convenience record whose
every field is re-derivable from the bundle root and an independently delivered
manifest pin.

## Decision

1. Retire ACA1 as a maintained verification layer. Remove `internal/alphacontrol`
   and `internal/alphacontrol/inspection`. `inspect-bundle` and
   `inspect-transitions` return a typed refusal before parsing arguments or
   opening any file, root, or floor. No unified replacement report is selected:
   successor per-domain commands verify the same authority facts, and
   acceptance outcomes come only from the qualification harness bound to the
   exact candidate. The facts-vs-outcomes separation already fixed by ADR-0120
   is preserved, not invented.
2. Retire the persisted-portable enrollment diagnosis. The three-argument
   `endpoint enrollment-check <alpha-enrollment.json>` form refuses before any
   file read, and the v1-document parser is removed. The four-argument
   bundle-root+pin form remains and cuts over to
   `internal/successor/enrollment.Verify`; its
   `manifest-pinned-enrollment-verified` output contract is preserved
   byte-identical. The `alpha-enrollment-verified` kind retires with the form.
3. Retire the ACA1 preparation routes `prepare-qualification-catalog` and
   `prepare-qualification-evidence` and their alpha scripts with the reader.
   Retire `prepare-qualification-node-record`/`prepare-qualification-epoch`
   and the qualification-network scripts: the successor preparation and
   signing commands (`ardents-next network prepare-initial-keys`,
   `prepare-initial-node-record`, `prepare-initial-epoch`,
   `sign-initial-node-record`, `sign-initial-epoch`, and the
   `ardents-control source` genesis chain) already cover the same initial
   Network preparation need with the same canonical ARNR-v2/AREP-v3 bytes.
4. Accepted compatibility is unchanged: the enrollment v3 descriptor grammar
   and its ACA1 companion bytes remain digest-bound retained evidence
   (ADR-0112/ADR-0119); existing on-disk v1 documents, ACA1 inspection floors,
   and old Release/State roots stay untouched (ADR-0124); accepted wire and
   persisted identities are not renamed. Retiring a reader deletes no stored
   file and waives no protection or qualification case.

## Consequences

- ADR-0110 decision 1 (ACA1 as the sole maintained disclosure-inspection
  format) and its decision 5 clause keeping ADR-0038 authoritative for ACA1
  maintenance are superseded. ADR-0038's authority-separation reasoning and
  the enrollment companion inventory remain historical evidence and unchanged
  grammar. ADR-0110's ACA2 retirement and before-effect refusal obligations
  stand. ADR-0112's sole v3 grammar and typed retired-version refusal continue
  at the remaining enrollment boundaries through the successor owner.
- The old `internal/enrollment` Module loses its last production callers and
  remains with its behavior tests until the gated legacy cohort removal; its
  production-dead symbols are reviewed in the deadcode allowlist.
- Same-operator inspection was never independent validation. Per-domain
  successor inspection outcomes remain diagnostics, not acceptance gates;
  numeric and privacy qualification remains with the selected harness.