# Node-role platform lifecycle operator procedure

The selected node-role platform path (Forwarding, JOIN, Descriptor, Issuer,
Introduction) uses platform-led delivery:
[ADR-0125](../adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
separates software delivery from infrastructure deployment and removes Installation
as a standalone domain. The platform delivers files and manages the process;
Enrollment and Release authenticate artifacts; the runtime validates its
configuration and completes its own work; Network State, participation, Hosting
and the role's semantic roots keep their data, rights and histories. The binding
contract, exact checks and refusal grammar live in the technical owner
([Platform-managed infrastructure launch](../technical/network-route-node.md#platform-managed-infrastructure-launch));
this procedure sequences the existing commands and adds no authority.

The runtime invocation declaration is domain-owned configuration validation
(ADR-0125 decision 6): `internal/node` owns one pure closed plan grammar
(`plan_declaration.go`) mirroring `network/source/servingdeclaration.ServingDeclaration`, shared by
the runtime, the retained repair commands and `node prepare-plan`. The codec
returns declaration facts only; it accepts no State, creates no root, initializes
no Hosting budget and signs nothing. The provisioning owner writes the
validated plan exclusively. The infrastructure launch owner holds the original
declaration and manager through retained repair. Network State verifies the
original dead invocation and decides the durable effect.

The original joined completion obligation is domain-owned: ordinary removal
requires the runtime's actual joined completion. Explicit bounded recovery may
abandon a verified dead invocation without inventing successful completion. A stop
signal, process death, lease release, expiry or exit status is not completion
evidence.

## First installation

Requires vacant State, participation, spend and Hosting roots (plus the
Descriptor Store root or Introduction Registry/History root for those roles, and
the Issuer's separately prepared Admission/key/results roots).

1. Prepare and sign the verified artifacts offline (Enrollment inventory, Release
   program/template/generation). Preparation signs nothing it does not own and
   authorizes no activation.
2. Accept the signed Network State through its existing owner:
   `ardents-next network accept-epoch ...` then `ardents-next network
   accept-profile ...` (or `network inspect` to verify). These consume already
   signed bytes; they create no roots and sign nothing. The State root must already
   exist and be vacant of history before the fresh pre-init below.
3. `ardents-next hosting initialize --config HOSTING.json` initializes the durable
   Hosting budget through `hosting.Initialize`. It is generic and role-agnostic.
4. `ardents-next node prepare-plan ROLE --network NETWORK.json --node-id HEX
   --participation-root ... --spend-root ... --hosting-root ... --certificate ...
   --private-key ... --work TX:RX --termination TX:RX --retirement-wait-ms N
   --output PLAN.json` writes the role's exact runtime plan JSON from operator
   inputs (`--descriptor-root` for Descriptor, `--introduction-root` for
   Introduction, `--issuance` and `--bootstrap-work` for Issuer). It validates the
   closed grammar and role-specific roots before exclusive output creation. A
   completed write and close precede the success receipt. It performs no State acceptance, no `hosting.Initialize` and no root
   creation; those stay the separate commands above. The produced plan decodes
   through the same node codec the runtime consumes.
5. `ardents-next infrastructure prepare-runtime-descriptor ...` and
   `ardents-next infrastructure prepare-runtime-inventory ...` produce public
   delivery bytes; Release signing is separate. They sign nothing they do not own.
6. `ardents-next platform initialize-trust LAUNCH.json INVENTORY_SHA256` performs
   the explicit first pin (empty Release floors, actual Enrollment executable
   provenance). It is never an automatic pre-start step.
7. `ardents-next platform unit LAUNCH.json` emits the rendered unit; the operator
   installs it as the fixed role service. The command installs nothing itself.
8. Fresh pre-init: run `ardents-next node ROLE --config PLAN.json` once outside the
   service to create the genuine first histories through the runtime owner. The
   installed unit then runs `node ROLE --retained-config PLAN.json`; the retained
   path supplies no first-history initialization.
9. `systemctl start ardents-ROLE` runs the mandatory privileged `platform check`
   pre-start, then the retained runtime.

## Stop and clean-stop verification

`systemctl stop ardents-ROLE` cancels the runtime, which joins State, its
observations, inputs and process protection before exit (`KillMode=control-group`,
`SendSIGKILL=no`, infinite stop wait). The operator confirms a clean stop: exit 130
(clean requested cancellation) with a joined report. A failure exit, a retained
producer or a completion-unavailable notice means completion is not established —
use recovery below, not a plain relaunch.

## Relaunch

After a verified clean stop, `systemctl start ardents-ROLE` reopens the same roots
through `state.OpenRetained`. Unresolved obligations refuse. Never re-run fresh
`--config` pre-init on a non-vacant root.

## Program change with state

Forward replacement is an operator sequence, not a live replacement:

1. Stop the old service cleanly (above).
2. Prepare a strictly forward Release generation (resource-only change requires a
   forward generation; a changed command adapter also requires fresh exact
   artifact admission). Floors stay monotonic.
3. Update `LAUNCH.json` (program path/digest, `config_sha256` when configuration
   changes) and reinstall the rendered unit if its binding changed. If the plan
   changes, regenerate it with `node prepare-plan` and update `config_sha256`.
4. `systemctl start ardents-ROLE`: `platform check` verifies the new exact proofs,
   configuration and loaded policy against retained floors, then the retained
   runtime opens the same roots. State is retained; no reset.

## Recovery after an interrupted invocation

Classify with the read-only pre-start inspection and the `OpenRetained` refusal;
preserve the roots. Never delete a producer, lease or history by hand.

- Clean completion: relaunch.
- Partial staging write, pointer/floor/pending divergence or role obligation:
  preserve the original bytes and refuse. Any wider repair needs its existing
  owner's separately admitted operation; manually deleting files or rerunning fresh
  preparation supplies no such authority.
- Sole unresolved node-duty participation fence: run outside the service as the
  fixed Node service principal:

  ```text
  ardents-next node ROLE repair-retained /etc/ardents/ROLE.json
  ```

  Forwarding, JOIN, Descriptor and Introduction have this retained repair route;
  Issuer has none. Use the original root-controlled public plan — the same full
  role declaration grammar the runtime consumes (Introduction repair now uses the
  full Introduction plan, not a subset). The command loads no private transport key
  and starts no runtime. It initializes no root, resets no floor; spend obligations
  (and Introduction slot claims) are never repaired. Existing roots must keep their
  exact fixed ownership and root-controlled ancestors. The actual original
  read-only manager connection must observe the fixed loaded service as
  `inactive/dead` or `failed/failed` with zero main/control PIDs, exact roots and
  loaded plan path, fixed account and required no-restart/control-group/confinement
  policy; independent kernel cgroup and bounded procfs inspection must exclude every
  other process of the account. These observations repeat after input checks;
  unavailable manager/unit facts refuse. Only then does participation remove that
  exact producer through its ordinary generation/watermark commit. State bytes and
  floors stay unchanged. `repaired` is emitted only after actual cleanup. A
  subsequent retained start is a new independently checked invocation.
- Missing or damaged history: permanent refusal. Preserve the roots; a fresh
  installation on new roots is a separate product decision that does not continue
  the old state.

The repair admission and its invariant justification are recorded in the technical
owner ([retained-only launch refusal](../technical/network-route-node.md#platform-managed-infrastructure-launch)).

## Evidence boundary

The native installed gates (`forwarding_platform_native_linux_test.go`,
`join_platform_native_linux_test.go`, `descriptor_platform_native_linux_test.go`)
provision the genuine State accept/profile, Hosting initialize, plan, launch,
first pin, rendered unit, fresh `--config` pre-init and retained `--retained-config`
start on a real Linux/systemd target, with positive acceptance and causal
before-effect refusals; they are the component evidence for this recipe. The plan
grammar refactor keeps those gates' plan maps admissible (the field set is
unchanged). Issuer and Introduction have native runtime evidence but no separate
platform installed gate yet. Domain admission/commit units, the `node prepare-plan`
command tests and Linux compilation do not by themselves establish native repair or
installed acceptance; actual systemd startup, stop, repair, relaunch and
interruption refusal require separate native evidence on the Linux target. This
procedure grants no qualification by itself.