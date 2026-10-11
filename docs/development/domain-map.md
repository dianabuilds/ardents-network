# Domain ownership map

This is the maintained semantic map for the DDD transition. The Product Owner
assigned its upkeep to the orchestrator on 2026-10-04. It records responsibilities,
exclusions, collaborations and realized architectural results. It does not
select work, accept a protocol change or replace the GitHub execution ledger.

Accepted ADRs and current product/security/technical owners remain authoritative.
[CONTEXT.md](../../CONTEXT.md) owns product language;
[package-map.md](package-map.md) owns actual Go packages and permitted imports.
Earlier strategic proposals and source receipts are in the [archive](../archive/README.md).
This map records current ownership; proposed future boundaries remain proposals.

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
| **Route:** which protected leg and role channel may this operation use? | Entry/Interior selection and retention; role-purpose rules; prefix/lanes and credit; forwarding parents/children and permitted parent refill transport; receiving Introduction slots/delivery/floors; JOIN pairing and physical termination | State acceptance, Admission quotas/verification/spend, Hosting budget, publication readiness, Descriptor authority/history, Instance authentication and logical recovery | [Route protocol](../technical/protected-route-protocol.md), [migration contract](route-migration-contract.md), [implemented retained prefix](../technical/successor-route-prefix.md), [registration owner](../technical/successor-route-introduction.md) and [JOIN owner](../technical/successor-route-join.md); `internal/successor/route`, `selection`, `ardp`, `channel`, `role`, `issuer`, `prefix`, `join`, `receiver`, `transport/tls`, `transport/quic` and `introduction` with real `ardents-next` consumers. Lower `transport` owns no operation or adapter selection; former `carrier` and `operation` are removed. Genuine forwarding-parent refill and issuer Control use real Admission/Hosting owners. Descriptor Control composes separately owned Reachability Store/history. Opaque delivery remains a responsibility under the [remaining behavior design](route-delivery-join-boundary.md). These implemented boundaries are not the complete domain |
| **Local Application execution:** may this process invocation perform this operation? | Volatile Local Grant generation; local session with exact current/last Job and one-use handoff; separate supervisor cleanup capacity and terminal admission latch; qualified launch and joined original worker descendants. Job is a lifetime entity within the session consistency boundary, not an independent network Context | Token allowance, Service authority, Target/path choice, private routing history, remote publication, Connection recovery, software acceptance, global resource pressure, text semantics and qualification Run | [Execution contract](../technical/application-execution.md), current evidence `application/broker`, Endpoint Job/worker and scattered Service effect/handoff checks; [confinement owner](../technical/application-confinement.md). New `internal/successor/execution` implements local authority/session/Job and independent cleanup supervision; `execution/worker` implements fixed native mechanisms; `execution/runtime` has genuine permission-preparation and bounded live Route-holder consumers. One exact operation retains qualified original cleanup through Route borrower join; complete Service Connection and full installed qualification remain absent; bounded installed Source/permission bootstrap has a genuine independently admitted consumer |
| **Service Publication:** which authorized Instance is accepting work? | Publication generation/revision, current/pending/predecessor registration pair, private recipient lifetime, ACK/readiness, refresh and withdrawal; Instance authority/material remain purpose-scoped | Credential root signing, raw Route transport, Descriptor Store conflict floors, logical Connection recovery, Local Grant/Job mutation and worker cleanup; retain an exact live Execution operation instead | `internal/successor/publication` public proof rules, `publication/durable` exclusive floors, `publication/instance` purpose-scoped private lifetime and `publication/runtime` live qualified Publisher have real `ardents-next` consumers; [Service owner](../technical/endpoint-service-runtime.md), [private reachability](../technical/private-reachability.md), [Execution seam](../technical/application-execution.md#required-lifecycle-and-linearization) |
| **Reachability:** what current proof is valid for this Target? | Receiving Descriptor Store and durable conflicts; local lookup verification/history is a separate owner at another principal | Publication readiness, admission spend, path choice, Service Instance authentication and logical Connection | `internal/successor/reachability` implements portable proof/conflict/history rules and native receiving Store mechanisms; real Route Prefix/Receiver and `ardents-next` consume them. [Reachability contract](../technical/private-reachability.md) records signed-input acceptance and its limits. Predecessor `service/reachability`, Endpoint history and Node resolution remain independent; their complete Service consumers are not transferred by this boundary |
| **Service Connection:** how does one authenticated logical stream retain its identity? | Immutable destination/provenance, exact Instance TLS authentication, ordered byte state, Attachment generation, continuity/recovery deadline and retained Service terminal outcome; checks the exact retained Execution operation at effects and final handoffs | New Target/Application operation, new Grant, Job mutation/worker cleanup, publication authority, path policy and independent physical reservations | New `internal/successor/connection` retains initial binding; remaining logical owners are defined by the Service contract. Predecessor evidence: `service/connection`, Endpoint Service/binding/recovery; [Service owner](../technical/endpoint-service-runtime.md), [Execution seam](../technical/application-execution.md#required-lifecycle-and-linearization) |
| **Software acceptance:** which exact generation may be installed or run? | Enrollment pin, Release authorization/monotonic floors, installation/replacement/recovery transaction are separate authority and state owners | Network membership, admission permissions, Service authority, generic signing and automatic rollback | New `enrollment`, `release` and `installation` have genuine new command consumers. Portable byte authentication, native transactions/recovery and bounded Source/permission startup have separate boundaries; complete installed Service acceptance is absent. [Enrollment](../technical/enrollment-verification.md), [Release/Custody](../technical/release-update-custody.md), [Installation](../../internal/successor/installation/README.md) |
| **Authority Custody** (supporting boundary) | Purpose-specific encrypted authority storage, approved signing commitments and recovery floors; authority kinds never become one signing right | Generic `Sign`, exported private authority, Network/Service/Admission decisions assigned to their own domains; runtime Instance/Node keys | Existing current Custody is source evidence; new integration remains explicit future work; [Custody contract](../technical/release-update-custody.md) |

## Supporting mechanisms and composition

Installation's portable owner authenticates the program/generation pair. Its
Endpoint owner admits native effects and owns initial, replacement, recovery
and startup lifetimes. Command composition consumes these owners directly,
without a delegating facade. Native Modules retain physical custody; neither
detached observations nor durable phase presence grant Release, mutation,
Start, ACK replay or runtime rights. [Installation's current owner](../../internal/successor/installation/README.md)
and [Release/Custody](../technical/release-update-custody.md) hold exact admission,
record, recovery and fixed-resource rules.

| Owner | Scope | Boundary to preserve |
|---|---|---|
| New `nodeidentity` | Imported offline Node signing material and validated issuer-profile signing purpose, as its current `doc.go` states | It grants no duty and is not Person/Device/Persona, Network membership or a generic runtime Node identity service |
| Endpoint / Node composition | Construct genuine domain owners, perform live operation ordering and supervise their lifetimes | Composition must not accumulate domain policy or become the new version of the old monolithic owner; old composers cannot consume new Route |
| State/Hosting/Route physical adapters | Byte verification, durable transactions, counters, sockets and Carrier mechanisms under their declared owners | Mechanisms cannot mint authority, reset floors or copy a second domain decision implementation |
| Installation mutable-directory birth | `installation/directory` retains one original creation caller and private born inode state, exclusively creates caller-admitted paths and synchronizes same-inode account/mode promotion; checked Identity facts are detached, and Matches serves independent installed inspection | Root Installation selects paths/account, admits Release/journal/lease/effects and projects binding facts. Directory creation grants no adoption/recovery, selection, manager or startup authority; shared trusted-root predicates remain with Root lease/fixed/recovery lifetimes |
| Installation request declarations and original input | `installation/request` owns canonical bounded declarations, detached copies, native direct root-owned input and opaque original byte/inode/access reobservation; the root Request facade is consumed by actual command and transaction composition | No Release authorization, writer lease, journal, account/root creation, generation selection, manager or startup authority. General trusted-path observations remain with root Installation lease/staging/inspection, independently of request provenance |
| Installation native observations | `installation/cgroup` owns bounded actual kernel inventory and original event-descriptor observations; `installation/systemd` owns bounded typed facts, canonical manager version, fixed unit/instance inventory and joined subprocess mechanisms for daemon-reload, fixed Endpoint start and fixed sockets/Endpoint stop | The separate `installation/process` owns original proc-directory/start clock and exact executable/argv/credentials/supplementary groups/cgroup/InvocationID observations. Root-only predecessor opening and actual non-root self opening remain separate; private startup inspection retains its own read Snapshot and process observations, consumed by independently admitted bounded startup after actual root archival/ACK. Installation retains Release admission, expected generation/protection and process binding, original lease, stop decision and first failure. Native Modules have no parent import, shared lease or startup authority; Installation admits stop/reload and retains original physical join; its root manager_binding_linux.go collects request projection and actual manager/activation checks across initial, predecessor, candidate and recovery lifetimes; typed bus facts and kernel observations do not qualify an installed invocation |
| Installation fixed-unit contract | `installation/unit` owns closed Endpoint template validation, immutable original template/command-path retention and exact rendering of caller-admitted write paths, alongside pure expected Endpoint/activation protection, typed execution/invocation and distinct fresh-stopped/quiescent/running observations | Installation admits request write roots and transaction effects, systemd obtains actual typed manager facts, process/cgroup retain original physical custody. Detached configuration and property maps grant no Release or start/recovery authority |
| Installation journals | `installation/journal` owns closed preparation grammar/order and opaque original preparation/transition directory/file custody; transition owns finite creation/directory/replacement groups, exact record bytes/access, durable append/resync and partial/first-failure custody through physical close; independently reopened recovery may retain an exact original replacement group and synchronize its directory links, including an empty group | Installation retains transition record schemas/phases, original caller/lease, Release proofs, account/root/fixed-resource effects and barrier ordering. Recovery consumes preparation grammar through separate provenance/fresh-proof admission. Detached journal bytes and presence grant no adoption, mutation, generation selection or startup right |
| Installation local completion exchange | `installation/completion` owns the shared closed 160-byte frame encoding/validation consumed by root start-barrier production and independent startup inspection; it independently owns one original Unix connection, read-only directory/guard/socket-record metadata, actual root peer PID/UID/GID and exact finite frame/reply under its original deadline; cancellation physically joins its callback and Close | Private startup inspection consumes Connect/Wait between independent generation/process/manager observations. The exchange neither reads private intent nor borrows a writer lease or grants runtime/readiness authority. Root retains start observation, durable archival and ACK decision. A private producer now orders actual retained process/peer checks, a synced start record, archival and ACK; a private candidate-start lifetime now retains original process/scope custody and failed-attempt quiescence until join, with a positive actual-manager start/ACK/original-join scenario and bounded Source/permission consumer; complete installed Service acceptance remains separate |
| Installation fixed-file mutation | `installation/fixedfile` owns independent original parent/file descriptors, exclusive empty birth, bounded prefix repair on an original inode, access/ctime rechecks after journal I/O, physical writes/sync and retained original cancellation through Close | Installation keeps allowed paths/bytes/access, exact birth/replacement records and phase admission, original lease, Release proofs, predecessor join, selection and recovery. Neither a detached inode nor this physical mechanism grants mutation authorization or installed startup; no shared live root or parent import |
| Installation immutable generation files | `installation/generation` owns closed artifact names, exclusively created original directory/files, exact finite inventory, same-inode read-access promotion, detached bytes and retained partial/failure custody through physical close; independent read-only Snapshot owns sealed fifteen-file custody with its own descriptors, original file ctime/bytes and caller failure, without a writer lease or creation ownership | Installation records a mandatory closed file-inventory marker and each original per-file birth before artifact bytes; compatible complete recovery independently matches recorded native inodes and fresh-bound target facts. Installation retains fresh exact-byte proofs, original lease and caller, birth journal before writes, phase-appropriate container access, fixed-resource effects, selection, original predecessor join and startup. An independent physical Prefix retains recorded original incomplete leaves and directory, checks complete preimages, repairs only original inodes and seals complete inventory; Installation separately owns ordered writing-generation admission and fresh-proof continuation. Original successor recovery admission, continuation, terminal cleanup and close are collected with one retained owner in successor_recovery_linux.go; independent Prefix/Snapshot/process custody remains distinct. Filesystem/race admission and causal ordering controls do not prove actual-manager recovery or process-crash acceptance. Sealed files and copied physical facts grant no Release, adoption, selection or startup authority |
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

## Realized boundaries and their limits

| Owner | Maintained new behavior | Separate remaining boundary |
|---|---|---|
| Network, Admission, Hosting | Authenticated State/current observations, separately owned rights/history and durable physical budgets; genuine new command and Route consumers | Complete old-runtime cutover and installed/public qualification are not inferred |
| Route | Prefix/channel and real Carriers, registration/withdrawal, paired JOIN, parent refill, issuer and Descriptor Control, opaque delivery transport | Complete Service/Connection integration, final full-domain regression and qualification |
| Execution | Volatile local authority/session/Job, fixed native worker, original operation and joined cleanup; permission, holder and installed bootstrap consumers | Logical Service recovery and complete installed journey remain separate |
| Publication and Reachability | Public proof, consumed private Instance, live qualified Publisher, accepting registration/Store-ACK pairs, refresh/withdrawal, private recipient/replay; independent Store and local lookup history | Full Service TLS, logical Connection and installed product qualification are not implied by accepting Link or capsule transport |
| Connection | Immutable initial recipient/provenance and Attachment transcript binding under exact Execution/Publication | Complete authenticated logical stream, continuity/recovery and Application bytes require their own source-matched acceptance |
| Enrollment, Release, Installation | Pinned immutable inventory; offline exact-byte authorization/floors; separate generation authentication and native initial/replacement/recovery/startup consumers | Full interruption matrix, private Service journey and installed acceptance remain separate |
| Custody | Existing purpose-scoped authority contract and predecessor source; new destination is README-only | New domain implementation and genuine integration remain future work |

Actual source and imports are in the [package map](package-map.md) and
[implementation entry point](../../internal/successor/README.md). Component
readiness, full issue acceptance, repository integration and qualification are
different results. [GitHub #497](https://github.com/dianabuilds/ardents-network/issues/497)
owns their dependency order and live criteria. Exact earlier source hashes,
failed attempts, inspected/reproduced distinctions and fixture limits remain
in the [archived map](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/development/domain-map-before-consolidation.md)
and issue receipts. Consolidation does not upgrade their evidence level.

## Collaborations to review

| Collaboration | Evidence or right passed | Consumer check / prohibited expansion |
|---|---|---|
| Network to Admission/Route application | `CurrentRuntime` with coherent authenticated profile/member/time; `RetainDuty` from that same observation and `MatchDuty` against a fresh one | Reobserve at effect/commit points; no diagnostic Snapshot/receipt authority, successor rebinding, cached approval or callback into an already locked owner; exact role/Purpose and actual peer authentication remain consumer rules |
| Admission to Route | Purpose/receiver-bound finite accepted allowance and exactly transferred reservation lifetime | Admission owns Control 64 KiB/30s, Forward 32 MiB/1800s and Registration 1 MiB/600s. No token refund, new deadline, second claim or transport authorization based only on Node identity. Receiving Admission may request reservation rollback; Route's `Channel.HoldReservation` retains its physical return until joined retirement, including refusal without a Grant |
| Admission root creation to Route slot-history initialization | `receiving.Owner.TakeFreshRoot` passes an opaque one-use fact of genuinely durable fresh receiving-root creation for an exact public duty binding, before admission attempts; Route consumes it through its own history initialization | This is storage lifecycle evidence, not token/Network/Service authority. Retained reopen or pruned-empty spends cannot recreate a missing Route floor; attempts, Close, stale binding and late completion invalidate the fact. No shared lease/mutex or private spend parsing. The [implemented owner](../technical/successor-route-introduction.md) retains the failure/adoption boundary |
| Hosting to actual work owner | Finite work and termination reservation | Work owner joins physical borrowers before release; no duplicated budget or hidden cleanup reserve. Hosting still owns the budget and actual return; Route delays the composed return callback until physical join rather than changing Hosting or Admission policy |
| Execution to operation composition / Publication / Connection / Route | Exact live Job/operation permit and local generation; joined result checked against exact last Job | Revoke is synchronous; neighbors check before effects and final handoff, stop/join their own resources and never mutate Job state. Late completion cannot enter replacement generation; no eventually delivered event substitute |
| Execution to permission preparation composition | Private retained provenance of a successfully joined verified preparation launch under its still-live local session | It is not a live worker permit or network authority; Network/Admission still verify current facts and permission. Do not retain a dead worker's rights through this proof |
| Software acceptance to Execution launch | Exact authorized installed artifact/binding; distinct runtime byte/invocation checks | Execution cannot mint Release authorization, reset floors, choose arbitrary executables or install privilege; worker compatibility is not product platform qualification |
| Publication through Route and Reachability | Authorized registration/revision and signed bounded Descriptor | Receiving slot transport, publication readiness and Store ACK remain different facts |
| Reachability to Connection | Verified exact-Target proof with its bounds and provenance | Descriptor verification is not Instance authentication, readiness or an accepting stream |
| Connection to Route | Already authorized fresh Attachment attempt within original logical bounds | Route cannot change destination, restart an Application operation or own the recovery decision |
| Domain decision to Custody | Exact purpose-specific approved signing commitment | Custody does not acquire the caller's policy, and the domain does not receive a private root |
| Software acceptance to installation/runtime | Fresh exact-artifact authorization with retained floors | Files alone do not grant authorization, and recovery cannot introduce rollback |

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
GitHub, and assigns a real acceptance boundary only within current selection under the
[execution workflow](agent-execution.md#delegated-project-wide-domain-transition).
Completion of one slice does not complete a domain; completion of one domain
continues the program until every maintained scenario has transferred.
