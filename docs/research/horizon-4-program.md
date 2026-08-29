# Horizon 4 research program

Status: **pre-development program complete as of 2026-08-24.** This is the
decision/evidence handoff for Horizon 4. It is not a second product
specification or implementation backlog; implementation-linked and later
questions resume only at their recorded triggers under the
[research workflow](README.md).

**Post-handoff update (2026-08-27):** the Product Owner subsequently selected
and implemented full H4-2. ADR-0048 now maintains State-selected TCP/TLS and
QUIC-v1 Carriers, while ADR-0049 selects no blocked-entry/camouflage profile for
the functional alpha. References below to “TCP/TLS-only H4-2A”, “no maintained
QUIC”, or a future QUIC selection describe the 2026-08-24 pre-development
handoff and are retained only as dated provenance. The current contract is the
[H4-2 delivery brief](../product/horizon-4/02-live-network-transport.md), not
those historical rows.

## Rule of operation

One question is active when it can unlock the next bounded implementation
slice. Every question must state falsification criteria, sources, experiment
conditions where needed, and the exact document/ADR/code boundary it may
change. A later item may be framed early, but is not treated as active evidence
collection until its predecessor contracts are sufficient.

The existing H4 briefs own product intent. Accepted ADRs, product and threat
contracts, and completed research remain authoritative over this program.

## Ordered decision map

| Order | Horizon decision | Research question / current input | Unlocks | Cannot establish |
|---:|---|---|---|---|
| 1 | H4-1 Portable alpha lifecycle | [R-095](records/r-095-portable-endpoint-alpha-lifecycle.md), [R-101](records/r-101-endpoint-state-attachment-profile.md), and [R-102](records/r-102-endpoint-liveness-lock.md) | Selected H4-1A Ubuntu Portable, one-release Alpha Enrollment Pin, per-user state/liveness composition, and optional unsigned best-effort Windows artifact | A supported installer, auto-update, recovery, or public platform qualification |
| 2 | H4-2 live Node operating profile | [R-092](records/r-092-native-node-operating-profile.md) | Selected Rendezvous-first duty and one-VPS isolated-process tracer for H4-2A; real implementation then unlocks NET-01A measurement | Independent operator capacity, public operation, or numeric capacity before measurement |
| 3 | H4-2 carrier replacement and blocked-entry design | [R-094](records/r-094-hostile-network-carrier-profiles.md) | Selected TCP/TLS-only H4-2A and no maintained QUIC; a later QUIC/Entry choice requires a new question | Censorship resistance or a safe fallback by itself |
| 4 | H4-3 service/browser alpha adapter | [R-096](records/r-096-browser-adapter-alpha-contract.md) | Selected explicit connection-scoped loopback Adapter and static sandboxed Reference Site | Ordinary-browser privacy, arbitrary HTML, or Web PKI integration |
| 5 | H4-6 alpha control observability | [R-098](records/r-098-alpha-control-manifest.md) | Selected signed disclosure catalog with individually verified H4-6A components | independent public control |
| 6 | H4-4 named alpha and future Namespace | [R-097](records/r-097-named-alpha-private-resolution.md), after R-098 | named-alpha selection or deferral | permissionless root claims without shared Epoch close |
| 7 | H4-5 voluntary contribution | [R-093](records/r-093-voluntary-endpoint-contribution.md), after R-092 | a bounded opt-in experiment or an explicit rejection | independent public capacity or an incentive economy |
| 8 | H4-7 protected application mode | [R-099](records/r-099-protected-application-profile.md) | a disposable isolation experiment or a reason to defer | generic browser privacy |
| 9 | H4-8 qualification and closure | [R-100](records/r-100-alpha-qualification-closure.md) | H4-8A/B release matrix | Public Beta without its external gates |
| 10 | Public-control, permissionless admission, and Public Beta | Later only if the Product Owner explicitly opens a public-claim programme | H4-4C, H4-5D, H4-6D, and promotion review | H4-6C project-control simulation |

