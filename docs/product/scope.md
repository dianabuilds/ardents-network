# Product scope and delivery boundaries

Status: **accepted scope control**

Accepted: 2026-08-08

Amended: 2026-08-24

This document is authoritative for **what may be built now**. The vision,
functional map, threat model, operating model, research records, and ADRs may
describe later public-network obligations; they are not an undifferentiated
implementation backlog.

## Audit verdict

The product goal did not become incoherent, but the delivery scope did expand
too far. Product invariants, the first tracer, public-beta launch controls, and
stable-network qualification were all described as one `V1`. That accidentally
made permissionless naming, hostile bootstrap, Bridges, multiparty governance,
reproducible releases, two desktop platforms, general Application isolation,
and a large performance matrix appear to be prerequisites for the next line of
research code.

That interpretation is rejected. Ardents now has one Product Core and five
explicit delivery horizons. A
decision can be fixed for a later horizon without entering the current backlog.
The horizons are conditional research gates, not a committed roadmap: passing
one does not oblige the one-to-one team to start the next.
The unqualified term `V1` is retired from current planning; older records using
it mean the first public product contract, not the next prototype.

## Product Core — not a delivery horizon

The Product Core is the smallest durable promise that defines Ardents:

- a local client Application opens a connection to a location-independent
  Service Target backed by one active local Service Instance;
- the exact Target and current Instance are authenticated, and Application Data
  is confidential and integrity-protected end to end;
- a low-latency multi-hop Route aims to hide each endpoint's ordinary location
  from the opposite endpoint and to keep the direct origin-to-destination/full-
  Route binding out of one ordinary Node's permitted role-local view;
- Applications exchange a live reliable ordered byte stream through a local
  Application Interface and own every higher-level data meaning;
- failures, unavailable destinations, resource exhaustion, and unsupported
  security profiles are explicit; there is no direct-network or weaker-profile
  fallback;
- the network does not index an exact human-readable Service Name, but anyone
  already knowing it may attempt to connect; a machine Target Link remains a
  complete naming-independent path;
- independently operated infrastructure is required before Ardents may claim a
  decentralized public network.

The Product Core fixes information-flow and responsibility boundaries. ADR-0009
selects Go as the maintained project foundation, but dependencies, wire format,
cryptographic suite, discovery mechanism, route implementation, and operating
model remain separate decisions.

The five-position split-circuit Route with a separate Introduction is the first
Carrier Lab candidate. Four Role Domains, Candidate View, and source-exclusion
are conditional designs for enforcing the same promise in a later public
network; they enter experiments only if the data-path candidate survives and
only as separately promoted slices. Neither set is an irrevocable product
feature. A maintained alternative may replace them only if it satisfies the
same claim and evidence contract without weakening the Application Interface.

## Horizon 1 — Completed Research Slice: Carrier Lab

The **Carrier Lab** was the first authorized implementation slice and completed
Gate B with `advance`. R-091 retains its accepted records as C4 provenance and
retires its execution corpus; it is not a current regression interface, release,
or production network.

It contains only:

1. Ubuntu LTS `x86-64` controlled client and publisher fixtures;
2. one deterministic Application byte-stream workload and one active Service
   Instance;
3. ephemeral project-owned test keys and a preconfigured authenticated Target/
   reachability fixture;
4. a fixed synthetic topology with the current five-position split-circuit
   candidate and separate Introduction, plus only the comparison controls needed
   to falsify that candidate;
5. exact Target/Instance authentication, end-to-end confidentiality and
   integrity, fresh connection keys, and explicit failure;
6. per-role traffic/state inspection proving what each position can and cannot
   learn;
7. coarse setup-latency, one-stream goodput, memory, CPU, and one injected path-
   failure measurements sufficient to decide whether further work is rational.

Carrier Lab deliberately does **not** implement:

- Service Names, Namespace allocation, Name recovery, or private naming;
- public discovery, permissionless Node admission, operator incentives, or Sybil
  defense;
- Network Epoch governance, threshold ceremonies, transparency auditors, or
  independent builders;
- hostile first-install bootstrap, Bridges, pluggable camouflage, or censorship
  UX;
- production installers, automatic updates, Authority Recovery Bundles,
  uninstall/purge workflows, or long protocol migrations;
