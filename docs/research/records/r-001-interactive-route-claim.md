---
id: R-001
title: What does the Interactive Route protect?
status: active
owner: product research
started: 2026-08-07
reviewed: 2026-08-07
---

# R-001 — Interactive Route claim

## Decision this unlocks

Define a falsifiable privacy and anonymity contract for the low-latency route
before comparing routing families, libraries, protocols, or implementation
languages. The result must say what each relevant actor can observe, what
collusion breaks the claim, how the claim is tested, and what remains exposed.

## Current contract

- [Product vision](../../product/vision.md)
- [Network functional map](../../product/functional-map.md)
- [Threat model](../../security/threat-model.md)
- [R-002: Live Application Interface](r-002-live-application-interface.md)
- [Domain language](../../../CONTEXT.md)

Already fixed: the Interactive Route is the low-latency Route Profile for one
live, authenticated Service Connection; Application Data is protected end to
end from carrier Nodes; the User and Service should not receive each other's
ordinary location as part of their relationship; and security and performance
are coequal gates.

### P2-D1 — No broad-observer promise for the Interactive Route

**Product Owner decision, accepted 2026-08-07:** V1 does not promise that the
Interactive Route resists a Broad Traffic Observer able to compare timing and
volume near both endpoints or across enough network locations to correlate one
low-latency Service Connection.

The accepted outer claim remains:

- the Service does not learn the User's ordinary network location from Ardents;
- the User does not learn the Service Instance's ordinary network location from
  Ardents;
- any one ordinary intermediary does not learn both ordinary endpoint locations
  and plaintext Application Data under the final Interactive Route conditions;
- none of these statements is a blanket claim that a User, Service, or
  connection is globally anonymous or untraceable.

A delayed, padded, or cover-traffic-heavy Route Profile is a separate optional
product decision under R-005. It is not silently included in the Interactive
Route and is justified only by a concrete Application job, measurable observer
advantage, and acceptable performance and resource budget.

Consequences:

- routing candidates are not rejected merely because a broad observer can
  correlate low-latency timing and volume;
- the product must disclose this limitation wherever it presents the
  Interactive Route privacy claim;
- payload encryption, multi-hop routing, and Isolation Contexts cannot be
  described as protection against broad traffic correlation by themselves;
- padding or cover traffic is not a hidden V1 requirement or an unmeasured
  anonymity setting;
- R-023 may set useful low-latency performance budgets without pretending that
  those budgets also buy broad-observer resistance.

## Remaining decisions

1. **P2-D2 — Local observer:** exactly what an ISP or observer near one endpoint
   sees, what Ardents must hide, and what a Bridge may change without becoming a
   baseline promise.
2. **P2-D3 — One intermediary:** the knowledge and control available to each
   ordinary carrier role and the minimum separation the claim requires.
3. **P2-D4 — Collusion:** which combinations of Nodes or network vantage points
   break location privacy and which combinations must still be resisted.
4. **P2-D5 — Malicious endpoint:** what a malicious User or Service can learn
   from Application Data, connection behavior, and repeated use.
5. **P2-D6 — Active attacks:** required resistance to tagging, replay, delay,
   redirection, route manipulation, and target substitution.
6. **P2-D7 — Conditions and falsification:** required honest behavior,
   diversity, measurements, failure thresholds, and user-facing limitation
   text for the final claim matrix.

## Evaluation rule

Every accepted row in the final claim matrix must state:

1. the information protected;
2. the adversary and its capabilities;
3. the conditions required for the statement to hold;
4. an experiment or analysis that can falsify it;
5. what remains visible, linkable, blockable, or attackable.

The record remains active until every relevant actor has a complete row. No
routing technology can be selected merely by using the words `anonymous`,
`onion`, `mix`, or `decentralized`.

## Evidence plan

### Primary sources

Compare official threat models and protocol/design documents for Tor, I2P, and
Nym, followed by primary traffic-analysis research where those systems make
different claims. Extract adversary capability, protected information,
collusion threshold, latency/resource cost, and explicit non-guarantees rather
than copying system labels.

### Measurement

For each candidate route family, define a reproducible topology and collect the
endpoint, intermediary, and observer views of connection setup, steady traffic,
failure, and repeated connections. R-023 supplies the honest-workload budget;
R-011 later tests whether claimed operator and network diversity exists.

## Alternatives rejected by P2-D1

- **Broad-observer resistance as a V1 promise:** rejected because it is not part
  of the accepted low-latency contract and would be dishonest without a separate
  measurable mechanism and cost.
- **No location-privacy promise:** rejected because it would remove the central
  product value of reaching a Service without revealing ordinary endpoint
  locations to the opposite endpoint or one ordinary intermediary.

## Disposition

- State: `active`.
- P2-D1 accepted: the Interactive Route has no Broad Traffic Observer
  timing-and-volume correlation-resistance claim.
- The endpoint-location and single-intermediary goals remain accepted, but their
  exact conditions and collusion boundary are not yet complete.
- P2-D2, the local-observer boundary, is next.
- No routing family, protocol, library, implementation language, ADR, or code is
  selected.
