# Security Exceptions

## Purpose

This document records active residual security risks that cannot be removed
immediately by a safe dependency upgrade or a local code change, but that still
have an explicit containment path.

It is not a history log for already remediated vulnerabilities. Closed findings
must move into the relevant decision log, dependency review, or remediation
loop evidence instead of remaining here as active exceptions.

## Required Fields For Every Active Exception

Each active exception must record:

- `Component`
- `Vulnerability`
- `Reachability`
- `WhyNotFixed`
- `AttackSurfaceReduction`
- `CompensatingControls`
- `BlastRadiusLimits`
- `OperationalDetection`
- `UpgradeTrigger`
- `OwnerDecision`

## Allowed Compensating Controls

Depending on the risk, acceptable controls include:

- disabling or not exposing non-required transport paths;
- rate limits, size limits, timeouts, and connection caps;
- process isolation and supervised restart;
- restricting access to plaintext, key material, and diagnostics payloads;
- operator-visible degradation and explicit diagnostics;
- safe-by-default configuration that keeps risky functionality out of the
  active product path.

## Invalid Ways To "Close" Risk

The following are not acceptable:

- leaving a known vulnerability undocumented;
- assuming a vulnerable path is "probably unused";
- carrying risk into release without compensating controls;
- treating a risk as closed only because it is hard to reproduce locally.

## Active Exceptions

CI reconciles this register with pinned `govulncheck` JSON and verbose output
through `tests/ci/security-gate.ps1`. The gate requires the exact active ID and
reachability set; it does not suppress scanner output or accept new findings.

### 1. Contained transitive residual: `github.com/pion/dtls/v2`

- `Component`: `github.com/pion/dtls/v2@v2.2.12`, selected transitively through
  `go-waku -> discv5 -> go-ethereum/p2p/nat -> pion/stun/v2` and also declared
  by the Waku pubsub dependency graph.
- `Vulnerability`: `GO-2026-4479` / `CVE-2026-26014` / `GHSA-9f3f-wv7r-qc8r`.
  Random AES-GCM nonces can collide and leak the authentication key in a real
  affected DTLS session. The Go vulnerability database reports no fixed v2
  release.
- `Reachability`: `govulncheck ./...` reports one symbol-reachable finding,
  primarily through Waku/DTLS initialization and generic error/string methods.
  No supported Ardents profile installs a WebRTC or DTLS transport or opens a
  DTLS endpoint. This distinction is containment, not reclassification: the
  finding remains recorded as symbol-reachable until v2 leaves the graph.
- `WhyNotFixed`: DTLS v2 and v3 are different Go major-module paths. A local
  `replace` would not be type- or API-safe. go-waku v0.10.3 remains the current
  compatible patch line and still requires v2 transitively. Removing Waku is
  prohibited because it is the canonical v1 network foundation.
- `AttackSurfaceReduction`:
  - implemented profiles are only `tcp_only` and `tcp_wss`;
  - both use `libp2p.NoTransports` and explicitly add only the TCP transport;
  - `tcp_quic` is rejected as not implemented;
  - runtime profile truth suppresses QUIC, WebTransport, and WebRTC;
  - no configuration surface can select WebRTC/DTLS.
- `CompensatingControls`:
  - `TestImplementedProfilesSuppressDTLSBearingTransportFamilies` fails if a
    supported profile advertises QUIC, WebTransport, or WebRTC;
  - `TestTransportTCPWSSExposesTCPAndWSSOnly` and constrained-mode integration
    tests inspect actual Waku listen endpoints;
  - negative `tcp_quic` tests keep unsupported transport expansion fail-closed;
  - dependency changes in Network Foundation require dependency-safety review,
    focused real Waku tests, and `govulncheck` reclassification;
  - enabling WebRTC/DTLS is release-blocking until v2 is absent and a dedicated
    security review passes.
- `BlastRadiusLimits`:
  - Ardents does not create or accept an affected DTLS session in supported
    profiles, so the vulnerable AES-GCM record path has no network endpoint;
  - the residual is confined to compiled/initialized transitive code in the
    node process, not retained payload encryption or the TCP/WSS Waku carrier;
  - a future profile expansion invalidates this exception automatically.
- `OperationalDetection`:
  - `govulncheck ./...` and `govulncheck -show verbose ./...` on every release
    candidate;
  - `go mod why -m github.com/pion/dtls/v2` to verify the upstream path;
  - transport status and diagnostics expose active/suppressed families;
  - integration tests fail if listen endpoints widen beyond TCP/WSS;
  - security-exception reconciliation is mandatory after Waku, libp2p,
    go-ethereum, Pion STUN, pubsub, or transport-profile changes.
- `UpgradeTrigger`: immediately remove this exception and DTLS v2 when a tested
  compatible Waku/discovery graph no longer selects it. Also reopen the release
  decision on any proposal to enable QUIC, WebTransport, WebRTC, DTLS, or a new
  underlying libp2p transport, or if `govulncheck` discovers a non-initializer
  execution path used by supported profiles.
- `OwnerDecision`: Network Foundation dependency governance accepts the
  residual only for the current TCP/TCP-WSS profiles and only while the stated
  tests and detection gates remain mandatory. This is not approval for public
  DTLS exposure. A release candidate must revalidate the upstream removal path.

### 2. Module-only residual: `golang.org/x/crypto/openpgp`

- `Component`: `golang.org/x/crypto@v0.52.0`, selected for libp2p Noise and
  other maintained cryptographic packages.
- `Vulnerability`: `GO-2026-5932`. The `openpgp` package is unmaintained and
  unsafe by design; the Go vulnerability database reports no fixed version.
- `Reachability`: module-only. `govulncheck -show verbose ./...` reports no
  imported-package or symbol path. `go list -deps ./...` contains no
  `golang.org/x/crypto/openpgp` package. The actual required path is
  `Ardents transport stack -> go-libp2p -> p2p/security/noise ->
  x/crypto/chacha20poly1305`.
- `WhyNotFixed`: the finding applies to one deprecated package inside an
  otherwise maintained module. There is no fixed module version because
  `openpgp` itself must not be used. Removing `x/crypto` would remove required
  maintained cryptography from libp2p rather than remediate the unused package.
- `AttackSurfaceReduction`:
  - no root or transitive compiled package imports `openpgp`;
  - Ardents exposes no OpenPGP parsing, encryption, signing, keyring, or packet
    API;
  - product cryptography uses owned payload primitives and libp2p Noise, not
    OpenPGP.
- `CompensatingControls`:
  - release scanning must preserve module-only classification;
  - any new `openpgp` import is security-review and release-blocking;
  - dependency updates are not allowed to convert the signal into an imported
    package without an explicit supported replacement design.
- `BlastRadiusLimits`: there is no compiled OpenPGP code or external OpenPGP
  input surface in Ardents. The maintained `x/crypto` packages remain in use.
- `OperationalDetection`:
  - `govulncheck -show verbose ./...`;
  - `go list -deps ./...` checked for `golang.org/x/crypto/openpgp`;
  - repository import guard and release vulnerability review.
- `UpgradeTrigger`: remove this exception if the Go project splits/removes the
  deprecated package so the module signal disappears. Immediately block and
  redesign any change that imports `openpgp` or introduces an OpenPGP input
  surface.
- `OwnerDecision`: dependency governance accepts this as module-only metadata
  debt, not a reachable runtime risk. Acceptance expires if package or symbol
  reachability changes.

## Closed Exception Policy

When a previously active exception is remediated:

1. Remove it from this file.
2. Record closure evidence in the relevant dependency review, quality
   scorecard, or decision log.
3. Keep only currently active residual risks in this register.
