---
status: accepted
date: 2026-08-09
---

# Keep a modular first-party monorepository

Ardents remains one first-party monorepository through at least the Closed Test
Network horizon. Maintained Go code uses the single root module selected by
ADR-0009. The repository may produce several executables and may contain code
for separate runtime trust zones; repository co-location does not merge those
process, privilege, credential, or deployment boundaries.

This shape fits the actual Product Owner plus Codex team: one change can update
a narrow Interface, its callers, evidence, documentation, and quality gates
atomically, while one dependency graph and one integration gate avoid the
versioning, release, and cross-repository coordination cost of a distributed
organization that does not exist. Cohesive deep Modules under `internal`, thin
executable adapters under `cmd`, the factual package registry, and executable
dependency checks provide the modularity.

Carrier Lab and qualification code may depend on proven product Modules.
Product Modules never depend on a harness, laboratory fixture, experiment, or
developer script. An Interface lives with the Module that owns the behavior or
with its caller at the real Seam; a shared abstraction is not created merely to
anticipate a future transport, storage engine, cryptographic suite, or runtime.
The normative growth rules are in
[repository layout](../development/repository-layout.md).

The decision creates no requirement to store secrets, private keys, reusable
credentials, authority material, generated evidence, or deployment state in
the repository. Those values remain outside version control even when the code
that uses them shares this repository.

A part may move to another repository only after the Closed Test Network by a
superseding accepted ADR, and only when a real independently released or
externally consumed Interface, materially different access or disclosure
policy, or independently operated lifecycle makes separation cheaper and safer
than co-location. The extraction must have an owner the actual team can
support, independent build and test gates, a versioned compatibility contract,
and no circular source dependency. Repository size, aesthetic symmetry, a
second binary, or a distinct runtime trust zone is not sufficient by itself.

The costs are a wider CI impact, easier accidental imports, one shared
dependency-upgrade surface, and no path-level confidentiality boundary.
Package registration, dependency-direction tests, scoped commands, external
secret handling, and the common quality gate limit those risks. If those
controls become measurably inadequate, the extraction criteria above provide a
deliberate replacement path rather than an unplanned repository split.
