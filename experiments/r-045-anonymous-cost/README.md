# R-045 Anonymous Cost experiment

This disposable experiment calibrates the scoped challenge-work candidate in
[R-045](../../docs/research/records/r-045-anonymous-cost.md). It is not a
personhood, fairness, anti-squatting, or global Sybil-resistance claim.

## Question and hypotheses

Can HMAC-bound SHA-256 work plus finite single-use local admission keep claim,
renewal/update, resolution, policy, and recovery work accessible on the weaker
client while bounding verifier amplification, replay state, and parallelism?
H1 is R-045 O1. H0 is that any predeclared latency, memory, replay, restart, or
amplification criterion fails.

## Confirmed public experimental seam

The experiment will exercise one behavior seam:

`Admission.Verify(proof) -> admitted | admission-denied`

The Product Owner confirmed this seam on 2026-08-20. Challenge issuance and
proof solving are setup around that behavior; they do not expose a maintained
Stage 6 package.

## Frozen scenarios and falsifiers

The exact initial difficulties, caps, latency/resource thresholds, scope
bindings, and failure criteria are frozen in R-045 before the run. The corpus
adds wrong surface/epoch/Node/context, expiry, replay, spent-set saturation,
parallel duplicate submission, restart, malformed framing, and cheap-request
amplification cases.

## Run, evidence, result, and disposition

```powershell
go test ./experiments/r-045-anonymous-cost/admission.go `
  ./experiments/r-045-anonymous-cost/proof.go `
  ./experiments/r-045-anonymous-cost/admission_test.go -count=10
go run ./experiments/r-045-anonymous-cost/admission.go `
  ./experiments/r-045-anonymous-cost/proof.go `
  ./experiments/r-045-anonymous-cost/measure.go -candidate=o1b
```

`Request.Nonce` is deterministic injected entropy for the disposable harness;
the selected maintained issuer must generate its fresh nonce internally with
the operating system CSPRNG.

O1 ran on the current Windows endpoint and in a declared weaker
`1 vCPU/512 MiB` Linux experimental profile. It failed every
solve p95 gate while passing the verification, retained-state, hostile,
restart, and parallel invariants. R-045 retains the exact measurements and a
separately predeclared O1b profile. O1b then passed every unchanged scope,
replay, restart, capacity, verifier, memory, and accessibility gate with the
frozen `16/16/17/18`-bit profile. Result: O1 rejected; O1b decision-ready,
pending Product Owner acceptance.
