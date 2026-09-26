---
status: accepted
date: 2026-09-26
---

# ADR-0098 — Remove the unwired `internal/naming` Service-Link formatter and parser

## Context

The deadcode allowlist group classified `internal/naming.FormatServiceLink`
and `internal/naming.ParseServiceLink` as an "unwired closed-alpha tracer":
exercised by deterministic behavior tests, but no executable C0 path starts
from them. The retirement condition was to keep only the selected
command-facing boundary or remove the tracer with its rejected candidate.

The audit confirmed:

- Neither function has any production caller. `cmd/ardents name encode`
  consumes only `naming.Parse`/encoding; the recognized `name resolve` and
  `name control` verbs return their retirement refusal before any effect.
- The only exercise was one in-package test
  (`TestParseAndFormatServiceLink`).
- The retained alpha-only Service Link grammar
  (`internal/naming/alpha.ParseServiceLink`, ADR-0088 retained-evidence
  compatibility) is a separate package surface with its own finite signed
  corpus and is untouched by this decision.
- The package-map row for `internal/naming` already claims only canonical
  Service Name parsing, normalization, and deterministic encoding checks —
  the `ardents://` link surface was never a selected command boundary.

## Decision

Remove the tracer:

1. `FormatServiceLink`, `ParseServiceLink`, and the `serviceLinkScheme`
   constant are deleted from `internal/naming/name.go`.
2. `parseName` loses its `allowServiceLink` parameter; the URL-scheme
   rejection becomes the single unconditional rule, so `Parse` and
   `IsDescendant` behavior is unchanged for every canonical input.
3. `TestParseAndFormatServiceLink` is deleted with the surface it exercised.
4. The guard
   `internal/architecture/naming_service_link_retirement_test.go` pins the
   absence of the link surface, the retained canonical parsing
   declarations, and the untouched alpha-only grammar.
5. The allowlist group is dissolved: common symbols 357 → 355,
   windows-amd64 561 → 559, linux-amd64 358 → 356.

## Consequences

- `internal/naming` exposes exactly one textual entry shape: the canonical
  schemeless Service Name. Link presentation, if an accepted command ever
  needs it, arrives with its own contract and ADR.
- The alpha-only Service Link grammar remains solely under its ADR-0088
  retained-evidence disposition.
