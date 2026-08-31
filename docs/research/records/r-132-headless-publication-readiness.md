---
id: R-132
title: Which authenticated readiness transition may commit one headless Publisher generation?
status: decided
owner: Product Owner and Codex
started: 2026-08-31
reviewed: 2026-08-31
---

# R-132 — Headless Publisher publication readiness

## Decision this unlocks

Select the exact Endpoint-owned transition that may turn one accepted
Service Instance binding into a discoverable live publication and retained C-2
Introduction slot without an operator-supplied Route plan, Target, raw key, or
hidden publication authority.

This decision blocks only the maintained headless Publisher start path and the
complete B6 artifact journey. Service Authority/Instance acquisition,
non-exporting signing and Introduction opening, the User-side headless
Connection Interface, Browser separation, and the four-binary enrollment-v3
artifact boundary can proceed independently.

## Current contract

- ADR-0035 requires the Publisher to retain one TLS-authenticated,
  State-selected live Introduction slot and treats
  `IntroductionSlotReady` as the exact confirmation of that retained slot.
- The retired lower-level `Endpoint.Publish` path requires an `ARIA`
  acknowledgement signed by the public key supplied separately as
  `IntroductionPublic`. Its owner-local Unix client and `IntroductionSocket`
  field are no longer production code; the maintained combined Publisher
  start commits the native slot-ready transcript.
- R-105 records that the historical Unix operation was narrow publication
  control, not a C-2 channel. No maintained Node duty, enrolled command, or
  State projection owns such a socket or supplies its signing key.
- R-129 forbids an operator Route plan in the supported headless journey.
  R-131 and ADR-0064 forbid raw Credential, Instance-key, Target, or Service
  Authority input in the maintained runtime plan.
- The immutable public Publication record contains only the SHA-256 digest of
  the acknowledgement under the Instance signature. A User verifies the
  Credential and Instance signature but cannot inspect or authenticate the
  omitted acknowledgement bytes.
- `internal/service/publication` advances its durable generation floor before
  reporting a local publication. The host Instance root treats a generation at
  or below that floor as successor-required after restart.
- Transit Grant v1, Credential v2, SealedIntroduction v1,
  EndpointTransitBinding v1, Introduction slot records, Route, and Target
  semantics are fixed and are not reopened by this question.

There is therefore no honest owner for the current `ARIA` socket/key in the
headless product. Supplying it from a fixture or runtime plan would recreate
the exact hidden authority and operator-route dependency that R-128/R-129
remove.

## Hypotheses

- **H1:** Endpoint can stage the local Instance-signed Publication, register
  the already State-selected TLS slot, and expose the record only after exact
  `IntroductionSlotReady`, using that verified local transcript as the
  publication acknowledgement commitment without changing any public wire
  grammar.
- **H2:** The Introduction Node must provide a new portable signature over the
  publication facts before Endpoint may commit the generation, requiring a
  versioned Node wire change.
- **H3:** The current owner-local `ARIA` signer can be retained only if an
  already accepted product owner and key-provisioning path can be identified.
- **H0:** None preserves current authority boundaries; fresh Publisher start
  must be removed from the candidate.

## Evaluation criteria

- A Publisher invokes only the local Service Administration surface after
  Instance acceptance; it supplies no Target, Credential file, key file,
  Introduction socket, Node identity, endpoint, Route plan, or grant.
- Endpoint derives every Node/profile fact from current authenticated State
  and every one-use TLS identity from its acquisition owner.
- The operation reports `published` only after one exact live slot is retained
  for the same Network, epoch/digest, Introduction identity, reachability,
  JoinHandle, and deadline.
- Cancellation, slot refusal, crash before readiness, ambiguous durable state,
  and withdrawal cannot leave a reusable Instance generation or a
  discoverable record without a live slot.
- No Route Node learns Target, Credential, Instance material, Authority, or a
  complete Route; no Browser or enrollment artifact receives runtime
  authority.
- Existing public record, Credential, Route, Target, and C-2 wire readers
  remain byte-compatible unless the Product Owner explicitly chooses the
  versioned-wire option.
- The design is testable from unpacked enrollment-v3 artifacts with no fixture
  command and fits the one-human-plus-Codex operating model.

## Evidence plan

### Primary sources

- ADR-0034, ADR-0035, ADR-0062, ADR-0063, and ADR-0064, inspected
  2026-08-31.
- R-105, R-128, R-129, R-130, and R-131, inspected 2026-08-31.
- `internal/endpoint/service_publication.go`,
  `internal/endpoint/service_introduction.go`,
  `internal/endpoint/publisher_introduction.go`,
  `internal/service/publication/publication.go`,
  `internal/service/instance`, `internal/node/introduction_listener.go`,
  and the canonical Introduction codecs, inspected 2026-08-31.

### Experiment

No disposable experiment is needed before the decision. A test-first retained
tracer for the selected option must prove:

1. no record is made discoverable before exact slot readiness;
2. slot failure and cancellation leave the generation unavailable and require
   a successor after any committed publication floor;
3. crash at every boundary between local publication, slot registration,
   readiness, exposure, and Instance commit fails closed;
4. exact retry never creates a second live slot or second publication;
5. withdrawal prevents acquisition, drains work, closes the slot, erases the
   binding, and makes restart successor-required; and
6. the lower-level legacy `ARIA` path remains compatibility evidence only.

