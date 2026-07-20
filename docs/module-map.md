# Repository Module Map

The repository has four different kinds of surface. Keeping them separate is a
design constraint, not a naming preference.

| Path | Owns | Public interface |
| --- | --- | --- |
| `internal/` | Product domain implementations and Node Runtime composition | Go contracts owned by each domain |
| `boundary/` | Product adapters for human or machine callers | `ard` CLI/TUI and local control client |
| `docker/` | Declarative container images, Compose topologies, and observability assets | Docker/Compose configuration only |
| `scripts/deploy/` | Supported Docker deployment lifecycle implementation | `./ardents.ps1 up/status/start/stop/down/backup/restore/upgrade/rollback` |
| `scripts/release/` | Reproducible packaging, metadata, and artifact smoke implementation | `./ardents.ps1 package <version>` |
| `tests/` | Canonical Docker/Linux test orchestration and QA evidence | `./ardents.ps1 test <suite>` or `tests/run.ps1` |
| `.github/workflows/` | CI scheduling and artifact retention | Calls canonical repository interfaces; owns no test selection semantics |

## Operator Path

`ardents.ps1` is the repository-level operator interface. A caller should not
need to know which implementation script owns cluster formation, backup, or
rollout. The deployment implementation may use several focused files, but those
files are not parallel public interfaces.

The supported local result of `./ardents.ps1 up` is one seed, two service peers,
a real Waku network, persistent node state, distinct local credentials, and an
isolated Docker-in-Docker workload engine. `./ardents.ps1 status` reports
network and lifecycle truth. The local profile provisions a real isolated
local-only realm while nodes are stopped; production still requires an
external deployment-managed authority and never reuses local realm material.

## Boundary Is Not Deployment

`boundary/cli` is a product adapter at the local-control seam. Its command files
translate operator intent into canonical product contracts; they do not own
Docker, CI, release, or process orchestration. Product domains must not depend
on `boundary/*`.

## CI Rule

CI may distribute canonical commands across workers and retain their outputs.
It must not reimplement package selection, scenario metadata, reporting,
deployment bootstrap, or release identity logic in workflow YAML.
