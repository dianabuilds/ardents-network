---
id: R-145
title: Offline enrollment owner contracts and command order
status: decided; governed by ADR-0072
owner: Product Owner and Codex
started: 2026-09-05
---

# R-145 — Offline enrollment owner contracts

## Decision this unlocks

Determine the smallest set of owner contracts, durable records, and
operator-visible commands that can implement accepted ADR-0071. The result
must make one separate C0 implementation issue selectable without collapsing
recipient enrollment, challenge-bound time, Entry admission, Source client
authorization, State, Route, or Service Custody into a generic coordinator.
It does not authorize code, a package, a wire grammar, a public service, a
second active C0 implementation issue, or an automatic credential operation.

## Current contract

ADR-0071 selects an offline recipient-enrollment issuer and one exact sealed
response, plus a purpose-separated offline time witness. Existing
`internal/enrollment` verifies a static artifact only. `internal/entry` owns
State-referenced Invite validation and its replay ledger; `internal/network/source`
owns client TLS authorization; `internal/network/state` owns State and Time
Confidence; `internal/route` owns carrier and attachment behavior; and
`internal/custody` owns interactive Service Authority issuance. C0-05 still
requires a no-authored-JSON User/Publisher journey, while C0 WIP permits one
implementation issue and one selected research question.

## Hypotheses

- **H1:** A narrow offline enrollment owner can durably bind an approved public
  request to one response and coordinate only signed receipts from Entry,
  Source, and time owners; each owner remains the sole issuer/verifier of its
  own capability.
- **H2:** One Endpoint-composition owner can construct and apply the entire
  response without a new durable enrollment owner.
- **H3:** A project Node or existing Custody/State owner can be extended to
  issue the response while retaining the existing trust boundaries.
- **H0:** No bounded split can satisfy the C0 journey without an online
  responder or a broader product/operational decision.

## Evaluation criteria

- First binding uses an independently approved exact request digest; no
  first-presenter, public-artifact, package pin, or generic network connection
  becomes enrollment entitlement.
- Each private key has one purpose and one owner. The enrollment owner cannot
  sign State, Entry, Source, Route, Release, Transit Grant, or Service
  Credential facts; it neither learns a Target nor selects a Route.
- Challenge-bound time is accepted only inside a bounded monotonic request
  window, has a durable replay/floor rule, and fails closed after restart or
  interrupted delivery.
- A copied response or Invite cannot authorize another recipient key; a full
  private-root clone remains an explicit Endpoint-compromise limitation.
- Source authorization preserves CA, hostname, leaf-key pin, expiry, and
  controlled-restart gates. Entry preserves State-currentness, two-slot
  lineage, attachment replay, and no old-bearer fallback.
- Every issuer/receiver journal has bounded storage, exact retry semantics,
  crash ordering, withdrawal/revocation behavior, and no empty-root reset.
- Commands use derived local paths and typed inputs/outputs; no user edits
  JSON, provides secret material, approves a hidden fallback, or automates the
  Authority password.
- The result can be exercised by the maintained command/test profiles and one
  Product Owner without a standing responder or availability/anonymity claim.

## Evidence plan

### Primary sources

- ADR-0071, product scope, threat model, package map, testing owner, and the
  current enrollment, State, Entry, Source, Route, Endpoint, Node, and Custody
  owners, accessed 2026-09-05.
- R-144, ADR-0025, ADR-0027, ADR-0053, ADR-0062, and ADR-0064 for named
  provenance only, accessed 2026-09-05.
- The primary time and proof-of-possession specifications retained by R-144,
  with access dates, before a format is selected.

### Experiment

Use only disposable external diagnostics and test keys. Before selecting an
interface, trace an exact request through every existing owner seam and prove
that malformed, copied, replayed, delayed, expired, interrupted, or
post-restart input fails before Route work. Do not create a production
enrollment issuer, Source certificate, Entry Invite, or time witness for this
research.

### Failure scenarios

- first-presenter substitution, recipient-key swap, and copied complete root;
- issuer/Endpoint/Entry/Source/time-journal crash, reopen, rollback, conflict,
  or capacity exhaustion;
- delayed time reply, restart before reply, stale reply, and time-witness key
  compromise;
- Entry/Source issuance disagreement, partial response, withdrawal race, and
  expiry before delivery;
- malformed/oversized/unknown-version inputs, missing receipt, and old bearer
  Invite fallback; and
- lost keys, issuer root, Source policy root, or Entry admission root.

## Findings

- **Current-code fact (2026-09-05):** `internal/entry.Issue` takes a raw
  Ed25519 private key and produces a v1 bearer Invite. It retains neither the
  signer nor an issuance journal, and its signed body has no recipient key.
  `entry.Admitter` persists attachment-ID replay but deliberately accepts the
  same Invite under a distinct attachment/client-key tuple. This is a useful
  low-level construction seam, not a supported Entry-issuer custody or
  recipient-admission contract.
