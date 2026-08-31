# Pre-audit product-candidate baseline inventory

Status: **active remediation inventory; formal audit and qualification are not
in progress.**

This is a factual retention register for the stabilization work before the
next qualification candidate. It is not a release process or a second product
specification. Each entry identifies the current owner of any retained fact,
the evidence value, and the permitted disposition.

## Baseline rule

Historical evidence is split: RC1 has A1-A10; RC2 has a separate two-fresh-
Endpoint control result and A11 campaign; A12 is closure/disposition evidence.
No candidate has A1-A12, and none qualifies the post-refactor baseline. A future
candidate needs its own immutable source identity and qualification matrix. No
current document may imply otherwise.

## Confirmed retirement set

| Item | Current owner | Retained evidence | Disposition |
|---|---|---|---|
| `experiments/r-124-public-control-simulation/` | ADR-0055, R-124, ADR-0060, `internal/publiccontrol` and Release tests | Historical versioned JSON receipt/schema plus immutable Git evidence | **Runbook retired on 2026-08-29; generator and command retired on 2026-08-30.** No synthetic role became a product Interface. |
| `experiments/r-125-controlled-project-control-transitions/` | ADR-0056, R-125, ADR-0060, alpha-control and Release tests | Historical versioned JSON receipt/schema plus immutable Git evidence | **Runbook retired on 2026-08-29; generator and command retired on 2026-08-30.** Domain owners retain the safety assertions. |
| `experiments/r-126-project-control-canonical-name-lifecycle/` | ADR-0057, R-126, ADR-0060, Record/Epoch/Authority tests | Historical versioned JSON receipt/schema plus immutable Git evidence | **Runbook retired on 2026-08-29; generator and command retired on 2026-08-30.** The unique pending-fork assertion moved to its owner. |
| `experiments/r-127-project-control-root-claims/` | ADR-0058, R-127, ADR-0060, Claim/Epoch tests | Historical versioned JSON receipt/schema plus immutable Git evidence | **Runbook retired on 2026-08-29; generator and command retired on 2026-08-30.** Unique close and lease assertions moved to their owners. |
| `experiments/r-110-safe-endpoint-replacement/` | R-110, `internal/endpoint/replacement`, `endpoint replace` command/tests | The maintained interruption/recovery matrix and current H4-1B technical contract | **Retired on 2026-08-30.** Its two-file shell prototype had no unique current behavior or evidence. |

Each named research record retains its historical command and exact receipt
identity. ADR-0060 deliberately removes the reproduction route: the retained
receipt and Git revision are provenance, not a current command contract.

## Classified retention set

| Item | Classification | Required migration before retirement |
|---|---|---|
| `docs/research/horizon-4-program.md` | Historical research handoff, not current architecture | **Retired on 2026-08-29.** H4 scope/status and non-claims are in `docs/product/horizon-4/README.md`; active/open/decided question state is in `docs/research/questions.md` and the named records; maintained code/command boundaries are in `package-map.md` and `commands.md`; its dated campaign narrative is Git provenance. It had no inbound references. |
| `docs/research/s6-0-preparation.md` | Historical stage preparation | **Retired on 2026-08-29.** Canonical naming, claim ordering, recovery, admission, and current materialization are owned by ADR-0014 and ADR-0017--0020 plus `docs/technical/naming.md`; R-041--R-047/R-055/R-057 retain their decision evidence; its stage gate/campaign summary is Git provenance. It had no inbound references. |
| `docs/development/deep-audit.md` | Inactive, reusable formal engineering policy for one exact future candidate | **Retain.** It is intentionally not a current technical contract or qualification receipt; it owns formal audit evidence, post-discovery remediation, and requalification policy and is linked by the headless product-candidate objective. |
| `docs/development/closed-alpha-input-request.md` and `closed-alpha-release-ceremony.md` | RC2 process/provenance | **Retired on 2026-08-29.** The exact former command/schema/output contract is retained as historical provenance in [release-custody-assembly.md](../reference/release-custody-assembly.md); ADR-0059 removed the completed assembly routes. Historical RC2 execution evidence remains in the matrix and R-119/R-120/R-121. |
| `docs/product/horizon-4/08a-alpha-1-readiness-matrix.md` and `08b-alpha-1-release-profile.md` | split RC1/RC2 evidence ledger and RC1 profile provenance | Retain as corrected historical evidence; never aggregate candidates or represent either as qualification of a changed baseline. |
| `experiments/r-105-live-introduction-tracer/` and `r-117-firefox-proxy-auth/` | Active research | **Retain.** R-105 and R-117 are open records; their experiments remain their declared falsification route. |
| `experiments/r-115-firefox-zone-proxy/` | Retained negative Browser Entry evidence | **Retain.** R-115 directly cites the temporary PAC/add-on/native-host scripts and their resolver limitation; deleting them would remove its reproducible negative evidence. |
| `experiments/r-118-credential-relay/`, `r-118-entry-carrier/`, and `r-118-private-transit-issuance/` | Retained decision evidence | **Retain.** R-118 explicitly leaves their distinct negative-transcript crosswalk unresolved; do not remove them until a named maintained-test owner supersedes each case. |
| `experiments/r-096-browser-loopback/` | Retained browser-origin counterexample | **Retain.** R-096 directly cites its reproducible base/sandbox navigation probes; the generic H4-3B path does not supersede that static-fixture evidence. |
| `experiments/r-098-alpha-control-catalog/` | Retained authority-separation/parser counterexamples | **Retain.** R-098 directly cites its bounded synthetic reader and duplicate-key/oversize observations; the maintained ACA1 reader has a different wire contract. |
| `experiments/r-095-portable-endpoint-alpha-lifecycle/` | Retained bootstrap/lifecycle measurements | **Retain.** R-095 directly cites its GPG, SSHSIG, enrollment-digest, lifecycle, and Windows fixture evidence; maintained alpha enrollment uses a different first-contact contract. |
| `experiments/r-101-endpoint-state-attachment-profile/` and `r-102-endpoint-liveness-lock/` | R-101/R-102, `internal/endpoint/portable`, `runtime_test.go` | Measured ordinary-user/abrupt-loss/path-substitution findings retained in both records; maintained tests own roots, exclusive lock, expected stale socket only, unexpected-entry preservation, and path budget | **Retired on 2026-08-30.** No inbound experiment links remained. |
| `experiments/r-092-native-node-profile/` and `r-092-rendezvous-tracer/` | Retained native-Node and rendezvous counterexamples | **Retain.** R-092 directly links the profile source and tracer evidence; removing either would break its reproducible resource, pairing, and drain evidence. |
| `experiments/r-094-carrier-baseline/` and `r-094-carrier-seam-spec/` | Retained Carrier baseline and rebinding counterexamples | **Retain.** R-094 directly links both runbooks as the source of its bounded transport and path-rebinding observations. |

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

- `internal/enrollment/enrollment.go` had four distinct
  responsibilities at 496 lines: public verification/input projection, static
  inventory, canonical descriptor, and current-companion provenance. They are
  now separate cohesive files in the same package with unchanged public
  interfaces and existing behavior tests.
- [Closed-alpha enrollment verification](../technical/enrollment-verification.md)
  is the current technical contract for that module. It replaces the need to
  recover its interface and failure rules from historical enrollment/campaign
  narratives.
