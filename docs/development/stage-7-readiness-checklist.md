# Stage 7 readiness checklist

Status: **stopped by the Product Owner on 2026-08-22.** The earlier coding-start
authorization is historical only; [the Stage 7 stop record](stage-7-stop-record.md)
controls the disposition.
Stage 6 is complete at `b8eb3b6ff8386c2b6166ca3a4ed35b04f75ac3bb`;
the Product Owner accepted the documentation, reduced development coverage,
future qualification gates, ADR-0015, ADR-0016, and ADR-0021. Windows
installation remains separately blocked until an artifact- and operation-
specific Product Owner command.

The checklist distinguishes documentation acceptance, research completion,
coding start, and final Stage 7 disposition. Checking one level does not imply
the next.

## A. Predecessor and joint documentation acceptance

- [x] Stage 6 is complete at clean commit
  `b8eb3b6ff8386c2b6166ca3a4ed35b04f75ac3bb`, and the Product Owner advances
  work to Stage 7.
- [x] R-048 O1 scope and decision order are accepted.
- [x] R-056 O1 topology, first-class direct-binary use, optional browser
  integration, and relatively equivalent Installed/Portable Distribution
  Profiles are accepted; every resulting documentation change is incorporated.
- [x] `horizon-3-stage-7-brief.md` is accepted.
- [x] `stage-7-application-adapter-spec.md` is accepted after the R-056
  development-surface experiment and limitation walkthrough.
- [x] `stage-7-application-principal-spec.md` is accepted after R-051/R-052 and
  the ADR-0016 disposition.
- [x] `stage-7-application-isolation-spec.md` is accepted after R-052 native
  development evidence, explicit deferred native-qualification inventory, and
  the isolated-browser limitation walkthrough.
- [x] `stage-7-authority-custody-spec.md` is accepted after R-053 Docker/current-
  machine evidence and the password/loss/locked-restore walkthrough; deferred
  weakest-supported-host qualification is accepted as a limitation, not pass.
- [x] `stage-7-lifecycle-spec.md` is accepted.
- [x] `stage-7-development-plan.md` is accepted.
- [x] `stage-7-platform-evidence.md` behavior inventory is accepted.
- [x] `stage-7-host-campaign-spec.md` exact 392-episode reference inventory,
  Docker/current-machine observer profile, scheduled/pending/deferred coverage,
  controls, Windows-install authorization stop, and no-replacement rule are
  accepted.
- [x] Every statement and field in `stage-7-joint-review.md` has an explicit
  disposition; blank evidence/ADR fields do not count as acceptance.
- [x] Product Owner confirms the H3 limitation: project-controlled test roots,
  builders, distributors, hosts, and review do not satisfy H4 independence.
- [x] Product Owner explicitly accepts that Docker is not Ubuntu Desktop/native-
  host qualification and the current Windows machine is not a pristine host;
  every unavailable reference episode remains a visible future gate.
- [x] Product Owner records joint Stage 7 documentation acceptance on
  2026-08-20.

## B. S7.0 decisions

- [x] R-049 selects the exact maintained release-verification
  candidate and freezes the TUF-compatible H3 profile, dependency/license/
  advisory closure, conformance vectors, cache/path bounds, and removal gate.
- [x] R-050 selects Ubuntu and Windows Installed lifecycle Adapters from the
  authorized development evidence and freezes
  their stable bootstrap, filesystems, volumes, ACL/modes, durability, crash
  states, repair, uninstall, purge, external tools, and cleanup. It also freezes
  each raw Portable executable/companion target, trusted pre-execution
  verification, package-payload digest equality, stopped replacement/recheck/
  deletion, state separation/locking, and feature/claim parity without inventing
  a Portable lifecycle Adapter. Windows Installed evidence runs only after the
  Product Owner names its artifact and authorized lifecycle mutations.
- [x] R-051 selects the exact
  [Application Principal specification](stage-7-application-principal-spec.md) on
  the available Docker/current-machine surfaces, including inherited-channel
  identity, stable root handle, observable Job/cgroup tree, replay, restart,
  bounds, revocation, cleanup, and an explicit native-qualification deferral
  wherever Docker cannot expose the host fact.
- [x] ADR-0016 authorizes only the minimal fixed-structure Windows
  `unsafe.Pointer` bridge with layout/lifetime/race/failure/cleanup tests; no
  implicit exception remains.
- [x] R-052 selects the exact profiles and explicitly defers unavailable native
  qualification of the frozen
  `ubuntu-bwrap-native-v1` and `windows-appcontainer-native-v1` candidates,
  their privilege/ingress/egress/child/escape/resource/cleanup profiles, and the
  explicit unsupported Stage 7 isolated-browser result.
- [x] R-053 Docker/current-machine evidence accepts the frozen
  `ardents-authority-envelope-v1` O2 candidate, exact Argon2id/AES-GCM profile,
  dependency/resource/password/persistence/restore/reconciliation rules, and
  deletion limitations.
- [x] R-054 accepts the frozen canonical manifest/evidence/verdict/
  cleanup serialization,
  `ardents-h3-stage-7-development-host-campaign-v1` identity, 91-cell/392-
  episode reference inventory, exact scheduled/pending/deferred partition,
  clocks, resources, observer controls, mutation corpus, and independent
  predicates.
- [x] R-056 freezes the direct-binary CLI/input/output contract and optional
  Service Link/browser handoff, browser/origin/profile boundary, normal
  Internet/VPN coexistence, claim ceiling, explicit registration/removal, and
  fallback behavior. Browser absence or failure never disables direct-binary
  use. The retained candidate uses no extension, native host, proxy, or bundled
  browser. Its scheduled Windows/Docker falsification subset remains S7.6
  evidence and cannot represent deferred desktop/native cells as success.
