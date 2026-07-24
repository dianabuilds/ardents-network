# Wave 0 baseline

## Baseline

- Research source baseline: `main@7c0965c`
- Date: 2026-07-24
- Host: Windows orchestration environment
- Go cache: external task cache under the system temporary directory
- Worktree during verification: intentionally dirty with Wave 0 documentation
  and `.gitattributes`; this is not release evidence

## Passed local checks

| Check | Result |
|---|---|
| `scripts/generate-api.ps1 -Check` | passed |
| `go vet ./...` | passed |
| tooling tests: testcatalog, audittrace, archaccept, doccontract | passed |
| executable testcatalog invocation | exposed an empty-catalog false pass; remediated in Wave 1 |
| audit traceability | passed: 21 findings, 5 gates |
| entrypoint contract negative matrix | passed |
| release source identity negative matrix | passed |
| release materials policy gate | passed |
| `go test ./... -count=1` | passed |
| critical lifecycle packages with `go test -race ... -count=1` | passed |

The race command covered:

- ingress proxy;
- Waku adapter;
- private messaging;
- content;
- identity access;
- daemon lifecycle;
- workload runtime and Docker adapter;
- transfer;
- configuration.

These results support `locally_verified` status in
`current-remediation-ledger.md`. They do not promote findings to `qualified`
because the canonical environment is Linux and the complete release matrix has
not run against a clean exact commit.

The initial static catalog command returned `[]` because it omitted tagged
suite build constraints. Wave 1 corrected the workflow to validate both
`integration` and `e2e` metadata and to reject an empty result. The corrected
contract is verified separately; the earlier empty result is not counted as
evidence.

## Formatting checkout finding

The current checkout has `core.autocrlf=true` and historically had no
`.gitattributes`. Git considers CRLF Go files clean, while `gofmt -l` considers
them unformatted. Running `gofmt -w` over the current tree would be a large
mechanical rewrite unrelated to product behavior.

Wave 0 adds:

```gitattributes
*.go text eol=lf
```

This must be validated in a fresh Windows checkout after the Wave 0 changes are
committed. The existing worktree is deliberately not mass-normalized.

## Checks not executed on this worktree

| Check | Reason |
|---|---|
| canonical formatting gate | requires fresh checkout to validate new LF contract |
| fast Docker/Linux suite | Docker daemon unavailable |
| integration Docker/Linux suite | Docker daemon unavailable |
| E2E Docker/Linux suite | Docker daemon unavailable |
| deployment gate | Docker daemon unavailable |
| native systemd gate | requires canonical Linux/container runner |
| segmented multi-node topology | Docker daemon unavailable |
| independent release builds | requires clean commit and hosted/canonical runners |
| vulnerability evidence gate | requires its canonical network/tooling environment |

Existing reports from 2026-07-23 are historical evidence for earlier commits.
They are not substituted for a current-head Wave 0 run.

## Current promotion counts

| Status | Count |
|---|---:|
| locally_verified | 19 |
| remediated_candidate | 5 |
| qualified | 0 |
| reopened | 0 |

The five remaining `remediated_candidate` findings are:

- CI-002 — native install evidence;
- OPS-001 — verified backup before upgrade;
- OPS-002 — transactional rollout compensation;
- OPS-003 — full composite deployment readiness;
- OPS-004 — configured native readiness target.

OPS-003 has passing local unit/race evidence but remains a candidate until its
deployment matrix runs.

## Next ready backlog

1. Commit the Wave 0 documents and LF checkout contract intentionally.
2. Validate `tests/check-format.ps1` in a fresh Windows checkout.
3. Run the canonical static job from the clean commit.
4. Run Docker fast, integration and E2E suites without retry.
5. Run deployment, native-install and segmented multi-node gates.
6. Reconcile security evidence and execute independent release builds.
7. Promote ledger rows only from retained commit-bound evidence.

New feature work may proceed in parallel only when it does not change the
qualification baseline. The first bounded research candidate remains
Application Discovery.
