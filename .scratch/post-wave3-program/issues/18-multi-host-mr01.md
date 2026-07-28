# PW3-18: MR-01 compile and validate a three-host topology

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R0 versioned deployment contract

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, slice
`MR-01 — Compile and validate a three-host topology`, under accepted
`../../../docs/adr/0013-bounded-multi-host-reachability.md`.

## User story

As an Operator, I can validate one bounded three-host topology manifest and
inspect deterministic redacted host-local plans before any host, network,
Authority or Node state is touched.

## Complete vertical behavior

Implement the smallest deployment tracer:

```text
strict ardents.topology/v1 bytes
  -> version/unknown-field/size admission
  -> secret and unsafe-reference rejection
  -> exact three-host support-matrix validation
  -> cross-Node Authority/repository/clock/recovery invariants
  -> deterministic host-local plan compilation
  -> bounded redacted validation result
```

The compiler is pure with respect to hosts and Ardents state. It does not open
SSH, read a signer, authenticate a Session, dial a Node, probe an address,
mutate DNS/firewall/runtime configuration, create an Authority repository or
write a rollout/fence journal.

## In scope

- strict `ardents.topology/v1` schema and canonical decoding;
- exactly three stable Node slots on three distinct operator-owned Linux amd64
  hosts, each with the `service_node` profile;
- one topology-wide `private_lan` or `public_direct` variant;
- `tcp_only` and `tcp_wss` transport/profile compatibility;
- at least two independently hosted bootstrap and persistent Store providers;
- at least two static cross-host recovery peers per Node;
- optional bounded signed-DNS roots as additive bootstrap only;
- exactly one designated Authority slot and its declared failure domain;
- independent checkpoint-repository reference/failure domain with immutable
  history, capacity capped at 65,536 and every accepted head retained until
  exhaustion;
- 30-second maximum clock skew and 60-second Authority validity margin;
- per-host SSH alias and host-key-pin references, workstation signer alias,
  expected Node Principal/Waku Peer ID equality bindings and immutable image
  references;
- deterministic, bounded, redacted host-local plans and stable validation
  errors.

## Mode-specific contract

For `private_lan`:

- each Node has exactly one private literal translated-host TCP multiaddr;
- the compiled plan requires later cross-host probe evidence before runtime
  publication;
- no field or plan may imply public reachability.

For `public_direct`:

- at least two Nodes have one manually routed public TCP or WSS address;
- a Node without explicit inbound routing is compiled as `outbound_only`;
- each public Node has exactly one advertised address and the plan requires
  later fresh AutoNAT `Public` evidence before publication;
- no plan mutates a router, firewall, DNS provider or certificate service.

Cross-scope, loopback, unspecified, ambiguous, duplicate, mixed-mode and
profile-incompatible addresses fail validation. WSS references require a
later operator-managed certificate whose DNS-ID or exact IP-ID matches; the
manifest contains no certificate private key.

## Out of scope

- MR-02 SSH forwarding, signer use, Session handling or protected status;
- MR-03 Authority/checkpoint placement or restore execution;
- MR-04 fencing, `DeploymentFenceEvidence/v1` submission or rejoin;
- MR-05/MR-06 LAN/WAN formation, probes, AutoNAT, DNS or certificate
  qualification;
- MR-07 rollout journals, host mutation, compensation or recovery;
- MR-08 qualification and any capability promotion;
- remote Operator/Application APIs, remote shell/helper installation,
  long-running controllers, schedulers, Kubernetes or Swarm;
- automatic NAT traversal, Circuit Relay, QUIC, WebTransport or WebRTC;
- arbitrary Node counts, elastic membership or mixed ingress modes.

## Dependencies and admission

- ADR-0013 was explicitly accepted in
  `68e80808b11fedbbf00ac1cfd2d9874bb74b039c`.
- ADR-0011 and CGA-01 through CGA-06 define the Authority, fencing,
  checkpoint, restore and migration vocabulary consumed by the manifest.
- ADR-0008 supplies composite readiness fields for later status/rollout; this
  slice compiles them but does not claim readiness.
- ADR-0009 requires immutable release/image references.
- No real-host, SSH, PKI, WORM or WAN environment is required to implement
  this pure contract slice. Those environments remain mandatory in their later
  integration/qualification slices.

## Authority and state ownership

- The compiler owns schema validation and deterministic plan projection only.
- Deployment owns topology intent. The future coordinator owns rollout/fence
  journals only and has no Ardents Principal.
- Each Node remains sole owner of its Principal, Waku identity/Store, runtime,
  capability state and stopped-Node consistency group.
- The designated Authority Node hosts, but does not become, the sole Realm
  Authority Principal. Authority ledger/signer/delivery state is a separate
  consistency group.
- The checkpoint repository owns independent monotonic freshness evidence; it
  is not a membership authority and cannot be created or repaired here.
- SSH identity, WSS identity, Waku Peer ID, Node Principal, Operator Principal
  and Realm Authority Principal remain distinct.

## Bounds and validation

- Manifest decoding rejects unknown fields, unsupported versions, trailing
  values and inputs above a documented finite byte limit.
- Stable names, paths/references, aliases, reasons, DNS roots, addresses,
  peers, plan rows and error detail all receive explicit finite limits.
