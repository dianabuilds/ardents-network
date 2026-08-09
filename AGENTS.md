# Repository instructions

## Repository state

- `main` is a greenfield product and protocol research workspace.
- The previous Go/Waku implementation is preserved in the remote `old` branch.
- Do not copy architecture, terminology, dependencies, or generated artifacts
  from `old` unless a current research record explicitly justifies doing so.
- Go is the selected language and runtime foundation for the maintained project
  under ADR-0009. Transport, storage engine, consensus system, blockchain,
  route implementation, public wire protocol, and application runtime remain
  unselected.

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

## Order of authority

When materials disagree, use this order:

1. accepted ADRs;
2. the product contract and threat model;
3. completed research records and their evidence;
4. experiments;
5. legacy code and documents in `old`.

Open questions are not decisions. Experiments are evidence, not project
foundations.

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
- Name each Go file after one implementation responsibility. Production files
  have a hard maximum of 250 lines; every Go file,
  including tests, has a hard maximum of 500. Split a file before creating a
  package. Catch-all filenames such as `model.go`, `support.go`, `types.go`,
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
  `docs/development/dependencies.md` before changing `go.mod`.
- Run `make quick-check` while writing code and `make check` before integration.
  Do not weaken or bypass a failing gate. Tools are installed only through the
  explicit `make tools-install` command.
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
- Interactive and Shielded operations may have different guarantees; never
  silently downgrade one to the other.

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
