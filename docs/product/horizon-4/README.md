# Horizon 4 delivery briefs

Status: **historical evidence is split: RC1 has A1-A10, while RC2 has a
separate two-fresh-Endpoint control result and A11 campaign. No candidate has
A1-A12. The post-remediation headless candidate is not yet frozen or qualified.
H4-4 product delivery, H4-5C/D, H4-7, capacity/availability, independent
operation, and Public Beta retain separate gates.**

These briefs make the H4 epics navigable without turning later work into an
undifferentiated backlog. [Product scope](../scope.md) remains authoritative:
an epic enters implementation only after the Product Owner selects a bounded
claim and its evidence plan. A brief records current facts, dependencies, and
open decisions; it does not silently select a protocol, package format,
browser, platform-isolation mechanism, or operator economy.

`H4-*` is roadmap vocabulary only. It is not a runtime entity, package/API
boundary, wire identity, release identity, or product edition. Immutable
historical evidence names containing `H4` remain unchanged only for provenance;
maintained product code and contracts use domain language or receive an
explicit migration/retirement disposition.

| Epic | Delivery brief | First useful result |
|---|---|---|
| H4-1 | [Endpoint lifecycle](01-endpoint-lifecycle.md) | A person can run an authenticated distributable Endpoint rather than a source checkout. |
| H4-2 | [Live network and transport](02-live-network-transport.md) | Separate endpoints use a repeatable remote multi-host network profile. |
| H4-3 | [User, Service, and web access](03-user-service-web-access.md) | A User opens a remote Service through the headless local Application Interface; browser presentation is optional. |
| H4-4 | [Namespace and private resolution](04-namespace-private-resolution.md) | A public canonical Name is resolved privately without weakening the Target-Link path. |
| H4-5 | [Contributor viability and admission](05-contributor-viability-admission.md) | An explicit operator role can be evaluated without pretending installations are independent capacity. |
| H4-6 | [Transparent control and transition](06-transparent-control-transition.md) | Provisional control gives way to auditable release, view, and transition authority. |
| H4-7 | [Application boundary](07-application-boundary.md) | A supported protected local-application profile can earn its exact claim. |
| H4-8 | [Qualification and promotion](08-qualification-promotion.md), [H4-alpha-1 release profile](08b-alpha-1-release-profile.md), [H4-alpha-1 readiness matrix](08a-alpha-1-readiness-matrix.md) | Evidence determines whether a live network may make the corresponding beta claim. |

## Current delivery status

This table distinguishes a maintained functional slice from its later
participant, capacity, or release qualification. A passing local or
project-operated host is never silently promoted into a public-alpha claim.

