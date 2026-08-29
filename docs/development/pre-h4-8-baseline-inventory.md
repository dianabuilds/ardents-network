# Pre-H4-8 baseline inventory

Status: **active closure inventory; H4-8 qualification is not in progress.**

This is a factual retention register for the stabilization work before the
next qualification candidate. It is not a release process or a second product
specification. Each entry identifies the current owner of any retained fact,
the evidence value, and the permitted disposition.

## Baseline rule

The former RC2/A1--A12 results remain historical functional-alpha evidence,
not qualification of the post-refactor baseline. A future candidate needs its
own immutable source identity and requalification matrix. No current document
may imply otherwise.

## Confirmed retirement set

| Item | Current owner | Retained evidence | Disposition |
|---|---|---|---|
| `experiments/r-124-public-control-simulation/` | ADR-0055, R-124, `internal/publiccontrolsimulation`, command tests | Versioned JSON simulation receipt and behavior tests | **Retired on 2026-08-29.** It contained only a duplicated README runbook. |
| `experiments/r-125-controlled-project-control-transitions/` | ADR-0056, R-125, `internal/publiccontrolsimulation`, command tests | Versioned JSON simulation receipt and behavior tests | **Retired on 2026-08-29.** It contained only a duplicated README runbook. |
| `experiments/r-126-project-control-canonical-name-lifecycle/` | ADR-0057, R-126, `internal/namespacelifecyclesimulation`, command tests | Versioned JSON simulation receipt and behavior tests | **Retired on 2026-08-29.** It contained only a duplicated README runbook. |
| `experiments/r-127-project-control-root-claims/` | ADR-0058, R-127, `internal/rootclaimsimulation`, command tests | Versioned JSON simulation receipt and behavior tests | **Retired on 2026-08-29.** It contained only a duplicated README runbook. |
| `experiments/r-110-safe-endpoint-replacement/` | R-110, `internal/endpoint/replacement`, `endpoint replace` command/tests | The maintained interruption/recovery matrix and current H4-1B technical contract | **Retired on 2026-08-30.** Its two-file shell prototype had no unique current behavior or evidence. |

Each named research record retains its exact command and receipt contract, so
deleting these directories does not remove the reproduction route.

## Classified, pending fact migration

| Item | Classification | Required migration before retirement |
|---|---|---|
| `docs/research/horizon-4-program.md` | Historical research handoff, not current architecture | **Retired on 2026-08-29.** H4 scope/status and non-claims are in `docs/product/horizon-4/README.md`; active/open/decided question state is in `docs/research/questions.md` and the named records; maintained code/command boundaries are in `package-map.md` and `commands.md`; its dated campaign narrative is Git provenance. It had no inbound references. |
| `docs/research/s6-0-preparation.md` | Historical stage preparation | **Retired on 2026-08-29.** Canonical naming, claim ordering, recovery, admission, and current materialization are owned by ADR-0014 and ADR-0017--0020 plus `docs/technical/naming.md`; R-041--R-047/R-055/R-057 retain their decision evidence; its stage gate/campaign summary is Git provenance. It had no inbound references. |
| `docs/development/deep-audit.md` | Future candidate review procedure, not current technical contract | Replace its repository facts with the current package/technical maps; retain only a compact candidate-integrity checklist if a future qualification needs it. |
| `docs/development/closed-alpha-input-request.md` and `closed-alpha-release-ceremony.md` | RC2 process/provenance | **Retired on 2026-08-29.** Current command/schema/output ownership is [release-custody-assembly.md](../reference/release-custody-assembly.md), with command discovery in `commands.md` and broader Module limits in `release-update-custody.md`; historical RC2 execution evidence remains in the matrix and R-119/R-120/R-121. |
| `docs/product/horizon-4/08a-alpha-1-readiness-matrix.md` and `08b-alpha-1-release-profile.md` | RC2 qualification/provenance | Retain as historical evidence and label it as such; never represent it as qualification of a changed baseline. |
| Earlier experiment directories | Mixed decided/open research evidence | Classify individually by linked record status, maintained-test replacement, source inputs, and inbound references before deletion. R-105 and R-117 remain because their records are open. |

## Technical documentation target

Current technical documents must describe one maintained module or command in
terms of its interface, invariants, normal/failure behavior, resource limits,
and verification owner. Chronological campaign narrative belongs in Git or in
the named research/ADR provenance route, not in a technical contract.

The current technical-owner set is `docs/technical/`,
`docs/reference/commands.md`, `docs/development/package-map.md`,
`docs/development/dependencies.md`, and the engineering/test policy documents.
The next inventory update will name every migration and replacement before any
larger process-document removal.

## Executed stabilization changes

- `internal/endpoint/enrollment/enrollment.go` had four distinct
  responsibilities at 496 lines: public verification/input projection, static
  inventory, canonical descriptor, and current-companion provenance. They are
  now separate cohesive files in the same package with unchanged public
  interfaces and existing behavior tests.
- [Closed-alpha enrollment verification](../technical/enrollment-verification.md)
  is the current technical contract for that module. It replaces the need to
  recover its interface and failure rules from historical enrollment/campaign
  narratives.
