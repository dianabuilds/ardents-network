# Ardents Network

Ardents is a managed peer-to-peer node for private discovery, messaging, hosted
services, workloads, and encrypted data availability. `v1` uses Waku as its
canonical network foundation and keeps product truth in explicit domains rather
than hiding it behind a generic runtime facade.

> Status: stabilization candidate, not a production release. The implementation
> has real multi-node Waku transport and extensive Docker/Linux evidence, but the
> final adversarial and release gates in the active stabilization
> plan are not complete.

## What The Node Provides

- persistent Ardents and Waku identities;
- Waku Relay, Store, Filter, and Lightpush participation through supported
  transport profiles;
- capability-bound encrypted discovery and data envelopes with durable replay
  protection;
- local workload control and hosted-service publication;
- encrypted blob/manifest storage, replica commitments, repair, and fetch;
- a loopback-only authenticated operator API, CLI/TUI, diagnostics, readiness,
  structured logs, and bounded Prometheus metrics.

Ardents does not implement its own carrier network, silently fall back to
plaintext, expose remote administrative HTTP, or treat an announced/cache copy
as a committed data replica.

## Trust And Security Model

Each node owns a long-lived Ardents signing identity and Waku peer identity.
Private discovery and data exchange require an Identity-owned capability issued
by a trusted realm authority. Possessing retained ciphertext, a network address,
or a bearer token for another node does not grant that authority.

The local API and observability listener bind to loopback only. Deployment
secrets are separate from retained node state. Missing keys, partial restores,
invalid capabilities, untrusted records, corrupt payloads, and failed runtime
proofs fail closed and remain visible through Diagnostics.

Start with:

- [system concept](docs/product/system-concept.md)
- [canonical Waku foundation](docs/protocols/canonical-network-foundation.md)
- [network privacy protocol](docs/protocols/network-privacy-protocol.md)
- [persistent state and key security](docs/security/persistent-state-security.md)
- [deployment contract](docs/operations/deployment-contract.md)

## Distribution Model

The public release contains two executables:

- `ardentsd` is the self-contained node daemon; `ardentsd init` initializes a
  local node, its protected state, operator configuration, and API credential;
- `ardentsctl` is the local operator client and can be installed independently
  on an administrator workstation.

Docker is an optional delivery and workload-execution adapter. A node starts
without Docker by default because `workloads.executor` defaults to `disabled`.
Set it explicitly to `docker` only where the node is allowed to control a
Docker Engine, or to `trusted-process` for the restricted local-development
profile. `ardents-ingress-proxy` is an internal, optional adapter used only for
isolated hosted-service ingress; it is not required to start or operate a node.

For a native Linux node, extract the release archive and run the included
installer. It creates the `ardents` service account, protected state, a systemd
unit, and the initial operator credential:

```sh
sudo ./scripts/install/linux.sh install \
  --node-name node-1 --transport-port 61001
sudo ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --token-file /var/lib/ardents/secrets/api-token node status
```

Use `--bootstrap-peer` during first installation when the node must join an
existing endpoint. Re-running `install` repairs only the same build and
preserves configuration, identity, API credential, and retained data; changing
build identity requires the readiness-gated `upgrade` command. See the
[native Linux installation contract](docs/operations/native-linux-installation.md)
for upgrade and uninstall behavior.

From an administrator workstation with a scoped node credential and OpenSSH
key/agent access, connect without exposing the control API:

```sh
ardentsctl --ssh ops@node.example \
  --ssh-port 22 \
  --ssh-identity ~/.ssh/id_ed25519 \
  --ssh-known-hosts ~/.ssh/known_hosts \
  --token-file ~/.config/ardents/node-1.token \
  --node-name node-1 node status
```

The SSH transport uses the remote node's loopback API and normal OpenSSH host
key verification. It requires non-interactive key or agent authentication.

## Docker Local Quick Start

Requirements: a supported host from [the platform matrix](docs/product/supported-platforms.md),
Docker Engine, Docker Compose v2, and PowerShell 7 for the lifecycle command.

```powershell
./ardents.ps1 up -Build
./ardents.ps1 status
```

The command creates a seed and two peers, generates distinct local credentials,
discovers the seed's real non-loopback Waku endpoint, and forms the network
without manual peer-ID copying. State and private local credentials live under
the git-ignored `var/deployment/local-multinode/` directory.

For local development the command provisions an isolated, local-only realm
authority while all nodes are stopped. Every node receives a distinct
subject-bound discovery/data capability and distinct protected storage keys;
the authority key remains in a separate Docker volume and is never copied into
a node. `up` succeeds only after all three nodes report product readiness, so a
network-only or `privacy.capability.missing` cluster is rejected. Production
never uses this local authority and requires deployment-managed realm material.

Stop without deleting state:

```powershell
./ardents.ps1 stop
./ardents.ps1 start
./ardents.ps1 down
```

See [Docker deployment](deploy/docker/README.md) for backup, restore, upgrade, rollback,
and production service definitions.

## Operator CLI

Inside a node container, use its private runtime token:

```powershell
docker compose -p ardents-local -f deploy/docker/compose/docker-compose.multinode.yml exec seed `
  ardentsctl --token-file /run/ardents/api-token node status
```

Useful groups are `node`, `network`, `diagnostics`, `config`, `workload`, and
`data`. `ardentsctl version` prints the build identity without connecting to a node.
Use `--output json` for automation and explicit `--node-name`, `--principal`,
and scoped contexts where operator identity must be pinned.

## Testing

Canonical tests run in Docker/Linux; Windows is orchestration only.

```powershell
./ardents.ps1 test fast
./ardents.ps1 test integration
```

Run targeted scenarios during development. Slow commands must have explicit
deadlines and trigger CPU/RAM/disk diagnosis instead of being left as unbounded
waits.

## Current Limitations

- single-host Docker Compose is the supported deployment shape; the native
  systemd installer is a qualification candidate with lifecycle acceptance,
  pending distribution-matrix and real-release transition evidence;
- Kubernetes and multi-host schedulers do not have a `v1` support contract;
- remote operator API exposure is unsupported;
- private capability issuance is an external realm-authority operation;
- QUIC, WebTransport, and WebRTC are suppressed in supported profiles;
- Linux arm64 requires a native CGO build/qualification runner and is not yet a
  release target;
- tag releases receive GitHub OIDC/Sigstore artifact and SBOM attestations, but
  adversarial QA and broader release acceptance remain open in the
  stabilization plan.

Security exceptions and upgrade triggers are recorded in
[security exceptions](docs/security/security-exceptions.md). Do not delete retained keys
or databases to “repair” a node; follow the [operator runbook](docs/operations/operator-runbook.md).

Ardents Network is distributed under the [MIT License](LICENSE). Third-party
components retain their respective licenses.
