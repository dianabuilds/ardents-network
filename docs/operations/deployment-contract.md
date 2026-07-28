# Ardents v1 Deployment Contract

## Supported Target

The supported `v1` deployment target is Docker Engine with Docker Compose v2 on
one Linux amd64 host. Native systemd installation on Linux amd64 is a
qualification candidate: Debian/systemd CI covers install, startup, restart,
same-build reinstall, old-to-new build transition, failed and explicit binary
rollback, stopped-node backup/restore, and non-destructive uninstall. Production
support still requires distribution-matrix and real-release transition evidence.
Both shapes run the same `ardentsd`; Docker is packaging and an optional workload
adapter, not a runtime prerequisite. Multi-host scheduling and Kubernetes
packaging remain outside this target until a separate support contract and
acceptance environment exist.

The versioned
[multi-host topology manifest contract](multi-host-topology-contract.md)
validates and redacts the accepted exact-three-host design without touching a
host. That pure MR-01 compiler does not broaden this supported deployment
target or qualify multi-host operation.

Two supported profiles and one qualification profile are versioned:

- `native` (qualification): one node installed as the unprivileged `ardents` system service,
  with protected local state and separate Operator/Application Unix sockets;

- `local-multinode`: three service nodes on an isolated Compose network, with
  generated operator credentials, an isolated local realm authority, real
  subject-bound discovery/data capabilities, and automatic discovery of the
  seed's real Waku multiaddr;
- `production`: service definitions with external deployment secrets, explicit
  persistent volumes, restart policy, health checks, and transport exposure.

## Self-Formation

The lifecycle command owns bootstrap orchestration. It starts the seed, waits
for its authenticated local control surface, reads the seed's published Waku
endpoint, validates that it contains the expected transport and peer identity,
and only then creates dependent nodes with that endpoint. Operators never copy
peer IDs, generated node keys, or hidden runtime values by hand.

Startup fails closed when the seed has no usable endpoint or a dependent node
does not become ready within the bounded deadline. A partially formed cluster
is never described as ready. A failed `up` removes only the partial containers
and Compose network from that deployment project, retains persistent volumes
and deployment state for recovery, and returns the original startup failure.
If cleanup also fails, both failures remain operator-visible.

## Exposure And Secrets

- Operator and Application APIs are protected Unix sockets and are never
  published as TCP listeners. The observability listener remains loopback-only
  inside each container and is not published to the host.
- Operators use the profile's isolated helper container locally. Remote
  Operator access uses SSH stream-local forwarding to the same protected socket.
- Only Waku transport ports are published where the selected profile requires
  host ingress.
- Local Principal/device signer material is generated into private,
  deployment-scoped volumes. Production device signer, observability, and TLS
  secrets are externally supplied files; Compose never embeds them in
  environment values or images.
- Every node has a distinct Node Principal and persistent data volume. Node and
  Waku keys are generated once in that volume and survive restart. Normal
  Operator/Application access uses short-lived Principal sessions; no reusable
  control-plane token exists.
- The local realm issuer key and channel authority state live in a dedicated
  deployment-managed volume, separate from every node data and capability
  store. Per-node capability/replay keys are distinct protected secrets.
- Local provisioning runs only while nodes are stopped, imports signed grants
  into Identity-owned encrypted stores, and writes versioned operator
  configuration. It is not a runtime fallback or a public realm authority.
- The local profile uses an isolated Docker-in-Docker workload engine over a
  Unix socket in a dedicated named volume. It never exposes a plaintext Docker
  TCP endpoint or mounts the host Docker socket. Production requires an external
  TLS-authenticated Docker endpoint with per-node client credentials.

## Health And Readiness

Container health uses the local authenticated `ardentsctl node status` path. Accepted
local cluster readiness requires every node to report product readiness, real
private discovery/data channels, joined network truth for peers, and a usable
published Waku endpoint. Missing or invalid authority/capability material fails
startup; orchestration never weakens privacy or accepts a degraded quick start.

## Lifecycle And Persistence

- `SIGTERM`: stop accepting API connections and cancel active request/stream
  contexts immediately. API drain is bounded to 5 seconds; a missed deadline is
  recorded as `API drain deadline exceeded` and remaining connections are
  closed. Domain cleanup then receives a separate 19-second budget and identity
  storage close receives 5 seconds. The combined 29-second process budget
  remains below the packaged systemd
  `TimeoutStopSec=30s`, and the process does not exit before domain cleanup
  returns or its deadline expires.
- `up`: generate missing local-only Principal and authority material, provision or reuse the
  stopped-node local realm, start the seed, discover its canonical endpoint,
  finalize peer configuration, start peers, and retain a sanitized manifest.
- `status`: read authenticated node/network/diagnostics truth without printing
  credentials.
- `stop` / `start`: preserve volumes and node identities.
- `down`: stop containers while retaining volumes by default. Destructive
  volume removal is a separate explicit operation.
- `backup`: stop the selected node and archive its complete data volume as one
  consistency group. Deployment secrets are backed up separately.
- `restore`: require an empty stopped target volume, restore the complete
  archive, then prove the same Ardents identity, Waku peer identity, and
  retained-state readiness before acceptance.
- `upgrade`: take a verified backup, record the previous immutable image
  reference, durably journal each Node before recreation, recreate Nodes one at
  a time, and require readiness after each. Any subsequent failure compensates
  the current and all previously changed Nodes to one fallback digest.
- `rollback`: restore the previous image reference through the same durable
  transaction journal; restore data only when the documented migration contract
  says the newer version changed persisted format. An interrupted or failed
  compensation remains operator-visible and is resumed before another rollout.

Live copying of `ardents.db`, Waku Store, or blob state is unsupported. Detailed
consistency groups and failure rules remain canonical in
`docs/security/persistent-state-security.md`.

## Acceptance

The deployment slice is accepted only when Linux evidence proves:

- native install initializes and starts a node without Docker, repeat install
  preserves identity and credentials, and ordinary uninstall retains state;

- clean `up` forms the cluster without a manually supplied peer ID or endpoint;
- every node is product-ready with real protected capability material; a merely
  running or network-only/degraded cluster is rejected;
- every service has a bounded health check, a persistent volume, and explicit
  transport configuration;
- restart preserves Ardents and Waku identity plus retained state;
- backup/restore preserves the complete stopped-node consistency group;
- an invalid bootstrap result, missing production secret, or failed readiness
  check stops the lifecycle command with an actionable, redacted error and
  leaves no partial project container running;
- upgrade and rollback procedures retain immutable image references and do not
  claim success before per-node readiness is re-proved.
