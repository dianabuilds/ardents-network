# Test Suites

`tests/` contains repository-level `integration` and `e2e` layers plus shared
`testkit` helpers.

Execution model:

- `tests/run.ps1` always executes suites inside the Linux test container;
- Windows is a host orchestration surface, not a test runtime;
- the Linux test container is the canonical `fast` runtime on every supported
  host, including the GitHub-hosted Ubuntu runner used by CI;
- the fast suite executes `go test ./...` inside that container;
- `integration` tests are opt-in via `-tags integration`;
- `e2e` tests are opt-in via `-tags e2e`;
- tagged suites compile stable Linux binaries under `.artifacts/testbin/` and
  remove those temporary binaries in `finally`, while retaining the existing
  canonical JSON/JUnit reporting contract;
- Docker volumes `ardents-go-mod-cache` and `ardents-go-build-cache` retain
  dependency/build caches without writing them into the Windows workspace;
- `-EphemeralCache` replaces those named caches with anonymous volumes that
  Docker removes with the test container; use it for clean release/CI evidence,
  not for the normal incremental loop;
- `.dockerignore` excludes workspace caches, reports, runtime data, IDE state,
  and Git markers from every Docker build context.

Practical entry points:

- `powershell -NoProfile -File tests/run.ps1 fast`
- `powershell -NoProfile -File tests/run.ps1 integration`
- `powershell -NoProfile -File tests/run.ps1 e2e`
- `powershell -NoProfile -File tests/run.ps1 all`
- `powershell -NoProfile -File tests/run.ps1 all -EphemeralCache`
- `powershell -NoProfile -File tests/run.ps1 integration -Domain network-foundation -Scenario NFI-001`
- `powershell -NoProfile -File tests/run.ps1 e2e -Domain network-foundation -Scenario NFE-001`
- `powershell -NoProfile -File tests/resource-snapshot.ps1 -Label manual`

Cache policy:

- normal focused tests reuse the external host Go cache and the two named
  Docker volumes because deleting them after every run makes development much
  slower;
- `scripts/clean-go-cache.ps1 -StatusOnly` reports the host cache, and the
  normal command removes it only above 5 GiB;
- `scripts/clean-docker-cache.ps1 -StatusOnly` reports only Ardents Docker Go
  caches, and the normal command removes only those two volumes above 8 GiB;
- run both cleanup commands at leaf/release checkpoints, when disk space is
  low, or while diagnosing stale output; `-Force` is for an intentional cold
  rebuild;
- `scripts/clean-docker-cache.ps1 -BoundBuildKit` additionally bounds the
  Docker-wide BuildKit cache. It is opt-in because BuildKit is shared with
  other repositories; never replace it with a broad automatic system prune.

## Mandatory Repository Gate Matrix

This is the single local/CI gate contract. CI jobs may split the rows across
workers, but must call these commands and must not reproduce their selection or
reporting semantics in workflow-specific scripts.

| Order | Gate | Canonical command | Pass condition |
|---:|---|---|---|
| 1 | formatting | `powershell -NoProfile -File tests/check-format.ps1` | no handwritten Go file under `boundary`, `cmd`, `internal`, or `tests` is listed by `gofmt` |
| 2 | static analysis | `go vet ./...` | process exits zero with no vet finding |
| 3 | architecture acceptance | `go test ./tests/tooling/archaccept -count=1` | file ceilings, package documentation, service composition, agent tooling, and private protocol boundaries match the machine-readable policy |
| 4 | release source identity | `powershell -NoProfile -File tests/ci/release-source-identity-gate.ps1` | mismatched commits and dirty source fail before Docker; clean HEAD builds from an exported snapshot that excludes ignored files |
| 5 | release materials policy | `powershell -NoProfile -File tests/ci/release-materials-policy-gate.ps1` | every release builder/runtime is digest-pinned, policy-bound, and independent rebuilds cannot share mutable caches |
| 6 | audit traceability | `go run ./tests/tooling/audittrace` | every audit P1 has existing critical files, exact deterministic evidence, and a declared CI gate |
| 7 | critical diff + lifecycle race | `go run ./tests/tooling/audittrace -base <merge-base>` followed by the `critical-lifecycle` CI job | changed critical files remain mapped; ingress, replay, Waku, identity, stream, Docker, content, transfer, and configuration contracts pass under `-race` |
| 8 | fast + import boundary | `powershell -NoProfile -File tests/run.ps1 fast -CoverageProfile tests/.artifacts/coverage/fast.out` | import guard and default `go test ./...` both exit zero; coverage is retained |
| 9 | integration | `powershell -NoProfile -File tests/run.ps1 integration -ReportDir tests/.artifacts/reports/integration` | runner exits zero and summary reports zero failures |
| 10 | E2E | `powershell -NoProfile -File tests/run.ps1 e2e -ReportDir tests/.artifacts/reports/e2e` | runner exits zero and summary reports zero failures |
| 11 | reachable vulnerabilities | `powershell -NoProfile -File tests/ci/security-gate.ps1` | exact finding IDs and reachability agree with the active exception register; any drift fails |
| 12 | vulnerability evidence | produced by the same security gate | pinned JSON, verbose output, and reconciliation JSON are retained under `tests/.artifacts/security` |

