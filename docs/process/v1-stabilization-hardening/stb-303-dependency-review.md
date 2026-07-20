# STB-303 Dependency Review — Waku Peer Discovery

Date: 2026-07-19

## Affected Domain

Network Foundation bootstrap and peer replenishment. `Waku` remains the
canonical substrate; this review does not authorize a second discovery plane.

## Existing Dependency

- Direct dependency: `github.com/waku-org/go-waku@v0.10.3`.
- Relevant transitive implementation: go-ethereum signed ENR-tree DNS
  discovery and libp2p peer addressing already selected by go-waku.
- License: dual MIT/Apache-2.0.
- Maintenance posture: the upstream repository has moved to Logos Messaging
  and remains active, but the module/repository rename and newer release line
  mean a future upgrade requires a separate compatibility slice.
- Vulnerability posture: the current graph retains the documented
  `pion/dtls/v2` containment exception. STB-303 must not activate WebRTC/DTLS.

No new carrying dependency is accepted by this decision.

## Mechanism Decisions

### Accept: signed DNS ENR trees plus explicit static peers

go-waku's `dnsdisc.RetrieveNodes` delegates verification of `enrtree://`
records to go-ethereum DNS discovery. The URL embeds the tree signing public
key, so configured URLs are explicit operator trust roots rather than unsigned
endpoint feeds. DNS results can supply multiple libp2p peer addresses without
opening a new local transport family.

Required controls:

- validate signed `enrtree://` URLs before startup;
- bound URL and peer counts and all refresh timeouts;
- filter discovered multiaddrs to the active TCP/TCP-WSS profile;
- keep discovered peers in memory and replace them on refresh; do not promote
  stale results into durable trusted state;
- retain static peers as an explicit source and periodically replenish below
  the supported relay-peer target;
- expose DNS source failure separately from relay-readiness failure.

Recommendation: **accept** for STB-303.

### Reject for this slice: Peer Exchange

The current dependency exposes `/vac/waku/peer-exchange/2.0.0-alpha1`.
Its responder cache is populated by DiscV5, and upstream tests explicitly mark
rate-limit coverage as flaky/opt-in. Enabling only the API would therefore be a
false replenishment claim; enabling its required DiscV5 companion would widen
this slice substantially.

Recommendation: **reject for the current product path**. Reassess after the
upstream protocol/support line is stable and DiscV5 exposure is accepted.

### Reject for this slice: DiscV5

DiscV5 is a mature mechanism in the broader ecosystem, but go-waku v0.10.3
opens a UDP discovery listener, integrates NAT mapping, and contains an
upstream comment that ENR filtering is incomplete for its beta stage. It also
touches the transitive graph named by the active DTLS containment exception.
Adding it before STB-304 reachability/NAT policy and STB-306 exposure controls
would widen the runtime surface without the required operator truth.

Recommendation: **defer** to an explicit post-STB-304 dependency/security
review; do not mount it in STB-303.

## Decision

Implement the smallest supportable combination: operator-pinned signed DNS ENR
trees, explicit static peers, bounded periodic refresh, and relay-peer
replenishment. This preserves Waku ownership, avoids a self-built discovery
protocol, and does not expand supported carrier families.

Upstream evidence:

- <https://github.com/logos-messaging/logos-delivery-go>
- <https://github.com/logos-messaging/logos-messaging-go/releases>
