---
id: R-155
title: Which old starts retire while owned shutdown remains available?
status: completed
owner: Product Owner and Codex
started: 2026-09-22
reviewed: 2026-09-22
---

# R-155 — Which old starts retire while owned shutdown remains available?

## Decision this unlocks

Decide the bounded operational disposition required by issue #144 before the
separate command gates #100, #118, #119, #155, and #158 may change runtime.
The decision must distinguish starting old network work from authenticating,
stopping, withdrawing, diagnosing, or removing one already owned Contributor
installation.

## Current contract

The [C0 product scope](../../product/scope.md) selects the protected closed
Route duties as the maintained candidate and does not make historical planning
labels into runtime authority. The [threat model](../../security/threat-model.md)
requires finite work, explicit retirement, retained owner control, and no
silent fallback. The current [Node owner](../../technical/network-route-node.md),
[Contributor runbook](../../reference/rendezvous-contributor.md), and
[command reference](../../reference/commands.md) still expose both old and
closed starts. Product Owner direction in issue #140 retires old network
implementations while preserving the separately selected closed duties.

This record selects no public Node protocol, authority migration, new wire,
operator organization, profile rename, data conversion, floor reset, or code
deletion. Each accepting adapter and exclusive implementation remains a
separate GitHub slice.

## Hypotheses

- **H1:** Rejecting every new old-role, old-Source-profile, Transit-issuer, and
  Contributor apply/restart start before effects, while preserving no-start
  diagnose/drain/withdraw/remove for an authenticated owned installation,
  retires the old execution authority without orphaning owner-held resources.
- **H2:** Removing every Contributor lifecycle command together with new starts
  is safer because no old code remains operable.
- **H3:** Allowing old restart or update until a replacement migration exists is
  required to preserve owner control.
- **H0:** The current owners cannot separate new execution authority from safe
  retirement without choosing a new protocol or migration.

## Evaluation criteria

- Every named new old start has one explicit disposition and effect boundary.
- Every current closed start remains named and unchanged by the decision.
- An authenticated existing Contributor deployment has a bounded path to
  observe, stop, disable, and remove only its own files without starting or
  restarting its executable.
- Interrupted or ambiguous installation state fails explicitly; recovery does
  not manufacture permission to run old bytes.
- No profile is renamed, duty inherited, key regenerated, persisted root or
  floor reset, wire converted, or foreign installation adopted.
- The resulting implementation slices remain independently testable and fit
  the one-human/Product-Owner plus Codex operating capacity.

H1 is falsified if any preserved retirement operation inherently requires a
new old start, or if an old accepting path cannot be refused before its first
root, key, listener, supervisor, or network effect. H2 is falsified if removing
retirement control can leave a verified owned unit active or force manual
filesystem/systemd mutation. H3 is falsified if owner control can be preserved
through stop/withdraw/remove without admitting restart or update. H0 is chosen
if current ownership cannot express that split without a new authority or data
migration.

## Evidence plan

### Primary sources

- Current accepted product, security, technical, reference, and development
  owners in this repository, accessed 2026-09-22.
- Current command adapters and owner implementations under `cmd/ardents-node`,
  `internal/node`, and `internal/contributor`, inspected at the exact source
  revision recorded below.
- GitHub issues #100, #118, #119, #140, #144, #155, and #158 as the execution
  ledger, accessed 2026-09-22.

### Experiment

No runtime experiment is needed for this operational authority decision. The
reproducible check is an exact symbol/caller and state-transition inspection at
the recorded revision. Each later code slice owns its own early-refusal or
retirement-control behavior oracle.

### Failure scenarios

- An old reservation reaches a listener or opens a local root before refusal.
- An absent old Source selector silently falls back to the old profile.
- Transit issuer initialize creates a key/root or serve binds before refusal.
- `diagnose`, pre-control recovery, `drain`, `withdraw`, or `remove` starts or
  restarts an old Contributor executable.