- [x] The release-activation proposal is accepted as ADR-0015 after R-049/R-050.
- [x] The Application-principal/isolation proposal is accepted as ADR-0016 after
  R-051/R-052.
- [x] The R-053 custody/cryptographic decision is accepted as ADR-0021.
- [x] `stage-7-authority-custody-spec.md` and the password-derived custody
  proposal are accepted together.

## C. Architecture and scope contract

- [x] Release Decision, Update Transaction, Install Lifecycle, Authority
  Custody, Application Broker, and Application Isolation have distinct
  responsibilities and narrow Interfaces.
- [x] Package distribution, release authority, installation activation,
  protocol transition, and Authority Custody are not collapsed.
- [x] Each Installed package contains the exact authenticated platform
  executable released as Portable; both expose the same Client/Publisher
  capabilities, Application Interfaces, resources, state compatibility, and
  security/privacy claim ceilings.
- [x] Portable is the executable plus only unavoidable declared companions, not
  a reduced/development edition or a second lifecycle stack. It has no installer,
  stable bootstrap, elevation, or implicit OS integration.
- [x] Portable is authenticated before execution by the Owner or an already
  trusted verifier and rechecked after replacement; no claim relies on untrusted
  executable bytes authenticating themselves after they start.
- [x] Deleting/replacing a Portable executable never deletes or migrates Vaults,
  roots, floors, Grants, Endpoint identity, or network state; concurrent use of
  one state root is rejected.
- [x] Development, H3-test, and public roots/state are disjoint.
- [x] Immutable versioned payload, activation record, copy-on-write migration,
  safe rollback, and repair-required behavior are exact.
- [x] Authority Vault and every non-decreasing security watermark are outside
  payload rollback.
- [x] Generic and isolated Application profiles have different explicit claim
  ceilings and no silent fallback.
- [x] Ardents changes no system DNS, route table, default proxy/browser, VPN, or
  kill-switch policy; blocked Carrier networking fails visibly rather than
  bypassing the active policy.
- [x] Runtime is unprivileged by default; every elevated action, binary,
  operation, path, capability, and cleanup is enumerated and excludes Authority.
- [x] No custom crypto, kernel driver, cgo, experimental-only OS foundation,
  central account, mandatory vendor endpoint, or fake external team is selected;
  first-party `unsafe` is absent unless ADR-0016 accepted only the exact fixed
  Win32 Job-limit bridge and its dedicated risk tests pass.
- [x] Future packages/commands are absent from `package-map.md` until real code,
  tests, exact imports, and callers exist.

## D. Evidence design contract

- [x] Manifest, evidence, private fixture, verifier output, and cleanup roots are
  disjoint and immutable after their owning phase.
- [x] Candidate/runner/command output cannot author or mutate a verdict.
- [x] Independent result semantics are exactly `pass|fail|invalid`.
- [x] Expected fail-closed runtime outcomes are distinguished from verifier
  `invalid`.
- [x] Every cell in the complete A–H inventory has exact required fields,
  runtime outcome, expansion axes, episode count, deadline, resource bounds,
  exact `stage7-<cell>-v1` case ID, precommitted probe order, and predicate; the
  total is exactly 392 episodes.
- [x] Mutation vectors cover missing, unknown-critical, duplicate, reordered,
  stale, rollback, mixed, path-escaping, hash-mismatched, secret-bearing,
  cross-cell, and cleanup-incomplete evidence.
- [x] Privacy claims name protected information, adversary, conditions,
  measurement, and limitation.
- [x] Exact Ubuntu and Windows images, package/supply identities, clocks,
  observers, cleanup inventories, coverage partitions, and any Windows install
  authorization are frozen before results.
- [x] Generated packages, keys, state, VMs, captures, databases, logs, and
  evidence are outside Git.

## E. Coding start rule

Stage 7 maintained coding may start only when A-D are complete, Stage 6 has
advanced, the starting commit is clean and recorded, the Product Owner has
accepted the reduced development coverage and future gates, and records
`start S7.1`.

Current verdict: **ready; Product Owner authorized `start S7.1` on 2026-08-20.**

## F. Implementation and completion evidence

- [ ] S7.1 release-decision behavior and independent evidence pass.
- [x] S7.2 update/rollback/custody-preservation engineering behavior passes;
  independent qualification recomputation is deferred to Stage 9 S9.5.
- [ ] S7.3 Ubuntu 26.04 Docker `.deb` lifecycle smoke plus minimal Portable
  direct-run/state-separation evidence pass; native Ubuntu Desktop lifecycle
  remains explicitly deferred.
- [ ] S7.4 Windows Installed lifecycle plus minimal Portable direct-run,
  stopped-replacement/deletion, state-separation, and parity evidence pass only
  within the Product Owner's explicit installation authorization.
- [ ] S7.5 Application Broker behavior and hostile-sibling evidence pass on both
  platforms.
- [ ] S7.6 direct-binary, optional-browser generic limitation, and
  network-isolation evidence pass for both Distribution Profiles on all
  required development platform pairings.
- [ ] S7.7 scheduled Docker/current-machine development subset recomputes as
  `pass|fail|invalid` correctly, its coverage ledger accounts all 392 reference
  episodes, and the Product Owner accepts every pending/deferred disposition.
- [ ] `make check` passes on the clean committed Stage 7 candidate.
- [ ] Package/dependency maps, external evidence inventory, known limitations,
  and cleanup are factual and complete.
- [x] Product Owner records `stop`: S7.3–S7.7 remain incomplete and are not
  Stage 7 acceptance evidence.
