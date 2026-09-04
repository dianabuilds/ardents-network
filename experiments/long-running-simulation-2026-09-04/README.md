# R-138 long-running simulation

Status: **S3.1 smoke and S3.6 mock-user code are implemented and pass the
hermetic pre-flight; neither slice is accepted.** The Docker-backed 100-tick
run and external evidence have not been run. No production-coordination,
substitute-user, availability, load, or privacy claim follows from this
experiment.

This experiment answers R-138 from `docs/research/questions.md`: can a
long-running multi-actor simulation keep the slice 2 invariants green across
hundreds of accelerated ticks, and surface deviations before the consumer-side
verifier would? It is the natural extension of the slice 1/2 multi-node pilot
from one-shot `prebake → consume → verify` to a continuous simulation where
state evolves, actors take per-tick actions, and an Observer judges the
artefacts every tick.

## Roles in scope (S3.1, smoke slice)

| Role | S3.1? | How realised |
|------|-------|--------------|
| HonestSource | yes | two `ardents-node source` containers on the 172.32.0.0/24 bridge (source-a at 172.32.0.10:4101, source-b at 172.32.0.11:4102), both serving the same pre-baked State |
| HonestConsumer | yes | one `ardents refresh-sources` subprocess, invoked per tick by the sim-driver container |
| Authority | not in S3.1 | folded into prebake; one static ed25519 key |
| Adversary | not in S3.1 | deferred to S3.3 |
| Timekeeper | yes | goroutine inside the sim-driver container |
| Observer | yes | goroutine inside the sim-driver container |
| DriftInjector | not in S3.1 | deferred to S3.2 |

**Why two source containers (not one).** The production source plan at
`cmd/ardents/source_plan.go:93` rejects any plan with
`len(plan.Sources) != 2` ("source plan is not canonical or complete"),
so every `ardents refresh-sources` invocation — including the
sim-driver's per-tick consumer — must dial exactly two sources. The
original "1 source" wording in this role table was therefore overridden
by this product constraint, and the fix lives in the contract (this
README + the slice-internal `docker-compose.yml`, which starts both
source-a and source-b), not in the implementation. Both containers serve
the same pre-baked State and never diverge across the 100-tick run; that
identity is the S3.1 no-drift invariant. The sim-driver role itself
remains a single consumer + single Timekeeper + single Observer inside
one container.

S3.1 is the smoke test: a 100-tick run with 2 source containers + 1
sim-driver container, on a 172.32.0.0/24 bridge, no adversary, no drift.
Generation does not change across ticks. The Observer must accept every
tick; if any trip-wire fires the run aborts.

## Slice plan

| Slice | Adds | Files (+/-) | LOC delta | Depends on |
|-------|------|-------------|----------:|------------|
| **S3.1** (implemented; pre-flight green, Docker smoke unrun) | Tick loop, Timekeeper, Observer, 2 HonestSource containers, 1 HonestConsumer subprocess, trip-wire catalog | +7 new | ~500 | slice 2 pilot |
| **S3.6** (implemented; pre-flight green, Docker smoke unrun) | UserActor with 3 personas (honest, confused, impatient) + in-memory credential store + per-tick action selection + user-action trip-wires. **Mock state, no real `ardents` CLI calls in S3.6** | +3 new, ~3 modified | ~600 | S3.1 |
| S3.2 | DriftInjector (consumer churn + multi-generation roll-forward via Authority) | +2 new, ~1 modified | ~600 | S3.1, S3.6 |
| S3.3 | Adversary with scripted policy (forge / replay-old / withhold / split-view) | +2 new, ~1 modified | ~700 | S3.2 |
| S3.4 | Auditor agent + 4–5 more trip-wires + mutation tests on the Observer | +1 new, ~1 modified | ~400 | S3.3 |
| S3.5 | Multi-source + consumer diversity + cross-network isolation tests | +2 new, ~1 modified | ~500 | S3.4 |
| S3.7 | Hostile user + churn user + transition from mock state to real `ardents` CLI shell-out for user actions | +2 new, ~1 modified | ~500 | S3.6 |
| S3.8 | Operator role (sign_next_epoch, rekey_authority, freeze_consumer) | +2 new, ~1 modified | ~500 | S3.7 |