- An interrupted update is mistaken for authority to resume old work.
- Retirement removes or rewrites current closed duty inputs, old evidence,
  foreign host files, keys, State roots, role floors, or journal history.

## Findings

The inspected source revision is
`b318efeadd19b1f2e5a89ce1033d780ae9d91c9f`.

### Accepting start surfaces

- **Sourced fact:** `cmd/ardents-node/node_config.go` accepts the old
  `rendezvous`, `initiator`, `introduction`, `responder`, and `transit_issuer`
  reservations alongside the separately named closed reservations. The old
  reservations select `route.Profile`; the closed reservations select
  `route.ClosedRouteProfile` only with the pinned closed-profile authority.
- **Sourced fact:** `internal/node/duty_server.go` dispatches the old
  `rendezvous`, `initiator`, `introduction`, `responder`, and
  `transit-issuance` assignments under `route.Profile`. Its closed-profile
  dispatch is a separate issuer, forwarding, resolution, introduction, or
  data-join path.
- **Sourced fact:** `cmd/ardents-node/config.go` accepts
  `native_rendezvous_profile` as the old Source profile selector. The selected
  closed Source path instead requires `state_profile` equal to
  `ardents-route-v3` and its separately pinned authority.
- **Sourced fact:** `cmd/ardents-node/issuer_initialize.go` accepts both the old
  `ardents-transit-issuer-initialize-v1` initialization and the closed issuer
  schema. Old initialize can create or open the issuer root and key; old serve
  is admitted by the `transit_issuer` Node plan.
- **Sourced fact:** `cmd/ardents-node/contributor_mode.go` sends `apply` to
  `Profile.Apply` and sends `restart`, `diagnose`, `drain`, `withdraw`, and
  `remove` to `Profile.Control`. `internal/contributor/apply.go` starts a first
  installation, starts an update, and can restart a prior generation during
  rollback.
- **Inference:** Refusing only old wire traffic is too late. These adapters
  grant local authority to open roots, create keys, bind listeners, invoke the
  supervisor, or begin Network work, so each old selection must be rejected
  before its first such effect.

### Existing owned installation

- **Sourced fact:** no deployment inventory, host observation, or operator
  receipt was supplied to or inspected by this decision. Whether any owned
  installation currently exists, its count and hosts, stored profile identity,
  deployment ID and generation, active/enabled/lifecycle state, interrupted-
  update state, or external bundle/journal/snapshot residue is unknown.
  Source-code states below are capabilities and failure cases, not observations
  of a real installation.
- **Sourced fact:** `internal/contributor/control.go` verifies an installation
  and implements bounded status inspection, stop-to-`WITHDRAWN`, disable, and
  exact-confirmation removal. Removal is already limited to the managed
  installation and refuses an active, enabled, non-withdrawn, or differently
  identified deployment.
- **Sourced fact:** every Control action currently runs
  `recoverInterruptedUpdate` before action dispatch.
  `internal/contributor/recovery.go` can call supervisor Start or Restart while
  recovering a previous or current generation. Therefore even `diagnose`,
  `drain`, `withdraw`, and `remove` can currently revive old executable bytes.
- **Inference:** ownership authentication and old profile recognition remain
  necessary to retire an existing installation, but they do not confer fresh
  execution authority. A retirement command may inspect, Stop, Disable, or
  remove exactly verified managed state; it may not Start, Restart, Enable, or
  finish an interrupted update by running either generation.
- **Inference:** an interrupted or ambiguous installation that cannot be
  authenticated and retired without execution must return an explicit bounded
  failure and retain its evidence. It must not adopt foreign files, rename the
  profile, choose a generation by convenience, or invite manual mutation as a
  supported path. The exact no-start recovery state machine belongs to the
  later implementation slice.

### Compatibility and retained paths

- **Sourced fact:** `internal/contributor/profile_identity.go` recognizes the
  historical `h4-5-rendezvous-alpha-v1` identity and normalizes it to
  `ardents-rendezvous-dedicated-host-v1`; the current runbook limits that
  recognition to already pinned bundles, Node plans, and installation records.
