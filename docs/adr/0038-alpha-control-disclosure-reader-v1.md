---
status: accepted
date: 2026-08-25
supersedes: none
---

# ADR-0038 — Keep alpha disclosure separate from acceptance authority

## Context

H4-6A requires an alpha participant to inspect the release, Network State, and
compatibility inputs that control a result. R-098 selected one signed disclosure
catalog plus component-local verification, but deliberately left its concrete
wire, roots, and reader open.

Existing Release and Network State owners already verify their own signed
inputs and maintain their own floors. A catalog that supplied replacement roots
or drove Endpoint admission would silently collapse those authorities into its
project-controlled signing key.

## Decision

The first H4-6A reader consumes a small, closed binary `Alpha Control Catalog
v1` and four fixed sibling statements: `catalog.ac1`, `release.ac1`,
`network.ac1`, and `compatibility.ac1`. The Alpha Enrollment inventory also
contains the four corresponding raw Ed25519 public-key companions:
`catalog.pub`, `release.pub`, `network.pub`, and `compatibility.pub`. There
are no catalog-supplied paths, URLs, mirrors, keys, or optional/unknown
component classes. Each catalog entry binds only the exact statement's
SHA-256 digest, byte length, class, validity interval, identifier of the
already pinned component root, and non-decreasing component generation.

The catalog is signed with one Ed25519 **disclosure** key. Its public key, the
three distinct component public keys, and first catalog digest are
first-artifact companions covered by the Alpha Enrollment Pin's exact
inventory; later catalog replacement is a manual, explicit reader operation
under a durable catalog floor. A component statement must verify under its
independently pinned companion key before its own verifier runs. Thus the
catalog cannot invent a replacement component key by merely changing `RootID`.
None of these disclosure keys is passed to Release, Network State, Namespace,
Update, Route, or Endpoint admission.

The standalone `ardents-control` reader claims only an empty or already marked
inspection root, then owns its fixed `catalog`, `release`, and `network`
children. It writes only inspection-local floors there, verifies catalog
signature, expiry, predecessor and floor, then invokes a separate verifier for
each fixed component:

- `release` uses the maintained Release verification path and reports its
  independently authenticated TUF root/floor/outcome;
- `network` uses the maintained Network State acceptance verifier against its
  disclosed authority configuration and reports the authenticated Epoch/View;
- `compatibility` uses a distinct, versioned Ed25519-signed compatibility
  statement and its own persisted generation/digest floor.

The reader produces a structured inspection report. A report never authorizes
an Endpoint, installs an artifact, imports a State generation into the active
Endpoint root, starts a listener, or makes a project-controlled root appear
independent. Any unavailable, oversized, altered, expired, unknown, conflicting
or lower-floor input yields a named component rejection without fallback.

## Consequences

- A closed binary catalog avoids JSON canonicalization, duplicate-key, and
  parser-policy ambiguity in the outer signed authority boundary.
- Component files remain purpose-specific; the catalog gives a coherent cohort
  index but cannot select a component signer or validate altered Release,
  State, or compatibility bytes.
- H4-4 is absent from v1. A named-alpha corpus can only add a new component
  class through a later accepted versioned decision.
- The initial companion is covered by the Alpha Enrollment Pin, not by a new
  bootstrap key or automatic downloader. Real participant provenance and a
  released reader remain H4-1/H4-8 qualification work.

## Compliance

R-098 supplies the authority-separation decision. R-108 records the exact
wire/root choice and behavior evidence: altered, expired, oversized,
unavailable, substituted-root, conflicting, and lower-floor catalog/component
inputs reject; Release and Network State run their maintained verifiers on
separate roots; and a Linux process test performs a pin-verified first check
and cached restart without starting the candidate Endpoint.
