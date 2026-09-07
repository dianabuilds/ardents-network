---
parent_question: R-149
title: Voting-core contracts and conditional development/verification packages
status: draft-not-ready
reviewed: 2026-09-07
---

# R-149 companion - contracts and conditional work packages

The [composition assessment](r-149-voting-core-design.md) is the rationale.
This is a preparatory dependency map, not the live issue tracker or permission
to implement an incomplete core. All D/T labels below are research references,
not runtime identities, package names or issued tracker identifiers.

## Architecture gate

All development packages require a coherent selected core and closure of B1-B5
in the assessment. An unchecked membership/finality callback, assumed honest
service or proof-shaped fixture cannot satisfy this gate. No package is ready
today. Resolve the design in this order, revisiting earlier choices on failure:

1. A real admission resource and its common non-duplication/bootstrap context
   together (B1/B2), including whether additional allocation increases influence.
2. An exact sampling, entropy and willingness protocol (B3).
3. Joint fault/progress/privacy/resource parameters and counterexamples (B4).
4. Selected components, exact formats, interfaces, storage and integration (B5).
5. Promote accepted decisions to their current owners and necessary ADRs;
   turn these conditional packages into sequential implementation-ready issues.

The issues must point to current specifications, not require Terra to infer
rules from these research tables. No C0 milestone, issue or parallel task is
created by this document.

## Proposed semantic contract

These are design requirements to resolve/promote, not an adopted wire grammar.

| Requirement | Observable property |
|---|---|
| V01 - purpose | A result authenticates one declared voting context and allowed choice. It supplies no arbitrary action, private-data access or general network authority. |
| V02 - scope | Exact rules, task type/version, conflict/predecessor, proposal/attempt and outcome are bound together. No proposer-selected weaker classification of the same effect. |
| V03 - influence | The same accounted resource split across keys cannot improve aggregate selection distribution, weight, task allowance or retry priority. Proof copying and key-count shortcuts fail. Actual mechanism remains B1. |
| V04 - selection | Required proposal/pool/willingness commitments precede the specified unpredictable selection input. Verification proves the entire required context, not merely inclusion of convenient participants. |
| V05 - willingness | The original selected denominator remains fixed. Missing responses, withdrawn local willingness and censored acknowledgements do not silently change the instance. |
| V06 - ballots | Choices are publicly verifiable under separate Voting Pseudonyms. Signatures bind the exact context; a signature from another context grants no weight here. |
| V07 - same-instance result | Exact distinct selected units/weights satisfy the selected certificate rules. Duplicates, inconsistent signers or invalid signatures cannot create weight. Protocol conflict remains observable. |
| V08 - continuation | A local timer or missing proof cannot finalize global rejection or authorize a conflicting retry. Close and successor authority require their actual selected evidence. |
| V09 - durability | Reopen preserves admitted context, signed-choice restrictions, verified result and continuity floors. A crash cannot manufacture another valid vote or restore stale authority under the selected correct-host model. |
| V10 - bounded work | Parse, proof checking, queues, bandwidth, history, retry and signing fit one finite role budget, charged across identities and helper processes. |
| V11 - privacy | No real-identity enrollment, Node/Persona/Service key reuse as an implicit voter identity, owner-secret access, private-traffic accounting or disclosure through diagnostics/consumers. Public ballot visibility is explicit. |
| V12 - dependency loss | Missing pool, entropy, close, freshness or required data yields the defined bounded observation. It cannot select an administrator, weaker verification or a substitute seed. |
| V13 - compatibility | Unknown versions, noncanonical encodings and incompatible contexts are rejected without reinterpretation, downgrade or writes to another owner's root. |

### Objects that the final format must represent

