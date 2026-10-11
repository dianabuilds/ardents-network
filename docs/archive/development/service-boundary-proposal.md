# Service lifecycle boundaries proposal

Status: draft architecture analysis, not an accepted contract, implementation
assignment, qualification receipt, or replacement package map.

Analyzed on 2026-10-02 against local `dev` commit
`a2918a8c691522e903a2f4bb48917f355fb6554b`. The existing modification to
`repository-reconstruction-findings.md` was excluded and preserved. No product
code was changed or tests executed for this analysis. Concurrent repairs must
be reconciled against the next implementation baseline.

The proposed direction is a protected distributed network with stronger traffic
protection. Exact additional adversaries, measurements and protection claims
remain to be selected. Current authority, role separation, finite resource and
cleanup requirements remain obligations. OpenTelemetry is the Product Owner's
preferred telemetry direction, not an installed dependency or accepted exporter
configuration.

## Scope and evidence

This document analyzes publication, private Target lookup, Service Connection
binding, refresh and retirement. Facts below come from production source;
architectural consequences are interpretations, not newly confirmed bugs.

Current contract owners are product scope, threat model,
`docs/technical/endpoint-service-runtime.md`, private reachability, Application
confinement and common privacy architecture. The factual package map and agent
execution policy govern any eventual implementation. Historical findings and
the `old` branch do not supply requirements.

The app snapshot showed an active chat named `Найти противоречия в проекте`
examining qualification and Hosting accounting. Other listed chats were not
loaded; that state does not prove their work or pending repairs have finished.
No messages or implementation assignments were sent to other chats.

## Current owners and proposed responsibility

| Behavior | Current source owners | Proposed responsibility |
|---|---|---|
| Local admission | application/broker; endpoint/duty_context.go | Broker owns permission and revocation; scenario owner consumes a terminating lease |
| Application invocation | endpoint/job_lifecycle.go; endpoint/worker | Execution owns one invocation and joined cleanup; worker remains a platform mechanism |
| Instance signing | service/instance | Keep non-exporting key ownership and bounded signing operations |
| Publication readiness | endpoint/descriptor_publication.go; endpoint/introduction/pair_lifecycle.go; service/publication | One Publication lifecycle owner coordinates registration, proof, ACK and readiness |
| Publication refresh | endpoint/publication_refresh.go; endpoint/publication | Publication owns revision and switching policy; scheduler is internal mechanism |
| Private lookup | endpoint/resolution.go; endpoint/descriptorhistory; service/reachability | Reachability operation owns exact Target verification, history and finite lookup lifecycle |
| Logical Connection | endpoint/service_binding.go; service/connection; endpoint/service | Connection owns immutable destination, attachment continuity, recovery and terminal result |
| Token stock | endpoint/tokens; endpoint/tokenjournal | Admission stock owner retains permission, pending issuance and consumption semantics |
| Protected path | entry; route/client; endpoint/source and interior selection | Preserve Endpoint-selected paths; distinguish selection policy from transport lifecycle |
| Context shutdown | endpoint/duty_context_retirement.go | Explicit scenario supervisor orders cancellation and joining across owners |

These are responsibility proposals. They do not require a new package for every
row and do not authorize the illustrative directory tree from the conversation.

## Boundary contracts observed in code

| Transition | Admission and commit | Required failure semantics |
|---|---|---|
| Create duty context | Broker activates exact capability, Principal and surface before effects | Failure releases the acquired lease; foreign surface cannot create Publisher authority |
| Register to publish Descriptor | Administration context, live profile, registration, Instance binding, Source acquisition and available resolution flight | No worker-supplied signer, Target or recipient; concurrent withdrawal/opening/drain denies admission |
| Send Descriptor | Retain exact proof bytes and recipient for exact retry; consume actual token stock | Failed exchange is not readiness; retry must not silently replace proof or key |
| Accept publication ACK | Recheck profile, owner, Instance binding, registration, Source flight, cancellation and recipient | Late ACK cannot revive retired authority; verified ACK switches registration and starts refresh |
| Lookup Target | Connection surface, live permission and Source, finite history capacity, available flight | Verification uses independently selected Target; lookup success is not connection readiness |
| Accept lookup result | Recheck exact flight, profile and caller lifetime before history acceptance | Changed authority or canceled caller refuses; no alternate Target fallback |
| Refresh | Retain current/pending/previous registration and original timing | Exact retry does not renew original lifetime; successor readiness requires ACK |
| Retire context | Stop children under shared lock; join outside it in explicit dependency order | Job reservation survives until child completion; cleanup errors remain observable |

