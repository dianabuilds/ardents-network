# PW3-11: AD-05 Lifecycle and Qualification Evidence

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R3 qualification

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/application-discovery.md`, slice
`AD-05 — Lifecycle and qualification evidence`.

## User story

As a release integrator, I want Application Discovery lifecycle convergence
and protected-socket evidence bound to an exact clean commit so that the
capability catalogue can distinguish implemented, reachable, operable, and
qualified truth without borrowing evidence from Operator Discovery.

## Full vertical behavior

1. resolve a trusted imported `NetworkPublished` service record through the
   admitted Application Discovery interface;
2. prove imported withdrawal, trust-registry replacement, and route-policy
   reload affect the next Resolve without a restart or request-triggered
   refresh;
3. exercise enrollment, authentication, grant issuance, and Discovery Resolve
   through the protected Linux Application Unix socket without using Operator
   custody in the Discovery Application identity/session;
4. preserve the AD-03 projection/privacy contract and AD-04 authority contract;
5. update architecture acceptance and the capability/evidence catalogue only
   from retained current-head evidence;
6. bind qualification to one exact clean implementation commit and retain
   commands, environments, attempts, timestamps, and artifact hashes.

## Confirmed TDD seams

- admitted Discovery RPC over real maintained Discovery truth;
- public Go SDK through the protected Application Unix socket;
- architecture-acceptance and capability-catalogue tooling.

Confirmed by the user on 2026-07-26 before the first test edit.

## In scope

- trusted imported-record integration;
- imported withdrawal convergence;
- current trust-registry replacement convergence;
- current route-policy reload convergence;
- protected-socket Linux Application Discovery E2E;
- API generation and architecture acceptance;
- capability/evidence catalogue and immutable qualification snapshot;
- clean matching-commit Docker/Linux/repository evidence for this capability.

## Out of scope

- new actions, resources, RPCs, SDK methods, or wire messages;
- projection, ordering, deduplication, endpoint, trust, or route-policy
  semantics changes unless a test proves an AD-05 blocker;
- Delegation changes beyond AD-04 regression evidence;
- remote Application transport, endpoint dialing, TLS/service authentication,
  Hosting, Messaging, or a Direct Service adapter;
- persistence, cache, queue, worker, or background-refresh additions;
- qualification of any capability other than `application.discovery`;
- blanket release or production-readiness claims.

## Authority, state, bounds, and restart

- Action remains `application.discovery.resolve`.
- Resource remains ownerless `service-type`.
- Application and Operator interfaces remain separate.
- `identity/access` remains the authority state owner.
- Discovery, trust, and policy owners remain authoritative for maintained
  truth and current projection inputs.
- Existing action, artifact, lifetime, header, unary, projection-work,
  target-count, and one-hop bounds remain unchanged.
- Reload convergence must be visible on the next call without adding cache
  invalidation, replay, compensation, or restart-only state.
- Restart must reload the same durable identity/access and Discovery truth;
  AD-05 adds evidence, not a second lifecycle owner.

## Acceptance criteria

- [x] A trusted imported current `NetworkPublished` record resolves through
  the admitted Application interface.
- [x] An imported withdrawal removes the target on the next Resolve.
- [x] Removing and restoring `discovery.publish` trust affects the next
  Resolve.
- [x] Denying and restoring the endpoint scheme through current route policy
  affects the next Resolve.
- [x] Lifecycle misses remain privacy-uniform `NotFound`.
- [x] Lifecycle changes trigger no request-time observation, refresh, probe,
  fetch, or dial.
- [x] Linux E2E enrolls an Application, authenticates it, and resolves through
  the protected Unix socket using the public SDK.
- [x] The Discovery Application identity/session never uses Operator
  credentials; a separate adversarial assertion proves an Operator credential
  cannot authenticate or authorize on the Application interface.
- [x] Generated API check and architecture acceptance pass.
- [ ] Capability catalogue truth is updated only for
  `application.discovery`, with complete retained gate evidence.
- [ ] Qualification evidence is bound to one exact clean implementation
  commit; retries never replace first-attempt evidence.
- [x] AD-03 projection/privacy, AD-04 authority, Content compatibility, and
  Application/Operator separation remain unchanged.
- [x] No out-of-scope product surface is introduced.

## Required evidence

- focused integration and E2E red/green runs;
- negative locator-call/external-work assertions where applicable;
- full `go test ./... -count=1`;
- integration and E2E tagged suites selected by scenario;
- scoped `go vet`;
- `scripts/generate-api.ps1 -Check`;
- architecture, audit, test-catalogue, and capability-catalogue tooling;
- focused `govulncheck`;
- independent Standards, Spec, and security reviews;
- current-head clean implementation checkpoint;
- retained JSON/JUnit/resource/coverage artifacts with SHA-256 hashes;
- final exact `git status`.

Use external task-specific `GOCACHE`. Temporary tagged test binaries must be
removed on success and failure.

## Commit and governance policy

- Implementation/lifecycle test changes form one logical implementation
  commit.
- Qualification executes against that exact clean commit.
- Capability catalogue, generated evidence register, and immutable
  qualification metadata form a separate qualification commit.
- The canonical PRD, Wave 3 decision register, and aggregate snapshots are not
  edited by this slice unless an explicit integrator decision expands scope.
- No push without a new explicit user request.

## Exit condition

AD-05 exits only if every required gate for `application.discovery` is retained
against one exact clean commit and catalogue validation accepts the resulting
snapshot. Missing Linux/Docker/security/release evidence blocks `Q=yes`; it is
not replaced by local evidence.

## Comments

- Began from clean AD-04 checkpoint
  `069aa058c59db544c932bdf3ca44051094968930` on branch
  `codex/ad-05-lifecycle-qualification`.
- Origin:
  `https://github.com/dianabuilds/ardents-network.git`.
