---
id: R-153
title: Preserve issuer-bootstrap restrictions across authenticated Node children
status: completed
owner: Product Owner and Codex
started: 2026-09-09
reviewed: 2026-09-09
---

# R-153 — How does bootstrap restriction survive an opaque inner TLS channel?

## Decision this unlocks

Specify the child-local restriction already required by the closed admission
contract before enabling Entry bootstrap and the real issuance path. The
Product Owner approved the proposed authenticated restriction on 2026-09-09.
This does not approve a new authority, bypass receiver admission, or qualify
an implemented path. Canonical encoding and compatibility need an explicit
amendment of ADR-0081's selected owner documents.

## Current contract

[ADR-0081](../../adr/0081-select-closed-protected-service-contract.md),
[private admission](../../technical/private-admission.md),
[forwarding](../../technical/protected-route-protocol.md) and
[migration](../../development/privacy-qualification.md#integration-and-migration-contract)
require purpose-restricted bootstrap, independent inner admission and exact
adopted artifact/profile bindings. A pooled Node Carrier grants no Endpoint
admission. [The threat model](../../security/threat-model.md) includes malicious
peers; an Endpoint assertion cannot supply missing receiving authority.

## Hypotheses

- H1: a mandatory restriction in the authenticated outer child's allocation can
  enforce issuer-only bootstrap without becoming a context identity or a grant.
- H2: Endpoint behavior alone or one property of a pooled Carrier suffices.
- H0: neither can enforce the restriction under opaque inner TLS and mixed
  children without revising the selected framing contract.

## Evaluation criteria

The receiving honest Interior must refuse a private ADMIT even if its token is
otherwise valid when the preceding honest Entry admitted only bootstrap. No
Endpoint field may remove this restriction. Ordinary and bootstrap children
must coexist in one exact Node/profile/Carrier pool. No context label crosses
hops. Missing/unknown encodings and mixed old/new peers fail before inner
allocation. Existing byte/time/queue limits remain finite and include the new
byte. No extra exchange, transport, dependency, authority or fallback is added.

## Evidence plan

### Primary sources

Read on 2026-09-09: the linked current owners and maintained source
`internal/route/closed_outer_handshake.go` (49-byte OPEN),
`internal/route/closed_lane.go` (length validation),
`internal/node/closed_forwarding_link.go` (opaque next-hop transport), and
`internal/node/closed_forwarding_listener.go` (inner admission). These local
sources determine the grammar; no external library behavior is assumed.

### Experiment

This is a bounded contract analysis, not a network qualification measurement.
Implementation acceptance must serialize both forms and drive the real
receiving code with absent/unknown restriction, valid private tokens inside a
restricted child, mixed pooled children, attempted relabelling, both Carriers,
withdrawal and queue exhaustion. Those tests remain implementation obligations.

### Failure scenarios

Endpoint submits a valid Interior token through an Entry bootstrap prefix;
an old sender omits restriction; a compromised Application adds a field;
a peer supplies an unknown value; issuance succeeds but the old restricted
child is reused privately; cancellation races allocation; an old binary or
profile is restored during adoption.

## Findings

- **Sourced fact:** the former OPEN is exactly 49 bytes and has only recipient,
  duty, purpose and deadline. The outer Carrier has mutually authenticated
  Node identities; inner TLS terminates at the next role.
- **Inference:** the former grammar cannot distinguish a bootstrap child from
  an ordinary child to the same Interior. A valid inner ADMIT can consequently
  bypass the Entry purpose restriction while retaining other hop budgets.
- **Inference:** an immutable child restriction authenticated by the existing
  Carrier supplies the missing negative constraint. It does not prove the
  previous hop's token or grant permission at the next receiver.
- **Sourced fact:** current adoption binds an exact artifact/profile and keeps
  generation 2 separate. Existing compatibility policy requires explicit
  quiescence, floor preservation and unavailable incompatible peers.

## Options

1. Keep 49 bytes and enforce only honest Endpoint behavior: fails H1's adversarial
   case. A Carrier-wide flag also fails mixed-child isolation.
2. A new frame kind or protection generation: can carry the constraint, but
   expands identifiers and migration across otherwise unchanged role frames.
3. Select exact50-byte Node-outer OPEN with mandatory trailing restriction u8;
   retain exact49-byte Endpoint-role OPEN. Authentication already distinguishes
   the two channel states. Both are fixed grammars, with no optional field or
   negotiated fallback. Old/new Node OPEN cannot be silently accepted.

## Recommendation

Choose option 3: value 0 means no additional child restriction, never a grant;
value 1 means issuer-bootstrap only. No other value or omission is valid. A
receiving Node binds it before inner TLS allocation. The forwarding owner
sets it from its actual parent reservation; Endpoint-role OPEN has no such
field. Restricted children remain restricted until terminal close. Private work
requires genuine admission and fresh children under an admitted parent.

The strongest objection is incompatible framing within generation 3. Address
it explicitly as a pre-qualification contract amendment: retire the former
49-byte Node-outer form, reject both mixed directions before child effects,
quiesce experimental users of that form, and adopt the exact revised artifacts
with a fresh signed profile/duty binding under the existing migration rules.
No former receipt qualifies the revised bytes. Generation2 remains unchanged.

## Disposition

Promoted to [ADR-0082](../../adr/0082-bind-bootstrap-restriction-to-node-child.md) and current technical
owners. No deployed compatibility or security qualification claimed.