- **Current-code fact (2026-09-05):** `internal/network/source.Config` takes
  a complete client TLS certificate/key and a server's explicit client-CA and
  one-to-three leaf-key-pin policy. `source.New` copies and validates that
  static plan; it exposes no authorization issuer, policy mutation receipt, or
  hot-reload operation. A new Source authorization therefore needs its own
  owner-approved policy-apply/restart contract and cannot be inferred from a
  Source-plan loader.
- **Current-code fact (2026-09-05):** the only maintained headless time gate
  is `freshOperatorRegularFile`, which tests whether an operator-owned regular
  file was recently modified. Both headless runtime and Entry import consume
  it as a boolean. It contains no signed witness, request nonce, monotonic
  request window, durable replay floor, or restart rule; treating it as the
  ADR-0071 witness would silently weaken Time Confidence.
- **Current-code fact (2026-09-05):** `internal/network/state` already
  persists `trustedTimeFloor` and refuses an observation more than two seconds
  from its local clock; it also prevents backward progress within one process
  with an in-memory monotonic anchor. Its live observation port, however, is
  only `func() time.Time` (or a file modification time). It carries no witness
  signer, challenge, signed response, response interval, delivery delay, or
  durable request/replay evidence, and its monotonic anchor is recreated by
  `Open`. Feeding a witness timestamp through that port would discard exactly
  the ADR-0071 facts needed after restart. State is therefore the existing
  floor owner, but needs a typed, durable witness-verification contract before
  it can be the time receiver.
- **Current-code fact (2026-09-05):** `cmd/ardents` still reads complete
  `ardents-headless-runtime-v1`, source, and Entry-import JSON plans. Its
  runtime adapter opens the State/Entry/Endpoint owners but does not create a
  recipient enrollment identity or derive local runtime facts. The current
  command shape therefore cannot satisfy the C0 no-authored-JSON journey by
  merely adding another field.
- **Current-code fact (2026-09-05):** the C0 product contract selects exactly
  four non-interchangeable command roles. `ardents` owns the receiving Endpoint
  lifecycle; `ardents-node` owns project Node/Transit issuer operation;
  `ardents-custody` owns Service Authority custody; and `ardents-control` is a
  verifier/acceptor. None owns an offline Endpoint-enrollment issuer, a time
  witness, or a Source CA/pin-policy operator. Adding the issuer as an
  `ardents` subcommand would make the recipient runtime retain a distinct
  approval journal; adding it to Node, Custody, or Control would similarly
  violate their accepted ownership. The prepared R-144 design's separate
  `ardents-enrollment` command is therefore a product/artifact-inventory
  decision, not an implementation detail.
- **Current-code fact (2026-09-05):** Source's serving boundary accepts only
  a complete server certificate, a client-CA root, and one to three client
  leaf-key digests. Its client side likewise requires a complete client
  certificate and two independently declared server CA/hostname/pin entries.
  No maintained owner has a Source client-certificate issuance, authorization,
  withdrawal, or policy-apply API. An enrollment response may carry an
  already-issued public certificate and bounded public policy, but it cannot
  make the enrollment key an accepted Source CA or pin.
- **Current-code fact (2026-09-05):** production Source TLS construction has
  no time port. `fetch` uses `time.Now` for deadlines and leaves `tls.Config.Time`
  unset, so X.509 validity uses the process wall clock. The new TLS behavior
  tests set the private client/server configs' `Time` only in their fixture;
  that is useful proof of CA/hostname/pin/expiry behavior, not a deployed
  Time Confidence integration. A witness cannot safely bootstrap Source until
  State supplies Source an already-validated bounded verification clock without
  allowing Source to decide time confidence or import State.
- **Current-code fact (2026-09-05):** the v1 EntryBinding has only the
  bearer Invite, attachment ID, and fresh client-TLS-key digest. Route
  validates and consumes it before Route allocation, but it has neither an
  Entry-recipient public key nor a channel-proof field. `internal/node`
  adapts the current Entry admittance transaction; the current Node Config
  retains the candidate identity signer but exposes no supported Invite-issue
  operation. A recipient-bound successor necessarily changes coordinated
  Entry, Route, and receiving-Node contracts; appending an unsigned wrapper
  at Endpoint would not satisfy ADR-0071.
- **Current-code fact (2026-09-05):** ADR-0026 fixes one
  `ardents-interactive-route-v1` magic/ALPN/profile and explicitly requires a
  separate accepted record for a future overlapping generation. Both the
  Invite and EntryBinding bodies declare version `1`; a recipient proof cannot
  be added without changing their canonical signed/transport bytes. The
  successor therefore needs a State-selected Route/Entry profile and a
  no-downgrade v2 reader, not an implicit v1 extension or peer negotiation.