Each slice ≤ 1500 LOC production, ≤ 15 files, single new `cmd/<name>` per
slice, single new top-level Go file per role. Per-slice the rule is "one
new capability, one new file".

## S3.1 contract

### Question
Can two identical honest Source containers + one consumer + Timekeeper +
Observer, running 100
accelerated ticks at 100 ms wall-clock per tick, with no adversary and no
drift, keep all slice 2 invariants green on every tick?

### Hypothesis
Yes. The slice 2 invariant set is robust to repeated refresh cycles on a
single generation when no other actor is perturbing the system. The Observer
surfaces exactly the same checks the slice 2 verifier did, applied per tick
instead of one-shot.

### Falsification
The simulation is falsified if any of:
- a tick's `tick.json` exits non-zero on the consumer side
- a tick's Observer verdict is `accept=false` for a reason other than the
  trip-wire test (which the smoke slice disables)
- the source container exits non-zero at any point
- two consecutive ticks have different `state.observed_digest` from the
  consumer's perspective (no drift means the digest must be stable)
- wall-clock for the 100-tick run exceeds 30 s (10× the configured budget)

### Method
- `cmd/sim-driver/main.go` is the simulation driver. It owns the tick loop,
  Timekeeper, and Observer. It shells out to `ardents refresh-sources` once
  per tick and reads the resulting event log.
- `docker-compose.yml` (slice-internal) starts two `ardents-node source`
  containers bound to an internal `172.32.0.0/24` subnet (deliberately
  different from the slice 2 pilot's `172.30.0.0/24` so the two harnesses
  can run side by side without collision).
- The driver reuses the prebake machinery from the slice 2 pilot verbatim
  (calling `cmd/test-driver/prebake.go` as a library is the cheapest path;
  if Go package import proves too entangled, copy the minimum needed into
  `cmd/sim-driver/fixtures.go` and document the duplication in a comment).
- The Observer reads the same JSON event shape the slice 2 verifier
  consumes and reuses the verdict structure where it fits.

### Acceptance criteria

| AC | Verification |
|----|--------------|
| AC1 Build green | `bash ./test.sh` exits 0 and prints `build OK`. The build-ignored driver files are deliberately passed as explicit paths. |
| AC2 Vet green | `bash ./test.sh` exits 0 and prints `vet OK`; generic `go vet ./...` intentionally does not include the build-ignored driver. |
| AC3 Self-test green | `bash ./test.sh` exits 0 and prints PASS for the S3.1 and S3.6 self-tests (18 PASS lines currently). |
| AC4 100-tick smoke green | **Unrun; not accepted.** With `ARDENTS_LONGRUN_EVIDENCE_DIR` set to an existing directory outside the repository, `docker compose --profile build up --abort-on-container-exit --exit-code-from sim-driver` runs 100 ticks. Evidence must show every `tick.json` has `verdict=accept`, `state.observed_digest` is constant, and wall-clock ≤ 30 s. |
| AC5 Trip-wire catalog present | the catalog has at least 4 trip-wires (generation drift, source exit, consumer parse error, tick budget) and each is unit-tested in self-test |
| AC6 Out of simulation scope | The `sim` network is Compose `internal: true`; only the ephemeral builder uses the separate `build` network to download modules. Before accepting a Docker run, inspect the Compose-created `sim` network and record `Internal=true`, record that `prebake`, both sources, and `sim-driver` attach only to `sim`, and verify `docker ps` shows no production containers or `172.30.0.0/24` network. This is the egress-negative check for the simulation services. |
| AC7 No modification of slice 2 | `git status -s` shows slice 2 files (`experiments/multi-node-network-2026-09-04/`) unchanged |

### Required output format (after an S3.1 acceptance run)

