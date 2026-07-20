# STB-101 Dependency Remediation Matrix

Date: 2026-07-18

## Decision Summary

The current graph is remediable in three controlled groups, with one residual
upstream migration problem:

1. move the build/runtime baseline from Go 1.26.1 to at least Go 1.26.5;
2. update the compatible leaf security lines (`quic-go`, `x/net`, `x/sys`, and
   `x/crypto`) without enabling a new transport profile;
3. evaluate `go-waku` as one compatibility set with libp2p and the Waku pubsub
   fork, rather than independently forcing their latest versions.

`github.com/pion/dtls/v2` cannot be repaired in place. Its advisory has no
fixed v2 release. The accepted path is removal from the selected graph through
an upstream Waku/discovery dependency migration to DTLS v3. A Go `replace`
from the v2 module path to v3 is rejected because the major-version APIs and
module identities are not interchangeable.

The scanner classification at this baseline is exactly:

- 11 symbol-reachable findings;
- 5 findings in imported packages without a discovered call path;
- 28 module-only findings.

Only the first group is called reachable below. The other groups remain upgrade
inputs and verification evidence, not evidence of an executing vulnerable
symbol.

## Runtime Reachability Context

Ardents constructs Waku in `internal/network/transport/startup.go`. The
implemented profiles are `tcp_only` and `tcp_wss`. Both install
`libp2p.NoTransports` and then the libp2p TCP transport explicitly. `tcp_quic`
is truthfully rejected as not implemented. Therefore QUIC, WebTransport, and
WebRTC/DTLS are not operator-selectable Ardents transport profiles today.

This containment does not turn scanner-reachable dependencies into clean
dependencies. `go-waku` and libp2p still compile and initialize portions of the
graph, and future adapter changes could make the dormant paths active. Safe
versions are required where they exist; a tested exclusion and exception are
required where they do not.

## Reachable Finding Matrix

| Finding | Affected module / baseline | Representative Ardents symbol path | Active surface | Fixed line | Action | Compatibility risk | Owner |
|---|---|---|---|---|---|---|---|
| `GO-2026-5856` | `crypto/tls@go1.26.1` | `cmd/ardd.main -> http.Server.ListenAndServe -> tls.Conn.HandshakeContext` | local control HTTP/TLS graph; data and generated client call paths also discovered | Go 1.26.5 | upgrade toolchain | low; patch release, then all gates | runtime assembly |
| `GO-2026-5039` | `net/textproto@go1.26.1` | `cmd/ardd.main -> http.Server.ListenAndServe -> ReadMIMEHeader` | local control boundary | Go 1.26.4 | covered by Go 1.26.5 | low | local control boundary |
| `GO-2026-5037` | `crypto/x509@go1.26.1` | `cmd/ardd.main -> http.Server.ListenAndServe -> Certificate.Verify` | TLS verification; WSS configuration | Go 1.26.4 | covered by Go 1.26.5 | low | runtime assembly / Network Foundation |
| `GO-2026-4971` | `net@go1.26.1` | `transport.Service.Start -> net.ResolveTCPAddr`; `StartWakuNode -> net.Dialer.DialContext` | `tcp_only` and `tcp_wss`, Windows | Go 1.26.3 | covered by Go 1.26.5 | low; repeat Windows network scenarios | Network Foundation |
| `GO-2026-4947` | `crypto/x509@go1.26.1` | `cmd/ardd.main -> Certificate.Verify` | TLS verification | Go 1.26.2 | covered by Go 1.26.5 | low | runtime assembly |
| `GO-2026-4946` | `crypto/x509@go1.26.1` | `cmd/ardd.main -> Certificate.Verify` | TLS verification | Go 1.26.2 | covered by Go 1.26.5 | low | runtime assembly |
| `GO-2026-4918` | `x/net@v0.50.0` and `net/http@go1.26.1` | generated Connect stream client -> `http.Client.Do`; `ardd.shutdownServerOnContext -> http.Server.Shutdown` | local API client/server | `x/net@v0.53.0`, Go 1.26.3 | use Go 1.26.5 and `x/net` at least v0.55.0 | medium; HTTP/2 and Connect streaming tests | local control boundary |
| `GO-2026-4870` | `crypto/tls@go1.26.1` | control server/client -> TLS handshake/read/write | TLS connections | Go 1.26.2 | covered by Go 1.26.5 | low | runtime assembly |
| `GO-2026-4866` | `crypto/x509@go1.26.1` | `cmd/ardd.main -> Certificate.Verify` | TLS verification | Go 1.26.2 | covered by Go 1.26.5 | low | runtime assembly |
| `GO-2026-5676` | `quic-go@v0.59.0` | graph reaches `http3.ConfigureTLSConfig`, QPACK errors, and HTTP/3 stream methods | no active QUIC profile; compiled via libp2p/WebTransport graph | `quic-go@v0.59.1` | upgrade to v0.59.1 or compatible later line; keep QUIC disabled | medium; go-libp2p pins and HTTP/3 API compatibility | Network Foundation dependency adapter |
| `GO-2026-4479` | `pion/dtls/v2@v2.2.12` | `transport` imports Waku node; Waku/discovery initialization reaches DTLS v2 cipher-suite initialization | no configured WebRTC/DTLS profile; present through Waku discovery/NAT/STUN graph | no v2 fix; v3 corrected line exists | remove v2 through upstream migration, or formally contain and register until upstream graph permits removal | high; major-version migration cannot be forced with `replace` | Network Foundation dependency governance |

