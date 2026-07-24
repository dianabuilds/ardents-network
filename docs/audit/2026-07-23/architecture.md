# Security Architecture Reconnaissance

Audit point: commit `52af3b2480b62da60ae82c7f1d43f45cd5778230` (`main`), 2026-07-23.

## System shape

Ardents Network is a Go daemon and CLI suite. The daemon composes identity and
capability enforcement, two local ConnectRPC interfaces, Waku/libp2p transport,
discovery, encrypted content transfer and replication, workload execution,
hosting/publication, persistence, and a loopback observability surface.

Entry points:

- `cmd/ardentsd` — long-running node daemon;
- `cmd/ardentsctl` — operator CLI;
- `cmd/ardents-ingress-proxy` — per-workload TCP ingress proxy.

Concrete infrastructure adapters are isolated under `internal/network/waku` and
`internal/workload/docker`; domain packages are intended to own product truth
and expose narrow interfaces to the composition root in `internal/daemon`.

## Actors and trust boundaries

- **Node Principal** — Ed25519-derived root identity and grant issuer.
- **Operator/Application Principal** — authenticates with a root-signed device
  credential on a Unix socket and receives a binding-specific session.
- **Local transport peer** — process identity from Unix socket peer credentials
  on Linux; the peer binding is part of challenges and sessions.
- **Channel issuer/member** — receives a secret-bearing channel grant with
  independent Subscribe, Publish, and StoreFetch permissions.
- **Remote peer/node** — untrusted Waku/libp2p input until message signatures,
  discovery identity, current purpose-scoped trust, and policy are checked.
- **Workload** — either a local-development process or a constrained Docker
  container; the daemon/Docker-engine boundary remains privileged.
- **Observability client** — reaches a separate loopback-only HTTP surface;
  health is public and metrics can require a bearer token.

Primary boundaries:

1. local process → Operator/Application Unix socket;
2. remote peer → public Waku relay/store protocols;
3. trusted channel member → encrypted messaging/transfer protocols;
4. daemon → persistent files and bbolt/SQLite state;
5. daemon → Docker Engine → workload/proxy containers;
6. release source → CI, generated contracts, binaries, and images.

No first-party on-chain execution, wallet, RPC, or transaction boundary was
found. Ethereum packages are transitive dependencies.

## Main data and authorization flows

1. A local Principal proves a root-signed device credential against a challenge
   bound to node, interface, protocol major, and transport peer.
2. The access service issues a random session whose lookup key is HMAC-derived.
   Each use rechecks audience/source binding, expiry, and device revocation.
3. ConnectRPC interceptors map procedures to frozen actions and server-derived
   targets, admit the call centrally, and pass handlers a sealed
   `AuthorizedCall`.
4. Channel capabilities carry a shared secret plus permission bits. Outbound
   content is signed and then encrypted with XChaCha20-Poly1305; inbound content
   is decrypted, replay-checked, decoded, signature-checked, and authorized.
5. Discovery records and transfer/replication controls are signed and
   re-evaluated against current trust before local persistence.
6. Full/service nodes subscribe to the default Waku pubsub topic and enable
   Relay, Store, Filter, and LightPush. The Waku message provider persists to
   SQLite.
7. Admitted workloads run in constrained Docker containers. A separate
   containerized proxy forwards accepted ingress to the workload.

## Security mechanisms observed

- domain-separated Principal/Device ID derivation and canonical encodings;
- root/device signatures, purpose-scoped grants, revocations, and delegation;
- audience/source-bound one-use challenges and bounded sessions;
- XChaCha20-Poly1305, HKDF, X25519/HPKE, and deterministic signed canonical
  payloads;
- restrictive private-file handling and secret redaction in config snapshots;
- strict config decoding, unknown/duplicate-key rejection, and bounded input;
- immutable digest-pinned workload images, non-root users, read-only root,
  dropped capabilities, `no-new-privileges`, and resource bounds;
- loopback-only observability validation and constant-time token comparison;
- import-boundary checks keeping Waku/libp2p and Docker SDK usage in adapters.

## Audit focus and unknowns

The audit treats local authenticated callers, channel members, trusted remote
peers, unauthenticated Waku peers, malformed persisted state, infrastructure
failures, and CI/release inputs as adversarial where applicable. It examines
all tracked source, tests, scripts, deployment definitions, and documentation.

Runtime data under ignored `var/`, IDE metadata, Git internals, live external
networks, production nodes, registry contents, and real Docker workloads are
outside the evidence boundary. Generated protobuf/Connect code is inventoried
and checked for drift but not manually reviewed line by line as handwritten
logic.
