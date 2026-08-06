# Repository instructions

## Repository state

- `main` is a greenfield product and protocol research workspace.
- The previous Go/Waku implementation is preserved in the remote `old` branch.
- Do not copy architecture, terminology, dependencies, or generated artifacts
  from `old` unless a current research record explicitly justifies doing so.
- There is no selected production language, transport, storage engine, consensus
  system, blockchain, or application runtime.

## Order of authority

When materials disagree, use this order:

1. accepted ADRs;
2. the product contract and threat model;
3. completed research records and their evidence;
4. experiments;
5. legacy code and documents in `old`.

Open questions are not decisions. Experiments are not production foundations.

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

## Code and experiments

- Research code belongs under `experiments/<question-id>-<slug>/` until the
  development entry gates are met.
- Each experiment must include a README stating the question, hypothesis, run
  instructions, captured evidence, result, and disposition.
- Do not create a production `src`, `internal`, `api`, `sdk`, or deployment tree
  merely to make progress look like implementation.
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
