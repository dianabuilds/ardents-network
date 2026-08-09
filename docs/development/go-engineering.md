# Go engineering rules

These rules apply to every maintained Go change. They are enforced by
`internal/architecture`, the Make targets, the local Git hook, and CI.

## Project shape

[Repository layout and growth rules](repository-layout.md) is the normative
source for top-level zones, Modules, commands, tests, packaging, generated
artifacts, and dependency direction. [The package map](package-map.md) records
only the commands, packages, and project imports that currently exist. The
architecture gate checks both records against the Go tree.

## Code rules

- Keep exported interfaces small; default to unexported implementation details.
- Return errors with actionable context. Do not use `panic` for first-party
  control flow and do not hide errors.
- First-party `unsafe`, cgo, and implicit `init` require a superseding accepted
  ADR and dedicated risk tests.
- Use `gofmt`; package comments are mandatory; command packages remain thin.
- Name each file after one implementation responsibility. Production files
  have a hard limit of 250 lines; every Go file, including
  tests, has a hard limit of 500. Split files without inventing package seams.
- Do not use catch-all filenames such as `model.go`, `support.go`, `types.go`,
  `helpers.go`, `common.go`, `misc.go`, or `util.go`.
- Add tests for behavior and failure paths in the same change.
- Prefer the standard library. A third-party runtime dependency requires an
  entry in `dependencies.md` and a documented review before `go.mod` changes.
- Do not put caches, evidence, generated dependencies, credentials, binaries,
  profiles, or test artifacts in the repository.

## Required workflow

Run `make quick-check` while writing code. It performs structure/format checks,
`go vet`, shuffled tests, a build, and a module-tidiness check without installing
or upgrading anything.

Run `make check` before integration. It additionally verifies exact tool
versions, runs the race detector, Staticcheck, and govulncheck. The vulnerability
check reads the Go vulnerability database, so this full gate requires network
access unless the database is already cached.

Run `make tools-install` explicitly when the pinned development tools are
missing. Run `bash ./scripts/install-git-hooks.sh` once per clone to enable the
local pre-commit gate. CI independently runs `make check`; the hook is not the
security boundary.

## Basis

The layout follows the official Go module-layout guidance. Style starts with
Go's Code Review Comments; automated analysis augments code review rather than
replacing design judgment.

- https://go.dev/doc/modules/layout
- https://go.dev/wiki/CodeReviewComments
- https://go.dev/doc/security/vuln/
