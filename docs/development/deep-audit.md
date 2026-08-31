# Deep audit campaign

Status: **current audit method; activation still requires the Product Owner to
freeze one exact C0 commit, artifact set, claims, non-claims, and review
environment**

Prepared: 2026-08-26

This document defines the formal internal whole-codebase audit that runs after
known product, architecture, and truth-remediation work is complete and one
exact candidate is frozen, but before promotion. It is a reusable engineering
process, not a delivery-horizon backlog, a Qualification receipt, an external
security review, or evidence that the present repository has passed the
campaign.

A preliminary read-only state review may inventory obvious blockers before
remediation. It is not C0 activation, complete A-F coverage, or a reusable
verdict for the later candidate. Its purpose is to avoid spending full audit
and qualification effort on already known transitional defects.

The campaign investigates one immutable candidate from accepted product and
security claims down to individual implementation paths. It records complete
declared coverage, proves material findings, repairs root causes in separately
identified candidates, and reruns every affected evidence profile before a
final digest may be accepted.

## 1. Purpose and honest limit

The campaign must:

- cover every maintained package, Go file, public command, persisted format,
  wire format, state machine, trust boundary, and selected release claim;
- review the candidate along six distinct tracks: architecture/invariants,
  hostile security, concurrency/state/lifecycle, network/protocol/wire, code
  quality/simplicity, and test adequacy/missing scenarios;
- distinguish sourced fact, implementation fact, code-derived inference,
  executed measurement, and unresolved uncertainty;
- require a reachable behavior or attack trace and a proof strategy for every
  material finding;
- retain initial failures instead of retrying or rewriting them away;
- identify which Qualification evidence becomes invalid after each change; and
- leave a final residual-risk register even when every selected gate passes.

No model, reviewer, static tool, test suite, or finite campaign can prove that
the implementation contains no unknown defect. Completion means that the
declared corpus and claims were systematically investigated by the recorded
methods, all known material findings received a disposition, remaining
uncertainty is explicit, and the exact changed build passed applicable
requalification.

The internal campaign never counts as the independent external review required
by a public promotion gate. Product Owner participation also does not make the
campaign independent market, usability, operator, governance, or security
validation.

## 2. Authority and change control

Reviewers use the repository authority order:

1. accepted ADRs;
2. product contract and threat model;
3. current technical contracts;
4. completed research records and their evidence;
5. implementation and maintained tests;
6. completed research or historical Git material, only when a current owner
   explicitly requires provenance.

Open research questions are not implementation defects merely because the
candidate cannot answer them. Conversely, working code does not resolve an
open product or architecture decision.

Discovery is read-only. A reviewer may run non-mutating checks and create
disposable evidence outside the repository, but cannot edit the Audit Baseline.
Proof tests and repairs occur only after findings synthesis on separately
identified branches or worktrees. A change to source, executable, release
profile, selected claim, platform, topology constraint, dependency, toolchain,
or safety/control input creates a new candidate identity.

An audit finding that requires a consequential new architecture, wire,
dependency, product claim, or threat boundary leaves the campaign and returns
to its owning research/ADR/product process. It is not implemented under the
label `cleanup` or `refactoring`.

## 3. Activation boundary

The preferred activation point is after the selected product journey is
functionally complete, known accepted remediation and Network/Application
separation are integrated, and its release-readiness matrix exists, but before
the promotion decision. A separate post-release campaign may later use
operational evidence, but cannot retroactively qualify an earlier build.

For C0, Network and Application/Browser are distinct audit surfaces joined only
by `ardents-application-interface-v1`. Activation records both extraction
receipts: the Network candidate must build and test without
`internal/browser/...` or Browser commands/packaging, and the Application
candidate must build and reproduce its command archive from only
`application-browser` plus `application-interface-v1` ownership. The audit
corpus uses [`ownership.json`](ownership.json) rather than inferring ownership
from directory names. `internal/application/broker` remains Network-owned.

The Product Owner activates the campaign only when all of the following are
true:

