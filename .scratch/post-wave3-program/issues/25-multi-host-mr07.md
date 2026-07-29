# PW3-25: MR-07 journal one-at-a-time rollout and recovery

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1 local-substitutable failure injection plus deferred R3

## Parent

`../PRD.md`

## Canonical sources

- `../../../docs/engineering/research/multi-host-reachability.md`, MR-07;
- accepted `../../../docs/adr/0013-bounded-multi-host-reachability.md`;
- accepted ADR-0006 transactional rollout journal;
- accepted ADR-0008 composite rollout readiness;
- accepted ADR-0009 immutable release materials.

## User story

As an Operator, I upgrade the exact three-host topology one Node at a time
without losing durable mixed-generation truth, and an interrupted or failed
rollout converges back to one declared fallback before another rollout begins.

## Complete vertical behavior

```text
strict manifest + compatibility declaration + immutable fallbacks
  -> exact three-Node/authority/clock/backup/material preflight
  -> durable topology-rollout-transaction/v1
  -> deterministic compatible authority-last OR migration authority-first order
  -> mutation_pending persisted before each Node recreation
  -> recreated -> started -> composite ready, identity/image/network/Store exact
  -> migration-only DR-03 fresh-generation activation
  -> ready_to_commit persisted -> exact manifest commit -> journal clear

any failure/interruption
  -> compensating persisted
  -> every journalled Node restored in reverse order
  -> data restore only when the compatibility declaration requires it
  -> fallback composite readiness for every changed Node
  -> journal clear and recovered-only return

failed compensation
  -> recovery_required retained
  -> next invocation resumes compensation and never starts a new rollout
```

## Frozen MR-07 contract

- Input is one already admissible topology manifest, a bounded request ID and
  deadline, one canonical compatibility declaration, and exactly one immutable
  fallback image per manifest slot.
- The compatibility declaration is protected and digest-bound. A compatible
  release must explicitly allow the declared mixed-version window and use the
  authority-last order. An Authority schema/protocol migration must disallow an
  ordinary mixed window, use authority-first order, and require the accepted
  DR-03 activation seam after all stopped-member mutations.
- Protected preflight occurs before journal creation or host mutation and
  exactly binds manifest/request/compatibility. It proves all three Nodes
  ready, expected identities/current images, clock bound, verified complete
  Node backups, verified separate Authority consistency-group backup, exact
  external monotonic repository head, and immutable release-material policy.
- The coordinator owns only `topology-rollout-transaction/v1`. The strict
  protected journal records revision, immutable core binding, order, target
  and fallback images, compatibility digest, per-Node phase, last stable
  reason, compensation direction and recovery requirement. It contains no
  raw Principal, Peer ID, host, address, path, signer, session, key or backup
  content.
- Journal persistence is optimistic, atomically replaced in the same protected
  directory and durable before every host mutation. A mutation whose response
  is lost remains in the compensation set.
- Exactly one Node mutation may be in flight. Compatible order is the two
  sorted non-authority slots then the authority slot. Migration order is the
  authority slot then sorted members. No next Node starts until the current
  Node has exact composite readiness and the two bootstrap/Store providers are
  again known-good.
- Host recreate and start are distinct idempotent boundaries. Readiness accepts
  only the authorized ADR-0008 composite result plus exact slot, expected Node
  identity, Waku identity, immutable image, joined/reachability truth for the
  admitted mode and required Store health.
- Migration activation is a separate accepted DR-03-owned seam. It must bind
  exact manifest, request, compatibility, generation, checkpoint/repository
  truth and all three active receipts. Deployment creates no membership or
  Channel Grant truth.
- `ready_to_commit` is persisted only after all three Nodes are applied and,
  for migration, fresh activation is exact. Manifest commit is idempotent.
  On resume, an exact already-committed target clears the journal; an
  uncommitted/ambiguous target compensates.
- Any mutation/readiness/activation/commit failure stops forward progress and
  persists `compensating`. Compensation walks every journalled Node in reverse
  order, persists `compensating` before fallback recreation, starts the
  fallback and requires the same exact readiness contract.
- Data is restored only when the compatibility declaration says the target
  migration is not in-place rollback-compatible and requires a complete
  stopped-Node consistency-group restore. Partial data restore is never an
  input or outcome.
- Failed or interrupted compensation remains `recovery_required`. A later
  invocation resumes compensation first and returns a recovered-only outcome;
  it never begins the newly requested rollout in the same invocation.
