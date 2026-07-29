# MR-05 security architecture — run 1

Target: `C:\Users\vitek\code\ardents-network`

Reviewed implementation: `3e4b6c6` (MR-05 logical range
`4512baf..3e4b6c6`)

## Application and comparable baseline

Ardents is a managed private peer-to-peer Node implemented as a Go daemon and
Operator CLI. Waku/libp2p owns transport; local protected Unix sockets own
Operator/Application APIs. MR-05 adds no network API. It consists of an
in-process Deployment reconciliation library and a Waku runtime gate for one
translated-host private endpoint.

Comparable boundaries are libp2p/Waku external-address advertisement,
Consul-style health-gated registration, and Ansible/Nomad-style bounded
reconciliation. The inventory/host adapter is trusted separately from peer
traffic. Transport reachability is neither identity nor Realm membership.

The slice is R0/local-substitutable. It contains no production SSH,
service-manager, firewall/router, DNS administration, or protected
proof-install adapter. Ordinary `ardents.config/v1` cannot assert the protected
proof scope and therefore fails closed for `private_lan`.

## Trust boundaries

1. Raw `ardents.topology/v1` bytes enter
   `deployment.PrivateLANCoordinator.Reconcile`. Strict bounded decoding and
   full three-host validation occur before an adapter call.
2. `PrivateLANHosts.Apply/ApplyProbe` crosses into privileged host
   administration and target runtime mutation.
3. `PrivateLANDialer.Probe` crosses from one manifest host to another and
   returns a bounded observation time or error.
4. `PrivateLANStatus.Observe` crosses into protected three-Node status and
   bounded Store cache outcome reads.
5. `network.PrivateLANProbe` is an internal Go input, not an RPC or Waku wire
   message. Source attribution is supplied by the future protected adapter.
6. Remote Waku peers and DNS/bootstrap sources are untrusted transport inputs.
   They cannot directly submit proof or own topology truth.

## Input and state flow

- The manifest is capped at 256 KiB and rejects duplicate, unknown, trailing,
  noncanonical, secret-shaped, cross-scope, unsafe-address, identity and
  topology inputs.
- Exact raw manifest bytes are SHA-256-bound. Deterministic protected targets
  contain the digest, target slot, address, SSH/pin references, static peers,
  DNS roots, and the two other manifest source slots.
- All three apply operations and three ring-selected probes are serial,
  step-bounded, and inside a 30-second operation bound.
- Waku startup requires one canonical private literal TCP address plus a
  lowercase SHA-256 digest, one valid target slot, and exactly two distinct
  non-target source slots.
- Runtime proof admission compares exact digest, target, allowed source and
  address; enforces a two-minute maximum age and 30-second future-skew bound;
  and maintains monotonic success/withdrawal ordering. Failure dominates
  equal-time success and an older success cannot resurrect withdrawn truth.
- Success makes the configured endpoint publishable only as LAN scope.
  Failure, stop/start, or expiry withdraws it. Expiry through runtime tick,
  snapshot, or endpoint access notifies the daemon publication observer.
- Coordinator success additionally requires exactly three sorted ready,
  joined, image-matched and LAN-reachable Nodes, healthy required Stores, and
  bounded nonnegative retained-fetch/gap counts.

## Protected and ordinary data

Protected data includes the raw manifest/digest, host aliases/pins, private
addresses, static peers, DNS roots, Principal/Peer IDs, proof slots/times, raw
adapter errors, host plans and Node observations.

Ordinary `PrivateLANResult` contains only schema version, closed outcome/reason,
ready count, retained Store fetch count and explicit Store gap count. Raw
adapter errors collapse to stable reason codes.

## Relevant entry points and sinks

- `internal/deployment/private_lan.go`
- `internal/deployment/topology_decode.go`
- `internal/deployment/topology_validate.go`
- `internal/network/reachability.go`
- `internal/network/types.go`
- `internal/network/waku/reachability.go`
- `internal/network/waku/runtime.go`
- `internal/network/waku/service.go`
- `internal/network/waku/startup.go`
- `internal/daemon/configuration.go`
- `internal/daemon/assembly.go`
- `internal/daemon/startup.go`
- `internal/publication/refresh.go`

Privileged sinks are abstract host apply/proof installation, cross-host TCP
dial, protected status reads, Waku listener startup, and discovery endpoint
publication. MR-05 contains no shell, SSH, filesystem mutation, DNS
administration, or production adapter implementation.

## Prior audit context

Earlier MR-04 security work is outside this run. No prior MR-05
`findings.json` exists. Review findings corrected before this run include exact
proof-scope binding, confirmed withdrawal after failed/ambiguous apply,
profile-aware safe defaults, fail-before-start scope validation, monotonic
replay ordering, ambiguous IPv6/link-local rejection, and expiry observer
notification.
