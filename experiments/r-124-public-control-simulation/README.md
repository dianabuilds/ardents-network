# R-124 project-control simulation

## Question

Can the Product Owner and Codex reproduce the selected H4-6C shared-control
mechanics and reject every specified fault without relying on other people?

## Hypothesis

The bounded simulator accepts its six mechanics cells and rejects its sixteen
fault cells while issuing no persistent authority or Public Beta claim.

## Environment and inputs

Run from a clean checkout with the repository's selected Go toolchain. The only
input is the exact lowercase 40-character Git source revision being declared
for evaluation. The runbook refuses a dirty checkout before that revision is
supplied. The simulator creates fresh in-memory keys; it reads no VPS, Docker,
CI, network, credential, or participant input.

## Run and captured evidence

In PowerShell, choose an external evidence location and run:

```powershell
$dirty = git status --porcelain
if ($dirty) { throw 'run only from a clean checkout' }
$sourceRevision = git rev-parse HEAD
$receiptPath = Join-Path $env:TEMP 'r-124-public-control-receipt.json'
go run ./cmd/ardents-control simulate-public-control --source-revision $sourceRevision | Set-Content -NoNewline $receiptPath
Get-Content -Raw $receiptPath
```

The receipt must contain the supplied `declared_source_revision`,
`contract: h4-6c-project-control-simulation-v1`,
`simulation_result: passed`, `simulation: true`, `qualified: false`, one
`receipt_digest`, six `passed` cells, and sixteen `rejected` cells. The digest
summarizes receipt fields, not the unexported ephemeral keys or signatures. It
is an external generated artifact and must not be committed to the repository.

## Falsification

The hypothesis fails if the command rejects a valid revision, reports any other
result, omits/changes a required cell, accepts a fault cell, writes authority or
keys, or changes a public claim. Confirm the maintained behavior with:

```powershell
go test ./internal/publiccontrolsimulation ./cmd/ardents-control -count=1
```

## Result and disposition

The selected result is a project-controlled simulation only. ADR-0055 and
R-124 accept a retained receipt as H4-6C evidence; no Public Beta or independent
operation claim follows.
