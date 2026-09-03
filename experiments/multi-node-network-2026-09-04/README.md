# Multi-node pilot — 2026-09-04

## Question

Does the closed-alpha `ardents-node` source-server pair plus `ardents
refresh-sources` consumer converge identically across six independent
consumer containers when they all consume from the same two published
source servers? The pilot is a 1:1 scale-up of the maintained
`TestFiniteSourceCommandsAsBlackBoxProcesses` test, isolated into a real
Linux Docker network so the convergence property is exercised by six
independent operating-system processes, not by six goroutines in one
process.

This experiment does **not** claim:

- C0 release qualification, hostile-load resistance, or hostile-network
  resilience.
- A specific alpha corpus target or service target. The convergence fact
  is "all six consumers saw the same `source-wave-accepted` event", not
  "all six consumers resolved a specific Service Target".
- Track-B work, service-level or application-level correctness, multi-host
  qualification, VPS qualification, or the alpha-control bundle.
- A public or hostile two-source deployment. The source servers and the
  authority keypair are owned and pre-baked by the pilot itself.

## Hypothesis

- **H1:** The two source servers accept the pre-baked State and emit a
  `source-ready` event within five seconds of container start.
- **H2:** All six consumer containers, started after the source servers
  pass their TCP healthcheck, complete a single `refresh-sources --once`
  wave and emit a `source-wave-accepted` event with identical
  `generation`, `epoch`, `source_outcomes`, and `latest_completeness`
  fields.
- **H0:** The consumer contract rejects pre-baked source plans, the
  source server's per-instance local-role state root is non-canonical, or
  the alpha corpus authority path is not what `accept-offline` actually
  consumes.

## Method

A disposable Docker Compose spike (this directory) with one Docker network
of twelve services:

| Service     | Role                                                              | Lifecycle              |
|-------------|-------------------------------------------------------------------|------------------------|
| `builder`   | Cross-compiles `ardents`, `ardents-node`, `test-driver` for linux/amd64. | one-shot, profile `build` |
| `prebake`   | Runs `test-driver prebake` to write fixtures, certs, plans.       | one-shot               |
| `source-a`  | Long-running `ardents-node source --config …/source-a.json`.     | long-running, healthcheck on `127.0.0.1:4101` |
| `source-b`  | Long-running `ardents-node source --config …/source-b.json`.     | long-running, healthcheck on `127.0.0.1:4102` |
| `clock-tick` | Keeps the shared clock-observation file fresh every 500 ms.    | long-running, healthcheck on marker freshness |
| `node-1..6` | One-shot `ardents refresh-sources --once`, writes event JSON.    | one-shot each          |
| `test-driver` | Runs `test-driver verify` and reads all six event JSONs.       | one-shot, depends on all six `node-*` |

Resource caps (per container, Compose v2 `deploy.resources.limits`):

| Container    | Memory | CPU  |
|--------------|--------|------|
| builder      | 4 GB   | 4.0  |
| prebake      | 512 MB | 0.5  |
| source-a / b | 512 MB | 0.8  |
| clock-tick   | 128 MB | 0.1  |
| node-1..6    | 256 MB | 0.4  |
| test-driver  | 256 MB | 0.3  |
| **Active pilot peak** | **~2.7 GB** | **~4.1** |

The active peak is the two sources, clock owner, and six consumers; the
4 GB / 4 CPU builder completes before they start. These are hard caps rather
than expected consumption. The pilot is sized for the local Docker host and
must not be moved to a resource-constrained VPS.

## Inputs and outputs

- **Inputs:** the maintained Go tree at HEAD of `main` and the four
  accepted `ardents`/`ardents-node`/`ardents-control`/`ardents-custody`
  commands.
- **Outputs** (in `ARDENTS_PILOT_EVIDENCE_DIR`, which must be outside the
  repository):
  - `artifacts/` — built Linux/amd64 binaries and their `SHA256SUMS`.
  - `fixtures/` — the pre-baked State root (`generations/<gen>/…`),
    `source-a.json`, `source-b.json`, `client.json`, `source-ca.pem`,
    `source-a.pem` + `source-a-key.pem`, `client.pem` + `client-key.pem`,
    and the live `clock.observation` marker.
  - `state/node-N/` — each consumer's per-run state root.
  - `nodes/node-N.json` — each consumer's emitted `source-wave-accepted`
    event.
  - `pilot-convergence.json` — machine-readable verdict.
  - `pilot-verdict.md` — human-readable verdict and per-node table.

## Falsification

The experiment fails if:

- The `prebake` service exits non-zero (fixture, cert, or plan write
  failure).
- A source server fails its TCP healthcheck within 30 retries.
- Fewer than six `node-N.json` files appear in `nodes/`.
- Any `node-N.json` cannot be parsed as a `source-wave-accepted` event.
- The six consumer events do not all reduce to a single distinct result
  set.
- The converged `generation` does not equal the pre-bake
  `fixtures/current` generation.

## Disposition

After the run, write the run's `pilot-verdict.md` and
`pilot-convergence.json` artefacts to the agreed evidence dir and tag the
slice as `implemented, not accepted` until Codex + owner review. The pilot
is a one-time, single-network shape: slice 2 (adversary + multi-scenario)
will reuse the build-ignored driver files and the compose topology but add a scripted
adversary container and at least one negative scenario.

## How to run

```bash
cd experiments/multi-node-network-2026-09-04
export ARDENTS_PILOT_EVIDENCE_DIR="$(mktemp -d)"
docker compose --profile build up --detach
driver_status="$(docker wait ardents-multi-node-pilot-test-driver-1)"
docker compose --profile build logs test-driver
docker compose --profile build down
test "$driver_status" -eq 0
```

On PowerShell, create a local temporary directory and set the same
variable:

```powershell
$env:ARDENTS_PILOT_EVIDENCE_DIR = Join-Path $env:TEMP "ardents-multi-node-pilot-$PID"
New-Item -ItemType Directory -Path $env:ARDENTS_PILOT_EVIDENCE_DIR
docker compose --profile build up --detach
$driverStatus = docker wait ardents-multi-node-pilot-test-driver-1
docker compose --profile build logs test-driver
docker compose --profile build down
if ([int]$driverStatus -ne 0) { throw "pilot failed with exit code $driverStatus" }
```

The external evidence directory survives `docker compose down`; generated
logs and binaries never enter the repository.

## Self-test

`test-driver self-test` builds one synthetic fixture, writes six
identical synthetic consumer events, runs `VerifyConvergence`, and exits
non-zero if the convergence logic accepts fewer than six reports or
rejects a six-of-six match. Run it after every change to
`cmd/test-driver/convergence.go`:

```bash
bash ./experiments/multi-node-network-2026-09-04/test.sh
```

On PowerShell:

```powershell
.\experiments\multi-node-network-2026-09-04\test.ps1
```

## Scope of slice 1

- 2 source servers, 6 consumers, 1 test-driver. No adversary.
- Single smoke scenario: `alpha_round_trip` (all six consumers converge on
  the same `source-wave-accepted` event).
- No scripted source mutation, no service-instance workload, no rendezvous
  leg traffic, no OHTTP relay.
- The compose file and `cmd/test-driver/convergence.go` are written so slice
  2 can add `cmd/adversary/main.go`, a second scenario, and one
  per-attack assertion without changing the topology.
