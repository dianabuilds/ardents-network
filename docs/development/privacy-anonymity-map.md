# Privacy and anonymity implementation path

Status: **selected closed design and dependency order** under
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md).
GitHub Issues in the C0 Closed Alpha milestone owns execution and status.
This document is the reader route and coverage map, not a second task ledger.

## One selected path

First implement and qualify the complete closed text-Service journey by
Target Link on Ubuntu. It includes real enrollment/State/time inputs, private
admission and reachability, publication, both protected legs, end-to-end
Service authentication, confined reader and Publisher, resource exhaustion,
recovery, migration and controlled installation. No public autonomy,
canonical Name producer, general browser or Windows confinement is assumed.

Then design and qualify canonical naming with its real authority/close producer,
and public autonomous eligibility/admission under R-149. Neither can inherit
closed operator appointments as a permanent public authority. Stronger
traffic-correlation protection is a separately measured claim that may revise
the one common construction; it is not a parallel privacy mode.

## Authoritative reading route

| Question | Current owner |
|---|---|
| User outcome, Link-first scope, supported platform and visible failures | [Protected Service workload](../product/protected-service-workload.md) |
| Protected facts, adversaries, conditions, measurement and honest limits | [Closed successor claim contract](../security/threat-model.md#closed-successor-claim-contract) |
| Whole construction and receiving ownership | [Common architecture](../technical/common-privacy-architecture.md) |
| Exact profile, selection, lane/control bytes, introduction and Connection binding | [Protected forwarding protocol](../technical/protected-route-protocol.md) |
| Permissions, token/SPKI composition, bootstrap, debit/spend and compromise | [Private admission](../technical/private-admission.md) |
| OS boundary, installed units, local interfaces, workers and cleanup | [Application confinement](../technical/application-confinement.md) |
| Selected components and continued acceptance | [Dependency register](dependencies.md) |
| Parameters, complete test matrix and state/artifact migration | [Qualification](privacy-qualification.md) |
| Why these choices and what the experiments actually establish | [R-152 contract assessment](../research/records/r-152-closed-scheme-contract.md) |

Current Network, Endpoint, reachability, naming, Application and release owners
describe generation-2 implementation facts and compatibility obligations.
Their successor sections point to this contract. They do not select another
live Route or require copying old campaign implementations.

## Dependency disposition map

| Component or obligation | Selected disposition |
|---|---|
| Go toolchain and standard TLS/RSA/Ed25519 | Select supported-branch patch 1.26.8 for the successor; old 1.26.6 receipts retain their actual identity. Update build/CI/tool pins and run actual candidate checks before integration. |
| CIRCL v1.6.5 HPKE and ordinary blindrsa | Retain HPKE; select the exact new blind-token use in the admission owner after the R-152 review. No generic cryptographic framework or private fork. |
| quic-go v0.62.0 | Retain both accepted Carriers; qualify the same inner composition on each. |
| Current OHTTP/twoway/bhttp and transitive closure | Keep while a maintained generation-2 use exists. Remove only with the last migrated owner and verified dependency closure. A new TLS design alone does not delete them. |
| x/crypto, x/sys, x/term and other retained modules/tools | Reuse matching evidence and check changed inputs/current advisories. The unused OpenPGP advisory has scoped evidence; it does not exempt new imports or label all x/crypto abandoned. |
| Ubuntu/kernel/systemd | Select the stated supported family and required mechanisms; record exact security-updated package inventory and installed-artifact tests for each candidate. WSL is mechanism evidence only. |
| Source, State, time, enrollment and custody | Preserve independently pinned authorities and monotonic floors. Add authenticated closed-profile projection without accepting caller-made validity flags. |
| Public Name production and autonomous admission | Separate next-stage design, not a hidden dependency or pretend service inside the Link-first implementation. |
| Mixing networks, Sphinx, traffic shuffling, general sandboxes | Not selected dependencies. Retain only decision-relevant research evidence; no implementation tickets for speculative candidates. |

## Implementation order and coverage

The execution parent is [#50](https://github.com/dianabuilds/ardents-network/issues/50).
The following links identify the detailed work packages; their live state belongs
only in GitHub and the C0 Closed Alpha milestone.

| Work package | Issue | Depends on |
|---|---|---|
| Integrate the selected closed privacy design and retire conflicting guidance | [#51](https://github.com/dianabuilds/ardents-network/issues/51) | — |
| Admit the selected Go and cryptographic dependency baseline for the successor | [#52](https://github.com/dianabuilds/ardents-network/issues/52) | [#51](https://github.com/dianabuilds/ardents-network/issues/51) |
| Provision and verify the finite closed State profile and scoped issuance permissions | [#53](https://github.com/dianabuilds/ardents-network/issues/53) | [#52](https://github.com/dianabuilds/ardents-network/issues/52) |
| Issue blind tokens through bounded confidential bootstrap on both Carriers | [#54](https://github.com/dianabuilds/ardents-network/issues/54) | [#53](https://github.com/dianabuilds/ardents-network/issues/53) |
| Enforce receiver-local one-use admission and bounded protected lane forwarding | [#55](https://github.com/dianabuilds/ardents-network/issues/55) | [#54](https://github.com/dianabuilds/ardents-network/issues/54) |
| Install and bind confined Ubuntu text workers to Endpoint-owned local authority | [#56](https://github.com/dianabuilds/ardents-network/issues/56) | [#52](https://github.com/dianabuilds/ardents-network/issues/52) |
| Publish and refresh private reachability through protected Introduction channels | [#57](https://github.com/dianabuilds/ardents-network/issues/57) | [#55](https://github.com/dianabuilds/ardents-network/issues/55) |
| Complete the confined Target-Link text read over the common protected Route | [#58](https://github.com/dianabuilds/ardents-network/issues/58) | [#56](https://github.com/dianabuilds/ardents-network/issues/56), [#57](https://github.com/dianabuilds/ardents-network/issues/57) |
| Preserve logical authority and ordered streams across protected Attachment recovery | [#59](https://github.com/dianabuilds/ardents-network/issues/59) | [#58](https://github.com/dianabuilds/ardents-network/issues/58) |
| Enforce whole-owner resource budgets and retained concurrent-stream workloads | [#60](https://github.com/dianabuilds/ardents-network/issues/60) | [#59](https://github.com/dianabuilds/ardents-network/issues/59) |
| Adopt the complete protected candidate without losing floors or reopening retired paths | [#61](https://github.com/dianabuilds/ardents-network/issues/61) | [#60](https://github.com/dianabuilds/ardents-network/issues/60) |
| Qualify the complete closed protected text-Service journey and publish its evidence | [#62](https://github.com/dianabuilds/ardents-network/issues/62) | [#61](https://github.com/dianabuilds/ardents-network/issues/61) |

The issue bodies supply implementation mechanisms, acceptance cases and
dependencies. All entries refer to the already selected owner contracts.
A predecessor may be implementation work, never an unresolved consequential
design choice passed to an executor.

| Order | Observable result | Receiving owners | Main acceptance |
|---|---|---|---|
| 1 | Reviewable, integrated design bundle and current toolchain/dependency baseline | Documentation, build, dependencies | Owner links, ADR identities, exact inventory, fresh findings and required checks |
| 2 | Operator provisions and State accepts one real closed profile and finite permissions | State/Source, Credential, existing commands | Pin/authority separation, canonical bytes, conflict/expiry, durable allocation and restart |
| 3 | An Endpoint privately obtains and spends tokens through bounded forwarding on either Carrier | Entry/Route/Node, Credential, Resource | No token cycle; role/receiver binding; duplicate spend and reservation ambiguity; finite flood/refusal |
| 4 | Real installed reader/Publisher workers exchange a bounded local job with Endpoint | Application, Broker, Endpoint, install | Verified launch before Grant, descriptor audit, malicious child/sibling and joined revoke/cleanup |
| 5 | Owner publishes and User reads the intended text by Target Link over both legs | Instance/Publication, Reachability, Route, Service Connection, Endpoint | Recipient-only join, exact Target/content, first/warm access, withdrawal, no worker escape |
| 6 | Sustained and concurrent Service streams retain authority, fairness and bounded recovery | Route/Connection, Node, Resource, Endpoint | 64/256 open and 16/64 active, real byte replenishment, immutable context, fresh attachments, no request replay |
| 7 | One complete candidate migrates/adopts without resurrecting old state or accepting an older path | Release/install and every persistent owner | Interrupted conversion, rollback restrictions, tombstones/floors, owned worker inventory |
| 8 | The installed complete journey passes its declared system trials | Existing test/operations owners | P1–P11, both Carriers, real Ubuntu, hostile/cost/idle/overlap cases and complete evidence |

The retained network capacity workload is broader than the 4 MiB text job.
Its declared harness is a test input using real Service streams, never a
second supported unconfined Application. Every implementation slice includes
its own behavior and hostile-boundary tests; final trials do not postpone all
security checks until the end.

## Verification and test environment map

[Qualification P1–P11](privacy-qualification.md#acceptance-matrix) is the one
acceptance matrix. Test the real product commands and deep Modules. Fix exact
source/artifact/profile identities, role/family placement, platform,
impairments and prerequisites before collecting results. Real common operator
control is disclosed even when synthetic role facts exercise a positive path.

Use deterministic parsers/ledger tests for receiving boundaries, process tests
for actual command composition, and the exact Ubuntu installed-artifact profile
for confinement and the final journey. Exercise both accepted Carriers.
Cold/warm trials preserve durable quotas and authority floors; long workloads
retain NET-14AD and the complete NET-14 capacity/recovery requirements.
The easier no-loss latency cell cannot replace the retained normal cell.

Protocol-field disclosure, forged authority, unconfined admission, replay,
lost floors, unbounded work or a missed required budget fails qualification.
P9 records residual correlation diagnostically; it does not turn a successful
attack into a claim pass or a failed attack into an anonymity proof. An
unavailable prerequisite invalidates the affected run.

## Change and handoff rules

Only one C0 implementation issue may be in progress; this ordering does not
start all work at once. R-152 and R-099 have completed their bounded selections. R-149 remains
separate future design, not simultaneous implementation. No additional human team is assumed.

An executor may choose routine internal algorithms and file splits within the
contract. A new authority, weaker condition, changed privacy relation, wire
generation, platform or required budget returns to design before dependent
implementation. Update current owners with each implementing change and retire
only behavior whose compatibility obligation has actually been replaced.
