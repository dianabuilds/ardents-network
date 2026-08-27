---
id: R-121
title: Which bounded authority operation may create the first functional-alpha Network State without reusing Release/control keys, fixture identities, or implying independent governance?
status: decided
owner: Product Owner and Codex
started: 2026-08-27
reviewed: 2026-08-27
---

# R-121 — Which bounded authority operation may create the first functional-alpha Network State without reusing Release/control keys, fixture identities, or implying independent governance?

## Decision this unlocks

Create the real Network State input required by the fixed H4-alpha-1 release
profile so ADR-0052 can assemble and preflight the signed H4-1/H4-6A static
directory. The decision must not turn fixture encoders into release machinery,
reuse a Release or disclosure key, or claim a threshold organization that does
not exist.

## Current contract

ADR-0004 separates Release, Network Epoch, Namespace, qualification, and
emergency powers and states that the one-person project remains visibly
centralized and provisional. ADR-0038 keeps the H4-6A Network disclosure key
non-authorizing. ADR-0050 fixes a ten-role Release/control seed record whose
selected encrypted-envelope digest is already part of the alpha profile.
ADR-0052 accepts only a complete verifier-accepted Network State decision.

The active team is one Product Owner and Codex. H4-alpha-1 must qualify the
exact candidate on TCP/TLS, but it does not claim persistent capacity,
availability, independent operators, censorship resistance, or Public Beta
governance. The current State Module verifies canonical records and Epochs;
only tests encode or sign them.

## Hypotheses

- **H1:** One separately encrypted Product Owner-held 1-of-1 Epoch key and a
  fixed 30-day empty-candidate genesis can supply an honest, verifier-accepted alpha
  control input without widening the State Module into a generic signer.
- **H2:** Extending the Release seed record with a State key or publishing a
  test-generated Epoch can satisfy the same separation and provenance rules.
- **H0:** No local one-person operation can produce an acceptable State input;
  release must wait for external custodians or persistent operators.

## Evaluation criteria

- The exact H4-alpha-1 public input passes the maintained State verifier.
- State authority uses an independent key and encrypted record; no private key
  or passphrase enters Git, chat, command arguments, environment, logs, bundle,
  CI, GitHub, or a VPS.
- The operation accepts no arbitrary message, signer, profile, Node record,
  source, URL, upload destination, or threshold.
- The result states the actual candidate topology and does not manufacture
  operator independence, capacity, availability, or route readiness.
- The retained key permits a later explicit successor decision, but the first
  operation cannot sign one.
- The implementation is testable through the State Module interface and adds
  no new cryptographic primitive or runtime dependency.

## Evidence plan

### Primary sources

- ADR-0004, ADR-0038, ADR-0050, ADR-0052 and the H4-alpha-1 profile, accessed
  2026-08-27.
- `internal/network/state` verifier, canonical decoder, commitment, and durable
  acceptance implementation, inspected 2026-08-27.
- `internal/release/custody` fixed Argon2id/AES-GCM custody profile, inspected
  2026-08-27 only as an existing reviewed construction, not as a State key
  owner.
- H4-2 local and multihost qualification contracts, accessed 2026-08-27.

### Experiment

Implement one fixed `InitializeAlphaGenesis` State Module interface and test
that its public bytes pass `state.Open` plus `Accept` at the recorded reference
time. Tests must also prove refusal to overwrite either output, reject unsafe
times before asking for a secret, retain no Node candidate, and never expose the
private seed in the public output.

### Failure scenarios

- Reusing the Release/control encrypted record or disclosure key.
- Calling a general signer or supplying arbitrary Epoch bytes to sign.
- Treating a known fixture key, fixture record, or ephemeral qualification
  identity as release authority.
- Recording fictitious Node/operator families or a non-running candidate as
  alpha topology.
- Losing the single passphrase/key, replacing an existing record, partial
  publication, or allowing the signed State to outlive its release interval.

## Findings

- **Sourced fact:** ADR-0004 requires separate Release and Network Epoch roles
  and explicitly says the one-to-one project uses centralized provisional
  keys.
- **Sourced fact:** ADR-0038's `alpha-network-component` key signs disclosure
  evidence only and is never passed to Network State acceptance.
- **Sourced fact:** ADR-0050 fixes exactly ten Release/control roles. Adding an
  Epoch key would change the selected encrypted record and invalidate the
  recorded H4-alpha-1 custody companion.
- **Measurement:** Maintained non-test State code verifies Epochs and records
  but exposes no encoding or signing operation. The encoders and private keys
  found by repository search are test fixtures.
- **Sourced fact:** State accepts a canonical Epoch whose complete candidate
  view is empty; materialization is required only when an accepted candidate
  exists.
- **Inference:** An empty candidate view is the only truthful initial topology
  until a persistent Node is separately provisioned. It proves no readiness or
  availability, but H4-1 enrollment and H4-2 temporary two-host qualification
  do not require those excluded claims.
- **Inference:** A 3-of-5 key set held by the same person adds ceremony and
  misuse surface without adding independent control. A labelled 1-of-1 root is
  more accurate for this alpha.

## Options

### A. Separate encrypted 1-of-1 State authority and fixed empty genesis

The State Module owns one profile-bound initialization interface. It generates
the Network identifier, Epoch key, and assignment seed internally; fixes the
interactive Route profile, four role domains, threshold one, epoch one, a
30-day validity interval, and an empty input/view; verifies the result before
atomically publishing one encrypted seed plus public request fragment below the
owner root. This fits the actual team and makes its limitations machine-visible.

### B. Add a State role to Release custody

Rejected. It changes an already selected record, collapses Module ownership,
and makes a disclosure/release ceremony the holder of Network authority.

### C. Promote a test fixture or H4-2 qualification Epoch

Rejected. Fixture identities are known or ephemeral, their topology and
validity are test-owned, and the release ceremony expressly prohibits them as
release inputs.

### D. Create nominal 3-of-5 State keys under one custodian

Rejected for functional alpha. It provides no independent governance and
creates a misleading resemblance to the Public Beta threshold.

### E. Wait for independent custodians and persistent operators

Rejected for the current objective, whose scope explicitly excludes those
Public Beta gates. It remains the required direction before stronger claims.

## Recommendation

Choose Option A with high confidence. The strongest argument against it is
that an empty State cannot give a participant Target Connect readiness. That
is an honest limitation, not a hidden failure: the current goal proves
authenticated enrollment and exact TCP/TLS candidate behavior, while persistent
operator capacity and availability are excluded. Any later non-empty State or
successor needs a new bounded operation and real Node custody/deployment facts.

## Disposition

Decided on 2026-08-27 and promoted to ADR-0053. Implement the fixed State Module
interface, terminal Adapter, verifier tests, package/dependency ownership, and
release-ceremony text. Retain no experiment directory; the maintained tests are
the reproducible behavior evidence.
