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

Headless v6 bundles carry the Endpoint program and its Enrollment and
Release inputs. Protected Endpoint delivery supplies the signed document
worker and its managed resources. The offline Authority operator verifies
a separate `ardents-custody` executable under
[ADR-0131](../adr/0131-distribute-authority-custody-separately.md). The
qualification worker has a separate installed verification lane.

| Artifact lane | Entrypoint | Disposition | Boundary |
|---|---|---|---|
| Offline operator preparation and signing | ardents infrastructure provision, ardents infrastructure sign-release | retained tooling | Verified initial Network artifacts, protected role-specific key and Release production, and the separate Source Timestamp purpose |
| Offline Authority operator | ardents-custody | separate offline release artifact | Interactive Service and Admission Authority creation, issuance, verification and recovery. |
| Endpoint participant | ardents | participant distribution | Enrollment and platform-led Endpoint runtime. The document Application uses `publish`, `link` and `read`. |
| Source and Node operator | ardents source, ardents node | installed infrastructure | Distinct pinned role inventories use the same command build for serving and retained role duties. |
| Installed document worker | signed worker artifact `ardents-text` | automatic | Fixed confined Reader/Publisher worker entrypoints invoked by systemd. |
| Installed qualification | ardents-stream-qualification | keep as verification tooling | Fixed NET-14 stream-test Application worker entrypoints, built from the qualification fixture. |
| Local engineering diagnostics | ardents-diagnostics | Linux diagnostic image | Monitor, capture and detached evidence through the explicitly selected tool and evidence directory. The source is `internal/diagnostics/command`. |

The pair-qualification launcher passes a temporary document worker executable
through `ARDENTS_PREBUILT_DOCUMENT_WORKER`. The signed artifact inventory and
installed unit, socket and program names continue to use their protected
release declarations. Changing those names requires a versioned signed
inventory and an explicit predecessor-to-successor installation transition.

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
| ardents | enrollment verify | read-only initial artifact and pin verification |
| reviewed build executable | enrollment artifact-name / check-candidate / check-protected-generation | deterministic packaging and static inventory validation, independent of runtime authority |
| ardents-next | source prepare-state | offline first signed State and empty participation history |
| ardents-next | infrastructure prepare-runtime-descriptor / prepare-runtime-inventory | public artifact preparation for fixed roles |
| ardents-next | admission initialize / issuance issue | initialize a fresh quota root and use confirmed accounting through the Issuer |
| ardents-next | hosting initialize / observe | initialize a provider-period budget and inspect its accounting |
| ardents-next | endpoint withdraw | local Administration client |
| ardents-next | diagnostics timeline | local projection of bounded runtime events from standard input |
| ardents-next | network inspect-closed-profile / issuance inspect-public-profile / source inspect-latest | independent public-context verification and bounded Source contact |
| ardents-custody | create-service-authority, issue-service-credential, create-admission-authority, issue-admission-permission, verify-record, export-recovery-bundle, restore-recovery-bundle, purge-record | keep |
| ardents | publish, link, read | Participant document commands |
| Installed document worker | worker-reader, worker-publisher | Fixed entrypoints invoked by systemd |
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
| standalone `admission debit` | issuance uses the confirmed transaction through the owning Issuer |
| timed `hosting hold` | qualification exercises the Hosting reservation owner directly |
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
