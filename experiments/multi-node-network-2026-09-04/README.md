# Multi-node pilot — 2026-09-04

## Question

Can the closed-alpha `ardents-node` source surface and `ardents
refresh-sources` consumer satisfy both parts of one six-consumer pilot?

1. In `alpha_round_trip`, do six independent consumers converge on the same
   State from two honest source servers?
2. In `adversary_rejected`, do five honest consumers retain that outcome while
   one designated probe rejects the same State body re-signed by an
   unauthorized authority, with all six still reporting the expected
   content-addressed generation?

The pilot is a scale-up of the maintained
`TestFiniteSourceCommandsAsBlackBoxProcesses` test, isolated into a real Linux
Docker network so each consumer is an independent operating-system process.
Slice 1 retains the honest-only scenario; slice 2 is the default compose run.

This experiment does **not** claim:

- C0 release qualification, hostile-load resistance, or hostile-network
  resilience.
- A specific alpha corpus target or service target. The convergence fact
  is "all six consumers saw the same `source-wave-accepted` event", not
  "all six consumers resolved a specific Service Target".
- Track-B work, service-level or application-level correctness, multi-host
  qualification, VPS qualification, or the alpha-control bundle.
- A public or generally hostile source deployment. All source processes,
  certificates, and authority keys are owned and pre-baked by the pilot. The
  adversary is one scripted forged-signer injection, not an independent
  operator or a general network attacker.

## Hypothesis

- **H1:** The two honest sources and one forged-signer source accept their
  pre-baked State and become ready within the bounded healthcheck window.
- **H2:** `node-1..node-5` each emit exactly `[valid, valid, not-attempted,
  not-attempted]` for source outcomes.
- **H3:** `node-6`, and only `node-6`, emits exactly `[valid, invalid-state,
  not-attempted, not-attempted]`, demonstrating rejection at the Epoch
  authority boundary rather than at TLS or framing.
- **H4:** All six parsed events report the pre-baked generation and reduce to
  one distinct result set despite the probe rejecting the forged source.
- **H0:** A source cannot serve the prepared State, consumers diverge or
  accept the unauthorized signer, or the harness cannot distinguish the
  intended rejection from a transport/setup failure.

## Method

A disposable Docker Compose spike (this directory) with one Docker network
of thirteen services:

| Service     | Role                                                              | Lifecycle              |
|-------------|-------------------------------------------------------------------|------------------------|
| `builder`   | Cross-compiles `ardents`, `ardents-node`, `test-driver` for linux/amd64. | one-shot, profile `build` |
| `prebake`   | Runs `test-driver prebake_adversary` to write both scenario fixtures, certs, and plans. | one-shot |
| `source-a`  | Long-running `ardents-node source --config …/source-a.json`.     | long-running, healthcheck on `127.0.0.1:4101` |
| `source-b`  | Long-running `ardents-node source --config …/source-b.json`.     | long-running, healthcheck on `127.0.0.1:4102` |
| `adversary` | Long-running `ardents-node source --config …/source-c.json` serving the forged-signer State. | long-running, healthcheck on `127.0.0.1:4103` |
| `clock-tick` | Keeps the shared clock-observation file fresh every 500 ms.    | long-running, healthcheck on marker freshness |
| `node-1..5` | One-shot honest-plan `ardents refresh-sources --once`, writes event JSON. | one-shot each |
| `node-6` | One-shot probe-plan refresh against source-b plus source-c, writes event JSON. | one-shot |
| `test-driver` | Runs `test-driver verify_adversary` and reads all six event JSONs. | one-shot, depends on all six `node-*` |

Resource caps (per container, Compose v2 `deploy.resources.limits`):

| Container    | Memory | CPU  |
|--------------|--------|------|
| builder      | 4 GB   | 4.0  |
| prebake      | 512 MB | 0.5  |
| source-a / b / adversary | 512 MB | 0.8  |
| clock-tick   | 128 MB | 0.1  |
| node-1..6    | 256 MB | 0.4  |
| test-driver  | 256 MB | 0.3  |
| **Active pilot peak** | **~3.2 GB** | **~4.9** |

The active peak is the three sources, clock owner, and six consumers; the
4 GB / 4 CPU builder completes before they start. These are hard caps rather
than expected consumption. The pilot is sized for the local Docker host and
must not be moved to a resource-constrained VPS.

## Inputs and outputs

- **Inputs:** the selected maintained Go tree and the accepted `ardents` and
  `ardents-node` commands used by this pilot.
- **Outputs** (in `ARDENTS_PILOT_EVIDENCE_DIR`, which must be outside the
  repository):
  - `artifacts/` — built Linux/amd64 binaries and their `SHA256SUMS`.
  - `fixtures/` — the pre-baked State root (`generations/<gen>/…`),
    `source-a.json`, `source-b.json`, `source-c.json`, `client.json`,
    `client-probe.json`, the honest and forged epoch inputs, private per-run
    key material, and the live `clock.observation` marker.
  - `source-{a,b,c}-state/` — the three source-server State roots populated
    through the production `accept-offline` command.
  - `state/node-N/` — each consumer's per-run state root.
  - `nodes/node-N.json` — each consumer's emitted `source-wave-accepted`
    event.
  - `pilot-adversary-convergence.json` — machine-readable slice-2 verdict.
  - `pilot-adversary-verdict.md` — human-readable slice-2 verdict and per-node
    tables. The legacy slice-1 verifier writes `pilot-convergence.json` and
    `pilot-verdict.md` when invoked explicitly.

