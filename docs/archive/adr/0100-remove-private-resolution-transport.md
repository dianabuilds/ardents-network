---
status: accepted
date: 2026-09-26
---

# ADR-0100 — Remove the uncomposed private-resolution transport package

## Context

ADR-0090 retired the `ardents name resolve` and `ardents name control`
operator commands but retained `internal/naming/resolution` — fifteen
production files implementing the fixed-size OHTTP Client/Relay/Gateway
exchange — because the retirement behavior test used the live transport
as the zero-effect oracle: the fixture first proved the former
resolve/control shapes actually worked against an authenticated State
root, a committed Namespace root, and live Relay/Gateway handlers, and
then proved the retired commands refused with zero transport attempts
and byte-for-byte unchanged durable roots.

The reconstruction findings (F-51) made this package the first removal
candidate: it has no production importer, owns no filesystem root or
durable evidence, and its sole external consumer is the retirement
fixture. Keeping an unused network implementation indefinitely as the
refusal oracle inverts the cite-or-die rule (ADR-0091): the oracle's
evidence value is the unchanged durable roots and the zero-attempt
counter, not the working transport. F-51 directed preserving the exact
effect-free command refusal and its evidence with a bounded fixture,
then removing the uncomposed transport package, tests, and package-map
row together. This decision supersedes ADR-0090's retention rationale
for this package only; ADR-0090's command refusal, refusal text, and
Namespace/custody retention are unchanged.

## Decision

1. `internal/naming/resolution` is deleted whole: fifteen production
   files and ten test files. No production symbol outside the package
   imported it.
2. `cmd/ardents/name_retirement_fixture_test.go` becomes the bounded
   fixture: it still materializes one authenticated State root
   (`prepareRetiredNameState`), one committed Namespace root
   (`epoch.Open` + `record.SignRecord` + threshold-attested
   `CommitLegacy`), one admission gate with issued resolve/renewal
   challenges, and plan/operation JSON files shaped like the former
   adapter inputs. It composes no Gateway, Relay, or Resolver, starts
   no server, and drops the `GatewayProfile` plan field whose type came
   from the removed package. `TestNameNetworkCommandsRetireBeforeEffects`
   — the zero-effect oracle asserting the exact refusal text, zero
   output, zero blocked-transport attempts, and byte-identical State and
   Namespace trees — is unchanged. The now-unused `gateway` and `view`
   fields leave the state fixture struct; its candidate verification of
   the accepted State root stays.
3. The Namespace-owned Gateway/verifier views
   (`OpenResolutionGateway`, `OpenResolutionVerifier`) remain pending
   the separate Namespace disposition (F-51). Four retained symbols that
   the removed package's tests used to exercise — `ResolutionGateway.Network`,
   `ResolutionGateway.AdmitResolution`, `epoch.Store.Network`, and
   `authority.Submission.Digest` — gain direct coverage in the focused
   Namespace/authority behavior suites instead
   (`resolution_view_test.go` now proves Node-binding, operation-binding,
   one-use consumption, and replay refusal of an admitted resolution
   proof).
4. The guard `internal/architecture/naming_resolution_retirement_test.go`
   pins the directory's absence, the deterministic-profile and allowlist
   cleanup, the exact command refusal, the fixture's transport-free
   durable-root evidence, the unchanged zero-effect oracle, and the
   retained Namespace views.
5. Deadcode allowlist: the "unwired private-resolution tracer" group
   (27 symbols) is dissolved and the 33 `internal/naming/resolution.*`
   symbols leave the "retained uncomposed private Resolution closure"
   group, whose 22 Namespace-side symbols remain with an updated
   rationale pointing at the Namespace disposition. Common symbols
   352 -> 292, windows-amd64 556 -> 496, linux-amd64 353 -> 293.

## Consequences

- The refusal oracle no longer depends on maintaining a working copy of
  the retired transport; its evidence is the durable roots and the
  zero-attempt counter, which the bounded fixture still materializes.
- The removed package was the repository's last OHTTP importer.
  `go mod tidy` drops `openpcc/ohttp`, `openpcc/twoway`, `openpcc/bhttp`,
  `cespare/xxhash/v2`, `go.opentelemetry.io/otel`,
  `go.opentelemetry.io/otel/trace`, and `golang.org/x/text` from
  `go.mod`/`go.sum`; CIRCL remains solely for the closed-token Route
  credential's Blind RSA use. ADR-0014's reviewed OHTTP set becomes
  historical evidence: a future confidential Name exchange requires a
  separately selected authority/wire and a fresh dependency review, not
  revival of this closure.
- The Namespace cluster (54 files, `epoch.Open` filesystem roots,
  signed records, the 22 remaining closure symbols) is untouched; its
  disposition — authenticated read-only export, bounded migration, or
  typed incompatibility — remains the separate PO-level decision F-51
  requires before its Store or old Record reader can move.
- `docs/technical/naming.md`, the package map, dependency register,
  transition map, behavior map, C0 reconstruction, and the file/package
  CSV registers drop the removed package; research records and ADR-0090
  stay as historical evidence.
