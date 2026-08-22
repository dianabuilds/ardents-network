# Stage 8 current test portfolio

Status: **S8.0 factual inventory at source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.** This temporary record reports
the entry test tree and observed execution conditions. It is neither a test
policy nor a Qualification receipt. It is deleted at S8.6 after each retained
test/evidence fact has a current canonical owner.

## Inventory and roles

The following counts are source-navigation facts, collected from `*_test.go`
at the entry. "Named declarations" counts top-level `Test`, `Benchmark`, and
`Fuzz` declarations; it is not a count of subtests, assertions, or coverage.

| Surface | Test files | Named declarations | Current role |
|---|---:|---:|---|
| `cmd/` | 18 | 34 | Command/composition behavior |
| non-laboratory `internal/` | 165 | 449 | Maintained Module behavior |
| `internal/architecture` | 15 | 19 | Package-map and policy enforcement |
| `internal/lab/` | 103 | 264 | Historical reproduction/evidence |
| `tests/e2e/` | 28 | 11 | Cross-process tracer behavior |
| `tests/live/` | 98 | 71 | Host/Docker/network evidence |

The inventory totals 427 test files. `cmd/ardents-service`,
`cmd/blocked-entry-lab`, and `cmd/blocked-entry-verify-lab` have no package
test files. The no-test status is an observation, not evidence that their
compositions are supported or unsupported.

Current test roles are not yet a maintained profile. In particular, the
ordinary package set excludes `cmd/carrier-lab`, `cmd/named-site-lab`, and
`internal/lab/...`, while four other laboratory/evidence commands still occur
in the ordinary `go list ./cmd/... ./internal/...` set. S8.2 must assign each
suite exactly one retained role: product behavior, qualification/evidence,
historical reproduction, or removal.

## Observed execution timings

This audit used `GOENV=off`, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`, and
caches outside the repository. The following source set was run once with
`-short -shuffle=on -count=1`:

```
go test <all cmd and internal packages except cmd/carrier-lab,
cmd/named-site-lab, and internal/lab/...> -short -shuffle=on -count=1
```

It produced 43 terminal package result lines, all `ok` or explicit `[no test
files]`; no `FAIL` line was emitted. Package times are Go-reported elapsed
times, so parallel execution means they are not additive. The slowest observed
packages were:

| Package | Go-reported elapsed |
|---|---:|
| `cmd/stage6-verify-lab` | 46.465 s |
| `cmd/stage6-evidence-lab` | 46.212 s |
| `internal/bridge` | 43.639 s |
| `internal/network/state` | 41.801 s |
| `internal/route` | 9.105 s |
| `internal/network/store` | 3.573 s |
| `internal/nameresolution` | 3.562 s |
| `internal/architecture` | 3.459 s |

An `./tests/e2e/...` run was initiated with the same isolated cache, but a
terminal result was not captured in this audit record. It is therefore
**unmeasured**, not passed. `tests/live/...` was not run: its receipt requires
the selected host orchestration, Docker/network cells, and pinned external
inputs. Its timing is consequently **not applicable until an eligible live
environment is selected**. A later profile may measure it; it must not replace
this source-entry observation with an inferred pass.

## Explicit skips and external requirements

The source has the following explicit conditional skips at the entry:

| Condition | Current affected evidence |
|---|---|
| `-short` mode | A Release Decision corpus is skipped in `cells_test.go`. |
| Pinned external WebTunnel binaries | Linux camouflage tests require `ARDENTS_WEBTUNNEL_CLIENT` and `ARDENTS_WEBTUNNEL_SERVER`. |
| Host orchestrator | Live blocked-entry child cells are not independently runnable. |
| Elevated Windows symlink/junction privilege | Update-recovery corruption cases use alternative Windows cases when the privilege is unavailable. |
| Windows/Linux filesystem semantics | Historical module-cache and native-circuit tests skip where links or Unix permission bits cannot be represented. |
| Immutable Docker image IDs | Named-site and native-circuit laboratory integrations skip until their image variables are supplied. |
| Root-only Linux fixture construction | A blocked-verify laboratory permission test skips without it. |
| Private Gate-B seam | Three Update Transaction cleanup-overrun tests explicitly do not claim public `Recover` driver coverage. |
| Helper subprocesses | A name-resolution helper-process test is intentionally skipped in the normal parent run. |

These conditions define the minimum evidence inputs; they do not authorize
loosening a test. An absent input, skipped branch, or non-eligible host is an
unrun/invalid environment outcome rather than a pass.

## Duplicate-role and flake observations

No source-owned duplicate-role register or empirical flake campaign exists at
the entry. The audit has not classified overlapping unit/e2e/live/laboratory
tests as duplicates, and it has not run a repeated failure-rate experiment.
Accordingly, duplicate status and flake status are **unclassified**, not
"none". Static searches found retry and quarantine language used by product
behavior and architecture boundary tests, but that is not evidence of flaky
test behavior.

S8.2 must define a maintained manifest, owner, environment contract, expected
duration budget, retry rule, and receipt format for every retained suite before
G3 can be accepted. Until then the source-bound journey/claim trace remains
the only statement of which current tests are evidence for a named behavior.
