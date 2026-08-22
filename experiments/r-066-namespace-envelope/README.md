# R-066 Namespace tracer envelope

This disposable experiment measures the retained 127-record Namespace tracer
corpus described by `docs/research/records/r-066-namespace-tracer-envelope.md`.

Run from the repository root:

```text
go run experiments/r-066-namespace-envelope/main.go
```

It uses a new operating-system temporary directory and removes it before exit.
It deterministically creates a 127-record parent hierarchy, commits one signed
materialization, measures local lookup and reopen-plus-lookup samples, checks
eight simultaneous lookups, and prints one JSON result. It is not a benchmark
of a distributed Namespace or a product capacity claim.
