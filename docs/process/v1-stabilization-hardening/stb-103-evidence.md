# STB-103 Evidence — Residual Security Findings

Date: 2026-07-18

## Current Scanner Truth

`govulncheck -show verbose ./...` reports:

- symbol-reachable: 1, `GO-2026-4479` in `pion/dtls/v2@v2.2.12`, no fixed v2
  release;
- imported-package without call path: 0;
- module-only: 1, `GO-2026-5932` for the unmaintained
  `golang.org/x/crypto/openpgp` package, no fixed version.

`docs/security-exceptions.md` contains exactly these two active residuals and
no longer contains the remediated `x/net` exception.

## DTLS V2 Containment Evidence

The upstream path is:

`Ardents Network Foundation -> go-waku -> discv5 -> go-ethereum/p2p/nat ->
pion/stun/v2 -> pion/dtls/v2`.

go-waku and its pubsub graph both declare DTLS v2. A cross-major `replace` is
not a safe remediation. Supported Ardents runtime profiles install only TCP,
optionally adding secure WebSocket over TCP. QUIC, WebTransport, and WebRTC are
suppressed; `tcp_quic` remains fail-closed and unimplemented.

Executable controls:

- `TestImplementedProfilesSuppressDTLSBearingTransportFamilies` covers every
  implemented profile and fails if QUIC, WebTransport, or WebRTC becomes
  active;
- readiness and transport negative tests reject the unimplemented profile;
- tagged endpoint integration tests prove actual Waku endpoints remain TCP or
  TCP+WSS rather than trusting only configuration metadata.

Focused unit, tagged endpoint integration, formatting, and code-size checks
passed after adding the control.

## OpenPGP Classification Evidence

`go list -deps ./...` reports no `golang.org/x/crypto/openpgp` package.
`go mod why -m golang.org/x/crypto` resolves through libp2p Noise to
`x/crypto/chacha20poly1305`, a maintained required package. The finding is
therefore retained as module-only and must be reclassified immediately if an
OpenPGP package import appears.

## Runtime Security Guard Assessment

- Sensitive asset: authentication keys and confidentiality/integrity of a
  hypothetical DTLS AES-GCM session; cryptographic inputs for deprecated
  OpenPGP code.
- Owner: Network Foundation dependency governance; runtime profile enforcement
  belongs to the canonical network transport owner.
- Invariant: supported profiles must not create or accept DTLS sessions while
  vulnerable DTLS v2 remains selected; unused OpenPGP code must not enter the
  compiled package graph or external input surface.
- Assessment: pass with active exception. No plaintext, key material, token, or
  retained payload is newly exposed; no default-open path was introduced.
- Exact residual risk: DTLS v2 remains symbol-reachable through initialization
  and generic method paths in the process. A future transport expansion or
  upstream call-path change can invalidate containment, so the exception is not
  a permanent release waiver.

## Gate Result

Passed. The active exception register equals current verbose scanner evidence;
the remediable findings are closed; both no-fix residuals have reachability,
containment, blast radius, detection, owner, and upgrade triggers; disabled
risky transport families are covered by executable tests.