Publication ACK checks are in `descriptor_publication.go:150` onward. Lookup
checks are in `resolution.go:19` and `acceptResolutionResult`. The transition
policy lives in `introduction/pair_lifecycle.go:223` onward. Retirement is in
`duty_context_retirement.go:35` and `:71`.

## Architectural findings

### Shared synchronization crosses extracted packages

Code fact: dutyContextState contains publication, introduction, resolution,
Descriptor history, Source, token authority, lease and Job state. tokens.Owner
is initialized with the same mutex. Its Host interface includes Locked methods,
State profile, journal, lease, bootstrap selection and prefix-current checks.

Interpretation: physical extraction has not produced independent state ownership.
The interface requires callers and implementations to understand the parent
coordination protocol. This is not evidence that the shared lock is incorrect.

Proposal: enumerate every invariant protected by that lock before changing it.
Separate owner-local invariants from atomic cross-owner admission. Preserve a
single admission/retirement authority where needed; do not replace immediate
revocation with eventual event delivery.

### Publication is distributed across policy and mechanism owners

Code fact: service/publication owns the durable generation and drain;
endpoint/publication owns scheduler mechanics; introduction owns registration
pairs; Endpoint validates proof effects and publishes readiness.

Interpretation: callers must reconstruct the semantic meaning of publication
from multiple state machines. Proposal: place the complete public lifecycle
behind one owner, with registration and scheduling as implementation details.
Node Store commit remains independent and does not prove live Publisher readiness.

### Connection binding carries multiple responsibilities

Code fact: serviceBinding retains Job, Credential, protected context facts,
destination binding, Introduction recipient and recovery owner. The service
mechanism receives authority through Binding; root Endpoint coordinates recovery.

Proposal: Connection should own continuity and recovery admission. It consumes
a terminating local execution lease and bounded verified destination facts;
it must not acquire Authority custody or choose another destination.

### Qualification is an active scenario driver

Code fact: qualification.Session exposes provisioning, token reserve management,
Introduction preparation and stream opening. PublisherWorker exposes prefix
replenishment and issuer reserve operations.

Interpretation: measurement and production work coordination need separate
review. This does not establish a qualification bypass. Proposal: distinguish
workload driving, production lifecycle, measurement and verdict calculation;
use independent numeric expectations for budget and duration checks.

## Traffic protection and telemetry boundaries

For each transition, record permitted knowledge and retention separately for
Application, local Endpoint, adjacent role, Interior, Introduction, resolution
role, Rendezvous and issuer. Include failures, retries, refresh and recovery.
The proposed component map alone does not prove location privacy or resistance
to traffic correlation. Role-local data must not become a shared global object.

Telemetry proposal: owners define finite categories, counters and lifecycle
observations; runtime composes OpenTelemetry providers and bounded export;
Collector and storage remain external. No destination, secret, permission body
or raw error is exported by default. Trace IDs are local in scope and are not
automatically propagated over Ardents role channels. Export queues, drops,
shutdown and unavailable Collector behavior need explicit bounds and tests.
Mandatory accounting and security state cannot depend on sampled telemetry.

Candidate observations are admission refusal category, operation duration,
in-flight count, ACK rejected after retirement, refresh outcome, cleanup duration,
reserved/consumed budget and telemetry drops. Attribute cardinality and temporal
disclosure require review before enabling export.

## Verification before migration

These are proposed acceptance scenarios, not tests already run:

1. Revoke during registration, publication exchange and lookup; reject late completion.
2. Change profile between send and ACK; do not publish readiness.
3. Retry exact publication; preserve proof bytes, recipient and original expiry.
4. Fail successor refresh; preserve only the predecessor's bounded accepted lifetime.
5. Lose Source during lookup; join acquired resources and preserve token-spend semantics.
6. Replace Connection Attachment; retain destination, authority and original work limits.
7. Stop during every opening phase; observe no surviving child work after joined completion.
8. Verify accounting against independent values, including overflow and unit boundaries.
9. Saturate telemetry export; bound memory and preserve production admission and cleanup.

## Migration proposal

First reconcile the active repairs and source commit, then inventory existing
tests against the scenarios above. Extract the Publication lifecycle first if
its files are free of active repairs: move state and transition authority together,
switch real consumers, eliminate the old path, update factual owners and run
required checks. Follow with Reachability and Connection using the verified
contracts. Keep one implementation slice active and respect project WIP limits.

No branch, issue, ADR, package, dependency or wire change was created by this
analysis. Further architectural selection requires reviewing the concrete
contracts and compatibility implications, not adopting this draft implicitly.
