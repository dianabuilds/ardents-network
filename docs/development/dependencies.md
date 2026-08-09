# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security signals, alternatives considered, and removal plan.

## Current runtime dependencies

None. The project currently uses the Go standard library only.

## Development tools

| Tool | Version | Purpose |
|---|---:|---|
| Go | 1.26.x; CI and Carrier Lab pin 1.26.5 | compiler, formatter, tests, vet |
| Staticcheck | 2025.1.1 | additional correctness analysis |
| govulncheck | v1.1.4 | reachable Go vulnerability analysis |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.

## Blocked Carrier Lab tool inputs

R-013 requires `tc netem` plus packet capture for the native C-5/C2 condition,
but no version, immutable artifact, digest, or reviewed license/maintenance
record has been selected. They are not dependencies yet and must not be fetched
with `apt`, a mutable image, or runtime download. R-025 must close this supply
decision before iteration 6 resumes.
