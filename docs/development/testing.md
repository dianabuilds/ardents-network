# Testing

Ardents separates deterministic Module checks, local process behavior,
artifact profiles, and explicitly selected qualifications. A historical result
is evidence about its exact candidate; it is not an entrypoint for current
code.

## Ordinary checks

- `make unit` runs the positive deterministic package inventory.
- `make e2e` runs the positive local process package inventory.
- `make quick-check` runs formatting, architecture, vet, unit, named command
  builds, module tidiness, and the Browser artifact lane.
- `make headless-check` builds the exact Network command inventory, checks the
  enrollment-v3 artifact, and runs bounded Endpoint, Source, Node, and Service
  process evidence without Browser artifacts.
- `make browser-check` builds the exact Browser command inventory, checks the
  enrollment-v4 artifact, and verifies it separately from Network-v3.
- `make check` runs unit, process, race, command build, formatting,
  Staticcheck, and vulnerability checks. It is the pre-integration gate.
- `make fuzz` exercises the maintained bounded parser/encoder fuzz surface.

The headless and Browser command inventories are positive, disjoint manifests
under `tests/profiles/`. Architecture tests check actual transitive dependency
graphs and verify that every maintained package and suite belongs to its exact
profile.

## Current profiles

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Every active profile has one real Make entrypoint and exact
prerequisites. Missing Docker, binaries, privilege, platform, host input, or
artifact is an invalid environment, never a skip or passing result.

The maintained local profiles are:

- developer, deterministic, process, headless-network, browser-adapter, race,
  and fuzz;
- `qualification`, the aggregate selected Ubuntu Endpoint lifecycle profile;
- `endpoint-portable-ubuntu` and `endpoint-replacement-ubuntu`;
- `native-rendezvous-multihost`;
- `browser-signed-xpi`, `browser-entry-windows`, and
  `browser-entry-ubuntu`; and
- `alpha-control-two-endpoints`.

Profile, target, directory, build-tag, environment-variable, and test names use
domain language. Historical wire, persisted, release, and artifact identities
remain unchanged where their exact bytes are a compatibility obligation.

There is no generic live or soak profile. Selecting VPS, hostile-load, soak,
platform-matrix, release, or Firefox runtime qualification requires a new exact
product claim, environment, fixture, and Product Owner decision.

## Candidate boundaries

The C0 Network candidate is exercised by the deterministic/process/race lanes,
`headless-check`, and the selected Network qualifications. This includes the
Endpoint-owned `internal/application/broker`; its directory does not make it a
Browser-only package. The separate Application/Browser candidate is exercised
by shared Application Interface tests, Browser-owned Module tests,
`browser-check`, and its selected artifact mechanics. Browser commands are
excluded from the Network artifact inventory, and Network implementation
packages are excluded from Browser dependencies.

`tests/compatibility/browser-endpoint-v4` is the sole retained non-executable
source exception. ADR-0061 requires it to remain outside Go package discovery,
ordinary checks, and current qualification until an explicit supersede or
retirement decision. Completed experiments and the former `reference-c2`
fixture are available only from Git history and accepted research records.

## Historical qualification provenance

Only the concise verdicts below remain in the current documentation surface.
Detailed runners, process chronology, and split-candidate ledgers are historical
Git material.

| Exact source | Retained result | Limitation |
|---|---|---|
| [`70bf425eec937edcc22e8f0534db992aa2002a16`](https://github.com/dianabuilds/ardents-network/commit/70bf425eec937edcc22e8f0534db992aa2002a16) | Historical RC1 supplied A1-A10 bounded project-operated evidence. | It has no A11/A12 closure and does not qualify C0. |
| [`2c18bdf92f11f84075915576f595202f48eb05bc`](https://github.com/dianabuilds/ardents-network/commit/2c18bdf92f11f84075915576f595202f48eb05bc) | Historical RC2 supplied a separate two-fresh-Endpoint control result and accepted A11 campaign. | It does not inherit RC1 A1-A10; no candidate has an aggregate A1-A12 result. |
| [`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf) | Last source snapshot containing the retired experiment implementations, `reference-c2`, generic Update tracer, stage runners, and planning chronology. | Provenance only; none of those sources qualifies or belongs to the cleaned C0 candidate. |

## Ownership

Unit and single-Module integration tests live beside their implementation.
They replace real time or external I/O only at the owning seam and do not start
Docker. Cross-process tests live under `tests/e2e/<behavior>/`; they build real
named commands, observe public behavior, create fresh temporary fixtures, and
clean every owned process and file.

Every maintained Go package belongs to the deterministic inventory. Every
Go-bearing `tests/e2e` suite root belongs to exactly one process profile. A new
package or suite cannot enter through a negative filter, wildcard exception, or
directory naming alone.

`tests/compatibility/` is non-executable provenance. Compatibility evidence
must name its former observer and deletion/reactivation condition and does not
belong to a maintained package inventory.

## Validity and reruns

An environment-dependent run has four possible outcomes: product assertion
failure, harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved.

Rerunning is diagnostic evidence and never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
timeout must leave enough observed state to distinguish product, harness, and
environment.

## Test design and retirement

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps are not readiness
or eventual-consistency oracles. Every goroutine, process, listener, file, lock,
timer, and fixture has one owner, cancellation path, join, residue assertion,
and cleanup-failure path.

Two tests are duplicate-removal candidates only when requirement, owning seam,
oracle, transition/fault, platform/format, and independence role are all the
same. A seam migration moves behavior tests to the new Module Interface,
retains only higher-level tests that prove a distinct fact, and retires obsolete
fixtures and runners in the same change.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs beside each retained untrusted
decoder and canonical encoder. Race coverage is required wherever a Module owns
goroutines, callbacks, locks, mutable admission, cancellation, or durable state,
and must assert terminal join and state invariants in addition to race-detector
cleanliness.