- **Inference:** the proposed time receiver cannot be represented as merely a
  new scalar `trustedNow` callback. A witness proves an interval, not one
  instant: accepting `NotBefore` safely needs the interval's earliest bound,
  while accepting expiry safely needs its latest bound. The current scalar
  distribution floor is still useful anti-rollback state, but a new bounded
  interval/elapsed-time model and Source TLS verification-clock port must be
  specified before implementation; otherwise one side of certificate or State
  validity would be silently weakened.
- **Inference:** H2 and H3 fail the current authority split. Giving Endpoint
  the combined issuer would make a composition owner retain Entry/Source/time
  issuance state; giving State, Node, or Custody that response issuer would
  add enrollment authority to an accepted distinct root. H1 remains plausible
  only as a dedicated offline owner with receipt-only ports; its exact ports,
  artifact/command route, and versioned recipient-bound Entry format remain
  unselected.

R-144 supplies the copied-capability evidence and ADR-0071 selects the
high-level trust direction. This question must now identify the minimal
receipt-only owner contracts and their crash/withdrawal ordering before an
implementation issue is selected.

## Options

1. A dedicated, offline enrollment owner with receipt-only ports to the
   existing capability owners.
2. Endpoint composition owns the transaction and imports every capability
   owner directly.
3. Extend an existing State, Node, or Custody owner.
4. Retain C0-05 blocked if no bounded contract survives the criteria.

## Recommendation

The evidence rejects a single "enroll and start" implementation issue. Before
one can be selected, prepare three bounded follow-on owner decisions in this
order:

1. **Time receiver:** State verifies and durably floors a purpose-separated
   witness receipt. The receipt must retain the witness-key identity, request
   challenge, signed interval, locally measured request-window/delivery facts,
   and exact replay identity; `trustedTimeFloor` remains State-owned. Endpoint
   may prepare and pass a bounded opaque receipt but may not decide time
   confidence. Source receives only State's bounded verification-time callback
   for TLS certificate checks; it cannot use the system clock as an alternate
   trust source. Restart without a still-valid local request window requires a
   fresh request. This is a consequential Time Confidence decision, not a
   replacement of the existing `ClockObservation` test seam.
2. **Recipient Entry authorization:** the State-selected Initiator signer
   issues a new, versioned Invite whose signed body commits to the Entry
   recipient public key and finite generation. Entry and Node retain State
   currentness, admission capacity, and durable replay; Route carries and
   verifies a channel-bound possession proof before allocation. No v1 bearer
   fallback, peer-negotiated version, or Endpoint-side wrapper is permitted.
   This is a new State-selected Route/Entry profile, with new magic/ALPN and
   canonical v2 records; it cannot alter the closed v1 bytes in place.
3. **Offline enrollment and Source authorization transaction:** a dedicated
   offline enrollment root commits the independently approved full request
   digest and one sealed response. It records only exact component receipts;
   it never signs Entry or Source facts. The existing project-controlled Entry
   signer and the separate Source CA/pin-policy operator each need their own
   explicit approve/issue-or-apply/withdraw receipt and restart rule. Only
   after all component receipts are durable may the enrollment owner commit
   immutable response bytes. The Endpoint stages and validates the response,
   then derives its private paths and runtime plan without operator-authored
   JSON.

The resulting operator order is: verify artifact; Endpoint prepares one
request plus time challenge; Product Owner confirms the exact request digest;
time witness and the existing Entry/Source authorities issue their separate
receipts; enrollment issuer commits and seals the exact response; Endpoint
accepts/stages it; then the existing Service Instance/Custody and Target-Link
journey proceeds. Each retry reconciles the same request/receipt digest;
different input is a conflict. This is a decision proposal, not a command
addition or implementation authorization.

The Product Owner must expressly decide whether C0-05 may introduce that new
Route/Entry profile. If the current no-wire-change boundary remains in force,
H1 cannot meet its recipient-copying criterion and C0-05 must remain blocked;
there is no safe implementation-only substitute.

The Product Owner must also decide whether C0 admits a fifth, offline-only
`ardents-enrollment` artifact or retains the four-command inventory. Retaining
four commands leaves no owner for H1's approval journal and sealed response;
reusing a current command would contradict its explicit owner boundary.

## Disposition

Decided on 2026-09-05 and promoted to ADR-0072. The Product Owner authorized
the Route/Entry v2 profile and the separate `ardents-enrollment` artifact.
The current-code seam evidence supports H1 only with the three owner decisions
above. Exact grammar, package, command, artifact, and implementation changes
belong to selected C0-05; no dependency was selected here.
