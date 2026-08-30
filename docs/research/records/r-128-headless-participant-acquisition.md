---
id: R-128
title: Headless participant current-input acquisition owner
status: open
owner: Product Owner and Codex
started: 2026-08-30
reviewed: 2026-08-30
---

# R-128 — Which enrolled owner supplies current headless participant inputs?

## Decision this unlocks

Select the smallest maintained owner and lifecycle that let one enrolled
headless Endpoint acquire current State, an Entry, and one-use transport inputs,
then publish, open, and withdraw through existing Network behavior without an
operator route plan or Browser ownership.

Until this question is decided, implementation must not invent a new authority,
distribution service, long-lived credential, route planner, wire message, Route
semantic, or Target semantic. Work independent of participant acquisition, such
as the headless command and artifact evidence profile, remains permitted.

## Current contract

The product boundary is
[Network/Application separation](../../product/network-application-separation.md).
The maintained Network remains standalone; Browser and Desktop are replaceable
Application Adapters. [R-106](r-106-target-reachability-alpha.md) retains the
bounded reachability tracer but leaves first-run acquisition and normal Endpoint
composition open. [R-118](r-118-participant-transit-credential-lifecycle.md) and
[ADR-0047](../../adr/0047-dynamic-membership-transit-grants.md) select a
State-declared Credential Relay shape, not its complete participant operation.

Already fixed: current State authorizes topology; Entry is selected from that
State; Transit Grants are finite and one-use; payload encryption is not an
anonymity claim; no Node, Gateway, Browser, or operator may silently choose a
fallback route. Still open: the durable issuance budget and erasure owner,
pending-grant crash retention, withdrawal and rotation behavior, enrolled input
distribution, and the participant-facing recovery/exhaustion lifecycle.

