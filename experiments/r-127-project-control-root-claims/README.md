# R-127 project-control root claims

## Question and hypothesis

Can a root Name reach threshold-authenticated current Namespace state only
after locally admitted commit/reveal, deterministic selection, and one
authenticated Epoch close? The hypothesis is that withholding, incomplete
evidence, incompatible rule, and conflicting authenticated close have no
current-state fallback.

## Run and captured evidence

Run from a clean checkout:

```powershell
$revision = git rev-parse HEAD
go run ./cmd/ardents-control simulate-root-claims --source-revision $revision
```

Retain the JSON receipt outside Git. It must report
`ardents-h4-4c-root-claim-simulation-v1`, `simulation_result: "passed"`, the
four `case/outcome` results, four named rejected cases, exact source revision,
`simulation: true`, and `qualified: false`. Any other result, a caller-built
corpus, a current state without threshold Epoch attestation, or accepted
withholding/incomplete/fork input falsifies the experiment.

The run commits during synthetic Epoch 8, reveals and closes during Epoch 9,
uses a temporary local Store and deterministic non-secret test keys, then
removes them. It demonstrates neither a public Namespace, authority
legitimacy, Sybil resistance, anti-squatting, governance legitimacy,
independent operation, nor Public Beta.

## Result and disposition

A passing receipt is evidence only for the Product Owner-and-Codex
project-control simulation. A falsifier leaves H4-4C open and requires an
addressed rerun.
