# STB-306 Dependency Review — Abuse, Resource, And Admission Controls

Date: 2026-07-19

## Affected Domain

`Network Foundation / Messaging`, with operator diagnostics and the automatic
restricted-defense mode.

## Dependency Role

The carrier remains `github.com/waku-org/go-waku` v0.10.3 over its accepted
libp2p dependency. Ardents now configures the mature substrate controls already
owned there:

- Waku/libp2p maximum message size, peer count, and connections per IP;
- Waku Filter subscriber capacity and per-peer request limiter;
- Waku Lightpush per-peer request limiter;
- Waku Store client request rate;
- Relay-only node reconstruction during restricted defense.

Ardents owns only product-level operation concurrency/rate admission, bounded
Store aggregation, outbound provider penalties, public diagnostics, and mode
orchestration. No network substrate or protocol was added.

## Security Posture

- limits are non-zero by default, bounded during validation, and configurable
  only within the accepted safety envelope;
- a 140 KiB default carrier bound stays below go-waku's 150 KiB default while
  accommodating the 132 KiB private-envelope bound;
- Store accepts at most four endpoints, eight topics, and 128 results by
  default; Filter and Relay accept at most eight topics per local operation;
- three failed outbound operations against the same provider cause a 30-second
  local ban; expiry and a successful retry recover automatically;
- go-waku retains its own Relay peer scoring and protocol-specific per-peer
  limiters. Ardents does not create a second, conflicting global reputation
  score. Its provider penalty means only “repeated failures observed by this
  process”, not proof of malicious identity;
- restricted defense rebuilds the Waku node with Relay only. Store provider,
  Filter server, and Lightpush server are absent from the live protocol shape;
  steady recovery rebuilds and rejoins the full provider shape.

## RLN Assessment

go-waku exposes static and dynamic RLN Relay options when built without
`gowaku_no_rln`. Enabling either is not a local boolean: static RLN requires a
shared membership group and member index; dynamic RLN additionally requires a
membership contract, Ethereum endpoint, keystore/password, tree persistence,
and enrollment/revocation operations. Ardents has no accepted Waku-realm
membership authority or credential lifecycle yet. Inventing one inside this
slice would create an unreviewed identity/admission substrate.

Decision: do not claim RLN. For the current private/operated `v1` realm, accept
the documented equivalent *resource admission path*: Waku/libp2p connection
and per-peer protocol limits plus Ardents size, concurrency, aggregate-result,
temporary-provider-ban, and restricted-defense controls. This bounds resource
use but does not provide anonymous cryptographic spam resistance. Public Waku
realm participation remains blocked until realm selection, membership
issuance/revocation, proof budget, and interoperability are specified and
tested.

## Windows Posture

The upstream go-waku source does not disable RLN by GOOS; it gates RLN with
`gowaku_no_rln`. Its Windows build instructions require Git Bash, Chocolatey,
Make, and MinGW, while RLN depends on native zerokit artifacts. The current
Windows build proves only that the dependency graph compiles; there is no
qualified Windows RLN enrollment, proof generation, persistence, or recovery
evidence. Therefore Windows has the same bounded non-RLN admission path and
must not advertise RLN capability. A future RLN proposal needs separate native
artifact provenance and Windows lifecycle testing or an explicit unsupported
platform declaration.

## Recommendation And Mitigation

Retain go-waku/libp2p controls and the Ardents admission wrapper. Do not add a
custom RLN substitute or identity-derived tracking token. Before enabling RLN,
approve the realm and membership authority, review zerokit/native artifacts,
define secret and tree backup/rotation behavior, and pass Linux plus Windows
support-matrix tests. Re-run dependency and runtime-security reviews after any
change to Waku, libp2p, zerokit, protocol limits, or the restricted node shape.
