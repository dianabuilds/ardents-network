---
id: R-133
title: Role-scoped Transit Grant acquisition for a headless Publisher
status: open; decision-ready
owner: Product Owner and Codex
started: 2026-08-31
reviewed: 2026-08-31
---

# R-133 — Which issuance request may acquire a Responder Transit Grant?

## Decision this unlocks

Select the smallest versioned issuance request and Endpoint lifecycle that can
supply the headless Publisher's State-selected Responder attachment without a
fixture authorization, operator Route plan, Target disclosure, or change to
Transit Grant v1.

This decision blocks only the maintained Publisher's Responder attachment and
therefore the complete artifact-native B6 journey. The local Service
Administration surface, Instance/Authority acquisition, Browser separation,
and artifact boundary remain independent implementation work.

## Current contract

- ADR-0035 requires the Publisher to use a separately admitted Responder first
  hop after it accepts one sealed Introduction.
- ADR-0039 fixes one `Transit Grant v1` for either `IntroductionRole` or
  `ResponderRole`; the role and receiving Node are signed exact-tuple fields.
- ADR-0047 and ADR-0062 define only dynamic Introduction admission. Their
  versioned OHTTP request names `IntroductionNodeID` and has an implicit fixed
  `IntroductionRole`.
- R-132 requires every one-use Publisher TLS identity to come from Endpoint's
  acquisition owner, while preserving Route, Target, C-2, Credential, and
  Transit Grant v1 semantics.
- Enrollment-v3 must not carry fixture commands or an operator-prepared Route
  plan. Browser and Application callers must not receive a Grant, key, State
  view, or peer facts.

The maintained issuer therefore cannot honestly issue the Responder Grant
required by the maintained Publisher path. Treating `IntroductionNodeID` as a
Responder identifier would contradict the accepted request grammar; embedding
a fixed fixture Grant would contradict the supported acquisition journey.

## Hypotheses

- **H1:** A v2 target-free issuance request containing one exact `TransitRole`
  and `TransitNodeID` can issue either the already accepted Introduction or
  Responder Transit Grant while preserving Transit Grant v1 and every C-2,
  Route, and Target semantic.
- **H2:** Introduction and Responder require distinct issuance operations or
  profiles because one shared grammar would give the signer an unacceptably
  broad purpose.
- **H3:** A finite enrollment-v3 inventory of fixed Responder Grant/key pairs
  can satisfy the Publisher journey without dynamic role-scoped issuance.
- **H0:** No option satisfies the accepted headless usable-alpha contract;
  fresh Publisher start must remain unavailable.

## Evaluation criteria

- `publish` and later `accept` receive no Node, role, endpoint, Grant, key,
  Target, Credential, or Route-plan input from the Application or operator.
- Endpoint derives the exact receiving Node and role from one current
  authenticated State and creates a fresh attachment/TLS key for one request.
- Each request consumes at most one existing issuer budget unit and retains
  R-128's Request-ID idempotency, fixed encrypted outcomes, and at-most-once
  present/burn recovery.
- The issuer sees no Name, Target, Publication, Credential, JoinHandle,
  Reachability, complete Route, endpoint address, or Application data.
- A receiving Node still admits only a matching current duty, Grant role,
  Node ID, attachment, TLS-key digest, and deadline; the signer never selects
  or substitutes a Route position.
- The request remains one fixed-size OHTTP message and adds no periodic retry,
  fallback, standing operator ceremony, Browser artifact, or runtime
  dependency.
- A malformed or unsupported role, role/Node substitution, State successor,
  withdrawal, exhaustion, expiry, ambiguous exchange, or crash fails closed.
- Transit Grant v1 bytes/signature domain, EndpointTransitBinding v1, Route,
  Target, Descriptor v2, and C-2 public records remain byte-compatible.

## Evidence plan

### Primary sources

- ADR-0035, ADR-0039, ADR-0047, ADR-0062, and ADR-0065; inspected
  2026-08-31.
- R-118, R-128, and R-132; inspected 2026-08-31.
- `internal/route/credential`,
  `internal/endpoint/transit_credential_acquisition.go`,
  `internal/endpoint/publisher_introduction.go`, and
  `internal/node/transit_grant_admission.go`; inspected 2026-08-31.

### Experiment

No disposable experiment is required before the product decision. The
selected implementation must add canonical request vectors and one retained
process tracer proving sequential Introduction and Responder acquisition from
one issuer budget without exposing either Grant outside Endpoint.

### Failure scenarios

- An Introduction request is replayed as a Responder request under the same
  Request ID.
