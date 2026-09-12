# Testing

Ardents separates deterministic Module checks, local process behavior,
artifact profiles, and explicitly selected qualifications. A historical result
is evidence about its exact candidate; it is not an entrypoint for current
code.

## Qualification evidence

A Qualification Evidence Bundle is an immutable, content-addressed record for
one exact candidate and its declared conditions. It retains precommitted
inputs, complete raw observations, invalidations, and deterministic verdict
outputs so the result can be recomputed. A selected log excerpt or ordinary
test report is insufficient. Public retention and change-impact requirements
remain NET-14AJ and NET-14AK in the
[requirements registry](../product/functional-map.md#accepted-requirements-registry).
An internal audit uses the separate [audit method](deep-audit.md) and cannot
substitute for independent review.

## Ordinary checks

- `make unit` runs the positive deterministic package inventory.
- `make e2e` runs the positive local process package inventory.
- `make quick-check` runs formatting, architecture, vet, unit, the four named
  command builds, module tidiness, and the canonical artifact representation
  proof.
- `make headless-check` builds the exact Network command inventory, checks the
  enrollment-v3 artifact, runs bounded Endpoint, Source, Node, and Service
  process evidence, then rebuilds and tests the headless command candidate in
  a fresh temporary tree containing no Browser/Application implementation. It
  also proves that canonical command bytes are unchanged when the same owned
  source is represented as a Git repository or as a VCS-free extraction.
- `make check` runs unit, process, race, command build, formatting,
  Staticcheck, and vulnerability checks. It is the pre-integration gate.
- `make text-role-durable-state-capture` runs the Linux-only isolated Role
  publication/lookup/withdrawal observation. It requires
  `ARDENTS_TEXT_ROLE_OBSERVATIONS` to name a writable capture directory outside
  Git; the test writes secret-bearing raw observations there and retains them
  locally. A non-Linux host or missing capture directory is an invalid profile,
  not a passing skip.
- `make fuzz` mutation-fuzzes the selected State and Contributor targets for a
  bounded 30 seconds each. State owns canonical Epoch/Node Record framing and
  checks successful parser digest/key/raw invariants; Contributor owns strict
  JSON decoding and checks its successful round trip. The command fails if
  either named target is absent or fails, and does not claim coverage of every
  untrusted grammar. The deterministic architecture profile independently
  checks the selected inventory, its declarations, the Make entry point, and
  the exact mutation commands. It does not mutation-fuzz under an ordinary
  deterministic profile.

Ordinary checks do not build or run Docker and never install tools implicitly.
`make check` also verifies pinned tool versions and the dead-code inventory.
Its vulnerability check uses the [Go vulnerability database](https://go.dev/doc/security/vuln/)
and requires network access unless the needed data is already cached. On Linux,
it additionally runs `package-e2e`, which uses root to exercise `dpkg` and
`setpriv`; missing privilege invalidates that selected profile. Dedicated
Rendezvous qualifications have their own targets and prerequisites. Local
setup and hook installation belong in [CONTRIBUTING.md](../../CONTRIBUTING.md#local-setup).

## Dependency security evidence

The [dependency acceptance rule](dependencies.md#maintenance-and-vulnerability-acceptance)
covers current and future components and their continued support.
`make vuln` currently runs `govulncheck ./...` for the selected environment;
it does not by itself cover every target, tests, build tool or delivered
artifact. State exactly which of those were inspected. Review source and
test closures for the applicable `GOOS`, `GOARCH`, `CGO_ENABLED`, build tags
and Go version; use `-test` when assessing test dependencies. Inspect the exact
built artifact and its package/tool/environment inventory when qualifying
delivery. Cross-target source analysis does not execute that target's tests
or qualify its artifacts.

Record the full finding classification, including imported-package and
module-only findings, and the dated non-applicability evidence required by the
register. The pinned scanner's
[official documentation](https://pkg.go.dev/golang.org/x/vuln@v1.1.4/cmd/govulncheck)
(accessed 2026-09-07) describes build-specific coverage and limitations around
reflection, `unsafe` and binary call information. Structured output modes can
return success despite findings, so exit status alone is not a verdict for
such reports. Source checks, exact-artifact inspection, primary advisory review
and the relevant behavior tests provide complementary evidence; none proves
the absence of unknown vulnerabilities or continued upstream maintenance.
Required `make check` gates remain binding.

The successor [privacy/anonymity map](privacy-anonymity-map.md#verification-and-test-environment-map)
defines additional evidence obligations for a selected new scheme. These are
design and qualification requirements; no new execution profile or automated
anonymity verdict exists merely because the map names them.

The selected installed text Endpoint and its context, token-journal, protected
publication and read component tests compile on Linux. Run those behavior
checks in the existing Linux deterministic/race profiles; a Windows unit pass
does not execute or qualify them. Windows retains the pre-existing Endpoint
journey and an explicit unsupported text-command refusal. Shared State,
Custody, Node and protocol consumers keep their own platform contracts. The
Endpoint-originating Route bootstrap/prefix client and its stream, credit,
JOIN and network-issuance tests also execute in the Linux profiles. Receiving
Node checks that do not require that client remain on both platforms. Codec
round trips, private-capsule cryptography and network tests that construct
Endpoint operations execute with the Linux client. Shared outer-handshake
admission and receiving-listener address/certificate fixtures remain separately
available to the platform-independent Node tests.
## Reachability audit

`make deadcode` runs `golang.org/x/tools/cmd/deadcode` for the maintained
Windows and Linux production builds, then for their test executables. The test
result must be empty. Production results must match
[`tests/profiles/deadcode-allowlist.json`](../../tests/profiles/deadcode-allowlist.json)
exactly: an entry names every retained symbol, its classification, rationale,
owner, and deletion condition. Prefix, package, or count-based exceptions are
not permitted.

Most retained production entries are intentionally unwired closed-alpha
tracers with deterministic behavior evidence; this is not a claim that they
are a selected product path. A newly reported symbol fails the gate until its
owner either removes it or reviews it with a concrete retirement condition.
An absent listed symbol also fails the gate, so the registry cannot silently
accumulate stale exemptions. `make check` includes this audit.

The headless command inventory declares the four-command Network artifact lane;
the text-worker inventory declares its separately owned Application lane under
`tests/profiles/`. Architecture tests check actual transitive dependency
graphs, the explicit [`ownership.json`](ownership.json) registry, exact
qualification/artifact-lane ownership, and every maintained package and suite.

Canonical command builds use the non-overridable
`-trimpath -buildvcs=false` policy. The public Make proof builds all four current
headless artifacts from two independent normal clones, a linked worktree, and
two independent VCS-free ownership extractions,
including the Endpoint source dependencies owned by both Application Interfaces
and text-application without changing their ownership or the artifact lane,
then requires byte-identical
outputs without implicit `vcs.*` settings. Source revision and builder
provenance remain explicit authenticated release-metadata,
attestation, and receipt inputs; command bytes never infer those facts from the
builder's clone or linked-worktree representation.

## Preliminary Gate A1 regression coverage

The current deterministic and race inventories retain the accepted preliminary
Network authority/lifecycle repairs:

- the alpha-corpus diagnostic cannot accept or create an Endpoint floor, while
  the separately enrolled acceptance command remains the sole floor mutator;
- the enrollment package, Windows behavior tests, and both alpha-bundle scripts
  share the canonical `.exe` control-artifact identity;
- revoked, draining, or closed Connection admission reaches no State, Entry,
  issuer, Route, or Introduction work; pending capability expiry does not end
  active Endpoint work; Connection and Administration budgets are separate;
  and active sessions are canceled, drained only under a permitted finite
  non-extendable bound, and released once;
- Node rejects old-generation Entry and Transit Grant admissions immediately
  after a successor and reports terminal cleanup faults without publishing
  `WITHDRAWN`; and
- concurrent caller and Route Attachment close join one exactly-once cleanup
  and return the same terminal result; and
- Entry close cancels acquisition, joins and terminalizes active attachment
  cleanup before releasing its root, including concurrent and failure cases.

The architecture suite additionally prevents floor authority from returning to
`inspect-alpha-corpus`. These are ongoing regression checks for the repaired
candidate, not completion of the formal deep audit or authorization to begin
its later security, concurrency, or wire tracks.

## Current profiles

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Every active profile has one real Make entrypoint and exact
prerequisites. Missing Docker, binaries, privilege, platform, host input, or
artifact is an invalid environment, never a skip or passing result.

The maintained local profiles are:

- developer, deterministic, process, package-e2e, headless-network, race, and fuzz;
- `qualification`, the aggregate selected Ubuntu Endpoint lifecycle profile;
- `endpoint-portable-ubuntu` and `endpoint-replacement-ubuntu`;
- `service-credential-response-linux`;
- `native-rendezvous-multihost`;
- `text-role-durable-state-capture`, the Linux-only isolated Role and Publisher
  durable-state observation; it retains secret-bearing raw observations only in
  an existing, writable, non-symlink directory outside the Git worktree;
- `alpha-control-two-endpoints`;
- `text-worker-network`, the installed confined worker/network composition invoked
  by `make text-worker-network-check`; requires all eight document/Carrier and elapsed-refresh cases,
  exact invocation evidence and terminal service success. State and authority setup
  remain explicit fixtures; ordinary command and full host qualification are separate.
- `text-command-network`, the separately pinned installed ordinary-command journey
  invoked by `make text-command-network-check`; it exercises the real `ardents`,
  `ardents-custody`, `ardents-node` and `ardents-text` candidate binaries through
  both Carriers, with exact empty, 64 KiB and 4 MiB command cases. It is functional
  journey evidence, not full host, privacy, hostile-network or p95 qualification.
- `text-worker-policy`, the root-driven installed authorization matrix invoked
  by `make text-worker-policy-check`; it does not qualify the complete host.
- `text-worker-lifecycle`, the separately selected installed Endpoint launch/Grant/
  cgroup profile invoked by `make text-worker-lifecycle-check`; its tagged test binary
  and exact temporary unit must be independently pinned. No-tests success is
  refused; this profile does not replace hostile-worker or Service journey tests.
- `text-worker-escape`, the separately pinned installed P6/P7 escape matrix invoked
  by `make text-worker-escape-check`; it tests hostile worker access attempts under
  the effective selected unit policy and does not establish whole-host qualification.

Profile, target, directory, build-tag, environment-variable, and test names use
domain language. Historical wire, persisted, release, and artifact identities
remain unchanged where their exact bytes are a compatibility obligation.

There is no generic live or soak profile. Selecting VPS, hostile-load, soak,
platform-matrix, release, or Firefox runtime qualification requires a new exact
product claim, environment, fixture, and Product Owner decision.

## Candidate boundaries

The C0 Network candidate is exercised by the deterministic/process/race lanes,
`headless-check`, and the selected Network qualifications. This includes the
Endpoint-owned `internal/application/broker`; its directory is not a separate
Application product. The neutral Application Interface v1 remains covered by
its conformance vectors and the Endpoint's shared seam. No Browser command,
Browser implementation, Browser artifact, or Browser qualification is part of
the current candidate.

ADR-0067 retires the completed release-seed and fixed State-genesis ceremony
commands, their deterministic writers, and their separate artifact/process
profile. R-119 through R-121 retain the exact historical result; current test
inventories contain no compatibility exception or hidden replacement route for
those writers.

`tests/compatibility/browser-endpoint-v4` is the sole retained non-executable
source exception. ADR-0061 requires it to remain outside Go package discovery,
ordinary checks, and current qualification until an explicit supersede or
retirement decision. Completed experiments and the former `reference-c2`
fixture are available only from Git history and accepted research records.

## Historical qualification provenance

Only the concise verdicts below remain in the current documentation surface.
Detailed runners, process chronology, and split-candidate ledgers are historical
Git material.

| Exact source | Retained result | Limitation |
|---|---|---|
| [`70bf425eec937edcc22e8f0534db992aa2002a16`](https://github.com/dianabuilds/ardents-network/commit/70bf425eec937edcc22e8f0534db992aa2002a16) | Historical RC1 supplied A1-A10 bounded project-operated evidence. | It has no A11/A12 closure and does not qualify C0. |
| [`2c18bdf92f11f84075915576f595202f48eb05bc`](https://github.com/dianabuilds/ardents-network/commit/2c18bdf92f11f84075915576f595202f48eb05bc) | Historical RC2 supplied a separate two-fresh-Endpoint control result and accepted A11 campaign. | It does not inherit RC1 A1-A10; no candidate has an aggregate A1-A12 result. |
| [`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf) | Last source snapshot containing the retired experiment implementations, `reference-c2`, generic Update tracer, stage runners, and planning chronology. | Provenance only; none of those sources qualifies or belongs to the cleaned C0 candidate. |

## Ownership

Unit and single-Module integration tests live beside their implementation.
They replace real time or external I/O only at the owning seam and do not start
Docker. Cross-process tests live under `tests/e2e/<behavior>/`; they build real
named commands, observe public behavior, create fresh temporary fixtures, and
clean every owned process and file.

Every maintained Go package belongs to the deterministic inventory. Every
maintained repository file—including source/test, command, build and CI input,
documentation, packaging, profile, qualification, and retained evidence—plus
every qualification and artifact lane matches exactly one ownership rule.
Environment-owned `.codex-*`, `.git`, and editor state are not repository
inputs and are excluded from the walk. Every Go-bearing `tests/e2e` suite root
belongs to exactly one process profile. A new package, file, or suite cannot
enter through a negative filter, wildcard exception, or directory naming
alone.

`tests/compatibility/` is non-executable provenance. Compatibility evidence
must name its former observer and deletion/reactivation condition and does not
belong to a maintained package inventory.

## Validity and reruns

An environment-dependent run has four possible outcomes: product assertion
failure, harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved.

Rerunning is diagnostic evidence and never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
timeout must leave enough observed state to distinguish product, harness, and
environment.

## Test design and retirement

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps are not readiness
or eventual-consistency oracles. Every goroutine, process, listener, file, lock,
timer, and fixture has one owner, cancellation path, join, residue assertion,
and cleanup-failure path.

Two tests are duplicate-removal candidates only when requirement, owning seam,
oracle, transition/fault, platform/format, and independence role are all the
same. A seam migration moves behavior tests to the new Module Interface,
retains only higher-level tests that prove a distinct fact, and retires obsolete
fixtures and runners in the same change.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs beside each retained untrusted
decoder and canonical encoder. Race coverage is required wherever a Module owns
goroutines, callbacks, locks, mutable admission, cancellation, or durable state,
and must assert terminal join and state invariants in addition to race-detector
cleanliness.

The closed text Endpoint's durable Entry Set owner and its persistence and
class-specific adjacency tests compile on Linux, matching their sole production
consumer. Legacy Entry Invite and receiving-admission checks remain portable.

Holder-request construction and its end-to-end Custody allocation tests run on
Linux with the text Endpoint client. The shared request decoder, proof verifier,
Custody command and authority storage remain portable; independent authority
creation and allocation-journal tests stay in the portable profile.

The closed blind-token client batch and result round-trip tests run on Linux
with the Endpoint that retains the opaque pending state. Issuer request parsing,
signature checks and fixed result encoding remain available to receiving Nodes.

Private Descriptor issuance round trips and Store tests that produce those
proofs run on Linux with the Publisher. Independent receiving Gateway and wire
checks remain portable. Protected context and coalesced initial-authentication
tests run with the Linux text Endpoint; shared stream lifecycle tests remain
portable, including the common receipt-consumption representation.

Snapshot construction/response and published-Link client round trips execute on
Linux with their text command consumer. Shared document-read and raw published-Link
server framing tests remain portable; their failure checks are unchanged.

AAI3 server admission, lifecycle, cleanup and request round trips execute on
Linux with the Endpoint. Portable client cancellation/close tests retain their
independent socket peer and explicit terminal-frame fixture; they do not require
the selected Linux server to produce a response.
