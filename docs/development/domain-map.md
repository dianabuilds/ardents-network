# Domain ownership map

This is the maintained semantic map for the DDD transition. The Product Owner
assigned its upkeep to the orchestrator on 2026-10-04. It records responsibilities,
exclusions, collaborations and realized architectural results. It does not
select work, accept a protocol change or replace the GitHub execution ledger.

Accepted ADRs and current product/security/technical owners remain authoritative.
[CONTEXT.md](../../CONTEXT.md) owns product language;
[package-map.md](package-map.md) owns actual Go packages and permitted imports.
The [transition design](ddd-transition-design.md) records the strategic proposal
and its original evidence. This map keeps the ownership inventory current as
that proposal is implemented. A proposed future boundary remains a proposal.

## How to preserve the map

Read this map before delegating a domain slice and after every orchestration
compaction/handoff. Compare it with the actual source and current owners before
updating it. A source path, successful import or directory rename cannot prove
that a responsibility moved to the correct domain.

Keep every identified domain in the inventory, including domains not yet
implemented. Do not delete a responsibility when its old source is removed.
Give each maintained rule one semantic owner and name its actual consumer and
evidence. If ownership changes, update both the receiving owner and the former
owner's exclusions, their collaboration, package registry and affected oracles
in the owning change. A shared technical mechanism does not merge authorities,
databases, principals or lifecycle owners.

Newly found responsibilities must be classified: existing domain rule,
application composition, technical adapter, or a proposed new boundary. Do not
silently put them in a generic shared package or invent a domain to hold a test.
Escalate consequential unresolved ownership with exact evidence and a concrete
proposal, while continuing unaffected authorized work.

The entries below describe DDD contexts and supporting owners. Network's numeric
Role Domains are a different protocol concept. Temporary `successor` paths,
delivery labels and issue numbers are not product domain identities.

## Domains and boundaries

This inventory describes ten semantic families, not ten packages, processes,
databases or global aggregates. Each family may contain several consistency
owners at different principals. In particular, Software acceptance groups
Enrollment, Release verification and installation/replacement; it must not be
implemented as one authority or shared transaction. Local Application execution
is the execution boundary, not all Application functionality. Future boundaries
remain proposed until their current contracts and real consumers are verified.

