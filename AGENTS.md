# Repository instructions

## Repository state

- `main` is a greenfield product and protocol research workspace.
- The previous Go/Waku implementation is preserved in the remote `old` branch.
- Do not copy architecture, terminology, dependencies, or generated artifacts
  from `old` unless a current research record explicitly justifies doing so.
- Go is the selected language and runtime foundation for the maintained project
  under ADR-0009. Accepted ADRs also select one bounded closed-alpha native
  Route, wire grammar, and TCP/TLS plus QUIC Carrier set; those selections do
  not imply a public or long-term protocol commitment. Public transport,
  storage, consensus and blockchain remain unselected. ADR-0081 selects the
  bounded closed Ubuntu text-Service successor, including its Application
  confinement design; it is not implemented or qualified by that selection.
- New delivery-horizon and epic labels are planning provenance only. They must
  not become runtime identities, package boundaries, wire fields, or domain
  terms. Immutable historical evidence may retain its original candidate
  identifiers. Existing accepted persisted or wire identities that contain an
  old stage label remain compatibility obligations until an explicit researched
  migration retires them; new maintained product code and contracts use domain
  language.

## Current collaboration model

- The active project team is one human Product Owner and Codex working
  one-to-one.
- Do not assume access to additional developers, researchers, interview panels,
  operators, auditors, community managers, or other staff unless the Product
  Owner explicitly adds them.
- Scope research, implementation, operations, and maintenance for this actual
  capacity. Prefer a smaller product contract and maintained community
  components over plans that require a hidden organization to execute.
- A structured Product Owner walkthrough can accept a product hypothesis or
  architecture tracer. It is not external market, novice-usability, anonymity,
  or independent-security validation.
- External users and independent reviewers may be recorded as future release
  gates, but must not be scheduled as if they are currently available.

### Closed text-Service execution with Astra

- For the already authorized closed text-Service work under issue #50, the
  Product Owner selected Astra for the continuation. This does not reassign or
  authorize implementation of the separate R-149 agreement-system research.
- Follow [agent execution and handoff](docs/development/agent-execution.md).
  Use one active implementation slice with an observable acceptance boundary.
  Record other prepared work as paused, locally verified, or awaiting integration.
- Preserve the existing branch, staged changes and untracked implementation.
  A fresh agent session is not a reason to restart from main or discard work.
- Keep GitHub Issues as the execution ledger and give the Product Owner concise
  progress in the active conversation. Distinguish component readiness, full
  issue acceptance and integration; no test-only reachability or gate waiver.
- Do not start implementation agents or parallel branches merely to accelerate
  this work. A skill-required review may use read-only reviewers of one bounded
  delta; it is not independent security validation or another implementation.
### Agreement-system design and implementation responsibilities

- For the agreement-system work under R-149, the Product Owner assigns product
  research, requirements, threat analysis, architecture and ADR preparation to
  the design assistant in the current research task. Implementation is intended
  for Terra (`gpt-5.6-terra`) after a bounded slice is ready. This is a division
  of Codex work, not additional human staff or independent security review.
- The current phase is design and requirements in this repository. Do not turn
  the committee discussion, an open research question or a candidate brief into
  maintained subsystem implementation. This preference does not itself start
  another task or authorize parallel implementation work.
