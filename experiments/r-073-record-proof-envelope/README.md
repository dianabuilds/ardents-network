# R-073 Record and current-proof envelope

This disposable experiment measures the maximum canonical V4 Record payload
that fits the retained fixed 4,096-byte Namespace proof under the worst
technical tracer shape: 127 Records, 16 valid threshold signatures, and the
last membership path.

Run from the repository root:

```text
go run experiments/r-073-record-proof-envelope/main.go
```

It creates a new operating-system temporary store for every candidate and
removes it before returning. The variable data is a canonical conflict
identifier solely to grow the signed Record; the measurement is an envelope
test, not an assertion that conflicted Names resolve. It prints JSON containing
the largest fit, first rejection, signed Record bytes, and proof bytes.

The result is not a product capacity, distributed proof, or performance claim.
