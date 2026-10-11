# Architecture reconstruction: dev-merge qualification receipt

Status: **runnable gate battery green on the frozen candidate; installed
Ubuntu-only profiles remain boundary evidence, not executed here**

This receipt records the qualification evidence for merging the completed
architecture reconstruction (`refactor/architecture`) into `dev`. It is a gate
and boundary record, not release Qualification, a platform or deployment
result, or permission to start protected text operation.

## Identities

- Campaign: `architecture-reconstruction-dev-merge-qualification`
- Candidate branch: `refactor/architecture`
- Candidate commit: `483684371cad9137ab1d84e7ddac81f4d69edf37`
- Candidate tree: `ad6399e801c753ee6c9aa833285f38845f52943a`
- Integration base (`origin/dev` before merge):
  `c10d59dcfe9914be19bd5dd9a6602ffa7636317a`
- Cumulative base-to-candidate diff: 156 commits, 1079 files changed,
  26,521 insertions, 50,444 deletions (deletion-heavy by design: the
  reconstruction retired legacy subsystems under the product owner's standing
  decisions that legacy dies and no backward compatibility is owed).
- Evidence date: `2026-09-27`
- Execution hosts: the Product Owner Windows host (native Windows gates) and
  Docker Desktop `golang:1.26` Debian containers with `GOTOOLCHAIN=auto`
  (linux gates, including `-race`; the Windows host's ThreadSanitizer is
  machine-wide broken with allocate error 87, so all race evidence is linux).
- Parallel agent branch `codex/architecture-main-work` tip `d705580b` is
  merged and an ancestor of the candidate; the branches are converged.

## Gate battery executed on the candidate

Windows host (native):

- `make quick-check` (format-check/architecture registry, vet, unit
  `-short -shuffle=on`, build, mod-check, artifact-representation-check):
  green, re-run by the pre-commit hook at the candidate commit.
- `go test ./...` full Windows suite: green (exit 0).
- `make e2e` (all seven process packages incl. `tests/e2e/endpoint`): green.
- `make staticcheck`: green. `make deadcode` (exact bidirectional
  windows-amd64 + linux-amd64 allowlist compare, zero test-only functions):
  green. `make vuln`: 0 called vulnerabilities.

Docker `golang:1.26` linux (race detector):

- `go vet ./...`: green. Production builds of all seven maintained commands:
  green.
- Unit suite `go test -p 1 <deterministic-packages> -short -race
  -shuffle=on -count=1` with `umask 077`: 48 of the 52 deterministic
  packages green; the four failing packages fail only in the environmental
  tests catalogued below. The race detector reported zero data races.
- Process e2e packages `go test -p 1 <process-packages> -race -shuffle=on
  -count=1`: six of seven green; `tests/e2e/endpoint` fails only in the
  known environmental test below. Zero data races.
- `fixture-network-test`
  (`tests/qualification/stream-network-two-host/fixturecommand/qualification-network`):
  green (1.3s).
- `package-e2e` profile with `-tags packagee2e -race`
  (`TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes`, full dpkg
  install/upgrade/remove/purge cycle over the v3 enrollment bundle, run as
  container root): green (13.7s).

## Known environmental exclusions

Seven test failures reproduce in the bare `golang:1.26` Debian container and
are environmental, not candidate-caused. Every one self-declares the invalid
environment in its failure output (root bypasses the permission fixture, an
unprivileged linux account is required, `pwsh` is absent, or the fresh
Endpoint XDG root never appears); none is a race-detector or logic failure.
The uncommitted delta between the tested tree and the candidate commit was
docs-only, so every Go package under test was byte-identical to the
candidate; two of the seven were additionally stash-baselined on clean
ancestor commits at the time of discovery, as noted:

1. `internal/enrollment` `TestVerifyRejectsCallerOwnedPackageStaticRoot` —
   the container runs as root, so a root-owned package static root is
   indistinguishable from the caller-owned rejection fixture
   (stash-baselined on clean `133b7de7`). Requires non-root linux execution
   (Ubuntu host).
2. `internal/endpoint/replacement`
   `TestReplaceRetriesNoUpdateAfterStagingDirectorySyncFailure` — the test
   self-refuses: "staging retry requires a clean unprivileged Linux
   account; root bypasses chmod(0300)".
3. `internal/endpoint/replacement`
   `TestReplaceDoesNotRunCandidateBeforeSuccessfulSelfTestWhenActivationDirectorySyncFails`
   — same self-declared root-bypass guard.
4. `internal/service/publication`
   `TestUnpublishRetryFinishesFailedGenerationRemoval` — self-refuses:
   "requires an unprivileged Linux process".
5. `internal/architecture`
   `TestQualificationPreparationReadsCanonicalInstantsBeforePowerShellConversion`
   — execs `pwsh`, which the Debian container lacks (dependency introduced
   with `57237288`).
6. `internal/architecture`
   `TestQualificationWaitsBeforeConsumingAClosingAdmissionWindow` — same
   missing `pwsh` dependency.
7. `tests/e2e/endpoint`
   `TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree` — the fresh
   Endpoint XDG config root never appears in the bare container and the
   endpoint.sock attachment remains (stash-baselined on clean `29e2f481`).

These need the issue-#60 pipeline image or the Ubuntu host; they are recorded
here rather than silently skipped.

## Ubuntu/systemd boundary (not executed in this run)

The installed qualification lanes whose `prerequisites` name a Linux host
with systemd, passwordless sudo, real unit management, or netem
(`text-worker-*`, `text-command-network`, `qualification-endpoint-*-ubuntu`,
`text-worker-policy`, and the installed system-unit successor evidence for
c0 step 6) were NOT executed on the Windows host or in bare containers. The
runnable subset of their mechanics that is container-safe (dpkg package
shape, static enrollment root ownership, setpriv-dropped `enroll-installed`
lifecycle over the sole v3 descriptor grammar) is covered by the green
`package-e2e` container run above. Full installed qualification remains a
separately recorded Ubuntu-host obligation before any release Qualification
claim; this receipt does not widen that boundary.

## Merge record

- `origin/dev` at `c10d59dcfe9914be19bd5dd9a6602ffa7636317a` was verified an
  ancestor of the candidate, so the merge is a fast-forward:
  `git push origin refactor/architecture:dev`.
- Executed under the product owner's standing decision to merge into `dev`
  when the reconstruction is finished; `main` stays closed until full
  stabilization.
- The pushed `dev` tip is the commit containing this receipt — a docs-only
  child of the qualified candidate
  `483684371cad9137ab1d84e7ddac81f4d69edf37` (this file plus its
  `repository-file-map.csv` row); no Go code differs from the qualified
  candidate.