In the final assistant message:
1. baseline SHA + итоговый SHA (uncommitted, no delta)
2. список созданных файлов
3. каждый AC → конкретный тест или evidence (file:line + actual run output)
4. выполненные команды + точные результаты
5. известные ограничения
6. подтверждение отсутствия изменений вне scope (no slice 2 files modified; no internal/cmd-ardents/cmd-ardents-node/ADRs touched)
7. end with literal "implemented, not accepted"

### Out of scope for S3.1 (explicit)
- Adversary (S3.3)
- DriftInjector (S3.2)
- Multi-source, multi-consumer (S3.5)
- Cross-network isolation (S3.5)
- Mutation tests on the Observer (S3.4)
- Auditor agent (S3.4)
- UserActor / user simulation (implemented as S3.6 mock state; not accepted)
- `go test -race` (Windows host cannot run `-race`; defer to carrier lab)
- any modification to slice 2 files
- any modification to `internal/`, `cmd/ardents/`, `cmd/ardents-node/`
- any modification to ADRs or R-NNN records (R-138 entry in
  `docs/research/questions.md` is the only docs change, and it was made by
  the spawner, not the implementer)

## S3.6 contract

### Question
Can a UserActor with three personas (honest, confused, impatient), each
emitting 0–1 user action per tick over a 100-tick accelerated run, keep the
S3.1 network invariants green (no trip-wires, all consumer ticks
`verdict.accept=true`, constant `observed_digest`) while the in-memory
credential store accumulates a realistic-looking sequence of Service
Instance lifecycle events? Does the Observer's user-action trip-wire
catalog flag impossible action sequences (e.g. `publish` without a prior
`open` on the same Service Instance)?

### Hypothesis
Yes. Honest persona runs a scripted 4-step lifecycle
(init → accept → headless_start → open → publish → withdraw) per Service
Instance, with a 10-tick cooldown between actions. Confused persona runs
the same lifecycle but with 5% of actions replaced by
"impossible" sequences (e.g. publish before open, withdraw on a SI not
owned, enroll a Device with an unknown authority). Impatient persona runs
the lifecycle with no cooldown. The Observer flags every impossible
sequence as a `user_impossible_action` trip-wire but does not abort the
run (the trip-wire is informational, not fatal). Network invariants stay
green because user actions are **mock** — they do not invoke the real
`ardents` CLI, do not change the State served by source-a/source-b, and
do not affect the consumer's `observed_digest`.

### Falsification
- any tick the network invariants fail (consumer `verdict.accept=false`,
  `observed_digest` changes unexpectedly, source container exits)
- any honest-persona action fails that should succeed (e.g. publish
  refused when the SI is in `open` state)
- confused persona's "impossible" rate is not 5% ± 2% over 100 ticks
- impatient persona emits more than 1 action per tick
- 100 ticks not completed within 60 s wall-clock (relaxed from S3.1's 30 s
  because user actions add ~20 ms/tick)

### Method
- Add a new in-memory credential store at `cmd/sim-driver/credentials.go`
  that tracks, per persona: `[owned_service_instances, device_state,
  retry_count, last_action_tick]`. No persistence. No real `ardents` CLI
  calls.
- Add a new persona catalog at `cmd/sim-driver/personas.go` with three
  action distributions:
  - `honestPersona`: pre-scripted sequence of 4 actions per SI, 10-tick
    cooldown, max 1 action/tick
  - `confusedPersona`: like honest but with 5% probability of swapping
    the action for an "impossible" variant
  - `impatientPersona`: like honest but cooldown = 0
- Add a new UserActor at `cmd/sim-driver/useractor.go` that on each tick
  selects one persona at random (with configurable weights; default
  1/3 each), invokes the persona's `NextAction(tick)`, and records the
  action to a per-tick `user_actions.jsonl` log under the evidence dir.
- Extend `cmd/sim-driver/main.go` to invoke UserActor before the consumer
  step on every tick.
- Extend `cmd/sim-driver/observer.go` with 2 new trip-wires:
  - `user_impossible_action` (informational; does not abort; counts
    occurrences per persona)
  - `user_retry_storm` (fires if any persona emits >5 actions in 10
    ticks; informational)
