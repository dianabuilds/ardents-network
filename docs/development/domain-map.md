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

| Domain / question | Owned rules and distinct consistency owners | Explicitly outside | Current new owner or future source map |
|---|---|---|---|
| **Network:** which authenticated facts may be used now? | Candidate eligibility/assignment, Epoch/profile history and conflicts, bound membership, trusted time/floors, coherent accepted observations and same-observation retained duty; local participation is a separate lifetime | Route purpose/path choice, tokens/spend, physical Hosting budget, process-wide resource monitor, private profile signing, Job/Service authority | [new Network](../technical/successor-network-state.md), `internal/successor/network`; State application and authentication/Source/durable/duty adapters have separate responsibilities |
| **Admission:** which finite rights can be obtained and spent? | Allocation; issuance quota; private material and retained results; holder permission/pending batch/stock; presentation history; receiving verification, allowance and durable spend are separate owners | Network authenticity, transport scheduling, Introduction slots, provider budget, Local Grants, Custody keys and file delivery | [new Admission boundary](../technical/successor-admission-boundary.md), [implemented owner map](../../internal/successor/admission/README.md) |
| **Hosting:** which physical allowance can this owner reserve? | One provider-period Tx/Rx accounting policy, coherent measurements, durable budget, work/termination reservations and their exact release | Token authority/debit, Node assignment, role envelope calculation, transport stop/join, CPU/RAM/cgroups or global process monitor | [new Hosting](../technical/successor-hosting-budget.md), `internal/successor/hosting` |
| **Route:** which protected leg and role channel may this operation use? | Entry/Interior selection and retention; role-purpose rules; prefix/lanes and credit; forwarding parents/children; receiving Introduction slots/delivery/floors; JOIN pairing and physical termination | State acceptance, Admission quotas/verification/spend, Hosting budget, publication readiness, Descriptor authority/history, Instance authentication and logical recovery | [Route owner map](route-domain-boundary-analysis.md), [migration contract](route-migration-contract.md) and [implemented retained prefix](../technical/successor-route-prefix.md); `internal/successor/route`, `selection`, `ardp`, `carrier`, `transport` with real `ardents-next` consumers. Introduction slots/delivery/floors and JOIN remain future Route responsibilities; the prefix is not the complete domain |
| **Local Application execution:** may this process invocation perform this operation? | Volatile Local Grant generation; local session with exact current/last Job and one-use handoff; separate supervisor cleanup capacity and terminal admission latch; qualified launch and joined original worker descendants. Job is a lifetime entity within the session consistency boundary, not an independent network Context | Token allowance, Service authority, Target/path choice, private routing history, remote publication, Connection recovery, software acceptance, global resource pressure, text semantics and qualification Run | [Source-backed boundary/model](application-execution-domain-boundary-analysis.md), current evidence `application/broker`, Endpoint Job/worker and scattered Service effect/handoff checks; [confinement owner](../technical/application-confinement.md). This is verified design inventory, not a new implementation or installed qualification |
| **Service Publication:** which authorized Instance is accepting work? | Publication generation/revision, current/pending/predecessor registration pair, private recipient lifetime, ACK/readiness, refresh and withdrawal; Instance authority/material remain purpose-scoped | Credential root signing, raw Route transport, Descriptor Store conflict floors, logical Connection recovery, Local Grant/Job mutation and worker cleanup; retain an exact live Execution operation instead | Proposed new boundary; current source evidence: Instance/publication, Endpoint registration/Introduction; [Service owner](../technical/endpoint-service-runtime.md), [private reachability](../technical/private-reachability.md), [Execution seam](application-execution-domain-boundary-analysis.md#source-inventory-collect-split-and-leave) |
| **Reachability:** what current proof is valid for this Target? | Receiving Descriptor Store and durable conflicts; local lookup verification/history and caches are separate owners at separate principals | Publication readiness, admission spend, path choice, Service Instance authentication and logical Connection | Proposed new boundary; current source evidence: `service/reachability`, Endpoint descriptor history, Node resolution; [reachability contract](../technical/private-reachability.md) |
| **Service Connection:** how does one authenticated logical stream retain its identity? | Immutable destination/provenance, exact Instance TLS authentication, ordered byte state, Attachment generation, continuity/recovery deadline and retained Service terminal outcome; checks the exact retained Execution operation at effects and final handoffs | New Target/Application operation, new Grant, Job mutation/worker cleanup, publication authority, path policy and independent physical reservations | Proposed new boundary; current source evidence: `service/connection`, Endpoint Service/binding/recovery; [Service owner](../technical/endpoint-service-runtime.md), [Execution seam](application-execution-domain-boundary-analysis.md#source-inventory-collect-split-and-leave) |
| **Software acceptance:** which exact generation may be installed or run? | Enrollment pin, Release authorization/monotonic floors, installation/replacement/recovery transaction are separate authority and state owners | Network membership, admission permissions, Service authority, generic signing and automatic rollback | Proposed organization of existing owners, not one aggregate; [Release/Custody owner](../technical/release-update-custody.md), Enrollment/Release/installation source |
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

## Collaborations to review

The following are semantic relationships, not a general Go import allowance.
Actual package direction and third-party use still require exact registration.

| Collaboration | Evidence or right passed | Consumer check / prohibited expansion |
|---|---|---|
| Network to Admission/Route application | `CurrentRuntime` with coherent authenticated profile/member/time; `RetainDuty` from that same observation and `MatchDuty` against a fresh one | Reobserve at effect/commit points; no diagnostic Snapshot/receipt authority, successor rebinding, cached approval or callback into an already locked owner; exact role/Purpose and actual peer authentication remain consumer rules |
| Admission to Route | Purpose/receiver-bound finite accepted allowance and exactly transferred reservation lifetime | No token refund, new deadline, second claim or transport authorization based only on Node identity. Receiving Admission may request reservation rollback; Route's `Channel.HoldReservation` retains its physical return until joined retirement, including refusal without a Grant |
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
| New Hosting owns independent durable provider budget and finite reservations | Local commits `98052a2f9` and `b989b39da`; [contract](../technical/successor-hosting-budget.md), [checked profile](testing.md#current-profiles) | No admission/network authority, invoice correctness, transport progress or installed-product qualification |
| Cross-domain regression belongs to test composition under `cmd/ardents-next`; the subsequent cleanup removes `internal/successor/admittedwork` and `admission work` | `hosting_transfer_fixture_linux_test.go`, `hosting_work_fixture_linux_test.go` and their behavior tests; [current profile](testing.md#current-profiles); exact-tree 21-package new-domain race/isolation/vet/build receipt recorded in the execution ledger | Test composition grants no product workload, Route consumer, Carrier qualification or whole-repository acceptance |
| New Network has pure rules, its own State root/application, cohesive adapters and real signed-intake/Admission command consumers; duplicate runtime projections, private signing and process monitoring are excluded | [Network owner](../technical/successor-network-state.md), [committed handoff evidence](https://github.com/dianabuilds/ardents-network/issues/493#issuecomment-5977818747), commit `4d56c37a`; `accepted_state.go`, `retained_duty.go`, State `accepted_observation.go` and command `network_authority.go`; earlier dirty regression independently repeated, committed source and later receipts inspected as distinguished above | No old Node/Endpoint/Route cutover, TCP/TLS or QUIC Route qualification, whole-repository gate or installed-product qualification is established. The scoped commit does not integrate the neighboring regression-composition removal |
| Genuine new Network, Admission and Hosting preserve authority and physical-resource ownership across durable and joined boundaries | `cmd/ardents-next/network_admission_hosting_linux_test.go`: refill original deadline and dual reservations, pre-spend loss, successor after burn/reopen, conflict/clock refusal and joined socket work; signed command fixtures use separate installation-local roots; independently repeated with the new-domain race suite | The local socket harness is not a selected Carrier implementation. Earlier standalone supplied-fact Admission tests do not prove Network authenticity; external Admission staticcheck findings remain with their owners |
| Route isolation has a mandatory disk-read contract, root instruction and independent import guard | [migration contract](route-migration-contract.md), [AGENTS.md](../../AGENTS.md), [architecture guard](../../internal/architecture/route_migration_isolation_test.go); both focused Route guards independently passed alongside new-domain isolation | Policy preparation is not a Route implementation. Focused guard passes do not establish full repository regression |
| New Route owns retained Entry/Interior prefixes, authenticated role channels, bounded serialized OPEN and joined physical reservation retirement; receiving/pool terminal results retain late physical failures | [Bounded owning issue](https://github.com/dianabuilds/ardents-network/issues/496), final source `69406f7c896aa45321b5556f2b63d0c99f2dba44`; initial prefix `13f3fb0e4`, OPEN correction `d08dbc18b`, receiving terminal correction `944975f9e`, strict expiry oracle correction `69406f7c8`. [Prefix owner](../technical/successor-route-prefix.md); real holder `prefix-open`/`prefix-close` and `route receive`; genuine both-Carrier/compiled-process/refusal/reopen tests, transport scheduling/OPEN/terminal/pool tests and selection floors. The orchestrator matched all 495 non-AGENTS working files and committed blobs to the final 496-file manifest SHA-256 `5f66ba9e93e9b30aafc1c4f9f5b2d7e412c8ed57d4fe80f7e4e417df1836b4d3` (one registry CRLF normalization), inspected executor red/green, full Linux new-domain race, Windows full-gate and normal-hook receipts, and reviewed authority/spend/join and both terminal/oracle corrections. Root independently ran Linux-targeted Route/command staticcheck successfully; the full transport/regression tests were inspected, not independently rerun | This bounded source establishes no complete Route, Introduction, JOIN, Publication, Connection, installed-product or privacy qualification. Late writer-failure controls are mechanical; genuine Carrier/process scenarios establish actual consumer reachability and regression, not an injected late-writer trial through the entire admitted path. Fixed defects and passing controls do not establish absence of all defects or independent security validation |

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