- Windows support, the full cross-platform matrix, `64/256` concurrency, or the
  complete R-023 qualification suite;
- a public SDK, generic proxy, browser, Application sandbox, or malicious-
  sibling Application isolation;
- a public anonymity, decentralization, availability, or production-security
  claim.

The harness Applications have no ordinary network access by construction, but
this is a controlled-test condition, not a reusable Application isolation
product.

### Carrier Lab lifecycle boundary

| Area | In Carrier Lab now | Explicitly later |
|---|---|---|
| Installation | Developer-run Ubuntu fixture; setup may be scripted only for repeatability. | Supported package, unprivileged installer, repair, recovery, uninstall, or end-user UX. |
| Warm-up and readiness | Preconfigured authenticated test state and a fixed synthetic participant set; report whether the exact lab path is ready. | Hostile first-install bootstrap, peer expansion, Bridge acquisition, public readiness, or stale/fork recovery. |
| Operation | One active Instance, one ordered byte stream, role-local inspection, explicit terminal results, and one injected path failure. | General route optimization, multipath, public diagnostics, arbitrary Applications, retained delivery, or operational support tooling. |
| Security | Exact Target/Instance authentication, end-to-end payload protection, fresh connection keys, fail-closed behavior, and inspection of the permitted view at each synthetic role. | Production key custody, public admission, governance, update security, platform sandboxing, or a claim against adversaries not exercised by the lab. |
| Performance | Coarse comparable setup latency, one-stream goodput, CPU, RSS, and failure-reaction observations with stop conditions. | Product SLOs, capacity floors, long soak/load cells, cross-platform qualification, or the complete R-023 evidence bundle. |
| Updates | Replace and rerun the disposable experiment from declared inputs. | In-place update, rollback, protocol transition, reproducible release, or long-lived state migration. |
| Privacy | Attempt to falsify the candidate's role-local knowledge separation and record observed traffic/state. | A public anonymity, unlinkability, decentralization, censorship-resistance, or Broad Traffic Observer claim. |

Carrier Lab stops when it can answer one question: **is the current route
candidate plausibly capable of the Product Core's knowledge separation and a
useful live stream on modest hardware?** Failure reopens route design before
naming, governance, packaging, or public-network work begins.

## Horizon 2 — Conditional Reference Application: Named Unlisted Site

The first Reference Application is **Named Unlisted Site**. Carrier Lab produced
a viable route candidate, and official Gate C run `gatec-31464163490-1`
completed this controlled Ubuntu slice with `advance`.

Its first controlled slice adds only:

- the stable local Application Interface and Service Target/Instance Credential
  lifecycle;
- private reachability resolution;
- one pre-provisioned exact Service Name and private resolution binding to the
  Target;
- a deterministic local HTTP Service and single-response client;
- ordinary one-Instance host migration and explicit offline/failure behavior;
- Ubuntu-to-Ubuntu first, followed by Windows cells only after the contract is
  stable.

It does not yet require permissionless name claiming, leases, delegation,
catastrophe recovery, anti-abuse allocation, general browser support, public
Contributor operation, or production release governance. Those mechanisms may
be prototyped separately, but do not join shared product code merely because the
full public contract eventually needs them.

Gate C completion means only that the closed tracer worked: 20/20 positives,
17/17 required failures, and 5/5 migrations passed with complete cleanup. It
does not promote the laboratory Route/OHTTP protocol, qualify a privacy claim,
or satisfy public naming, platform, operator, or release gates.

## Horizon 3 — Closed Test Network

The Named Unlisted Site slice now works in its bounded Gate C environment, so
Horizon 3 is the next permitted scope. Ardents may assemble one separately
scoped persistent multi-host test-network slice at a time. It remains
project-key-controlled, invite-only or
otherwise bounded, visibly centralized, and unable to make a public anonymity or
decentralization claim.

This horizon may add one vertical slice at a time:

- authenticated epoch/bootstrap distribution and finite source handling;
- public-role process separation, bounded Node admission, and withdrawal;
- Bridge and transport-camouflage experiments;
- permissionless Namespace, lease, recovery, and private-resolution candidates;
- install/update/rollback and authority-recovery prototypes;
- Windows compatibility plus brokered Application-isolation experiments;
- broader recovery, overload, and role-capacity measurements.

Each slice has its own stop condition. Passing the Named tracer does not
authorize building all of them in parallel, and a project-operated test network
is not evidence of independent operation.

