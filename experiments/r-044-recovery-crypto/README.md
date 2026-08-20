# R-044 recovery-cryptography experiment

This disposable experiment evaluates the individually authenticated `t`-of-`n`
Ed25519 candidate in
[R-044](../../docs/research/records/r-044-cryptographic-suite.md). It does not
implement new cryptographic primitives.

## Question and hypotheses

Can `2 <= t <= n <= 8` individually verified standard-library Ed25519
signatures implement the accepted distinct Recovery Authority trust model with
bounded proof size and latency? H1 is R-044 O2. H0 is that duplicate, rogue,
mixed-policy, replay, custody, or resource behavior violates the contract.

## Confirmed public seam

The experiment will exercise one behavior seam:

`Authorize(recovery proof) -> authorized | denied`

The Product Owner confirmed this experiment seam on 2026-08-20. The disposable
implementation does not authorize a maintained recovery interface.

## Frozen scenarios and falsifiers

The corpus covers setup, `t-1`, `t`, and `n` participants; duplicate/unknown
keys; duplicate signatures; wrong Name/network/generation/policy/operation;
successor mutation; delayed completion; threshold cancellation; lost
participants; malformed framing; restart; and independent verification. It
fails if the current Name Authority alone succeeds, fewer than `t` distinct
participants succeed, or any malformed proof changes Recovery Pending state.

## Run

```powershell
go test ./experiments/r-044-recovery-crypto/recovery.go `
  ./experiments/r-044-recovery-crypto/recovery_test.go -count=10
go run ./experiments/r-044-recovery-crypto/recovery.go `
  ./experiments/r-044-recovery-crypto/measure.go
```

Cross-compile the implementation and measurement files and run the binary in
the pinned Ubuntu image with `--network none`, `--cpus=1`, `--memory=512m`,
`--pids-limit=64`, `--read-only`, and `--cap-drop=ALL` for the weaker profile.
Generated keys, binaries, and raw outputs remain outside Git.

## Evidence, result, and disposition

The worst `5-of-8` proof passed with `1,248` logical bytes, `1,720` heap bytes
per verification, and `0.404001 ms` Linux p95 over `10,000` iterations. The
complete hostile matrix passed ten repetitions with zero false authorization.
Exact measurements and limitations are recorded in R-044.

Retain the small simulator and tests as decision evidence. It does not prove
real participant independence, custody, operational availability, or external
security review.