- one bounded release profile names its source revision, executable, platforms,
  Carrier/Entry profile, topology, participant/operator conditions, workload,
  resource ceilings, claims, and non-claims;
- feature development and functional refactoring for that profile are frozen;
- known preliminary-state findings have an implemented, rejected, deferred, or
  claim-reducing disposition, rather than being left for formal rediscovery;
- the working tree used to create the candidate is clean and reproducible;
- `make check` passes for the exact revision;
- `make headless-check` and `make browser-check` pass their isolated extraction
  rehearsals for the exact revision;
- every required higher execution profile is active with a checked entrypoint,
  or is an explicit release-blocking missing prerequisite rather than a skip;
- current ADR, product, security, technical, package-map, dependency, command,
  and testing owners describe the selected behavior; and
- the Product Owner explicitly authorizes read-only discovery against the
  exact baseline.

The campaign stops before discovery if it cannot distinguish the selected
candidate from a moving development branch.

## 4. Candidate identities

The campaign distinguishes two identities:

**Audit Baseline** is the immutable candidate under investigation. Findings and
proofs always retain this identity.

**Release Candidate** is a later exact candidate containing accepted repairs.
It receives its own source and executable digests and never replaces the
baseline in historical evidence.

The activation manifest records at least:

```text
campaign identifier
source commit and tree digest
executable digest and build invocation
Go version and complete toolchain identity
dependency/module state
operating-system images and architectures
release/profile identifier
network and topology identity
selected claims and non-claims
active execution profiles and entrypoints
required external prerequisites
audit model/agent configuration and tool access
campaign start time and Product Owner decision
```

Model aliases alone are insufficient when the audit environment offers a
stable snapshot identifier. A later model may repeat a pass as additional
evidence, but cannot silently replace the recorded reviewer configuration.

## 5. Evidence locations

The repository retains only current engineering truth and durable regression
evidence:

- this process policy;
- accepted repairs;
- behavior, property, fuzz, race, process, live, and Qualification tests that
  retain an ongoing purpose;
- updated current technical and participant-facing limitations;
- updated execution-profile registrations; and
- a concise final verdict in the canonical Qualification or release owner
  selected at activation.

Raw and generated campaign material remains in an explicitly selected external
audit workspace, never in the repository:

```text
<audit-root>/<campaign-id>/<baseline-digest>/
  manifest
  claim-map
  invariant-register
  surface-inventory
  coverage-ledgers/
  findings/
  proofs/
  rejected-hypotheses/
  qualification-impact
  raw-runs/
  residual-risk
  final-verdict
```

The audit root must not contain unredacted Authorities, Instance Keys, Local
Grants, raw Credentials, live Entry membership, Application Data, complete
Routes, or metadata captures outside their separately accepted evidence and
handling boundary. Large logs, traces, profiles, captures, fuzz output, and
model transcripts are not committed merely for convenience.

## 6. Required campaign registers

### 6.1 Claim map

Every selected claim has one trace:

```text
claim identifier and exact user-visible wording
protected behavior or information
adversary and conditions
honest limitation
canonical product/security owner
implementing Module and packages
enforcement points
tests and observable oracles
Qualification cells and retained evidence
known gaps or inactive prerequisites
```

A claim without an implementation or evidence owner is a release gap. A code
path without a claimed or technical responsibility is an ownership or closure
candidate, not automatically a feature.

### 6.2 Invariant register

Every Module entry records:

```text
cohesive responsibility
owned mutable and durable state
accepted inputs and callers
produced outputs and consumers
trusted dependencies
attacker-controlled inputs
authority and secrets held
resource ownership and ancestor budget
normal lifecycle
failure, cancellation, expiry, and shutdown lifecycle
cleanup and residue obligations
forbidden knowledge, authority, and fallback
implementing and testing locations
```

### 6.3 Surface inventory

The finite inventory includes:

- every maintained package and Go file;
- every command and supported local/public Interface;
- every untrusted decoder and canonical encoder;
- every public, persisted, command, configuration, evidence, and migration
  format;
- every state machine and durable store;
- every package owning goroutines, callbacks, locks, timers, cancellation, or
  mutable shared state;