The relevant security owners are the
[threat-and-response matrix](../../security/threat-model.md#threat-and-response-matrix)
for malicious infrastructure, Sybil/flooding, operator loss, supply chain, and
governance capture, plus the
[security invariants](../../security/threat-model.md#security-invariants) for
State, Entry, Route, credential, and fail-closed behavior. Product terms keep
their canonical meanings from the [glossary](../../../CONTEXT.md): Endpoint,
Endpoint Owner, Entry Set, Service Target, Application Interface, and Capability
Readiness are not interchangeable ownership or authority boundaries.

The separately reported transparent-origin Browser Entry defect remains a
dependency of a future Browser security slice and is outside this question.

## Hypotheses

- **H1:** One enrolled Endpoint-owned composition of already selected State,
  Entry, Credential Relay, and one-use-spend owners can close the journey without
  adding an authority or changing public wire, Route, or Target semantics.
- **H2:** A distinct participant acquisition Adapter is required, but it can be
  limited to distributing already-authorized inputs and cannot choose routes or
  issue independent authority.
- **H0:** Neither option can satisfy crash recovery, finite issuance, withdrawal,
  and fail-closed exhaustion within the current product and authority contract.

## Evaluation criteria

- A fresh enrolled CLI participant can obtain authenticated current State and
  Entry facts, acquire exactly the required finite transport input, publish or
  open, withdraw, restart, and observe exhausted or withdrawn inputs fail closed.
- The owner never learns or receives a Target unless the already accepted flow
  requires it; a Credential Relay cannot plan a Route or mint State authority.
- A crash between acquisition and spend has one durable, bounded outcome and
  cannot duplicate use, create an ambient credential, or silently consume an
  unbounded grant budget.
- State successor, withdrawal, credential expiry, relay unavailability, stale
  input, and local state loss have explicit recovery or terminal behavior.
- Latency is bounded by the earliest current-State, Entry-admission, request,
  or Grant deadline; a timeout returns explicit unavailability and starts no
  fallback. Acquisition adds no periodic retry after that deadline.
- Bandwidth adds at most one bounded request and response for each required
  one-use input and no traffic proportional to Application bytes. A candidate
  without exact message-size limits is rejected before implementation.
- Durable storage adds at most one finite pending reservation per in-flight
  acquisition plus the already owned current inputs; it retains no unbounded
  request history. Crash recovery must converge that reservation to one usable
  spend or one terminal erasure, never both.
- Availability is no stronger than current authenticated State, one admitted
  Entry path, and the State-selected relay. Missing, withdrawn, saturated, or
  expired inputs produce a visible unavailable result; there is no service,
  clearnet, stale-state, or operator-plan fallback and no public availability
  claim.
- The design fits one Product Owner and Codex, uses maintained components, and
  adds no standing operator procedure that the actual team cannot sustain.
- Distribution must remain inside the authenticated headless alpha profile and
  must not add an unsigned side channel, Browser artifact, separate participant
  secret delivery, or new license obligation. Any new runtime dependency is
  rejected unless its license, distribution, maintenance, and threat-model fit
  are first recorded in `docs/development/dependencies.md`.
- The supported developer and accessibility surface is the existing headless
  CLI: one documented non-interactive journey, stable exit status, and a
  specific actionable unavailable/withdrawn/exhausted diagnostic. No GUI,
  pointer interaction, visual-only state, manual per-request operator approval,
  or source-code fixture is required.
- The implementation adds no speculative package or Interface and changes no
  public wire, Route, or Target semantics without separate accepted research and
  an ADR where the choice is consequential and hard to reverse.

Falsification is defined before an experiment: reject H1 or H2 if any tested
crash/restart can reuse a one-use input, if any component outside current State
can select the next Route position, if withdrawal can still produce a usable
fresh input, if operation requires Browser state, or if a required authority or
operator is absent from the accepted contract.

## Evidence plan

### Primary sources

- Current product contract, threat model, R-106, R-118, ADR-0039, ADR-0047,
  endpoint runtime owner, and package map; accessed 2026-08-30.
- Maintained source and behavior tests for State projection, Entry selection,
  Credential Relay, Transit Grant spending, Service publication/opening, and
  withdrawal; accessed 2026-08-30.

### Experiment

After the Product Owner accepts one owner/lifecycle, create one local,
deterministic process experiment using fresh protected state. Record exact
inputs and exercise enroll, acquire, publish/open, withdraw, restart, successor,
relay-unavailable, and crash-between-acquire-and-spend cases. Do not run VPS,
soak, hostile-load, platform-matrix, or release qualification during this
decision slice.

### Failure scenarios

- malicious or stale relay response;
- current-State successor during acquisition;
- crash before durable reservation, after reservation, and after spend;
- withdrawal racing an open or publish attempt;
- exhausted issuance budget and unavailable Credential Relay;
- replayed Entry or transport input;
- local protected-state loss or replacement; and
- project operator unavailable or governance key compromised.

## Findings

- **Sourced fact:** R-106 leaves participant first-run/acquisition, normal
  Endpoint composition, and user-facing `open` unresolved.
- **Sourced fact:** R-118 and ADR-0047 select the Credential Relay declaration
  and target-free issuance shape, while durable budget/erasure, withdrawal,
  crash treatment, and participant lifecycle remain delivery gates.
- **Sourced fact:** The maintained `endpoint run` process accepts an operator
  plan; that is behavior evidence, not an honest participant acquisition owner.
- **Inference:** Implementing the complete participant journey now would require
  choosing at least one owner or lifecycle rule not established by accepted
  product, research, or ADR material.
- **Current-code fact (2026-08-30):** the maintained
  `credential.Issuer` accepts the raw Ed25519 `GrantSigner`, keeps it in the
  online issuer process, and signs every accepted Transit Grant with it. The
  Endpoint and receiving Node accept that signer only when its public-key
  identifier is one of the current Network State authorities. This is not a
  separately scoped transit-issuance key.
- **Accepted-contract fact (2026-08-30):** R-121 and ADR-0053 make the
  functional-alpha Network State authority a separate encrypted 1-of-1
  Product Owner-held seed. Its private key may not enter a VPS, and its Module
  exposes neither the seed nor an arbitrary signing capability. ADR-0047's
  online State-authority signing operation therefore cannot be composed with
  the accepted functional-alpha custody contract.
- **Current-code fact (2026-08-30):** `ardents-node` does not start a
  `transit-issuance` duty. Maintained production code has no caller of
  `credential.NewIssuer`; current callers are behavior tests with temporary
  keys. The conflict is consequently a design and operations blocker, not
  evidence that a real State authority key has already been placed on a VPS.
- **Design comparison (2026-08-30):** returning a Grant, TLS key, State view,
  Entry contact, Route, or issuer fact through a CLI-facing Interface would
  spread crash, spend, and erasure ordering across callers. The smallest deep
  Module keeps acquisition inside the Endpoint. It retains at most one
  non-secret durable pending marker, keeps the one-use private key volatile,
  and treats every crash after a possible issuance as terminal at-most-once
  loss with no automatic retry or Application-operation replay.
- **Ownership conclusion (2026-08-30):** the signer-side
  `transit-issuance` duty is the only honest owner of a durable finite issuance
  budget. It must consume one exact State-duty-scoped unit before signing;
  ambiguous failure burns that unit. Endpoint, Entry, Initiator, Browser, CLI,
  and the relay Adapter cannot enforce an issuer-wide budget.
- **Protocol consequence (2026-08-30):** the current issuer collapses budget
  exhaustion, withdrawal, and ordinary unavailability into one response. An
  honest remote `exhausted` diagnostic requires a fixed-size encrypted,
  versioned credential outcome. Without that separately accepted protocol
  change the Endpoint may report only generic credential unavailability.

## Options

### Endpoint-owned composition

The enrolled Endpoint composes existing authenticated State/source, Entry, and
Credential Relay owners and durably owns finite reservations and spends. This
has the smallest product surface, but is rejected unless one crash-safe budget
and withdrawal lifecycle can be specified without concentrating new authority.

### Distribution-only acquisition Adapter

A replaceable headless Adapter distributes already-authorized inputs to the
Endpoint while remaining unable to select a Route or issue authority. This may
isolate deployment concerns, but is rejected if it becomes a hidden online
registrar, route planner, required project service, or duplicate state owner.

### Keep the journey blocked

Retain current tracers and the independent headless artifact boundary. This is
the correct option if neither operational contract can be maintained by the
actual Product Owner-and-Codex team.

## Recommendation

Do not deploy or implement the current dynamic issuer with the functional-alpha
Network State authority key. That would directly contradict R-121 and ADR-0053
and would give one online process an unscoped copy of the sole 1-of-1 State
signing key.

The proposed participant lifecycle is Endpoint-owned and at-most-once:

1. the Endpoint derives Capability Readiness from its authenticated enrollment,
   current State, and the capability's distinct Entry Set;
2. one operation creates a volatile key and one durable non-secret pending
   marker before the Credential Relay exchange;
3. a successful exact Grant is consumed immediately by the same Endpoint-owned
   operation and erased on every terminal outcome;
4. a crash, State successor, withdrawal, or ambiguous response burns the pending
   attempt and never resumes or retries the Application operation; and
5. the signer-side duty atomically consumes its separate finite durable budget
   before signing.

Before implementation, the Product Owner must choose whether to authorize a
new bounded decision for a State-authenticated, purpose-scoped online Transit
Grant signer and fixed encrypted issuance outcomes. That successor may preserve
the Transit Grant, Route, and Target grammars, but it must not reuse a Network
State root key and any State/profile change requires its own research and ADR.
If that scoped delegation is rejected, dynamic participant acquisition remains
blocked and the usable-alpha product scope must narrow.

## Disposition

R-128 is open and blocks only participant-acquisition implementation. No ADR or
experiment is created by this record. The first independent headless
build/test/artifact slice remains valid. The next decision slice must either
select a bounded owner/lifecycle and promote it to current product/technical
owners (and an ADR if required), or retain the journey as blocked and select a
different usable-alpha scope.