## Research cadence

For each active item:

1. update its record before examining candidates or running an experiment;
2. gather primary-source facts and clearly label assumptions/inferences;
3. define and run only the smallest reproducible experiment that can falsify a
   consequential option;
4. write the recommendation and strongest counterargument;
5. promote an accepted decision to its current owner and retire temporary
   research material when its unique fact is no longer needed; and
6. update this map, the Horizon brief, and the implementation queue with the
   new dependency state.

## Current handoff

The Product Owner selected H4-1A and TCP/TLS-only H4-2A on 2026-08-24, and the
H4-6A signed disclosure catalog with individually verified components on
2026-08-25. R-094, R-095, R-096, R-098, R-101, and R-102 are decided for their
pre-development questions and promoted to their product/technical owners.

On 2026-08-25, the maintained H4-2A Rendezvous candidate reached functional
qualification: State-authorized command composition and reciprocal LegBinding
passed locally, in an unprivileged Linux Docker cell, and in the same isolated
Docker cell on the project-operated VPS. This activates R-092's NET-01A
measurement trigger, but does not complete that campaign or select a capacity.

The Product Owner selected H4-3A on 2026-08-25 as the next maintained
user-facing objective: the exact two-Endpoint Target-Link-to-existing-browser
journey. R-096's loopback Adapter direction therefore becomes
implementation-linked; it still does not select a browser/version until the
live qualification stage. On 2026-08-28 the exact immutable H4-1–H4-3 profile
existed and activated R-100: H4-8A A1-A10 are green for the bounded
functional-alpha candidate, while A11/A12 remain open. R-093 and R-099 require
later Product Owner choices and create no speculative implementation work.

The retained experiments establish feasibility, counterexamples, and exact
oracles only. In particular, R-094's route-only dual-homed follow-up observed no
packet on the required alternate path and gives no QUIC migration verdict. The
accepted implementation authority comes from the H4 briefs and current
technical contracts, not this program or disposable code.

## Pre-development closure

The Product Owner accepted the first-cohort enrollment choice and the bounded
H4-1A/TCP-TLS-only H4-2A start on 2026-08-24. This does **not** mean every
Horizon 4 question has been
experimentally closed. It means the remaining questions either require the
maintained candidate they are intended to measure or belong to a later optional
product claim.

The following decisions and falsification results are sufficient to begin the
first maintained slice:

| Boundary | Pre-development result | Development consequence |
|---|---|---|
| Product claim | Usable alpha is a bounded participant journey, not Public Beta, anonymity, censorship resistance, public naming, or independent operation. | Implement only named H4-1A/H4-2A outcomes and preserve explicit non-claims. |
| Participant platform | Ubuntu LTS `x86-64` Portable is release-gating; Windows is an unsigned, best-effort, non-gating companion; GitHub Releases is only the distributor. | No installer, paid signing identity, service elevation, system proxy/DNS/VPN, or trust-store mutation is required. |
| Endpoint ownership | R-101/R-102 selected the per-user state/runtime layout, owner-only filesystem policy, held liveness lock, guarded stale-socket recovery, and no fallback. | Integrate those decisions into the purpose-owned Endpoint lifecycle and qualify them on native hosts later. |
| First network duty | R-092 selected bounded Rendezvous-first operation and proved its pairing/refusal/drain shape locally, under a race, and across separate hosts. | Implement the duty against real State/Node contracts, then measure its capacity on the declared reference host; do not invent capacity now. |
| First carrier | Native mutually authenticated TCP/TLS 1.3 remains the sole maintained carrier. R-094 proved the common result oracle with disposable TCP/QUIC adapters but selected no QUIC dependency or migration contract. | Build H4-2A with TCP/TLS. Do not create a speculative generic Go Interface around one maintained adapter. |
| Browser path | R-096 selected one explicit, fresh, connection-scoped loopback origin for one Target, with a static sandboxed Reference Site and no public CA/DNS/proxy/extension. | Integrate it only after the maintained Endpoint and Service Connection exist, then qualify a real supported browser. |
| Later epics | Target Links remain complete while R-097/H4-4 is deferred; R-093 contribution and R-099 protected mode require later Product Owner selections; R-098 keeps alpha control separated; R-100 owns final qualification shape. | None blocks H4-1A/H4-2A. They may not be silently implemented or claimed by the first slice. |

