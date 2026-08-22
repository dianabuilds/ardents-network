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
- Name each file after one implementation responsibility. Every Go file,
  including tests, has an interim hard limit of 500 lines. Split files by
  responsibility without inventing package seams. When cohesion is not obvious
  from the file's responsibility, record the local invariant, rejected split,
  and behavior coverage; do not use a soft line-count quota.
- Do not use catch-all filenames such as `model.go`, `support.go`, `types.go`,
  `helpers.go`, `common.go`, `misc.go`, or `util.go`.
- Add tests for behavior and failure paths in the same change.
- Prefer the standard library. A third-party runtime dependency requires an
  entry in `dependencies.md` and a documented review before `go.mod` changes.
- Do not put caches, evidence, generated dependencies, credentials, binaries,
  profiles, or test artifacts in the repository.

## Architecture review signals

Line count, exported-declaration count, broad records, direct clock use, string
outcomes, and normalized duplication are investigation signals rather than
correctness verdicts. A cohesive implementation can legitimately be large; a
small one can still hide forged authority, unowned state, an unsafe format
cutover, unbounded work, or a leaked trust boundary. Review the owning Module's
responsibility, caller knowledge, state/lifecycle writer, failure/cleanup rule,
format observers, and behavior/fault evidence together.

S8.2 replaced the command and internal-export caps with source-bound hard facts:
package-map/import direction, command adaptation without exported product
behavior, package responsibility/comments, dependency/artifact gates, and the
scoped high-risk exception registry. They are not a reason to split one
cohesive invariant into choreographing packages, widen a result record, or
introduce a generic helper. A review records the local invariant, why an
obvious split would add choreography, the real caller/compatibility boundary,
and the tests that cover normal and failure behavior.

## Required workflow

Run `make quick-check` while writing code. It performs structure/format checks,
`go vet`, deterministic unit tests, a build, and a module-tidiness check without
Docker or tool installation.

Run `make check` before integration. It additionally verifies exact tool
versions, runs cross-process end-to-end tests, the unit race detector,
Staticcheck, and govulncheck. Run `make live` separately on a Docker-capable
host when network-container behavior changes. The vulnerability check reads the
Go vulnerability database, so this full gate requires network access unless the
database is already cached.

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
