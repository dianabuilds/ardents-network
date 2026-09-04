# Multi-agent real-network run — post-mortem (INCONCLUSIVE)

**Status: harness-invalid. Run does NOT prove what it was intended to prove.**

## Summary verdict

| Sub-claim | Status | Evidence |
|-----------|--------|----------|
| LLM → shell → Docker → real `ardents refresh-sources` against real `ardents-node source` | **PASS** | `battery_saver-4.json` shows `result_kind: accept`, source_outcomes `[valid, valid, not-attempted, not-attempted]`, exit_code 0 |
| 4-persona comparison in adversarial conditions | **FAIL/INCONCLUSIVE** | 3/4 personas errored every tick after the first few; reasons are harness-side, not network-side |
| Probe consumer correctly rejects source-c with `invalid-state` | **FAIL** in this run | 47/47 probe reports have `actual_outcomes: null`; the run never observed source-c rejection live. Slice 2 already proved this formally; this run does not. |
| Honest_user refreshes converge on real network | **FAIL** in this run | 0/84 accepts. Not network failure — harness state-lifecycle bug. |
| Battery_saver "idle on no change" persona works | **PARTIAL** | 1 accept, 5 noops, 8 errors. Tick 4 succeeded, ticks 5–10 went back to wrong shared plan and errored. Real persona decision was visible. |
| `cap=64` DoS attack surface | **NOT PROVEN** by this run | The old "local role state exceeds its bound" error came from the composite `validRecords` check, which covered record count, producer count, invalid/duplicate records, and identity/family conflicts. It did NOT come from `underInstallationSourceCap`, which returns `source exposure set is full: direct-source exposure set is full`. The actual cap=64 DoS test was not run. |

## Honest accounting of errors (corrected attribution)

Earlier verdict attributed most errors to "cap=64 cumulative direct-source records". This was wrong. The real mechanism is a **state-lifecycle bug** in the harness, not a production DoS:

1. **State-lifecycle bug (root cause for honest_user / paranoid / probe failures)**: each tick created a new `--state-root` (e.g. `/tmp/honest-1`, `/tmp/honest-2`, ...) but all ticks for a persona used the **same** `local_role_state_root` (e.g. `/tmp/honest_user-roles`). The producer key in `local_roles.go` includes `state_root`, so it changed every tick. `Replace` retains duties from other producers; overlapping non-Initiator source duties can therefore fail the conflict check immediately, while continued rotation can later hit producer or record bounds. The old aggregate error did not reveal which invariant fired. This is a harness design bug, not evidence of the direct-source cap.

2. **Shared `local_role_state_root` (root cause for early-tick errors)**: in the first attempt, the LLM agents used `client.json` (shared) instead of per-agent plans. Mitigated by per-agent plans with unique `local_role_state_root`, but some early ticks pre-dated the fix.

3. **15-second "deadline exceeded"**: this is the per-consumer cycle deadline in `internal/network/state/selection.go:56`. NOT a maximum refresh interval. After a successful wave, `cycleActive` resets (see `outcomes.go`). Deadline failures in this run were downstream of state-lifecycle race interrupting the cycle, not the persona's refresh cadence.

4. **2-hour cert TTL** (`internal/network/source/credentials.go`): re-prebake handled this.

5. **No-such-container**: transient during a docker restart between prebake iterations.

6. **cap=64 is a real production constraint**, but the explicit test ("retain more than 64 distinct direct-source exposures through a realistic caller capability, expect `ErrInstallationSourceExhausted`") was NOT run. It needs a separate contract after the attacker boundary is defined; repeated refreshes with one stable producer merely replace that producer's duties and cannot prove this claim.

## Evidence inconsistencies (audit gaps)

1. **47 probe files vs 68 probe-anomaly entries** — two probe-loop instances overwrote the same filenames. The evidence contract did not enforce single-writer. Anomaly counts are unreliable.

2. **Verdict says "paranoid anomalies: 0" but 66 paranoid reports have `anomaly_flagged=true`** — the aggregator script's anomaly counter looked for a different field. Counts under-count.

3. **`probe-1.json` was overwritten by a later loop run** — the initial "LLM wrote a PowerShell script as probe-1.json" finding is not auditable post-hoc. Without immutable event storage, early evidence is lost.

4. **Two parallel LLM agents with the same persona name** (probe_consumer) ran simultaneously because the spawn did not guarantee unique worker names per persona. They raced on filenames.

## What actually worked

- **battery_saver tick 4 (single accept)**: persona LLM correctly chose to use the per-agent `battery_saver-plan.json` after 3 failed attempts with the shared `client.json`. The accept was real mTLS, real cert verification, real State acceptance. This is the **only real, auditable success** of the run.

- **LLM goal-driven behavior is observable**: battery_saver's noop-after-accept pattern matched its goal. honest_user's "refresh frequently" matched its goal — it just couldn't refresh because of the harness bug.

- **Docker network is real**: mTLS handshakes complete against source-a, source-b, source-c with real certs.

## What was NOT proven (and what should be a separate qualification slice)

- **4-persona comparison**: cannot compare because 3/4 errored on the harness bug, not on real network conditions.
- **probe_consumer behavior**: 47/47 reports have `actual_outcomes: null`; the run never proved source-c rejection live.
- **`cap=64` DoS attack surface**: the error seen was from a different code path (`validRecords`, not `underInstallationSourceCap`). The actual cap=64 DoS test was not run.
- **15-second cycle as a DoS lever**: not exercised.

## File-level diagnosis

- `--state-root` and `local_role_state_root` are different stores, but both are stable for one consumer run. `--state-root` advances the consumer's State in place; `local_role_state_root` retains the installation-wide local-role truth.
- In the run, `local_role_state_root` was stable per persona, but `--state-root` changed every tick. The correct lifecycle keeps both paths stable per persona so the State advances normally and the local-role producer identity does not drift.
- The correct fix: **one persona = one stable `--state-root` + one stable `local_role_state_root`**, with the State actually advancing in-place (not via a new root per tick).

## Status

This run is **harness-invalid**. The tree under `experiments/multi-agent-real-2026-09-04/` should be kept as a post-mortem, not as a successful multi-agent experiment. A qualification rerun is required before any persona-comparison or DoS claim can be made.

Recommended next slice: **S3.6.5 — qualification rerun** (see separate contract), addressing the harness fixes from this post-mortem. A green honest/accept and probe/reject (expected) run qualifies the harness; the direct-source cap remains a separately scoped security question.
