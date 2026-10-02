# Successor implementation zone

This temporary source zone separates the new implementation from existing
product packages. It is a grouping directory, not a Go package, domain term,
wire identity or persisted format identifier. The Product Owner selected this
organization on 2026-10-02. Final responsibility-based paths may be restored
after replacement and removal of the corresponding old implementation.

No component packages or executable are created until a real bounded behavior,
its contract, tests and non-test consumer are implemented together.

## Isolation

All Go files here, including tests and platform-specific files, follow the exact
package imports in [the package map](../../docs/development/package-map.md).
Directory nesting grants no implicit dependency. There is no shared legacy
product allowlist. The command's exact OTel imports and test-only OTLP decoding
imports are enumerated in the isolation test and dependency register.
`admission/issuance` alone may consume reviewed CIRCL blindrsa. New dependencies
require explicit review and the existing dependency acceptance process.

[Admission](admission/README.md) owns its public `issuerprofile` contract, quota
ledger, `issuance` storage/signing owner and `issuer` operation composition.
`issuerprofile` and Hosting use only the standard library. Node Identity depends
only on `issuerprofile` to accept a purpose-bound signing request. Admission's
ledger imports that public grammar; it cannot import identity, issuance or its
operation coordinator.

The reserved command path is cmd/ardents-next. It may compose successor packages,
the standard library and its exact registered OTel imports, but no existing product packages. Existing product
packages cannot import successor packages. The architecture test package may
inspect source files without importing successor code.

The finite executable's grammar and lifecycle belong to
docs/technical/successor-permission-inspection.md and
docs/technical/successor-hosting-budget.md. The latter also defines command
growth and extraction rules.
Offline issuance-right accounting belongs to
docs/technical/successor-admission-ledger.md; its debit never grants network
permission or consumes/refunds Hosting capacity.
Offline immutable issuer material and unsigned public inventory belong to
docs/technical/successor-issuer-key-material.md. No private key leaves that API.
No automatic reads, conversion or reuse of old state are authorized.
The confirmed offline issuance cycle and its result journal are owned by
docs/technical/successor-token-issuance.md. No arbitrary signing API is exposed.

internal/architecture/successor_isolation_test.go enforces import isolation
across build profiles. It does not establish runtime confinement, correctness,
secret separation or qualification. Dynamic execution and state access require
their own contract checks.
## Operation composition

`admission/issuer` owns the ordered offline Admission/key/result lifecycle. It
also composes pinned Nodeidentity/key/profile provisioning; its exact domain
imports are Admission, its Issuance child and Node Identity. Command
adapters own configuration, export and telemetry.

Signed profile and existing Node key import contracts belong to
docs/technical/successor-issuer-profile.md.
