# MR-07 security architecture — run 1

Target commit: `21460bc`

Scope: MR-07 logical range `15c44da..21460bc`, concentrated on the
crash-resumable topology rollout coordinator, protected transaction journal,
cross-process exclusion, strict state transitions, bounded external effects,
Authority activation, manifest commit ambiguity and reverse compensation.

## Entry points and flow

1. `deployment.RolloutCoordinator.Rollout` strictly admits one exact topology,
   request, compatibility declaration, immutable fallback set and deadline.
2. A cross-process operation lease is acquired before journal load and retained
   through every external effect and terminal journal clear.
3. Protected preflight binds the exact three Nodes, clock, backups, repository
   head and release-material policy before journal creation.
4. Each forward host boundary is persisted separately and runs under the
   request deadline plus a bounded per-effect context.
5. Migration activation records monotonic generation/checkpoint/repository
   truth before the exact manifest commit. Once activated, commit convergence
   uses a bounded recovery context and never compensates the old generation.
6. Compatible failure and confirmed non-commit compensate every journalled
   Node in reverse order. Every reverse checkpoint is independently resumable.
7. The strict protected file adapter validates binding, revision, transition,
   size, parent protection and Unix ownership; operation and revision locks
   use separate OS-visible lock files.

## Trust boundaries

- Protected Operator manifest and compatibility input -> strict deployment
  admission.
- Coordinator -> future authenticated preflight, host, Authority and commit
  adapters.
- Coordinator process -> protected local journal and OS-visible locks.
- Host/Authority/commit responses -> exact request, manifest, compatibility,
  slot, image and monotonic truth validation.
- Protected transaction -> bounded ordinary status without host, identity,
  image, repository, path or adapter-error content.

## Dangerous effects

- Host recreate/start/readiness, including complete stopped-group restore.
- Irreversible Authority generation activation.
- Exact target-manifest commit.
- Protected journal replacement and deletion.

## Baseline and prior coverage

The comparable is a transaction-journalled deployment orchestrator with
optimistic revision control, one operation owner, idempotent remote effects and
authoritative status reconciliation. Earlier MR-05/MR-06 audits covered
private-LAN and public-direct reachability. This run targeted rollout ordering,
crash recovery, file races, time bounds and irreversible activation.