`tests/run.ps1 all -ReportDir tests/.artifacts/reports/all` is the release
cross-check for tag interaction. It does not replace the separately attributable
integration and E2E jobs.

All commands are run from the repository root with the `go.mod` toolchain. A
managed environment that cannot write the configured external Go cache may
point `GOCACHE` at a task-specific system temporary directory, never inside
the repository. A source snapshot without valid Git
metadata may set `GOFLAGS=-buildvcs=false`; the job must record either
normalization in its retained environment evidence.

The matrix is fail-closed: warnings, missing summary/JUnit files, non-zero child
processes, and vulnerability drift cannot be converted into a successful job
by wrapper logic. The security gate permits only the exact IDs and reachability
documented in `docs/security/security-exceptions.md`; disappearance also fails until the
stale exception is deliberately removed. Earlier findings were Phase 1
stabilization work, not an implicit waiver of gates 11–12.

`tests/ci/audit-test-traceability.json` is the executable audit-to-test matrix.
The verifier derives the required P1 set from the remediation backlog instead
of trusting a duplicated list. It resolves every referenced file and Go test
symbol, proves that the owning CI job selects the Go package or transitively
executes the contract script, and rejects a changed file in any declared
critical package or script area unless deterministic evidence covers that
exact file. The `critical-lifecycle` job is a dependency of both the fast and
tagged suites, so release work cannot bypass its selected race and lifecycle
contracts.

Selection flags for tagged suites:

- `-Domain <domain>`
- `-Scenario <scenario-id>`
- `-Tag <tag>`
- `-Profile <suite-profile>`

Expected tagged-suite artifacts when selection or explicit report paths are used:

- raw per-test JSON captures under `tests/.artifacts/reports/<suite>/raw/`
- canonical JSON summary under `tests/.artifacts/reports/<suite>/summary.json`
- JUnit-compatible export under `tests/.artifacts/reports/<suite>/junit.xml`

Focused test selection uses `testkit.Spec` metadata declared directly in test
code. `testcatalog` reads only that executable metadata; it does not depend on a
parallel scenario-document or requirement-coverage hierarchy.

Artifact publication and storage rules:

- `fast` keeps console-only output unless a caller opts into extra artifacts
- `integration` and `e2e` CI jobs must publish `summary.json`, `junit.xml`, and
  raw per-test JSON under `tests/.artifacts/reports/<suite>/`
- raw tagged-suite captures are the canonical debugging payload and should be
  retained at least on failures and release-gate runs
- failed integration/E2E jobs must retain raw JSON, `summary.json`, `junit.xml`,
  console output, and the environment/toolchain snapshot for at least 30 days
- release-candidate runs must retain those artifacts, both
  vulnerability outputs, and checksums with the release evidence for the whole
  supported lifetime of that release
- a job that cannot publish its required failure or release artifacts is itself
  failed; a later retry must not erase the earlier failure evidence

DR-06 retention and environment contract:

