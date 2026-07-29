# PW3-19: MR-02 inspect three Nodes through pinned host-local control

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R0 local-substitutable protected status

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, slice
`MR-02 — Inspect three Nodes through pinned host-local control`, under accepted
`../../../docs/adr/0013-bounded-multi-host-reachability.md`.

## User story

As an Operator, I can inspect one validated three-Node topology through pinned
host-local Operator sockets and receive one truthful bounded status without
exposing a remote control API or protected identities.

## Complete vertical behavior

```text
strict ardents.topology/v1 bytes
  -> accepted MR-01 validation
  -> protected workstation context resolution per stable Node slot
  -> one distinct host-key-pinned SSH stream-local client per Node
  -> protected runtime + Network + feature observations
  -> exact identity/image/Store/reachability checks
  -> deterministic redacted ready|degraded|partial aggregate
```

The workstation coordinator has no Ardents Principal. It never runs a remote
command, opens a remote shell, installs a helper, copies a signer/session to a
host, or mutates topology/Node/Authority state.

## Frozen MR-02 contract

- The public seam is one bounded status inspection over strict
  `ardents.topology/v1` bytes and a Node observation adapter.
- `ardentsctl topology status --manifest FILE` is a workstation-local command.
- The manifest `host.ssh_alias` selects one protected named CLI context. That
  context must declare the exact manifest `host_key_pin_ref` and topology-wide
  `operator_signer_alias`.
- Every selected context requires an SSH target, explicit known-hosts file,
  absolute remote Operator Unix socket, local signer file, expected stable
  Node name, and expected Node Principal matching the manifest.
- One client/session manager is created and closed per Node. Sessions therefore
  cannot cross Node, Operator interface, or host-key-pin boundaries. Existing
  Operator-session behavior permits exactly one refresh after
  `Unauthenticated`; denial, unavailability, tunnel or pin failures do not
  refresh.
- Each Node receives at most 10 seconds and the complete operation at most
  30 seconds. Exactly three sorted rows are returned even when observations
  are partial.
- The protected adapter queries `NodeService.GetNodeRuntime`,
  `NetworkService.GetNetworkStatus`, and `NodeService.GetNodeFeatures`.
- Composite readiness is accepted only from
  `runtime.readiness.ready == true`. Identity must exactly match the manifest.
- Network status supplies joined/reachability mode/state and bounded Waku Store
  enabled/pressure truth.
- Node features supplies the configured immutable runtime image reference.
  Status reports only `match`, `mismatch`, or `unverified`; it never returns
  the image name/digest.
- A topology is `ready` only when all Nodes are observed, identity/image truth
  matches, composite readiness is ready, network truth is mode-compatible,
  and declared persistent Store providers report Store enabled without failed
  pressure. Observed but negative truth is `degraded`; any inaccessible Node
  is `partial`.

## Stable failure ownership

Per-Node inaccessible outcomes are closed and redacted:

- `host_key_mismatch`;
- `tunnel_timeout`;
- `tunnel_failure`;
- `local_signer_unavailable`;
- `remote_unauthenticated`;
- `remote_denied`;
- `node_unavailable`;
- `remote_invalid_response`.

Node identity mismatch is observed negative truth, not a transport failure.
No error or ordinary output may contain hostnames, paths, socket locations,
Principal/Waku identifiers, signer details, sessions, addresses, images, or
digests.

## Bounds and output

- exactly three rows sorted by stable Node slot;
- stable strings are drawn from closed vocabularies or capped at 128 bytes;
- no more than three protected calls per Node;
- no retry except the existing one-refresh Operator Session rule;
- ordinary output contains slot, role, observation outcome, readiness,
  joined/reachability/Store/image states, and one stable reason code only;
- partial observation never upgrades a reachability, readiness, Store, image,
  operability, or qualification claim.

## Out of scope

- rollout, host mutation, install/up, fencing, rejoin, recovery, journals or
  compensation;
- DNS resolution, reachability probing, AutoNAT qualification, certificate
  acquisition or router/firewall changes;
- Authority/checkpoint placement or restore;
- remote Operator/Application APIs, remote shell, helper installation,
  Kubernetes, Swarm or long-running controllers;
- real-host qualification or capability promotion.

## Acceptance criteria

- [ ] Exact three-context resolution binds SSH alias, host-key pin, signer
      alias, Node slot and Node Principal before any protected observation.
- [ ] Three independent per-Node clients use stream-local SSH forwarding only;
      no remote command/helper and no signer/session leaves the workstation.
- [ ] Composite readiness, reachability, Store and image truth are projected
      without protected identifiers.
- [ ] `Unauthenticated` receives one refresh only; denial, unavailable,
      pin/tunnel failure and timeout do not refresh.
- [ ] Host mismatch, tunnel timeout/failure, remote denial, unavailable Node,
      invalid response and partial result remain distinct stable outcomes.
- [ ] Full/degraded/partial aggregation is deterministic, bounded and fail
      closed.
- [ ] Contract, security-negative and adapter tests cover redaction, context
      mismatch, cross-Node session separation and all stable failure classes.
- [ ] Full/tooling/architecture/capability checks pass and
      `deployment.multi-host` remains `Q=no`.

## Required evidence

- red-first tests through the public status-inspection seam;
- table-driven full/degraded/partial and error-class corpus;
- protected-context resolver and three-client separation tests;
- existing SSH `-N -T`/stream-local/known-host tests and Operator Session
  one-refresh tests remain green;
- focused package tests and race coverage;
- `go test ./... -count=1`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `git diff --check`.

## Capability impact and no-Q rule

- Capability: `deployment.multi-host`.
- MR-02 adds local protected inspection only.
- It does not make the topology reachable, operable or qualified.
- `Q` remains `no`; only MR-08 matching-commit real-host qualification may
  promote it.

## Exit condition

MR-02 exits when one logical implementation range provides the protected
three-Node status path, all required checks pass, independent Standards and
Spec reviews have no actionable findings, and the handoff states that no real
host or production state was touched.

## Comments

- 2026-07-29 admission audit:
  - MR-01 exact implementation
    `c981cc5a6409f9827d470fa95fb16be01107dd80` was explicitly accepted in
    governance commit `b70b47f`;
  - the existing CLI already owns OpenSSH `-N -T` stream-local forwarding,
    strict known-host support, process-local Operator Sessions, exact audience
    binding and one-refresh behavior;
  - protected Node runtime and Network status already own composite readiness,
    reachability and Waku Store observations; MR-02 only adds the bounded
    cross-Node projection and the missing protected Store/image fields;
  - no real-host, WAN, PKI, WORM or Authority environment is required for this
    R0 local-substitutable slice;
  - outcome: `ready-for-agent`. No host, deployment, qualification, capability
    promotion or push occurred.