The selected first closed cohort uses R-095's one-release **Alpha Enrollment
Pin**: an already authenticated Product Owner contact
delivers the exact manifest SHA-256 plus cohort/release/platform independently
of GitHub. The selected contact class is an authenticated direct Product Owner
message. On 2026-08-28 the Product Owner enacted that handoff in their own
authenticated walkthrough; this is not independent external-participant
validation.

With that choice, this *pre-development program* is closed and preserves the
remaining work as implementation-linked or optional research triggers:

| Trigger | Question to reopen/run | Current disposition or remaining limit |
|---|---|---|
| Maintained Rendezvous and State/Node integration exists | R-092 NET-01A pressure, capacity, cancellation, and cleanup campaign | Numeric limits and supported-host behavior must measure the real implementation. |
| Maintained Endpoint/Release artifact exists and a participant is invited | R-095 native Ubuntu lifecycle and exact first-enrollment handoff | Executed on 2026-08-28 for immutable `h4-alpha-1-rc-1` through the Product Owner's own authenticated walkthrough; an independent external participant remains unclaimed. |
| Maintained two-Endpoint Target/Service path exists | R-096 supported external-browser and request/DNS/WebRTC observation | The current fixture has no real Target, Grant, Service Connection, or remote Publisher. |
| Concrete alpha component identities and reader artifact exist | R-098 real disclosure-catalog mapping and parser/resource review | The immutable RC published the concrete catalog plus Release, Network, and Compatibility identities; two fresh enrollment-pinned inspection roots and one cached repeat passed. These are standalone reader roots, not two Endpoint processes. `corpus.pub` is only a manifest-pinned authority companion, not a signed-corpus result. Independent parser/security review remains a later external gate. |
| One exact usable H4-1–H4-3 build/profile exists | R-100 qualification, live/soak, defect, and repository-closure matrix | Triggered: A1-A10 pass for the exact immutable profile. A11 soak/fault acceptance and A12 closure inventory remain. |
| Product Owner later selects QUIC as a maintained candidate | R-094 explicit or genuine access-path migration, supported-host UDP resources, and dependency review | The route-only lab observed no alternate-path packets; these facts are irrelevant to a TCP/TLS-only first profile. |
| Product evidence justifies names, co-resident contribution, or protected mode | R-097, R-093, or R-099 respectively | Each changes the product claim and needs its own newly selected job/profile rather than speculative code. |

Closing the pre-development program does not mark any of these later gates as
passed. Their record, trigger, falsifier, and claim boundary remain the handoff
into the corresponding implementation task.

## Evidence ledger

This is a deliberately narrow status view: a local passing fixture is evidence
about that fixture, never a substitute for the listed environment or release
gate.

