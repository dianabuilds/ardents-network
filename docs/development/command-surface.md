# Command surface inventory

Status: **current engineering boundary.** The user-facing syntax and exact
behavior remain in [the command reference](../reference/commands.md). This
inventory answers a different question: why each process exists, which
artifact owns it, and whether its routes are retained, internal, pending a
separate decision, or retired.

**Retired dispatch paths.** `entry import` and `entry recipient`
joined the refusals under ADR-0106: both return `entry Invite command is
retired` before any argument interpretation or filesystem effect, and no
Invite root reader, converter, or deleter remains. `inspect-alpha-corpus`
joined them under ADR-0110: the exact route returns `inspect-alpha-corpus is
retired` before parsing arguments or opening any file, root, or floor, and no
ACA2 verifier remains in the tree. ADR-0127 (#647) added the persisted-input
`endpoint enrollment-check` form, `inspect-bundle`, `inspect-transitions`, and
the four `prepare-qualification-*` routes: each refuses at dispatch before
arguments, files, roots, or floors, and the ACA1 reader/inspection Modules are
removed.
The F-08 operator-surface reduction is complete:
the Invite root's data obligation was resolved as typed incompatibility
(existing roots stay on disk byte-for-byte), the import writer is stopped,
and the read-only recipient command is retired with it.
The per-user Portable Endpoint lifecycle is retired under ADR-0125 (#617
S3.5): the proved protected system-unit successor (platform launch/unit/check
F-25 trust transfer plus the authenticated F-27 plan constructor) is merged,
and the Product Owner confirmed the command-surface cutover. `endpoint
enroll/enroll-installed/headless/user-unit/installed-user-unit/replace/
rollback/replacement-self-test/replacement-recovery` now return typed
retirement refusals before input or root effects; the canonical delivery,
preparation and forward-only replacement/recovery composition lives in
`ardents-next` (platform, endpoint construct-plan/prepare-history/
start-installed, platform stage-endpoint). The
read-only `endpoint enrollment-check` diagnostic and the `endpoint withdraw`
Administration Interface client remain. Renaming a Portable event to C0
readiness would still leave the missing trust transfer unresolved; retirement
resolves it by selecting the platform-led successor instead.

## Process boundaries

New headless alpha bundles contain three participant and control binaries.
Enrollment verifies earlier inventories according to their pinned schema.
The installed text Application and qualification workers are separate process
boundaries; they are not alpha-bundle artifacts.

| Artifact lane | Binary | Disposition | Boundary |
|---|---|---|---|
| Network participant | ardents | keep and deepen | Headless Endpoint, Network State and selected closed Service interfaces; Entry Invite and Name routes are retired. |
| Network participant | ardents-control | keep after contraction | Bounded closed-profile operator adapter and offline infrastructure preparation compositions; the ACA1 disclosure/transition reader and ACA2 corpus intake/inspection are retired (ADR-0110, ADR-0127). |
| Network participant | ardents-custody | keep and deepen | Separate interactive Authority trust zone. It is intentionally not merged into Endpoint or Node. |
| Isolated domain composition | ardents-next | maintained domain consumer | New-domain commands and bounded native consumers; no old-runtime imports, installed Service or final artifact cutover implied. |
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
| ardents | accept-offline, accept-closed-profile; endpoint enrollment-check <bundle-root> <manifest-sha256>/withdraw | keep; enrollment-check verifies through the successor enrollment owner (ADR-0127) |
| ardents | endpoint enrollment-check <alpha-enrollment.json> | retirement refusal at dispatch before any file read; the persisted v1 document parser is removed and existing documents stay on disk (ADR-0127, #647) |
| ardents | refresh-sources | retirement refusal at dispatch before input or root effects; the platform-led runtime owns automatic Source refresh and the successor State never samples process resources (#493) |
| ardents | service-instance initialize/accept | retirement refusal at dispatch; canonical lifecycle is ardents-next publication-instance (#562) |
| ardents | endpoint enroll/enroll-installed/headless/user-unit/installed-user-unit/replace/rollback/replacement-recovery | retirement refusal at dispatch before input or root effects; canonical platform-led delivery and forward-only replacement/recovery is ardents-next platform/endpoint/installation (#617 S3.5, ADR-0125) |
| ardents | endpoint publish | retirement refusal at dispatch; use ardents-text publish |
| ardents | entry recipient/import | retirement refusal at dispatch before arguments or effects; no Invite reader, converter, or deleter remains (ADR-0106) |
| ardents | endpoint replacement-self-test | retirement refusal at dispatch; the per-user replacement controller is retired (#617 S3.5) |
| ardents | diagnostics timeline | keep local, read-only projection of bounded runtime events from standard input; no storage, authority, or network effect |
| ardents | name encode/resolve/control | retired at command dispatch before arguments or effects; no successor or fallback is implied (ADR-0113) |
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
unit. `endpoint enrollment-check` remains a read-only bundle diagnostic and
`endpoint withdraw` remains a thin Administration Interface client; neither
starts a runtime or transfers enrollment state.

## Retired surface

The following inputs have no maintained production caller or selected
qualification owner and are rejected:

| Removed input | Reason |
|---|---|
| ardents endpoint portable | bypassed enrollment and Release Decision while duplicating the selected endpoint enroll lifecycle |
| ardents endpoint enroll/enroll-installed/headless/user-unit/installed-user-unit/replace/rollback/replacement-self-test/replacement-recovery | retired per-user Portable lifecycle (ADR-0125, #617 S3.5); each route returns its typed refusal before input or root effects, the legacy cmd implementation is removed, and the platform-led ardents-next successor owns delivery/preparation/forward-only replacement. The owning portable/replacement Modules remain until the separately gated Endpoint cohort removal; their process and Ubuntu qualification evidence retires with the surface |
| ardents-control inspect | caller-keyed low-level ACA1 reader with its own mutable floor duplicated the enrollment-pinned participant inspection |
| ardents-control inspect-public-control | rendered a future public-control declaration that was definitionally never qualified and had only a unit-test caller |
| completed ardents-control simulate-* routes | historical planning-campaign generators retired by [ADR-0060](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0060-retire-completed-planning-campaign-generators.md) (archived) |
| ardents-control accept-alpha-corpus | retired fresh corpus intake; the exact route returns `accept-alpha-corpus is retired` before parsing arguments or opening any file or floor |
| ardents-control inspect-alpha-corpus | retired ACA2/corpus diagnostic; the exact route returns `inspect-alpha-corpus is retired` before parsing arguments or opening any file, root, or floor, and the ACA2 production verifier is removed (ADR-0110) |
| ardents endpoint enrollment-check <alpha-enrollment.json> | retired persisted portable-document diagnosis (ADR-0127, #647); the typed refusal precedes any document read, the v1 parser is removed, and the bundle-root+pin form through the successor verifier is the sole accepting route. Existing documents stay on disk byte-for-byte |
| ardents-control inspect-bundle/inspect-transitions | retired ACA1 disclosure reader and unified transition report (ADR-0127, #647); each returns `<route> is retired` before parsing arguments or opening any file, root, or floor. The alphacontrol/inspection Modules and report schemas are removed; successor per-domain commands verify the same authority facts and no unified replacement is selected. Existing inspection floor roots stay unchanged |
| ardents-control prepare-qualification-evidence/prepare-qualification-catalog | retired ACA1 signing-input preparation (ADR-0127, #647); typed refusal at dispatch and the alpha preparation scripts are removed with the reader |
| ardents-control prepare-qualification-node-record/prepare-qualification-epoch | retired old-epoch Network preparation (ADR-0127, #647); typed refusals point at the successor `ardents-next network` preparation commands that produce the same canonical ARNR-v2/AREP-v3 bytes, and the qualification-network scripts are removed |
| ardents-release-custody initialize/inspect | completed RC1/RC2 release-seed ceremony; retired by ADR-0067 |
| ardents-state-custody initialize-alpha-genesis | completed fixed functional-alpha genesis ceremony; retired by ADR-0067 |
| ardents endpoint headless with `ardents-headless-runtime-v1` | retired startup schema; refused before plan-owned runtime effects without converting retained roots or floors |
| ardents endpoint open | retired generic AAI2 route; the exact legacy syntax returns `endpoint open is retired` before effects, and its codec/server/client plus exclusive Endpoint adapter are absent |
| ardents-node source with `native_rendezvous_profile` | retired old Source profile selector; the retained field returns `old Source profile is retired` before trust-map, root, key, listener, output, or Network effects, including mixed old/current input |
| ardents-node node with `rendezvous`, `initiator`, `introduction`, `responder`, or `transit_issuer` reservation | retired old duty starts; the v1 plan schema and separately named closed reservations remain, while each old selector returns a stable typed refusal before plan-owned runtime effects |
| ardents-node contributor (every action) | retired dedicated-host Contributor retirement mechanism removed entirely (ADR-0114): no live `ardents-rendezvous-contributor` installation remains, the subcommand is no longer recognized and fails with the standard usage error before any platform, bundle, installation, supervisor, output, or Network effect |

Removal of a command route does not remove the owning verification Module when
that Module still has maintained callers. Wire and persisted identities are
unchanged.

The retired `endpoint open` row is protected by the bounded
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
