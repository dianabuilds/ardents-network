---
id: R-064
title: What update lifecycle may Stage 8 retain while Horizon 3 remains a closed technical slice?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-064 — H3 update-tracer scope

## Decision this unlocks

DA-09 and M2 need to distinguish a bounded, recoverable H3 technical tracer
from a supported local update product. Without that distinction, moving the
already-existing transaction into its owner would either silently claim an
installer/activator or leave the legacy `updatetransaction` package stranded.

## Current contract

The accepted product scope permits one H3 vertical slice at a time, including
install/update/rollback and authority-recovery *prototypes*. Each needs a
research record that freezes its protected claim, finite resources, evidence,
falsification conditions, and exclusions. Horizon 4, rather than H3, contains
supported signed Windows/Ubuntu packages, repair, safe update, uninstall, and
authority backup.

ADR-0015 separates Release Decision from local activation, but expressly does
not select a public release authority, complete native-host qualification, or
Windows installation. ADR-0021 is a custody-envelope decision, not authority
for an Update-owned Vault. R-050 and the Stage 7 lifecycle material are useful
development evidence for the transaction shape; they do not promote its
platform mutations or package formats to a supported H3 product.

The current command composes one offline accepted Release authorization with
one stopped-runtime/no-op-self-test Update transaction. M1 has retained that
command only as the named C2 V0 tracer observer through M13. It is not a
bootstrap, installer, activation, custody, or compatibility promise.

## Hypotheses

- **H1:** Stage 8 may retain and transfer one bounded offline Update tracer to
  its `update` owner. It protects only transaction integrity: accepted Release
  authorization, immutable local staging, bounded stopped-runtime Adapter
  calls, journal/restart/rollback handling, and terminal result classification.
  It creates no supported lifecycle or custody surface.
- **H2:** M2 may treat the existing tracer as a supported installer/update
  command and complete a native activation lifecycle from it.
- **H0:** No H3 update tracer may remain; delete the transaction and command
  until a supported lifecycle is promoted.

## Evaluation criteria

H1 is admissible only if all of the following remain true:

- the protected fact is an accepted Release authorization and its exact local
  candidate identity; accepted authorization never becomes an installer,
  signing, or Authority capability;
- the finite scope remains one local transaction root, one candidate of at
  most the existing bounded artifact size, one predecessor reservation, the
  existing finite journal/checkpoint set, and the already-declared stopped
  runtime and self-test Adapter seams;
- it claims only deterministic local transaction/recovery behavior observed by
  its tests, not installation, host registration, package repair/uninstall,
  service readiness, Windows support, availability, or product activation;
- Release owns trusted metadata/floors and Update owns only its own transaction
  root; Custody secrets, recovery bundles, and signing watermarks enter neither
  Module; and
- the retained V0 command remains a C2 observer adapter with the exact expiry
  and removal decision in M13. It is not a public command contract.

Any M2 change which adds a package manager, bootstrap, service/startup/desktop
registration, privilege elevation, platform-specific installed activation,
authority storage, or a claim that the candidate is running falsifies H1 and
requires a new promoted lifecycle/custody decision.

## Evidence plan

### Primary sources

- `docs/product/scope.md`, Horizon 3 and Horizon 4, accessed 2026-08-23.
- ADR-0015 and ADR-0021, accessed 2026-08-23.
- R-050 and the Stage 7 lifecycle specification, accessed 2026-08-23.
- `docs/development/stage-8-decision-authority-register.md`, target
  architecture, refactoring plan, and G2 delta review, accessed 2026-08-23.
- Current `internal/updatetransaction` and `cmd/ardents-release` source and
  tests, inspected 2026-08-23.

### Experiment

Before M2 integration, run the existing offline transaction's checkpoint,
interruption, restart, idempotence, rollback, cleanup, and authorization
characterizations after the ownership transfer. The frozen V0 command test
must still identify its result as a tracer adapter. The evidence is local to
the bounded test root and cannot substitute for a native-host or Qualification
campaign.

### Failure scenarios

- a forged or rejected Release Decision reaching staging;
- restart, interruption, corruption, or lock contention changing the durable
  transaction classification;
- a self-test or stopped-runtime adapter being represented as a real endpoint
  activation/readiness proof;
- an Update result being represented as an installed, repaired, or running
  product;
- any Custody secret, recovery material, signing watermark, or floor writer
  crossing into Update; and
- a discovered external command observer requiring an unsupported compatibility
  commitment.

## Findings

- **Sourced fact:** Horizon 3 explicitly permits separately scoped
  install/update/rollback prototypes, while Horizon 4 owns supported packages,
  repair, safe update, uninstall, and authority backup.
- **Sourced fact:** ADR-0015 supplies a separation between release verification
  and local activation, but does not authorize a public release authority,
  native-host qualification, or Windows installation. ADR-0021 does not make
  Custody an Update responsibility.
- **Sourced fact:** the accepted Stage 8 target already requires
  `cmd/ardents-release` to retire as an H3 product command and limits it to
  technical inputs until DA-09 changes scope.
- **Measurement:** the G2 review's cited offline corpus passed for the current
  tracer and its recovery matrix; it expressly records that this is not a
  product activation or platform claim.
- **Inspection:** the sole production composition supplies `stoppedRuntime`
  and `offlineCandidateTest`, which do not stop/drain a real runtime, activate
  an installed payload, or prove IPC readiness.
- **Inference:** H1 preserves the useful security-forward transaction
  characterization without manufacturing the missing product lifecycle. H0
  would discard a bounded H3 prototype that the accepted scope expressly
  permits, while H2 would jump directly to Horizon 4 obligations.

## Options

| Option | Product and security fit | Consequence | Disposition |
|---|---|---|---|
| H1: technical tracer only | Fits the H3 prototype allowance and preserves Release/Update separation without a host claim. | M2 may transfer/refine the bounded transaction owner and tests; command remains C2 through M13; real platform work is excluded. | Accepted. |
| H2: supported lifecycle | Would make a useful end-user feature. | Contradicts the H3/H4 boundary and would select unmeasured platform, privilege, repair, uninstall, and custody behavior by refactoring. | Rejected. |
| H0: delete now | Avoids any chance of a false lifecycle claim. | Discards the accepted H3 technical input and its recovery evidence without a product or security reason. | Rejected. |

## Recommendation

Accept H1. DA-09 is closed only for the limited M2 decision: a bounded offline
technical tracer is an authorized H3 vertical slice, not a real supported
release/update/custody lifecycle. M2 may make the transaction's ownership,
authorization boundary, and finite recovery behavior coherent under `update`.
It must neither enlarge the transaction into a platform installer nor introduce
a Custody Module. A future supported lifecycle, native adapter, package format,
or custody/secret decision reopens DA-09 and requires its own authority path.

Confidence is high because the result follows the explicit H3 prototype and H4
promotion split. The strongest objection is that an executable command can be
mistaken for a product update surface; the named C2 observer, its frozen test,
the no-claim wording, and mandatory M13 retirement remove that ambiguity from
the maintained scope.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** DA-09 is closed for M2's technical-tracer scope only. No ADR is
required: this selects no platform, package format, custody model, public
command, or durable representation. M2 must retain the stated exclusions and
run its transaction/recovery characterization after transfer; any falsifier
stops the work pending a new lifecycle decision.
