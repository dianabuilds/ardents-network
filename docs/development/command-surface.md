# Command surface inventory

Status: **current engineering boundary.** The user-facing syntax and exact
behavior remain in [the command reference](../reference/commands.md). This
inventory answers a different question: why each process exists, which
artifact owns it, and whether its routes are retained, internal, pending a
separate decision, or retired.

## Process boundaries

Six binaries remain. Each owns a current participant or Application
compatibility process seam.

| Artifact lane | Binary | Disposition | Boundary |
|---|---|---|---|
| Network participant | ardents | keep and deepen | Headless Endpoint, Network State, Service Instance, Entry, and current naming adapters. |
| Network participant | ardents-node | keep | Source, Node duty, Transit Grant issuer, and dedicated-host Contributor process lifecycle. |
| Network participant | ardents-control | keep after contraction | Enrollment-pinned alpha-control/corpus reader and the sole corpus-floor acceptance adapter. |
| Network participant | ardents-custody | keep and deepen | Separate interactive Authority trust zone. It is intentionally not merged into Endpoint or Node. |
| Application compatibility | ardents-browser | keep pending Application product choice | Optional Browser Adapter over Application Interface v1; it imports no Network implementation. |
| Application compatibility | ardents-browser-entry | compatibility-only; research retirement | Native host plus explicit enrollment-v4 registration. ADR-0061 prevents treating it as the selected participant UI. |

The root cmd directory is Go's conventional collection of thin executable
adapters; it is not a product ownership boundary. Source and artifact ownership
are enforced separately by ownership.json, import tests, extraction checks,
and the two command inventories under tests/profiles.

## Retained routes

| Binary | Routes | Classification |
|---|---|---|
| ardents | accept-offline, refresh-sources; service-instance initialize/accept; endpoint enrollment-check/enroll/enroll-installed/headless/open/publish/withdraw/user-unit/installed-user-unit/replace/replacement-recovery/rollback; entry import | keep |
| ardents | endpoint replacement-self-test | keep internal-only; invoked by the replacement controller, not an operator route |
| ardents | name encode/resolve/control | deepen/research: keep current evidence, but later recompose naming access behind the selected Application product boundary rather than expanding direct operator input |
| ardents-node | source, node, issuer initialize/serve, contributor apply/diagnose/restart/drain/withdraw/remove | keep |
| ardents-control | inspect-bundle, inspect-transitions, inspect-alpha-corpus, accept-alpha-corpus | keep |
| ardents-custody | create-service-authority, issue-service-credential, inspect-envelope, verify-record, export-recovery-bundle, restore-recovery-bundle, purge-record | keep |
| ardents-browser | run | keep compatibility lane pending a future Desktop/Browser product decision |
| ardents-browser-entry | native-host, install, remove | keep compatibility evidence; do not add product behavior |

## Retired surface

The following inputs have no maintained production caller or selected
qualification owner and are rejected:

| Removed input | Reason |
|---|---|
| ardents endpoint portable | bypassed enrollment and Release Decision while duplicating the selected endpoint enroll lifecycle |
| ardents-control inspect | caller-keyed low-level ACA1 reader with its own mutable floor duplicated the enrollment-pinned participant inspection |
| ardents-control inspect-public-control | rendered a future public-control declaration that was definitionally never qualified and had only a unit-test caller |
| ardents-browser-entry install --at | parsed and discarded; it never affected enrollment or installation |
| completed ardents-control simulate-* routes | historical planning-campaign generators retired by ADR-0060 |
| ardents-release-custody initialize/inspect | completed RC1/RC2 release-seed ceremony; retired by ADR-0067 |
| ardents-state-custody initialize-alpha-genesis | completed fixed functional-alpha genesis ceremony; retired by ADR-0067 |

Removal of a command route does not remove the owning verification Module when
that Module still has maintained callers. Wire and persisted identities are
unchanged.

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
