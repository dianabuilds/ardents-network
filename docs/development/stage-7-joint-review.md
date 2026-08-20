# Stage 7 joint review record

Status: **accepted by the Product Owner on 2026-08-20.** S7.0 decisions,
limitations, ADRs, and `start S7.1` are recorded; implementation evidence remains
owned by S7.1–S7.7.

This is the single review/acceptance surface requested for Stage 7. It links to
the normative sources instead of copying their detailed rules. The
[readiness checklist](stage-7-readiness-checklist.md) remains the sole coding-
start gate.

## 1. Already recorded product direction

- [x] R-056 O1 topology was accepted on 2026-08-20: one first-class binary in
  relatively equivalent Installed and Portable Distribution Profiles; browser
  integration is optional.
- [x] Portable means the authenticated platform executable plus only an
  unavoidable declared companion, not a reduced edition or a second lifecycle
  stack.
- [x] Ardents does not change system DNS, routes, default proxy/browser, VPN, or
  kill-switch policy. Blocked Carrier networking remains visibly unavailable.
- [x] Generic default-browser handoff is
  `application-networking-unverified`; every Stage 7 isolated-browser request
  returns `isolation-unsupported` without fallback. Claim-bearing native
  Applications remain in scope.
- [x] Stage 7 development evidence uses Ubuntu 26.04 Docker and the current
  Windows 11 machine. Neither surface is represented as a clean/pristine host,
  and unavailable native qualification remains visible in the coverage ledger.
- [x] No Windows install, repair, update, uninstall, purge, or installer-owned
  registration runs until a separate Product Owner command identifies the
  artifact and authorized mutation scope.

Authoritative record: [R-056](../research/records/r-056-stage-7-desktop-browser-integration.md)
and the [Application Adapter specification](stage-7-application-adapter-spec.md).

## 2. Accepted development decisions and remaining evidence

| Decision | Exact candidate prepared for falsification | Acceptance evidence | Consequence |
|---|---|---|---|
| Stage 7 product slice | R-048 O1: release/deployment, Installed+Portable, Authority Custody, Application Principal/Isolation | Stage 6 complete; documentation and limitations accepted | accepted for S7.1–S7.7 |
| Release verification | R-049 O1: go-tuf/v2 `v2.4.2`, bounded no-cache H3 profile and Ardents-owned floors | conformance/source/dependency/advisory review; repeat at integration | selected for S7.1; no new ADR unless trust semantics change |
| Install/update lifecycle | R-050 O1: Ubuntu `.deb`, Windows WiX v7 MSI, immutable Installed activation, raw Portable target | Ubuntu-Docker smoke, Portable parity, separately authorized Windows cells, accepted durability deferrals | accepted as ADR-0015; scheduled evidence remains |
| Application Principal | R-051 O2: inherited private channel + stable root handle + complete cgroup/Job tree | scheduled hostile/restart/resource/cleanup evidence and accepted native-observation deferrals | accepted as ADR-0016 with one bounded Windows bridge |
| Application Isolation | R-052: bubblewrap v0.11.2 on Ubuntu; ephemeral zero-network AppContainer inside the Job on Windows | scheduled F-cell observer/escape/cleanup evidence and explicit claim ceiling | accepted in ADR-0016; native failure cannot become generic success |
| Authority Custody | R-053 O2: password-derived canonical Argon2id/AES-GCM envelope for bounded Vault and Bundle | scheduled vector/KDF/RSS/interruption/cross-restore evidence and accepted weakest-host deferral | accepted as ADR-0021; falsifier reopens it |
| Evidence/verdict | R-054 S7E1 plus `ardents-h3-stage-7-development-host-campaign-v1` | passing synthetic profile; scheduled observers and exhaustive coverage ledger remain | accepted development profile; deferred is never pass |

Normative sources are [R-048](../research/records/r-048-h3-stage-7-contract.md),
[R-049](../research/records/r-049-stage-7-release-verifier.md),
[R-050](../research/records/r-050-stage-7-install-update-adapters.md),
[R-051](../research/records/r-051-stage-7-application-principal.md),
[R-052](../research/records/r-052-stage-7-application-isolation.md),
[R-053](../research/records/r-053-stage-7-authority-recovery.md),
[R-054](../research/records/r-054-stage-7-evidence-profile.md), and
[R-056](../research/records/r-056-stage-7-desktop-browser-integration.md).