- every listener, outbound network client, direct-source path, and Application
  attachment;
- every authority, credential, grant, key, and capability boundary;
- every platform-specific implementation;
- every dependency and generated/release input; and
- every active and inactive execution profile relevant to the selected claim.

### 6.4 Coverage ledger

Coverage is a surface-by-track matrix:

| Surface | A | B | C | D | E | F |
|---|---|---|---|---|---|---|
| Example Module | required | required | required | limited | required | required |
| Canonical codec | required | required | limited | required | required | required |

Each cell has one state:

- `pending`;
- `in-progress`;
- `reviewed-no-finding`;
- `reviewed-findings`;
- `not-applicable`, with a concrete reason; or
- `blocked`, with the exact missing contract, evidence, or environment.

`reviewed-no-finding` records files, responsibilities, invariants, transitions,
tools, tests, and rejected hypotheses examined. It cannot be an unsupported
reviewer assertion. Sampling does not complete a `required` cell.

### 6.5 Finding register

Every hypothesis and finding uses:

```text
ID and title
track and status
baseline commit/digest
affected claim/profile
owning Module and exact source locations
preconditions and adversary/trigger
expected behavior
observed or code-derived behavior
impact and evidence level
confidence and unresolved assumptions
reproducer/test/measurement
root-cause hypothesis
proposed repair boundary, not an unreviewed patch
required documentation changes
affected evidence and requalification
Product Owner disposition
```

### 6.6 Rejected-hypothesis register

Rejected hypotheses remain searchable. Each records the suspected problem,
reviewed trace, evidence that rejected it, and conditions under which the
conclusion would need reopening. This prevents repeated rediscovery and makes
negative review work auditable.

## 7. Evidence and impact scales

Evidence level and impact are independent.

### Evidence level

- `H0 hypothesis`: plausible suspicion without a complete reachable trace;
- `H1 code trace`: the reachable implementation chain is established by
  inspection, but no executable proof exists;
- `H2 deterministic proof`: a minimized deterministic test or reproducer fails
  on the baseline for the expected reason;
- `H3 environment proof`: the defect is reproduced in its required process,
  platform, live, or hostile environment; and
- `H4 Qualification observation`: a selected Qualification cell records the
  violation.

`H0` and `H1` are not reported as confirmed defects. Failure to build a valid
reproducer may reject the hypothesis, expose a harness defect, or retain an
explicit uncertainty; it does not silently confirm or erase it.

### Impact

- `Blocker`: breaks a selected security, privacy, integrity, authority,
  recovery, release-safety, or truthful-profile claim; risks key/authority or
  irreversible state loss; or makes the selected release label false;
- `Major`: violates supported correctness, lifecycle, resource, availability,
  platform, or compatibility behavior without necessarily breaking a security
  claim;
- `Minor`: bounded local correctness or maintainability defect with limited
  product impact;
- `Hardening`: no demonstrated contract violation, but a bounded change could
  reduce attack surface, ambiguity, or proof cost;
- `Rejected`: the hypothesis was falsified; and
- `Duplicate`: another finding owns the same root cause and evidence.

Severity never depends on how easy a repair appears. Confidence never replaces
evidence level.

## 8. Campaign phases and gates

### Phase 0: activate and freeze

The Product Owner selects the baseline and release profile. The coordinator
creates the manifest, external workspace, initial claim map, and empty coverage
matrix. No audit pass begins until the baseline is reproducible and immutable.

**Gate 0:** candidate identity, claims, non-claims, authority documents,
execution profiles, and reviewer configuration are complete.

### Phase 1: inventory and traceability

The coordinator enumerates the complete corpus and connects claims to Modules,
implementation, tests, and Qualification cells. It does not infer completeness
from directory naming, package discovery filters, or a previous campaign.

**Gate 1:** every selected claim has an owner and every maintained surface has
an assigned set of required audit tracks.

### Phase 2: A — architecture and invariants

Track A executes first because later tracks need an explicit invariant and
ownership model. It reviews:

