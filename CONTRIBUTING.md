# Contributing

Ardents Network is maintained by the Product Owner and Codex. Every change
must leave the project buildable and its product boundary honest.

## Change workflow

1. Read [AGENTS.md](AGENTS.md), [product scope](docs/product/scope.md), the
   [threat model](docs/security/threat-model.md), and the affected current
   technical and development owners. Follow the repository authority order:
   accepted ADRs precede the product contract and threat model. Consult ADRs
   through current-owner links, and closed research or historical material
   only for named provenance.
2. Confirm that a current product or technical contract authorizes the behavior.
   A consequential, hard-to-reverse selection also needs an accepted ADR.
   Experiments, historical campaigns, and familiar libraries cannot select a
   protocol, dependency, or product runtime by themselves.
3. Implement the smallest vertical behavior in an existing deep module. A new
   package needs a cohesive responsibility, maintained implementation, behavior
   tests, a real non-test caller, and its exact permitted imports in the
   [package map](docs/development/package-map.md). Follow the
   [Go and architecture rules](docs/development/repository-layout.md).
   Review runtime dependencies in the
   [dependency register](docs/development/dependencies.md) before changing
   `go.mod`.
4. Add or update behavioral and failure-path tests with code. Run
   `make quick-check` while writing and `make check` before integration. The
   [testing model](docs/development/testing.md) owns the commands, selected
   environments, and qualification boundaries. Missing selected prerequisites
   invalidate a profile; they are not passing skips.
5. Promote changed behavior to its current documentation owner and review
   `git diff --check`. Keep caches, generated evidence, credentials, binaries,
   and unrelated edits out of the change.

An experiment starts with an active question in the
[research queue](docs/research/questions.md), a falsifiable hypothesis,
declared inputs and environment, an evidence plan, and a disposal condition.
Follow the [research workflow](docs/research/README.md#workflow); an experiment
creates no maintained dependency, protocol, or compatibility promise.

A release, security, privacy, availability, or platform claim needs the exact
conditions and measurements in the threat model. Local tests and a
project-controlled host provide implementation evidence, not independent
qualification. C0 work selection and work-in-progress limits remain in
[AGENTS.md](AGENTS.md#c0-work-in-progress-limit).

## Local setup

Use the repository's pinned Go toolchain and explicitly run `make tools-install`
when the pinned quality tools are missing. Checks never install tools
implicitly. Run `bash ./scripts/install-git-hooks.sh` once per clone to enable
the local pre-commit gate; CI independently runs `make check`. The hook is not
the security boundary.

Keep dependency caches, build outputs, and test evidence outside the repository.
The Makefile configures the quality cache under the system temporary directory;
`QUALITY_CACHE_ROOT` may name another external location.
