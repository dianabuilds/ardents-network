# R-125 controlled project-control transitions

## Question and hypothesis

R-125 asks whether a project-controlled transition accepts only continuous
overlap and otherwise stops or becomes unavailable. The hypothesis is that one
bounded local evaluator emits every required outcome without an alternate
source, an older generation, retained authority, or Endpoint action.

## Environment and inputs

Run from a clean repository checkout with the root Go toolchain. The only input
is the exact checked-out source revision. The command is local-only; it uses no
network, VPS, secret, persistent key, or participant data.

## Procedure

In PowerShell:

```powershell
$dirty = git status --porcelain
if ($dirty) { throw 'run from a clean checkout' }
$sourceRevision = git rev-parse HEAD
$receiptPath = Join-Path $env:TEMP "r-125-controlled-transitions-receipt-$sourceRevision.json"
go run ./cmd/ardents-control simulate-public-control-transitions --source-revision $sourceRevision |
  Set-Content -NoNewline $receiptPath
Get-Content -Raw $receiptPath
```

Retain the receipt outside the repository. It must report schema
`ardents-h4-6d-transition-simulation-v1`, contract
`h4-6d-project-control-transitions-v1`, `simulation_result: "passed"`, the
declared revision, a digest, `simulation: true`, and `qualified: false`.

## Captured evidence and result

The passing array has seven `{case,outcome}` entries: `overlap-accepted` /
`overlap-accepted`, `expiry-stops` / `stop-expired`, `revocation-stops` /
`stop-revoked`, `incompatible-generation-stops` /
`stop-incompatible-generation`, `rollback-stops` / `stop-rollback`,
`distribution-outage-stops` / `unavailable-distribution`, and
`emergency-disablement-stops` / `stop-emergency-disabled`. The rejected array
has `overlap-without-continuity`, `emergency-escalation`, and
`emergency-expired`.

Any missing/different label, non-passing result, digest/revision mismatch, a
qualified result, or successful fallback falsifies the experiment. A local pass
does not establish public operation, independent control, availability, or
Public Beta.

## Disposition

Retain this runbook and the maintained implementation in
`internal/publiccontrolsimulation`; retain only the JSON receipt outside Git.
