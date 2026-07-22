# Repository agent instructions

- Never place Go build or module caches inside this repository. In particular,
  do not create `.cache/go-build`, `.gocache`, or `.tmp-go-cache*`.
- Use the configured external `GOCACHE`. If a sandbox cannot write it, set
  `GOCACHE` to a task-specific directory below the system temporary directory,
  never below the workspace.
- Temporary generated files must be removed when their command finishes. Keep
  the Git worktree limited to source, generated contract outputs, tests, and
  documentation that belong to the repository.
- Keep the external Go cache for normal incremental test runs. Run
  `scripts/clean-go-cache.ps1` only after a release gate, when the cache exceeds
  5 GiB, when disk space is low, or while diagnosing suspected stale build
  output. Do not clear it after every unit test.
- Use disposable Docker containers for clean release/CI verification when
  required. Remember that `--rm` removes containers, not Docker image or
  BuildKit caches; never run a broad Docker prune automatically.
