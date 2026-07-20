# STB-707 Evidence — Final Acceptance Gate

Date: 2026-07-20

## Capability And Candidate

This gate validates the complete root Ardents `v1` stabilization candidate:
real Waku network/messaging, private discovery and data exchange, durable data,
Docker workload execution, hosted-service publication, operator control,
diagnostics, deployment, and release evidence.

- tested product commit: `a920832e441e77f61be61e8a150f856685d53fea`;
- canonical test image: `ardents/test-runner:dev`;
- image ID: `sha256:cb89e28ece946267d7768edd439052e58d4f7cbcd3cc177ca994f032b03f0e5d`;
- image creation: `2026-07-19T06:13:18.680441727Z`;
- toolchain: `go1.26.5 linux/amd64`;
- license: MIT.

The evidence commit changes process documentation only. Any later product-code
change requires affected acceptance checks again before candidate freeze.

## Final Checks

| Check | Result |
| --- | --- |
| canonical Docker `fast` | passed; wall `50.3s` |
| canonical Docker `all` | `150/150` reported scenarios passed, `0` failed; wall `842.9s` |
| `go vet ./...` in Linux container | passed; wall `13.3s` |
| catalog/requirements validation | `153` formal bindings, `51` scenarios, `0` issues; `55/55` requirements covered, `0` blocked |
| security reconciliation | passed; `GO-2026-4479` remains symbol-level and `GO-2026-5932` remains module-level exactly as registered |
| targeted race detector | `internal/data` passed in `2.428s`; `internal/workload/execution` passed in `1.118s` |
| production Go size | `0` hard breaches; two non-blocking soft findings recorded below |
| cleanup/resources | no test container remained; after `all`, WSL working set was `3.24 GiB`, available host memory `25.8 GiB`, disk free `192.9 GiB` |

Primary current artifacts:

- `tests/.artifacts/reports/stb-707-all-final/summary.json`;
- `tests/.artifacts/reports/stb-707-all-final/junit.xml`;
- `tests/.artifacts/reports/stb-707-all-final/raw/`;
- `tests/.artifacts/security/stb-707-final/reconciliation.json`;
- `tests/.artifacts/security/stb-707-final/govulncheck.jsonl`;
- `tests/.artifacts/security/stb-707-final/govulncheck.verbose.txt`;
- `tests/.artifacts/resources/20260720-151444-36888-before.json` and
  `20260720-151444-36888-after.json`;
- `tests/.artifacts/resources/20260720-151550-53100-before.json` and
  `20260720-151550-53100-after.json`.

The current `all` runner is the tagged superset over `./...` and therefore
replaces redundant same-commit `integration`, `e2e`, and repeated `all` runs.
Separate integration/E2E reports from STB-606 remain accepted evidence for the
runner split itself. Import guard executes inside both current canonical runs.

## Cross-Phase Evidence Retained

- mutation resistance: `stb-702-evidence.md`;
- chaos and multi-host recovery: `stb-703-evidence.md`;
- performance/resource baseline: `stb-704-evidence.md`;
- bounded soak tooling and smoke execution: `stb-705-evidence.md`;
- code, bug, vulnerability, error, and regression reviews: all STB-706 review
  documents;
- deployment, backup/restore, upgrade, reproducible package, SBOM, provenance,
  and clean bundle start: `stb-605-evidence.md` and `stb-606-evidence.md`.

No critical path uses a fake transport, fake workload engine, plaintext
private carrier, metadata-only availability, or readiness-independent service
publication. Failure/degraded coverage is present for persistence, policy,
reachability, startup/shutdown, publication, workload exit, repair, quota,
corruption, revocation, and terminal loss.

## Known Non-Blocking Limitations

- `cmd/ardd.main` is `41` LOC and
  `internal/node/readiness.ApplyTransportHealth` is `44` LOC, both above the
  `40` LOC soft limit and below the `70` LOC hard limit. They enter the
  refactoring backlog; neither changes domain ownership or runtime truth.
- `tcp_quic` remains explicitly unsupported; supported profiles are TCP and
  TCP/WSS over Waku.
- The two security exceptions remain bounded by their documented controls and
  upgrade triggers.
- The uninterrupted 48-hour SOAK-001 qualification is intentionally outside
  this stabilization/refactoring goal and must run only after final refactor
  freeze against the exact candidate commit and image.

## Acceptance Decision

`passed`. STB-707 and the stabilization loop are complete with no unresolved
release blocker. The candidate is ready for the CI/Docker/boundary refactoring
stage and, after that stage freezes the product candidate, the separate 48-hour
pre-release qualification. This is not yet a public-release authorization.
