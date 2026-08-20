# Horizon 3 Stage 7 implementation brief

Status: **review package; no Stage 7 maintained implementation is authorized.**
R-048, this brief, the lifecycle specification, development plan, readiness
checklist, evidence contract, and two decision proposals await one Product
Owner walkthrough. The
[readiness checklist](stage-7-readiness-checklist.md) is the sole normative
coding-start gate.

Audience: Product Owner and implementation agents after explicit authorization.

## 1. Exact outcome

Stage 7 produces one project-controlled H3 release/install lifecycle on frozen
Ubuntu LTS `x86-64` and Windows 11 `x86-64` hosts. Against the same H3 network,
an Endpoint Owner or Network Contributor can:

1. verify and install a package obtained through an untrusted source;
2. start an unprivileged default Endpoint or remain offline;
3. repair without losing Authority or monotonic security state;
4. retrieve/import, stage, drain, activate, self-test, commit, or safely roll
   back one authenticated update;
5. export and test-restore an Authority Recovery Bundle without granting it
   signing power;
6. attach an Application through either an honestly limited generic profile or
   a launcher-bound Network-Isolated Application Boundary;
7. rebind a Local Grant to a fresh Application Principal after restart; and
8. uninstall normally or separately perform an explicit destructive purge.

A Contributor can additionally stop new assignments by the signed deadline,
drain every active role within its Work Safety Lease, update, rejoin only with
current safety state, or withdraw without reviving old duty.

The result is a controlled H3 mechanism. Test roots, builders, distributors,
hosts, and reviewers remain project-controlled. Stage 7 proves no independent
custody, reproducible public package, public updater privacy, App Store/Ubuntu
repository acceptance, external usability, independent security review, or H4
release suitability.

## 2. Authority and predecessor rules

Authoritative inputs, in order, are accepted ADRs; product contract and threat
model; decided research; experiments; and only then earlier implementation.

Required direct inputs:

- [R-048 Stage 7 contract](../research/records/r-048-h3-stage-7-contract.md);
- [lifecycle specification](stage-7-lifecycle-spec.md);
- [development plan](stage-7-development-plan.md);
- [readiness checklist](stage-7-readiness-checklist.md);
- [evidence contract](stage-7-platform-evidence.md);
- ADR-0006, ADR-0007, the
  [release-activation proposal](stage-7-versioned-release-activation-proposal.md),
  and the
  [Application-principal proposal](stage-7-launcher-bound-application-principals-proposal.md);
- J-00, J-04, J-08, the operating model, threat model, and R-024; and
- the accepted final Stage 6 disposition and clean Stage 6 handoff identity.

Stage 6 progress is a predecessor, not evidence for update or platform behavior.
Stage 7 must consume the maintained Stage 6 destination/name/application paths
through their existing Interfaces and may not rewrite them behind an installer.

## 3. Scope

### Included

- visibly separated development, H3-test, and future-public environment roots;
- TUF-shaped threshold release metadata and a maintained Go verification
  candidate chosen by R-049;
- complete target identity: source, build, dependency, SBOM, platform,
  environment/network, size, digest, qualification status, and safety policy;
- explicit `private-only`, `direct-allowed`, and `offline-import` retrieval modes
  with no silent fallback;
- immutable versioned payloads, finite staging/rollback reserve, bounded drain,
  atomic activation, self-test, commit, safe rollback, and repair-required;
- separate protocol and build state machines and current Work Safety deadlines;
- stable platform bootstrap and thin install/repair/remove Adapters;
- Authority Vault preservation, encrypted Recovery Bundle export, isolated test
  restore, reconciliation, `authority locked`, and export-only behavior;
- Ubuntu local IPC/principal/resource/process ownership;
- Windows local IPC/principal/resource/process ownership;
- generic Application attachment with explicit claim ceiling;
- launcher-bound network isolation for complete client and publisher process
  trees on both platforms;
- grant revocation, restart rebinding, process/helper cleanup, and no reusable
  bearer; and
- independent manifest/evidence/verdict ownership with `pass|fail|invalid`.

### Excluded

- public release keys, public signing ceremony, independent custodians/builders,
  public package repository/store publication, rollout cohorts, telemetry, or
  installation IDs;
- macOS, mobile, other Linux distributions, other CPU architectures, containers
  as a supported desktop platform, or remote administration;
- live process or Service Connection migration during update;
- transparent Application proxying, traffic interception, universal browser
  sandboxing, content safety, malware protection, or endpoint-compromise
  protection;
- custom cryptography, first-party kernel drivers, cgo, `unsafe`, implicit
  `init`, or an unreviewed syscall-filter generator;
- automatic cloud backup, help-desk recovery, invented recovery secrets or
  destinations, and guaranteed secure deletion from snapshots/backups;
- Stage 8 integration, multi-day soak, S9.6 qualification, Route Qualification,
  or H4 claims; and
