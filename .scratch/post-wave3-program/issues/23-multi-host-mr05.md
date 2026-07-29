# PW3-23: MR-05 form and recover the private-LAN topology

Status: ready-for-human
State: closed
Labels: ready-for-human
Research class: R0 implementation plus deferred R3 three-host qualification

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/multi-host-reachability.md`, MR-05, under
accepted `../../../docs/adr/0013-bounded-multi-host-reachability.md`.

## User story

As an Operator, I can form the accepted three-Node private-LAN topology from
one admitted manifest and recover its usable static topology after bootstrap,
DNS, or segment loss without publishing an unverified translated-host address.

## Complete vertical behavior

```text
strict ardents.topology/v1 private_lan manifest
  -> deterministic host-local install/up plans
  -> all three Nodes start with their translated-host advertisements withheld
  -> deterministic probe source is a different manifest host
  -> bounded exact-address TCP probe succeeds
  -> target runtime accepts fresh source/target/address-bound LAN proof
  -> exactly one private translated-host endpoint becomes publishable
  -> protected topology status proves all three joined and LAN-reachable
  -> reconciliation repeats the same plans/probes after restart or partition
```

Static recovery peers remain the mandatory restart floor. Signed DNS is
optional and additive; its observations may disappear or be replaced without
removing the static floor. At least two manifest Nodes remain persistent Store
and bootstrap providers. Waku Store remains a bounded cache, so recovery
reports retained fetches and explicit gaps rather than inventing authoritative
message completeness.

The R0 implementation supplies runtime address admission, bounded proof
gating, and a deterministic consumer-owned reconciliation coordinator. It does
not implement SSH, Linux service-manager, firewall/router, DNS administration,
or a production host adapter and cannot claim real three-host support.

## Frozen MR-05 contract

- `private_lan` accepts exactly one configured advertised address. It must be a
  literal private IPv4 or IPv6 TCP multiaddr without Peer ID, relay, DNS,
  loopback, unspecified, public, or profile-incompatible components.
- Merely binding a listener or configuring the translated-host address does
  not establish LAN reachability. The configured endpoint and ordinary
  `ReachabilitySnapshot` remain withheld/unverified until a fresh successful
  probe is applied.
- Probe proof binds the exact configured address, target slot and one distinct
  manifest source slot. It has a finite validity window, rejects future,
  expired, cross-target, cross-address, same-host, and failed observations, and
  contains no authority or public-reachability claim.
- Probe expiry or an explicit failed observation withdraws the configured
  endpoint and reports stable private-LAN-unverified truth. A later fresh
  successful observation may republish it.
- The coordinator strictly re-admits the manifest before work. It accepts only
  `private_lan`, uses all three deterministic host plans, and selects a
  different manifest host as each target's probe source.
- Host-local apply/start and proof installation are idempotent for the exact
  manifest digest and plan. Probe calls are bounded. No remote shell, helper,
  signer, session, host key, address, Peer ID, Principal, or path appears in
  ordinary output.
- Formation succeeds only when all three exact targets have successful
  cross-host proof and the protected status projection reports daemon ready,
  joined, `private_lan`, `lan`, reachable, expected immutable image, and
  healthy required Store truth.
- Reconciliation is the recovery operation. It reapplies the same plans and
  fresh probes after restart, bootstrap loss, partition repair, peer churn, or
  DNS replacement; it creates no second cluster membership or network truth.
- Static peers in the admitted manifest are always passed to host-local plans.
  DNS roots are additive and replaceable. A DNS outage never removes static
  peers or alone blocks a topology already joined through them.
- Store fetch recovery is reported as retained/found or an explicit bounded
  gap. The coordinator never treats Store as authoritative history.

## TDD seams

- `network.PrivateLANProbe` is the bounded exact observation supplied to the
  target runtime.
- `network.PrivateLANReachability` is the optional runtime proof-application
  boundary implemented by Waku without expanding the general transport
  interface.
- `deployment.PrivateLANCoordinator` owns deterministic formation and
  reconciliation.
- `deployment.PrivateLANHosts` owns idempotent host-local apply/start and
  proof installation.
- `deployment.PrivateLANDialer` owns the bounded source-host TCP probe.
- `deployment.PrivateLANStatus` owns the protected three-Node status
  observation and retained-Store/gap projection.

Tests substitute only these consumer-owned boundaries. They must not introduce
a second topology, membership, reachability, DNS, or Store authority.

## Acceptance criteria

- [x] Runtime validation accepts exactly one valid private literal TCP
      translated-host address and rejects all unsafe/cross-scope variants.
- [x] Private endpoints are withheld before proof, published only after fresh
      exact different-host proof, and withdrawn on failure or expiry.
- [x] Public-direct AutoNAT gating, outbound-only, local-only, WSS identity,
      and ordinary service publication behavior do not regress.
- [x] Coordinator formation is deterministic, bounded, redacted, and requires
      all three exact plans, cross-host probes, proof installation, and status.
- [x] Restart, bootstrap loss/recovery, segment partition/rejoin, peer churn,
      DNS outage/replacement, and retained Store fetch/gap semantics have
      local-substitutable failure-injection evidence.
- [x] Focused, full, race, tooling, architecture, capability, API-generation,
      vet, vulnerability, and diff checks pass.
- [x] `deployment.multi-host` remains `Q=no`.

## Out of scope

- production SSH, Linux service-manager, firewall/router, DNS, package
  installation, Compose, native-install, or host adapter composition;
- real independent-host routing, partition, churn, DNS, Store, or matching
  commit qualification;
- public-direct endpoint admission (MR-06), rollout (MR-07), qualification
  (MR-08), release, push, or deployment;
- changing Realm Authority, Rejoin, Channel Grants, Waku wire protocols, or
  Store into authoritative message history.

## Required evidence

- red-first runtime validation/publication and coordinator contract tests;
- exact proof binding, freshness, withdrawal, redaction, and replay negatives;
- local-substitutable restart, partition, churn, DNS replacement, bootstrap
  recovery, and retained Store/gap scenarios;
- focused package and race tests;
- `go test ./... -count=1`;
- `go test ./tests/tooling/... -count=1`;
- `go test ./tests/tooling/archaccept -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `scripts/generate-api.ps1 -Check`;
- `go vet ./...`;
- `govulncheck ./...`;
- `git diff --check`.