| Area | Current verified state | Still open before its broader claim |
|---|---|---|
| H4-1 | Ubuntu Portable enrollment, foreground authenticated replacement, custody export/locked-restore/confirmed-purge, and the direct Ubuntu `.deb` package lifecycle are maintained. Portable and replacement have native non-lingering Ubuntu user-session qualification; the package has Linux `dpkg` process qualification. On 2026-08-28 the exact historical RC1 Ubuntu Portable artifact was signed, deterministically assembled, published as an immutable GitHub prerelease, independently pinned through the authenticated Product Owner channel, and exercised through the Product Owner's own first-enrollment/start/restart/stop walkthrough. That result does not transfer to RC2 or a post-remediation candidate. | An independent external participant remains a later validation gate. Native Desktop package qualification, Windows, package repositories/signing/automatic repair, and every Public Beta release claim remain outside this functional-alpha result. |
| H4-2 | TCP/TLS and QUIC v1 are maintained behind one ordered Carrier contract with common TLS 1.3 peer and reciprocal `LegBinding` authentication. Signed State selects the exact local/candidate profile; unknown profiles fail deterministically; neither Adapter can fall back to the other. The same full Publisher-to-User C-2 behavior passes over both profiles. The restricted local Docker campaign runs that product path, State projection/rejection, Carrier admission/binding/no-fallback, and hard-stop behavior from exact Linux bytes at 1 vCPU/1 GiB. H4-2C explicitly selects no camouflage/Bridge profile for alpha, while profile rollover drains and withdraws through the existing State successor lifecycle. | NET-01A pressure/capacity, physical-host/outage recovery, public-path diversity, and any later censorship-oriented Entry profile remain broader qualification or new research. The Docker emulator proves functionality, not capacity, censorship resistance, anonymity, availability, or independent operation. |
| H4-3A / H4-3B | The separate-process Publisher-to-User C-2 fixture carries one exact Target Link through private reachability and an optional loopback Reference Site. RC1 evidence covers application-transparent HTTP behavior, explicit withdrawal, classified Publisher/Application/Carrier/Node loss, and no fallback. | Product delivery remains open: the supported participant command has no normal State/Entry/one-use input acquisition plus `publish`/`open`/`withdraw` journey. RC2 A11 is separate campaign evidence and does not close RC1 or the post-refactor product. Browser presentation, capacity, public deployment, availability, independent operation, and protected-Application claims remain separate. |
| H4-4 | Signed corpus verification, ACA2-bound durable floors, bounded alpha OHTTP Relay/Gateway exchange, threshold-current Namespace behavior tests, and Firefox compatibility evidence are maintained. The completed planning simulators have been retired from the current product surface. The Firefox 154 resolver trace recorded native resolution for `.ard` before the HTTP-proxy route, so the XPI/native-host path is not a participant Browser Entry or no-DNS/DoH mechanism. | Product delivery is not closed. The non-Namespace alpha acquisition/distribution work remains active, Browser Entry requires a distinct resolution and HTTP/HTTPS trust design, and public Namespace control/operation remains absent. |
| H4-5 / H4-7 | H4-5A/B accepted the existing H4-2 Rendezvous duty for one project-qualified dedicated Ubuntu Functional Alpha profile. The bounded 260-cycle installed workload, fault/lifecycle cells across two existing VPS hosts and local Docker, final smoke, and exact cleanup passed after classified affected-cell repairs. H4-5C/D are deferred; H4-7 retains only its research boundary. | H4-5 is closed for that exact operator profile but makes no public capacity, availability, co-resident, permissionless-admission, Source-independence, or independent-operation claim; H4-7 remains unnecessary for the ordinary browser path. |
| H4-6A | The alpha-control reader and signed component-catalog verification are maintained. RC1 A5 verified fresh A, cached A, and fresh B inspection roots. RC2 separately verified two fresh Endpoint roots, but its profile explicitly had no cached repeat and therefore is not A5. The bundles pin `corpus.pub` only as an authority companion and claim no ACA2/signed-corpus acceptance. | Independent control/custody and later public-governance claims remain separate gates. |
| H4-8 | RC1 has historical A1-A10. RC2 has a separate two-fresh-Endpoint control result and accepted A11 6/6 campaign evidence. A12 retains closure/harness dispositions but supplies no executable-candidate qualification. | No historical candidate has aggregate A1-A12. The post-remediation candidate needs its own exact profile and evidence; Public Beta retains all independent external gates. |

## Shared alpha boundary

The usable-alpha target is intentionally narrower than Public Beta: a User
installs and starts an Endpoint, reaches a Service on another Endpoint through
a live multi-host network, and receives explicit readiness and failure states.
An alpha may use an explicitly bounded participant set and known operational
conditions. It does not establish permissionless independent operation,
browser-level location privacy, censorship resistance, or external security
review.

Target Links remain a complete destination path throughout the early alpha.
The first browser Adapter is the explicit one-connection loopback handoff
selected in H4-3, a compatibility surface over the local Application Interface;
it is not a proxy setting, bundled browser, public DNS, clearnet exit, or
automatic privacy claim.

## Working rule

The briefs are ordered for dependency visibility, not an instruction to run all
epics at once. Each selected slice must state its exact user outcome, protected
information and adversary, platform/profile, evidence, and stop condition.
External users, independent operators, auditors, builders, and reviewers are
future gates unless they have actually joined the work in a declared scope.