- package, command, dependency, or Interface placeholders created before a real
  slice owns them.

## 4. Deep Modules and seams

These are logical responsibilities, not permission to create packages. R-049
through R-053 must prove the seam and the first maintained slice must satisfy the
repository package rules before package-map registration.

| Module | Small external Interface | Hidden Implementation | Forbidden authority |
|---|---|---|---|
| Release Decision | Evaluate authenticated release inputs for one exact local platform/environment and return a bounded decision | Metadata roles, thresholds, expiry, rollback/freeze/mix checks, target and attestation validation, persisted release floors | Download, install, activate, sign, or select protocol capacity |
| Update Transaction | Apply one accepted release and return committed, rolled-back, or repair-required | Staging, disk reserve, drain, immutable payload, copy-on-write state, activation, self-test, crash recovery | Release trust, Authority Custody, OS package trust, hidden network retrieval |
| Install Lifecycle | Install, inspect, repair, uninstall, or explicitly purge one owned installation | Platform package/registration, stable bootstrap, path ACL/mode, owner prompts, service/start integration | Release signing, payload admission, Authority use, Application grants |
| Authority Custody | Export/test-restore/reconcile one Vault or Bundle under explicit owner action | Encryption, monotonic authority commitments, locked/export-only state, tamper handling | Runtime Instance Keys, Local Grants, package activation, network work while locked |
| Application Broker | Launch/bind/revoke one principal and grant-scoped session | OS peer identity, process-tree ownership, local IPC, fresh capabilities, resource parents | Network isolation claim, Authority Custody, network identity |
| Application Isolation | Launch and observe one named isolation profile | OS token/namespace/job/filter/storage policy, helper inheritance, ingress/egress denial, cleanup | Local Grant decisions, Route policy, content safety, generic fallback |

Deletion test: removing any row must force its complexity into several callers;
otherwise the proposed package is shallow and must not be created.

## 5. Stage slices

### S7.0 — Decision and artifact freeze

The [readiness checklist](stage-7-readiness-checklist.md) sections A-D are the
complete normative entry gate. They cover joint document acceptance, decided
research, promoted/rejected decision proposals, frozen platform/evidence inputs,
Stage 6 `advance`, and explicit Product Owner coding authority. Research spikes
remain disposable.

### S7.1 — Authenticated offline release decision

An offline-import fixture supplies one complete valid metadata set and artifact
for the current environment/platform. The Release Decision Module returns one
accepted target or a bounded rejection and atomically advances only its own
release floors.

Required negatives: threshold, delegated authority, expiry, version rollback,
freeze, mix-and-match, platform/environment/network, size/hash, unrecognized
critical fields, artifact path confinement, and timestamp/snapshot attempting
to add executable authority.

Exit: identical bytes from two distributors yield the same decision; neither
distributor gains authority; independent evidence cells A/B pass.

### S7.2 — Atomic update, rollback, and Authority preservation

Use an accepted release decision to install an immutable staged payload, reserve
one safe rollback, stop new work, drain, atomically activate, self-test, and
commit. Inject interruption before/after every durable transition. Exercise
forward success, self-test failure, safe rollback, refused rollback, disk
pressure, and repair-required.

Authority Vault, Bundle state, release/epoch/Namespace/freshness/generation
watermarks remain outside payload rollback. Copy-on-write mutable schema becomes
current only at commit.

Exit: independent evidence cells C/D pass on a platform-neutral filesystem
fixture and the exact platform activation Adapters chosen by R-050.

### S7.3 — Ubuntu install lifecycle

Build the frozen H3 Ubuntu package. Exercise clean install, offline start,
network start, routine restart, repair, update, rollback, non-empty-Vault
uninstall block/export, empty-Vault uninstall, explicit purge, and zero owned
residue. Runtime is unprivileged; any elevated install action is enumerated.

Exit: Ubuntu cells pass on the frozen image; no OS package signature is treated
as Ardents release authority.

### S7.4 — Windows install lifecycle

Repeat the same Interface and outcomes using the selected Windows package,
bootstrap, activation, ACL, process, and repair Adapters. A Windows-only success
class cannot weaken or rename a shared result.

Exit: Windows cells pass on the frozen image; privilege is no broader than the
declared install action; all owned resources are inventoried and cleaned.

### S7.5 — Cross-platform Application Broker

Launch and bind authorized client, publisher, and service-administration
principals through OS-local IPC. Exercise hostile same-user siblings, stolen and
replayed bearer material, PID reuse, channel substitution, restart, privilege
crossing, resource pressure, drain/revoke, and immediate revoke.

Exit: a capability is useless off the bound channel/process tree; no bearer
survives restart; failed OS identity checks fail closed; evidence cells E pass
on both platforms.

### S7.6 — Generic and network-isolated Application profiles