- accepted ADR and technical-contract conformance;
- Module responsibility and dependency direction;
- mutable-state, resource, authority, and cleanup ownership;
- deep versus shallow Interfaces and unnecessary exported surface;
- forbidden collapse of Person, Endpoint, Application, Service, Target,
  Authority, Credential, Capability, and Node;
- hidden control roots, fallback paths, and second specifications;
- states that are impossible to express or impossible to exclude; and
- package-map accuracy and architecture-gate completeness.

The Product Owner reviews the invariant register for product meaning. Track A
does not treat generic best practice or aesthetic preference as authority.

**Gate A:** the invariant register and ownership graph are accepted for use by
later tracks; every A coverage cell has a verdict.

### Phase 3: B, C, and D — independent primary review

Tracks B, C, and D may run concurrently in separate clean contexts after Gate
A. They receive the same immutable baseline, authority documents, inventory,
and accepted invariant register. They do not receive one another's findings
until their independent primary passes finish, reducing anchoring and shared
blind spots.

#### B — hostile security review

Track B asks how every declared adversary can cause accepted falsehood,
information disclosure, authority misuse, resource denial/amplification, or a
weaker state. It covers:

- malicious Users, Services, Applications, endpoints, ordinary Nodes, source
  families, and control participants;
- replay, substitution, downgrade, fork, tagging, cross-context confusion, and
  confused-deputy paths;
- key, Authority, Credential, Capability, Local Grant, custody, backup, and
  recovery misuse;
- bootstrap, time, Network Epoch, release, update, rollback, transition, and
  supply-chain compromise;
- Sybil participation, collusion, family overlap, concentration, and hidden
  common control within the selected claim boundary;
- hostile admission, queue pressure, resource exhaustion, cleanup failure, and
  traffic/work amplification; and
- every privacy claim as protected information, adversary, conditions,
  measurement, and honest limitation.

Each material hypothesis states attacker, prerequisites, attack trace,
violated claim, observable impact, existing control failure, proof strategy,
and excluded adversaries.

#### C — concurrency, state, and lifecycle

Track C reviews every create/start/readiness/use/cancel/expire/revoke/drain/
supersede/close/crash/restart/rollback/cleanup path. It covers:

- goroutine, process, listener, connection, callback, lock, channel, timer,
  file, and fixture ownership;
- race, deadlock, lost wakeup, double-close, use-after-close, and shutdown
  ordering;
- partial initialization, cancellation between checkpoints, and failure after
  external effect but before local acknowledgement;
- deadline and Work Safety Lease monotonicity without retry clock reset;
- durable generations, revisions, watermarks, crash consistency, partial
  writes, restart, rollback, and stale-state rejection;
- abandoned attempts, recovery resources, queued copies, descriptors, and
  cryptographic-state cleanup; and
- repeated sequential and overlapping failure rather than one happy shutdown.

Detector cleanliness without terminal join, state, and residue assertions is
not sufficient.

#### D — network, protocol, and wire

Track D inventories every byte-consuming boundary from input through state
effect. It covers:

- framing, canonicality, length and allocation bounds, trailing data,
  truncation, duplicates, reordering, and malformed input;
- version, algorithm, network, epoch, Route Profile, Target, generation,
  context, role, and transcript binding;
- authentication and freshness ordering before expensive or durable work;
- replay domains, downgrade, alternate interpretation, and unknown-value
  behavior;
- connection, route, introduction, resolution, continuity, recovery, and
  terminal state-machine legality;
- flow control, backpressure, amplification, fragmentation, and queue
  accounting; and
- absence of silent fallback, hidden reconnect, Application-operation replay,
  or weaker-profile negotiation.

Parser tests alone do not complete a boundary whose accepted value later
causes unbounded or unauthorized state.

**Gate BCD:** every required B/C/D cell has a primary verdict before findings
are shared across these tracks.

### Phase 4: E — code quality and simplicity

Track E begins after semantic review establishes the protected behavior. It
asks where complexity obstructs correctness, security, maintenance, or proof.
It covers:

- duplicated enforcement or ownership;
- shallow wrappers and speculative Interfaces;
- excessive exports, mutable request bags, implicit dependencies, and hidden
  side effects;