### Failure scenarios

- A runtime plan substitutes the Introduction identity, endpoint, key, socket,
  or acknowledgement.
- State changes between selection, TLS admission, and slot readiness.
- The Node acknowledges a different reachability, JoinHandle, or expiry.
- Endpoint crashes after the publication floor advances but before slot
  readiness or record exposure.
- A local caller repeats publish while a generation or slot is pending/live.
- A malicious local peer sends an `ARIA` receipt to the maintained headless
  administration socket.
- Withdrawal races an Introduction delivery or retained Service Connection.

## Findings

- **Sourced fact:** the current `ARIA` receipt is not part of any maintained
  Node duty or enrollment inventory.
- **Sourced fact:** ADR-0035 already selects mutually authenticated transit
  admission plus exact `IntroductionSlotReady` as the live-slot proof.
- **Sourced fact:** the public Publication record commits only an opaque digest
  of acknowledgement bytes; consumers have no receipt-verification operation.
- **Measurement:** current headless runtime configuration has no Publisher
  Instance root, publication root, Service Administration owner, or
  State-derived Publisher slot composition.
- **Measurement:** commits `58bab5a6` and `34e757eb` let Endpoint consume the
  accepted host binding as an opaque Instance signer and fixed-purpose
  SealedIntroduction recipient without private-byte export.
- **Measurement:** commit `ed13b09d` makes the real Endpoint, Node, control,
  and Custody binaries independently manifest-pinned in the headless
  enrollment-v3 candidate while preserving ADR-0042 v3 compatibility.
- **Inference:** inventing an owner-local signer or accepting
  `IntroductionSocket` in the headless plan would be a hidden authority, not a
  harmless Adapter detail.
- **Inference:** a combined Endpoint-owned local-publication/slot transaction
  can preserve all public wire bytes because the existing acknowledgement
  digest has no external verifier semantics.

## Options

### 1. Endpoint-owned combined publication and slot transaction

Endpoint consumes one Service Administration capability, opens the accepted
Instance binding, creates the local Instance-signed Publication and advances
its floor, registers the State-selected slot with acquired one-use transport
inputs, and makes the record discoverable only after exact
`IntroductionSlotReady`. The acknowledgement commitment is the canonical
verified slot-ready transcript plus its bound registration context. Any
failure withdraws the local publication and terminally closes the generation;
a crash after floor advancement requires a successor.

This changes maintained local readiness ownership but changes no Credential,
Publication record size, Route, Target, or C-2 wire grammar. The old
`IntroductionSocket` client path and the compiled raw `Endpoint.Publish` API are
retired. Historical ARIA receipt material remains only in `referencec2`-tagged
source evidence.

### 2. Versioned signed slot-ready wire

Add a Node signature over the Credential/publication attempt facts to a new
slot-ready record and bind its digest into Publication. This gives a portable
receipt but changes the selected Node wire, requires key-purpose review and
compatibility handling, and does not add a consumer who can validate the
receipt. It is disproportionate unless an external receipt claim is selected.

### 3. Retain the owner-local ARIA signer

Define and distribute another signer beside Endpoint. No accepted authority,
key lifecycle, enrollment boundary, or runtime owner exists for it. This
duplicates live-slot trust and violates the no-hidden-organization/product
capacity constraints.

### 4. Remove fresh Publisher start

Ship a User-only candidate and retain Publisher fixtures. This is internally
consistent but contradicts the accepted publish/open/withdraw usable-alpha
journey and requires an explicit scope reduction.

## Recommendation

Choose option 1. Confidence is high that it is the smallest construction
consistent with ADR-0035 and the accepted headless boundary: it uses the
already authenticated live-slot outcome as local readiness evidence, keeps
record exposure behind readiness, and needs no new wire, signer, key, or
operator input.

The strongest argument against it is crash complexity. Publication-floor,
slot, record-exposure, and Instance-state transitions become one recoverable
local transaction even though their underlying owners remain separate. The
retained implementation therefore must test every interleaving listed above
and must require a successor rather than attempt rollback after an ambiguous
commit.

## Exact Product Owner decision requested

The Product Owner accepted this statement on 2026-08-31. ADR-0065 fixes the
selected readiness, failure, restart, and compatibility boundaries.

> **R-132:** For the headless usable-alpha Publisher, select one
> Endpoint-owned publication transaction. Endpoint may advance the local
> Publication floor using the opened non-exporting Instance binding, but it
> exposes the record and reports `published` only after the exact
> State-selected TLS slot returns matching `IntroductionSlotReady`. That
> verified registration/ready transcript is the local acknowledgement
> commitment; no new public wire field or portable Node signature is claimed.
> Failure or ambiguity terminally closes the generation, and restart requires
> a successor. The legacy owner-local `ARIA` socket remains lower-level
> compatibility evidence and is absent from the maintained headless runtime.

Acceptance authorizes a test-first combined Publisher start/withdraw/recovery
slice and the artifact-native B6 journey. A future reversal must select the
versioned-wire option or explicitly remove fresh Publisher start from the
candidate.

## Disposition

R-132 is **decided and promoted to ADR-0065**. The accepted implementation must
exercise the combined transaction and fail-closed recovery from unpacked
headless artifacts before B6 closes. No public wire, Route, Target, or C-2
semantics are changed. The transparent-origin Browser Entry defect remains a
separate future security dependency.