- External Go cache:
  `C:\Users\vitek\AppData\Local\Temp\ardents-ad05-gocache`.
- TDD red evidence:
  - admitted trusted-record test initially failed to compile because no
    admitted client fixture existed in the integration slice
    (`2026-07-26T20:04:38Z..20:06:27Z`);
  - imported-withdrawal test initially failed on invalid fixture issuance
    time (`2026-07-26T20:07:26Z..20:07:31Z`);
  - protected-socket APP-001 reached the intended red boundary when the public
    probe had no `discover` command
    (`2026-07-26T20:22:22Z..20:23:18Z`).
- Focused green evidence:
  - all five admitted lifecycle/security tests pass from the canonical
    `tests/integration/discovery` package;
  - protected-socket APP-001 passed after the probe used the existing public
    Go SDK Discovery method
    (`2026-07-26T20:23:55Z..20:24:54Z`).
- Implementation gates:
  - `go test ./... -count=1` passed
    (`2026-07-26T20:29:57Z..20:30:57Z`);
  - scoped `go vet`, API generation check, all tooling tests, and the current
    capability-catalogue check passed;
  - Docker/Linux scenario `APP-DISC-001` passed with four retained lifecycle
    reports (`2026-07-26T20:32:29Z..20:33:23Z`);
  - Docker/Linux scenario `APP-001` passed with the protected Application
    socket, public SDK Resolve, restart, revocation, Content compatibility,
    and Operator-separation assertions
    (`2026-07-26T20:33:33Z..20:34:31Z`);
  - architecture acceptance required no manifest edit because the existing
    Application listener composition already declares Discovery exactly once.