- misleading names, catch-all responsibilities, dead code, unreachable states,
  and obsolete compatibility paths;
- functions/files whose size hides distinct responsibilities;
- inconsistent error classification or wrapping;
- generic helpers that erase domain meaning; and
- simplifications that reduce attack surface or the number of states.

Every proposed simplification names the invariants it preserves and the proof
needed after change. Pure style preference is not a defect.

**Gate E:** all E cells have a verdict, and correctness/security-relevant
simplifications are separated from optional maintainability work.

### Phase 5: F — test adequacy and missing scenarios

Track F maps each claim and invariant to observable falsification. It distrusts
an existing green result until it reviews the oracle and environment. It
covers:

- positive, negative, malformed, boundary, hostile, concurrent, cancellation,
  cleanup, crash/restart, rollback, and platform behavior;
- assertions that observe the required public/Module outcome rather than an
  incidental implementation detail;
- tests that cannot fail, hidden retry, swallowed error, reduced denominator,
  skipped required cells, and early termination;
- fuzz/property coverage for every retained untrusted decoder/canonical encoder
  pair and appropriate state machines;
- race/stress coverage for every concurrency-owning Module;
- fault injection at irreversible or externally visible checkpoints;
- resource attribution, leakage, backpressure, fairness, and amplification;
- harness fast paths, shared fixtures, response caching, in-process shortcuts,
  and other candidate bias; and
- every missing proof scenario discovered by A through E.

Line or branch coverage may guide inspection but never substitutes for the
claim-to-oracle map.

**Gate F:** every selected claim has sufficient evidence or an explicit gap;
every A-E material hypothesis has a proof strategy or a recorded reason it
cannot yet be tested.

### Phase 6: X — cross-track synthesis and falsification

A separate synthesis pass receives all A-F results only after their primary
coverage is complete. It must:

- verify material source traces rather than trust summaries;
- merge duplicates under one root-cause owner;
- challenge impact, evidence level, assumptions, and reachability;
- record rejected hypotheses instead of deleting them;
- find contradictory Module, state, trust, or failure models between tracks;
- connect local symptoms into cross-track cause/effect chains;
- audit every `not-applicable`, `blocked`, and `reviewed-no-finding` claim;
- verify that every inventory surface and claim has complete matrix coverage;
  and
- produce the residual-uncertainty and qualification-impact registers.

Example synthesis chain:

```text
C: abandoned recovery goroutine
  -> E: lifecycle ownership is duplicated
  -> B: attacker can accumulate endpoint state
  -> D: one accepted frame triggers disproportionate work
  -> F: no repeated-failure cleanup/resource test exists
```

**Gate X:** the Product Owner receives one deduplicated register with evidence,
impact, root cause, proof plan, release effect, and unresolved uncertainty.

### Phase 7: proof campaign

Proof work occurs against the immutable baseline in a separate branch/worktree
or external harness. It adds no repair. For each accepted hypothesis it seeks
the smallest valid evidence at the required profile:

```text
hypothesis
  -> complete code trace
  -> minimal reproducer
  -> failing behavior/property/fuzz/race/fault/process/live test
  -> baseline confirmation
  -> accepted, rejected, or retained uncertainty
```

A proof must fail on the baseline for the predicted product reason. A harness,
environment, timing, fixture, or observer failure is not confirmation. A fuzz
failure retains its minimized input and seed. An environment-dependent proof
records exact prerequisites and cannot pass by skip.

New audit tooling that would change dependencies or execution policy requires
its normal research, dependency, tool-installation, and gate updates before it
becomes evidence. The campaign does not bypass repository tooling rules.

**Gate Proof:** every Blocker/Major hypothesis has an H2-H4 result or remains an
explicit release-blocking uncertainty; lower-impact hypotheses have a Product
Owner disposition.

### Phase 8: Product Owner disposition

The Product Owner chooses one disposition for each deduplicated item:

- repair and rerun;
- reduce or withdraw the selected profile/claim;
- return to the owning H4 epic or research question;
- retain a truthful bounded limitation when the release contract permits it;
- reject the finding with evidence; or
- reject the release candidate.

