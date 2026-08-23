# Stage 8 compatibility-observer inventory

Status: **S8.3 factual input for DA-10; not a compatibility decision.**
Question authority is accepted [R-058](../research/records/r-058-h3-reassessment-and-closure.md);
the decision route and stop condition are DA-10 in the
[decision-authority register](stage-8-decision-authority-register.md). This
inventory is limited to repository-visible observers at `5d764ee`; it cannot
establish that a Product Owner, operator, or external Application has no
unrecorded consumer.

## Question and falsification criterion

Before selecting `read/migrate`, `break`, or `delete` for an old command, local
protocol, plan, result, or evidence shape: which observer has a bounded support
obligation?

The preliminary conclusion below is falsified by any source-controlled script,
packaging/deployment definition, current product/operations/reference promise,
or Product Owner-provided external consumer that requires the observed syntax
or bytes. A falsifier adds the observer, required behavior, support horizon,
and migration test to the owning S8.4 wave. It does not restore a format merely
because a historical stage document described it.

## Method and sources

On 2026-08-22, the repository was searched for the current non-laboratory
command names and `ASRS`, `ARDENTS_STREAM`, `.result`, and local socket terms.
The audit examined `README.md`, `CONTRIBUTING.md`, `.github/`, `scripts/`,
`packaging/`, `deployments/`, and current `docs/product/`, `docs/security/`,
`docs/reference/`, and `docs/operations/` zones. It distinguishes source code,
the factual package map, and completed-stage material; older briefs are
provenance, not a support promise under the documentation policy.

## Repository-visible observers

| Surface | Repository-visible observer | Result |
|---|---|---|
| `cmd/ardents`, `cmd/ardents-node` | Root `README.md` calls them current maintained commands. | Retained product journey, but syntax/plan/result compatibility is not established. M13 must publish a current command reference before a support promise exists. |
| `cmd/ardents-stream-app`, `cmd/ardents-publish-app` | The former root README statement was the sole observer; the process e2e suite is their only current caller. | **C0 completed, 2026-08-23:** the commands are now explicit `tests/e2e/service/fixturecommand` fixtures, built only by the recovery process test. Their unit suites belong to the e2e process profile. The Endpoint Application stream and separate publication authority boundary remain tested; neither tracer is product UI. |
| `cmd/ardents-route` | No active command, package-map, profile, README route, or repository caller remains. | **C0 completed before M13:** commit `6b6e3c2` removed the legacy H3 Route process and its `routeplan`; the current Endpoint retains its private Route Attachment journey. No operator Route tracer is retained. |
| `cmd/ardents-name` | The former root README statement was the sole observer. | **C0 completed, 2026-08-23:** `ardents name encode|resolve|control` retains the selected canonical-name, private-resolution, and private-control journeys. The former standalone executable has no reader, adapter, or support window; its inputs remain target-owned temporary inputs, not general operator configuration contracts. |
| `cmd/ardents-bridge` | The former root README statement was the sole observer. | **C0 completed, 2026-08-23:** `ardents entry import <entry-import-plan.json>` retains the one signed State-referenced Entry Invite import journey. The former standalone executable has no reader, adapter, or support window. The input remains an explicitly temporary Entry-owned plan, not a general operator configuration contract. |
| `cmd/ardents-service` | The same README statement was the sole observer. | **C0 completed, 2026-08-23:** `ardents endpoint run <endpoint-plan.json>` replaces the unversioned tracer binary and keeps the Endpoint-owned process journey. The old executable has no reader/adapter or support window. |
| `cmd/ardents-release` | Package map and completed S7.2 briefs reference offline import/apply behavior. | Technical input only under S8.1; no current product/operations support promise found. DA-09 and DA-10 control removal or bounded export. |
| A01/A02 local Application stream, `ASRS` terminal frame, `.result`, raw tail | Source/e2e and current surface inventory show test/tracer use. | No current external Application contract, versioned reference, or non-test consumer found. Preserve one classified terminal result, not the representation. |
| A03 publication Administration socket | `cmd/ardents-publish-app` and source tests use it. | No current operator reference or external automation found. Preserve authority separation, not `publish`/`published` text. |
| A05 plans and A06 JSON results | Command/source/e2e tests and stage records consume them. | No current reference schema or support version found. Tests are characterization input, not external observers. |
| A07 `ARDENTS_STREAM_*` and direct stream modes | Source and live/tracer harnesses use them. | Test-only workload evidence unless a Product Owner names a consumer; not shipped configuration. |
| Laboratory commands, manifest/verifier formats | Historical profile manifests and readers consume them. | R-080 makes the Stage-5 Bridge/WebTunnel subset C4 provenance only; it is not product compatibility or native-profile Qualification evidence. M14 decides the remaining corpus. |

No reference was found in source-controlled packaging, deployments, CI, or
operations/reference documentation that invokes the old product tracer commands
or fixes a public configuration/result/IPC version. Their absence is evidence
only about this repository, not about private automation or an installed
population.

## Required Product Owner input before mutation

For each currently observable external consumer, provide its command or local
protocol, deployed version/source, required behavior, maximum support window,
security-forward-only constraints, and whether a coordinated switch is
available. If no consumer is named, the Product Owner may authorize a
coordinated break for that surface; M13 records that decision and removes the
old reader/writer/test in the same wave.

Until that input exists, S8.3 permits only design/characterization. It does not
permit removal of a command, socket/frame decoder, plan/result parser, or
historical verifier on an assumption that nobody uses it.