- **Inference:** both identities must be unable to authorize a new apply or
  restart. Historical identity recognition remains only for authenticating
  retained evidence and an owned retirement operation; persisted identity is
  not rewritten.
- **Sourced fact:** the current closed Node reservations are `closed_issuer`,
  `closed_forwarding`, `closed_resolution`, `closed_introduction`, and
  `closed_data_join`. Closed Source and issuer inputs have separate profile,
  authority, and schema checks.
- **Inference:** the Node-plan schema, Source server, issuer command, and probe
  behavior are not retired wholesale. The decision changes only the named old
  selectors. Current closed starts retain their existing authority checks and
  effects; an omitted or refused old selector cannot fall back to them.
- **Inference:** no new product term is introduced. `CONTEXT.md` needs no
  glossary change; this is an authority/lifecycle decision over existing
  Person-independent domain concepts.

## Options

- **H1 — refuse new starts, preserve no-start retirement.** Meets every
  criterion and keeps the smallest authority surface. It requires the later
  Contributor implementation to separate retirement inspection/recovery from
  start-capable update recovery.
- **H2 — remove every lifecycle command.** Rejected. It can strand an active
  verified owned service or force unsupported manual systemd and filesystem
  mutation, so it fails retained owner control.
- **H3 — retain restart/update until migration.** Rejected. Stop, withdraw,
  diagnose, and exact owned removal preserve owner control without granting a
  fresh old start. No replacement migration is required to retire the old
  execution authority.
- **H0 — require a new protocol or data migration.** Rejected. Existing plan
  selection, installation identity, supervisor, and exact removal boundaries
  already express the split. Ambiguous interrupted state can fail explicitly
  rather than manufacture a new authority.

## Recommendation

Select H1.

Refuse these old starts before their first root, key, listener, supervisor, or
Network effect:

- Node reservations `rendezvous`, `initiator`, `introduction`, `responder`,
  and `transit_issuer` and their old runtime assignments;
- Source `native_rendezvous_profile`;
- Transit issuer `ardents-transit-issuer-initialize-v1` and serve selected by
  the old Transit issuer reservation; and
- Contributor `apply` and `restart`, for both canonical and historical profile
  inputs.

Retain the existing closed `closed_issuer`, `closed_forwarding`,
`closed_resolution`, `closed_introduction`, and `closed_data_join` starts,
closed Source selection, and closed issuer initialize/serve without changing
their authority. Retain Contributor `diagnose`, `drain`, `withdraw`, and
confirmed `remove` only after exact owned-installation authentication and with
no Start, Restart, Enable, update completion, or fallback. Drain stops an
active owned unit and waits finitely for `WITHDRAWN`; it does not start an
inactive unit. Withdraw additionally disables it. Remove keeps the existing
exact deployment confirmation and inactive/disabled/withdrawn preconditions
and deletes only managed state.

Before retiring a real installation, the operator must obtain its bounded
authenticated inventory and status through the no-start path. The decision
does not infer existence, quantity, host placement, profile/generation,
active/enabled/update state, or external residue from source inspection. An
unknown fact remains unknown and cannot be filled by starting old bytes.

Do not rename a profile, inherit a duty, regenerate keys, reset a root or
floor, convert wire/state, or adopt foreign files. Old engines and readers are
deleted only after their accepting callers are independently closed and their
remaining compatibility consumers are proved.

Confidence is high for the authority split. The strongest objection is that a
crash-interrupted update may lack a deterministic no-start recovery. That is a
reason for an explicit operator-visible failure and retained evidence, not a
reason to revive old executable bytes.

## Disposition

Completed on 2026-09-22 and promoted to
[ADR-0089](../../adr/0089-retire-old-node-starts-preserve-owned-shutdown.md),
the product scope, Network/Node owner, Contributor runbook, and command
reference. Runtime gates and exclusive-code deletion remain separate bounded
implementation slices; this record does not make them integrated.