Horizon 3 is closed as a source of maintained behavior. Its laboratory results
are neither a product contract nor Route Qualification. Current Network State,
Entry, Route, and Node constraints are owned by
[Network Route and Node](../technical/network-route-node.md); endpoint and
Service behavior is owned by
[Endpoint and Service runtime](../technical/endpoint-service-runtime.md).

Any future work starts from the current product, security, technical, and ADR
owners. It must define a new bounded claim and evidence plan; it cannot revive
H3 stage plans, wire bytes, capacity figures, or a previous campaign as an
implicit compatibility requirement.

## Horizon 4 — Usable Network Alpha and Public Beta

Horizon 4 turns the retained Product Core and current technical contracts into a
network that a person can actually use. Its first product proof is deliberately
concrete: a User installs Ardents, starts a local Endpoint, obtains an explicit
Target Link or Service Name, and opens a Service published by another Endpoint
over a live multi-host network. A web Service may be the first Reference
Application: a Developer publishes a local HTTP Service and a User opens it in
an existing browser through a local Ardents Adapter. Ardents is not thereby a
browser, public DNS, clearnet exit, or generic anonymous-Internet proxy.

An externally usable alpha is a product milestone, not a Public Beta claim. It
may use an explicitly bounded participant set and measured, known operational
conditions. It cannot claim independent public operation, browser-level
location privacy, censorship resistance, or a public permissionless Namespace
merely because the User journey works. A Product Owner walkthrough can accept a
bounded product tracer, but is not evidence that new users understand it or
that independent operators exist.

The following are ordered **H4 epics**. They are not authorization to build all
work in parallel. Before an epic enters implementation, it needs a named
bounded claim, evidence plan, stop condition, and current Product Owner
selection. An epic may supply an alpha capability before every later Public
Beta gate is met; it may not silently inherit a stronger claim.

### [H4-1 — Endpoint lifecycle and distributable profiles](horizon-4/01-endpoint-lifecycle.md)

Deliver an unprivileged Endpoint lifecycle, beginning with an authenticated
Ubuntu LTS Portable profile that is a real alpha product rather than a
developer checkout. A Windows 11 build is a best-effort experimental companion
until its real execution/provenance path is evidenced; it does not block the
Ubuntu-first alpha. The first Windows artifact is explicitly unsigned; the
project does not purchase OV, EV, or a hosted commercial signing service for an
alpha without users. A free OSS signing path may be evaluated later but is not
a release prerequisite. Installed profiles remain optional, one platform at a
time, when observed user needs justify them. Every profile must keep immutable
executable bytes, release/update state, disposable cache, runtime state, and
Authority Vault materially separate.

The completed public-product epic includes signed packages, repair, safe
update, rollback protection, uninstall, and explicit Authority backup/recovery.
Repair and uninstall must never silently erase Authority material; a rejected
or interrupted update must never activate an unverified or older build. The
existing Release, Update, and Custody modules are technical inputs, not proof
that the supported platform lifecycle exists.

### [H4-2 — Reachable live network and transport operation](horizon-4/02-live-network-transport.md)

Turn current Route, Entry, Node, and state contracts into a repeatable live
multi-host operating profile. It includes authenticated bootstrap, finite
direct-source handling and separation, explicit blocked/degraded outcomes,
an explicit decision whether any blocked-entry profile is justified, transport
replacement, and bounded recovery. Each selected
transport and Entry profile requires its own evidence; a former H3 wire or
campaign is not retained by default.

The functional alpha maintains State-selected TCP/TLS and QUIC-v1 Carriers
without implicit fallback. It selects no Bridge/camouflage profile and makes no
censorship-resistance claim; a future profile requires a new bounded decision.

The alpha outcome is a User and Publisher on separate endpoints that can become
ready, connect, recover or fail explicitly through remote Node roles. It does
not imply public censorship resistance or that a project-operated topology is
independent capacity.

### [H4-3 — User, Service, and web-access path](horizon-4/03-user-service-web-access.md)

Make the Network usable without requiring an Application developer to know
Route internals. A User can run the supported Ardents binary, receive named
capability readiness and bounded failures, share an explicit Target Link, and
connect to a Service. A Developer can publish one local Service Instance with
separate Service Administration authority.

