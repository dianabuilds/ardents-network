# PW3-22: MR-04b rejoin one fenced Node through fresh Authority truth

Status: needs-info
State: open
Labels: needs-info
Research class: R1 local-substitutable recovery injection plus deferred R3 host qualification

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, the Rejoin
portion of `MR-04 — Fence and rejoin one Node truthfully`, under accepted
`../../../docs/adr/0013-bounded-multi-host-reachability.md` plus its proposed
2026-07-29 compatibility amendment, accepted DR-03/CGA-04 membership-add
semantics, and accepted MR-04a fencing tip
`fa942e9f52ea7ae2fc4ddf9db81322fd72732c09`. The amendment requires explicit
maintainer acceptance before this issue can become `ready-for-agent`.

## User story

As an Operator, I can recover one previously fenced Node only through fresh
Realm Authority membership and generation truth, and an interrupted recovery
can never silently restore old authority or leave the target ambiguously
reachable.

## Complete vertical behavior

```text
strict ardents.topology/v1 bytes + exact target/Actor/request
  -> exact immutable terminal Fence Transaction and removal checkpoint binding
  -> proposed durable topology-rejoin-transaction/v1 before mutation
  -> bounded clock and still-fenced preflight
  -> target start in deployment quarantine + exact identity/image check
  -> fresh delivery-key attestations from all three recipients
  -> fresh DR-03 membership-add operation and pending deliveries
  -> fresh pending delivery installation on all three recipients
  -> signed generation activation checkpoint
  -> exactly two survivor active receipts
  -> idempotent restoration of static/DNS/ingress/peer configuration
  -> target fresh-generation activation + identity/clock/composite readiness
  -> target active acknowledgement + final signed membership checkpoint
  -> rejoined | phase-truthful isolated recovery_required
```

The R1 implementation supplies a deterministic coordinator, strict durable
Rejoin journal, consumer-owned adapter contracts, and crash/compensation
corpus. It does not compose Linux service-manager, firewall/router, DNS,
static-peer, Waku peer-allow, SSH, or production Authority adapters and cannot
claim real host recovery.

## Frozen MR-04b contract

- The proposed Rejoin Transaction is not an inverse fence operation. The accepted
  `topology-fence-transaction/v1` remains immutable and terminal `fenced`.
  A separate `topology-rejoin-transaction/v1` links the exact manifest digest,
  target slot, expected Principal/Waku hashes, fence request/evidence digest,
  removal operation/generation/checkpoint, Actor, new request, start, and
  deadline.
- The workstation coordinator has no Principal and introduces no broad
  `topology.node.rejoin` server permission. It uses one direct
  workstation-authenticated Actor and rejects Delegation.
- The complete Ardents authorization intersection is frozen as:
  `realm.channel.membership.change` on the exact realm-channel;
  `realm.channel.delivery.prepare` on each exact recipient Principal;
  `realm.channel.delivery.install` and
  `realm.channel.delivery.acknowledge` on exact fresh deliveries;
  `realm.channel.activation.commit` on the fresh operation;
  `realm.channel.generation.activate` on that operation;
  `realm.channel.activation.acknowledge` on exact fresh deliveries;
  `realm.channel.audit.read` on the exact Realm/channel;
  `node.start`, `node.runtime`, and `node.features` on the target Node;
  `transport.network_status` on its network;
  `config.effective`/`config.reload` only when Ardents configuration changes;
  and bounded Session lifecycle cleanup. Tests may use only these existing
  action/resource owners.
- OS service-manager, firewall/router, DNS/static-set, and peer-allow controls
  are external deployment administration, not Ardents RPC authorization.
  There is no accepted production adapter or permission vocabulary for them in
  R1; fakes cannot imply one.
- Complete manifest, context, target, immutable-image, identity, prior-fence,
  repository, and failure-domain admission occurs before opening an adapter or
  writing a new journal. Any mismatch fails closed without mutation.
- Preflight proves the target is still isolated and obtains bounded
  authenticated UTC observations for all three manifest hosts. Absolute
  inter-host skew above 30 seconds, unavailable time, or an Authority validity
  window without the accepted 60-second margin blocks rejoin.
- The target starts in deployment quarantine before membership preparation:
  deployment ingress and discovery/static publication remain withdrawn and
  both survivors continue to deny its Waku Peer ID. Only the protected
  host-local Operator path is available. Exact manifest Principal, Waku
  identity, immutable image, and bounded clock are checked, but this phase
  provides no joined/readiness claim.
