# Stabilization baseline evidence: 75471a6

## Result

The R0 stabilization program passed every applicable Windows/local/static gate
against source commit
`75471a6c08bf0c8a130db65d64c7f37dc33f03b5`.

This is a reproducible research and implementation baseline. It is not
production or release qualification. Docker/Linux, native systemd, multi-host,
vulnerability, and independent release-build gates did not run in this
environment.

## Source identity

| Field | Value |
|---|---|
| Commit | `75471a6c08bf0c8a130db65d64c7f37dc33f03b5` |
| Commit date | `2026-07-25T00:33:54+03:00` |
| Subject | `chore: establish stabilization research baseline` |
| Host | Windows orchestration environment |
| Starting worktree | clean |
| Final worktree | clean |
| Retry policy | no failing gate was retried as a success |

## R0-001 — Frozen preparation baseline

The commit contains the research plan, capability/evidence register,
Application Discovery packet, current remediation ledger, local Markdown
tracker, Go LF policy, tagged-catalogue correction, and the agreed audit
reduction.

Pre-commit and clean-baseline verification included:

- `git diff --cached --check`;
- `go vet ./...`;
- `go test ./... -count=1`;
- `tests/ci/entrypoint-contract-gate-test.ps1`;
- `go run ./tests/tooling/audittrace`.

Audit traceability reported 21 findings and 5 gates.

## R0-002 — Fresh Windows LF checkout

A disposable clone was created from a temporary Git bundle, configured with
`core.autocrlf=true`, and checked out at the exact baseline commit.

| Observation | Result |
|---|---|
| CRLF Go files at baseline | 0 |
| `tests/check-format.ps1` | passed |
| Worktree after format check | clean |
| Negative-control commit | `7c0965c4b4aeaccd1aefe8c1c0c267159eb01e87` |
| CRLF Go files without policy | 2 |
| Negative-control format gate | failed as expected |

The temporary checkout and bundle were deleted after the results were
captured. Two earlier direct local-clone attempts stopped before checkout
because the sandbox user did not own the source `.git`; they are not counted as
test attempts or evidence.

## R0-003 — Tagged scenario catalogue

The canonical catalogue command used both required tags:

```text
go run ./tests/tooling/testcatalog -tags "integration e2e" ./tests/...
```

| Observation | Result |
|---|---|
| Catalogue entries | 142 |
| Metadata validation | passed |
| Missing-tag rejection | passed through entrypoint negative matrix |
| Empty-result rejection | passed through entrypoint negative matrix |
| Local JSON | `tests/.artifacts/reports/catalog/r0-003-75471a6-validation.json` |
| JSON bytes | 121314 |
| JSON SHA-256 | `754BBF7A24486D6F6FF3DB9FF90B785FC49EF0065F88137571B10686FD6C940A` |

The JSON is retained local evidence under the repository's ignored artifact
root. Canonical CI retains the equivalent catalogue as an uploaded artifact.

## R0-004 — Clean commit-bound snapshot

| Gate | Result |
|---|---|
| API generation check | passed |
| Fresh-checkout formatting | passed through R0-002 |
| `go vet ./...` | passed |
| `go test ./... -count=1` | passed |
| architecture and documentation contracts | passed in tooling tests |
| audit traceability | passed: 21 findings, 5 gates |
| tagged catalogue | passed: 142 entries |
| entrypoint negative matrix | passed |
| release source identity negative matrix | passed |
| release materials policy | passed |
| critical lifecycle packages with `-race` | passed on Windows |

The race slice covered ingress proxy, Waku, private messaging, content,
identity access, daemon lifecycle, workload runtime and Docker adapter,
transfer, and configuration. This Windows result is diagnostic local evidence;
the supported Linux race runner remains required for qualification.

## Explicitly unavailable gates

| Gate | Reason |
|---|---|
| fast Docker/Linux | Docker daemon unavailable |
| integration Docker/Linux | Docker daemon unavailable |
| E2E Docker/Linux | Docker daemon unavailable |
| deployment and segmented multi-node | Docker daemon unavailable |
| native systemd installation | no canonical Linux/native runner |
| vulnerability evidence | canonical network/tooling environment not run |
| independent release builds | requires clean hosted/canonical runners |

No remediation-ledger row was promoted to `qualified`. The existing status
counts remain 19 `locally_verified`, 5 `remediated_candidate`, and 0
`qualified`.

README and Changelog continue to state that the project is a stabilization
candidate with no accepted production release and pending final release gates.