The first web-access slice uses an explicit Endpoint-to-loopback Adapter. After
the User gives one exact Target Link to the Endpoint and it authenticates one
Service Connection, the Endpoint opens a fresh one-connection
`127.0.0.1:<ephemeral-port>` origin in the selected existing browser. The
Adapter maps only the bounded H4-3A method/resource set and accepts no ordinary
URL, proxy-form target, arbitrary Ardents destination, or administration
operation. Closing the connection removes the origin; it never falls back to
DNS, search, public HTTP, or another Service.

The first controlled Reference Site is a static, exact-resource profile. Its
browser response uses a header-delivered CSP sandbox in addition to restrictive
fetch directives; it does not pass Publisher redirects or cookies. This does
not qualify arbitrary Publisher HTML, scripts, or external navigation.

Ordinary Internet browsing remains outside Ardents and continues by the
browser's ordinary path. A browser extension, custom URI scheme, custom CA,
certificate purchase, browser configuration, system proxy/DNS/VPN change, and
bundled browser are not dependencies of this slice. The loopback address is
presentation plumbing rather than remote Web PKI identity, and it carries no
automatic privacy claim. Optional later browser integration remains explicit,
reversible, scoped, and separately inventoried.

The generic Adapter is a compatibility surface only. Until the complete
browser/Application process tree passes a supported Network-Isolated
Application Boundary profile, it receives no Application-level Endpoint
Location Privacy claim. An exact external-resource, DNS, WebRTC, callback, and
fallback policy is a prerequisite for any later protected browser mode.

### [H4-4 — Canonical names and private resolution](horizon-4/04-namespace-private-resolution.md)

Promote from explicit Target Links to the canonical permissionless Namespace
only when the name product is ready to carry public expectations. This epic
owns lease, delegation where selected, recovery, governance, bounded abuse,
and qualified Private Resolution. A failed Name never rewrites into another
destination or silently falls back to DNS, HTTP, a local alias, or a Target
Link.

Target Links remain a complete alpha destination path while this work is
incomplete. Public human-readable names do not become a directory, search
service, registrar discretion, or application identity system.

### [H4-5 — Contributor viability and permissionless admission](horizon-4/05-contributor-viability-admission.md)

Build an operator product before asserting that a permissionless network will
operate. A Contributor has an explicit role, resource envelope, health state,
update/drain/withdrawal path, and abuse limits. Permissionless public admission
must preserve direct-source separation, role-domain restrictions, finite
probation, and the ability to reject unsafe or exhausted work explicitly.

Voluntary operator participation is a product and research question, not an
assumption. A small alpha may measure known participants, but their machines,
addresses, or nominal Node identities do not count as independent public
capacity. The current dedicated-host qualified Contributor rule remains in
force unless a later accepted research result changes it; see
[R-093](../research/records/r-093-voluntary-endpoint-contribution.md).
Token, staking, payment, and incentive-market systems remain outside the
network core.

### [H4-6 — Transparent control, release, and transition](horizon-4/06-transparent-control-transition.md)

Exercise project-controlled shared-control mechanics through transparent
Candidate View construction, simulated custody/build/audit roles, reproducible
artifacts, and explicit protocol/build transition. This H4-6C result makes no
claim of independence. Release/update safety, Network Epoch state, and protocol
compatibility remain distinct transitions with their own roots, floors, and
failure outcomes.

### [H4-7 — Qualified local Application boundary](horizon-4/07-application-boundary.md)

Where Ardents makes an Application-level location claim, provide
OS-enforced or brokered per-Application principals and a qualified
Network-Isolated Application Boundary on the supported platforms. This epic
adds a protected mode above the generic Application Interface; it does not
retroactively make every local socket, browser extension, or same-user process
isolated. Unsupported platforms or attachment modes fail explicitly and remain
generic/unqualified.

### [H4-8 — Qualification, independent evidence, and promotion](horizon-4/08-qualification-promotion.md)

Run the applicable R-023 cross-platform performance, recovery, overload,
evidence, and requalification cells against the selected live profile. Measure
effective independent operator and authenticated source capacity after every
mandatory exclusion, not merely raw Node counts. Obtain the external security
review and public control evidence required for the exact claim.