- Follow the [research-to-implementation handoff](docs/development/documentation.md#research-to-implementation-handoff).
  Terra may make routine implementation choices within the selected contract;
  consequential product, authority, privacy or protocol gaps return to design
  instead of being silently resolved in code. Existing authority, ownership,
  dependency and C0 work-in-progress rules remain binding.

## Order of authority

When materials disagree, use this order:

1. accepted ADRs;
2. the product contract and threat model;
3. completed research records and their evidence;
4. experiments;
5. legacy code and documents in `old`.

Open questions are not decisions. Experiments are evidence, not project
foundations.

## Current-document route

For maintained C0 work, read the current [product scope](docs/product/scope.md),
[threat model](docs/security/threat-model.md), the affected current technical
owner, and the relevant development owner before searching more broadly. Read
an ADR only when that current owner or scope links to it; read a completed
research record, experiment, audit receipt, compatibility tree, or `old` only
to recover named provenance. Historical material cannot supply a current
requirement, implementation plan, support claim, or backlog item.

The C0 Closed Alpha delivery state belongs in the selected issue tracker and
milestone. Current product documents state the contract and its limits; they
must not become a second task ledger or repeat historical campaign chronology.

## C0 work-in-progress limit

The one live C0 ledger is GitHub Issues in the
[`C0 Closed Alpha` milestone](https://github.com/dianabuilds/ardents-network/milestones).
Until that milestone exists and is accessible, do not begin a new C0
implementation slice; only contract clarification, review, or a green-baseline
repair may proceed. The milestone permits exactly one in-progress C0
implementation issue and at most one explicitly selected active research
question. An open or deferred question is not active merely because its
historical evidence remains in the repository.

## Research discipline

- Every research effort starts with a decision-relevant question from
  `docs/research/questions.md` or a new question added there.
- Use `docs/research/template.md` for durable research records.
- Prefer primary sources: specifications, papers, official documentation,
  source code, security advisories, and reproducible measurements.
- Record access dates and distinguish sourced facts, measurements, assumptions,
  and recommendations.
- Define falsification criteria before running an experiment.
- A library being popular is evidence of ecosystem maturity, not proof that its
  threat model fits Ardents.

## Go project and experiments

- The repository has one root Go module. Maintained Go code belongs in thin
  `cmd/<name>` adapters and cohesive deep modules under `internal/<domain>`.
- Do not create empty packages, speculative interfaces, or generic dumping
  grounds named `util`, `common`, `misc`, `types`, `interfaces`, or `api`.
- Package and command names describe one responsibility, use normal Go naming,
  and must be registered in `docs/development/package-map.md`. Renaming or
  adding a package is an explicit architecture change.
- Keep module interfaces small and implementation details unexported. A new
  package requires a real cohesive boundary, not merely another source file.
- Name each Go file after one implementation responsibility. Every Go file,
  including tests, has an interim hard maximum of 500 lines. Split by
  responsibility, not merely by line count, before creating a package. Record
  the cohesive responsibility, local invariants, rejected split, and behavior
  evidence when a file's size or complexity makes that judgment non-obvious.
  Catch-all filenames such as
  `model.go`, `support.go`, `types.go`,
  `helpers.go`, `common.go`, `misc.go`, and `util.go` are forbidden.
- A nested directory is a real package, not visual grouping. Create a
  subpackage only when it independently satisfies the same responsibility,
  Interface, Implementation, test, and package-map requirements.
- Every new package or subpackage must add `doc.go`, maintained Implementation,
  behavior tests, at least one non-test caller, its exact permitted imports,
  and command ownership where applicable in the same change. Directory nesting
  grants no implicit import direction; the package map is authoritative.
- First-party cgo, `unsafe`, implicit `init`, and `panic` require a superseding
  accepted ADR and dedicated risk tests.
- Prefer the standard library. Record and review every runtime dependency in
  `docs/development/dependencies.md` before changing `go.mod`. Existing and new
  direct/transitive dependencies, toolchains and build/test tools must satisfy
  its maintenance and vulnerability acceptance rule. Every known finding needs
  a fix or scoped, reproducible non-applicability evidence; severity alone is
  not an exemption, and a passing scanner does not prove ongoing support.
- Run `make quick-check` while writing code and `make check` before integration.
  Do not weaken or bypass a failing gate. Tools are installed only through the
  explicit `make tools-install` command.
- Tests belong to one checked execution profile. A missing selected Docker,
  binary, privilege, platform, or orchestration prerequisite is an invalid
  environment, not a passing skip; retries never erase an earlier failure.
- Current documentation is promoted to its product, security, technical,
  operations, reference, or engineering owner with the owning change. Stage
  material and historical evidence are provenance, not a second specification.
- Disposable research spikes may use `experiments/<question-id>-<slug>/`, but
  maintained Go modules and project packages do not belong there.
- Each experiment must include a README stating the question, hypothesis, run
  instructions, captured evidence, result, and disposition.
- Do not create `src`, `pkg`, `api`, `sdk`, or deployment trees merely to make
  progress look like implementation.
- Do not implement cryptographic primitives. Evaluate reviewed, maintained
  implementations against the declared threat model.
- Keep generated files, dependency caches, databases, captures containing
  sensitive metadata, and build outputs outside the repository.

## Product and domain language

- `CONTEXT.md` is the canonical glossary and contains product language only.
- Update the glossary when a domain term is resolved; avoid implementation
  details there.
- Human-facing names are Service Names. Opaque cryptographic targets are not the
  normal user experience.
- Person, Device, Persona, transport identity, Service Target, Credential, and
  Capability are separate concepts and must not be collapsed silently.

## Security claims

- Assume censorship, malicious peers, Sybil actors, relay collusion, endpoint
  compromise, infrastructure seizure, traffic analysis, supply-chain attacks,
  and governance capture.
- State every privacy claim as: protected information, adversary, conditions,
  measurement, and honest limitation.
- Encryption of payload is not anonymity. Decentralized storage is not
  availability. Multiple nodes are not independent operators.
- Ordinary Ardents use evolves one common protection baseline under ADR-0077;
  do not introduce parallel security modes or an optional stronger Route.
  Follow the system protection workstream in `docs/development/documentation.md`.
  Missing required protection never authorizes a weaker accepting path.

## Durable decisions

- Create an ADR only for a consequential, hard-to-reverse trade-off.
- Keep ADRs short and place them under `docs/adr/`.
- Technology selection requires a research record and an accepted ADR when it
  creates meaningful lock-in.

## Git and workspace hygiene

- Preserve unrelated user changes.
- Keep commits scoped to one research result, decision, or tracer slice.
- Never place caches or temporary generated files inside the repository.
- Do not rewrite or delete the `old` branch.
