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

New headless alpha bundles contain three participant and control binaries.
Enrollment verifies earlier inventories according to their pinned schema.
The installed text Application and qualification workers are separate process
boundaries; they are not alpha-bundle artifacts.

| Artifact lane | Binary | Disposition | Boundary |
|---|---|---|---|
| Network participant | ardents-control | keep after contraction | Bounded closed-profile operator adapter and offline infrastructure preparation compositions; the ACA1 disclosure/transition reader and ACA2 corpus intake/inspection are retired (ADR-0110, ADR-0127). |
| Network participant | ardents-custody | keep and deepen | Separate interactive Authority trust zone. It is intentionally not merged into Endpoint or Node. |
| Network participant and domain composition | ardents-next | maintained | Network, Node and Endpoint composition. The signed Endpoint artifact retains the `ardents-<platform>` filename. |
| Installed Application | ardents-text | keep | Trusted local text UI and fixed confined Reader/Publisher worker entrypoints. |
| Installed qualification | ardents-stream-qualification | keep as verification tooling | Fixed NET-14 stream-test Application worker entrypoints. |

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
| ardents-next | enrollment verify / verify-headless | read-only initial artifact and pin verification |
| ardents-next | endpoint withdraw | local Administration client |
| ardents-next | diagnostics timeline | local projection of bounded runtime events from standard input |
| ardents-control | prepare-closed-profile, sign-closed-profile, inspect-closed-profile, inspect-closed-issuer-profile | keep |
| ardents-control | inspect-bundle, inspect-transitions | retirement refusal at dispatch before arguments, files, roots, or floors; the ACA1 reader/report Modules are removed and no unified replacement is selected (ADR-0127, #647) |
| ardents-control | prepare-qualification-evidence, prepare-qualification-catalog | retirement refusal at dispatch; the ACA1 preparation scripts are removed with the reader (ADR-0127, #647) |
| ardents-control | prepare-qualification-node-record, prepare-qualification-epoch | retirement refusal at dispatch pointing at ardents-next network prepare-initial-node-record/prepare-initial-epoch; the qualification-network scripts are removed (ADR-0127, #647) |
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
| ardents-control source prepare-template / prepare-install-inputs | the platform-led Source lifecycle and `ardents-next platform template` replace these preparation routes under ADR-0125 |
| ardents-control forwarding, join and descriptor prepare-template / prepare-request / prepare-plan / prepare-hosting / prepare-state | platform-led role preparation owns delivery under ADR-0125. The former routes exit with code 2 before input I/O |
| ardents-control inspect | caller-keyed low-level ACA1 reader with its own mutable floor duplicated the enrollment-pinned participant inspection |
| ardents-control inspect-public-control | rendered a future public-control declaration that was definitionally never qualified and had only a unit-test caller |
| completed ardents-control simulate-* routes | historical planning-campaign generators retired by [ADR-0060](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0060-retire-completed-planning-campaign-generators.md) (archived) |
| ardents-control accept-alpha-corpus | retired fresh corpus intake; the exact route returns `accept-alpha-corpus is retired` before parsing arguments or opening any file or floor |
| ardents-control inspect-alpha-corpus | retired ACA2/corpus diagnostic; the exact route returns `inspect-alpha-corpus is retired` before parsing arguments or opening any file, root, or floor, and the ACA2 production verifier is removed (ADR-0110) |
| ardents-control inspect-bundle/inspect-transitions | retired ACA1 disclosure reader and unified transition report (ADR-0127, #647); each returns `<route> is retired` before parsing arguments or opening any file, root, or floor. The alphacontrol/inspection Modules and report schemas are removed; successor per-domain commands verify the same authority facts and no unified replacement is selected. Existing inspection floor roots stay unchanged |
| ardents-control prepare-qualification-evidence/prepare-qualification-catalog | retired ACA1 signing-input preparation (ADR-0127, #647); typed refusal at dispatch and the alpha preparation scripts are removed with the reader |
| ardents-control prepare-qualification-node-record/prepare-qualification-epoch | retired old-epoch Network preparation (ADR-0127, #647); typed refusals point at the successor `ardents-next network` preparation commands that produce the same canonical ARNR-v2/AREP-v3 bytes, and the qualification-network scripts are removed |
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
