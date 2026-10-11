---
id: R-108
title: Alpha-control reader v1 wire and authority mapping
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-108 — Which bounded wire and component mapping lets an H4-6A reader verify disclosed alpha control without treating the catalog as Endpoint authority?

## Decision this unlocks

The maintained H4-6A catalog/reader implementation and its exact alpha-bundle
companions.

## Current contract

R-098 selects one signed disclosure catalog and component-local verification.
Release and Network State already own their roots, verification, and floors.
The Alpha Enrollment Pin authenticates only an exact first bundle inventory.

## Hypothesis and falsifier

**H1:** a closed binary catalog referencing fixed component statements, each
verified under a different enrollment-pinned component key, plus a reader-owned
catalog floor and individually invoked component verifiers, makes one cohort
inspectable without granting the catalog signing key Endpoint admission
authority.

The hypothesis fails if an altered or lower-floor catalog/component can be
reported accepted, a catalog can select or replace a component signing key,
a catalog key can alter Release/State acceptance, a component can select a
filesystem path or network source, or reader inspection mutates an Endpoint's
Release/State root.

## Evidence plan

Use the current maintained Release and Network State verifiers with synthetic
bounded inputs. Prove valid inspection, changed bytes, expired catalog,
catalog replay/conflict, unavailable/oversized component, a statement signed
by a catalog-selected but enrollment-unpinned key, and independent local
floors. Inspect exact filesystem ownership and prove the reader never starts
the candidate Endpoint.

The TUF documentation confirms that roles, expiration, hashes, and versioned
metadata separate authority for software update decisions; it does not provide
a Network State or general catalog authority. Accessed 2026-08-25:
<https://theupdateframework.io/docs/metadata/> and
<https://theupdateframework.github.io/specification/v1.0.28/>.

## Result

ADR-0038 is accepted with the following concrete v1 result:

- `ACA1` is a signed, fixed three-entry catalog. It names no location or key;
  it binds a component class, exact statement digest/length, component expiry,
  predecessor, and reader-local generation floor.
- `ACS1` is a signed component statement. `catalog.pub`, `release.pub`,
  `network.pub`, and `compatibility.pub` are distinct raw Ed25519 roots in the
  enrollment-pinned inventory. A catalog entry has only a root identifier, so
  it cannot select a replacement signing key.
- `ACR1`, `ACN1`, and `ACC1` respectively bind the independently evaluated
  Release Decision, complete offline Network State decision (including Epoch
  digest), and the resulting Release/Network compatibility tuple. The reader
  claims one marked inspection root with distinct catalog, Release, and Network
  State child floors; it refuses a nonempty unowned root.

## Behavior evidence

- `go test ./internal/alphacontrol` covers canonical signatures, altered and
  expired bytes, oversize input, unavailable component, component-root
  substitution, conflict, rollback, persistent floors, and lease/restart.
- `go test ./internal/alphacontrol/inspection` evaluates the maintained frozen
  Release vector, a synthetic but maintained Network State Epoch, and the
  cross-component compatibility binding.
- On 2026-08-25, Linux Docker completed
  `go test ./tests/e2e/endpoint -run TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart -count=1 -v -timeout 3m`:
  the separately built reader accepted an enrollment-pinned TUF bundle and
  its cached restart, with all three statements accepted. It did not execute
  the enrolled Endpoint artifact.

## Limitations retained

The alpha enrollment pin, TUF keys, component roots, and disclosed Network
authority configuration remain project-operated. This gives participants a
reproducible, bounded view of the actual inputs; it is not independent control,
automatic distribution, live Endpoint readiness, or a public-control result.
