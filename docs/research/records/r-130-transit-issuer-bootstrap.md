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

## Current contract

ADR-0062 fixes a purpose-scoped online Transit Grant signer with a finite
global budget, Request-ID idempotency, fixed OHTTP outcomes, and no Route,
Target, or Service authority. ADR-0053 keeps the Network State root outside
online Node duties. R-128 requires the normal participant path to derive
membership facts from authenticated State rather than an operator Route plan.

The maintained issuer must publish its exact Node-signed OHTTP profile before
authenticated State can select that duty, but it may serve only after State
authenticates the same profile. State may contain several Initiator candidates,
while the accepted issuer ingress admits one exact mutual-TLS Initiator. The
open question was therefore the owner that breaks this bootstrap cycle and the
source of the exact Initiator binding.

## Hypotheses

- **H1:** One owner-only issuer root can atomically create and retain the
  purpose key and OHTTP material, emit only a stable public profile, and later
  serve only the same State-authenticated duty and Initiator binding.
- **H2:** A separate encrypted custody owner is required to release the
  purpose key to the online duty without exposing it through configuration.
- **H3:** Caller-supplied keys and profile can satisfy the contract without
  turning the runtime plan into an undeclared authority.
- **H0:** No evaluated owner preserves the accepted State, Route, and key
  boundaries; the online issuance candidate must then be removed.

## Evaluation criteria

- A runtime plan cannot supply or substitute the signer, profile, or
  Initiator identity.
- Network State root custody never enters the online issuer process.
- Initialization interruption cannot create two profiles for one root, and
  reopen reproduces the same profile and durable ledger.
- The issuer serves only its current authenticated State duty and stops on
  successor, withdrawal, expiry, or mismatch.
- One finite global budget and Request-ID outcome survive restart without
  rollback or duplicate issuance.
- The lifecycle is maintainable by the Product Owner and Codex without adding
  an undeclared custody operator or secret-delivery organization.

## Evidence plan

### Primary sources

- [ADR-0053](../../adr/0053-bootstrap-functional-alpha-network-state.md),
  [ADR-0062](../../adr/0062-scope-online-transit-grant-signing.md), and
  [R-128](r-128-headless-participant-acquisition.md), inspected 2026-08-30.
- `internal/route/credential/contract.go`,
  `internal/route/credential/issuer.go`, and
  `internal/route/credential/profile.go`, inspected 2026-08-30.
- `internal/network/state/node_duty_view.go` and the maintained Node duty
  admission code, inspected 2026-08-30.

### Experiment

No disposable experiment was required. The selected implementation was to be
falsified through behavior tests for initialize/reopen, interrupted root
creation, wrong State, successor, withdrawal, exhaustion, and stable public
profile reproduction, followed by the unpacked enrollment-v3 process tracer.

### Failure scenarios

- interrupted first initialization or profile publication;
- copied or substituted runtime plan, root, profile, or Initiator;
- State authenticates another profile or successor duty;
- budget or Request-ID ledger rollback after restart;
- old duty continues after withdrawal, expiry, or State succession;
- online process obtains Network State root authority.

## Findings

- **Measurement:** the maintained issuer already had fixed encrypted outcomes
  and a durable budget/idempotency ledger, but its callers supplied a raw Grant
  signer and one Initiator identity.
- **Sourced fact:** authenticated State must contain the exact signed issuer
  profile before participants and receiving Nodes may accept the duty.
- **Sourced fact:** ADR-0053 and ADR-0062 forbid reuse of the State root as the
  online Grant-signing key.
- **Measurement:** no accepted owner broke the first-start cycle between
  retained issuer material, public profile publication, and State selection.
- **Inference:** selecting an Initiator from plan order or local configuration
  would add undeclared Route authority.
- **Inference:** accepting every current Initiator would enlarge the ingress
  and budget-abuse surface beyond ADR-0062.

## Options

1. **Issuer-root bootstrap ceremony.** A bounded owner-only initialization
   creates the purpose key and OHTTP material, emits only the stable public
   profile, and serves later only under the matching State duty. This adds a
   two-phase operator lifecycle but no secret-delivery owner.
2. **Separate encrypted issuer custody.** Custody creates and releases the
   purpose key/profile. It strengthens at-rest separation but adds an
   unselected secret-delivery and maintenance workflow.
3. **Caller-supplied raw keys/profile.** Preserve the test-shaped config as a
   product plan. This exposes issuance authority through configuration and
   cannot prove normal restart ownership.

Ingress additionally required either one State-declared issuer/Initiator
binding or admission of the complete candidate set. The latter would amend the
issuer TLS and abuse contract.

## Recommendation

Choose option 1 with one State-declared Initiator binding. Confidence is high:
it keeps private material and interruption reconciliation local to one owner
while reusing the already selected State duty. The strongest argument against
it is the two-phase ceremony and its replacement burden; option 2 would be
preferable if an independently maintained custody workflow were later chosen.

## Disposition

The Product Owner accepted option 1 and the single State-declared Initiator
binding on 2026-08-30. One owner-only issuer root creates and retains the
purpose Grant signer and OHTTP material, emits only the stable Node-signed
public profile, and never exposes the Network State root key. The runtime plan
may name the root and finite bounds but cannot supply a signer, profile, or
Initiator. Restart reproduces the same profile and ledger; succession,
withdrawal, expiry, or mismatch ends the old duty. Rotation requires a distinct
empty root and explicit State ceremony. ADR-0063 records the decision; R-128
and ADR-0062 remain accepted and are not reopened.
