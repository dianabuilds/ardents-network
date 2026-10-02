# Successor implementation zone

This temporary source zone separates the new implementation from existing
product packages. It is a grouping directory, not a Go package, domain term,
wire identity or persisted format identifier. The Product Owner selected this
organization on 2026-10-02. Final responsibility-based paths may be restored
after replacement and removal of the corresponding old implementation.

No component packages or executable are created until a real bounded behavior,
its contract, tests and non-test consumer are implemented together.

## Isolation

All Go files here, including tests and platform-specific files, may import only
the standard library and packages under this zone. There is currently no shared
product dependency allowlist. The command's exact OTel imports and test-only
OTLP decoding imports are enumerated in the isolation test and dependency
register; domain packages receive no third-party allowance. New dependencies require explicit
review and the existing dependency acceptance process.
Admission and Hosting each permit only standard-library imports; the import
gate also rejects dependencies between these independent domain owners.

The reserved command path is cmd/ardents-next. It may compose successor packages,
the standard library and its exact registered OTel imports, but no existing product packages. Existing product
packages cannot import successor packages. The architecture test package may
inspect source files without importing successor code.

The finite executable's grammar and lifecycle belong to
docs/technical/successor-permission-inspection.md and
docs/technical/successor-hosting-budget.md. The latter also defines command
growth and extraction rules.
No automatic reads, conversion or reuse of old state are authorized.

internal/architecture/successor_isolation_test.go enforces import isolation
across build profiles. It does not establish runtime confinement, correctness,
secret separation or qualification. Dynamic execution and state access require
their own contract checks.