| Record | Evidence now recorded | Next trigger or evidence |
|---|---|---|
| R-092 | Rendezvous-first selected; a disposable five-case local matrix, Linux race cell, and separate-host two-client run support bounded pairing, reservation, refusal, pump, and drain behavior | Maintained State/Node integration plus exact NET-01A evidence on the low-resource Ubuntu reference host |
| R-093 | Volunteer-operation context and a four-input no-offer gate | R-092 duty plus H4-5A/B evidence and an explicit co-resident alpha choice |
| R-094 | TCP/TLS-only first profile selected; refined QUIC local/separate-namespace oracle, selective refusal, nonzero loss/reorder, MTU 1280, synthetic same-IP NAT-port rebinding, real separate-host public path, cleanup, the Linux UDP-buffer warning, and a route-only dual-homed falsifier whose B-path counter remained zero | Only after a later Product Owner QUIC selection: a new question for genuine access-path or explicit QUIC-path evidence, complete supported-host resources, and dependency review |
| R-095 | Ubuntu Portable and Alpha Enrollment Pin selected; exact immutable publication plus Product Owner pre-execution verification, Release Decision, non-lingering first start, retained-state restart, and cleanup passed on native Ubuntu | An independent external participant and explicit Windows unsigned-artifact verification/failure journey |
| R-096 | Endpoint-to-loopback direction selected; fetch-only CSP failed external-navigation probing, then a header CSP sandbox retained rendering and blocked tested refresh/link navigation; exact routing, header stripping, proxy rejection, fresh-path, and post-stop refusal also passed | A supported external-browser, carrier-backed Target/Grant request-and-DNS-observation run; no browser privacy or arbitrary-HTML claim |
| R-097 | Maintained-verifier test and contract analysis: a static corpus cannot stand in for current Namespace | H4-3 name-value evidence and an explicit choice: full current-proof path or distinct alpha overlay |
| R-098 | Synthetic separation cases plus concrete immutable RC component identities and one cached/two fresh standalone inspection-root agreement | Independent parser/resource/security review; no independent-control claim |
| R-099 | Generic-browser boundary confirms no current protected job | Product Owner-selected job/platform, then a platform-specific isolation experiment |
| R-100 | Exact immutable H4-alpha-1 profile selected; H4-8A A1-A10 are green with retained failure dispositions and release evidence | A11 soak/fault contract and A12 closure inventory for broader H4-8 closure |
| R-101 | Selected shared Unix-socket and XDG/LocalAppData layout; matching startup, crash, substitution, and permission-policy fixtures | Purpose-owned Endpoint integration, actual cross-account denial, and complete platform lifecycle qualification |
| R-102 | Selected held-lock/guarded-recovery sequence; matching live-contention, crash, unexpected-entry, and substitution fixtures | Maintained integration, bounded delayed-release retry, and complete lifecycle qualification |

## Evidence campaigns after their maintained inputs exist

The following are not generic “next steps.” Each is a bounded campaign with a
named missing input. It is deliberately unsafe to substitute a stronger
development machine, a production service, an unsigned fixture, or an imagined
participant for its evidence.

| Campaign | Missing input | Run only when available | Evidence it may establish | It cannot establish |
|---|---|---|---|---|
| Native Node profile | Separate Ubuntu LTS `x86-64` NET-01A host: 2 vCPU, 2 GiB RAM, symmetric 100 Mbit/s | Integrate R-092's exact tracer oracle with the maintained State/Node duty, then run its recorded role-carriage and pressure/cleanup campaign while retaining raw observations outside Git | One measured preannouncement Node profile or a justified refusal to select one | Independent operator capacity, public operation, or an H4-5 contribution profile |
| Carrier fault profile | A later Product Owner selection of QUIC plus either a genuine changed access path or explicit QUIC path-management topology, and observed host UDP buffers | R-094's existing oracle under the selected path mechanism, cancellation, and resource census | Whether the selected QUIC Adapter retains the Carrier oracle across that changed path with bounded resources | Censorship resistance, a generic fallback, or a maintained QUIC selection by itself; the prior route-only run observed zero B-path packets and is not a migration verdict |
| Ubuntu first enrollment | Satisfied for the bounded Product Owner walkthrough: authenticated direct-message class plus immutable `h4-alpha-1-rc-1` | Executed on 2026-08-28: exact Pin/inventory/descriptor verification, Release Decision, start/restart/stop, and cleanup on the same artifact/root/metadata bytes | One bounded Product Owner obtain/verify/run handoff | Independent external-participant validation, Public Beta identity, independent custody, scalable onboarding, or protection from a malicious pinned artifact |
| Windows experimental provenance | A released explicitly unsigned Windows artifact | R-095's exact digest/attestation/status/lifecycle run on the intended Windows profile | What a best-effort Windows participant can verify and which execution policies reject it | SmartScreen reputation, supported Windows qualification, updater support, or a reason to delay Ubuntu alpha |
| Browser/service alpha | A selected H4-1/H4-2 profile and two declared Endpoints plus one actual supported browser/version and request observer | R-096's bounded Reference Site journey and failure cases | One generic browser handoff with observed request boundaries | Browser privacy, a protected application profile, DNS/CA integration, or ordinary browsing control |
| Alpha control and names | Product Owner-selected alpha component identities and reader release path, after R-095 provenance | R-098 reader artifact evaluation; only then reconsider the R-097 name decision | An inspectable project-controlled alpha input | Current public Namespace or independent public control |