- All three recipients produce fresh bounded delivery-key attestations for the
  Rejoin request. Realm Authority then creates or replays one
  `MembershipChangeAdd` operation for the removed target on the exact channel
  and returns a new operation ID, a generation strictly newer than the removal
  generation, fresh delivery identities, and each signed
  prepare/activation/completion checkpoint retained in the independent
  repository.
- All three fresh pending deliveries are installed and acknowledged through
  their exact recipient-bound protected paths while the target remains
  quarantined. Only then may Authority commit the signed activation
  checkpoint. The two manifest survivors must activate and return
  Authority-valid `active` receipts for that exact
  operation/generation before Deployment restores target ingress,
  discovery/static usable sets, or survivor peer allowance.
- Restoration and quarantined-start adapters are idempotent for the immutable
  Rejoin request. The target may install and activate only the new generation;
  no prior grant, receipt, removal checkpoint, or fence evidence can be
  converted into current authority.
- After restoration, the target activates the fresh generation and must return
  the exact manifest Principal and Waku identity, immutable image, bounded
  clock result, composite readiness, and joined network truth. Only then may
  the coordinator submit its Authority-valid `active` receipt.
- Activation commit is the first Authority transition that makes the fresh
  membership and generation current. The target active receipt is the final
  completion input. Authority completion must bind all three current
  recipients to the new generation and final signed checkpoint before
  `rejoined`.
- Durable phases are monotonic:
  `requested -> preflight_persisted -> target_quarantined ->
  attestations_prepared -> authority_pending -> deliveries_prepared ->
  deliveries_installed ->
  activation_committed -> survivors_acknowledged -> restoration_pending ->
  readiness_verified -> target_acknowledgement_pending ->
  checkpoint_persisted -> rejoined`. Any stable failure becomes
  `recovery_required` plus its safe resume phase.
- Before activation commit, failure re-enforces the exact isolation controls,
  leaves the accepted removal as current Authority truth, and resumes the same
  pending add operation.
- From activation commit onward, failure re-enforces the exact isolation
  controls while the fresh membership and generation remain current but
  incomplete. Recovery resumes the same operation and reports neither
  `fenced` nor `rejoined`; it never cancels or replaces that operation
  implicitly.
- An ambiguous target acknowledgement is immediately re-isolated and remains
  `recovery_required` until exact idempotent Authority reconciliation. If the
  add completed, the target is a fresh active member that is deployment-
  isolated, not `fenced`; restoration and readiness must be re-proved before
  `rejoined`. If it did not complete, the same pending operation resumes. A new
  compensating removal is never invented implicitly.
- Ordinary output contains only version, target slot, phase, outcome, stable
  reason, and bounded acknowledgement/readiness counts. Principal/Waku values,
  grants, checkpoint/evidence/receipt digests, hosts, paths, signer/session
  material, and raw adapter failures remain protected.

## TDD seams

- `deployment.RejoinCoordinator` owns the monotonic orchestration.
- `deployment.RejoinJournalStore` owns strict compare-and-save persistence.
- `deployment.RejoinAuthority` consumes the existing DR-03 membership-add,
  activation, checkpoint, repository, reconciliation, and active-receipt
  workflow.
- `deployment.RejoinMembers` owns recipient-bound fresh attestation,
  pending-delivery installation, and generation activation calls without
  owning Realm membership truth.
- `deployment.RejoinRestoration` owns still-fenced inspection, idempotent
  quarantined target start, configuration restoration, and idempotent
  re-isolation.
- `deployment.RejoinReadiness` owns exact target identity, clock, immutable
  image, composite readiness, joined truth, and target acknowledgement
  observation.

Tests substitute only these consumer-owned boundaries. They must not introduce
a second membership ledger, fake production support, or a remote shell.

## Acceptance criteria

- [ ] Invalid manifest, Actor, request, target, prior fence, or removal binding
      causes no Authority, restoration, or target mutation.
- [ ] If the amendment is accepted, the Rejoin Transaction is durable before
      the first Authority or configuration mutation; the terminal Fence
      Transaction is never changed.
