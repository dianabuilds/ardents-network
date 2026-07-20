# STB-304 Dependency Review — Reachability And NAT

Date: 2026-07-19

## Affected Domain

Primary owner: Network Foundation / Messaging. Discovery consumes only
advertisable endpoint truth; Publication owns publication/withdraw outcomes.

The required capability is to distinguish outbound participation, scoped local
or LAN ingress, verified direct-public ingress, and unsupported traversal paths.
Peer connectivity is not accepted as evidence of inbound reachability.

## Existing Dependency Role

- direct carrier dependency: `github.com/waku-org/go-waku@v0.10.3`;
- selected direct libp2p line: `github.com/libp2p/go-libp2p@v0.48.0`;
- license: MIT/Apache-compatible existing graph;
- maintenance/release posture: active libp2p and Waku projects with documented
  NAT, AutoNAT, Circuit Relay v2, and DCUtR/hole-punch mechanisms;
- no new module is required.

Official references:

- <https://docs.libp2p.io/concepts/nat/autonat/>;
- <https://docs.libp2p.io/concepts/circuit-relay/>;
- <https://docs.libp2p.io/concepts/hole-punching/>;
- <https://docs.waku.org/guides/nwaku/config-options/>.

## Mechanism Decisions

### Accept: AutoNAT reachability observation

go-libp2p already installs the ambient AutoNAT client. `public_direct` consumes
its stateful `EvtLocalReachabilityChanged` result and publishes configured
public addresses only after a peer dialback reports `Public`. `Private` or
`Unknown` immediately withholds those addresses.

Public-direct nodes expose the mature AutoNAT helper service with explicit
limits of 30 global and 3 per-peer requests per minute. The helper uses the
existing TCP/WSS carrier configuration; it does not add UDP, QUIC, WebRTC, or a
second network foundation.

Security posture: accepted for the bounded dialback role. AutoNAT's address
policy prevents arbitrary third-party amplification targets, and local limits
bound the remaining request surface. STB-306 still owns system-wide connection
and protocol resource controls.

### Reject: automatic UPnP/NAT-PMP port mutation

`libp2p.NATPortMap()` is not enabled. Automatic router mutation is environment
dependent, hard to make operator-visible, and does not itself prove that the
resulting address is reachable. Operators may configure external forwarding or
a load balancer, but `public_direct` still requires AutoNAT observation before
endpoint publication.

### Defer: Circuit Relay v2 and hole punching

Circuit Relay v2 and DCUtR are mature libp2p mechanisms, but a working product
path also requires trusted relay-provider discovery, reservations, relay
capacity/expiry/error truth, resource limits, and recovery evidence. The current
Waku construction contains AutoRelay plumbing but Ardents supplies no eligible
relay-provider source and claims no reservation.

Enabling relay or hole punching now would create a false NAT-traversal claim and
would cross the STB-306 abuse-control boundary. `nat_blocked` therefore remains
an explicit non-public/outbound-capable state in STB-304.

### Reject for current profiles: browser inbound mode

The current Go `service_node` is not a browser node. WSS is a server ingress
carrier, not proof of a browser-compatible constrained client. Browser/light
participation remains blocked until STB-305 implements real Filter/Lightpush
product paths; no browser reachability mode is advertised by STB-304.

## Security Reconciliation

The accepted path adds no dependency and no DTLS/UDP transport. It preserves the
containment assumptions for `GO-2026-4479` in `docs/security-exceptions.md`.
Any future WebRTC, UDP discovery, relay-provider, or hole-punch activation must
reopen dependency safety, runtime security, and abuse-control review.

## Recommendation

Accept the existing AutoNAT client plus rate-limited helper for verified direct
public ingress. Keep explicit external ingress deployment-managed. Reject
automatic port mapping and do not claim Circuit Relay/hole-punch/browser support
until their complete product paths and controls are delivered.
