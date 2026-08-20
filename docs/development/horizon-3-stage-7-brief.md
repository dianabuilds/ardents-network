# Horizon 3 Stage 7 implementation brief

Status: **accepted implementation brief; S7.0 completed and `start S7.1`
authorized on 2026-08-20.** R-048–R-054/R-056, the specifications, development
plan, evidence contract, ADR-0015, ADR-0016, and ADR-0021 form the accepted
baseline. The [readiness checklist](stage-7-readiness-checklist.md) records the
normative coding-start gate.

Audience: Product Owner and implementation agents after explicit authorization.

## 1. Exact outcome

Stage 7 produces one project-controlled H3 release/deployment lifecycle with
relatively equivalent Installed and Portable Distribution Profiles for Ubuntu
LTS `x86-64` and Windows 11 `x86-64`. The current development campaign uses
Ubuntu 26.04 Docker and the current Windows 11 machine; it does not claim native
desktop or pristine-host qualification. Against the same H3 network,
an Endpoint Owner or Network Contributor can:

1. verify and deploy an Installed package or Portable artifact obtained through
   an untrusted source;
2. start the same unprivileged default Endpoint or remain offline and use its
   first-class binary without any browser integration;
3. repair Installed or replace the stopped Portable executable without losing
   Authority or monotonic security state;
4. retrieve/import, stage, drain, activate, self-test, commit, or safely roll
   back one authenticated Installed update;
5. export and test-restore an Authority Recovery Bundle without granting it
   signing power;
6. attach an Application through either an honestly limited generic profile or
   a launcher-bound Network-Isolated Application Boundary;
7. rebind a Local Grant to a fresh Application Principal after restart; and
8. uninstall Installed or directly delete Portable program artifacts without
   treating either action as protected-state authority, or separately perform
   an explicit destructive purge.

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
- [R-056 desktop/browser integration](../research/records/r-056-stage-7-desktop-browser-integration.md);
- [direct-binary/browser Adapter specification](stage-7-application-adapter-spec.md);
- [Application Principal specification](stage-7-application-principal-spec.md);
- [Application Isolation specification](stage-7-application-isolation-spec.md);
- [Authority Custody specification](stage-7-authority-custody-spec.md) and
  [password-derived custody proposal](stage-7-password-derived-authority-custody-proposal.md);
- [lifecycle specification](stage-7-lifecycle-spec.md);
- [development plan](stage-7-development-plan.md);
- [readiness checklist](stage-7-readiness-checklist.md);
- [evidence contract](stage-7-platform-evidence.md);
- [development-host campaign specification](stage-7-host-campaign-spec.md);
- [joint review record](stage-7-joint-review.md);
- [S7.0 start record](stage-7-start-record.md);
- ADR-0006, ADR-0007, the
  [release-activation proposal](stage-7-versioned-release-activation-proposal.md),
  and the
  [Application-principal proposal](stage-7-launcher-bound-application-principals-proposal.md)
  with its active exact
  [R-051 candidate](stage-7-application-principal-spec.md) and
  [R-052 candidate](stage-7-application-isolation-spec.md);
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
- one accepted per-platform Endpoint executable released directly as Portable
  and wrapped by the Installed package, plus two thin Installed lifecycle
  Adapters;
- Authority Vault preservation, encrypted Recovery Bundle export, isolated test
  restore, reconciliation, `authority locked`, and export-only behavior through
  the exact R-053 password-derived envelope candidate;
- Ubuntu local IPC/principal/resource/process ownership;
- Windows local IPC/principal/resource/process ownership;
- generic Application attachment with explicit claim ceiling;
- a first-class direct-binary Application Adapter in both Distribution Profiles;
- accepted R-056 O1 with exact `connect`/`accept` raw-stream commands, optional
  per-user Service-Link registration, an extension-free numeric-loopback HTTP
  Adapter, generic default-browser mode, and an explicit unsupported isolated-
  browser result, without changing system DNS, routes, default proxy/browser,
  or active VPN policy;
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
- transparent or system-wide Application proxying, traffic interception,
  TUN/VPN ownership, bundled-browser maintenance, universal browser sandboxing,
  content safety, malware protection, or endpoint-compromise protection;