The generic profile completes the Application Interface journey and reports
`application-networking-unverified`. It must not emit a stronger claim.

The isolated profile launcher covers the complete Application/helper tree,
permits only its scoped local Ardents channel and declared context storage, and
denies every F-cell ingress/egress class. It has no silent generic fallback.

Exit: all required client/publisher and cross-platform development pairings pass
the frozen short matrix. Full Route/isolation qualification remains S9.6.

### S7.7 — Combined Stage 7 development campaign

From clean packages on both frozen hosts, run install → start → attach generic
and isolated Applications → connect/publish/name-resolve through the maintained
H3 path → update success and failed-update rollback → restart/rebind → Authority
Bundle locked restore → repair → uninstall/purge → cleanup.

Exit: the independent verifier accepts every mandatory Stage 7 cell, all
maintained checks pass, no secret is retained in evidence, and the Product Owner
records `advance-to-S8`, `redesign`, or `stop`.

## 6. Security and privacy claims

| Claim | Protected information | Adversary | Conditions | Measurement | Honest limitation |
|---|---|---|---|---|---|
| Release authorization | Executable and policy choice | Malicious distributor, mirror, cache, or package source below threshold | Frozen test roots, current metadata, accepted verifier, persistent floors | Reject substituted/old/mixed/wrong target; identical bytes decide identically | Captured threshold or malicious trusted build may authorize harmful code; H3 keys are not independent |
| Update rollback safety | Authority and non-decreasing security state | Crash, malicious/failed payload, stale retained payload | Immutable versions, copy-on-write migration, safe retained build, durable activation | Crash injection at every transition and exact post-restart state | Filesystem/hardware compromise and malicious bootstrap remain outside claim |
| Local principal separation | One Application's grant/context/diagnostics | Same-user hostile sibling | Launcher/OS-bound tree, scoped IPC, fresh nonpersistent session | Theft/replay/PID-reuse/channel-substitution/restart cells | Endpoint Owner/OS compromise defeats it; generic indistinguishable apps form one trust domain |
| Application network isolation | Ordinary endpoint location from direct app traffic | Malicious peer/content/request and external scanner | Both complete endpoint Application/helper trees in qualified isolated profile | No listener, packet, DNS, fetch, callback, WebSocket/WebRTC, QUIC, or socket escape | Content, credentials, fingerprints, behavior, timing, intended peer plaintext, and OS compromise remain |

## 7. Evidence and verification

The [Stage 7 evidence contract](stage-7-platform-evidence.md) is normative.
Manifest, evidence, and verdict roots are disjoint. Candidate/runner code cannot
author a verdict. Expected fail-closed runtime outcomes can verify as `pass`;
malformed or untrustworthy artifacts are `invalid`; a complete valid contract
breach is `fail`.

Generated packages, keys, artifacts, VMs, state, evidence, captures, logs,
databases, caches, and cleanup inventories remain outside Git. Repository
fixtures contain only synthetic public vectors and no reusable secret.

## 8. Definition of Done

Stage 7 development completes only when all conjuncts hold:

- S7.0 decisions and ADRs are accepted and traceable;
- Stage 6 has an accepted `advance` disposition;
- S7.1–S7.7 Interfaces and maintained callers exist without speculative packages;
- every new package has a cohesive responsibility, `doc.go`, exact imports,
  behavior tests, real caller, and factual package-map row;
- every runtime dependency is selected by research and recorded before `go.mod`;
- Ubuntu and Windows expose the same shared outcomes with no hidden weaker path;
- Authority, monotonic state, release roots, and environment roots remain
  separated through repair/update/rollback/uninstall;
- generic and isolated claims remain visibly distinct;
- all mandatory evidence cells and schema mutation vectors recompute correctly;
- `make quick-check` passed while developing and `make check` passes from the
  clean committed candidate;
- retained external artifacts and cleanup are inventoried; and
- the Product Owner records one final disposition.

## 9. Stop/redesign conditions

Stop or redesign if any required result needs:

- a central account, wallet, installation identity, cohort, or mandatory vendor
  endpoint;
- package distribution to act as release authority;
- a custom TUF/cryptographic implementation when no reviewed dependency fits;
- in-place irreversible Authority or security-watermark migration;
- rollback to revoked/incompatible code or rollback of authenticated floors;
- privilege beyond the declared install/isolation operation, or privileged code
  that can access Authority material;
- PID, port, same-user identity, or bearer alone as a claim-bearing principal;
- an experimental OS Interface as the sole maintained foundation;
- a kernel driver, first-party cgo/unsafe/syscall-filter machinery, or permanent
  privileged daemon without new accepted research and ADR;
- a network-isolated success when any child/helper escapes or when the system
  silently falls back to generic attachment;
- fake independent custodians/builders/reviewers or an H4 claim from H3 test
  identities; or
- maintenance or operational work beyond the actual one-to-one team.