| Domain / question | Owned rules and distinct consistency owners | Explicitly outside | Current new owner or future source map |
|---|---|---|---|
| **Network:** which authenticated facts may be used now? | Candidate eligibility/assignment, Epoch/profile history and conflicts, bound membership, trusted time/floors, coherent accepted observations and same-observation retained duty; local participation is a separate lifetime | Route purpose/path choice, tokens/spend, physical Hosting budget, process-wide resource monitor, private profile signing, Job/Service authority | [new Network](../technical/successor-network-state.md), `internal/successor/network`; State application and authentication/Source/durable/duty adapters have separate responsibilities |
| **Admission:** which finite rights can be obtained and spent? | Allocation; issuance quota; private material and retained results; holder permission/pending batch/stock; presentation history; receiving verification, allowance and durable spend are separate owners; opaque one-use fresh receiving-root creation fact belongs to storage lifecycle | Network authenticity, transport scheduling, Introduction slots and their independent history, provider budget, Local Grants, Custody keys and file delivery; a creation fact grants no token or transport authority | [new Admission boundary](../technical/successor-admission-boundary.md), [implemented owner map](../../internal/successor/admission/README.md) |
| **Hosting:** which physical allowance can this owner reserve? | One provider-period Tx/Rx accounting policy, coherent measurements, durable budget, work/termination reservations and their exact release | Token authority/debit, Node assignment, role envelope calculation, transport stop/join, CPU/RAM/cgroups or global process monitor | [new Hosting](../technical/successor-hosting-budget.md), `internal/successor/hosting` |
| **Route:** which protected leg and role channel may this operation use? | Entry/Interior selection and retention; role-purpose rules; prefix/lanes and credit; forwarding parents/children and permitted parent refill transport; receiving Introduction slots/delivery/floors; JOIN pairing and physical termination | State acceptance, Admission quotas/verification/spend, Hosting budget, publication readiness, Descriptor authority/history, Instance authentication and logical recovery | [Route owner map](route-domain-boundary-analysis.md), [migration contract](route-migration-contract.md), [implemented retained prefix](../technical/successor-route-prefix.md), [registration owner](../technical/successor-route-introduction.md) and [JOIN owner](../technical/successor-route-join.md); `internal/successor/route`, `selection`, `ardp`, `channel`, `role`, `issuer`, `prefix`, `join`, `receiver`, `transport/tls`, `transport/quic` and `introduction` with real `ardents-next` consumers. Lower `transport` owns no operation or adapter selection; former `carrier` and `operation` are removed. Genuine forwarding-parent refill and issuer Control use real Admission/Hosting owners. Descriptor Control composes separately owned Reachability Store/history. Opaque delivery remains a responsibility under the [remaining behavior design](route-delivery-join-boundary.md). These implemented boundaries are not the complete domain |
| **Local Application execution:** may this process invocation perform this operation? | Volatile Local Grant generation; local session with exact current/last Job and one-use handoff; separate supervisor cleanup capacity and terminal admission latch; qualified launch and joined original worker descendants. Job is a lifetime entity within the session consistency boundary, not an independent network Context | Token allowance, Service authority, Target/path choice, private routing history, remote publication, Connection recovery, software acceptance, global resource pressure, text semantics and qualification Run | [Source-backed boundary/model](application-execution-domain-boundary-analysis.md), current evidence `application/broker`, Endpoint Job/worker and scattered Service effect/handoff checks; [confinement owner](../technical/application-confinement.md). This is verified design inventory, not a new implementation or installed qualification |
| **Service Publication:** which authorized Instance is accepting work? | Publication generation/revision, current/pending/predecessor registration pair, private recipient lifetime, ACK/readiness, refresh and withdrawal; Instance authority/material remain purpose-scoped | Credential root signing, raw Route transport, Descriptor Store conflict floors, logical Connection recovery, Local Grant/Job mutation and worker cleanup; retain an exact live Execution operation instead | Proposed new boundary; current source evidence: Instance/publication, Endpoint registration/Introduction; [Service owner](../technical/endpoint-service-runtime.md), [private reachability](../technical/private-reachability.md), [Execution seam](application-execution-domain-boundary-analysis.md#source-inventory-collect-split-and-leave) |
| **Reachability:** what current proof is valid for this Target? | Receiving Descriptor Store and durable conflicts; local lookup verification/history is a separate owner at another principal | Publication readiness, admission spend, path choice, Service Instance authentication and logical Connection | `internal/successor/reachability` implements portable proof/conflict/history rules and native receiving Store mechanisms; real Route Prefix/Receiver and `ardents-next` consume them. [Reachability contract](../technical/private-reachability.md) records signed-input acceptance and its limits. Predecessor `service/reachability`, Endpoint history and Node resolution remain independent; their complete Service consumers are not transferred by this boundary |
| **Service Connection:** how does one authenticated logical stream retain its identity? | Immutable destination/provenance, exact Instance TLS authentication, ordered byte state, Attachment generation, continuity/recovery deadline and retained Service terminal outcome; checks the exact retained Execution operation at effects and final handoffs | New Target/Application operation, new Grant, Job mutation/worker cleanup, publication authority, path policy and independent physical reservations | Proposed new boundary; current source evidence: `service/connection`, Endpoint Service/binding/recovery; [Service owner](../technical/endpoint-service-runtime.md), [Execution seam](application-execution-domain-boundary-analysis.md#source-inventory-collect-split-and-leave) |
| **Software acceptance:** which exact generation may be installed or run? | Enrollment pin, Release authorization/monotonic floors, installation/replacement/recovery transaction are separate authority and state owners | Network membership, admission permissions, Service authority, generic signing and automatic rollback | New `internal/successor/enrollment` owns initial pinned byte verification under the [Enrollment contract](../technical/enrollment-verification.md); new `internal/successor/release` owns isolated offline authorization and its own history; Installation remains a separate future owner. This family is not one aggregate; [Release/Custody owner](../technical/release-update-custody.md) retains their current contracts |
| **Authority Custody** (supporting boundary) | Purpose-specific encrypted authority storage, approved signing commitments and recovery floors; authority kinds never become one signing right | Generic `Sign`, exported private authority, Network/Service/Admission decisions assigned to their own domains; runtime Instance/Node keys | Existing current Custody is source evidence; new integration remains explicit future work; [Custody contract](../technical/release-update-custody.md) |

## Supporting mechanisms and composition

| Owner | Scope | Boundary to preserve |
|---|---|---|
| New `nodeidentity` | Imported offline Node signing material and validated issuer-profile signing purpose, as its current `doc.go` states | It grants no duty and is not Person/Device/Persona, Network membership or a generic runtime Node identity service |
| Endpoint / Node composition | Construct genuine domain owners, perform live operation ordering and supervise their lifetimes | Composition must not accumulate domain policy or become the new version of the old monolithic owner; old composers cannot consume new Route |
| State/Hosting/Route physical adapters | Byte verification, durable transactions, counters, sockets and Carrier mechanisms under their declared owners | Mechanisms cannot mint authority, reset floors or copy a second domain decision implementation |
| Process resource/lifecycle owner | Process-wide observation, pressure availability, supervision and terminal process outcome | It is distinct from Network accepted-state policy and Hosting provider-period budget; final new implementation is not assumed present |
| Provisioning composition and signing owner | Subsequent purpose-specific approval and signing of prepared public Node/Epoch/profile material | New Network's unsigned grammar preparation grants neither a signature nor current authority; private profile signing and a new Control implementation are absent, not silently absorbed into Network |
| Integration/regression harness | Registered test composition across genuine new public/application contracts | It is not a product domain, production coordinator or new test-only command; domain-local tests remain with domain rules |
| Text Application | Fixed bounded text exchange, selected snapshot import, UTF-8/document rules, multiplexing/credit and safe presentation; current `internal/application/textdocument` | Application semantics remain outside Execution and Service Connection. The confined worker receives the selected snapshot and bounded bytes, never destination, publication or Endpoint authority. This product-specific owner is not another network authority |
| Local Application IPC | AAI3 Connection requests/ordered local streams and separately authorized Administration Publish/Withdraw grammar; current `internal/application/connection` and `administration` | Interface admission and local transport cleanup remain explicit; an IPC request, peer socket or READY message cannot attest a qualified Job. Administration grants no byte-stream or Service authority |
| Disclosure and diagnostics | Bounded public catalog verification, process/timeline measurements, inspection and command projections; current `alphacontrol`, `diagnostics` | Catalog verification never authorizes State, Release or runtime work; observations never decide admission or create a global cross-role trace. These are supporting mechanisms, not a generic Control authority |
| Qualification | Selected Run/workload composition, observers, measurements, completion barriers and retained reports; current `qualification` and `application/streamqualification` | Qualification is a proof producer with its own selected environment, not an ordinary Job, a permission issuer or a runtime domain substitute. A passing module test cannot qualify an installed product |

## Consistency, resources and completion inventory

The semantic owner below decides transitions. Storage, crypto and operating
system adapters enforce its declared mechanism; they cannot acquire the rule.
Separate roots are separate histories, not arbitrary new root names that may
erase installed floors. Every live operation retains its original identity,
bounds and terminal result; no late completion can act on a replacement.

| Family | Distinct state/authority owners | Resources and completion responsibility |
|---|---|---|
| Network | Authenticated State/history/conflict/time floors; retained runtime duty; separate local participation | State owns its root/lease and intake work; participation stops its own effects. Observations grant no indefinite right to callers and cannot replace their final currentness checks |
| Admission | Allocation; issuer quota/debit; private key/results; holder stock/pending/presentation; receiving verification/spend | Each principal owns its own private durable history. Issuer stores debit before response; holder stores presentation before sending; receiver reserves before spending. Work owns transferred physical reservation until join; no later failure refunds a token |
| Hosting | Provider-period counters, durable budget, current measurements and finite work/termination reservations | Hosting owns budget and actual once-only return. Physical borrowers keep the reservation until joined; close of another domain cannot invent capacity or reset usage |
| Route | Installation Entry selection/floors; local retained Interior/Rendezvous; role channel/parent/children; Introduction registry and slot history; JOIN pair and queues | Route owns Carrier/TLS/readers/writers/timers, wire accounting and physical stop/join. Its channel-local identifiers and private selection stay local; no full-path object or shared private history at a Node |
| Local Application execution | Volatile authority generation; execution session with current/last Job and one-use handoff; supervisor capacity and first terminal failure | Exact verified invocation/attachment/pinned original descendants. Revoke synchronously denies new effects, interrupts all dependents, joins the original worker tree, then returns cleanup capacity. Job is an entity within the session, not an independent network context |
| Service Publication | Purpose-scoped Instance generation/material; publication lifecycle with current/pending/predecessor pair, revision, recipient/replay and readiness | Publication stops new acquisitions before joining registrations and retiring/erasing private recipients. Descriptor Store ACK and REGISTER ACK remain different prerequisites; a late ACK cannot renew a retired generation |
| Reachability | Receiving Descriptor Store/conflict floors; local exact-Target lookup/history at another principal | Receiving Store owns its durable root and ACK; lookup owns its bounded attempt/history. Refusal cannot expose an older proof or substitute another destination; proof validity is neither readiness nor Instance authentication |
| Service Connection | Immutable destination/provenance, authenticated Instance, logical ordered bytes, Attachment generation, original recovery bound and terminal result | Connection stops/joins its current Attachments and retains the logical outcome. Route joins each physical stream; Execution joins its worker. Recovery changes an Attachment, never Target, Job, Grant or original recovery time |
| Software acceptance | Enrollment first-pin/inventory; Release authentication and monotonic floors; installation/replacement journal and generation ownership | Enrollment cannot sign or install; Release authorizes exact bytes but cannot download/activate. Installation authenticates fixed resources, joins its predecessor and completes its own transaction before runtime admission; ordinary Job cleanup does not own replacement |
| Authority Custody | Purpose-specific encrypted vault/recovery record, approved signing commitment and retained authority floors; separate authority kinds | Custody owns unlock, secret erasure and durable successor-before-response. Service Credential issuance retains its own purpose-specific approval/generation policy; Admission supplies allocation rules. No generic Sign or shared right across Service, Admission, State, Name and Release. Runtime Instance/Node keys stay outside the vault authority owner |

## Responsibilities outside the selected transition

**Naming is an explicit future separate model:** Service Name to authenticated
Target binding. The current maintained journey uses Target Links; Namespace and
Name signing routes are retired. Keeping this future boundary visible does not
restore their code, wire, signing rights or select implementation. Person,
Device, Persona, transport identity, Service Target, Credential and Capability
also remain distinct glossary concepts; this map does not create an Identity
domain or a universal authorization model by conflating them.

Public storage, consensus, blockchain, governance and long-term public transport
are unselected product/technology decisions. They are not silently inferred
domains or tasks for this DDD transition. Current accepted bounded transport and
software compatibility obligations remain with their named owners. A new
boundary needs a concrete domain question, rules, state/termination owner and
real consumer before package or implementation admission.

## Realization and design limits

The [source-tree scaffold](../../internal/successor/README.md#каркас-доменов)
provides README-only destinations for Execution, Connection,
Installation and Custody. These directories grant no Go import or
runtime authority. New Enrollment has its own portable first-pin verification
Module, bounded root-contained physical reads, native ownership/open adapters
and private frozen inventory consumed by the real read-only `ardents-next`
command. The genuine new Release consumer separately projects its authenticated file facts and frozen bytes into offline metadata input; Enrollment itself constructs no Release inputs or metadata URLs, establishes no
Release floor and grants no installed or Execution authority. New Release has a serialized verifier and independently owned native history with private immutable authorization; its real command does not install, activate or establish readiness. Component source and probes do not establish complete acceptance of that owner. Its ordinary
initial result and optional protected group remain scoped data; Release,
Installation and qualified Execution are separate necessary owners. These
source responsibilities do not establish complete Software acceptance or installed
qualification. New Publication currently has only portable public
Credential/Publication proof verification, with a real Reachability verifier
caller. New Reachability currently verifies exact-Target private Descriptors;
its separate receiving Store implements durable signed conflict floors and
bounded interrupted-write recovery through native Linux/Windows mechanisms.
Separate portable local lookup history retains only monotonic facts and the
original joined lookup lifetime. Route Prefix and Receiver compose genuine
Store/history consumers through fresh purpose-3 admitted terminals; the holder
commands carry externally supplied public signed input and independently selected
lookup Targets. Store commits precede ACK; history completion follows physical
join under the original opening. These boundaries have no old-runtime consumer.
These partial components establish no private Instance, registration,
accepting Publication readiness or complete domain acceptance. Software acceptance
remains three separate owners; scaffolding selects no implementation tasks.
New Network, Admission and Hosting have maintained owners and genuine consumers;
their precise source/evidence is recorded below. New Route has implemented
prefix, registration and paired JOIN owners, not the complete transport domain. Local
Application execution has a source-backed tactical design and class model;
the new implementation and installed qualification are absent. Publication,
Reachability, Service Connection, Software acceptance and Custody keep their
identified predecessor responsibilities and current contracts; a proposed new
boundary is not proof that its implementation has transferred. Text, IPC,
resource pressure, disclosure and qualification mechanisms remain in the
inventory even though they are not extra network authority domains.

This section states realized architecture and its limits, not live task status.
The GitHub ledger alone selects the next executable slice and records progress.

## Collaborations to review

The full Route transport inventory also includes confidential issuer bootstrap
and ordinary issuer Control (purpose 1, operation 1), and Descriptor Control
(purpose 3, lookup operation 2 and publication operation 6). Route owns exact
State-bound recipient selection, restricted child propagation, canonical
transport bytes and joined channel completion. Admission retains permission,
bootstrap batch limits, pending requests, quota/debit, signing and results;
Reachability retains Descriptor verification, private lookup history, durable
Store conflicts and actual Store ACK. Publication retains genuine Instance
material and readiness. Neither local `IssueCurrent` with supplied request bytes
nor a Route RESULT decoder proves those network operations. The
[remaining behavior design](route-delivery-join-boundary.md#issuer-and-descriptor-control-transport)
records their boundaries and acceptance obligations. Purpose 2 Name is reserved
and refused; it is not additional transition work.

Within Route, portable `issuer` owns one bounded OPERATION/RESULT exchange,
nonce/outcome agreement and role TLS termination. Prefix prepares the genuine
Stock attempt and owns its final completion; Receiver authenticates/adopts the
original channel, derives bootstrap or ordinary issuing kind and retains its
exclusive work and reservations until physical join. Admission remains the
permission, quota, signing and result owner. This protocol boundary has actual
Prefix and Receiver callers; it grants neither authority nor platform support.

The following are semantic relationships, not a general Go import allowance.
Actual package direction and third-party use still require exact registration.

Within Route, one installation Entry owner lends domain-bound selection access
to separately owned context/role Interior selections. Exact Source/Responder
opening lifetimes and physical prefixes remain separate; a lifecycle before
physical opening is not a ready Source acquisition. Closing one prefix returns
its reservation and borrows after join, not the installation's Entry root or a
Hosting budget used by siblings. Hosting retains the single provider-period
budget and actual return; installation composition closes shared owners only
after all borrowers finish. The [lifetime seam](route-delivery-join-boundary.md#installation-selection-and-exact-source-lifetime)
is implemented by the [JOIN owner](../technical/successor-route-join.md) at the
bounded holder/receiver transport boundary. It does not establish a qualified
Execution session, Publication authority or Service Connection.

The [common channel design](route-channel-runtime-design.md) is implemented by
portable `route/channel`, separate TCP/TLS and QUIC adapters, and the Prefix,
Introduction, JOIN and receiving operation owners. It is an internal Route
boundary, not another domain. Network retains authority facts,
Admission retains presentation/verification/spend, Hosting retains physical
budget and return, and selection/Introduction retain their independent durable
histories. A shared channel mechanism grants none of those neighboring powers.

| Collaboration | Evidence or right passed | Consumer check / prohibited expansion |
|---|---|---|
| Network to Admission/Route application | `CurrentRuntime` with coherent authenticated profile/member/time; `RetainDuty` from that same observation and `MatchDuty` against a fresh one | Reobserve at effect/commit points; no diagnostic Snapshot/receipt authority, successor rebinding, cached approval or callback into an already locked owner; exact role/Purpose and actual peer authentication remain consumer rules |
| Admission to Route | Purpose/receiver-bound finite accepted allowance and exactly transferred reservation lifetime | Admission owns Control 64 KiB/30s, Forward 32 MiB/1800s and Registration 1 MiB/600s. No token refund, new deadline, second claim or transport authorization based only on Node identity. Receiving Admission may request reservation rollback; Route's `Channel.HoldReservation` retains its physical return until joined retirement, including refusal without a Grant |
| Admission root creation to Route slot-history initialization | `receiving.Owner.TakeFreshRoot` passes an opaque one-use fact of genuinely durable fresh receiving-root creation for an exact public duty binding, before admission attempts; Route consumes it through its own history initialization | This is storage lifecycle evidence, not token/Network/Service authority. Retained reopen or pruned-empty spends cannot recreate a missing Route floor; attempts, Close, stale binding and late completion invalidate the fact. No shared lease/mutex or private spend parsing. [Registration design](route-introduction-registration-boundary.md) and [implemented owner](../technical/successor-route-introduction.md) retain the failure/adoption boundary |
| Hosting to actual work owner | Finite work and termination reservation | Work owner joins physical borrowers before release; no duplicated budget or hidden cleanup reserve. Hosting still owns the budget and actual return; Route delays the composed return callback until physical join rather than changing Hosting or Admission policy |
| Execution to operation composition / Publication / Connection / Route | Exact live Job/operation permit and local generation; joined result checked against exact last Job | Revoke is synchronous; neighbors check before effects and final handoff, stop/join their own resources and never mutate Job state. Late completion cannot enter replacement generation; no eventually delivered event substitute |
| Execution to permission preparation composition | Private retained provenance of a successfully joined verified preparation launch under its still-live local session | It is not a live worker permit or network authority; Network/Admission still verify current facts and permission. Do not retain a dead worker's rights through this proof |
| Software acceptance to Execution launch | Exact authorized installed artifact/binding; distinct runtime byte/invocation checks | Execution cannot mint Release authorization, reset floors, choose arbitrary executables or install privilege; worker compatibility is not product platform qualification |
| Publication through Route and Reachability | Authorized registration/revision and signed bounded Descriptor | Receiving slot transport, publication readiness and Store ACK remain different facts |
| Reachability to Connection | Verified exact-Target proof with its bounds and provenance | Descriptor verification is not Instance authentication, readiness or an accepting stream |
| Connection to Route | Already authorized fresh Attachment attempt within original logical bounds | Route cannot change destination, restart an Application operation or own the recovery decision |
| Domain decision to Custody | Exact purpose-specific approved signing commitment | Custody does not acquire the caller's policy, and the domain does not receive a private root |
| Software acceptance to installation/runtime | Fresh exact-artifact authorization with retained floors | Files alone do not grant authorization, and recovery cannot introduce rollback |

## Recorded architectural results and evidence limits

This section records observed architecture, not live task progress. Update a
row when an owning change establishes or changes a boundary. Link its selected
issue/revision and reproducible evidence; status and unfinished tasks stay in
GitHub. Never overwrite a failed receipt with a later pass or label reported
results as independently verified by the orchestrator.

Inspection provenance: local `dev` at
`b989b39da389ece7414a27aec334340e1c040c20` on 2026-10-04, with preserved dirty
implementation. HEAD is the base, not the identity of the new Network result.
The earlier dirty Network handoff and the orchestrator's isolated repeat have the same
417-file Go manifest, SHA-256
`186b6978a5e54d9bc92949e39b96574e197e0fd1f8a2a525481b95caaa2c8104`.
Its roots are `internal/successor`, `cmd/ardents-next`, `tests/epochfixture`
and `internal/architecture`. `go.mod` SHA-256 is
`f89ec343c5f9ecbe7e426140aa9af951fa5851581b0a8bc3ad6729ab4a773c1d`;
`go.sum` is
`86418eb72387d24b0a58e8867a67684b021fb44cea5d4cb9275efcf81889870d`.
All covered files and the tree membership matched after the repeat. This
identity does not cover the unfinished old-consumer delta or all documentation.
The source and receipts stay outside Git under the agent-execution workflow;
the selected issue owns publication and integration status.

The subsequent scoped Network result is commit
`4d56c37a7605fd999850a7ae47f3d1a4673edcda`, parent `b989b39da`, tree
`aa5260fb58a19881dccfa2da0ca8d20def727688`. Both local `dev` and `origin/dev`
identified that commit during inspection. Its separate 261-file selected-source
manifest has SHA-256
`37bacd3a68663ee236a08a6b58bbdea88b149bc5a67732d16b26d1bcc0dda86f`.
The orchestrator checked every listed file against the supplied candidate's
SHA-256 and the committed Git blob. The implementer's exact-tree Linux
21-package race/isolation/vet/build, seven-package Windows and Network-only
staticcheck receipts were inspected; they are not the earlier independently
reproduced dirty-tree run. The socket fixture is now self-contained test
composition. The old dirty receipt does not qualify this changed test source.
No whole-project gate or Carrier qualification follows from these receipts.

The predecessor inspection distinguished working-tree and committed architecture: the prepared
removal of `internal/successor/admittedwork` and `admission work` is not part of
the Network commit. That commit retains their previous product implementation.
Neither is an authorized regression domain or a Route consumer. The removal's
execution and publication belong to the GitHub ledger, not to an assertion that
Network performed that neighboring change.

| Recorded result | Owning change / inspectable evidence | What it does not establish |
|---|---|---|
| New Admission has one quota-to-result issuing path and explicit allocation, stock, presentation and receiving owners | Local commit `a40d2148e`; [boundary/evidence map](../technical/successor-admission-boundary.md), [README](../../internal/successor/admission/README.md) | Earlier component evidence does not qualify a later Network/Route integration or encrypted Custody |
| New Admission restores selected Registration 1 MiB/600s without changing other classes, token bytes, issuance, spend or predecessor runtime | [Owning repair](https://github.com/dianabuilds/ardents-network/issues/498), commit `fd8f01971e5ee2c3b35fed51e0172cc605f01719`; genuine signed-Network class3 receiving/Hosting and compiled class2/class3 commands; real caller expiry after durable spend, idempotent transferred release and no refund after reopen. Root independently matched all 2457 non-AGENTS working files and committed blobs to the complete 2458-file manifest SHA-256 `9f100983650b57122acca1929e2dd5fb0eeb27374bb2ea1e95e05fdcaa07e70b` (57 text line-ending normalizations), verified exact remote dev, inspected causal red/green, final Linux new-domain race/both-Carrier/command, Linux staticcheck, Windows full-gate and normal-hook receipts | Full test runs were inspected, not independently reproduced by root. This numeric repair implements no Introduction registration/delivery, JOIN, Publication readiness, installed qualification or larger per-slot workload |
| New Hosting owns independent durable provider budget and finite reservations | Local commits `98052a2f9` and `b989b39da`; [contract](../technical/successor-hosting-budget.md), [checked profile](testing.md#current-profiles) | No admission/network authority, invoice correctness, transport progress or installed-product qualification |
| Cross-domain regression belongs to test composition under `cmd/ardents-next`; the subsequent cleanup removes `internal/successor/admittedwork` and `admission work` | `hosting_transfer_fixture_linux_test.go`, `hosting_work_fixture_linux_test.go` and their behavior tests; [current profile](testing.md#current-profiles); exact-tree 21-package new-domain race/isolation/vet/build receipt recorded in the execution ledger | Test composition grants no product workload, Route consumer, Carrier qualification or whole-repository acceptance |
| New Network has pure rules, its own State root/application, cohesive adapters and real signed-intake/Admission command consumers; duplicate runtime projections, private signing and process monitoring are excluded | [Network owner](../technical/successor-network-state.md), [committed handoff evidence](https://github.com/dianabuilds/ardents-network/issues/493#issuecomment-5977818747), commit `4d56c37a`; `accepted_state.go`, `retained_duty.go`, State `accepted_observation.go` and command `network_authority.go`; earlier dirty regression independently repeated, committed source and later receipts inspected as distinguished above | No old Node/Endpoint/Route cutover, TCP/TLS or QUIC Route qualification, whole-repository gate or installed-product qualification is established. The scoped commit does not integrate the neighboring regression-composition removal |
| Genuine new Network, Admission and Hosting preserve authority and physical-resource ownership across durable and joined boundaries | `cmd/ardents-next/network_admission_hosting_linux_test.go`: refill original deadline and dual reservations, pre-spend loss, successor after burn/reopen, conflict/clock refusal and joined socket work; signed command fixtures use separate installation-local roots; independently repeated with the new-domain race suite | The local socket harness is not a selected Carrier implementation. Earlier standalone supplied-fact Admission tests do not prove Network authenticity; external Admission staticcheck findings remain with their owners |
| Route isolation has a mandatory disk-read contract, root instruction and independent import guard | [migration contract](route-migration-contract.md), [AGENTS.md](../../AGENTS.md), [architecture guard](../../internal/architecture/route_migration_isolation_test.go); both focused Route guards independently passed alongside new-domain isolation | Policy preparation is not a Route implementation. Focused guard passes do not establish full repository regression |
| New Route owns retained Entry/Interior prefixes, authenticated role channels, bounded serialized OPEN and joined physical reservation retirement; receiving/pool terminal results retain late physical failures | [Prefix boundary within the full Route assignment](https://github.com/dianabuilds/ardents-network/issues/496), final source `69406f7c896aa45321b5556f2b63d0c99f2dba44`; initial prefix `13f3fb0e4`, OPEN correction `d08dbc18b`, receiving terminal correction `944975f9e`, strict expiry oracle correction `69406f7c8`. [Prefix owner](../technical/successor-route-prefix.md); real holder `prefix-open`/`prefix-close` and `route receive`; genuine both-Carrier/compiled-process/refusal/reopen tests, transport scheduling/OPEN/terminal/pool tests and selection floors. The orchestrator matched all 495 non-AGENTS working files and committed blobs to the final 496-file manifest SHA-256 `5f66ba9e93e9b30aafc1c4f9f5b2d7e412c8ed57d4fe80f7e4e417df1836b4d3` (one registry CRLF normalization), inspected executor red/green, full Linux new-domain race, Windows full-gate and normal-hook receipts, and reviewed authority/spend/join and both terminal/oracle corrections. Root independently ran Linux-targeted Route/command staticcheck successfully; the full transport/regression tests were inspected, not independently rerun | This bounded source establishes no complete Route, Introduction, JOIN, Publication, Connection, installed-product or privacy qualification. Late writer-failure controls are mechanical; genuine Carrier/process scenarios establish actual consumer reachability and regression, not an injected late-writer trial through the entire admitted path. Fixed defects and passing controls do not establish absence of all defects or independent security validation |
| New Route owns genuine Introduction registration/withdrawal and independent durable slot history; Admission supplies only an opaque fresh-root lifecycle fact | [Registration slice](https://github.com/dianabuilds/ardents-network/issues/499), final implementation `ca0767ebaa02513166d6bcb14cd55836a45ae661` (registration `80f31aa6e`, Network test-only completion oracle `c3f568809`, original-caller effect/handoff correction `ca0767eba`); [implemented owner](../technical/successor-route-introduction.md). Actual holder registration/withdrawal and receiving commands exercise signed Network, genuine class-3 stock/presentation/spend, separate Hosting/spend/slot roots, both Carriers and joined cleanup. Independent byte/capacity oracles, retained and lost-root reopen, uncertain durability, withdrawal races and deliberately delayed original-caller cancellation check refusal before ADMIT and after real spend without refund. Root independently matched all 2480 non-AGENTS working files, committed blobs and complete Git tree membership to manifest SHA-256 `9b01b9a8818f2455575db40a34199fe5a3ba8e674eed67cdeea5bb4528f0d85d`, allowing only 59 exact CRLF-to-LF normalizations, and verified actual remote `dev`. Final Linux full new-domain race/Carrier/compiled-consumer, Windows full-gate and ordinary commit-hook receipts were inspected. Root independently ran Linux-targeted staticcheck across every new Network, Admission, Hosting, Route and command package with exit 0, and the complete Windows repository make check during documentation integration against unchanged implementation | Native Linux transport/regression suites were inspected, not independently rerun by root; Windows repository regression and Linux-targeted static analysis were reproduced as stated. Earlier Network test-oracle failures and a Hosting lease-wait failure remain recorded; later passing receipts do not erase them. Registration establishes no opaque recipient ACK, Publication readiness, JOIN, forwarding-parent refill transport, complete Route or installed/privacy qualification. The fresh-root fact conveys no token or transport authority and cannot restore lost slot history after retained receiving use |
| New Route owns installation Entry borrowing, exact Source/Responder acquisitions, retained Rendezvous choice, paired JOIN and joined framed transport | [JOIN slice](https://github.com/dianabuilds/ardents-network/issues/500), commit `df94a181810548f977cd5aa03a24cd96447c3042`; [implemented owner](../technical/successor-route-join.md). Genuine signed Network, separate class-2 stock/presentation/spend and Hosting owners feed real holder/receiver and compiled two-sided consumers on TCP/TLS and QUIC. Checks cover matching RESULT-before-data, original bounds, forbidden JOIN refill, retained selection, cancellation/reopen, late cleanup, nested physical-error provenance and parent retirement during writer join. Root independently verified all 2517 committed files outside four preserved user/root documents against candidate manifest SHA-256 `C66C9D2055F503F0F51D2EDD558CC46C3C9D59D0E72F702065EDDC8D2AA915B9`, allowing 58 exact CRLF-to-LF normalizations, all tracked paths present, the exact 68-file commit and remote `dev`. The 2573-file snapshot also contains eight uncommitted README scaffolds and 44 non-source local artifacts, explicitly excluded from committed-source evidence. Final Linux new-domain race/isolation, Windows quick/full unit/e2e/race, normal-hook and genuine expiry-repeat receipts were inspected | Runtime suites were inspected, not independently rerun by root. Earlier expiry/cleanup failures and Admission/Hosting fixture failures remain evidence; repairs and later passes do not erase them. This result establishes no complete Route, opaque recipient ACK, forwarding refill, issuer/Descriptor Control, Publication readiness, Instance authentication or installed/privacy qualification. The fixture-bound encrypted payload is transport evidence only |
| New Route has one portable channel, separate actual TCP/TLS and QUIC adapters, distinct Prefix/Introduction/JOIN/Receiver lifetimes and genuine parent replenishment | [Corrective slice](https://github.com/dianabuilds/ardents-network/issues/501), implementation `e61f3bfc87a96b3cc05c0f49515c38295f09a60b`, exact remote `dev` verified by the executor. The executor matched all 236 changed paths and committed blobs to the tested candidate, allowing only Git CRLF normalization. Its final Linux new-domain race/real-command/architecture profile, Windows quick/full gate, both-platform static/deadcode and ordinary pre-commit passed. Compiled refill is followed by real Registration child traffic on the same prefix on both Carriers. Skill-required read-only standards/spec reviews inspected the committed source; they did not rerun runtime tests. Pure Entry rules retain original selection/body/bounds while durable roots stay with their native owner | Acceptance is performed by the executor under the Product Owner's 2026-10-06 direction to work without the separate orchestrator; it is not independent validation. Earlier failures, including the first Windows hook's pre-TLS socket bind refusal, remain retained. The final owning source matched; a concurrent hash change in excluded `network-boundary-reassessment.md` prevents claiming the entire mixed working tree was unchanged. Windows portable mechanisms do not qualify native durable roots or installed support. No complete Route, opaque recipient ACK, issuer/Descriptor exchange, Service or privacy qualification follows |
| New Route has genuine confidential issuer bootstrap and ordinary admitted issuance, with one portable issuer exchange and immutable restricted children | [Issuer slice](https://github.com/dianabuilds/ardents-network/issues/502), implementation `e3add846b1c43be0d136687fb836ee651aba7861`, exact remote `dev` verified by the executor. Route `bootstrap` owns finite shared duty/adjacency capacity; `issuer` owns canonical OPERATION/RESULT and role termination; Prefix and Receiver retain original lifetimes and physical join. Admission retains permission, quota/debit, signing, pending blinding and verified Stock deposit; Hosting retains actual reservation and return. Genuine holder bootstrap obtains two batches, refuses another, joins and opens fresh admitted channels before ordinary issuance on TCP/TLS and QUIC. Actual State/caller loss, exact retained retry/reopen, blocked inner TLS output, sibling progress and late return controls exercise genuine new owners. The executor reproduced the final 34-package Linux new-domain race suite, compiled both-Carrier REGISTER 40 times, Windows channel race 20 times, Linux staticcheck, architecture/profile checks, Windows full `make check` and ordinary commit hooks. It matched all 640 selected files and committed blobs to candidate manifest SHA256 `00F64F080022CEE17A8BA64668C01046FB2AD40FB5594D07D17ACCA426DAF1B2` and exact 74-path commit membership. Compared with the runtime-tested manifest, only six exported comments changed; reversing those comments reproduces the exact tested file hash | This is executor self-review under the Product Owner's direction, not independent validation. Earlier failures and incorrect test oracles remain retained, including the rare compiled TCP/TLS close failure that led to a causal CREDIT/read correction. Mechanical failure controls and blocked inner TLS output do not prove an injected physical failure across the complete outer Carrier path. Windows portable behavior does not qualify native durable roots or installed support. Descriptor Control, successful opaque recipient delivery, Publication/Execution/Connection and complete Route remain separate obligations; no installed or privacy qualification follows |

| New Route carries genuine Descriptor Control through separately owned Publication proof verification and Reachability Store/history | [Descriptor slice](https://github.com/dianabuilds/ardents-network/issues/503), implementation `ae21570018ffd150e21917b63e10cff337a343a1`, exact remote `dev` verified by the executor. Purpose-3 Prefix/Receiver use genuine signed Network, class-1 stock/presentation/spend and Hosting on TCP/TLS and QUIC; compiled consumers restart the actual receiving Store. Reachability owns durable signed conflict floors, bounded own-stage recovery and separate context-private monotonic history without cached proof bytes. Publication verifies public delegation; Publish requires both Publish and Connect. Store commit precedes ACK, physical join precedes holder history completion, and original caller/duty/expiry checks precede final handoff. The executor matched all 2639 committed files outside three preserved user documents and exact 51-path commit membership to the final 2642-file manifest SHA256 `F881DC9FDD9E37AE83B31C391C5F0AA09EB7C1E957C99DB6A8FAD4F489813670`, with Git text normalization. Full Linux 36-package new-domain race, genuine/compiled commands and architecture profile, Windows full `make check`, Linux staticcheck and normal commit hook passed. The Linux runtime-tested snapshot differs only in three subsequent documentation corrections, with unchanged Go source. Maintained causal queued/selected CLOSE tests reproduce the old refusal-to-physical-error defect; exact lane/kind pre-output provenance fixes it while retaining a started physical error. Native Linux/Windows subprocess probes cover four interrupted-write phases with 128 signed floors. Current-source external scheduling probes cover blocked RESULT/sibling progress, original caller cancellation and signed expiry at final handoff | This is executor self-review, not independent validation. Earlier failures and a 300-second command-suite timeout remain evidence; later 272/277-second complete runs do not prove the earlier timing cause. External output pauses exercise inner TLS scheduling, not injected outer-device failure. Process-crash recovery does not qualify power loss or storage hardware. Vulnerability gates found no called vulnerable symbols; package/module findings retain scoped non-applicability evidence, not a clean dependency claim. Signed-input Store ACK establishes no registered slot, live Publisher/Instance, qualified Execution or authenticated Connection. Genuine opaque recipient delivery, complete Route and installed/privacy qualification remain separate obligations |

| New Enrollment authenticates an initial pinned portable bundle and retains private immutable byte provenance, with a genuine new consumer | [Enrollment slice](https://github.com/dianabuilds/ardents-network/issues/504), implementation `d1b262d9dd7ca9026864fb7cc28d0a1ac5bcefff`, exact remote `dev` verified by the executor. Pin comparison precedes parsing; canonical v3, headless pairs and the exact protected resource group retain their accepted identities. Bounded root-contained reads check actual growth, regular-file identity and native ownership; original cancellation and physical close errors prevent a partial result. Independent fixtures and actual compiled Windows/Linux consumers exercise success and refusal. Pin-guard removal and retired-descriptor ordering controls retain causal failures. The executor matched the 2654 committed members outside three preserved user documents, exact 28-path change including the helper rename, and 27 present changed blobs to candidate manifest SHA256 `C988F42C673E7F1B1D33041567C3BB79B6BB4F1C19B17D693E2A5523BBD051EF`, with repository text normalization. Final Windows quick/full gates, Linux 37-package new-domain race and architecture gates, Linux-targeted staticcheck and the ordinary commit hook passed. A separate receiving-prefix fixture repair performs pre-admission clock-loss refusal before listener construction and requires the genuine Network error; both-Carrier modes passed twice, while an external amplified control reproduced the previous watcher race | This is executor self-review, not independent validation. Earlier timeouts, fixture failures and invalid-environment receipts remain retained; later passes do not prove the aggregate timing cause. The result verifies initial bytes only. It constructs no Release inputs or authorization, floors, installed resources, qualified Execution, Publisher readiness or Service Connection. Windows portable file checks do not attest installed ACLs; other Unix adapters have compilation evidence only. Self-verification cannot authenticate first execution. Complete Software acceptance, Route and installed/privacy qualification remain separate obligations |

| New Release owns offline exact-byte authorization, serialized evaluation and separate durable trust history, with a genuine Enrollment-backed consumer | [Release slice](https://github.com/dianabuilds/ardents-network/issues/505), implementation `cf2ec73c6c995d9fd83da7135cc5ba3189ed8964` and Root public-material correction `43da7f14b3fd8dc14495e29a9ed4b65684f24525`; exact remote `dev` verified by the executor. Enrollment supplies private frozen bytes and authenticated file facts; Release owns bounded signed metadata, consecutive Root rotation, target policy, monotonic floors and immutable authorization. Five canonical key IDs must also represent five distinct normalized public keys in each Root role. Native history retains its exclusive lease until admitted work joins, refuses lost pointers and foreign residue, and confirms exact current durability on reopen. Real compiled Windows/Linux consumers exercise acceptance, refusal and retained history. Maintained signature/rotation/rollback/emergency/dual-target/alias/cancellation tests and native process-crash tests cover the declared boundaries; mechanical guard-removal controls retain causal failures. The executor matched all 2685 committed members outside three preserved user documents, the exact 41-path implementation delta and three-path correction against snapshot SHA256 `D75C47FD254C0C2767479E7B6951CA958C0B58ED4AE4514415AD63FAA10073C9`, allowing only 64 exact Git text normalizations. Final Linux 38-package new-domain race, both-Carrier command and architecture profile, Windows full `make check`, affected Linux-targeted staticcheck and ordinary commit hooks passed | This is executor self-review, not independent validation. Earlier deadcode, fixture and concurrent-run timing failures remain retained; the unchanged five-minute Linux command profile subsequently passed in isolation in 277.753 seconds, which does not prove the absence of timing instability. Process-crash recovery does not qualify power loss or storage hardware. Windows native file/history checks do not attest installed ACLs; other Unix adapters have compilation evidence only. Public signing fixtures prove threshold behavior, not independent key custody or builders. Release neither installs nor executes, and first-pin self-verification cannot authenticate first execution. Installation, qualified Execution, genuine opaque recipient delivery, Publication/Connection, complete Route and installed/privacy qualification remain separate obligations |

## Updating after a completed slice

The orchestrator checks and updates this map with the owning result:

1. Reconcile every affected responsibility against final implementation, real
   caller and accepted contract. Record excluded/retired old ownership rather
   than dropping the responsibility from the inventory.
2. Update inbound/outbound collaboration and state/resource/termination owner.
   Verify both the receiving domain and the neighbor losing that responsibility.
3. Link the realized architectural result to its precise source and evidence,
   distinguishing inspected, reported and independently reproduced results.
4. Record unresolved consequential boundaries as proposals with named owner;
   do not convert a recommendation into an accepted product rule.
5. Update current technical documentation and factual imports with the owning
   implementation. Delivery status and acceptance remain in the selected issue.
6. Preserve all future domains and the map/contract paths in the orchestrator's
   external handoff. After compaction reread the actual map, then verify source.

Do not start new work merely because this inventory contains a future owner.
Under the delegated project transition, the orchestrator designs and verifies
the next dependency-ready domain, records its selection and WIP admission in
GitHub, and assigns a real acceptance boundary under the
[execution workflow](agent-execution.md#delegated-project-wide-domain-transition).
Completion of one slice does not complete a domain; completion of one domain
continues the program until every maintained scenario has transferred.
