---
id: R-168
title: Can the closed Service formats move directly to one v3 edition?
status: decided
owner: Product Owner
started: 2026-09-29
reviewed: 2026-09-29
---

# R-168 — Closed Service v3 format reset

## Decision this unlocks

Decide whether the closed Service can change its current text, wire, signed,
hash-domain, and persisted identities together without a reader, migration, or
grace period for earlier editions.

## Current contract

The [C0 scope](../../product/scope.md), [protected text workload](../../product/protected-service-workload.md),
and [Endpoint/Service owner](../../technical/endpoint-service-runtime.md) select one
Target-Link Service path. ADR-0075 selects Service Connection v2; ADR-0102
selects Credential v3 and Instance request/root v2; ADR-0109 selects private
Descriptor v3 and typed refusal of a legacy Store envelope. The scope's general
compatibility sentence conflicts with the Product Owner's explicit 2026-09-29
direction: there are no supported old clients, and the target scheme should be
used directly. No claim about uninspected external installations follows.

## Hypotheses and falsification

- **H1:** Every maintained producer and accepting reader can switch together to
  one Service v3 edition, while an old root/link/record fails before effects.
  A retained accepting old reader, a hidden producer of old bytes, or a new
  root that cannot complete a Service Connection would falsify H1.
- **H0:** A supported external client or required persisted root exists, or a
  safety floor cannot be restarted without adoption. Either would require a
  separate migration decision before this reset.

## Evidence and format inventory

Source inspected 2026-09-29 at `0304241118c4ce4c20a9572d11ed2456beefdeed`.
This is source evidence, not a deployed-client census.

| Format/identity | Current producer and reader | Current bytes | v3 disposition |
| --- | --- | --- | --- |
| Target derivation and Link | `publication.Target`; Custody, Endpoint, `targetlink` | target domain v1; `ardents-target:v1:` | domain and text prefix v3; old Target/Link invalid |
| Credential and private Descriptor | `publication`, `reachability`; Endpoint, Node | both v3 | retain v3 |
| Instance request, response, root | `instance`; Custody, command | v2 | v3; old root refused without state decode |
| Publication record and root | `publication`; Endpoint, Reachability | v1 envelope/root around Credential v3 | v3; old root/record refused |
| Reachability root and stored record | `reachability`; Node | root v1, private envelope byte 2 | root v3, envelope byte 3; old records refused |
| Service Connection | `connection`; Endpoint | record v2, fixed retired Route profile v2, context/continuity domains v1, TLS exporter v1 | record and domains v3; Service-specific fixed profile v3 |

The separate `ardents-route-v3` profile and the local Administration and
Application Connection interfaces are outside this decision. Target Link's
algorithm byte 1 denotes the same Target algorithm and does not change.

## Evaluation and failure scenarios

The Product Owner states that no supported old clients or roots require
compatibility. **Assumption:** a fresh closed deployment can provision new
Service Authority, Instance, Publication, and Reachability roots and distribute
new Links. Changing the Target hash domain changes every Target even if an
Authority public key is reused. Old State/reachability/publication facts cannot
be silently attached to the new Target. A mixed peer must fail at its exact
record boundary; an old root or Link must fail without deletion, import, or
network effects. New roots still retain their authority and rollback floors.

The alternatives were (a) retain existing format identities and document their
independent versions, or (b) reset Service-owned identities together. The
Product Owner explicitly selected (b), including v3 labeling and fresh-root
refusal, on 2026-09-29. This is a closed-network transition, not evidence for
public compatibility or security qualification.

## Recommendation and disposition

Choose one coordinated Service v3 reset in a superseding ADR. Change every
maintained producer and reader together, remove special legacy decoding and
typed legacy branches, reject unknown formats without touching their bytes,
and prohibit automatic cleanup or fallback. The strongest objection is the
Target identity change; it is accepted only under the stated fresh-deployment
assumption. Reopen design if a supported client or required persisted root is
found. No experiment or new runtime dependency is needed for this source-level
decision. The research question is decided, not an additional active C0 topic.
