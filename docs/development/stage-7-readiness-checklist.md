# Stage 7 readiness checklist

Status: **review; all acceptance and technology gates are open. Documentation
may be reviewed and R-049–R-054 experiments may be prepared, but Stage 7
maintained coding is not authorized.**

The checklist distinguishes documentation acceptance, research completion,
coding start, and final Stage 7 disposition. Checking one level does not imply
the next.

## A. Predecessor and joint documentation acceptance

- [ ] Stage 6 has a clean committed candidate and Product Owner `advance`.
- [ ] R-048 O1 scope and decision order are accepted.
- [ ] `horizon-3-stage-7-brief.md` is accepted.
- [ ] `stage-7-lifecycle-spec.md` is accepted.
- [ ] `stage-7-development-plan.md` is accepted.
- [ ] `stage-7-platform-evidence.md` behavior inventory is accepted.
- [ ] Product Owner confirms the H3 limitation: project-controlled test roots,
  builders, distributors, hosts, and review do not satisfy H4 independence.
- [ ] Product Owner records the joint Stage 7 documentation acceptance date.

## B. S7.0 decisions

- [ ] R-049 selects or rejects the exact maintained release-verification
  candidate and freezes the TUF-compatible H3 profile, dependency/license/
  advisory closure, conformance vectors, cache/path bounds, and removal gate.
- [ ] R-050 selects Ubuntu and Windows package/bootstrap/activation Adapters and
  freezes filesystems, volumes, ACL/modes, durability, crash states, repair,
  uninstall, purge, external tools, and cleanup.
- [ ] R-051 selects Ubuntu and Windows local-channel/Application Principal facts
  and freezes peer identity, launch binding, process-tree ownership, replay,
  restart, revocation, and failure behavior.
- [ ] R-052 selects stable Ubuntu and Windows Network-Isolated Application
  Boundary candidates and freezes privilege, ingress/egress/child/escape/
  resource/cleanup profiles.
- [ ] R-053 selects Authority Vault/Recovery Bundle protection and freezes
  format, cryptography/KDF, dependency, size, tamper, test restore,
  reconciliation, locked/export-only, and secret-cleanup behavior.
- [ ] R-054 freezes canonical manifest/evidence/verdict/cleanup serialization,
  profile/campaign identity, clocks, episode counts, resources, mutation corpus,
  platform cells, and independent predicates.
- [ ] The release-activation proposal is promoted to accepted ADR-0015, rejected,
  or superseded after R-049/R-050.
- [ ] The Application-principal proposal is promoted to accepted ADR-0016,
  rejected, or superseded after R-051/R-052.
- [ ] Any consequential R-053 custody/cryptographic decision has an accepted ADR.

## C. Architecture and scope contract

- [ ] Release Decision, Update Transaction, Install Lifecycle, Authority
  Custody, Application Broker, and Application Isolation have distinct
  responsibilities and narrow Interfaces.
- [ ] Package distribution, release authority, installation activation,
  protocol transition, and Authority Custody are not collapsed.
- [ ] Development, H3-test, and public roots/state are disjoint.
- [ ] Immutable versioned payload, activation record, copy-on-write migration,
  safe rollback, and repair-required behavior are exact.
- [ ] Authority Vault and every non-decreasing security watermark are outside
  payload rollback.
- [ ] Generic and isolated Application profiles have different explicit claim
  ceilings and no silent fallback.
- [ ] Runtime is unprivileged by default; every elevated action, binary,
  operation, path, capability, and cleanup is enumerated and excludes Authority.
- [ ] No custom crypto, kernel driver, first-party cgo/unsafe, experimental-only
  OS foundation, central account, mandatory vendor endpoint, or fake external
  team is selected.
- [ ] Future packages/commands are absent from `package-map.md` until real code,
  tests, exact imports, and callers exist.

## D. Evidence design contract

- [ ] Manifest, evidence, private fixture, verifier output, and cleanup roots are
  disjoint and immutable after their owning phase.
- [ ] Candidate/runner/command output cannot author or mutate a verdict.
- [ ] Independent result semantics are exactly `pass|fail|invalid`.
- [ ] Expected fail-closed runtime outcomes are distinguished from verifier
  `invalid`.
- [ ] Every cell in the complete A–H inventory has exact required fields, runtime outcome, platform,
  episode count, deadline, resource bounds, and predicate.
- [ ] Mutation vectors cover missing, unknown-critical, duplicate, reordered,
  stale, rollback, mixed, path-escaping, hash-mismatched, secret-bearing,
  cross-cell, and cleanup-incomplete evidence.
- [ ] Privacy claims name protected information, adversary, conditions,
  measurement, and limitation.
- [ ] Exact Ubuntu and Windows images, package/supply identities, clocks,
  observers, and cleanup inventories are frozen before results.
- [ ] Generated packages, keys, state, VMs, captures, databases, logs, and
  evidence are outside Git.

## E. Coding start rule

Stage 7 maintained coding may start only when A-D are complete, Stage 6 has
advanced, the starting commit is clean and recorded, and the Product Owner
records `start S7.1`.

Current verdict: **not ready**.

## F. Implementation and completion evidence

- [ ] S7.1 release-decision behavior and independent evidence pass.
- [ ] S7.2 update/rollback/custody-preservation behavior and evidence pass.
- [ ] S7.3 Ubuntu install/repair/update/remove/purge behavior and evidence pass.
- [ ] S7.4 Windows install/repair/update/remove/purge behavior and evidence pass.
- [ ] S7.5 Application Broker behavior and hostile-sibling evidence pass on both
  platforms.
- [ ] S7.6 generic limitation and network-isolation evidence pass on all required
  development platform pairings.
- [ ] S7.7 combined clean-package development campaign recomputes as `pass`.
- [ ] `make check` passes on the clean committed Stage 7 candidate.
- [ ] Package/dependency maps, external evidence inventory, known limitations,
  and cleanup are factual and complete.
- [ ] Product Owner records `advance-to-S8`, `redesign`, or `stop`.