- A caller changes only role or Node ID while reusing the durable request.
- The issuer signs an unsupported role or a receiving Node accepts a role that
  differs from its current State duty.
- The Introduction Grant is spent and the Endpoint crashes before acquiring or
  presenting the Responder Grant.
- The issuer exhausts or withdraws between the two requests.
- Enrollment supplies a stale fixed Responder authorization after State
  successor.

## Findings

- **Sourced fact:** Transit Grant v1 already has exact `TransitRole` and
  `TransitNodeID` fields and permits only Introduction and Responder roles.
- **Sourced fact:** both receiving duties use the same current-State signer
  projection and independently compare the Grant role/Node against their own
  State assignment before durable one-use spending.
- **Measurement:** `credential.Request`, its canonical codec, `Client.Issue`,
  and `Issuer.signGrant` name `IntroductionNodeID` and hard-code
  `IntroductionRole`.
- **Measurement:** Endpoint's durable acquisition scope/state and Grant match
  likewise contain `IntroductionNodeID` and require `IntroductionRole`.
- **Measurement:** `PublisherIntroduction.Wait` requires a separate opaque
  Responder authorization and matching one-use TLS certificate before it opens
  the accepted Responder attachment.
- **Inference:** the gap is not a missing caller. It is an accepted-grammar
  mismatch that cannot be repaired by composition alone.
- **Inference:** a role-scoped request does not make the signer a Route planner:
  Endpoint selects the exact peer from current State, and the receiving duty
  rejects any Grant that does not match its own current assignment.

## Options

### 1. Version the request as one exact transit-role tuple

Replace the v1 request's implicit Introduction role and `IntroductionNodeID`
with one explicit `TransitRole` and `TransitNodeID`. Permit exactly
Introduction or Responder. Keep one Request ID, attachment, TLS-key digest,
deadline, budget unit, encrypted outcome, and Transit Grant v1. Endpoint owns
selection and sequential acquisition; the issuer signs only the supplied
closed tuple and never receives peer endpoints or Route context.

### 2. Add a distinct Responder issuance operation/profile

Keep the Introduction request unchanged and add a second fixed-purpose OHTTP
operation or State profile for Responder. This narrows each grammar but adds a
second profile/version/lifecycle and either duplicates the same online signer
authority or introduces another State-selected duty.

### 3. Provision fixed Responder inputs in enrollment-v3

Retain ADR-0039's historical fixed-grant shape and inventory a finite set of
Responder Grant/key pairs. This avoids an issuance-wire change but makes the
participant artifact carry State- and attempt-specific private material,
cannot honestly support an open-ended usable-alpha lifecycle, and creates a
new replenishment ceremony.

### 4. Remove fresh Publisher acceptance

Keep the candidate User-only or fixture-bound. This preserves every current
protocol but contradicts the accepted headless publish/open/bytes/withdraw
journey.

## Recommendation

Choose option 1. Confidence is high that it is the smallest coherent repair:
the existing Grant and receiving duties are already role-scoped, so only the
private issuance request and Endpoint durable acquisition schema need a
versioned generalization. It preserves the single purpose signer and budget,
adds no new authority, and exposes no new Service or Route information.

The strongest argument against it is purpose broadening: compromise of the
one online signer can spend its remaining common budget on either transit
role. That is bounded by the existing State duty and receiving-Node checks but
couples Introduction and Responder availability. Option 2 is preferable only
if the Product Owner requires separate budgets or separately withdrawable
signer duties.

## Exact Product Owner decision requested

> **R-133:** Generalize the versioned, target-free Transit Grant issuance
> request from the fixed `IntroductionRole + IntroductionNodeID` tuple to one
> exact `TransitRole + TransitNodeID` tuple, permitting only the already
> accepted Introduction and Responder roles. Endpoint derives both facts from
> current authenticated State and owns a separate at-most-once request/key
> lifecycle for each adjacent attachment. The existing purpose-scoped signer,
> common finite duty budget, fixed encrypted outcomes, Transit Grant v1,
> receiving-Node checks, Route, Target, Descriptor v2, and C-2 semantics remain
> unchanged. No caller receives a Grant, key, peer, role, or Route plan.

Acceptance would authorize a versioned request/acquisition implementation and
the complete Publisher Responder path. Rejection requires selecting a separate
Responder issuer/profile or removing fresh Publisher acceptance from this
candidate.

## Disposition

R-133 is open and decision-ready. It records a newly proven dependency of the
R-128/R-132 composition; it does not accept a protocol change or authorize an
ADR. Independent artifact/custody and local-interface work continues while the
decision remains open.
