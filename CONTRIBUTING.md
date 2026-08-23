# Contributing

Ardents Network is currently maintained by the Product Owner and Codex. Every
change must leave the root project buildable and its current product boundary
honest.

1. Read `AGENTS.md`, the accepted ADRs, the product scope, the
   [repository layout](docs/development/repository-layout.md), and the relevant
   research record before changing code.
2. Implement the smallest vertical behavior in an existing deep module. Add a
   package only when a real cohesive boundary exists.
3. Add or update behavioral and failure-path tests with the code.
4. Run `make quick-check` during development and `make check` before integration.
   These gates never build or run Docker. Historical Carrier Lab material is
   not a current qualification flow or contributor prerequisite.
5. Review `git diff --check` and confirm no caches, evidence, credentials,
   binaries, or unrelated edits were included.

The Go workflow and setup commands are in
`docs/development/go-engineering.md`; the package registry is
`docs/development/package-map.md`.
