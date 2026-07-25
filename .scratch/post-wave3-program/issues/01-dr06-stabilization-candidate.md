# DR-06: Qualify the existing stabilization candidate

Status: ready-for-agent
State: open
Labels: ready-for-agent
Class: R3 qualification program

## Parent

`../PRD.md`

## User story

As a release reviewer, I want the existing Ardents stabilization capabilities
to be evaluated by one finite, commit-bound qualification program so that I can
accept or reject exact capability claims without implicitly including
unimplemented Wave 3 features or treating local test success as a production
release.

## Full vertical behavior

DR-06 selects one clean source commit and release-candidate version, executes
the complete canonical gate DAG in the supported environments, retains both
success and failure evidence, and produces one aggregate qualification index.
The index binds the source commit, version, exact commands, start and end times,
runner and toolchain identities, immutable base materials, outcomes, attempt
history, artifact paths, artifact hashes, and the capability claims covered by
each gate.

The Release owner evaluates each capability independently against its declared
gate intersection. A passing aggregate may promote only capabilities whose
complete required gates passed on the same commit. A failed, unavailable,
unattributed, or unpublished gate keeps every dependent capability at `Q=no`.
The result updates release-facing truth without changing implementation,
reachability, or operability status unless separate product evidence justifies
such a change.

## In scope

- Reconcile the canonical environment contract for `fast` and
  `native-install` before execution.
- Execute and retain evidence for `static`, `fast`, `tagged`,
  `application-process`, `network-foundation`, `workload-integration`,
  `security`, `deployment`, `native-install`, `multinode`, `release-builds`,
  and `release-candidate`.
- Preserve the prerequisite `critical-lifecycle`, `windows-interface`, and
  deliberate failure-contract results used by the release DAG.