The DTLS scanner traces include many generic `init`, `String`, and error-method
paths. They prove symbol reachability under `govulncheck`; they do not prove that
Ardents currently negotiates a DTLS session. The security action therefore
combines graph removal with an executable negative assertion that supported
profiles do not install WebRTC/DTLS transports.

## Imported-Package Findings

These five findings were found in imported packages but have no discovered
Ardents call path. They are not classified as reachable:

| Finding | Baseline | Fixed line | Planned treatment |
|---|---|---|---|
| `GO-2026-5038` | `mime@go1.26.1` | Go 1.26.4 | covered by Go 1.26.5 |
| `GO-2026-5026` | `x/net/idna@v0.50.0` | `x/net@v0.55.0` | choose v0.55.0 or later compatible line |
| `GO-2026-5024` | `x/sys/windows@v0.41.0` | `x/sys@v0.44.0` | choose v0.44.0 or later compatible line and repeat Windows tests |
| `GO-2026-4981` | `net@go1.26.1` | Go 1.26.3 | covered by Go 1.26.5 |
| `GO-2026-4970` | `os@go1.26.1` | Go 1.26.5 | covered by Go 1.26.5 |

## Module-Only Findings

The 28 module-only findings contain:

- 16 remediable `x/crypto` SSH/agent findings fixed in v0.52.0;
- the unmaintained `x/crypto/openpgp` finding with no fixed version;
- five `x/net/html` findings fixed in v0.55.0;
- `GO-2026-4559` in `x/net`, fixed in v0.51.0;
- six standard-library findings fixed between Go 1.26.2 and Go 1.26.3.

None has a scanner-discovered Ardents symbol call. STB-102 still upgrades Go,
`x/net`, and `x/crypto` so the selected graph does not retain preventable
module debt. STB-103 must verify that `openpgp` is absent from the compiled
package graph and record it as module-only only if it remains in a required
module.

## Dependency Posture And Compatibility Matrix

| Dependency | Selected version | Posture | License | Upgrade rule and risk |
|---|---|---|---|---|
| Go | language 1.26.0, toolchain 1.26.1 | supported release line, but below current security patch | BSD-style | set language/toolchain coherently to at least 1.26.5; low API risk |
| `go-waku` | v0.10.1, released 2025-12-10 | pre-v1, active Waku implementation and mandatory carrier; broad graph and migration-sensitive APIs | dual Apache-2.0/MIT | evaluate available v0.10.x as a whole; focused relay, Store, filter, lightpush, bootstrap, restart, and multi-node tests required |
| `go-libp2p` | v0.48.0, released 2026-03-17 | actively maintained; Ardents directly selects a substantially newer line than go-waku 0.10.1 declares (v0.39.1) | MIT | do not independently advance until Waku compatibility is proven; current version skew is already a material risk |
| Waku pubsub fork | upstream module v0.13.1 replaced by `waku-org` v0.13.1-gowaku | Waku-maintained compatibility fork, required by go-waku; latest tags cannot be assumed API-compatible | transitional dual MIT/Apache-2.0 | preserve Waku's declared replacement unless upgrading the Waku compatibility set; never silently switch to upstream pubsub |
| `quic-go` | v0.59.0 | active, production-oriented project with an explicit latest-two-Go-releases policy | MIT | v0.59.1 is the minimum surgical fix; QUIC stays unsupported in product truth |
| `x/net` | v0.50.0 | Go project module, actively maintained | BSD-style | minimum reachable fix v0.53.0, but v0.55.0 also clears imported-package/module findings; test Connect/HTTP and network integration |
| `x/sys` | v0.41.0 | Go project module, actively maintained | BSD-style | v0.44.0 minimum; Windows process/network/file tests are mandatory |
| `x/crypto` | v0.48.0 | Go project module, actively maintained except explicitly deprecated `openpgp` | BSD-style | v0.52.0 minimum for module findings; verify selected Waku/libp2p graph and absence of imported `openpgp` |
| `pion/dtls/v2` | v2.2.12 | superseded major line; advisory reports no fixed v2 | MIT | removal only; direct v2-to-v3 replacement is prohibited |
| `pion/dtls/v3` | v3.1.2 | actively maintained supported major line; current selected version is outside the advisory's affected v3 ranges | MIT | retain a non-vulnerable v3 line selected by upstream Pion/WebRTC dependencies; do not expose WebRTC implicitly |

