# Stage 8 current-system report

Status: **S8.0 in progress; this is the first factual diagnostic record.** It
is a temporary Stage 8 control document, not a target architecture, product
disposition, or a Qualification result. Its facts are frozen against the Stage 8 source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da` on 2026-08-22. It is deleted at
S8.6 after each fact has a current canonical owner.

## Scope and reproducibility

The entry is annotated by `stage7-stopped-2026-08-22`. The Stage 8 branch's
only difference from that source while this audit was performed was the
documentation-only start commit `d6e2be91b8b2d0ea1ab0ee6ea638e352007b6e29`:
`README.md`, the Stage 8 brief and workbook, and the Stage 8 start record. No
Go source, test, build, dependency, format, or configuration input changed.

The bound entry toolchain evidence is in the
[Stage 8 start record](stage-8-start-record.md): Go `1.26.6` on
`windows/amd64`, repository `GOTOOLCHAIN=local`, and the recorded `go.mod`,
`go.sum`, and `Makefile` digests. The source declaration remains Go `1.26.5`.
That difference is an S8.2/S8.3 decision input; it is not silently normalized
by this report.

The following diagnostics were reproduced with repository Go environment
settings and caches outside the repository:

| Diagnostic | Result | Scope and limitation |
|---|---|---|
| `make quick-check` | pass | Architecture, formatting, vet, unit, build, and module-integrity checks on the documentation-only derivative of the entry source. |
| `go test ./internal/releasedecision ./internal/updatetransaction ./cmd/ardents-release -count=1 -shuffle=on` | pass | Current release/update command and Module behavior; it is not an installed-product update proof. |
| `go test ./internal/updatetransaction -count=1 -shuffle=on` | pass | Reproduced R00--R14 recovery matrix, interruption, rollback, cleanup, storage, and bounded-pressure tests. Expected skips are recorded below. |

These are readiness diagnostics only. They do not prove a supported platform,
real activator, network, claim, or Stage 9 Qualification.

## Current source inventory

The [package map](package-map.md) is the exact source-bound package and import
inventory; its architecture test passed in the diagnostic run. Counts below are
navigation facts, not quality targets.

| Surface | Current inventory | Current classification |
|---|---:|---|
| Go packages | 60 | 15 commands, 30 non-lab product Modules, 14 historical-lab Modules, and `internal/architecture` |
| Production Go files | 722 | 66 command, 356 non-lab product, 299 historical-lab, 1 architecture |
| Test files | 427 | 18 command, 165 non-lab product, 103 historical-lab, 15 architecture, 28 end-to-end, 98 live |
| Markdown documents under `docs/` | 140 / 38,853 lines | Current product, security, development policy/map, and research/provenance material are mixed with completed-stage material |
| Direct runtime dependencies | 5 | CIRCL, OHTTP, Sigstore, go-tuf v2, and `x/sys`; the complete transitive closure is locked in `go.sum` |

### Commands and implementation boundaries

The nine non-laboratory commands are `ardents`, `ardents-bridge`,
`ardents-name`, `ardents-node`, `ardents-publish-app`, `ardents-release`,
`ardents-route`, `ardents-service`, and `ardents-stream-app`. The other six
commands are historical laboratory/evidence commands: `blocked-entry-lab`,
`blocked-entry-verify-lab`, `carrier-lab`, `named-site-lab`,
`stage6-evidence-lab`, and `stage6-verify-lab`.

The current non-lab Modules are application IPC; Bridge and WebTunnel
camouflage; local roles; seven naming Modules; Network epoch/source/state/store
Modules; node and probe; plan files; Release Decision; resource; Route and
route plan; Service Connection and Endpoint; stream workload; and Update
Transaction. The fourteen `internal/lab/*` roots remain present as historical
reproduction/evidence code. Their complete responsibilities, permitted
imports, and command ownership are source-checked in the package map.

Current durable roots are nevertheless owned by several current Modules and
command plans: Network State, local roles, Bridge, Release Decision, Update
Transaction, naming store, and candidate/carrier state. This is an inventory
fact, not an authorization to merge them. S8.3 must decide the target writer,
format disposition, compatibility reader, and rollback/forward-repair rule for
each root before a migration begins.

No `internal/custody`, `internal/endpoint`, `internal/application`,
`internal/network/duty`, `internal/naming/namespace`,
`internal/service/publication`, or `internal/route/webtunnel` package exists
at the entry. Those names in preparatory G1 material are proposed destinations,
not current architecture.

## Test and evidence inventory

Current test surfaces are package behavior, command tests, `tests/e2e`, live
Docker/network tests, and historical laboratory/evidence packages. The
end-to-end suites are `blocked-entry`, `blocked-entry-lab`, `network-source`,
`node`, `operations`, `route`, and `service`; live suites are `blocked-entry`,
`network`, and `stage5-final`.

The current test tree has explicit environment-conditioned omissions. In
particular, Release skips a corpus in short mode; WebTunnel tests require two
pinned external binaries; live blocked-entry child cells run only under their
host orchestrator; some Windows symlink/junction cases require elevated
privilege; and historical laboratory integrations require pinned images or
Linux permission semantics. `internal/updatetransaction` also contains three
private cleanup-overrun tests marked as Gate-B seam coverage rather than public
Recover-driver coverage. A missing prerequisite is evidence of an invalid or
unrun environment, never a passing coverage receipt.

The current Make targets retain Stage 7 classification: `unit` subtracts
`cmd/carrier-lab`, `cmd/named-site-lab`, and `internal/lab/...`, but four other
laboratory/evidence commands remain in the ordinary package set. The accepted
G3 test model is therefore planning input only until S8.2 promotes a current
profile/manifests policy.

The source-bound [journey and claim trace](stage-8-current-system-trace.md)
names the current caller and observable evidence (or explicitly records its
absence) for every accepted H3 journey and security/privacy invariant.

## Clean-baseline discrepancies and findings ledger

S8.0 compares the entry against the preparation identities instead of merging
their counts. The G2 source audit began at
`e843f556dfb003c7aa8862fe2e4095ddc134ae49`; from that commit through the
Stage 8 entry, 83 source/test files changed by 9,235 added and 234 removed
lines. The dominant delta is 45 production and 30 test files in
`internal/updatetransaction`; that package now has 77 declared tests. The
prepared G2 count of 319 non-lab product files is consequently stale; the
entry contains 356.

| ID | Factual result at the entry | Required owner before mutation |
|---|---|---|
| S8.0-F01 | Prepared G2 findings that described the pre-S7.2 Update Transaction, especially F008--F011, are not current findings merely because the package now has a passing recovery corpus. Their authority, state/format, lock, Adapter, and real-product claims require a focused current review. | S8.1 product disposition and S8.3 update/release design |
| S8.0-F02 | No Custody Module exists, while current Release/Update records still carry custody-limitation data. | S8.1 preservation decision, then S8.3/M12 |
| S8.0-F03 | The preparation baseline identifies Go `1.26.5`; the accepted entry test used `1.26.6` on Windows. Current live and platform evidence is not one source-identified supported-platform profile. | S8.2 test/toolchain policy and S8.3 platform/compatibility design |
| S8.0-F04 | G1 target names are absent from the source tree, and current durable state is distributed across stage-derived Modules/commands. | S8.1 preservation and S8.3 target/format map |
| S8.0-F05 | Historical laboratory and evidence code is still in the build/test graph, while G3 requires an explicit distinction among product behavior, qualification, and reproduction. | S8.1 product/claim disposition and S8.2 test policy |
| S8.0-F06 | The prepared document baseline was 138 Markdown files; the entry has 140. Current product truth, engineering policy, stage records, and research provenance remain co-located under `docs/`. | S8.2 documentation policy and M14 retirement ledger |

The recorded results neither invalidate the accepted G0--G5 preparation nor
confirm it wholesale. They identify exactly where the final Stage 7 delta must
be reviewed before its target disposition is accepted.

The source-bound [Release and Update delta review](stage-8-g2-release-update-delta-review.md)
now gives F001--F012 their current disposition. It confirms the prior
marker-lock and missing-recovery statements no longer describe the current
Update tracer, while preserving the unresolved authority, product-activation,
format, and Custody decisions.

The source-bound [Network State delta review](stage-8-g2-network-state-delta-review.md)
gives F013--F022 their current disposition: the entry contains no relevant
production delta, so all ten prepared findings remain live design inputs, with
F014 an explicit authority conflict that must be decided before state repair.

The source-bound [naming and private-resolution delta review](stage-8-g2-naming-delta-review.md)
gives F023--F033 their current disposition. No relevant production delta exists;
all eleven findings remain live inputs, with F027 and F032 requiring accepted
research/contract decisions before their respective mutations.

The source-bound [Service and Endpoint composition delta review](stage-8-g2-service-composition-delta-review.md)
gives F034--F040 their current disposition. No relevant production delta exists;
all seven findings remain live design inputs before any Service migration.

The source-bound [runtime, command, and policy delta review](stage-8-g2-runtime-policy-delta-review.md)
completes the G2 F001--F067 entry disposition. Only F008 and the former
recovery portion of F011 are invalidated by the Stage-7 delta; all other
findings remain open inputs to S8.1--S8.3.

## Open-decision ledger for S8.1

1. Select `continue`, `narrow`, `redesign`, or `stop` for the Product Core and
   H3 candidate.
2. Mark every command, Module, durable root, format, test/evidence suite, and
   document as `preserve`, `migrate`, `replace`, `remove`, or `decide first`.
3. Decide whether the current Release/Update tracer behavior and formats are
   retained product scope, a bounded compatibility reader, historical evidence,
   or removed; resolve the root-rotation transaction semantic before changing
   its codec or recovery rule.
4. Decide which route/entry, network-source, WebTunnel, platform, public
   command, and historical-laboratory surfaces are in the maintained product.
5. Select a source-identified supported toolchain/platform profile and the
   rule for unavailable Docker, pinned binary, privilege, or host-orchestrator
   prerequisites.

No answer is inferred by this diagnostic report. Until an S8.1 disposition is
accepted, the Stage 7 package, testing, dependency, and documentation rules
remain binding.

## S8.0 exit assessment

The source/input identity, initial current-system inventory, diagnostic
results, baseline discrepancies, and open-decision ledger are recorded. This
does **not** yet satisfy the S8.0 exit. Before Product Owner review, this report
needs (1) a source-bound inventory of every process, format, Interface, external
Adapter, and trust-zone crossing; (2) per-suite timing, skip,
external-requirement, duplicate-role, and flake observations; (3) a complete
code/test/document disposition inventory that distinguishes inventory from the
S8.1 decision; and (4) an evidence-based disposition of every prepared G2
finding affected by the Stage 7 delta. It has not passed S8.1 and authorizes no
Module migration.