Only after these epic-specific conditions and the applicable beta thresholds
pass may Ardents call the running network **Public Beta**. Missing people,
independent operators, auditors, builders, or external reviewers block the
corresponding claim; they do not create fictional work for the current
one-to-one team.

**Stable Network** is a later promotion of a running beta, not Horizon 5: its
larger effective family counts, mature operational drills, and extended
external evidence are evaluated only after beta operation produces real data.

The first public product contract retains only the qualified Interactive Route
claim. Ardents does not reopen the complete security/privacy model or add a
Shielded Route Profile before Horizon 4 is complete. The existing Route Profile
seam remains; it creates no H3 or H4 implementation work and permits no stronger
claim in advance.

## [Horizon 5 — Security and Privacy Model Review](horizon-5/README.md)

Horizon 5 opens only after Horizon 4 has produced the first public product and
Public Beta operational and Qualification evidence. It reassesses the security
and privacy model against observed traffic, failures, concentration, abuse, and
operator behavior rather than expanding the model from controlled-test
assumptions.

The review may reopen the Interactive Route adversary boundary and R-005. A
Shielded Route Profile is one possible result, not a preselected feature. It
requires a named Application job, exact protected information and adversary,
measurable correlation advantage, finite latency/traffic/resource budgets, no
silent downgrade, and independent Qualification. Timing re-shaping, padding,
cover traffic, mixing, multiplexing, and multipath remain candidates until that
review. Horizon 5 does not retroactively strengthen claims made by Public Beta.

## Outside the network core

These are Applications, optional Overlay Services, or separate future products:

- messenger, Inbox, Contacts, social graph, presence, calls, and notifications;
- offline delivery, retained queues, history, content persistence, and site
  replication;
- universal User identity, Persona, proof of personhood, wallet, token, payment,
  staking, or incentive market;
- public Service search, directory, recommendations, or discovery feed;
- clearnet exit, VPN behavior, or general anonymous Internet access;
- multi-instance availability, replicated origin state, and automatic failover;
- bundled browser, arbitrary application runtime, decentralized compute, or
  content-safety engine;
- gateways to other networks and mobile/macOS clients.

An item on this list may return only with its own product job, threat model,
resource budget, and explicit scope decision. It cannot be added as “future
proofing” of the carrier.

## Scope-change rule

1. Only the current horizon enters the implementation backlog.
2. Later-horizon requirements constrain claims and architecture seams, but do
   not authorize placeholder subsystems or speculative abstractions now.
3. Security is never silently deferred: either the required condition is built
   and tested for a claim, or that claim is withheld.
4. A feature advances horizons only after evidence from the preceding horizon
   shows that it is still needed and the Product Owner explicitly promotes it.
5. Failure of the current route candidate triggers redesign or project stop,
   not compensating scope in naming, governance, SDKs, or UI.
6. Go is the maintained project foundation under ADR-0009. Runtime dependencies
   and protocol-bound foundations remain open until evidence justifies them.

## How to read the other documents

| Document | Meaning after this scope decision |
|---|---|
| [Vision](vision.md) | Long-term Product Core and public product intent. |
| [Horizon 4 delivery briefs](horizon-4/README.md) | Ordered working briefs for usable alpha and Public Beta epics. H4-1A and full functional-alpha H4-2 are selected; every broader or later claim retains its own explicit selection and evidence. |
| [Horizon 5 review intent](horizon-5/README.md) | Future security and privacy model review: purpose, entry conditions, questions, evidence, possible outcomes, and explicit non-goals. It authorizes no current work. |
| [Functional map](functional-map.md) | Requirements registry across all horizons, not one backlog. `fixed` means decision maturity, not “build now.” |
| [Operating model](operating-model.md) | Target lifecycle for a public product; only explicitly promoted parts apply to Carrier Lab. |
| [Threat model](../security/threat-model.md) | Conditions required before making each claim; an unclaimed condition need not become a current feature. |
| [Network Route and Node](../technical/network-route-node.md) | Current route, Node, State, and Entry contract, including the limits on Route claims. |
| [Go project foundation](../adr/0009-go-project-foundation.md) | Maintained language and runtime decision. |
| [Threat model](../security/threat-model.md) | Conditions and evidence required for each security or privacy claim. |
| [Operating model](operating-model.md) | Product lifecycle and future qualification boundaries. |
| [Development gates](../development/entry-gates.md) | The only promotion path between horizons. |
