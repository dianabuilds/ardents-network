# Application Discovery implementation evidence: 441545e

## Result

The AD-03 through AD-05 implementation and post-review remediation passed the
applicable local and Docker/Linux feature gates against source commit
`441545ed6e553325b874530cced73d19e205a93f`.

This is retained implementation evidence, not a release qualification
snapshot. The `release-candidate` gate requires a canonical tagged or
workflow-dispatched Linux release run, so `application.discovery` remains
`Q=no`.

## Source identity

| Field | Value |
|---|---|
| Commit | `441545ed6e553325b874530cced73d19e205a93f` |
| Subject | `fix(discovery): close AD qualification blockers` |
| Starting worktree | clean |
| Docker cache mode | disposable anonymous module and build caches |
| Retry policy | the initial sandbox denial was not a test attempt; successful Docker runs were the first executions with Docker access |

## Local and static evidence

- `go test ./... -count=1` passed.
- Scoped `go vet`, including the tagged Application Discovery package, passed.
- `scripts/generate-api.ps1 -Check` passed.
- Architecture, audit-trace, capability-catalogue, and test-catalogue tooling
  passed.
- Focused `govulncheck` reported no reachable vulnerability.
- Independent Standards and Spec reviews reported no findings.

## Docker/Linux Application Discovery lifecycle

Command:

```text
tests/run.ps1 -Suite integration -Scenario APP-DISC-001 -ReportDir tests/.artifacts/reports/application-discovery-ad05-qualified-integration -EphemeralCache
```

All five scenarios passed, including production `Node.ReloadConfig` trust and
route-policy convergence and retained-truth capacity protection.

| Artifact | SHA-256 |
|---|---|
| `summary.json` | `98D41626264AF90C4BB1C72D958F26DF7EC2290E971E2CD819B424B00E780AD3` |
| `junit.xml` | `4E55E63F3C514F45DEB5124E3C3BBC440D2FC6EF2E85EF397969F476662C7817` |

## Docker/Linux protected Application process

Command:

```text
tests/run.ps1 -Suite e2e -Scenario APP-001 -ReportDir tests/.artifacts/reports/application-discovery-ad05-qualified-e2e -EphemeralCache
```

The protected-socket Application process passed enrollment, retry, Discovery,
Content, restart, grant/device revocation, and the adversarial Operator
credential assertion.

| Artifact | SHA-256 |
|---|---|
| `summary.json` | `55F0A7272555F2138770238E343510156925972A28142E2B4C72FC37CE9FE282` |
| `junit.xml` | `94155970CA95959B2664E36126BCB2D8D94CB4EDFD980CE3118E85A7AE6AA69F` |

Both canonical runners removed their temporary tagged test binaries.
