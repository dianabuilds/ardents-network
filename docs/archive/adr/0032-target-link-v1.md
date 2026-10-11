---
status: accepted
date: 2026-08-24
supersedes: none
---

# ADR-0032 — Use the canonical Target Link v1 grammar

## Context

H4-3 needs a shareable direct Target destination before private naming. The
product contract requires it to be tagged, versioned, network-bound, and
distinct from a Service Link, without turning an origin or mutable reachability
record into the Service identity.

## Decision

Adopt exactly `ardents-target:v1:<payload>`, where payload is unpadded RFC 4648
base64url of exactly 65 bytes:

```text
target-algorithm-id[1] | network-id[32] | target[32]
```

`target-algorithm-id` is exactly `0x01` in v1. Both 32-byte fields must be
non-zero. A decoder requires the exact lowercase prefix, rejects padding and
all malformed/unknown forms, and verifies canonicality by re-encoding. An
Endpoint additionally requires the embedded network id to equal its configured
network before it may perform resolution or connection work.

## Consequences

- H4-3 can copy/share one direct Target without DNS, a public origin, a
  Service Name, query/path semantics, reachability, a Local Grant, or browser
  authority;
- any additional target algorithm is a new explicit protocol decision rather
  than an implicit parser compatibility path; and
- parsing or network binding does not authenticate a remote Instance, resolve
  reachability, establish a Route, or create a browser presentation origin.

## Compliance

R-103 supplies the decision record and deterministic failure vectors. This
decision creates no cryptographic primitive, no compatibility reader, and no
network or privacy claim.