- Exactly three Node/host records are required; host aliases, Node slots,
  expected Node Principals and expected Waku Peer IDs are unique.
- Every static recovery peer resolves to another manifest Node; self-peers,
  duplicates and fewer than two distinct remote hosts fail.
- At least two distinct Nodes are both bootstrap and persistent Store-capable;
  no single failure domain satisfies both required providers.
- At most four signed-DNS roots are accepted; later resolution remains capped
  at 128 replace-on-refresh results and never replaces static recovery.
- Exactly one Authority slot exists. Node, Authority, Authority-backup and
  checkpoint-repository paths/failure domains cannot collapse into one
  writable owner.
- Checkpoint history is immutable, capped at 65,536, and never configured for
  prune/replace/reset.
- Clock bounds are exact, not operator-adjustable weakening knobs.
- Only immutable image/material references are accepted.

Implementation may freeze additional finite representation limits and the
closed failure-domain vocabulary in this slice. Those constants must be
versioned, documented, covered by golden/negative corpus and rejected by older
schema versions when incompatible; they may not broaden ADR-0013.

## Security, privacy and redaction

- Reject inline passwords, private keys, signer paths/key bytes, Sessions,
  Credentials, Channel Grants, channel selectors/secrets, certificate keys and
  repository credentials.
- A workstation signer alias is an opaque reference only. No validation result
  dereferences it.
- Protected identity bindings may participate in equality plans and fencing
  resources, but ordinary output, logs and metrics expose only stable Node
  slots, bounded phases and reason codes.
- Hostnames, IPs, multiaddrs, DNS roots, Peer IDs, Principals, paths and
  digests are absent from public metrics and redacted from ordinary validation
  output.
- Manifest validation performs bounded local parsing only and no
  request-triggered network, filesystem-secret or process work.

## Acceptance criteria

- [ ] Strict versioned decoding and every declared size/cardinality bound fail
      closed with stable errors.
- [ ] Exactly-three-host, profile, mode, address, peer, Store/bootstrap,
      Authority, repository, clock and immutable-image invariants are complete.
- [ ] Private-LAN and public-direct plans remain distinct and cannot claim
      runtime reachability without their later mode-specific evidence.
- [ ] Deterministic host-local plans are identical for canonical-equivalent
      input and contain no secret or unredacted protected identifier.
- [ ] Unknown fields, mixed/unsupported topology, duplicate identities,
      unsafe address/path/reference input and failure-domain collapse are
      rejected before plan output.
- [ ] Parser/compiler failure has no host, network, DNS, PKI, Authority,
      repository, Node or journal side effect.
- [ ] Contract/golden/negative tests cover supported and unsupported topology
      boundaries on Linux and Windows tooling runners.
- [ ] Architecture/documentation/capability checks pass and
      `deployment.multi-host` remains `Q=no`.

## Required tests and evidence

- canonical manifest and redacted-plan golden corpus;
- table-driven schema/version/unknown-field/size/cardinality negatives;
- full private-LAN/public-direct support-matrix and cross-field validation;
- secret/path/identity leakage and deterministic-output tests;
- spies proving no SSH, dial, DNS, PKI, signer, repository, host mutation or
  process execution;
- focused package tests and race coverage where shared caches are introduced;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `git diff --check`.

Development evidence is tied to the implementation commit. Local or Docker
tests do not qualify real three-host reachability.

## Capability impact and no-Q rule

- Capability: `deployment.multi-host`.
- MR-01 may add a versioned manifest/compiler contract only.
- It does not make the topology reachable, operable or qualified and cannot
  promote `I`, `R`, `O` or `Q` without a separate evidence-backed governance
  decision.
- `Q` remains `no`; only MR-08 matching-commit real-host qualification may
  promote it.

## Expected files and modules

- a deep deployment topology package owning schema, validation and plan
  compilation;
- versioned manifest/plan contracts and golden corpus;
- narrow CLI wiring for validation only if it does not introduce host access;
- architecture and operations documentation for the frozen contract;
- focused unit/contract/security-negative tests.

Do not add SSH adapters, topology status, host mutation, rollout/fence journals
or runtime reachability behavior in this slice.

## Exit condition

MR-01 exits when one logical implementation commit provides strict bounded
manifest admission, complete cross-host invariant validation and deterministic
redacted plans; all focused/governance checks pass; independent Standards and
Spec reviews have no actionable findings; and the clean handoff explicitly
states that no host was touched and `Q=no`.

## Comments

- 2026-07-28 admission audit:
  - exact clean baseline:
    `main@68e80808b11fedbbf00ac1cfd2d9874bb74b039c`;
  - ADR-0013 is accepted and the source packet defines the complete MR-01
    contract/compiler boundary;
  - accepted Authority implementation supplies the exact Authority,
    repository, clock, fencing and restore vocabulary consumed as manifest
    references without requiring MR-01 to implement those owners;
  - the slice is locally substitutable and needs no real-host/WAN/WORM/PKI
    environment until later MR integration/qualification work;
  - outcome: `ready-for-agent`. This publishes the issue only; no implementation
    assignment, host mutation, qualification, capability promotion, deployment
    or push occurred.