- custom cryptography, first-party kernel drivers, cgo, implicit `init`, an
  unreviewed syscall-filter generator, or first-party `unsafe` beyond the exact
  fixed Win32 Job-limit bridge explicitly accepted by ADR-0016 with dedicated
  risk tests;
- automatic cloud backup, help-desk recovery, invented recovery secrets or
  destinations, and guaranteed secure deletion from snapshots/backups;
- Stage 8 integration, multi-day soak, S9.6 qualification, Route Qualification,
  or H4 claims; and
- package, command, dependency, or Interface placeholders created before a real
  slice owns them.

## 4. Deep Modules and seams

These are logical responsibilities, not permission to create packages. R-049
through R-053 must prove the listed seams; R-056 must platform-falsify the thin
direct-binary and optional browser candidate frozen in the Application Adapter
specification. The first maintained slice must satisfy the repository package
rules before package-map registration.

| Module | Small external Interface | Hidden Implementation | Forbidden authority |
|---|---|---|---|
| Release Decision | Evaluate authenticated release inputs for one exact local platform/environment and return a bounded decision | Metadata roles, thresholds, expiry, rollback/freeze/mix checks, target and attestation validation, persisted release floors | Download, install, activate, sign, or select protocol capacity |
| Update Transaction | Apply one accepted release and return committed, rolled-back, or repair-required | Staging, disk reserve, drain, immutable payload, copy-on-write state, activation, self-test, crash recovery | Release trust, Authority Custody, OS package trust, hidden network retrieval |
| Install Lifecycle | Install, inspect, repair, uninstall, or explicitly purge one owned Installed profile | Ubuntu/Windows package/bootstrap/optional registration, program paths, ACL/mode, owner prompts | Portable direct execution, release signing, payload admission, Authority use, Application grants |
| Authority Custody | Export/test-restore/reconcile one Vault or Bundle under explicit owner action | Encryption, monotonic authority commitments, locked/export-only state, tamper handling | Runtime Instance Keys, Local Grants, package activation, network work while locked |
| Application Broker | Launch/bind/revoke one principal and grant-scoped session | Private inherited channel, non-reusable root process handle, complete Job/cgroup tree ownership, Local Grant, fresh session, resource parents | Network isolation claim, Authority Custody, network identity |
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
fixture and the two Installed platform activation Adapters chosen by R-050;
Portable stopped replacement separately proves release-floor/state safety.

### S7.3 — Ubuntu Installed and Portable lifecycle

Build the manifest-bound H3 Ubuntu Installed package and raw Portable executable
target, with the package containing that exact executable digest. Exercise
container `.deb` install/start/repair/update/rollback/uninstall/purge smoke and the small Portable
contract: no-install direct start, zero implicit registration, protected-state
separation/lock, stopped replacement/deletion, feature/claim parity, and zero
program-object residue. Runtime is unprivileged; any elevated Installed action
is enumerated.

Exit: the scheduled Ubuntu 26.04 Docker cells pass and every unavailable native
desktop/boot/filesystem/browser cell is explicitly `environment-deferred`; no
OS package signature is treated as Ardents release authority.

### S7.4 — Windows Installed and Portable lifecycle

Repeat the same Interface and outcomes using the selected Windows Installed
bootstrap/activation/ACL/repair Adapter and raw Portable executable target. A
Windows/profile-only success class cannot weaken or rename a shared runtime
result; package lifecycle conveniences need not be duplicated in Portable.

No Windows install, repair, update, uninstall, purge, or installer-owned
registration may execute before a separate Product Owner command identifies the
artifact and mutation scope. Exit: the authorized scheduled Windows cells pass;
privilege is no broader than the named install action; all owned resources are
inventoried and cleaned; pristine-host/power-loss cells remain visibly deferred.

### S7.5 — Cross-platform Application Broker

