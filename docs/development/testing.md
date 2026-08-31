# Testing

Ardents separates deterministic Module checks, local process behavior,
artifact profiles, and explicitly selected qualifications. A historical result
is evidence about its exact candidate; it is not a test entrypoint for current
code.

## Ordinary checks

- `make unit` runs the positive deterministic package inventory.
- `make e2e` runs the positive local process package inventory.
- `make quick-check` runs formatting, architecture, vet, unit, named command
  builds, module tidiness, and the Browser-v4 artifact lane.
- `make headless-check` builds the exact Network command inventory, checks the
  enrollment-v3 artifact, and runs bounded Endpoint, Source, Node, and Service
  process evidence without Browser artifacts.
- `make browser-check` builds the exact Browser command inventory, checks the
  Browser-v4 artifact, and verifies its enrollment separately from Network-v3.
- `make check` runs unit, process, race, command build, formatting,
  Staticcheck, and vulnerability checks. It is the pre-integration gate.
- `make fuzz` exercises the maintained bounded parser/encoder fuzz surface.

The headless command inventory is
`tests/profiles/headless-commands.txt`; the Browser inventory is
`tests/profiles/browser-commands.txt`. They are positive, disjoint manifests.
The architecture gate checks the actual transitive dependency graphs, not just
the names in those files.

## Current profiles

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Active profiles have a real Make entrypoint. Inactive
profiles have no entrypoint and state the decision needed to activate a new
suite; inactive never means passed or waived.

The maintained local profiles are developer, deterministic, process,
headless-network, browser-adapter, race, and fuzz. The current purpose-named
qualifications that retain entrypoints are:

- H4-1A Ubuntu Portable and H4-1B Ubuntu replacement through `make
  qualification` and their individual targets;
- the bounded native two-host Rendezvous target
  `qualification-h4-2-multihost`;
- signed-XPI input verification and separate Windows/Ubuntu Browser-v4
  enrollment mechanics;
- the two-fresh-Endpoint H4-6A artifact comparison.

These names are historical planning provenance, not runtime identities.
Environment-dependent profiles fail when their declared prerequisite is
missing; they do not skip to green.

Firefox runtime, the old Browser-bound `reference-c2` fixture, H4-2 local C-2,
H4-3B C-2, H4-5, and H4-8 A11 are not current qualification surfaces. Endpoint
and Firefox-only source is retained under
`tests/compatibility/browser-endpoint-v4/` with Go source stored as `.go.txt`;
the larger C-2 source remains in its fixture tree behind the purpose-named
`referencec2` evidence tag. Re-activating any of them requires new accepted
research, a current maintained fixture, and an explicit product decision. The
retained source is not compiled by ordinary or current qualification targets.

The generic live-network profile remains inactive until a native peer-facing
Route and a measured Node operating profile select a bounded live suite. A
development VPS or Docker environment cannot make that missing decision.

## Ownership

Unit and single-Module integration tests live beside their implementation.
They replace real time or external I/O only at the owning seam and do not start
Docker. Cross-process tests live under `tests/e2e/<behavior>/`; they build real
named commands, observe public behavior, create fresh temporary fixtures, and
clean every owned process and file.

Every maintained Go package belongs to the deterministic inventory. Every
Go-bearing `tests/e2e` suite root belongs to exactly one process profile. A new
package or suite cannot enter through a negative filter, a wildcard exception,
or directory naming alone.

`tests/compatibility/` is non-executable provenance. Compatibility evidence
must name its former observer and deletion or reactivation condition. It does
not belong to a maintained package inventory.

## Validity and reruns

An environment-dependent run has four possible outcomes: product assertion
failure, harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved. Missing Docker, external binaries,
privilege, platform support, pinned images, or host inputs is an invalid
environment for a selected profile.

Rerunning is diagnostic evidence and never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
timeout must leave enough observed state to distinguish the product from the
harness and environment.

## Test design and retirement

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps are not readiness
or eventual-consistency oracles. Every goroutine, process, listener, file,
lock, timer, and fixture has one owner, cancellation path, join, residue
assertion, and cleanup-failure path.

Two tests are duplicate-removal candidates only when requirement, owning seam,
oracle, transition or fault, platform or format, and independence role are all
the same. A seam migration moves behavior tests to the new Module Interface,
retains only higher-level tests that prove a distinct fact, and retires obsolete
fixtures and runners in the same change.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs beside each retained untrusted
decoder and canonical encoder. Race coverage is required wherever a Module
owns goroutines, callbacks, locks, mutable admission, cancellation, or durable
state, and must assert terminal join and state invariants in addition to race
detector cleanliness.