An absent external reviewer, operator, builder, custodian, platform, or valid
test environment remains a blocked gate, not an internal implementation task.

### Phase 9: remediation

Repairs are ordered by root cause rather than file or reviewer:

1. claim-breaking and authority/security defects;
2. architecture roots shared by several findings;
3. lifecycle, durability, protocol, and resource correctness;
4. missing evidence required to verify those repairs;
5. bounded simplicity/hardening work; and
6. optional local cleanup that still justifies candidate churn.

Each remediation wave has one cohesive root cause and follows:

```text
retained failing proof
  -> minimal coherent repair boundary
  -> implementation and current-owner documentation
  -> make quick-check while editing
  -> make check before integration
  -> affected higher profiles
  -> finding disposition and qualification-impact update
```

The initial failure remains in campaign evidence. Passing a rerun closes only
the repaired candidate result, never the baseline observation.

### Phase 10: change-induced defect review

An independent pass reviews the complete baseline-to-candidate diff. It asks:

- which invariants, states, authorities, bounds, error outcomes, and cleanup
  paths changed;
- whether the repair added fallback, cross-context reuse, new exported surface,
  dependency, persistent state, wire acceptance, or irreversible transition;
- whether a new test merely repeats the repaired implementation; and
- which earlier review and Qualification evidence no longer applies.

Architecture, trust, lifecycle, persistence, wire, or claim changes rerun the
complete affected track for the affected ownership boundary. A root-level
change may require a full A-F campaign against a new baseline.

**Gate Diff:** all repairs have independent review and an explicit evidence
reuse/requalification decision.

### Phase 11: requalification and final freeze

The final candidate runs:

- all active deterministic, architecture, build, module, static, vulnerability,
  process, race, and fuzz gates;
- selected affected-platform profiles;
- selected live, soak, hostile, recovery, overload, and cleanup profiles;
- every Qualification cell invalidated by source, executable, profile,
  platform, topology, safety/control, or evidence changes; and
- repository/documentation closure inventory.

Every retained result names exact candidate, profile, inputs, environment,
denominator, resources, duration, faults, failures, and reproducible artifacts.
The final verdict fixes source commit/tree digest, executable digest, toolchain,
dependency state, release profile, claims/non-claims, evidence identities,
known limitations, and residual uncertainty.

Only that exact digest is the audited and qualified result.

## 9. Product Owner checkpoints

The one-to-one team uses five explicit human decisions:

| Checkpoint | Product Owner decision |
|---|---|
| C0 Activate | After known remediation, freeze the exact baseline, profile, claims, non-claims, and read-only scope. |
| C1 Invariants | Confirm that the invariant/ownership model expresses the intended product. |
| C2 Findings | Accept or challenge evidence, impact, claim effect, and residual uncertainty. |
| C3 Disposition | Select repair, claim/profile reduction, research return, limitation, rejection, or release stop. |
| C4 Promotion | Accept only the final exact digest and its requalification verdict. |

The Product Owner is not expected to reread every line. The checkpoints control
product meaning, accepted risk, candidate identity, and release truthfulness.

## 10. Stop conditions

Stop the affected campaign phase and return work to its owner when:

- accepted product, threat, ADR, or technical authorities contradict one
  another;
- required behavior lacks an authority decision;
- a proposed repair selects a new architecture, public wire, runtime dependency,
  storage/consensus/transport mechanism, or claim;
- the selected profile or claim changes without a new candidate identity;
- a required environment cannot distinguish product failure from harness or
  platform invalidity;
- nondeterminism remains unowned or is hidden by retry, quarantine, skip, or a
  reduced denominator;
- evidence handling would expose forbidden secrets or metadata;
- a reviewer proposes a weaker security/profile fallback to obtain green
  evidence;
- the baseline changes during discovery; or
- a required independent person or organization does not exist.

## 11. Completion criteria

The campaign is complete only when:

- every maintained package and Go file appears in the surface inventory and
  every required matrix cell has a supported verdict;