Launch and bind authorized native client, publisher, and service-administration
principals through the exact R-051 private inherited
channel and complete Job/cgroup tree. Exercise hostile same-user siblings,
stolen/duplicated handles or FDs, replayed bearer material, PID reuse, channel
substitution, debugger/ptrace, child escape, restart, privilege crossing,
resource pressure, drain/revoke, Broker crash, and immediate revoke. Direct
binary remains a co-resident per-invocation path with claim `none`; named
attachment remains generic/coarse.

Exit: a capability is useless off the bound channel/process tree; no bearer
survives restart; failed OS identity checks fail closed; evidence cells E pass
on both platforms. The Windows candidate also has an explicit ADR-0016
disposition for the fixed Job-limit `unsafe.Pointer` bridge before code.

### S7.6 — Generic and network-isolated Application profiles

The generic profile completes the Application Interface journey and reports
`application-networking-unverified`. It must not emit a stronger claim.

The isolated profile launcher covers the complete Application/helper tree,
permits only its scoped local Ardents channel and declared context storage, and
denies every F-cell ingress/egress class. It has no silent generic fallback.

The accepted R-056 O1 topology keeps direct binary use primary and adds a bounded
explicit Service Link/browser Adapter for these two Application profiles. The
binary uses the frozen stdio/result/exit contract; direct `browse` requires no
registration. Browser delivery uses no extension, native host, or proxy: it is
one ephemeral numeric-loopback HTTP Adapter per context. Generic mode uses the
current default browser and is always unverified. R-052 selects no isolated-
browser profile for Stage 7: the exact request fails
`isolation-unsupported` without listener, browser, state change, or generic
fallback. Native controlled client/publisher Applications supply the
claim-bearing profiles on both platforms. Installed and Portable have no
feature/claim skew. No Adapter changes system DNS/routes/default proxy/browser/
VPN policy; an active VPN or kill switch may carry or block Endpoint traffic,
and a block remains an explicit unavailable result rather than a direct bypass.

Exit: all required client/publisher and cross-platform development pairings pass
the precommitted short matrix. Full Route/isolation qualification remains S9.6.

### S7.7 — Combined Stage 7 development campaign

From the manifest-bound Docker/current-Windows artifacts, run only the scheduled
and authorized portions of install/direct-start → direct binary → start → attach generic
and isolated Applications → connect/publish/name-resolve through the maintained
H3 path → Installed update success/failed-update rollback and Portable stopped
replacement/old-build refusal → restart/rebind → Authority Bundle locked restore
→ Installed repair/uninstall and Portable deletion → purge → cleanup.

Exit: the independent verifier accepts every scheduled Stage 7 episode, the
coverage ledger accounts for every reference episode, all maintained checks
pass, no secret is retained in evidence, and the Product Owner accepts or
rejects the pending/deferred claim ceiling before recording `advance-to-S8`,
`redesign`, or `stop`.

The final R-054 reference profile contains exactly 91 logical cells expanded to
392 one-attempt episodes across both hosts, both Distribution Profiles, four
platform/distribution pairings, both Application Data directions, and both
verifier platforms. Each development run freezes an exhaustive `scheduled`,
`authorization-pending`, and `environment-deferred` partition. No successful
subset is complete host qualification, and no replacement run is the final
campaign.

## 6. Security and privacy claims

| Claim | Protected information | Adversary | Conditions | Measurement | Honest limitation |
|---|---|---|---|---|---|
| Release authorization | Executable and policy choice | Malicious distributor, mirror, cache, or package source below threshold | Frozen test roots, current metadata, accepted verifier, persistent floors | Reject substituted/old/mixed/wrong target; identical bytes decide identically | Captured threshold or malicious trusted build may authorize harmful code; H3 keys are not independent |
| Update rollback safety | Authority and non-decreasing security state | Crash, malicious/failed payload, stale retained payload | Immutable Installed versions or stopped Portable replacement, copy-on-write migration, safe retained build, durable Installed activation | Installed crash injection at every transition plus exact Portable post-replacement start state | Filesystem/hardware compromise, malicious bootstrap, or Owner replacing Portable with unauthorized bytes remain outside claim |
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
- every scheduled host-campaign episode and schema mutation vector recomputes
  correctly, while the coverage ledger accounts for all 392 reference episodes
  without treating pending/deferred coverage as success;
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
