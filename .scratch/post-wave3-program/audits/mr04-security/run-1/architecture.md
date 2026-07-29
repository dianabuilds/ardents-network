# MR-04 security-audit architecture

## Scope and baseline

This run audits the MR-04 range `5ba408f...fa942e9`, not the whole Ardents
repository. Ardents is a Go 1.26 managed private peer-to-peer Node product.
MR-04 adds an Operator-side, crash-resumable fencing coordinator and protected
local journal. It adds no CLI command, RPC, listener, daemon composition, or
production host/network adapter; those remain R3 work. Its closest conceptual
baseline is Pacemaker/STONITH: a privileged coordinator invokes
deployment-specific, idempotent fencing agents. Ardents additionally requires
Realm Authority evidence acceptance, durable checkpoint truth, and both
survivor receipts.

The prior MR-02 skill-format audit retained an empty `findings.json`. That run
did not cover the new MR-04 state machine or journal.

## Actors and ownership

- A workstation-authenticated Operator is represented by the canonical
  `FenceRequest.Actor`. `FenceCoordinator` itself has no Principal.
- Deployment owns strict manifest/request admission, the Fence Transaction,
  adapter coordination, replay binding, and redacted status.
- `FenceIsolation` owns semantic truth of clock and target/ingress/discovery/
  peer-deny controls. The core validates receipt shape and Actor attribution.
- `FenceAuthority` owns the accepted DR-03/CGA-04 membership, generation,
  checkpoint repository, evidence-acceptance, and survivor receipt workflow.
- Realm Authority remains the only source of membership truth.
- `FenceFile` owns strict bounded protected persistence. Its caller owns the
  private directory path and single-coordinator exclusion.

The `topology.node.fence` action is registered only on the Operator vocabulary
(`api/ardents/identity/v1/contract.go`). MR-04 reuses the existing
`SubmitDeploymentFenceEvidence` procedure, but now protects it with that exact
action on `node:<target Principal>` rather than the broader membership action
and realm-channel resource. Actor must equal Effective and the target is
canonicalized before admission.

Authority additionally requires a configured `DeploymentFenceVerifier` to
authenticate the protected control receipts. The current daemon intentionally
does not configure one because R1 has no production isolation adapter, so
fresh wire submissions fail closed until the R3 composition exists. A
successful verifier call is recorded as a versioned provenance field inside
the signed ledger/checkpoint. Pre-remediation stored evidence without that
version is rejected as corrupt state rather than inherited as trusted proof.
The canonical accepted evidence digest is also stored in the hashed audit
record whose head is signed by the checkpoint. Reload validation requires a
one-to-one deterministic fence-audit record for every retained evidence item
and cross-checks operation, target, Actor, resource, class, generation, and
canonical digest; neither record can exist without its exact counterpart and
one audit cannot authenticate duplicate rotation/evidence records.

## Entry points and input surfaces

`FenceCoordinator.Fence` in `internal/deployment/fence.go` accepts strict
`ardents.topology/v1` bytes, target slot, closed reason, Actor, request ID, and
canonical start/deadline. The existing decoder rejects oversized, duplicate,
unknown, trailing, wrong-type, and forbidden-secret fields. Semantic admission
requires exactly three unique Nodes and exact Principal, peer, Authority,
repository, failure-domain, ingress, image, and clock invariants.

The coordinator persists a transaction before mutation. Immutable binding uses
the canonical manifest digest, target slot, hashes of expected Principal/Waku
identity, Actor, request, reason, and times. It passes protected raw bindings to
consumer-owned adapters.

`FenceFile.Load/Save` in `internal/deploymentjournal/file.go` accepts a
caller-supplied path. Load uses the strict-private regular-file storage
primitive and a 64 KiB bound, disallows unknown JSON fields and trailing
values, and validates the full transaction. Save checks revision, immutable
binding, legal transition, full schema and private parent, then uses the
existing same-directory private atomic replacement primitive.

## State, replay, and dangerous sinks

The durable progression is:

`requested -> isolation_pending -> evidence_persisted -> authority_pending ->
checkpoint_persisted -> peers_acknowledged -> fenced`.

Clock truth and validated control receipts are durable subprogress inside
`isolation_pending`. Failure stores `recovery_required` plus its safe resume
phase. Adapter calls that can be ambiguous across a process loss are explicitly
idempotent by immutable Realm/request. Core skips Enforce after receipts are
durable and skips all earlier calls at later phases.

Before Authority completion, persisted evidence is fully revalidated against
the admitted manifest/request. Completion requires exact operation and target,
the canonical domain-separated and control-sorted Authority evidence digest,
nonzero generation, durable checkpoint digest/repository flag, and exactly the
two manifest survivor slots with receipt digests. Authority accepts fencing
only for the removed membership target and separately requires every survivor
delivery to carry an active receipt; fencing evidence cannot substitute for a
survivor acknowledgement.

Dangerous sinks are protected journal read/write, irreversible consumer-owned
isolation controls, Authority membership mutation/evidence submission, and
ordinary rendering. Ordinary status contains only target slot, phase/outcome,
closed reason, and counts. Raw host, path, Principal/Waku, checkpoint, receipt,
session, signer, grant, and adapter error values are excluded.

## Primary hunt priorities

- state-machine skips, backwards transitions, corrupted journal replay,
  ambiguous-call repetition, concurrent writers, deadline/skew boundaries;
- forged or mismatched evidence/control/checkpoint/survivor bindings;
- path/symlink/ACL/atomic-replace races, oversized JSON, duplicate fields,
  protected-value leakage, raw error propagation;
- action/resource/Actor/Effective/delegation confusion at the future adapter
  boundary and weaker parallel Authority paths;
- false terminal fencing claims through adapter composition assumptions.

The principal audit finding was an action/resource and evidence-provenance
confusion in the pre-remediation Authority procedure. The final snapshot
closes that path with the exact Node action/resource, the fail-closed verifier,
the signed durable verification marker, target-only fencing, and independent
survivor receipt requirements.

R1 fakes and local files are not real-host qualification. Findings must show a
concrete exploitable path and meaningful impact inside this snapshot; missing
production composition alone is the declared scope boundary, not a
vulnerability.
