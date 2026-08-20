# R-042 claim-ordering experiment

This disposable experiment evaluates the decision-ready epoch commit/reveal
candidate in [R-042](../../docs/research/records/r-042-claim-ordering.md). It is
not a maintained Namespace implementation or a selected public wire protocol.

## Question and hypotheses

Can one bounded threshold-authenticated input log plus commit/reveal produce the
same claim outcome for every verifier without local arrival time, digest
priority, or a hidden registrar? H1 is the R-042 O1 profile. H0 is that copying,
withholding, partition, rollback, equivocation, or rule fork can create two
accepted controllers or a false deterministic loser.

## Confirmed public seam

The experiment will exercise one behavior seam:

`Verify(claim-set proof) -> accepted | ordered-collision | conflict | fork | unavailable`

The Product Owner confirmed this experiment seam and later accepted O1b and
ADR-0017 on 2026-08-20. The disposable implementation remains research evidence;
maintained behavior belongs to the S6.5 Module.

## Frozen scenarios and falsifiers

The corpus covers happy path, copied commitment, copied reveal, independent
pre-reveal collision, reveal withholding, input withholding, flood at the cap,
partitioned incomplete evidence, two authenticated roots, prior-epoch rollback,
and incompatible rule version. It fails if two controllers are accepted, a
loser is named without complete authenticated evidence, or any rejected path
mutates a Lease.

## Run

```powershell
go test ./experiments/r-042-claim-ordering/ordering.go `
  ./experiments/r-042-claim-ordering/ordering_test.go -count=10
go run ./experiments/r-042-claim-ordering/ordering.go `
  ./experiments/r-042-claim-ordering/measure.go -claims 32 -iterations 1000
```

For the weaker Linux profile, cross-compile the two implementation files and
run the resulting binary in the pinned Ubuntu image with `--network none`,
`--cpus=1`, `--memory=512m`, `--pids-limit=64`, `--read-only`, and
`--cap-drop=ALL`. Generated binaries and raw outputs stay outside Git.

## Evidence, result, and disposition

O1 with `64` conflicting claims failed its frozen Linux p95 gate. O1b with a
`32` per-Name conflict cap passed: `5,932` logical bytes and `1.640826 ms` p95
over `1,000` weaker-profile verifications. The complete hostile matrix passed
ten repetitions. Exact measurements and limitations are recorded in R-042.

Retain the small simulator and deterministic tests as decision evidence. It is
not production code, distributed-log availability evidence, or a Qualification
result.
