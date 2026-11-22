# Source platform lifecycle operator procedure

The selected Source platform path uses platform-led delivery:
[ADR-0125](../adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
separates software delivery from infrastructure deployment and removes Installation
as a standalone domain. The platform delivers files and manages the process;
Enrollment and Release authenticate artifacts; the runtime validates its
configuration and completes its own work; Network State and participation keep
their data, rights and histories. The binding contract, exact checks and refusal
grammar live in the technical owner
([Platform-managed infrastructure launch](../technical/network-route-node.md#platform-managed-infrastructure-launch));
this procedure sequences the existing commands and adds no authority.

The serving producer obligation is domain-owned: ordinary removal requires the original joined
`SourceCompletion.Finish`. Explicit bounded recovery may abandon a verified dead
invocation without inventing successful completion. The handle is volatile and
is acquired before supervision, so it supplies no post-crash evidence. A stop signal, process death, lease release,
expiry or exit status is not completion evidence.

## First installation

Requires a vacant State root and a vacant participation root.

1. Prepare and sign the verified artifacts offline (Enrollment inventory, Release
   program/template/generation). Preparation signs nothing it does not own and
   authorizes no activation.
2. `ardents-next source prepare-state SERVE_PLAN BUNDLE` creates the initial
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
- Partial staging write (`.stage-*` / `.current-*`), pointer/floor/pending
  divergence or wave/exposure obligation: preserve the original bytes and refuse.
  The bounded serving-producer repair cannot resume a wave, repair a pointer,
  prune staging or bypass mandatory read-only pre-start inspection. Any wider
  repair needs its existing owner's separately admitted operation; manually
  deleting files or rerunning fresh preparation supplies no such authority.
- Sole unresolved serving producer: run outside the service as `ardents-source`:

  ```text
  ardents-next source repair-retained /etc/ardents/source.json
  ```

  Use the original root-controlled public plan; the command loads no private
  transport key. Existing roots must stay under `/var/lib/ardents/roles/source/`
  with their exact fixed ownership and root-controlled ancestors. Both existing
  State and participation leases remain held from admission through the one
  commit. Signed current State, trusted time and exact floors must agree; the
  sole serving duty must match that authenticated current generation. Extra
  duties, pending generations, exposure history, active cycles and uncertain
  prefixes refuse before mutation. A clean root refuses without a Claim probe.

  The actual original read-only manager connection must observe the fixed loaded
  service as `inactive/dead` or `failed/failed`, with zero main/control PIDs,
  exact roots and loaded `serve-retained` plan path, fixed account and required
  no-restart/control-group/confinement policy. Independent kernel inspection must
  find its fixed cgroup absent or unpopulated, including descendants. Bounded
  procfs inspection must also exclude every other process of the Source account,
  including a manually started command outside the unit. Invoke repair directly;
  an interactive shell or second command under that account retains refusal.
  Restricted proc visibility, incomplete reads or exhausted bounds also refuse. These
  observations repeat after input checks; unavailable manager/unit facts refuse.
  Keep the failed unit's evidence until repair; resetting failed state is not a
  prerequisite. Elapsed time, lease availability or a supplied callback cannot
  authorize removal.

  Only then does participation remove that exact producer through its ordinary
  generation/watermark commit. State bytes and floors stay unchanged. Pre-commit
  refusal preserves the original files; commit or cleanup uncertainty returns
  failure without pretending rollback. `repaired` is emitted only after actual
  cleanup. Preserve the resulting history and use a new independently checked
  platform launch/producer Claim to determine the next outcome.
- Missing or damaged history: permanent refusal. Preserve the roots; a fresh
  installation on new roots is a separate product decision that does not continue
  the old state.

The repair admission and its invariant justification are recorded in the technical
owner ([retained-only launch refusal](../technical/network-route-node.md#platform-managed-infrastructure-launch)).

## Evidence boundary

Domain admission/commit units and Linux compilation do not establish native
repair acceptance. Actual systemd startup, stop, repair, relaunch,
interruption refusal and complete operator acceptance require separate native
evidence on the Linux target before the transitional Source orchestration is
retired (ADR-0125). This procedure grants no qualification by itself.