- Qualify exactly the fifteen existing capabilities listed under
  [Capability impact](#capability-impact).
- Reconcile the active security-exception register against the exact
  vulnerability scan output.
- Record unavailable environments and failed attempts as first-class evidence.
- Produce a durable, commit-bound aggregate decision and update release-facing
  status only after Release-owner review.

## Out of scope

- `application.discovery`
- `application.messaging`
- `application.hosting`
- `service.direct-interaction`
- `realm.channel-grant-authority`
- `deployment.multi-host`
- Real three-host support from ADR-0013; the existing `multinode` gate is the
  segmented Docker QA contract only.
- ADR-0011 through ADR-0015 acceptance or implementation.
- Kubernetes, schedulers, QUIC, WebTransport, WebRTC, automatic NAT traversal,
  remote Application transport, and non-Go SDKs.
- New product behavior, broad refactoring, dependency replacement, release
  publication, image promotion, or pushing Git changes.
- Treating the historical `75471a6` stabilization snapshot, local Windows
  diagnostics, or an earlier product baseline as matching-commit evidence.

## Dependencies

### Program dependencies

- The post-Wave 3 admission check passes: capability catalogue is valid with
  24 capabilities, 8 domains, and 0 qualified before DR-06.
- The selected qualification commit contains all intended stabilization
  remediation and regression tests and has a clean worktree.
- Release owner supplies an exact release-candidate version and authorizes the
  canonical hosted/candidate runners. This issue does not authorize publishing
  a release.
- Environment-contract inconsistencies are resolved:
  - `fast` currently says Windows in the catalogue/synthesis while the
    canonical test runtime and CI job are Ubuntu-hosted Linux containers;
  - `native-install` currently says a Linux systemd host while CI uses a
    privileged systemd acceptance container.
- Evidence retention is made durable for the supported lifetime of the
  candidate; current 1-day, 30-day, and 90-day workflow retention is
  insufficient as a final release record.

### Gate dependency order

```text
static -> critical-lifecycle -> fast/tagged
static -> security/deployment/native-install/multinode
windows-interface + failure-contract + all required gates
  -> independent release-builds A and B
  -> release-candidate verification
  -> aggregate capability decision
```

The `application-process`, `network-foundation`, and
`workload-integration` claims are focused tagged selections. They must remain
independently attributable even if CI executes them inside the complete tagged
suites.

## Authority and state ownership

- The Release owner owns the qualification run, aggregate evidence index,
  release-candidate decision, and any proposed `Q` promotion.
- Each capability's domain owner and evidence owner remain responsible for the
  meaning and adequacy of their claim-specific evidence.
- `docs/engineering/capabilities.json` remains the sole editable capability
  truth. Its generated register is not edited directly.
- CI runner results and retained artifacts are evidence projections. The
  aggregate index is the authoritative record of which exact attempt and
  artifact set the decision consumed; mutable console state is not
  authoritative.
- Historical audit documents remain immutable. Current reconciliation belongs
  in the remediation ledger and the new commit-bound qualification snapshot.
- A workflow retry never overwrites or invalidates the earlier attempt. Both
  attempts remain linked, and a flake must be resolved before a fresh
  qualification run can pass.
- No evidence artifact may contain Credentials, Bootstrap or Application
  tickets, Channel Grants, private selectors, private endpoints, payloads, or
  Principal identifiers that are not already approved public metadata.

## Gate execution and evidence matrix

Every row uses the same clean input commit. Unless stated otherwise, pass
requires a zero process exit, every required report to exist and publish, and
no failed scenario. Missing or unpublished evidence is a failure.

| Gate | Exact command or runner | Required environment | Retained evidence | Owner | Capability claims |
|---|---|---|---|---|---|
| `static` | `.github/workflows/ci.yml` job `static`: `./scripts/generate-api.ps1 -Check`; `./tests/check-format.ps1`; `go vet ./...`; tooling tests; tagged catalogue generation; `go run ./tests/tooling/capabilitycatalog -check`; `go run ./tests/tooling/audittrace`; entrypoint, source-identity, and materials-policy gates | Clean Ubuntu 24.04, `go.mod` toolchain, pinned generators | Catalogue JSON/text plus a commit/environment/command/timing manifest and hashes | Engineering / QA | All fifteen |
| `fast` | `./ardents.ps1 test fast -RebuildContainer -CoverageProfile tests/.artifacts/coverage/fast.out` | Resolved canonical host contract plus clean Linux Docker test runtime | Coverage, resource snapshots, environment manifest, console result and hashes | Engineering / QA | `node.lifecycle`, `operator.command-interface`, `identity.principal-access`, `application.installation-content`, `network.waku-foundation`, `discovery.operator-resolution`, `content.operator-lifecycle`, `transfer.replication`, `workload.lifecycle`, `hosting.operator-publication`, `operations.diagnostics`, `operations.configuration-reload` |
| `tagged` | `./ardents.ps1 test integration -ReportDir tests/.artifacts/reports/integration`; `./ardents.ps1 test e2e -ReportDir tests/.artifacts/reports/e2e` | Clean Ubuntu host and Linux test container | Raw per-test JSON, `summary.json`, `junit.xml`, resource snapshots, environment manifest and hashes for both suites | Engineering / QA | `node.lifecycle`, `operator.command-interface`, `identity.principal-access`, `discovery.operator-resolution`, `content.operator-lifecycle`, `transfer.replication`, `operations.diagnostics` |
| `application-process` | `./ardents.ps1 test e2e -Domain application-interface -Scenario APP-001 -ReportDir tests/.artifacts/reports/application-process` | Linux container with Unix-domain-socket Application fixture | Focused raw JSON, summary, JUnit, selector identity and hashes | Application Interface / QA | `application.installation-content` |
| `network-foundation` | `./ardents.ps1 test integration -Domain network-foundation -Scenario NFI-001 -ReportDir tests/.artifacts/reports/network-foundation` | Canonical local Linux Waku fixture | Focused raw JSON, summary, JUnit, selector identity and hashes | Network Foundation / QA | `network.waku-foundation` |
| `workload-integration` | `./ardents.ps1 test integration -Domain workload -Scenario WKI-001 -ReportDir tests/.artifacts/reports/workload-integration` | Canonical local Linux workload fixture | Focused raw JSON, summary, JUnit, selector identity and hashes | Workload and Hosting / QA | `workload.lifecycle`, `hosting.operator-publication` |
| `security` | `./tests/ci/security-gate.ps1` | Clean Ubuntu hostile-input runner with Docker and pinned vulnerability tooling | `govulncheck.jsonl`, verbose output, `reconciliation.json`, environment manifest and hashes | Security / QA | `identity.principal-access`, `application.installation-content`, `network.waku-foundation`, `release.artifacts-provenance` |
| `deployment` | `./tests/ci/deployment-gate.ps1 -Build` | Clean Linux Docker/Compose runner | Upgrade/backup, rollout transaction, composite readiness, three-Node status, logs, manifest and hashes | Operations / QA | `workload.lifecycle`, `hosting.operator-publication`, `operations.configuration-reload`, `operations.backup-upgrade-rollback` |
| `native-install` | `./tests/ci/native-install-gate.ps1` | Resolved supported Linux systemd environment | Native lifecycle evidence, systemd logs, pass marker, environment manifest and hashes | Operations / QA | `operations.native-installation` |
| `multinode` | `./tests/ci/multihost-gate.ps1 -BuildMode Always -ReportDir tests/.artifacts/reports/multihost` | Canonical segmented Linux multinode QA environment | Versions, summary, snapshots, Compose logs, optional stability samples, manifest and hashes | Network and Operations / QA | `network.waku-foundation`, `discovery.operator-resolution`, `transfer.replication` |
| `release-builds` | Two independent jobs, each running `./ardents.ps1 package $env:RELEASE_VERSION -OutputDir dist/repro` | Clean immutable Ubuntu runners with independent disposable caches | Complete `release-repro-a` and `release-repro-b`, source/toolchain/base identity and hashes | Release | `operations.backup-upgrade-rollback`, `operations.native-installation`, `release.artifacts-provenance` |
| `release-candidate` | Existing CI block: compare A/B `SHA256SUMS`; `./scripts/release/verify.ps1`; release metadata and image gates; pinned Debian smoke | Canonical clean Linux release environment | Verified final payload plus aggregate upstream evidence index and hashes retained for the supported lifetime | Release | All fifteen |

## Retry and failure policy

- A gate is run once for an accepted qualification attempt.
- A failing gate cannot be rerun and reported as if the first attempt did not
  exist.
- If infrastructure or a suspected flake requires another attempt, both
  attempts, their `run_id`/`run_attempt`, logs, artifacts, and classification
  are retained.
- A flake must be reproduced or remediated before acceptance. The affected
  gate then runs again from a clean environment on the same source commit.
- A changed source commit invalidates every earlier gate for the new aggregate.
- An unavailable environment is a failed qualification dependency, not a
  waiver.

## Acceptance criteria

- [ ] The issue records one exact clean source commit and release-candidate
      version.
- [ ] The canonical `fast` host/runtime contract is consistent across the
      synthesis, capability catalogue, test documentation, and workflow.
- [ ] The accepted `native-install` systemd environment is consistent across
      the same sources and is strong enough for the claimed native lifecycle.
- [ ] Static admission reports a valid catalogue with exactly 24 capabilities,
      8 domains, and 0 qualified before the final decision.
- [ ] All twelve gates in the execution matrix pass on the same commit.
- [ ] `critical-lifecycle`, `windows-interface`, and deliberate
      failure-contract prerequisites pass and are linked from the aggregate.
- [ ] APP-001, NFI-001, and WKI-001 results are independently attributable,
      even when executed as part of a larger tagged suite.
- [ ] Integration and E2E evidence contains raw JSON, canonical summary, JUnit,
      resource information, environment identity, and hashes.
- [ ] Deployment evidence covers upgrade/backup, transactional rollout,
      composite readiness, restart/recovery, and the declared three-Node
      product-ready checks.
- [ ] The existing multinode gate is described only as segmented Docker QA and
      makes no ADR-0013 real-host claim.
- [ ] Security evidence reconciles the exact active finding and reachability
      set with `docs/security/security-exceptions.md`; drift in either
      direction fails.
- [ ] Independent release builds use separate clean runners/caches and produce
      identical verified artifacts for the selected commit and version.
- [ ] The aggregate index includes exact commands, commit, version, start/end
      times, runner/toolchain/base identities, results, attempt history,
      artifact locations, and SHA-256 hashes.
- [ ] Success and failure evidence is retained for the full supported lifetime
      of the release candidate; a later attempt does not erase an earlier one.
- [ ] No artifact exposes prohibited identity, authority, endpoint, selector,
      payload, or secret material.
- [ ] Each of the fifteen capabilities is evaluated against its own complete
      gate set; partial success does not cause a blanket promotion.
- [ ] README, Changelog, remediation ledger, and capability truth describe the
      accepted or rejected result without overstating production readiness.
- [ ] The final qualification worktree is clean.

## Required tests and evidence

Preparation checks:

```text
go run ./tests/tooling/capabilitycatalog -check
go test ./tests/tooling/... -count=1
git diff --check
git status --short
```

The complete required execution is the command matrix above, including both
tagged suites and both independent release builds. The final evidence packet
must include:

- a source identity and clean-worktree record;
- a toolchain, generator, Docker engine, runner image, and immutable release
  material inventory;
- one manifest entry for every command and prerequisite job;
- raw, JSON, JUnit, coverage, security, deployment, native, multinode, and
  release artifacts as applicable;
- SHA-256 hashes for every retained artifact;
- explicit pass, fail, skipped, and unavailable outcomes;
- complete retry/attempt history;
- the per-capability gate intersection and final Release-owner disposition.

Local Windows `go test`, the historical stabilization evidence at `75471a6`,
the existence of test code, and Docker multinode evidence for a real
three-host topology are not substitutes for this packet.

## Capability impact

| Capability | Required DR-06 gates |
|---|---|
| `node.lifecycle` | static, fast, tagged, release-candidate |
| `operator.command-interface` | static, fast, tagged, release-candidate |
| `identity.principal-access` | static, fast, security, tagged, release-candidate |
| `application.installation-content` | static, fast, application-process, security, release-candidate |
| `network.waku-foundation` | static, fast, network-foundation, multinode, security, release-candidate |
| `discovery.operator-resolution` | static, fast, tagged, multinode, release-candidate |
| `content.operator-lifecycle` | static, fast, tagged, release-candidate |
| `transfer.replication` | static, fast, tagged, multinode, release-candidate |
| `workload.lifecycle` | static, fast, workload-integration, deployment, release-candidate |
| `hosting.operator-publication` | static, fast, workload-integration, deployment, release-candidate |
| `operations.diagnostics` | static, fast, tagged, release-candidate |
| `operations.configuration-reload` | static, fast, deployment, release-candidate |
| `operations.backup-upgrade-rollback` | static, deployment, release-builds, release-candidate |
| `operations.native-installation` | static, native-install, release-builds, release-candidate |
| `release.artifacts-provenance` | static, security, release-builds, release-candidate |

No capability may move to `Q=yes` until every gate in its row passes against
the same clean commit and the Release owner accepts the aggregate evidence.
This issue alone changes no `I`, `R`, `O`, or `Q` value.

## Expected files and modules

Qualification preparation or evidence fixes are expected to remain within:

- `.github/workflows/ci.yml`;
- `tests/README.md`;
- `tests/run.ps1`, `ardents.ps1`, and focused `tests/ci/*-gate.ps1` runners;
- release scripts under `scripts/release/`;
- a new commit-bound evidence snapshot under `docs/engineering/evidence/`;
- `docs/engineering/current-remediation-ledger.md`;
- `README.md` and `CHANGELOG.md`;
- `docs/engineering/capabilities.json`, only for a reviewed final
  qualification snapshot or status promotion;
- the generated capability register, regenerated from the canonical JSON.

Product implementation packages are not expected to change. A gate-discovered
product defect must leave this R3 issue as failed/blocked evidence and return
to a separately scoped R0-R2 implementation issue rather than being fixed
silently inside the qualification snapshot.

## Exit condition

This issue closes only when one of these outcomes is recorded:

1. **Accepted:** the complete gate matrix and prerequisites pass on one clean
   commit, the durable aggregate evidence is reviewed, each capability receives
   an explicit claim-level disposition, truthful release documentation is
   updated, and the worktree is clean; or
2. **Rejected:** at least one required gate, environment, artifact, security
   reconciliation, reproducibility check, or evidence-integrity rule fails,
   the failure is retained and linked to a follow-up issue, every affected
   capability remains `Q=no`, and no production release claim is made.

Passing only the locally available preparation checks is not an exit.