## Capability impact and R3 boundary

- Capability: `deployment.multi-host`.
- MR-05 is R0 runtime/coordinator implementation plus local-substitutable
  recovery evidence only.
- Real three-host private-LAN installation, cross-host dial, partition, DNS,
  Store and restart evidence remains MR-08 R3.
- `Q` remains `no`.

## Comments

- 2026-07-29 admission audit:
  - MR-03 and complete MR-04a/MR-04b are accepted and closed;
  - the current Waku runtime rejects private configured advertisements and
    marks a bound private listener reachable without the required different-
    host proof, so the canonical implementation gap is present;
  - the accepted manifest already enforces three hosts, two static recovery
    peers per Node, at least two bootstrap/Store providers, private address
    shape, failure-domain diversity, and deterministic redacted plans;
  - no production multi-host host adapter exists. This slice therefore freezes
    consumer-owned R0 seams and defers any real-host support claim to MR-08.

- 2026-07-29 implementation acceptance:
  - exact runtime/coordinator implementation tip is `3e4b6c6`; the logical
    implementation range is `4512baf..3e4b6c6`;
  - exact private address, canonical manifest digest, target and two-source
    scope are required before startup. Proof freshness and monotonic ordering
    prevent cross-target/source/address replay and old-success resurrection;
  - failed or ambiguous proof application requires confirmed withdrawal;
    stop/start, failure and expiry withdraw publication and notify discovery;
  - one centralized profile-aware default keeps service Nodes
    `outbound_only`; ordinary daemon configuration cannot self-assert protected
    private-LAN scope before production adapter composition;
  - local-substitutable reconciliation covers restart, bootstrap loss,
    partition/rejoin, churn, DNS outage/replacement and bounded Store
    retained/gap outcomes without treating Store as authority;
  - independent final Spec and Standards reviews pass;
  - retained security audit `evidence/mr05-security/run-1` found no exploitable
    vulnerability and its empty `findings.json` passes the audit schema;
  - full, focused race, tooling, architecture, catalogue, API generation, vet,
    vulnerability and diff gates pass;
  - no production adapter, real host, routing, DNS, Store, deployment, push or
    R3 qualification occurred. `deployment.multi-host` remains `Q=no`.
