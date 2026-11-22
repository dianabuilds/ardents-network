# Command surface inventory

Status: **current engineering boundary.** The user-facing syntax and exact
behavior remain in [the command reference](../reference/commands.md). This
inventory answers a different question: why each process exists, which
artifact owns it, and whether its routes are retained, internal, pending a
separate decision, or retired.

The former `cmd/ardents` executable has been removed. Its maintained
operations live in `ardents-next`. Historical refusal routes, including the
per-user Portable lifecycle and generic AAI2 client, have no command adapter.
The protected Endpoint uses the platform-led system unit and retains its
forward-only lifecycle.

## Process boundaries

New headless alpha bundles contain the Endpoint and Authority Custody
executables under [ADR-0130](../adr/0130-retire-control-and-aca1-from-alpha-bundles.md).
Protected Endpoint delivery adds the fixed text Application worker and its
resources. The qualification worker has its own installed verification lane.

| Artifact lane | Binary | Disposition | Boundary |
|---|---|---|---|
| Offline operator preparation and signing | provision-infrastructure, sign-infrastructure-release | retained tooling | Verified initial Network artifacts, protected role-specific key and Release production, and the separate Source Timestamp purpose |
| Network participant | ardents-custody | keep and deepen | Separate interactive Authority trust zone. It is intentionally not merged into Endpoint or Node. |
| Network participant and domain composition | ardents-next | maintained | Network, Node and Endpoint composition. The signed Endpoint artifact retains the `ardents-<platform>` filename. |
| Installed Application | ardents-text | keep | Trusted local text UI and fixed confined Reader/Publisher worker entrypoints. |
| Installed qualification | ardents-stream-qualification | keep as verification tooling | Fixed NET-14 stream-test Application worker entrypoints, built from the qualification fixture. |

The root cmd directory is Go's conventional collection of thin executable
adapters; it is not a product ownership boundary. Source and artifact ownership
are enforced separately by ownership.json, import tests, extraction checks,
and the headless command inventory under tests/profiles.

The retired `cmd/ardents-node` executable and its refusal routes were removed
under [ADR-0128](../adr/0128-retire-headless-node-companion.md). Installed
Node roles run `ardents-next` through the platform-led Infrastructure delivery.

## Retained routes

| Binary | Routes | Classification |
|---|---|---|
| ardents-next | network accept-epoch / accept-profile / inspect | bounded State intake and diagnostics |
| ardents-next | enrollment verify | read-only initial artifact and pin verification |
| ardents-next | source prepare-state | offline first signed State and empty participation history |
| ardents-next | infrastructure prepare-runtime-descriptor / prepare-runtime-inventory | public artifact preparation for fixed roles |
| ardents-next | endpoint withdraw | local Administration client |
| ardents-next | diagnostics timeline | local projection of bounded runtime events from standard input |
| ardents-next | network inspect-closed-profile / issuance inspect-public-profile / source inspect-latest | independent public-context verification and bounded Source contact |
| ardents-custody | create-service-authority, issue-service-credential, create-admission-authority, issue-admission-permission, inspect-envelope, verify-record, export-recovery-bundle, restore-recovery-bundle, purge-record | keep |
| ardents-text | read, publish, link; worker-reader/worker-publisher | keep; trusted local UI routes and fixed installed worker entrypoints |
| ardents-stream-qualification | worker-reader, worker-publisher | keep as fixed installed verification workers; never an operator route |

The former `endpoint enroll`/`enroll-installed` Portable per-user profile and
the separate `endpoint headless` protected text participant are retired. Their
joined installed C0 launch is now the platform-led successor composition in
`ardents-next`: the platform unit's ExecStartPre `platform check` transfers
accepted current-program identity before runtime start (F-25), the
authenticated `endpoint construct-plan` renders preparation plans from accepted
State pins (F-27), `endpoint prepare-history` initializes the first histories,
and `endpoint start-installed` runs the protected participant under the system
unit. `enrollment verify` remains a read-only bundle diagnostic and
`endpoint withdraw` remains a thin Administration Interface client; neither
starts a runtime or transfers enrollment state.

## Retired surface

The following inputs belonged to retired or refusal-only surfaces:

| Removed input | Reason |
|---|---|
| former ardents-control binary | offline production has distinct entrypoints and public inspection uses the owning product command; retired ACA1/ACA2 and qualification selectors have no executable |
| completed ardents-control simulate-* routes | historical planning-campaign generators retired by [ADR-0060](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0060-retire-completed-planning-campaign-generators.md) (archived) |
| ardents-release-custody initialize/inspect | completed RC1/RC2 release-seed ceremony; retired by ADR-0067 |
| ardents-state-custody initialize-alpha-genesis | completed fixed functional-alpha genesis ceremony; retired by ADR-0067 |

Removal of a command route does not remove the owning verification Module when
that Module still has maintained callers. Wire and persisted identities are
unchanged.

The retirement of the generic Endpoint Connection route is protected by the bounded
[AAI2 retirement closure gate](testing.md#reachability-audit). Testing owns its
exact receipt and scope; this command inventory does not widen that evidence
into a whole-release qualification.

The retired public-control parser and its diagnostic matrix have no selected
production caller. Their exact final source and tests remain available at
`0e580c153114dd32f4b4c1fff86842b882f71937:internal/publiccontrol`; the current
tree does not preserve an unselected placeholder Module.

## Change rule

Adding or retaining a route requires all of:

1. one owning product or ceremony boundary;
2. one registered artifact/profile lane;
3. exact command documentation;
4. behavior evidence that invokes the real route;
5. no second path around the owning lifecycle or authority transition.

A diagnostic is not retained merely because it can print useful JSON. Future
UI, managed Endpoint, naming, or public-control work must first select its
product boundary and then add the smallest route needed by that boundary.
