# Stage 8 current journey and claim trace

Status: **S8.0 in progress; factual trace at source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.** This temporary companion to the
[current-system report](stage-8-current-system-report.md) records current
caller and observable-test paths, including absent evidence. A path is not a
product-support or Qualification claim.

## H3 journey trace

The accepted H3 coverage rows are defined in
[the technical design](horizon-3-technical-design.md#79-product-journey-coverage).
The table names the current path rather than inferring that every cross-horizon
journey is implemented.

| Journey | Current real caller and owner | Observable current evidence | Entry assessment |
|---|---|---|---|
| J00 install and first run | `cmd/ardents-release.run` opens Release Decision state and invokes the Update Transaction path; `releasedecision.OpenFloorStore`, `Evaluate`, and `updatetransaction.Apply`/`Recover` own the current tracer seam. | `cmd/ardents-release` tests; Update Transaction's R00--R14 recovery, rollback, cleanup, and interruption tests. | An offline update tracer exists. No supported installer/repair/remove, real product activation, or Custody owner is established. |
| J01 start, join, and refresh | `cmd/ardents.runRefreshSources` opens `network/state`; `cmd/ardents-node.run` calls `node.Run`; current state/source/local-role Modules supply the inputs. | `cmd/ardents` tests; `tests/e2e/network-source`; `tests/e2e/node`. | Authenticated-state and Node tracer paths exist. Current evidence does not select a supported host profile or prove first-run/readiness across platforms. |
| J02 open an unlisted Service by exact name | `cmd/ardents-name.runResolution` calls `nameresolution.Open`; resolution uses current Network State. The retained Service Connection seam is `serviceconn`. | `TestResolveCommandRunsPrivateResolution`, `TestDeepestLegalNameResolvesThroughSeparateRoles`, and `TestNameOriginConnectionClosesWhenTargetBindingChanges`, as named in the retained Stage 6 trace. | The naming/resolution trace is retained. A current integrated H3 command journey from exact Name through Route to Application bytes still requires disposition and re-trace. |
| J03 publish and move a Service | `cmd/ardents-name.runControl` calls Name Authority lifecycle operations; `cmd/ardents-publish-app.run` and `cmd/ardents-service.run` are the current publication/Endpoint callers. | `TestControlCommandExecutesEveryPrivateControlShape`; `TestServiceProcessesKeepConnectionWhenReplacementFails`; Name Authority and recovery behavior suites. | Current stage-derived publication paths exist, but no Custody owner or accepted retained public command surface exists. |
| J04 integrate an Application | `serviceendpoint.Run` composes the current Endpoint path; `applicationipc` owns the local byte/result seam; `cmd/ardents-stream-app.run` is the deterministic application tracer. | `internal/applicationipc` behavior tests and `cmd/ardents-stream-app` tests. | There is no `application`/Isolation Module or qualified Application Principal boundary. An Application-level location claim is absent. |
| J05 named end-to-end tracer | `cmd/named-site-lab.runGateC` calls the historical `internal/lab/namedsite` runner. | `TestReferenceTopologyCarriesOneAuthenticatedWorkload` and `TestReferenceTopologyRejectsSupersededPublicationDuringMigration` when immutable Gate C images are supplied. | Historical reproduction evidence, not current product acceptance; its images are external prerequisites. |
| J-06 recover from failure or blocking | `serviceconn` owns current connection/recovery semantics; `route.Run` and `routeplan.Run` own current route actor execution; Bridge/WebTunnel callers are `cmd/ardents-bridge` and `cmd/ardents-route`. | `tests/e2e/service/recovery_process_test.go`; Route and Service Connection behavior tests; Update R00--R14 for update recovery. | Several bounded recovery seams exist. No current all-failure-class, multi-platform, or adversarial Qualification path is proven. |
| J07 contribute bounded network capacity | `cmd/ardents-node.run` calls `node.Run`; Node consumes Network State, local roles, and resource observations. | `cmd/ardents-node` tests, `tests/e2e/node`, and Node/resource behavior tests. | Project-controlled Node tracer only; no independent-operator or public-capacity claim. |
| J08 update, rollback, and recover authority | `cmd/ardents-release.run` is the only production caller of the current Release/Update composition. | `cmd/ardents-release` tests and Update Transaction's restart, rollback, retention, and pressure corpus. | Recovery corpus passes, but real activation and Authority Custody remain unowned/undisposed product facts. |

## Security and privacy invariant trace

These rows trace the twelve H3 invariants in the technical design, not the full
cross-horizon threat-model claim registry. `Documentation-only` means the
constraint is present as an honest limitation but has no current claim-bearing
implementation proof.

| Invariant | Current source or caller path | Current observable evidence | Assessment |
|---|---|---|---|
| Payload protection is not anonymity | Documentation-only: product scope and threat model. | No candidate Qualification evidence. | No anonymity claim. |
| Several project Nodes are not independent operators | Documentation-only: product scope and threat model. | No independent-operator evidence. | No decentralization claim. |
| No silent weaker/direct fallback | `nameresolution`, `serviceconn`, Route, and Bridge behavior boundaries. | Stage 6 no-fallback cells and current resolution/connection behavior tests. | Retained local evidence; complete current route trace is still required. |
| Identities remain separate | Current domain Modules (`naming`, `node`, Network State, service connection) and command-specific input schemas. | Module and command behavior tests. | Structural evidence only; target ownership awaits S8.3. |
| Direct-source exclusion survives its lease | `network/state`, `network/source`, and `localroles`. | `tests/e2e/network-source` and Network State behavior tests. | Current tracer evidence; no supported-platform Qualification. |
| Ordinary Node receives only role-local state | Route/route-plan/current Network State and historical carrier/named-site runners. | Route behavior tests and historical carrier/named-site evidence. | No current external privacy claim. |
| Authority roots are not runtime credentials | Release/Update current records; naming authority and service boundaries. | Release, naming, and recovery Module tests. | Custody is absent, so this is not a complete root-protection implementation. |
| Application Data is opaque to control/diagnostic state | `serviceconn` and `applicationipc` byte-stream seams. | Service Connection stream-semantics and Application IPC tests. | Current seam evidence only. |
| Security objects have lifetime/failure states | Network State, naming lifecycle, node, route, Release, and Update Modules. | Their behavior/recovery suites, including Update R00--R14. | Distributed current ownership is an S8.3 design input. |
| Unsafe readiness fails closed | Network State/source and Node readiness boundaries. | Network-source and Node end-to-end suites. | Current bounded evidence only. |
| Diagnostics are local and bounded | Current command renderers and result structures. | Command tests inspect bounded terminal output. | No accepted final diagnostics owner or export policy proof. |
| Every privacy/security claim states conditions and limits | Documentation-only: threat model/claim registry. | Stage 9 is the required claim-level evidence owner. | Stage 8 makes no final claim. |

## Trace result

Every accepted H3 journey and invariant now has one of: a named source/test
path, a retained historical-evidence path, or an explicit absence. S8.1 decides
which paths become the preserved product contract; S8.2/S8.3 then determine the
test, Interface, trust, and compatibility owners. This matrix does not close
the remaining S8.0 inventories or the G2 delta review.
