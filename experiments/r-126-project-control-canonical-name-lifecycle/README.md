# R-126 canonical Name lifecycle

## Question and hypothesis

Can a project-controlled canonical Name move through publication, update,
Grace, Released, and next-generation reclaim only through threshold-current
state? The hypothesis is that stale, forked, conflicting, and old-generation
inputs fail closed without a corpus fallback.

## Run and captured evidence

Run from a clean checkout:

```powershell
$revision = git rev-parse HEAD
go run ./cmd/ardents-control simulate-namespace-lifecycle --source-revision $revision
```

Retain the JSON receipt outside Git. It must report
`ardents-h4-4b-lifecycle-simulation-v1`, `simulation_result: "passed"`, six
`case/outcome` lifecycle results, four named rejected cases, the exact source
revision, `simulation: true`, and `qualified: false`. Any other result,
fallback, alpha corpus input, or accepted stale/fork/old-generation input
falsifies the experiment. The temporary Store and keys are local-only and are
removed after the run; it proves neither a public Namespace nor Public Beta.

## Result and disposition

A passing receipt is evidence only for the Product Owner-and-Codex simulation.
Retain this runbook and the maintained simulator; retain the JSON receipt
outside Git. Any falsifier leaves H4-4B open and requires an addressed rerun.