## 3. Required walkthrough statements

The Product Owner accepted every statement below on 2026-08-20 as the Stage 7
development contract. Runtime evidence still must satisfy it:

- [x] Installed and Portable are relatively equivalent product profiles. They
  contain the same platform executable digest and expose the same runtime
  Client/Publisher/Application capabilities, resources, protected-state
  compatibility, and claim ceilings; only package lifecycle convenience differs.
- [x] Portable is authenticated before execution and after stopped replacement;
  direct deletion never deletes or migrates Vaults, roots, floors, Grants,
  Endpoint identity, or network state.
- [x] Generic browser use preserves the browser's ordinary Internet/VPN behavior
  and carries no isolation claim. Stage 7 intentionally has no isolated-browser
  feature and never silently downgrades such a request.
- [x] Native controlled Applications may receive the claim-bearing isolated
  profile only after the complete helper tree passes R-051/R-052 on both hosts.
- [x] Authority Vault and Recovery Bundle use explicit separately entered
  passwords. The Vault is locked after restart; losing a password is
  unrecoverable; unattended root signing, guaranteed zeroization, and secure
  deletion are not claimed.
- [x] A v1 Vault contains at most 1024 independently atomic encrypted records/
  1 GiB under one Vault password and has no in-place password rotation.
- [x] ADR-0016 accepts only the exact bounded Windows Job/AppContainer
  `unsafe.Pointer` bridge and its dedicated layout/lifetime/race/failure/cleanup
  tests. No implicit or general `unsafe` exception exists.
- [x] The 392-episode reference inventory is partitioned before execution into
  `scheduled`, `authorization-pending`, and `environment-deferred`. The
  scheduled subset is conjunctive; failed/invalid evidence is not hidden by a
  successful subset or rerun, and pending/deferred coverage is never a pass.
- [x] H3 roots, builders, distributors, hosts, observers, and review remain
  project-controlled. These results do not satisfy H4 independent custody,
  supply, operation, audit, security review, usability, or public-release gates.

## 4. Documentation acceptance record

This record accepts the S7.0 decision sources. Post-implementation campaign
verdicts remain S7.1–S7.7 completion evidence and are not prerequisites for
starting the code that produces their real artifacts:

```text
Stage 7 documentation disposition: accept
Reviewed maintained-source baseline: b8eb3b6ff8386c2b6166ca3a4ed35b04f75ac3bb
R-049 decision source SHA-256: bb0512fd3f2b51c07f2a8bc1cf526327bec465aea317e8799b4473b615cd2952
R-050 decision source SHA-256: 0e68bb613660dbb934f235e639254af0d6e39ebf52a88373551cf28a4adfd595
R-051 decision source SHA-256: a6a1f76fe68c56e5e7cce7d5ca0a823294afdf3d8b38778aad5a63d82c5e237d
R-052 decision source SHA-256: 582bdb4712d2da175945a7a3039b726025b33e97f6fd985aea456307fb3132de
R-053 decision source SHA-256: b09c8c60bdd38bf9128fce449035a01dca6520137a3e3f42fc7d86da13634b72
R-054 profile source SHA-256: fd3bcbdcf67babd0ba19690ff4fc9b527196590f5f5718da341b6c5aa3b28b5b
ADR-0015 disposition: accepted
ADR-0016 disposition: accepted
Authority Custody ADR: ADR-0021 accepted
Known limitations accepted: yes
Product Owner: current repository Product Owner
Acceptance date: 2026-08-20
```

The hashes bind decision sources, not future candidate verdicts. A later source
change reopens the affected decision or requires a recorded compatible review.

## 5. Separate coding-start record

The separate [S7.0 start record](stage-7-start-record.md) binds the completed
predecessor, checklist, ADRs, and authorization:

```text
Stage 6 disposition: complete; advance to Stage 7
Stage 7 maintained-source baseline: b8eb3b6ff8386c2b6166ca3a4ed35b04f75ac3bb
Readiness checklist SHA-256: 24fcd10aefd67ad1af5381ee6f4465942a083343f91ce83592fe396fce270856
Product Owner authorization: start S7.1
Authorization date: 2026-08-20
```

This authorization starts maintained S7.1 work. It does not authorize Windows
installation or later slices without their declared implementation/evidence
gates.
