---
status: accepted
date: 2026-09-27
partially-supersedes: ADR-0042 (retained v1/v2 verification clause only)
---

# ADR-0112 — Network enrollment v3 is the sole accepted descriptor grammar at every maintained command boundary; recognized v1/v2 descriptors are refused with a typed error

(F-47. The finding recorded that the general verifier silently widened version
acceptance across four dispatchable boundaries and required version acceptance
to be decided per boundary; under the product owner's standing decisions that
legacy dies and that no support or backward compatibility is owed, this ADR
selects the sole grammar and the typed refusal, and discharges the ADR-0042
retention clause with a superseding decision.)

## Context

`internal/enrollment` parses three Network enrollment descriptor grammars
(`ardents-closed-alpha-enrollment-v1/-v2/-v3`). The selected Portable first-run
composition passes `VerifyHeadless`, whose Node/Custody companion requirement
already makes v3 a necessary condition. But three other dispatchable routes
call the general `Verify` and still accepted v1/v2: the bounded
`endpoint enrollment-check` diagnosis (both its manifest-pinned and legacy
three-argument forms), `endpoint enroll-installed` (the command also rendered
by the maintained per-user Installed unit generator), and the read-only
alpha-control inspection (`inspect-bundle`, `inspect-transitions`). ADR-0042
retained v1/v2 verification for exactly those non-acceptance uses; ADR-0088
retired the corpus accepting command without touching that clause, and
ADR-0110 retired the ACA2 inspection command without touching the descriptor
grammar. The linux installed-package process evidence still upgraded through a
v1 descriptor under `enroll-installed`, which made the retained grammar a
tested contract rather than dead code.

## Decision

1. `ardents-closed-alpha-enrollment-v3` is the sole accepted Network
   enrollment descriptor grammar at every maintained command boundary:
   `enrollment-check` (both forms), `enroll-installed`, the Portable first-run
   gate, and the read-only alpha-control inspection.
2. A descriptor whose recognized schema line names v1 or v2 is refused with
   the exported typed sentinel `enrollment.ErrLegacyEnrollmentDescriptor`
   before companion inventory, executable identity, or Release input
   construction. The refusal is version classification only: manifest-pin
   precedence, descriptor-digest equality, inventory exactness, and every
   other failure behavior are unchanged. An unknown schema — including Browser
   enrollment-v4 — keeps its generic invalid refusal; only recognized retired
   Network versions are classified.
3. The v1/v2 parsing grammar is deleted with its last consumers. The parser
   keeps one fixed v3 field ordering and the unconditional
   `corpus_authority=corpus.pub` and platform-named `control_artifact`
   companion identities; `validDescriptor` accepts no other schema value.
4. Existing v1/v2 bundles stay on disk byte-for-byte. There is no converter,
   migrator, or compatibility reader; a participant holding one sees the typed
   refusal and must obtain a fresh v3 bundle. ADR-0042's "preserve enrollment
   v1 and v2 verification for their existing non-acceptance uses" clause is
   superseded by this decision; its `ExecutableArtifactName` identity contract
   and its v3 grammar definition stand.
5. The `Verify`/`VerifyHeadless` distinction is inventory scope, not version:
   `VerifyHeadless` remains the Portable first-run gate that additionally
   requires the manifest-pinned Node and Authority Custody companions, and the
   general `Verify` never widens accepted versions. The v3 companion
   projection (pair-or-absent, partial pair refused) is unconditional.
6. The retained `enroll-installed` command and its per-user unit keep their
   separate retirement outcome, still bound to the supported protected
   system-unit launch (step 6). This decision only pins the grammar they
   accept until then.

## Consequences

- Every dispatchable enrollment route now accepts exactly one descriptor
  grammar; the repository can honestly be described as v3-only for Network
  enrollment, and the finding's "general-purpose verifier silently widens a
  selected path" consequence is gone at the parser, not per caller.
- The cross-platform enrollment-check fixture, the linux installed-package
  fixture, and the unit fixtures all emit the full v3 inventory
  (`corpus.pub` + platform-named control artifact); the former v1 upgrade
  vector becomes typed-refusal evidence.
- `Verify` and `VerifyHeadless` differ only in companion requirement; no
  caller can receive a v1/v2 acceptance through either entry point.
- The alpha-control inspection route keeps its ADR-0110 ACA1-only scope and
  now also classifies legacy descriptor versions identically to the endpoint
  routes.

## Verification

- `internal/enrollment` behavior tests: the canonical v3 fixture verifies and
  projects its control/corpus identities; hand-built v1 and v2 descriptors
  over consistent manifests return `ErrLegacyEnrollmentDescriptor` through
  `Verify`; `VerifyHeadless` still refuses a v3 inventory without its
  Node/Custody pair and accepts with it.
- The cross-platform enrollment-check process e2e drives a v3 bundle; the
  linux installed-package process e2e upgrades through a v3 descriptor and its
  refusal evidence covers the retired versions at the command boundary.