| Object | Required semantic content | Still blocking exact bytes |
|---|---|---|
| Rules/context reference | Network/domain, rules version, permitted task/outcome class, voting/policy parameters, admission/close context and lifecycle bounds. | Actual roots, parameter profile and version/upgrade contract. |
| Proposal | Exact bounded typed content or qualified available content commitment, task version, conflict/predecessor, attempt and proposer authorization/admission evidence. | Chosen admission proof and grammar. A content hash alone proves neither availability nor admissibility. |
| Eligible snapshot | Bound participant/resource entries, permitted weights, membership validity, any standing willingness and complete admitted-domain proof. | B1/B2. A list of keys or sampled inclusion proofs is not a complete independently justified pool. |
| Selection evidence | Snapshot/proposal commitment, exact randomness round and proof, selected units, deterministic order and selected algorithm/version. | B3 and whole-pool/variable-size semantics; no hidden `selected=true` oracle. |
| Ballot | Instance/rules/selection/conflict bindings, selected pseudonym/unit authority, one allowed choice and its signature. | Selected cryptographic/encoding suite and whether multiple eligible units are grouped without changing influence. |
| Result evidence | Exact bound outcome and distinct qualifying ballot/weight evidence; consumer-needed close/currentness evidence remains separately identifiable. | Actual proof sizes and finality rules; collecting signatures is not implicitly a threshold-signature/DKG scheme. |
| Close/successor evidence | A qualified admitted ballot boundary and final disposition, or the selected protocol's equivalent; old/new attempt continuity and conflict binding. | B2/B4; no invented universal expiry certificate. |

Compare the current native bounded encoding discipline with a narrowly specified
deterministic CBOR candidate. The final format must reject duplicate fields,
alternate encodings, integer/count overflow, non-finite or unsupported values,
unknown mandatory versions and aggregate oversize before expensive work. Do not
choose hashing/signing bytes by serializing an arbitrary language map.

### Proposed interface responsibilities

Names here are explanatory operations, not selected public RPC method names.
Separate local owner actions from peer evidence exchange. Final exposure,
transport and authentication depend on B5; a local interface need not become RPC.

| Operation | Input and result | Rights / failure and retry rule |
|---|---|---|
| Prepare/sign a proposal or choice locally | Validated context plus explicit owner choice; return a bound signed envelope. | Only the owner's local signing boundary; never accept a private key from a peer or expose a general remote signing command. Persist the required lock before emitting a signature. |
| Publish a proposal | Bounded signed proposal/admission evidence; return a scoped receipt or rejection. | A receipt acknowledges only its specified stage. Exact retry is idempotent within selected durable retention; replay scope/retention is not an unlimited promise. |
| Publish standing willingness, if selected | Scoped owner announcement for future eligible contexts; return receipt. | No global reputation or automatic permission; it cannot mutate an already fixed draw. This operation is absent if the selected profile does not need it. |
| Obtain context/selection evidence | Exact expected context and bounded cursor/object request; return evidence plus next cursor or defined unavailable result. | The reader verifies proof and completeness; pagination cannot omit entries while asserting complete state. No unbounded enumeration of all people or network activity. |
| Publish a ballot | Valid signed ballot; return duplicate, recorded/pending or typed refusal. | Validate scope and admission before expensive work. Duplicate delivery cannot add weight. Recording is not final inclusion unless that exact guarantee is proved. |
| Obtain result/close evidence | Instance/conflict reference and acquisition bound; return verified-by-caller evidence or observation. | A missing result is not proof of rejection. No remote `finalize`, `reroll`, `force-close` or unrestricted effect command. |
| Inspect local status | Exact context; return known evidence stage, local availability and bounded diagnostics. | Do not leak secrets, real identity, routing relationships or full raw payloads; a local timeout is labeled local. |

The concrete failure contract must distinguish invalid encoding, unknown rules,
unauthorized/out-of-scope, invalid proof, exact duplicate, replay mismatch,
missing required data, locally exhausted acquisition, stale context, conflicting
valid evidence and storage failure. External errors must not become an oracle
for private membership/activity. The final schema defines which details a
permitted caller can receive; these categories are not a ready wire enum.

## Conditional development packages

Each future issue contains V-references, current owner/ADR links, explicit
non-goals, finite limits, runnable checks, environment and stop conditions.
The future core owner/package is selected at B5; this map creates no speculative
package or parallel maintained implementation. Every D package is blocked by
the architecture gate in addition to the dependencies listed below.