- Review remediation:
  - Spec review found that the initial trust lifecycle test called
    `ReplaceRegistry` directly while production configuration treated trust as
    restart-required;
  - a production `Node.ReloadConfig` red test failed with
    `restart_required` (`2026-07-26T20:48:20Z..20:48:27Z`);
  - trust is now a transactional reloadable field, the daemon atomically
    replaces configured trust while preserving the local Discovery publisher,
    and the admitted integration test drives the production reload path;
  - Standards review corrected the evidence wording: the positive Discovery
    Application never receives Operator custody, while the existing separate
    adversarial check intentionally presents an Operator credential and proves
    it cannot establish an Application session.
  - Focused security review found that 65 current, signed records from an
    untrusted private/bootstrap publisher could previously enlarge retained
    truth past the AD-03 pre-projection bound and force `Internal` for an
    otherwise resolvable trusted target;
  - focused intake/load tests failed before remediation
    (`2026-07-26T20:59:52Z..20:59:54Z`) and passed after it
    (`2026-07-26T21:00:21Z..21:00:23Z`);
  - private/bootstrap records now require current
    `discovery.publish` trust before durable retention, and restart removes
    legacy untrusted bootstrap records; Operator-imported untrusted records
    remain retained for administrative diagnostics;
  - the exact 65-record Application regression passed at
    `2026-07-26T21:02:30Z..21:02:31Z`, preserving bounded projection and the
    trusted result.
  - final `go test ./... -count=1` passed
    (`2026-07-26T21:07:58Z..21:08:50Z`);
  - final canonical Docker/Linux `APP-DISC-001` passed all five scenarios and
    removed its temporary test binary
    (`2026-07-26T21:09:42Z..21:10:42Z`);
  - final canonical Docker/Linux protected-socket `APP-001` passed and removed
    its temporary test binary
    (`2026-07-26T21:10:53Z..21:11:53Z`).
  - repeat Standards review found that legacy bootstrap compaction ran before
    retained metadata/duplicate validation; the regression was red
    (`2026-07-26T21:16:15Z..21:16:18Z`) and validation now fails closed before
    compacting structurally valid untrusted bootstrap entries;
  - repeat Spec and security reviews found that classifying every `trust.*`
    change as reloadable left the privacy `channel.issue` consumer on its old
    registry; the purpose-aware classification test was red
    (`2026-07-26T21:18:40Z..21:18:42Z`);
  - only a trust change whose complete non-Discovery projection is unchanged
    is now reloadable; channel-issuer and other trust changes remain
    `restart_required`, while Discovery-only trust changes apply to the next
    Resolve. Config, Discovery, and daemon suites passed after remediation
    (`2026-07-26T21:19:09Z..21:19:39Z`).
  - post-remediation full Go, scoped vet, generation, and tooling gates passed;
    canonical Docker/Linux `APP-DISC-001` and `APP-001` also passed again
    (`2026-07-26T21:21:24Z..21:23:19Z`) and removed their temporary binaries.
  - final security revalidation found that records admitted while a
    private/bootstrap publisher was trusted could remain above the AD-03
    pre-projection bound after live revocation until restart;
  - the exact 65-record revocation regression and trust-refresh API were red
    (`2026-07-26T21:33:05Z..21:33:12Z`); Discovery trust reload now
    transactionally re-evaluates and persists retained truth before publishing
    the active generation, compacting only revoked Bootstrap records while
    preserving Operator-imported diagnostics;
  - focused Discovery, config, daemon, and admitted lifecycle tests passed
    after remediation (`2026-07-26T21:34:10Z..21:34:47Z`).
  - final Standards and Spec reviews then found two transaction gaps:
    rollback after a later applier failure did not restore Bootstrap records
    compacted by the candidate trust, and candidate trust became observable
    before compaction persistence succeeded;
  - the rollback regression failed before remediation
    (`2026-07-26T21:44:23Z..21:44:43Z`);
  - Discovery now owns one trust-and-retained-truth transaction: it validates
    with an unpublished candidate evaluator, persists the candidate snapshot,
    then publishes trust and records under the Discovery service lock;
  - persistence failure leaves both current trust and retained truth
    unchanged, while configuration rollback atomically restores the previous
    trust, records, state, and reason in memory and on disk;
  - focused transaction and rollback tests passed
    (`2026-07-26T21:55:41Z..21:55:44Z`), including the separately run
    Discovery race test;
  - post-remediation full Go, scoped vet, API generation, architecture,
    audit-trace, test-catalogue, and capability-catalogue gates passed;
  - final Docker/Linux `APP-DISC-001` passed all five lifecycle/security
    scenarios (`2026-07-26T21:53:56Z..21:53:57Z`), with SHA-256
    `AA3115E449109E5BABADC29E21E968C53867B6FDFB4DCBA837D2659399EF26E0`
    for `summary.json` and
    `FE9E2F3ACAEF53B6C328FF99AD7D515801B1AA2C0134BE6592ECDB24A38563B5`
    for `junit.xml`;
  - final Docker/Linux protected-socket `APP-001` passed
    (`2026-07-26T21:54:20Z..21:55:19Z`), with SHA-256
    `2BF2CC61D3739672B9B13F2EFCF697C12ADD1EC2BD1C8571EFFD23B5FEF46AE4`
    for `summary.json` and
    `FC723EF40D032ECE8B5F267A0D3098B88BAC666AB3E9B8AC26DF9524420620CB`
    for `junit.xml`; both runners removed their temporary tagged test
    binaries.
  - final independent Spec and Standards reviews found that the first rollback
    token was captured in `Prepare`, so an Operator import between `Prepare`
    and a downstream-applier failure could be overwritten, and that
    policy-only reloads unnecessarily rewrote Discovery persistence;
  - the manager commit callback, concurrent Operator import, and policy-only
    persistence regressions were all red
    (`2026-07-26T22:10:50Z..22:10:57Z`);
  - the rollback token is now captured atomically by Discovery only during an
    effective Discovery-trust change; the Discovery lock remains held until
    the configuration manager invokes non-failing `Commit` or `Rollback`;
    concurrent imports therefore wait and apply after the transaction, while
    policy/diagnostics-only reloads neither re-verify nor persist Discovery
    truth;
  - the three regressions and focused Discovery/configuration transaction
    tests passed after remediation
    (`2026-07-26T22:12:14Z..22:12:21Z`), and the concurrent transaction tests
    also passed under the race detector.
- Post-review remediation on `2026-07-27` produced clean implementation
  checkpoint `441545ed6e553325b874530cced73d19e205a93f`:
  - retained Discovery truth rejects a new 65th record, while projection
    budgets count only eligible matching records and endpoints; the confirmed
    trusted-publisher persistent denial path is closed;
  - route-policy deny and restore evidence now uses the production
    `Node.ReloadConfig` path before the next admitted Application `Resolve`;
  - Discovery-purpose trust classification moved from generic configuration
    into daemon composition, and the Application Discovery consumer retains
    ownership of the route-policy interface;
  - full Go, scoped vet, API generation, architecture, audit-trace,
    test-catalogue, capability-catalogue, and focused vulnerability gates
    passed; independent Standards and Spec reviews found no issue;
  - Docker/Linux `APP-DISC-001` passed 5/5 and protected-socket `APP-001`
    passed using disposable caches. Retained hashes and exact commands are in
    `docs/engineering/evidence/application-discovery-441545e.md`.
- The issue remains open and moves to `ready-for-human`: a canonical tagged or
  workflow-dispatched Linux `release-candidate` run is still required before
  a qualification snapshot may set `application.discovery` to `Q=yes`.