- Linux test container is the canonical `fast` runtime; Windows supplies only
  the separately retained `windows-interface` orchestration prerequisite.
- The current `native-install` job runs a privileged systemd acceptance
  container on a GitHub-hosted Ubuntu runner. That is the executable CI truth,
  but it does not become equivalent to a real Linux systemd host unless a
  maintainer explicitly accepts that environment for the native-installation
  claim.
- Every DR-06 workflow artifact name includes the GitHub `run_id` and
  `run_attempt`. A rerun therefore cannot replace an earlier artifact by name.
- Every actually executed gate attempt emits an `environment.json` manifest
  before its canonical command. The manifest binds the source SHA, declared
  environment, runner image identity, Go/PowerShell/Docker versions, workflow
  and gate-contract hashes, and immutable release/test base materials.
- Before indexing a retry, CI downloads the complete workflow log for every
  earlier attempt. The index rejects any failed attempt without that log or
  without its complete declared attempt evidence: environment identity, raw
  results, summary, JUnit, resource captures, and gate-specific outputs.
- Every third-party Action in the qualification workflow is pinned to an exact
  commit. Mutable major-version tags are not part of commit-bound evidence.
- `qualification-index` runs after the whole DAG even when a dependency fails
  or is skipped. A strict validation failure produces and uploads a
  commit-bound `status=rejected` aggregate before the workflow remains failed.
- A DR-06 `workflow_dispatch` never publishes a release, including when it is
  dispatched from a tag ref. The existing push-tag release path remains a
  separate Release-owner operation outside this qualification program.
- GitHub Actions provides 90-day staging retention for the candidate packet.
  Before Release-owner acceptance, every success and failure attempt must be
  exported to the approved immutable destination and retained for the supported
  lifetime of the candidate. The commit-bound index records that destination;
  an upload to workflow staging alone is not durable release evidence.
- After exporting and reading the packet back from that destination, run
  `powershell -NoProfile -File tests/ci/confirm-dr06-retention.ps1
  -IndexPath <restored-packet>/dr06-index/qualification-index.json
  -EvidenceRoot <restored-packet>/dr06-download
  -DurableEvidenceURI <approved-reference> -ReceiptPath <receipt.json>`.
  The verifier checks every indexed size and SHA-256 and emits a non-accepting
  receipt for Release-owner review; a receipt never sets `Q=yes`.
- A same-run retry remains visible in the index. An earlier failure requires an
  explicit Release-owner disposition and a clean rerun policy; a later green
  job never silently replaces it.

Release and review use cases:

- `fast`: default developer and pre-merge signal for non-tagged coverage
- `integration`: domain remediation, bug reproduction, and review-time runtime
  validation
- `e2e`: operator-facing lifecycle, participation, and recovery validation
- `all`: release-candidate and final QA gate
- `-Domain`, `-Scenario`, and `-Tag` selections are the focused review path for
  release-sized or incident-specific sweeps

Workflow coverage:

- local shell: `tests/run.ps1` is the canonical container-suite entry point
- IDE: configure the same `tests/run.ps1` command as an external tool/task;
  direct Windows `go test` is diagnostic-only and cannot produce acceptance
  evidence
- CI: canonical commands must call the same runner and validation paths instead
  of inventing workflow-specific selection logic

Multi-host network recovery:

- `tests/run-multihost.ps1` is the canonical STB-307 testnet runner;
- it builds the Linux testnet image when missing and then runs the topology in
  Docker networks; `-BuildMode Never` and `Always` remain explicit overrides;
- `tests/resource-snapshot.ps1` captures CPU, RAM, disk, `vmmemWSL`, and top
  processes before/after canonical suite runs and is the first diagnostic step
  whenever a run slows down.

Windows note: `tests/setup-windows-toolchain.ps1` is retained only for focused
toolchain diagnostics. It is not part of the canonical gate and its output cannot
replace Linux-container evidence.

`tests/testkit` is the only place for cross-package test harness code. Repeated
polling helpers, auth helpers, connect-rpc wiring, node bootstrap setup and
future reporters should accumulate there instead of being copied between test
packages.
