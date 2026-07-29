# PW3-21: MR-04 fence one Node with crash-resumable evidence

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R1 local-substitutable failure injection plus deferred R3 host qualification

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, slice
`MR-04 — Fence one Node with crash-resumable evidence`, under accepted
`../../../docs/adr/0013-bounded-multi-host-reachability.md` and accepted
DR-03/CGA-04 membership-removal and `DeploymentFenceEvidence/v1` semantics.

## User story

As an Operator, I can resume one exact Node fencing transaction after any
coordinator interruption and can never receive a successful removal result
unless durable isolation evidence, Authority checkpoint truth, and both
survivor acknowledgements agree.

## Complete vertical behavior

```text
strict ardents.topology/v1 bytes + exact target/Actor/request/reason
  -> durable topology-fence-transaction/v1 before control mutation
  -> attributable target/ingress/discovery/peer-deny isolation controls
  -> durable bounded DeploymentFenceEvidence/v1
  -> existing DR-03/CGA-04 removal workflow
  -> durable signed checkpoint tuple + exactly two survivor active receipts
  -> fenced | recovery_required
```

The R1 implementation supplies the deterministic coordinator, strict durable
journal, public consumer-owned adapter contracts, and crash/failure corpus.
It does not provide or pretend to provide a Linux service-manager,
firewall/router, DNS, or static-peer production adapter. Such controls require
matching-commit real-host R3 evidence in MR-08 before any support claim.

## Frozen MR-04 contract

- The protected action is exactly `topology.node.fence` on
  `node:<target Principal>`. Actor is the workstation-authenticated Operator,
  Effective equals Actor, and Delegation is rejected.
- The coordinator admits the complete `ardents.topology/v1` manifest and exact
  stable target slot before opening a protected adapter or writing a journal.
- Before the first control mutation it stores one strict
  `topology-fence-transaction/v1` journal bound to the canonical manifest
  digest, target slot, hashes of expected Node Principal and Waku Peer ID,
  closed reason, Actor, request ID, start time, and deadline.
- The only accepted reason in this first bounded slice is
  `membership_removed`; later reason additions require explicit contract
  review.
- Journal phases are monotonic:
  `requested -> isolation_pending -> evidence_persisted ->
  authority_pending -> checkpoint_persisted -> peers_acknowledged -> fenced`.
  Any stable failure becomes `recovery_required`; a repeated invocation may
  resume from its recorded safe boundary but may never skip one.
- Isolation evidence requires unique attributable receipts for
  `target_ingress_blocked`, `discovery_withdrawn`, and `peer_id_denied`.
  `target_stopped` may be present when the target is reachable, but target
  acknowledgement is never required.
- Receipt and checkpoint values are strict `sha256:<64 lowercase hex>`;
  identifiers and collections are bounded. The journal stores hashes rather
  than expected Principal/Waku identifiers and contains no grant, Channel
  secret, signer, key, session, host, socket, address, or credential.
- The Authority adapter consumes the existing DR-03/CGA-04 contract. It alone
  owns membership operation IDs, generation, signed checkpoint persistence,
  active-receipt semantics, and final evidence acceptance.
- `fenced` requires an exact non-zero Authority generation, durable checkpoint
  digest, repository persistence, and active receipts from exactly the two
  manifest survivors. Authority/repository/skew/identity/receipt mismatch or
  unavailable dependencies remains `recovery_required`.
- Replaying the same manifest/target/Actor/request is idempotent. Any binding
  mismatch against an existing journal fails closed before control mutation.
- Ordinary status returns only version, target slot, phase, outcome, stable
  reason, and bounded counts. Protected bindings remain journal/adapter
  internals and are never rendered as ordinary output.

## TDD seams

- `deployment.FenceCoordinator` is the public orchestration seam.
- `deployment.FenceJournalStore` is the durable compare-and-save boundary.
- `deployment.FenceIsolation` owns attributable control enforcement.
- `deployment.FenceAuthority` owns the already-accepted DR-03/CGA-04 workflow.
- Tests substitute only these consumer-owned boundaries and observe journal
  transitions, adapter calls, resumability, output, and error ownership.

## Acceptance criteria

- [ ] Invalid manifest, target, Actor, request, reason, or journal binding
      causes no adapter mutation.
- [ ] The first journal record is durable before isolation starts.
- [ ] Every crash boundary resumes deterministically without repeating a
      completed irreversible step.
- [ ] Missing, duplicate, forged, oversized, or malformed receipts fail
      closed and never reach Authority.
- [ ] Target unavailability is compatible with fencing when the required
      deployment controls, Authority checkpoint, and both survivor receipts
      remain provable.
- [ ] Authority/repository/skew or either survivor failure yields
      `recovery_required`, never `fenced`.
- [ ] Raw expected Principal/Waku identifiers appear only in protected evidence
      where the accepted Authority binding requires them; the core transaction
      binding uses hashes. Grants, secrets, signer/session data, host details,
      and raw adapter errors appear in neither journal nor ordinary output,
      while checkpoint values never appear in ordinary output.
- [ ] Restart, contract, failure-injection, security-negative, race, full,
      tooling, architecture, capability, and API-generation checks pass.
- [ ] `deployment.multi-host` remains `Q=no`.

## Out of scope

- production Linux service-manager, firewall/router, DNS, static-peer, or
  Waku peer-deny integration;
- real host mutation, WAN/LAN formation, PKI, WORM administration, deployment,
  production state, or matching-commit qualification;
- a second membership authority, checkpoint format, repository writer,
  generation algorithm, receipt format, or Channel Grant implementation;
- `recover --rejoin`, rollout, capability promotion, release, push, or deploy.

## Required evidence

- red-first tests through the four public TDD seams;
- restart after every durable phase plus replay/binding mismatch corpus;
- target-unreachable, clock-skew, Authority/repository, checkpoint, and
  survivor failure injection;
- journal strict-schema, bounds, redaction, malformed/duplicate receipt tests;
- focused package and race tests;
- `go test ./... -count=1`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `scripts/generate-api.ps1 -Check`;
- `git diff --check`.

## Capability impact and R3 boundary

- Capability: `deployment.multi-host`.
- MR-04 is R1 local-substitutable orchestration evidence only.
- No result from fake adapters or a local filesystem journal is qualification.
- Real three-host isolation, partition, firewall/DNS/static-peer enforcement,
  Authority repository, interruption, and survivor evidence remains MR-08 R3.
- `Q` remains `no`.

## Comments

- 2026-07-29 admission audit:
  - exact MR-03 implementation tip
    `e4b8fff08abb3e5ffe4a17a41a1fbc0499849017` was explicitly accepted and
    PW3-20 was closed in governance commit `1a42036`;
  - accepted CGA-04 exact implementation tip
    `cfbbacd2dcf9044c98f4d132c1d5b90743e030d7` already owns deterministic
    membership operation identity, removal generations, durable signed
    checkpoints, immutable repository truth, survivor active receipts, and
    strict `DeploymentFenceEvidence/v1` validation;
  - protected `StopNode` exists, but no accepted production seam yet owns all
    required ingress, discovery/static-set, and peer-deny mutations. MR-04
    therefore freezes consumer-owned adapters and R1 failure injection rather
    than introducing remote shell control or a false host-isolation claim;
  - outcome: `ready-for-agent`. No real host, network, Authority, repository,
    production state, qualification, capability promotion, push, or deployment
    occurred.
