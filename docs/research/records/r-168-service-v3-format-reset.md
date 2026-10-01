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
[threat model](../../security/threat-model.md), [glossary](../../../CONTEXT.md),
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
- **H2:** Required old state or a supported old client exists, so explicit
  migration is needed. An inventory confirming no such obligation falsifies H2
  for the selected closed deployment, but not for unknown external copies.
- **H0:** A supported external client or required persisted root exists, or a
  safety floor cannot be restarted without adoption. Either would require a
  separate migration decision before this reset.

## Evaluation criteria

- **User outcome:** a newly provisioned Authority can issue a Credential,
  Publication, Descriptor and Target Link and complete an authenticated Service
  Connection, Attachment recovery and Terminal; old Service formats never open.
- **Security and failure recovery:** keep the Authority private key and
  protected Service payload confidential against malicious peers under the
  linked threat model; keep signed statements bound to the right Target when
  peers have mixed editions. This format reset alone makes no anonymity claim.
  Unknown markers and versions fail before state mutation, deletion, network
  publication or fallback. New roots retain expiry, authority, anti-rollback
  and resource limits.
- **Budgets and dependencies:** no extra negotiation, read, write, storage or
  operator migration step is accepted. The operator must provision fresh roots
  and distribute a newly issued Link. The one Product Owner can perform that
  action; no other organization or governance actor is assumed.
- **Maturity and usability:** prefer existing maintained Go codecs and their
  tests; add no dependency, cryptographic primitive, license or distribution
  constraint. A visibly versioned Link and explicit refusal make misuse easier
  to diagnose. This source review does not establish installed Ubuntu
  qualification or novice usability.

## Evidence plan

### Primary sources

Source and current owners inspected 2026-09-29 at
`0304241118c4ce4c20a9572d11ed2456beefdeed`: `internal/service/{targetlink,
publication,instance,reachability,connection}`, `internal/endpoint`,
`internal/custody`, `cmd/ardents`, the C0 scope and Endpoint/Service owner linked
above, and ADR-0075, ADR-0102 and ADR-0109. The Product Owner's 2026-09-29
instruction supplies the no-supported-client premise. Source inspection is
not a census of external installations.

### Experiment

No separate spike is needed: the falsification procedure is to enumerate
literal format identities and all call sites in the root Go module, then run
canonical codec, old-format refusal, no-mutation and complete Service journey
tests on the coordinated change. Compare old-root file bytes before and after
attempted opens. `make quick-check` and `make check` are the reproducible
execution profiles; the implementation issue records results. A remaining
accepting old reader or a failed new journey falsifies H1.

### Failure scenarios

Exercise an old Link, connection record, request and persisted root; an unknown
marker or version; mixed peers; a corrupt record; restart after a failed open;
and a failed new-root publication. Preserve old files for operator recovery.
If the no-supported-state premise is disproven, stop this reset and make a
separate migration decision. No independent operator or governance body is
available for a migration service.

## Findings

**Sourced fact:** the source-level producer/reader inventory below was checked
on 2026-09-29 at the commit above. It records maintained code, not deployed
clients.

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

**Sourced fact:** the separate `ardents-route-v3` profile and local
Administration and Application Connection interfaces have different owners;
Target Link algorithm byte 1 identifies the unchanged Target algorithm.

**Assumption:** the Product Owner's report of no supported old clients or roots
applies to this closed deployment; a fresh Service Authority, Instance,
Publication and Reachability root can be provisioned and a new Link distributed.

**Inference:** changing the Target hash domain changes every Target even with
the same Authority key. Old State, Publication and Reachability facts cannot
be attached silently to that new identity. A mixed peer must fail at its exact
record boundary. This is not public compatibility or security qualification.

## Options

| Option | Product and security fit | Operations and governance | Implementation risk / rejection |
| --- | --- | --- | --- |
| Keep existing identities and explain their independent versions | Avoids Link replacement, but leaves mixed Service editions and accepting old roots | No fresh provisioning; preserves obsolete compatibility obligations | Smaller edit, rejected by the direct v3 target and no-client premise |
| Coordinated Service v3 reset | One accepting edition; strict refusal and existing safety floors | One operator provisions fresh roots and distributes new Links; no external governance actor | Target identity changes and simultaneous producer/reader update; choose only with the stated premise |
| Migrate or dual-read old state | Could retain old Targets, but adds an accepting path and a transition policy | Requires inventory, sequencing and support for clients the Product Owner says do not exist | Larger security and maintenance surface; reject absent evidence of required old state |

## Recommendation

Choose the coordinated Service v3 reset in a superseding ADR, without migration,
cleanup, dual-write, fallback or special legacy decoding. **Confidence: high
for the maintained source inventory, conditional for deployment:** the latter
rests on the Product Owner's no-supported-state premise. The strongest
objection is the Target identity change and possible undiscovered persisted
state. If a supported client or required root appears, reopen the decision
before using this path.

## Disposition

R-168 is decided and is not a second active C0 research topic. ADR-0117,
the C0 scope, Endpoint/Service technical owner, command reference, package map
and glossary receive the result with the implementation. No experiment code or
new dependency is retained. Issue #351 holds execution and verification state;
the installed Ubuntu qualification remains a separate gate.
