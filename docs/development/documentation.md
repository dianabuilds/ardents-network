# Documentation ownership and promotion

This policy governs current technical and operational truth. It does not make a
stage brief, experiment, test, implementation plan, or generated receipt a
second product specification.

## Authorities and reader routes

Use the [repository authority order](../../AGENTS.md#order-of-authority) when
documents disagree. Reader order is not authority order.

Product scope, journeys, and honest promises belong in `docs/product/`.
Threats, protected information, adversaries, conditions, measurements, and
limitations belong in `docs/security/`. Accepted hard-to-reverse decisions live
in `docs/adr/`; decision evidence and falsification live in `docs/research/`.
`docs/development/` owns engineering policy, factual source maps, and developer
workflow. Implementation and operator facts belong in the smallest current
technical, operations, or reference document that has the responsible Module
or command as its source.

The normal routes are: contributor from `README.md` to product scope, threat
model, the affected current technical owner, and package map; operator from
the runbook to command and configuration reference; auditor from product/threat
contract to current technical/format/Qualification facts and then selected
ADR/research provenance. An ADR index, research queue, experiment directory,
receipt, compatibility tree, and Git history are not default implementation
routes. No reader should have to infer implemented behavior from stage
chronology.

## Promotion and retirement

A code, format, command, support, or operational change updates its current
owner in the same change. It states normal and failure behavior, boundaries,
compatibility/retirement conditions, and honest limitations where applicable.
An ADR or research record remains rationale/evidence after that promotion; it
does not become the operator manual.

Retain accepted ADRs, completed research records, and immutable audit receipts
as decision evidence. Retire superseded procedures, design templates, and
experiment instructions to Git history after promoting unique current facts
and repairing links. A historical path quoted in a completed record identifies
that record's source revision; it does not promise a current procedure.

Each exact requirement has one current owner. The functional map owns product
requirement IDs and performance budgets; the operating model owns lifecycle
and public control thresholds; the threat model owns adversaries and claim
conditions. Journeys describe observable behavior and link to those rules.
Vision summarizes direction. Cross-references preserve a requirement without
copying its numbers, state machine, or qualification matrix into every reader
route. C0 scope selects the applicable subset; a public target is not an
implemented C0 guarantee.

Stage material is transitional. Before it is deleted, its unique current fact
is promoted to one canonical owner, inbound links are repaired, and any needed
historical source identity or claim evidence remains recoverable in Git or the
declared external evidence location. A document is not retained merely because
it is lengthy, stage-named, or previously reviewed.

New documentation earns a separate file only for a distinct authority,
audience/task, change cadence, or audit-retention need. Do not mirror package
file order or create empty future manuals. A target technical, operations, or
reference document first appears with real owned behavior; a package rename is
not a documentation boundary by itself.

The C0 delivery backlog and its live status belong in GitHub Issues under the
[`C0 Closed Alpha` milestone](https://github.com/dianabuilds/ardents-network/milestones),
not in an ADR, research record, experiment README, or product contract. Its
absence blocks a new C0 implementation slice. At most one C0 implementation
issue and one explicitly selected research question may be in progress. A
current document may link to a tracker item for operational status, but it
stays a stable contract rather than a second backlog.

## Research-to-implementation handoff

The [repository collaboration policy](../../AGENTS.md#agreement-system-design-and-implementation-responsibilities)
assigns R-149 product/architecture work to the design assistant and intended
implementation to Terra. Preserve one Product Owner's decisions and the actual
one-human capacity. No model handoff substitutes for independent qualification.

For the R-149 participant-selection and voting core, a precise individual brief
is insufficient until the whole core has a coherent selected architecture.
Resolve the critical admission, pool/bootstrap, randomness, voting/finality,
time/recovery, storage, interface, privacy and resource dependencies together,
with the evidence required for their claims and necessary ADRs. An unresolved
component that can change those contracts blocks dependent implementation;
an assumed honest service, mock proof or unchecked callback cannot close the gap.
Future application scenarios need not all be specified, but their supported
extension/effect boundary must be explicit. The
[research readiness assessment](../research/records/r-149-autonomy-transition.md#participant-selection-and-voting-core-readiness)
records open obligations; it is not the selected runtime specification.

### Task admission after completed design

**Product Owner clarification, 2026-09-07:** finish the product design of the
affected scope before creating its tasks. The design assistant resolves the
necessary architecture, mechanisms and dependency choices as part of that
preparation. Do not turn an unresolved product choice into a delivery task,
including a research-labelled task that merely passes that choice to its
executor. Research questions and evidence remain part of design under the
existing research discipline; a coverage map is working design material.

Before task creation, the selected scope must have concrete user outcomes and
non-goals, protection claims and limitations, normal/refused/interrupted
behavior, accepted cost bounds, chosen mechanisms, Module ownership and
Interfaces, lifecycle and compatibility rules, reviewed dependencies, and an
executable verification plan. Consequential decisions belong in their current
owners and accepted ADRs where required. For coupled behavior, establish one
coherent architecture before decomposing it into implementation tasks.

A task may depend on another implementation task whose contract is already
defined. An undecided product behavior, privacy condition, architecture or
library is not that kind of dependency. Only routine implementation choices
within the selected contract remain with the executor. Unrelated future
product scenarios do not block a bounded scope with a defined extension and
effect boundary. If new evidence invalidates a selected decision, return the
affected scope to design before dependent work continues.

After these conditions are met, prepare one bounded issue/brief with:

- the observable outcome, affected current owner and exact scope/non-goals;
- links to the applicable current requirements, threat conditions and accepted
  ADRs, with research sections only for necessary rationale;
- precise permitted inputs, effects, ownership and privacy invariants, lifecycle,
  conflicts, failure/unresolved behavior and whole-role resource bounds;
- the selected technical/proof/dependency decisions necessary for the slice;
  any remaining out-of-scope unknown must have no effect on its admitted path;
- concrete behavior/adversarial acceptance cases and required execution profiles;
- compatibility/migration obligations and the evidence needed for review.

The brief supplies a focused reading route instead of requiring the implementer
to infer the design from a long conversation or every historical research
record. It references authoritative contracts rather than copying a divergent
second specification. A remaining decision that changes the slice's security,
authority or observable behavior keeps that scope in design before its task is
created; routine coding details within its contract belong to the implementer.

The completed design handoff supplies dependency-ordered development and
verification tasks with requirement/ADR traceability. Each development task
includes its own behavior checks; explicit integration, adversarial, resource
and recovery tasks verify the complete supported core. State required execution
environments and acceptance evidence. Keep unresolved design work separate from
implementation-ready tasks and live status in the selected issue tracker.

Terra's implementation handoff returns the changed behavior, relevant diff and
owner-document updates, validation results and limitations. The design assistant
reviews conformance to the requirements/ADRs and addresses consequential design
gaps through their owning documents. Do not silently expand the contract,
weaken a gate or label proposed evidence accepted to complete a coding slice.
Other unaffected work may continue within its existing authorized bounds.

This workflow does not create an implementation issue, choose a public protocol,
authorize a new maintained package or amend accepted ADRs. The selected tracker
continues to own live implementation status; C0 retains its milestone and WIP
prerequisites. R-149 stays in design until an explicitly selected slice satisfies
these conditions.

## System protection workstream

The Product Owner selects a full product and technical workstream under
[ADR-0077](../adr/0077-evolve-one-common-protection-baseline.md): review a
substantial part of Ardents functionality, revise the relevant product and
technical decisions, integrate selected solutions into maintained behavior,
and test the resulting system in a declared test environment. The intended
result is an improved common protection baseline demonstrated by a working
candidate and evidence from its supported journeys.

The [privacy and anonymity integration map](privacy-anonymity-map.md) is the
current reader and implementation-dependency route for the selected closed
successor. It maps existing owners, selected dependency uses, implementation
outcomes and verification; public naming/autonomy remain separate design. Its identifiers are planning references, and
the issue tracker continues to own selection and execution status.

Start with one coverage map of the current
[product outcomes](../product/functional-map.md#outcomes-before-mechanisms) and
[whole-system threat coverage](../security/threat-model.md#whole-system-protection-review).
For each affected operation identify its current owner, inputs, data and powers
exposed across the complete lifecycle, claimed property, actual evidence and
unresolved decision. Include the exact dependency and build/tool closure under
the [maintenance and vulnerability acceptance rule](dependencies.md#maintenance-and-vulnerability-acceptance).
Assess existing selections as well as proposed ones; support evidence and any
advisory applicability argument belong with that dependency's owner.
Existing architecture is reviewable: an inadequate product
promise, protocol, Interface, authority or lifecycle returns to its owner for
an explicit researched revision. The map references authoritative requirements;
it does not become a second requirement registry or live backlog.

Carry the work through successive bounded slices:

1. Select one coherent user outcome and its dependencies from that map. Review
   the combined system behavior before assigning implementation work to Modules.
2. Resolve the decision-relevant question under the research discipline. Define
   observable normal and hostile behavior, protection metrics, complete resource
   costs and falsification criteria before experiments or mechanism selection.
3. Complete product design and the necessary architecture/mechanism choices,
   then create tasks under the
   [task admission rule](#task-admission-after-completed-design). Prepare the
   implementation handoff with the accepted product/security contract,
   consequential ADRs, concrete integration and migration rules, and the exact
   test-environment plan.
4. Implement the selected behavior through its maintained owners and normal
   consumer Interfaces, with their documentation and behavior checks. A slice
   includes its security obligations and complete failure lifecycle. It must
   not rely on a test fixture implementing missing product behavior.
5. Build and identify the exact changed candidate. Run it through real commands
   and Modules on the declared environment, exercise the complete journey, then
   its selected attack, overload, interference, restart and recovery cases.
   Compare the result and total cost with the predeclared baseline and criteria.
6. Resolve findings in the responsible product, security or technical owner,
   implement the required correction and repeat affected checks. Then select
   the next coherent slice; new evidence may change earlier design assumptions.

The environment plan states platform, process/host topology, owned roles,
configuration and state preparation, workload, adversary powers, fault controls,
resource limits, measurements and external evidence location. Use the applicable
[checked execution profiles](testing.md); a missing selected prerequisite is an
invalid environment. Test commands exercise the maintained product. Synthetic
traffic and adversarial peers are declared test inputs, not substitutes for its
implementation. Local deterministic checks are necessary evidence for their
scope; they do not complete the system trials.

Completion requires the selected scope to work through the integrated paths,
the applicable normal/adversarial/resource acceptance checks to pass for an
identified candidate, and the resulting operational behavior, claims and
remaining limitations to be promoted to their current owners. The candidate's
dependency support and advisory evidence must meet the acceptance rule; after
an update or replacement, repeat the affected integration and system trials. An unresolved
required property is a finding to resolve or an explicit product-scope decision,
not a passing result. A requirements document, ADR or test plan alone does not
complete the workstream. Project-controlled trials do not manufacture public
anonymity, independent operators or independent security review; those claims
retain their separate evidence gates.

The selected issue tracker owns execution and status. Keep the existing C0
milestone and one-slice/one-active-question limits; the workstream does not
activate all of its areas at once. Its framing is recorded by
[R-150](../research/records/r-150-common-protection-baseline.md), and the current
R-149 subject remains separate design. Completed R-152/R-099 selections live
in their current owners and ADR-0081; their former unanswered choices are not
active research prerequisites for the Link-first scope.

## Security work in an implementation slice

The workstream and later ordinary changes use the same
[common protection contract](../product/operating-model.md#common-protection-baseline).
Place each protection with the Module that owns the relevant state, authority,
resource or information flow; create a new Module only for a demonstrated
cohesive responsibility under the package rules. Callers and tests use the
same Interface. Preserve its existing consumer semantics or explicitly decide
and qualify the necessary migration. Do not add speculative security packages,
parallel protection modes, weaker failure paths or unqualified trusted flags.

Every handoff names the affected complete journey, observer/authority crossings,
required refusal and cleanup behavior, implementation and test owners, baseline
comparison, and evidence invalidated by the change. Retained claims and
performance gates remain binding until their owning decision changes them.

## Checks

The package map and dependency register remain factual and are architecture-gate
inputs. A retained command/configuration/format compatibility promise has a
named current reference and a behavior or compatibility test. A security or
privacy claim links to its product/threat statement and the named Qualification
evidence instead of duplicating a stronger shorthand claim in implementation
documentation.
