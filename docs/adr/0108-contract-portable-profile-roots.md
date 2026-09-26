---
status: accepted
date: 2026-09-27
---

# ADR-0108 — The Portable Endpoint profile contracts to its consumed roots; the grants, Vault, diagnostics, and cache scaffold is no longer created

(F-26. This completes the reduction that ADR-0099 began when it retired the
test-only `Run` facade; the findings' remaining candidate — stop creating
unconsumed directories — is executed here.)

## Context

Every `portable.Open` created and owner-validated nine directories, four of
which no non-test Go consumer ever used:

- `ConfigHome/grants` — no code reads or writes grants under the Portable
  config root; the selected C0 design has no Portable-side grant store.
- `StateHome/vault` — the Authority Vault lives in `internal/custody` with
  its own root; this directory was scaffold only.
- `StateHome/diagnostics` — diagnostics ship through the read-only timeline
  command on standard input; the Contributor owns its own diagnostics root.
- `CacheHome` — nothing in the repository reads or writes a Portable cache.

Creating them gave the generic profile apparent ownership of grant, Vault,
diagnostic, and cache locations whose real owners are separate Modules —
misleading surface area around the live enrollment/replacement path. The
roots that do have consumers are `StateHome` (owner lock under `live`,
Release-floor parent `floors`, and the `replacement` ledger parent) and
`RuntimeHome` (the `endpoint.sock` attachment); `release.Open` and the
replacement module each create and validate their own roots underneath.

## Decision

1. `Config` carries exactly two roots: `StateHome` and `RuntimeHome`. The
   configuration and cache fields and their `XDG_CONFIG_HOME`/`XDG_CACHE_HOME`
   derivations are deleted from both platform resolvers.
2. `prepareRoots` creates and owner-validates only the state base, its
   `floors` and `live` parents, and the runtime base. The grants, vault,
   diagnostics, and cache directories are no longer created.
3. Existing on-disk bytes are preserved: nothing is deleted or migrated. A
   former profile's scaffold directories simply remain unread, matching the
   operator procedure, which now cites the retained floors/replacement/live
   roots instead of the scaffold list.
4. The direct Portable test asserts the contracted invariants — the retained
   roots exist owner-only, the profile base holds exactly the state and
   runtime roots, and the retired vault/diagnostics scaffold never appears.
   The Ubuntu qualification oracles and the installed-package persistence
   asserts now prove retention through the `live` root and the Release floor
   instead of the retired Vault scaffold; the inert XDG config/cache
   environment entries leave the Unix process tests.
5. A new architecture guard (`portable_profile_contraction_test.go`) forbids
   the scaffold identifiers across the package and pins the retained
   consumer-root preparation in `roots.go`.

## Consequences

- The Portable profile now names exactly what it owns: one owner-leased state
  root with its enrollment-floor and lock parents, and one runtime attachment
  root. No generic claim over grant, Vault, diagnostic, or cache locations
  remains.
- Restart and replacement behavior is unchanged: the retained roots keep
  their owner-only validation, the lease, stale-socket recovery, and the
  probe/ready attachment are untouched, and the Release/replacement modules
  keep creating their own roots as before.
- Old installs keep their former scaffold directories as inert bytes; under
  the no-support direction there is no cleaner, and none is needed because
  no code path reads them.

## Verification

- `internal/endpoint/portable/runtime_test.go` proves the contracted creation
  set and the four retained lifecycle behaviors (concurrent-owner refusal,
  stale-socket recovery, unexpected-entry preservation, path-budget refusal).
- `internal/architecture/portable_profile_contraction_test.go` forbids the
  scaffold identifiers and pins the retained preparation.
- The Ubuntu qualification suites (`ubuntu_portable_user_unit`,
  `ubuntu_portable_replacement`, `installed_package_process`) assert that
  lifecycle operations preserve the `live` root and the Release floor; their
  runtime verification stays CI/Ubuntu-only.
- Gates: Windows and `GOOS=linux` build/vet, the full Windows suite (45 ok),
  `go test -c` for all five commands plus the portable and e2e endpoint
  packages, staticcheck, exact deadcode allowlist match with no allowlist
  change, and `make quick-check` all pass on this commit.