| Package / dependency | One observable development outcome | Required acceptance evidence |
|---|---|---|
| D1 - context verification; gate | One real selected context can be acquired as finite input, authenticated and reopened through its selected owner boundary. | Valid, forged, incomplete, stale, incompatible and oversized contexts; actual membership/close proof validation; no `valid=true` fixture; V01-V04,V10,V12,V13. |
| D2 - proposal and admission; D1 | One permitted proposal has an immutable admitted identity and bounded duplicate/conflict behavior. | Cross-key copied proof, changed bytes on retry, shared conflict/predecessor, grinding/admission limits, withheld payload; V02,V03,V08,V10. |
| D3 - exact draw; D2 | The selected algorithm derives/verifies one eligible committee for that admitted instance. | Independent reference vectors, changed roots/seed/round, reordered/omitted entries, key splitting, unknown/tied scores, withheld entropy and biased confirmation/reserve cases where applicable; V03-V05,V12. |
| D4 - local ballot signing and verification; D3 | An owner emits one permitted scoped choice and peers validate it without access to owner keys. | Wrong instance/selection, replay, duplicate weights, malformed signatures, crash before/after lock, unavailable local custody and prohibited remote signing; V06,V09-V11,V13. |
| D5 - result evidence; D4 | A caller verifies a real result certificate or receives the precise incomplete/conflict observation. | Boundary counts/weights, duplicate receipts, opposite certificates, equivocation, delayed ballots and no certificate at timeout; V05-V08,V10,V12. |
| D6 - close, continuity and reopen; D5 | The selected proof safely permits close/continuation and remains consistent after restart. | Partition, stale/withheld close, disjoint replacement juries, cross-attempt conflict, crash/rollback, required pruning/currentness bounds; V08-V10,V12,V13. |
| D7 - peer acquisition and owner-facing flow; D6 | One complete bounded journey exchanges actual evidence and exposes truthful progress/result through selected interfaces. | Retries, truncation, overload, cancellation, hostile peers, loss of dependencies and recovery with all helpers charged; V01-V13. No new application effect power. |
| D8 - application boundary and delivery docs; D7 | A declared consumer can inspect a result while independently refusing unauthorized effects; owner instructions cover start, vote, result, failure and recovery. | Valid voting proof cannot read a private owner store, change unrelated state or execute proposal code. Documentation walkthrough uses synthetic participants; V01,V02,V11-V13. No ban feature or other binding application is bundled. |

This ordering denotes dependency, not eight mandatory packages/directories or
an estimate. Refine/split actual issues around cohesive behavior after the gate.
Implementation starts one selected issue at a time under the repository WIP rule.

## Cross-cutting verification packages

These are additional evidence tasks, not a substitute for checks in D1-D8.
Their manifests, exact fault/resource bounds and expected outcomes must be fixed
before execution. Test failures are retained; retries do not erase them.

| Verification / dependencies | Test contract | Acceptance / honest limitation |
|---|---|---|
| T1 - parsing/proof/signing; D1-D5 | Independently generated positive/negative vectors; fuzz malformed/oversized canonical inputs, identity-domain confusion, duplicate weights and signing transitions. | No unauthorized accepting path, crash or unbounded allocation in the selected envelope. This is implementation correctness, not operator independence. |
| T2 - hostile lifecycle; D6-D7 | Partition, reorder, duplicate, selectively suppress readiness/ballots/beacons, equivocate, withhold snapshots, crash at durable transitions and present disjoint-jury certificates. | Exact invariant/result/unavailable outcomes for every predeclared schedule; no final rejection from silence, unsafe reroll or stale authority resurrection. Runs actual selected protocol. |
| T3 - resources and inclusion; D7 | Replay/admission flood across many keys, maximum valid evidence, honest competing work, end-of-context cleanup, history/restart and full process-tree measurement. | Joint CPU/RSS/disk/carrier/latency limits and honest-work policy pass. Per-key counters cannot hide aggregate attack work. No simulation-only benchmark presented as public performance. |
| T4 - privacy/effect isolation; D7-D8 | Observe all declared colluding roles, transport/diagnostic/storage surfaces; inject forbidden effects and try to read owner-secret markers through every core interface. | No prohibited access path or identifying fields/key reuse; public vote visibility is explicit. Anonymous routing/unlinkability claims need their own real evidence, not just different pseudonyms. |
| T5 - integration and ownership review; D8,T1-T4 | One Product Owner plus Codex walkthrough; trace every V requirement to actual tests, current docs and selected decisions; inspect dependency/upgrade/recovery obligations. | No unexplained coverage gaps or stronger claims than evidence. This is not independent security review or evidence of human turnout. |

## Handoff and stopping conditions

For each actual issue, Terra returns the changed behavior, scoped diff, updated
owner documentation and execution evidence. The design assistant reviews
requirements/ADR conformance. A new consequential ambiguity returns to design;
the implementer may make routine coding choices within the accepted contract.

Any unresolved B gate, unsupported required test environment, unauthorized
effect/private-data path, failing repository gate or incompatible dependency
prevents the affected handoff/acceptance. Unaffected current project work can
continue. Tests and tasks remain conditional until their prerequisites pass;
the selected issue tracker alone owns live delivery state.
