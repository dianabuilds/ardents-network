# Ardents Docker Assets

The supported `v1` target is Docker Engine plus Docker Compose v2 on one Linux
host. See `docs/deployment-contract.md` for the security and lifecycle contract.
This directory is declarative: Dockerfiles, Compose definitions, and
observability assets. Operator commands live behind the root `ardents.ps1`
interface; their implementation is under `scripts/deploy/`.

## Self-Forming Local Cluster

From the repository root:

```powershell
./ardents.ps1 up -Build
./ardents.ps1 status
```

The lifecycle command generates three distinct API credentials under the
git-ignored `var/deployment/local-multinode/` directory, starts the seed, reads
its authenticated canonical Waku endpoint, and starts both peers with that
endpoint. No peer ID or multiaddr is copied manually.

The local profile also starts a disposable shared Docker-in-Docker workload
engine. Ardents reaches it through a Unix socket in a dedicated named volume;
the profile never exposes a plaintext Docker TCP endpoint and never mounts the
host Docker socket. The engine is a development boundary and is removed only
when the operator explicitly removes deployment volumes.

Useful lifecycle commands:

```powershell
./ardents.ps1 stop    # keep containers and volumes
./ardents.ps1 start   # restart and re-prove network participation
./ardents.ps1 down    # remove containers, retain volumes and state
./ardents.ps1 down -RemoveVolumes # remove containers and project volumes
```

Stopped-node continuity operations are explicit:

```powershell
./ardents.ps1 backup -Node peer2
./ardents.ps1 restore -Node peer2 -Archive <archive> -ConfirmReplace
```

Restore verifies the archive checksum and then proves that Ardents principal,
device, and Waku peer identity match the backup manifest. Upgrade and rollback
recreate one node at a time and re-prove network participation:

```powershell
./ardents.ps1 upgrade -NewImage registry/ardents@sha256:<digest>
./ardents.ps1 rollback
```

`docker-compose.multinode.yml` publishes only Waku TCP ports on host loopback.
The operator API and observability listener stay on container loopback; inspect
them through `docker compose exec`, as the lifecycle command does.

## Production Service Definition

`docker-compose.production.yml` is the hardened service definition for the
supported target. It requires:

- an immutable `ARDENTS_IMAGE` reference, preferably a registry digest;
- separate versioned operator-config files for every node, including explicit
  advertised/bootstrap addresses and required private channel references;
- separate external API token, capability-store key, and replay-key files for
  every node; config files reference their copied `/run/ardents/` paths;
- an external TLS-authenticated Docker workload endpoint plus separate client
  CA/certificate/key files for each node; plaintext Docker API and host socket
  mounts are not supported in production;
- deployment-managed backup and secret handling.

It drops Linux capabilities, enables `no-new-privileges`, uses a read-only root
filesystem with a bounded temporary filesystem, retains data in named volumes,
and publishes transport ports only. It intentionally does not generate
production credentials or invent public addresses.

The production file can be validated without starting services after required
values are supplied:

```powershell
docker compose -f docker/docker-compose.production.yml config --quiet
```

## Testnet Topology

`docker-compose.testnet.yml` and `tests/run-multihost.ps1` remain adversarial QA
artifacts. They exercise segmented networks, WSS, partitions, churn, and Store
recovery; they are not the operator quick-start surface. Service nodes share a
test-only, volume-isolated Docker-in-Docker workload engine so the topology uses
the real Docker executor without exposing the host Docker socket. Production
continues to require the external TLS-authenticated workload endpoint described
above.