- [ ] Rejoin requires a distinct membership-add operation, strictly newer
      generation, repository-persisted prepare/activation/completion
      checkpoints, fresh recipient attestations, and fresh deliveries installed
      on all three Nodes; the target prepares/installs while quarantined and
      exactly two survivor active receipts precede restoration.
- [ ] Old grants, prior receipts, fence evidence, or the removal checkpoint can
      never satisfy fresh target authority or terminal readiness.
- [ ] Crash and ambiguous-call recovery is deterministic at every phase and
      never repeats a completed irreversible effect.
- [ ] Before activation commit, failure re-establishes isolation while the old
      removal remains current and the same add resumes.
- [ ] From activation commit onward, restoration/start/readiness failure
      re-establishes isolation while fresh membership remains current but
      incomplete. Ambiguous target acknowledgement is re-isolated and
      reconciled without falsely reporting `fenced` or `rejoined`.
- [ ] Target Principal, Waku identity, image, clock, joined truth, composite
      readiness, target active receipt, and final Authority result must all
      match the admitted manifest and fresh generation.
- [ ] Journal and ordinary status preserve the protected-data boundary.
- [ ] Contract, restart, compensation, security-negative, full, race, tooling,
      architecture, capability, and API-generation checks pass.
- [ ] `deployment.multi-host` remains `Q=no`.

## Out of scope

- production Linux service-manager, firewall/router, DNS, static-peer,
  Waku peer-allow/deny, SSH, Session, or Authority adapter composition;
- real host start, LAN/WAN formation, partition repair, PKI, WORM
  administration, deployment, production state, or matching-commit
  qualification;
- changing the accepted Fence Transaction, removal checkpoint, Realm
  Authority model, generation algorithm, receipt format, or Channel Grants;
- MR-05 private-LAN formation, MR-06 public-direct ingress, MR-07 rollout,
  MR-08 qualification, release, capability promotion, push, or deploy.

## Required evidence

- red-first tests through the six public TDD seams;
- strict prior-fence and fresh-generation binding corpus;
- restart after every durable phase and ambiguous adapter completion;
- survivor partition, clock, repository, restoration, start, target identity,
  old-grant, target receipt, readiness, and re-fencing failure injection;
- journal strict-schema, bounds, redaction, transition, replay, and conflict
  tests;
- focused package and race tests;
- `go test ./... -count=1`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `scripts/generate-api.ps1 -Check`;
- `git diff --check`.

## Capability impact and R3 boundary

- Capability: `deployment.multi-host`.
- MR-04b is R1 local-substitutable orchestration evidence only.
- Fake adapters and local journals cannot qualify Rejoin or complete canonical
  MR-04 production support.
- Real three-host isolation restoration, fresh Authority delivery, partition,
  interruption, re-fencing, and readiness evidence remains MR-08 R3.
- `Q` remains `no`.

## Comments

- 2026-07-29 admission audit:
  - the maintainer accepted exact MR-04a fencing implementation tip
    `fa942e9f52ea7ae2fc4ddf9db81322fd72732c09` and PW3-21 was closed in
    governance commit `af70ff9508b361864b1be0fe5b87ad7ef3dab197`;
  - the canonical MR-04 contract includes both terminal fencing and fresh
    Rejoin. PW3-21 intentionally excluded Rejoin, so MR-05 remains blocked
    until this dependency-ordered slice is accepted;
  - accepted CGA-04 already proves removed-to-added membership uses a new
    operation and generation, fresh deliveries, signed checkpoint/repository
    truth, all-recipient active receipts, and no resurrection of old grants;
  - independent Spec review found the accepted ADR-0013 Rejoin ordering
    incompatible with accepted CGA-04: the target's pending delivery must be
    installed before activation commit, and that commit already makes fresh
    membership current before active receipts;
  - a dated proposed ADR amendment now defines a separate linked Rejoin
    Transaction and phase-truthful recovery without rewriting the accepted
    terminal Fence Transaction;
  - no accepted production seam currently restores every ingress,
    discovery/static-set, peer-allow, target-start, and readiness control.
    MR-04b therefore admits only consumer-owned R1 coordination and explicit
    compensation, not host control or a support claim;
  - outcome: returned with blockers as `needs-info`. Explicit maintainer
    acceptance of the proposed ADR amendment is required before
    `ready-for-agent` or implementation. No real host, network, Authority,
    repository, production state, qualification, capability promotion, push,
    or deployment occurred.
