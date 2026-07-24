# Wave 0 and R0 baseline

## Baseline

- Frozen source baseline:
  `main@75471a6c08bf0c8a130db65d64c7f37dc33f03b5`
- Date: 2026-07-25
- Host: Windows orchestration environment
- Go cache: external task cache under the system temporary directory
- Worktree during R0 snapshot: clean before and after applicable gates
- Durable evidence:
  `evidence/stabilization-baseline-75471a6.md`

## Passed local checks

| Check | Result |
|---|---|
| `scripts/generate-api.ps1 -Check` | passed |
| `go vet ./...` | passed |
| tooling tests: testcatalog, audittrace, archaccept, doccontract | passed |
| fresh Windows formatting checkout | passed with `core.autocrlf=true`; zero CRLF Go files |
| tagged testcatalog invocation | passed: 142 integration/E2E entries |
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

These clean-baseline results support `locally_verified` status in
`current-remediation-ledger.md`. They do not promote findings to `qualified`
because the canonical environment is Linux and the complete release matrix has
not run against that exact commit.

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

Wave 0 added:

```gitattributes
*.go text eol=lf
```

R0-002 validated this policy from a disposable checkout of the frozen baseline
with `core.autocrlf=true`. The formatting gate passed and the checkout remained
clean. Parent commit `7c0965c` without the policy reproduced two CRLF Go files
and failed the gate, providing the negative control. The original shared
worktree was not mass-normalized.

## Checks not executed for the R0 snapshot

| Check | Reason |
|---|---|
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

1. Begin the prepared Application Discovery implementation slices AD-01–AD-04.
2. Complete the remaining R1 installation-journey, Operator-smoke, and
   capability-catalogue investigations.
3. Run Docker fast, integration, and E2E suites without retry as R3 work.
4. Run deployment, native-install, and segmented multi-node gates as R3 work.
5. Reconcile security evidence and execute independent release builds.
6. Promote ledger rows only from retained commit-bound evidence.

New feature work may proceed in parallel only when it does not change the
qualification baseline. The first bounded research candidate remains
Application Discovery.