- Extend `cmd/sim-driver/selftest.go` with 5 new test cases:
  - `honest_lifecycle_4_steps`: honest persona produces init → accept →
    headless_start → open in 4 consecutive ticks (no cooldown in
    self-test mode)
  - `confused_impossible_rate`: 1000 simulated ticks, count impossible
    actions, assert 4%–6%
  - `impatient_no_cooldown`: impatient persona emits 1 action per tick
    for 10 ticks
  - `user_publish_without_open`: trip-wire fires on confused
    `publish_before_open` event
  - `credential_store_isolation`: two personas owning the same SI ID is
    rejected by the store

### Acceptance criteria for S3.6

| AC | Verification |
|----|--------------|
| AC1 Build green | `bash ./test.sh` exits 0 and prints `build OK`; it passes the ten build-ignored driver files explicitly. |
| AC2 Vet green | `bash ./test.sh` exits 0 and prints `vet OK`; generic `go vet ./...` intentionally does not include build-ignored driver files. |
| AC3 Self-test green | `bash ./test.sh` exits 0 and prints PASS for the 5 new user cases plus all 13 S3.1 cases = 18 PASS lines |
| AC4 100-tick smoke green | `docker compose --profile build up` runs 100 ticks, all 100 `tick.json` show `verdict.accept=true` (network layer), and the new `user_actions.jsonl` shows ≥ 30 user actions across the 3 personas (≈1 every 3 ticks on average). Wall-clock ≤ 60 s. Trip-wire `user_impossible_action` may fire (confused persona) but is informational; trip-wires `generation_drift` / `source_exit` / `consumer_parse_error` / `tick_budget` must not fire. |
| AC5 User trip-wire catalog | 2 new wires (`user_impossible_action`, `user_retry_storm`), each unit-tested in self-test. The `user_impossible_action` is informational (does not abort the run); all other wires remain fatal. |
| AC6 Persona rates | honest emits actions in the expected 4-step sequence; confused has impossible-action rate 4–6 % over 1000 simulated ticks; impatient emits exactly 1 action per tick over 10 ticks (no cooldown) |
| AC7 No modification of S3.1 files or slice 2 | `git status -s` shows no M or new files under `experiments/multi-node-network-2026-09-04/` or under S3.1's existing files (the only new files are `cmd/sim-driver/useractor.go`, `cmd/sim-driver/personas.go`, `cmd/sim-driver/credentials.go`, and modifications to `main.go`, `observer.go`, `selftest.go`, `README.md` inside S3.6's dir) |
| AC8 No real `ardents` CLI calls | `grep -r "exec.Command" cmd/sim-driver/useractor.go cmd/sim-driver/personas.go cmd/sim-driver/credentials.go` returns 0 matches (no shell-out, no CLI invocation — S3.6 is pure mock state) |

### Out of scope for S3.6 (explicit)
- Real `ardents` CLI shell-out (deferred to S3.7)
- Hostile user (S3.7)
- Churn user (S3.7)
- Operator role (S3.8)
- Persistence of credential store across runs (deferred — in-memory only)
- DriftInjector (S3.2)
- Adversary (S3.3)
- `go test -race` (Windows host cannot run `-race`)
- any modification to slice 2 files
- any modification to S3.1's existing files beyond the 3 documented
  modifications (main.go, observer.go, selftest.go)
- any modification to `internal/`, `cmd/ardents/`, `cmd/ardents-node/`
- any modification to ADRs or R-NNN records

## Local layout (implemented files)

```
experiments/long-running-simulation-2026-09-04/
├── README.md           (this file)
├── build.sh            (cross-compile sim-driver for linux/amd64)
├── test.sh             (local pre-flight on Windows; skips -race)
├── docker-compose.yml  (isolated sim network, build network, 2 source containers + sim-driver)
├── cmd/
│   └── sim-driver/
│       ├── doc.go
│       ├── main.go
│       ├── timekeeper.go
│       ├── observer.go
│       ├── tripwires.go
│       ├── fixtures.go
│       ├── selftest.go
│       ├── credentials.go
│       ├── personas.go
│       └── useractor.go
└── (Docker evidence is deliberately external to this repository.)
```
