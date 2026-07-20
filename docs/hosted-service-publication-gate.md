# Hosted-Service Publication Gate

## Status And Ownership

This document defines the `v1` STB-405 publication gate. Hosted Services owns
readiness and exposure eligibility. Network Foundation owns node ingress
capability. Policy owns permission. Publication owns signed local intent,
network delivery outcome, withdrawal, retry, and compensation.

## Endpoint Model

A workload-backed service has two explicit endpoint sets:

- `probe_endpoints`: local loopback or Unix addresses used only by the trusted
  daemon readiness controller;
- `endpoints`: addresses placed in the signed discovery service record and
  used by a remote client.

The sets have equal cardinality and stable order. Each pair must use a
compatible protocol and, for HTTP/HTTPS, the same path/query. A host may differ
because the probe is local while the advertised address is LAN/public. Changing
either set invalidates readiness for the generation.

When `probe_endpoints` is omitted, `endpoints` may be reused only when every
endpoint is a local probe-safe address. This supports `LocalOnly` services and
does not create a network publication claim.

## Eligibility Gate

A `NetworkPublished` service is publishable only when all conditions hold at
the same evaluation:

1. the backing workload is observed running at the current immutable
   generation;
2. all current probe endpoints are fresh, ready, and generation-owned;
3. the paired advertised endpoint set is valid and unchanged;
4. service policy allows its type, mode, owner, and endpoints;
5. Network Foundation reports current inbound reachability;
6. reachability mode is `private_lan` or `public_direct`, never `local_only` or
   `outbound_only`;
7. the signed discovery record is delivered through the canonical Waku
   publication path.

`LocalOnly` services may be visible on the canonical local control surface but
are not sent as network service records.

## Address Validation

- probe targets follow `docs/hosted-service-probe-model.md` and never resolve
  workload-controlled DNS;
- LAN advertisements use literal private, link-local, or loopback addresses;
  loopback is permitted only for `LocalOnly`;
- public-direct advertisements use a public literal IP or an explicitly
  operator-configured public DNS name aligned with the node reachability
  configuration;
- URL credentials, fragments, wildcard/unspecified addresses, unsupported
  schemes, and ambiguous ports are rejected;
- `http`, `https`, and `tcp` are the direct-network v1 schemes; Unix endpoints
  are local-only.

For the Docker product backend, the execution adapter receives these pairs from
the canonical workload service declaration. It never accepts port or network
settings from container JSON. Services are attached to an Ardents-owned
per-generation internal bridge without arbitrary egress. A separately bounded
Ardents ingress proxy joins that bridge and the node ingress bridge, binds only
admitted ports on the explicitly configured host interface, and forwards only
to the fixed workload target at those same ports. With no admitted ingress the
workload retains `network=none` and no proxy exists.

## Withdrawal And Recovery

Publication re-evaluates the complete gate before every refresh. It writes and
delivers a signed withdrawal (empty endpoint record) within the bounded sync
operation when workload backing exits, a probe crosses the failure threshold,
a sample becomes stale, either endpoint set changes, policy denies, or network
reachability is lost.

Recovery never reuses an old decision. The current generation must satisfy the
readiness success threshold and current network/policy checks before a new
signed record is emitted. A stale generation cannot republish after a newer
generation or endpoint revision is observed.

If Waku delivery partially succeeds and then fails, Publication executes the
existing bounded compensation path and records operator-visible rollback
status. Local discovery truth, delivered network truth, and the failure reason
must not be collapsed into one boolean.

Receiving nodes use the same bounded periodic discovery refresh lifecycle to
fetch and authenticate new Waku Store envelopes after bootstrap. Exact replayed
envelopes are ignored without degradation; a newer signed publication or
withdrawal is merged by record freshness. Remote withdrawal convergence must
therefore not depend on restarting the receiving node.

## Acceptance Boundary

Linux-container tests must prove:

- unready, stale, wrong-generation, changed-address, policy-denied, and
  network-unreachable services are absent or withdrawn;
- readiness and network recovery require fresh evidence before republish;
- a second real node resolves the signed record and completes a request to the
  advertised endpoint only while it is published;
- workload exit causes bounded withdrawal and the remote request subsequently
  fails;
- delivery failure preserves explicit rollback/compensation diagnostics.
