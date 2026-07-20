# STB-304 Evidence — Reachability And NAT Strategy

Date: 2026-07-19

## Capability Validated

Owning domains: Network Foundation / Messaging, coordinated with Discovery and
Publication / Hosted Services.

The runtime now treats Waku participation and inbound reachability as separate
facts. A listener, configured address, or outbound peer no longer proves public
ingress. Supported modes are `local_only`, `private_lan`, `outbound_only`, and
`public_direct`.

`public_direct` accepts only bounded, transport-compatible public TCP/WSS
multiaddrs. They remain unpublished until the existing libp2p AutoNAT client
reports `Public`; `Private` or `Unknown` withdraws them and degrades the relevant
capabilities. Address changes require restart and a fresh observation.

## Dependency And Transport Decision

The complete assessment is retained in `stb-304-dependency-review.md`.

- affected domain: Network Foundation / Messaging;
- dependency role: existing go-libp2p AutoNAT observation and bounded helper
  service inside the canonical go-waku foundation;
- security posture: accepted with a 30-dial total, three-per-peer, one-minute
  rate-limit window and strict advertised-address validation;
- recommendation: retain AutoNAT for direct-public evidence;
- mitigation: do not enable automatic UPnP/NAT-PMP, unmanaged Circuit Relay
  reservations, hole punching, browser ingress, QUIC, WebTransport, or WebRTC;
- no new module or parallel network substrate was introduced.

## Runtime Truth And Failure Behavior

- `local_only` reports local scope only;
- `private_lan` reports LAN reachability without a public claim;
- `outbound_only` can join Waku but never publishes an inbound endpoint;
- `public_direct` starts with `reachable=false` and no published endpoint;
- AutoNAT `Public` publishes only explicitly configured addresses;
- AutoNAT `Private`/`Unknown` withdraws addresses and reports stable degraded
  state/reason through the canonical status surface;
- WSS advertised host and port must match the certificate-backed WSS ingress;
- unsupported browser, relay, hole-punch, router-mutation, peer-ID-suffixed, and
  circuit addresses fail explicitly or remain unavailable.

## Runtime Security Review

Sensitive asset: the node's public topology and advertised ingress address.

Security invariant: public topology is exposed only by explicit operator
configuration and only after peer dialback evidence; no key, token, plaintext
payload, or selector secret enters logs, diagnostics, or API snapshots.

Assessment: passed. Defaults are `private_lan` for service nodes and
`local_only` for local development. Public-direct loopback configuration fails
at the process boundary, AutoNAT helper activity is bounded, and degradation
reasons reveal only mode/state rather than resolver, credential, or payload
details.

## Real-Runtime Evidence

- `TestPublicIngressObservationWithdrawsAndRecoversRealWakuEndpoints` uses a real
  go-waku node and typed libp2p reachability events to prove publish, withdraw,
  and recover behavior;
- `TestPublicAddressChangeRequiresRestartAndFreshObservation` proves that a new
  address cannot inherit old reachability;
- formal scenario `NFI-004`,
  `TestPublicReachabilityGatesAndWithdrawsNodeAdvertisement`, starts a real
  process/Waku runtime and proves status plus signed local node-record update and
  withdrawal across public/private observations;
- focused unit coverage proves LAN, outbound-only, invalid public address,
  browser rejection, and WSS host/port mismatch behavior.

## Acceptance Gates

- focused transport/readiness/process/daemon/control/Connect tests — passed;
- focused formal `NFI-004` integration — 1/1 passed;
- formatting, `go vet ./...`, import boundary, and canonical fast runner —
  passed;
- race suite across transport, readiness, process/orchestration, status, and
  daemon configuration — passed;
- handwritten production code-size guard — passed with no soft or hard breach;
- full integration report at
  `tests/.artifacts/reports/stb-304-integration`: 106/106 passed, 0 failed;
- full E2E report at `tests/.artifacts/reports/stb-304-e2e`: 14/14 passed,
  0 failed;
- test catalog: 120 tests, 33 scenarios, 120 formal bindings, 0 issues;
- `govulncheck` reconciliation: unchanged one symbol-reachable
  `GO-2026-4479` in Pion DTLS v2 with no fixed v2 release, zero imported-package
  findings, and one module-only `GO-2026-5932`; this exactly matches
  `docs/security-exceptions.md`.

## Acceptance Decision

Accepted. The slice uses the mature Waku/libp2p foundation, covers success,
degraded, withdrawal, recovery, LAN, outbound-only, and address-change paths,
keeps public claims fail-closed, exposes current truth through the canonical
local surface, preserves documented security constraints, and defers no
critical behavior required by STB-304. Managed relay ingress and browser light
client flows remain explicitly unsupported here and are owned by later scoped
work rather than falsely advertised as complete.