Local module metadata confirms that go-waku 0.10.1 declares go-libp2p v0.39.1,
quic-go v0.49.0, `x/net` v0.43.0, and `x/sys` v0.35.0, while Ardents' minimal
version selection raises those to v0.48.0, v0.59.0, v0.50.0, and v0.41.0.
This is why the Waku/libp2p group is treated as an adapter compatibility change,
not a set of unrelated leaf updates.

## Waku Store Migration

The runtime currently calls both `WithMessageProvider(provider)` and
`WithWakuStore()`. In go-waku v0.10.1, `WithWakuStore()` sets `enableStore`, and
the default factory constructs `legacy_store.NewWakuStore`. The persisted
provider interface also belongs to `legacy_store`. The active wire protocol is
therefore `/vac/waku/store/2.0.0-beta4` despite the neutral option name.

The same go-waku release already contains a separate
`waku/v2/protocol/store` Store v3 client, but the node option does not switch
the server/provider path to it. Current Waku evolution also includes current
Store and Store Sync concepts. Consequently:

1. do not describe a go-waku patch upgrade as a Store migration;
2. introduce no second network owner: the change stays behind the canonical
   Network Foundation adapter;
3. first prove the exact supported server, persistence, pagination, peer
   selection, resume/sync, and retention APIs in the candidate go-waku line;
4. define wire compatibility and mixed-version behavior before disabling
   v2beta4;
5. add real two-node history/restart/partition scenarios for the replacement;
6. retire legacy provider/API imports only after the replacement scenarios pass.

Until that work is accepted, legacy Store is a named migration debt and not a
hidden implementation detail.

## Controlled Upgrade Sequence For STB-102

1. **Toolchain group**: select Go 1.26.5 or later 1.26 security patch; run
   formatting, vet, fast tests, and focused local-control/network tests.
2. **Go subrepository group**: select compatible `x/net >= v0.55.0`,
   `x/sys >= v0.44.0`, and `x/crypto >= v0.52.0`; run Windows, Connect stream,
   identity/crypto, and full fast tests.
3. **QUIC leaf group**: select `quic-go >= v0.59.1` compatible with the current
   libp2p line; prove TCP/WSS behavior and explicit QUIC rejection.
4. **Waku compatibility group**: inspect the latest compatible v0.10.x module
   graph and migration notes, then update go-waku/libp2p/pubsub only as a tested
   set. Do not accept a compile-only result.
5. **DTLS decision**: re-run verbose scanning and `go mod why`. If DTLS v2 is
   removed, prove the supported profile graph. If it remains, STB-103 owns the
   formal exception, containment tests, upstream trigger, and release decision.
6. Run the complete Phase 0 gate and real multi-node Waku relay/Store scenarios
   after the final graph settles.

## Source And Verification Record

- Scanner: `govulncheck -show verbose ./...`, executed 2026-07-18 against 89
  root packages, 144 modules, and Go 1.26.1.
- Selected module data: `go list -m -json` and locally cached module `go.mod`
  and license files.
- Vulnerability records: `https://pkg.go.dev/vuln/<GO-ID>`; in particular
  `GO-2026-4479` states that all DTLS v2 versions have no known fix and identifies
  corrected DTLS v3 ranges.
- Upstream repositories and release/migration evidence:
  - `https://github.com/waku-org/go-waku`
  - `https://github.com/waku-org/go-libp2p-pubsub`
  - `https://github.com/libp2p/go-libp2p`
  - `https://github.com/quic-go/quic-go`
  - `https://github.com/pion/dtls`
  - `https://github.com/waku-org/specs`
- quic-go's upstream repository reports v0.59.1 as the security-compatible
  maintenance release and documents support for the latest two Go releases.

## STB-101 Gate Result

Passed. Every symbol-reachable finding has an upgrade, removal, or explicit
containment/blocker path; imported-package and module-only results are kept in
their scanner categories; dependency ownership, license, maintenance posture,
version skew, and Store migration risk are explicit. Implementation proceeds to
STB-102 in the controlled groups above.
