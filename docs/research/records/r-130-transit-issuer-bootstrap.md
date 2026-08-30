---
id: R-130
title: How is the purpose-scoped Transit Grant issuer bootstrapped into authenticated State?
status: decided; promoted to ADR-0063
owner: Product Owner and Codex
started: 2026-08-30
reviewed: 2026-08-30
---

# R-130 — Transit issuer bootstrap and ingress ownership

## Decision this unlocks

Select the smallest honest bootstrap and restart lifecycle for ADR-0062's
purpose-scoped online Transit Grant signer, and select how its mutual-TLS
Initiator ingress is derived when current State contains more than one eligible
Initiator. This decision is required before `ardents-node` can expose a
supported issuer process and before the artifact-native enrollment-v3 journey
can exercise membership acquisition without a source fixture.

## Verified blocker

The maintained `credential.Issuer` is a real bounded handler with fixed OHTTP
outcomes and a durable budget/idempotency ledger, but every current caller is a
behavior test. `IssuerConfig` receives a raw Grant signer private key and one
static Initiator identity from its caller. `NewIssuer` also creates or reopens
the retained OHTTP key material and then emits the Node-signed issuer profile.

Authenticated Network State must already contain that exact profile before a
participant or receiving Node can accept the signer. A normal first start
therefore has a cycle:

```text
issuer root/key material -> exact signed profile -> authenticated State
authenticated State      -> current issuer duty  -> issuer process start
```

No accepted owner currently breaks that cycle. Supplying the signer key,
profile, State duty, or Initiator through an operator runtime plan would make
the plan an undeclared authority and would contradict R-128's normal
participant path. Reusing the State root key is forbidden by ADR-0053 and
ADR-0062.

Current State can authorize multiple Initiator candidates, while
`IssuerConfig` accepts exactly one mutual-TLS Initiator public key. Choosing one
from plan order or local configuration would silently add Route authority.
Accepting all current Initiators changes the issuer ingress and budget-abuse
surface and therefore also needs an explicit choice.

## Candidate decisions

1. **Issuer-root bootstrap ceremony.** A bounded local initialization creates
   the purpose key and OHTTP material inside a new owner-only issuer root,
   emits only the exact public Node-signed profile for the State ceremony, and
   can later serve only when current State authenticates the same profile and
   duty. The runtime plan never carries either private key. This is the
   smallest candidate consistent with ADR-0062, but it adds a two-phase
   operator lifecycle and needs an interruption/replacement contract.
2. **Separate encrypted issuer custody.** A custody operation creates and
   releases the purpose key/profile to the online duty. This gives stronger
   at-rest separation but creates another secret-delivery and maintenance
   workflow for the current team; no such owner is presently accepted.
3. **Caller-supplied raw keys/profile.** Retain the current test-shaped config
   as a product plan. This is rejected unless the Product Owner explicitly
   changes the authority contract: it exposes authority through configuration
   and cannot prove normal restart ownership.

Ingress must additionally choose either one State-declared issuer/Initiator
binding or the complete bounded set of current `initiator` candidates. The
former requires State to carry that binding; the latter requires an amended
issuer TLS/admission contract and explicit global-budget abuse analysis.

## Falsification and evidence

Reject a candidate if a runtime plan can substitute a signer/profile or
Initiator, if State root custody enters the online process, if interrupted
initialization can publish two profiles for one root, if restart changes the
profile, or if a State successor can continue serving under the prior duty.
The accepted implementation must prove initialize/reopen, wrong-State,
successor, withdrawal, exhausted, and interrupted-publication cases before the
unpacked enrollment-v3 journey can be called complete.

## Current disposition

The Product Owner accepted candidate 1 on 2026-08-30 with the single
State-declared Initiator binding. One explicit owner-only issuer-root ceremony
creates and retains the purpose Grant signer and OHTTP material, publishes only
the stable Node-signed public profile, and never exposes the Network State root
key. The exact profile declares one permitted Initiator identity/key; State
binds that ingress by authenticating the profile under its sole issuer duty.

The runtime plan may name the issuer root and finite local runtime bounds but
cannot supply a signer, profile, or Initiator. The first accepted State duty
binds the root's durable budget and ledger. Restart reproduces the same public
profile and ledger; State succession, withdrawal, expiry, or mismatch ends the
old duty. Rotation and replacement require a distinct empty root and an
explicit new State ceremony. ADR-0063 records the durable decision. R-128 and
ADR-0062 remain accepted and are not reopened.
