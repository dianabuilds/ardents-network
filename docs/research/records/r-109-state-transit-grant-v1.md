---
id: R-109
title: State-authorized C-2 transit grants v1
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-109 — Which authority can issue and a State-authorized Introduction or Responder Node can verify one finite `EndpointTransitBinding` authorization without learning Service data or adding a live control service?

## Decision this unlocks

Select or reject the missing maintained admission owner for the H4-3A
Introduction and Responder duties. The decision must let a two-Endpoint
Publisher-to-User Reference Site route use the accepted C-2 topology rather
than test-only callbacks.

## Current contract

- ADR-0035 fixes `EndpointTransitBinding` v1: it binds Network, Epoch digest,
  one attachment, exact transit role/Node, expiry, mTLS client-key digest, and
  an opaque finite authorization. The receiving Node must consume that
  authorization atomically.
- `internal/node` owns State-pinned Introduction and Responder duties, while
  `internal/route.AdmitEndpointTransitBinding` deliberately delegates the
  authorization decision to an opaque callback.
- The seven-process Reference C-2 fixture supplies that callback from
  synthetic configuration. It is explicitly not participant configuration.
- Service Authority and Publisher Instance material must not become a Node
  authorization root. A Node must not learn a Target, Publication, HPKE
  plaintext, complete Route, or an alternate peer from this decision.
- Network State authorities are already the trust root that Node State uses to
  accept its exact Network/Epoch/digest and local assignment.

## Hypotheses

- **H1:** a Network State-authority signature over an opaque exact transit
  tuple can authorize one Introduction or Responder TLS admission with only a
  local one-use replay ledger at the Node.
- **H2:** a separate live control/storage owner must register and spend every
  authorization before the Node admits a leg.
- **H0:** neither construction preserves State/Service separation; H4-3A must
  narrow rather than expose a participant route.

## Evaluation criteria

- The Node verifies only its current State authority key, Network, Epoch,
  digest, role, identity, mTLS key digest, attachment, expiry, and one grant
  identifier; it receives no Service Target or plaintext.
- A grant is exact, one-use, bounded by the selected State/record validity,
  and invalid after State withdrawal or key/role substitution.
- A Publisher can retain a Responder grant privately and publish only the
  independently necessary Introduction grant in the already bounded
  Reachability Descriptor.
- No live database, public broker, direct Publisher ingress, generic proxy,
  service-authority trust at Nodes, or Node-selected retry is introduced.
- The construction fits a closed one-person alpha: issuing a finite batch at
  publication time is explicit operational control, not a claim of
  permissionless service admission.

## Evidence plan

### Primary sources

- ADR-0035 and `internal/route/endpoint_transit_binding.go`, inspected
  2026-08-25.
- `internal/node` Introduction/Responder duty construction and the historical
  [Reference C-2 synthetic callback](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/tests/e2e/service/fixturecommand/reference-c2),
  inspected 2026-08-25.
- Network State authority and Node-duty views in `internal/network/state`,
  inspected 2026-08-25.

### Experiment

Build one canonical signed-grant vector and a Node-local durable spend test.
It must pass a valid Introduction and Responder admission and refuse changed
Network/Epoch/digest/role/Node/attachment/key/expiry, authority substitution,
replay after restart, and State withdrawal. A later two-Endpoint process run
must prove that the Introduction grant is the only such grant exposed through
the reachability path.

### Failure scenarios

- A malicious Publisher replays, swaps, extends, or presents a Responder grant
  as an Introduction grant.
- A Node accepts a grant after authority rotation, State withdrawal, or expiry.
- A grant leaks a Target or lets a Node choose a peer or fallback.
- A lost/delayed control service makes an existing finite grant unusable.
- Two attempts race the same grant, including across process restart.

## Findings

- **Current-code fact:** `EndpointTransitBinding` has the necessary exact
  tuple, but its opaque authorization has no maintained issuer or persistent
  spender; only the E2E fixture supplies an `Admit` closure.
- **Current-code fact:** Node duties already derive their listener identity,
  peer constraints, expiry, and State assignment from fresh authenticated
  State. They must not derive this missing authorization from an arbitrary
  Endpoint plan.
- **Inference:** signing a compact grant with an already trusted Network State
  authority gives a Node an offline verification root without adding Service
  Authority trust or a live control database.
- **Inference:** a finite Publisher-held batch is compatible with the one
  bounded Reference Site alpha. Its issuance is explicit project control and
  cannot support a permissionless or high-volume claim.
- **Implementation finding:** because the signed tuple contains the TLS
  client-key digest, a raw offline grant alone is unusable: the Endpoint must
  possess its matching one-use private TLS client key before opening the
  attachment. The closed-alpha provisioned capability is therefore the pair,
  while only the opaque Introduction grant can enter the public Descriptor.
- **Implementation finding:** Transit Grant is adjacent-hop admission only.
  Introduction must not compare it to the separately Publisher-chosen
  `JoinHandle`; the latter remains in the registered slot and sealed record.

## Options

1. **State-authority-signed opaque Transit Grant v1.** The grant carries only
   the exact `EndpointTransitBinding` tuple and a grant identifier under an
   already accepted Network State authority signature. Node State verifies its
   issuer key; the duty durably spends its identifier. Publisher retains the
   Responder grant and its published descriptor carries only the Introduction
   grant. This is the smallest offline construction, but requires a versioned
   record, authority issue operation, Node replay persistence, and an ADR.
2. **Live registration/control owner.** Publisher or User registers each grant
   with a selected remote owner, which Nodes query/spend. This adds a separate
   availability, privacy, storage, and governance role not selected by H4-3A;
   reject unless a later product decision explicitly adopts it.
3. **Static Node plan strings or Service-Authority signatures.** Reject:
   the former makes a fixture configuration a hidden authorization service;
   the latter gives Nodes a Service authorization root and changes exposure
   and revocation semantics without a current contract.

## Recommendation

The Product Owner accepted option 1 for the closed alpha on 2026-08-25: one
State-authority-signed opaque Transit Grant v1, with Node-local durable
spending and no live control service. The strongest counterargument is that
State-authority issuance remains project-operated control; the alpha must
disclose that limitation and must not claim independent admission.

## Disposition

Decided. ADR-0039 fixes the issuer and spending boundary. The maintained
codec, Node-local spend ledger, and composition evidence remain H4-3A delivery
work; acceptance does not establish a permissionless admission or independent
operation claim.