## Falsification

The experiment fails if:

- The `prebake` service exits non-zero (fixture, cert, or plan write
  failure).
- Any of the three source servers fails its TCP healthcheck within 30 retries.
- Fewer than six `node-N.json` files appear in `nodes/`.
- Any `node-N.json` cannot be parsed as a `source-wave-accepted` event.
- Any `node-1..node-5` outcome differs from `[valid, valid, not-attempted,
  not-attempted]`.
- `node-6` differs from `[valid, invalid-state, not-attempted,
  not-attempted]`, including rejection at TLS/framing instead of the intended
  State-signature boundary.
- Any parsed generation differs from `fixtures/current`, or the six events do
  not reduce to one distinct result set.
- The final counters differ from honest=5, probe=1, distinct=1,
  generation-mismatches=0, mismatched-signatures=0, parse-errors=0.

## Result and disposition

- **Current result:** a 2026-09-04 local Docker Desktop run after verifier
  hardening and the driver file split completed with test-driver exit 0. Five
  honest consumers and one probe converged on generation
  `f046fcc55118fbe041a2206ae8f5aa76a2de99e2c833697c6e19cd4474a8df87`;
  distinct results=1 and generation/signature/parse mismatch counts=0. The
  external `pilot-adversary-verdict.md` SHA-256 is
  `E6A2F2574AB5AC37F9B913D12BF0C2217750927E1C201C5890A0583FC6AC2448`.
  This is implementation evidence; Product Owner acceptance remains pending.
- **Captured evidence contract:** retain the external evidence directory with
  all six node logs, `pilot-adversary-convergence.json`,
  `pilot-adversary-verdict.md`, source/prebake logs, and artifact checksums.
  Generated evidence, keys, State, and binaries are never committed.
- **Disposition:** retain the experiment as a one-network, one-generation
  falsification harness. It does not qualify a VPS, independent operation, a
  generally hostile network, or multi-generation roll-forward.

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

`test-driver self-test` first exercises the slice-1 six-consumer convergence
path, then runs N1–N9 against `VerifyAdversaryScenario`: one accepted fixture
and eight independently mutated fixtures that must reject, including an
overlength `source_outcomes` array. Run it after every verifier or event-reader
change:

```bash
bash ./experiments/multi-node-network-2026-09-04/test.sh
```

On PowerShell:

```powershell
.\experiments\multi-node-network-2026-09-04\test.ps1
```

## Scope of slice 1 (alpha_round_trip)

- 2 source servers, 6 consumers, 1 test-driver. No adversary.
- Single smoke scenario: `alpha_round_trip` (all six consumers converge on
  the same `source-wave-accepted` event).
- No scripted source mutation, no service-instance workload, no rendezvous
  leg traffic, no OHTTP relay.
- **Status:** superseded by slice 2 for the multi-node harness. Slice 1
  plans and code paths are still runnable via the `prebake` + `verify`
  subcommands; the e2e compose profile runs slice 2 by default.

## Scope of slice 2 (adversary_rejected)

This is the **implemented default** run. The same six-node compose
topology is reused; one new source service and one new consumer
classification are layered on top.

- **Sources:** 2 honest source servers (`source-a`, `source-b`) plus 1
  adversary source (`source-c`). All three are the same `ardents-node
  source` binary against a pre-baked State root.
- **Adversary architecture:** the adversary container serves the SAME
  content-addressed State body re-signed with an attacker-controlled
  ed25519 key. It reuses `source-a`'s mTLS leaf chain so the probe
  consumer's `leaf_key_digest` pin still matches. The forge becomes
  visible only when the source client checks the Epoch signature against
  the consumer's `authority_public` list — which contains only the real
  authority — and the probe reports a non-valid outcome for source-c.
- **Consumers:** 5 honest consumers (`node-1..node-5`) using
  `client.json` (source-a + source-b) plus 1 probe consumer (`node-6`)
  using `client-probe.json` (source-b + source-c).
- **How to run:** use the canonical detached `docker compose`, `docker wait`,
  log capture, and `docker compose down` sequence above. The compose
  `test-driver` service runs `test-driver verify_adversary
  /workspace/evidence` and exits 0 only when the slice-2 acceptance
  criteria pass.
- **Acceptance criteria** enforced by
  `VerifyAdversaryScenario` in `cmd/test-driver/verify_adversary.go`:
  - exactly 6 node reports under `nodes/`
  - all 6 reported `generation` values equal the prebake
    `fixtures/current` generation
  - exactly 5 honest and 1 probe by exact-match on the
    `source_outcomes` array (honest: `[valid, valid, not-attempted,
    not-attempted]`; probe: `[valid, invalid-state, not-attempted,
    not-attempted]`)
  - distinct result sets = 1
  - the probe is `node-6` and the honest set is exactly `node-1..node-5`
- **Local self-test:** `test.sh` and `test.ps1` run both the slice-1 path (6
  honest consumers through `VerifyConvergence`) and the slice-2 path (N1–N9
  through `VerifyAdversaryScenario`: 1 happy + 8 negative).
- **Known limits:** this slice exercises a single generation;
  multi-generation roll-forward and cross-network isolation are deferred.
