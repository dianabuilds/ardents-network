# Source platform lifecycle operator procedure

The installed Source runs under platform-led delivery:
[ADR-0125](../adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
separates software delivery from infrastructure deployment and removes Installation
as a standalone domain. The platform delivers files and manages the process;
Enrollment and Release authenticate artifacts; the runtime validates its
configuration and completes its own work; Network State and participation keep
their data, rights and histories. The binding contract, exact checks and refusal
grammar live in the technical owner
([Platform-managed infrastructure launch](../technical/network-route-node.md#platform-managed-infrastructure-launch));
this procedure sequences the existing commands and adds no authority.

The serving producer obligation is domain-owned: only a joined
`SourceCompletion.Finish` removes it. A stop signal, process death, lease release,
expiry or exit status is not completion evidence.

## First installation

Requires a vacant State root and a vacant participation root.

1. Prepare and sign the verified artifacts offline (Enrollment inventory, Release
   program/template/generation). Preparation signs nothing it does not own and
   authorizes no activation.
2. `ardents-control source prepare-state SERVE_PLAN BUNDLE` creates the initial
   State and empty participation history through their existing owners
   (`state.OpenFresh` + `duty.Initialize`). A repeated or partial preparation
   refuses without cleaning saved data.
3. `ardents-next platform initialize-trust LAUNCH.json INVENTORY_SHA256` performs
   the explicit first pin (empty Release floors, actual Enrollment executable
   provenance). It is never an automatic pre-start step.
4. `ardents-next platform unit LAUNCH.json` emits the rendered unit; the operator
   installs it as `/etc/systemd/system/ardents-source.service`. The command
   installs nothing itself.
5. `systemctl start ardents-source` runs the mandatory privileged
   `platform check` pre-start, then `source serve-retained CONFIG`.

## Stop and clean-stop verification

`systemctl stop ardents-source` cancels the runtime, which joins State, its
observations, inputs and process protection before exit (`KillMode=control-group`,
`SendSIGKILL=no`, infinite stop wait). The operator confirms a clean stop: exit 0
(joined) or 130 (clean requested cancellation). A failure exit, a retained producer
or a completion-unavailable notice means completion is not established — use
recovery below, not a plain relaunch.

## Relaunch

After a verified clean stop, `systemctl start ardents-source` reopens the same
roots through `state.OpenRetained`. Unresolved obligations refuse. Never re-run
`prepare-state` on a non-vacant root.

## Program change with state

Forward replacement is an operator sequence, not a live replacement:

1. Stop the old service cleanly (above).
2. Prepare a strictly forward Release generation (resource-only change requires a
   forward generation; a changed command adapter also requires fresh exact
   artifact admission). Floors stay monotonic.
3. Update `LAUNCH.json` (program path/digest, `config_sha256` when configuration
   changes) and reinstall the rendered unit if its binding changed.
4. `systemctl start ardents-source`: `platform check` verifies the new exact
   proofs, configuration and loaded policy against retained floors, then
   `serve-retained` opens the same roots. State is retained; no reset.

## Recovery after an interrupted invocation

Classify with the read-only pre-start inspection and the `OpenRetained` refusal;
preserve the roots. Never delete a producer, lease or history by hand.

- Clean completion: relaunch.
- Partial staging write (`.stage-*` / `.current-*`): remove only owned staging
  names after identity checks (committed bytes untouched), then relaunch.
- Pointer/floor/pending divergence: relaunch through the owning runtime open,
  which performs the monotonic floor auto-repair; do not edit pointers by hand.
- Unresolved serving producer (a crash before joined `Finish`):
  `ardents-next source repair-retained CONFIG`, run as the fixed Source service
  principal. `state.RepairRetainedSourceProducer` clears only that producer after
  proving the original is defunct (inactive service, no live main/control process),
  the durable chain/journal/floors and wave guard are consistent, and the producer
  is the sole unresolved obligation. It creates nothing, resets no floor and
  deletes no history; any uncertainty refuses with bytes preserved. Then relaunch.
- Wave/exposure obligation (active cycle journal, plan mismatch): restore the exact
  configured Source plan so the owner can resume or release; otherwise refusal.
- Missing or damaged history: permanent refusal. Preserve the roots; a fresh
  installation on new roots is a separate product decision that does not continue
  the old state.

The repair admission and its invariant justification are recorded in the technical
owner ([retained-only launch refusal](../technical/network-route-node.md#platform-managed-infrastructure-launch)).

## Evidence boundary

These commands are unit-verified. Actual systemd startup, stop, relaunch,
interruption refusal and complete operator acceptance require separate native
evidence on the Linux target before the transitional Source orchestration is
retired (ADR-0125). This procedure grants no qualification by itself.