- every maintained source/test, command, packaging file, qualification lane,
  Interface-v1 file, and retained historical source has exactly one checked
  owner, with Network-v3 and Browser-v4 artifact lanes still disjoint;
- every public, persisted, command, configuration, evidence, migration, and
  wire format is covered;
- every selected claim traces to implementation, tests, and Qualification
  evidence or an explicit blocked gap;
- no unresolved Blocker or Major remains in an accepted release candidate;
- every confirmed defect has a reproducer/evidence and a disposition;
- every rejected material hypothesis retains its rejection basis;
- every accepted repair passed change-induced defect review;
- every invalidated execution profile and Qualification cell was rerun;
- residual risks and evidence limitations are explicit;
- repository and documentation closure leaves no unowned current behavior; and
- the final verdict identifies one exact source and executable digest.

Passing this internal campaign does not waive the external review, independent
operator/control evidence, or later operational review required by the selected
release label.

## 12. Reviewer runbook

Every track reviewer receives a finite list of assigned surfaces and the
following base instruction, specialized by the track sections above:

```text
Review immutable Audit Baseline <commit/digest> read-only.

Authority order:
1. accepted ADRs;
2. product contract and threat model;
3. current technical contracts;
4. completed research evidence;
5. experiments only where a current owner requires them;
6. implementation and tests.

Assigned track: <A-F>.
Assigned surfaces and required coverage cells: <finite list>.

Do not edit the repository. Do not propose a repair before establishing the
violated contract or invariant, reachable scenario, impact, assumptions, and
proof strategy. Review every assigned surface, including negative paths,
resource ownership, cancellation, failure, and cleanup where applicable.

Update every assigned coverage-ledger cell even when no finding exists. Record
rejected hypotheses. For each conclusion distinguish sourced contract fact,
implementation fact, code-derived inference, executed measurement, and
uncertainty.

Do not treat an existing green test as proof that its oracle or environment is
adequate. Do not treat a rerun as erasing an earlier failure. Do not count an
invalid or missing environment as a passing skip. Do not weaken a claim,
invent an authority decision, or reuse evidence outside its stated validity.
```

Reviewers return only:

- coverage-ledger updates;
- invariant/claim trace corrections within their assigned authority;
- findings in the required schema;
- rejected hypotheses;
- proposed proof work; and
- blockers or unresolved uncertainty.

## 13. Coordinator runbook

The coordinator uses:

```text
Maintain the immutable baseline identity, authority order, claim map, surface
inventory, invariant register, and complete surface-by-track coverage matrix.

Give every reviewer a finite corpus and clean primary-review context. Do not
share B/C/D findings until their independent primary passes finish. Do not
treat reviewer summaries, confidence, or model agreement as evidence. Require
exact source locations, reachable traces, assumptions, and proof strategies.

Audit every reviewed-no-finding, not-applicable, and blocked ledger result.
Challenge each material finding through an independent falsification pass.
Preserve rejected hypotheses and initial failures. Deduplicate under one root
cause without losing track-specific consequences.

Permit no repository edits during discovery. Track evidence validity and
requalification impact for every later change. Stop rather than invent a
missing product, threat, architecture, environment, or independence decision.

The campaign completes only when every required matrix cell has a supported
verdict, every material item has a Product Owner disposition, residual
uncertainty is explicit, repairs have independent diff review, and the final
exact digest has passed applicable requalification.
```

The coordinator may divide independent, finite work across subagents when the
selected audit environment supports it. Parallelism reduces elapsed time, not
the coverage or evidence standard. A synthesis agent must reopen material
source evidence rather than merely vote among subagent conclusions.

## 14. Preparation now versus activation later

This document is the only durable formal-audit preparation required before the
product candidate exists. Do not create empty audit directories, speculative package
assignments, future Qualification commands, new dependencies, or model-specific
prompts merely to make the campaign appear active.

At activation, instantiate this policy with the actual candidate manifest,
claims, packages, formats, execution profiles, commands, environments, and
reviewer configuration. That instantiated material is evidence for one
candidate, not a second permanent project specification.
