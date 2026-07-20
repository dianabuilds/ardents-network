# STB-606 Evidence — CI And Release Candidate Automation

## Accepted Product Properties

- `ardents.ps1` is the single operator entry point for lifecycle, data,
  rollout, tests, and packaging; deployment and release implementation lives
  under `scripts/deploy/` and `scripts/release/`.
- A clean Docker quick start provisions an isolated local realm, private node
  capabilities, three Waku nodes, persistent data, and a real Docker workload
  executor without mounting the host Docker socket.
- Canonical test selection, JSON/JUnit reports, deliberate-failure evidence,
  coverage, catalog validation, security scanning, deployment, multi-host, and
  release packaging are wired into `.github/workflows/ci.yml`.
- Release candidates are built only from committed source and include binaries,
  an OCI-compatible image archive, checksums, SBOM, and provenance.

## Candidate Identity

- Base stabilization commit: `14345ec17481c5325969945160b0cbd3ef852324`.
- Transient replica-repair fix: `0deed84`.
- Testnet workload-engine fix and accepted candidate: `1d04576`.
- Candidate version: `v0.1.0-rc.1`.
- License: MIT.

## Docker/Linux Evidence

| Check | Result |
| --- | --- |
| Canonical fast suite with coverage | passed in Docker/Linux |
| Full integration baseline | 132/132 passed, 0 failed |
| Current repair integration `DAI-004` | passed; scenario 16.55 s, wall 71.1 s |
| Current full E2E | 17/17 passed, 0 failed; wall 184.2 s |
| Deliberate failure contract | passed; JSON, JUnit, and raw evidence retained |
| Test catalog | 150 tests, 48 scenarios, 150 formal bindings, 0 issues |
| Security gate | passed with exact accepted-finding reconciliation |
| Clean source deployment | three product-ready joined Waku nodes |
| Segmented multi-host `NFM-001` | 13/13 steps passed; wall 173.9 s |
| Two release builds | 24/24 files SHA-256 identical |
| Clean start from release bundle | passed in 39.4 s; three product-ready nodes |
| Cleanup | test containers, networks, and volumes removed |

Primary reports:

- `tests/.artifacts/reports/stb606-integration/summary.json`
- `tests/.artifacts/reports/stb606-dai004-final/summary.json`
- `tests/.artifacts/reports/stb606-e2e-final/summary.json`
- `tests/.artifacts/reports/stb606-multihost-final/summary.json`
- `tests/.artifacts/security/`
- `tests/.artifacts/releases/stb606-final-a/`
- `tests/.artifacts/releases/stb606-final-b/`

## Failures Found And Closed

The first full E2E run exposed a real replica-repair race after peer loss. The
placement path queried already committed peers and treated one transient Waku
capacity/control exchange loss as terminal. Existing replicas are now excluded
before network I/O, capacity observation receives one bounded retry, and a
persisted repair attempt receives one additional placement cycle only when all
non-existing denials are transient. Quota, policy, capability, trust,
integrity, lease, and unsupported denials remain terminal for that attempt.

The first multi-host gate exposed deployment drift: service-node profiles had
adopted the real Docker executor, while the adversarial testnet Compose file did
not provide a workload engine. The topology now shares a test-only,
volume-isolated Docker-in-Docker Unix socket. It does not expose the host Docker
socket. The runner also creates its report directory before startup so early
failures retain evidence.

Resource snapshots ruled out CPU, RAM, and disk exhaustion. Long wall times
were dominated by cold Go dependency/build caches, Docker image construction,
and intentional network recovery deadlines. Commands used explicit hard
timeouts and the accepted reruns used warm caches where applicable.

## Phase 6 Transition Decision

`passed` on 2026-07-20. Operator configuration, least privilege,
observability, self-forming deployment, backup/restore, upgrade/rollback,
packaging, canonical CI gates, multi-node formation, and reproducible release
artifacts have accepted evidence. Remote GitHub-hosted execution is an external
publication concern; the workflow and every command it invokes were validated
locally on the supported Docker/Linux boundary.

