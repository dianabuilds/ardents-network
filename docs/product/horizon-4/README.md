# Horizon 4 delivery briefs

Status: **the selected RC2 bounded functional-alpha profile is closed:
H4-1 through H4-3, H4-6A, and H4-8 have their accepted evidence. H4-4 remains
closed only as compatibility evidence. H4-5, H4-7, capacity/availability,
independent operation, and Public Beta retain separate gates.**

These briefs make the H4 epics navigable without turning later work into an
undifferentiated backlog. [Product scope](../scope.md) remains authoritative:
an epic enters implementation only after the Product Owner selects a bounded
claim and its evidence plan. A brief records current facts, dependencies, and
open decisions; it does not silently select a protocol, package format,
browser, platform-isolation mechanism, or operator economy.

| Epic | Delivery brief | First useful result |
|---|---|---|
| H4-1 | [Endpoint lifecycle](01-endpoint-lifecycle.md) | A person can run an authenticated distributable Endpoint rather than a source checkout. |
| H4-2 | [Live network and transport](02-live-network-transport.md) | Separate endpoints use a repeatable remote multi-host network profile. |
| H4-3 | [User, Service, and web access](03-user-service-web-access.md) | A User opens a remote Service, initially including a bounded browser-based site path. |
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
| H4-1 | Ubuntu Portable enrollment, foreground authenticated replacement, custody export/locked-restore/confirmed-purge, and the direct Ubuntu `.deb` package lifecycle are maintained. Portable and replacement have native non-lingering Ubuntu user-session qualification; the package has Linux `dpkg` process qualification. On 2026-08-28 the exact Ubuntu Portable RC was signed, deterministically assembled, published as an immutable GitHub prerelease, independently pinned through the authenticated Product Owner channel, and exercised through the Product Owner's own first-enrollment/start/restart/stop walkthrough. | An independent external participant remains a later validation gate. Native Desktop package qualification, Windows, package repositories/signing/automatic repair, and every Public Beta release claim remain outside this functional-alpha result. |
| H4-2 | TCP/TLS and QUIC v1 are maintained behind one ordered Carrier contract with common TLS 1.3 peer and reciprocal `LegBinding` authentication. Signed State selects the exact local/candidate profile; unknown profiles fail deterministically; neither Adapter can fall back to the other. The same full Publisher-to-User C-2 behavior passes over both profiles. The restricted local Docker campaign runs that product path, State projection/rejection, Carrier admission/binding/no-fallback, and hard-stop behavior from exact Linux bytes at 1 vCPU/1 GiB. H4-2C explicitly selects no camouflage/Bridge profile for alpha, while profile rollover drains and withdraws through the existing State successor lifecycle. | NET-01A pressure/capacity, physical-host/outage recovery, public-path diversity, and any later censorship-oriented Entry profile remain broader qualification or new research. The Docker emulator proves functionality, not capacity, censorship resistance, anonymity, availability, or independent operation. |
| H4-3A / H4-3B | The separate-process Publisher-to-User C-2 journey carries one exact Target Link through private reachability and the bounded loopback Reference Site. H4-3B preserves POST/body/header/cookie, redirect, cookie follow-up, and chunked response semantics; explicitly unpublishes the exact Target/generation after drain; classifies Publisher Application, Endpoint, Carrier, and product Node loss; and refuses fallback to the prior/another alpha name or ordinary Internet. RC2's accepted A11 campaign completed all six cells in 41:02 with retained Windows/Ubuntu resource evidence. | Closed for the selected bounded functional-alpha profile only. H4-4 Browser Entry remains optional compatibility evidence; capacity, public deployment, availability, independent operation, and H4-7 browser claims remain separate. |
| H4-4 | The Product Owner selected an explicitly non-Namespace `ardents-alpha://` overlay. Signed corpus verification, ACA2-bound durable floors, bounded alpha OHTTP Relay/Gateway exchange, a caller-non-substitutable alpha C-2 journey, and canonical V3 Grace semantics are maintained. The signed Firefox XPI/native-host route has demonstrated a visible `http://reference.ard/` dynamic C-2 compatibility flow with exact `.ard` scoping and ordinary tabs left on their own browser path. A fresh Firefox 154 resolver trace, however, recorded native resolution for `.ard` before the HTTP-proxy route. Therefore the current H4-4 slice closes with that route retained as functional compatibility evidence only, never as a participant Browser Entry or no-DNS/DoH mechanism. | A future named-browser product requires a separately selected system/browser resolution and HTTP/HTTPS trust design. Public Namespace still requires H4-6 Release/reclaim/close control and is not part of the closed alpha slice. |
| H4-5 / H4-7 | Direction and research boundaries are recorded. | H4-5 requires a Product Owner selection; H4-7 remains unnecessary for the ordinary browser path. |
| H4-6A | The alpha-control reader and signed component-catalog verification are maintained. The concrete catalog plus Release, Network, and Compatibility identities were verified with the same control inputs on two fresh Endpoint roots on 2026-08-28; cached repeat evidence is retained separately. The bundle pins `corpus.pub` only as the authority companion and claims no ACA2/signed-corpus acceptance. | Independent control/custody and later public-governance claims remain separate H4-6 gates. |
| H4-8 | Immutable RC2 has green A1-A12: publication, enrollment, H4-6A, browser observation, TCP/TLS no-fallback, and the accepted A11 6/6 soak/fault campaign. | Closed for the selected functional-alpha profile. Public Beta retains all independent external gates. |

## Shared alpha boundary

The usable-alpha result is intentionally narrower than Public Beta: a User
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