- Ordinary status contains only outcome, phase, stable reason and bounded
  counts. Protected identities, images, compatibility, repository, journal
  and adapter errors never appear.
- R1 supplies consumer-owned adapters, strict local journal persistence and
  exhaustive failure injection. It supplies no production SSH, service
  manager, image pull, backup/restore, Authority, repository or manifest
  committer composition and makes no real-host support claim.

## TDD seams

- `deployment.RolloutCoordinator` owns serial ordering and recovery.
- `deployment.RolloutPreflight` owns exact protected compatibility, backup,
  clock, material and initial three-Node evidence.
- `deployment.RolloutHosts` owns idempotent recreate/start/readiness and
  complete stopped-group restore behavior.
- `deployment.RolloutAuthority` owns only accepted DR-03 migration activation
  truth.
- `deployment.RolloutCommitter` owns exact target-manifest commit/status.
- `deployment.RolloutJournalStore` owns strict optimistic durable persistence.
- `deploymentjournal.RolloutFile` is the protected atomic file adapter.

## Acceptance criteria

- [x] Strict request/compatibility/fallback validation and complete preflight
      happen before journal creation or mutation.
- [x] Compatible authority-last and migration authority-first orders are
      deterministic and serial.
- [x] Journal state is durable before every recreate/start/activation/commit
      effect and rejects unknown fields, invalid transitions, rebinding,
      revision conflicts and oversized content.
- [x] Fault injection covers every forward mutation/readiness/journal boundary
      and every reverse-compensation boundary.
- [x] Reverse compensation includes an ambiguously mutated current Node,
      converges every journalled Node to its exact fallback, and uses complete
      data restore only when declared.
- [x] Pending recovery blocks new rollout and a successful resume returns
      without starting the requested operation.
- [x] Final commit ambiguity distinguishes exact committed target from an
      uncommitted target without assuming success.
- [x] Results and errors are bounded/redacted; concurrency permits one
      coordinator revision chain and one in-flight Node mutation.
- [x] Focused, full, race, tooling, architecture, capability, API-generation,
      vet, vulnerability and diff checks pass.
- [x] Independent Spec, Standards and Security review findings are resolved.
- [x] `deployment.multi-host` remains `Q=no`.

## Out of scope

- production SSH/service-manager/container/native-install/image/backup/
  Authority/repository/manifest adapters;
- real mixed-version wire/persistence/Authority compatibility;
- real host interruption, backup/restore, release materials or matching-commit
  qualification;
- changing DR-03 membership/generation/checkpoint semantics;
- qualification (MR-08), release, push or deployment.

## Required evidence

- red-first state-machine, strict journal and file-adapter tests;
- fault injection at all forward/reverse effect and persistence boundaries;
- compatible/migration order, backup/head/material, readiness, restore-rule,
  commit ambiguity, replay/rebinding/concurrency and redaction negatives;
- independent Spec/Standards review and focused security audit;
- all repository gates listed above.

## Admission decision

MR-03 through MR-06 and ADR-0006/0008/0009 are accepted. The current tree has
single-host Compose journal evidence and pure authority rollout order, but no
durable topology rollout transaction or multi-host failure-injection seam.
PW3-25 is admitted as that bounded R1 implementation. Real release
compatibility and three-host evidence remain MR-08 R3.

Admission changes no production state or capability qualification.

## Implementation acceptance

- Exact implementation tip: `21460bc`; logical range:
  `15c44da..21460bc`.
- The accepted coordinator owns one strict, crash-resumable transaction,
  deterministic compatible/migration ordering, exact preflight/readiness,
  monotonic activation reconciliation, exact commit and phase-aware reverse
  compensation.
- An OS-visible operation lease excludes overlapping effects across processes;
  a separate revision lock provides optimistic file CAS. Every forward and
  reverse effect is bounded, and every durable reverse checkpoint resumes
  without illegal rewind.
- Independent final Spec and Standards reviews pass. Retained security audit
  `evidence/mr07-security/run-1` confirmed and remediated four
  integrity/availability defects; no exploitable finding remains at the target
  commit.
- Focused, full, race, tooling, architecture, catalogue, document,
  API-generation, vet, vulnerability and diff gates pass.
- No production adapter, real host, qualification, release, push or deployment
  occurred. `deployment.multi-host` remains `Q=no`.

The maintainer's standing instruction treats the completed MR decision as
accepted; PW3-25 is therefore closed.