No campaign is authorized by merely creating a host, certificate, or test
fixture: its corresponding record remains the authority for exact commands,
falsifiers, retention, and promotion boundary.

## Pre-development completion audit — 2026-08-24

The completion claim was checked against the current working tree rather than
inferred from the existence of these documents:

| Requirement | Authoritative evidence | Verdict |
|---|---|---|
| Horizon shape and implementation boundary | Eight H4 delivery briefs name their decision, bounded slices, evidence/exit conditions, stop/non-goal boundary, and later selections. H4-1A and TCP/TLS-only H4-2A are explicitly selected; later claims retain their own gates. | Complete for development entry. |
| Decision-ready research records | All eleven R-092–R-102 records contain the template's decision, current contract, hypotheses, criteria, evidence plan, findings, competing options, recommendation, and disposition. | Complete. |
| Reproducible experiment records | All nine retained H4 experiment READMEs state the linked question, hypothesis/falsifier, exact run path, evidence, actual result, limitation, and disposition. Every Go source remains build-ignored and outside the maintained package graph. | Complete as research evidence; not product qualification. |
| Final Product Owner choices | On 2026-08-24 the Product Owner accepted the one-release Alpha Enrollment Pin and the TCP/TLS-only first carrier profile. The former is promoted to H4-1, the operating model, and Release/Update ownership; the latter is promoted to H4-2. | Complete. |
| Maintained-repository integrity | `make check` exited zero on the current tree, including formatting/architecture, build, `go vet`, unit, e2e, race, `staticcheck`, module-tidiness, tool, and `govulncheck` gates. `govulncheck` reported no called vulnerability; one required-module advisory was not reachable from project code. | Complete for the present change. |
| Disposable-oracle regression | Current runs passed the R-092 native-leg baseline and 5/5 local Rendezvous cells; R-094 TCP/QUIC baseline and 15/15 local seam cells; R-095 exact-manifest pin and Ubuntu user-service lifecycle; R-098 bounded reader tests; R-101 Windows ordinary/crash cells; and R-102 Windows ownership/recovery/substitution cells. The R-095 sandbox attempt failed before the guest with `WSL E_ACCESSDENIED`; the exact wrapper passed once run with access to the installed WSL service, so the former is an invalid environment, not a hidden retry. | Complete for the named local regressions. Previously recorded namespace, rebinding, path-falsifier, browser, and separate-host evidence retains its stated scope. |
| Cleanup and residue | Every rerun reported or enforced exact temporary cleanup. Read-only Docker census found no local container and only default networks; the remote Ubuntu host had no container/network matching the R-092/R-094 experiment names or labels. `git diff --check` passed and no generated artifact appeared in the repository. | Complete. |
| Remaining Horizon research | The trigger table above preserves NET-01A, real enrollment, real browser, alpha-control, live/soak, optional QUIC, naming, contribution, and protected-mode work without claiming it passed. Each needs either maintained code or a later Product Owner product selection. | Correctly separated; none blocks the first maintained slice. |

**Audit conclusion:** the Horizon 4 pre-development research objective is
complete. Continuing to build synthetic substitutes for the implementation-
linked campaigns would not strengthen their decisions. Development can start
from H4-1A and TCP/TLS-only H4-2A; every remaining research campaign resumes at
its recorded trigger and remains mandatory for the claim it gates.
