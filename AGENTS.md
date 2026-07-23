# Repository agent instructions

- Never place Go build or module caches inside this repository. In particular,
  do not create `.cache/go-build`, `.gocache`, or `.tmp-go-cache*`.
- Use the configured external `GOCACHE`. If a sandbox cannot write it, set
  `GOCACHE` to a task-specific directory below the system temporary directory,
  never below the workspace.
- Temporary generated files must be removed when their command finishes. Keep
  the Git worktree limited to source, generated contract outputs, tests, and
  documentation that belong to the repository. Tagged test binaries under
  `tests/.artifacts/testbin` are temporary and must be removed on both success
  and failure; retained JSON/JUnit/coverage evidence is not temporary.
- Keep the external Go cache for normal incremental test runs. Run
  `scripts/clean-go-cache.ps1` only after a release gate, when the cache exceeds
  5 GiB, when disk space is low, or while diagnosing suspected stale build
  output. `scripts/clean-go-cache.ps1 -StatusOnly` reports its size; the normal
  cleanup command retains caches at or below 5 GiB, and `-Force` is reserved
  for the other listed cases. Do not clear it after every unit test.
- Use disposable Docker containers for clean release/CI verification when
  required. Remember that `--rm` removes containers, not Docker image or
  BuildKit caches. `tests/run.ps1 -EphemeralCache` also disposes its anonymous
  Go cache volumes. For normal cached runs, report/bound the two Ardents cache
  volumes with `scripts/clean-docker-cache.ps1`; never run a broad Docker prune
  automatically.
