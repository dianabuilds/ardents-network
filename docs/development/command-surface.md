# Command surface inventory

Status: **current engineering boundary.** The user-facing syntax and exact
behavior remain in [the command reference](../reference/commands.md). This
inventory answers a different question: why each process exists, which
artifact owns it, and whether its routes are retained, internal, pending a
separate decision, or retired.

## Process boundaries

The alpha bundle contains four headless participant and control binaries.
The installed text Application and qualification workers are separate process
boundaries; they are not alpha-bundle artifacts.

| Artifact lane | Binary | Disposition | Boundary |
|---|---|---|---|
| Network participant | ardents | keep and deepen | Headless Endpoint, Network State, Service Instance, Entry, and current naming adapters. |
| Network participant | ardents-node | keep | Source, Node duty, closed issuer, and dedicated-host Contributor process lifecycle. |
| Network participant | ardents-control | keep after contraction | Enrollment-pinned alpha-control/corpus reader and bounded closed-profile operator adapter; it has no corpus-floor mutation authority. |
| Network participant | ardents-custody | keep and deepen | Separate interactive Authority trust zone. It is intentionally not merged into Endpoint or Node. |
| Installed Application | ardents-text | keep | Trusted local text UI and fixed confined Reader/Publisher worker entrypoints. |
| Installed qualification | ardents-qualification | keep as verification tooling | Drives the selected installed network qualification run and records evidence. |
| Installed qualification | ardents-stream-qualification | keep as verification tooling | Fixed NET-14 stream-test Application worker entrypoints. |

The root cmd directory is Go's conventional collection of thin executable
adapters; it is not a product ownership boundary. Source and artifact ownership
are enforced separately by ownership.json, import tests, extraction checks,
and the headless command inventory under tests/profiles.

## Retained routes

| Binary | Routes | Classification |
|---|---|---|
| ardents | accept-offline, accept-closed-profile, refresh-sources; service-instance initialize/accept; endpoint enrollment-check/enroll/enroll-installed/headless/publish/withdraw/user-unit/installed-user-unit/replace/replacement-recovery/rollback; entry recipient/import | keep |
| ardents | endpoint replacement-self-test | keep internal-only; invoked by the replacement controller, not an operator route |
| ardents | name encode | keep local canonical encoding; this is not network Name availability |
| ardents | diagnostics timeline | keep local, read-only projection of bounded runtime events from standard input; no storage, authority, or network effect |
| ardents | name resolve/control | retired at command dispatch before arguments or effects; no successor or fallback is implied |
| ardents-node | source (including the explicit current closed profile), node with closed reservations, issuer initialize/serve, hosting initialize, contributor diagnose/drain/withdraw/remove | keep; old Source selector, Node duty reservations and Contributor start inputs retired |
| ardents-control | inspect-bundle, inspect-transitions, inspect-alpha-corpus; prepare-closed-profile, sign-closed-profile, inspect-closed-profile, inspect-closed-issuer-profile | keep |
| ardents-custody | create-service-authority, issue-service-credential, create-admission-authority, issue-admission-permission, inspect-envelope, verify-record, export-recovery-bundle, restore-recovery-bundle, purge-record | keep |
| ardents-text | read, publish, link; worker-reader/worker-publisher | keep; trusted local UI routes and fixed installed worker entrypoints |
| ardents-qualification | run from a local plan; preflight, verify-run, verify-pair, verify-network-manifest, verify-net14v, verify-failed-net14v | keep as installed verification tooling; never a participant route |
| ardents-stream-qualification | worker-reader, worker-publisher | keep as fixed installed verification workers; never an operator route |

The `endpoint enroll` and `enroll-installed` routes own artifact/Release
acceptance and the Portable per-user profile. Their `ready` event describes
the generic local probe attachment. The separate `endpoint headless` route
owns the protected text participant and its Network/Service lifetime. The
current command dispatcher does not transfer enrollment state between these
routes; the joined installed C0 launch remains a distinct composition task.

## Retired surface

The following inputs have no maintained production caller or selected
qualification owner and are rejected:

| Removed input | Reason |
|---|---|
| ardents endpoint portable | bypassed enrollment and Release Decision while duplicating the selected endpoint enroll lifecycle |
| ardents-control inspect | caller-keyed low-level ACA1 reader with its own mutable floor duplicated the enrollment-pinned participant inspection |
| ardents-control inspect-public-control | rendered a future public-control declaration that was definitionally never qualified and had only a unit-test caller |
| completed ardents-control simulate-* routes | historical planning-campaign generators retired by ADR-0060 |
| ardents-control accept-alpha-corpus | retired fresh corpus intake; the exact route returns `accept-alpha-corpus is retired` before parsing arguments or opening any file or floor, while read-only corpus inspection remains |
| ardents-release-custody initialize/inspect | completed RC1/RC2 release-seed ceremony; retired by ADR-0067 |
| ardents-state-custody initialize-alpha-genesis | completed fixed functional-alpha genesis ceremony; retired by ADR-0067 |
| ardents endpoint headless with `ardents-headless-runtime-v1` | retired startup schema; refused before plan-owned runtime effects without converting retained roots or floors |
| ardents endpoint open | retired generic AAI2 route; the exact legacy syntax returns `endpoint open is retired` before effects, and its codec/server/client plus exclusive Endpoint adapter are absent |
| ardents-node source with `native_rendezvous_profile` | retired old Source profile selector; the retained field returns `old Source profile is retired` before trust-map, root, key, listener, output, or Network effects, including mixed old/current input |
| ardents-node node with `rendezvous`, `initiator`, `introduction`, `responder`, or `transit_issuer` reservation | retired old duty starts; the v1 plan schema and separately named closed reservations remain, while each old selector returns a stable typed refusal before plan-owned runtime effects |
| ardents-node contributor `apply` or `restart` | retired old Contributor starts; their recognized command shapes return a stable refusal before platform, bundle, installation, supervisor, output, or Network effects; retirement-only diagnose/drain/withdraw/remove remain |

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
