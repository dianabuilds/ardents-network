# STB-604 Evidence — Self-Forming Safe Multi-Node Deployment

## Accepted Product Properties

- `./ardents.ps1 up` generated distinct local API credentials, started the
  seed, selected its non-loopback canonical TCP Waku endpoint, and formed a
  three-node network without operator-supplied peer IDs or multiaddrs.
- Local API and observability listeners remained on container loopback. Only
  explicitly configured Waku ports were published on host loopback.
- Every node used a persistent named volume, bounded health check, restart
  policy, explicit transport port, and private runtime copy of its Docker secret.
- Restart recovered seed network state plus joined peer state in 10 seconds.
- Stopped-node backup/restore verified checksum and preserved Ardents principal,
  device, and Waku peer ID.
- Rolling upgrade created stopped-node backups for all three nodes, recreated
  one node at a time, and required network readiness before continuing. Rolling
  rollback restored the previous image reference under the same gate.
- Production service definitions require immutable image reference, per-node
  versioned operator config, API token, capability-store key, replay key,
  persistent data, read-only root filesystem, dropped capabilities, and
  `no-new-privileges`. Missing material fails Compose interpolation.

## Docker/Linux Evidence

| Check | Result |
| --- | --- |
| PowerShell AST parse in `mcr.microsoft.com/powershell:7.5-debian-12` | 3/3 deployment scripts passed |
| Local and production `docker compose config --quiet` | passed |
| Clean self-formation | passed; seed plus two joined peers, no manual bootstrap value |
| Full-cluster stop/start | passed; seed `ready`, peers `ready` and joined |
| Peer backup/restore | passed with matching subject, device, and Waku peer ID |
| Pre-upgrade backup | 3/3 stopped-node archives accepted |
| Rolling upgrade | passed |
| Rolling rollback | passed |

The first bounded startup attempt exposed Compose's ignored secret `mode` and
the runtime correctly rejected the group-readable mount. The deployment now
copies each secret to a `0600` runtime file. The second attempt exposed selection
of the seed's loopback endpoint; the lifecycle now rejects loopback multiaddrs.
The accepted retry formed the cluster in four seconds. Both failures terminated
within their explicit deadlines and left no false-ready state.

Artifact-level revalidation in STB-605 exposed one additional deployment gap:
the local Docker-in-Docker workload engine was initially wired over plaintext
TCP, which the runtime security contract correctly rejected. The local profile
now shares an isolated Unix socket through a dedicated named volume. The clean
release-bundle quick start formed the complete three-node network in 27.2
seconds with the real workload executor available.

## Privacy Boundary Strengthened During STB-606

STB-604 originally accepted network formation with truthful
`privacy.capability.missing` product degradation. That evidence remains a
record of the earlier gate, but it is no longer the supported quick-start
contract. STB-606 added an offline `ard-provision` boundary and an isolated
local-only authority volume. The current `./ardents.ps1 up` provisions unique,
subject-bound discovery/data grants plus distinct node storage keys while every
node is stopped, imports signed member grants, and then requires all three nodes
to report product readiness. A clean current-code proof completed in 42.6
seconds with seed and both peers `ready`, both peers joined, three signed private
discovery records visible, and the real workload executor available. Production
Compose continues to require external authority and protected capability/replay
material; local authority state is never a production foundation.

## Primary Artifacts

- `docs/deployment-contract.md`
- `docs/qa/e2e/self-forming-deployment.md`
- `docker/docker-compose.multinode.yml`
- `docker/docker-compose.production.yml`
- `ardents.ps1`
- `scripts/deploy/cluster.ps1`
- `scripts/deploy/data.ps1`
- `scripts/deploy/rollout.ps1`
- `cmd/ard-provision/`
- `internal/identity/localrealm/`
- `docker/README.md`
