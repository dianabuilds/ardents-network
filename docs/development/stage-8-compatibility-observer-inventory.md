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
| `cmd/ardents-route`, `cmd/ardents-service`, `cmd/ardents-bridge`, `cmd/ardents-stream-app`, `cmd/ardents-publish-app`, `cmd/ardents-name` | Root `README.md` lists them as current Stage 1-6 tracer commands. | A current repository statement exists, but it names no external caller, version, or support window. Under the Product Owner's standing Stage 8 delegation, M5 removed the unobserved `ardents-name validate-record` C0 subcommand; its raw Record codec remains available only to Namespace and the named C4 evidence path. M13 still decides the remaining command shapes. |
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
