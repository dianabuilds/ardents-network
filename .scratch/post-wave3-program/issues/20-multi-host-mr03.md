# PW3-20: MR-03 place and recover Authority Checkpoint truth

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R0 local-substitutable implementation plus deferred R3 recovery evidence

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, slice
`MR-03 — Place and recover authority checkpoint truth`, under accepted
`../../../docs/adr/0013-bounded-multi-host-reachability.md` and the accepted
DR-03/CGA-06 same-Realm recovery contract.

## User story

As a Realm Authority Operator, I can bind one reviewed three-Node topology to
the designated Authority and verify a recovery-only restore against the latest
independent Authority Checkpoint without treating deployment or Node state as
Realm Authority truth.

## Complete vertical behavior

```text
strict ardents.topology/v1 bytes
  -> exact designated Authority context binding
  -> bounded authenticated clock observations from all three Nodes
  -> protected Authority status on the designated slot
  -> exact recovery-only sequence/digest acknowledgement
  -> existing Authority verification against the immutable repository head
  -> deterministic redacted already_ready|verified|recovery_required result
```

The topology coordinator never creates, appends, repairs, truncates, copies or
reconstructs the Authority ledger, signer, backup or Checkpoint Repository. It
only consumes the existing DR-03 protected status and recovery verification
seams. Archive extraction, configuration toggling and process restart remain
the stopped host-local procedure documented in
`docs/operations/upgrade-migration.md`.

## Frozen MR-03 contract

- `ardentsctl topology recover --manifest FILE` first applies the complete
  `ardents.topology/v1` admission contract and opens no connection for invalid
  input.
- The manifest-designated Authority slot selects one existing pinned topology
  context. That protected context additionally binds the expected Realm ID,
  Authority state reference, Authority backup reference and Checkpoint
  Repository reference exactly to the manifest.
- Three isolated per-Node clients obtain authenticated UTC observations. The
  coordinator proves the conservative inter-host skew bound is no greater
  than 30 seconds before any Authority recovery verification.
- The designated Authority client calls `InspectRealmAuthority` for the exact
  context-bound Realm. `ready` returns `already_ready` without mutation.
- Only `recovery_only/degraded` with exact non-empty Realm, sequence and
  checkpoint digest may call `VerifyRestoredAuthority`. Those exact observed
  values are acknowledged without operator rewriting.
- Verification success must return the same Realm, sequence and digest and a
  ready Authority state. Missing, stale, rolled-back, forked, ambiguous,
  corrupt, unavailable or mismatched repository truth remains owned by
  Authority and fails closed as `recovery_required`.
- Each Node is bounded to 10 seconds and the complete operation to 30 seconds.
  Each client/session is isolated by Node/interface/host pin and receives one
  bounded `EndSession` lifecycle cleanup call.
- Ordinary topology output contains only the Authority slot, outcome,
  readiness, phase and one stable reason. It never contains Realm ID,
  checkpoint digest/sequence, host, path, signer, session or repository
  locator.
- The reusable rollout-order projection is deterministic: an ordinary
  compatible release orders the two non-Authority slots first and the
  Authority slot last; an Authority schema/protocol migration orders the
  Authority slot first and the two stopped members afterward. It performs no
  mutation and does not weaken the complete backup/head precondition.

## Stable failure ownership

- local context/reference mismatch: Deployment, before protected recovery;
- host pin, tunnel, local signer or session failure: workstation adapter;
- Node time unavailable or excessive skew: Deployment preflight;
- protected denial or unavailable Authority: remote Operator boundary;
- ledger, signer, repository head/history, checkpoint or generation mismatch:
  Realm Authority;
- archive extraction, service stop/start and configuration review: host-local
  deployment procedure.

No failure class may expose protected identifiers or convert an unavailable or
ambiguous repository into a fresh Realm under the old Realm ID.

## Acceptance criteria

- [ ] Exact Authority slot/context/reference/Realm binding occurs before
      Authority inspection or recovery verification.
- [ ] Authenticated observations from all three Nodes conservatively enforce
      the 30-second skew bound; missing or excessive skew fails closed.
- [ ] An already-ready Authority is a bounded no-op and never calls recovery.
- [ ] Recovery-only verification acknowledges the exact observed Realm,
      sequence and digest and rejects any changed response.
- [ ] Partial, rollback, fork, repository loss, generation mismatch and
      protected denial remain distinct redacted failure classes.
- [ ] Authority Checkpoint retention remains exactly 65,536 immutable heads;
      exhaustion blocks mutation and neither Deployment nor recovery prunes it.
- [ ] Ordinary-compatible and Authority-migration order projections are
      deterministic, serial and Authority-last/Authority-first respectively.
- [ ] Unit, contract, adapter, security-negative and restart evidence pass
      without real-host, WORM-administration or qualification claims.
- [ ] Full/tooling/architecture/capability checks pass and
      `deployment.multi-host` remains `Q=no`.

## Out of scope

- archive creation/extraction, service-manager control or configuration
  mutation over SSH;
- repository provisioning, repair, reset, pruning or fallback;
- Realm creation after lost signer/repository truth;
- Authority transition, local-v2 migration or Channel rotation semantics
  already owned by DR-03;
- rollout mutation/journal, fencing, rejoin, LAN/WAN formation or real-host
  recovery qualification;
- capability promotion, release publication, push or deployment.

## Required evidence

- red-first tests through the public recovery and rollout-order seams;
- context-binding and three-client separation tests;
- exact already-ready, verified, skew, partial and Authority failure corpus;
- existing CGA-06 restore/repository tests remain green;
- focused package and race tests;
- `go test ./... -count=1`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `scripts/generate-api.ps1 -Check`;
- `git diff --check`.

## Capability impact and R3 boundary

- Capability: `deployment.multi-host`.
- MR-03 adds local-substitutable protected recovery coordination only.
- The existing CGA-06 drills remain non-qualification evidence.
- Real independent backup/WORM failure domains and supported three-host
  restore evidence remain R3 and are required by MR-08.
- `Q` remains `no`.

## Comments

- 2026-07-29 admission audit:
  - exact MR-02 implementation tip
    `97fed1b68d8b2a21cbf1ba44aae0b027d48ef4e3` was explicitly accepted and
    PW3-19 was closed in governance commit `5979e9b`;
  - the accepted DR-03/CGA-06 implementation already owns strict signed
    checkpoints, immutable compare-and-append, the 65,536-head exhaustion
    boundary, exact same-Realm recovery-only verification and fail-closed
    repository history validation;
  - MR-01 already owns exact Authority placement, independent backup and
    Checkpoint Repository failure-domain admission;
  - MR-03 therefore adds only the missing protected topology binding, clock
    preflight, recovery coordination and reusable rollout-order projection. It
    does not create a second Authority or repository administration path;
  - outcome: `ready-for-agent`. No real host, archive, repository, signer,
    deployment, qualification, capability promotion or push occurred.
