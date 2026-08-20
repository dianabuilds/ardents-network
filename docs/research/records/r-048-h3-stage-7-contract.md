---
id: R-048
title: What exact bounded contract and decision order makes Horizon 3 Stage 7 implementable?
status: review
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-048 — H3 Stage 7 contract and decision order

## Decision this unlocks

Accept one bounded, implementation-ready Stage 7 documentation set without
starting Stage 7 maintained code before Stage 6 advances. The decision fixes
the behavior, Module seams, research order, evidence ownership, falsification
criteria, and Product Owner gates for installation, release verification,
update/rollback, Authority recovery, local Application attachment, and
Ubuntu/Windows isolation.

This record does not select a package format, updater library, vault mechanism,
Windows sandbox, Linux sandbox, local IPC Adapter, or executable layout. Those
choices require R-049 through R-054 and, where consequential, an accepted ADR.

## Current contract

The following are already authoritative:

- [H3 Stage 7](../../development/horizon-3-technical-design.md#stage-7--install-update-platforms-and-application-isolation)
  requires Ubuntu and Windows install, repair, update, rollback, recovery,
  principal rebinding, generic attachment, and network-isolated attachment;
- [J-00](../../product/journeys.md#j-00--install-repair-and-remove-ardents),
  [J-04](../../product/journeys.md#j-04--integrate-an-application), and
  [J-08](../../product/journeys.md#j-08--update-an-endpoint-publisher-or-contributor)
  define the human outcomes and honest failures;
- the [operating model](../../product/operating-model.md#6-updates-compatibility-and-withdrawal)
  fixes a TUF-shaped release model, three explicit retrieval modes, separate
  protocol/build state machines, finite Work Safety Leases, and atomic local
  update semantics;
- [ADR-0006](../../adr/0006-separate-release-safety-from-protocol-transition.md)
  separates release safety from protocol transition;
- [ADR-0007](../../adr/0007-separate-carrier-privacy-from-application-egress.md)
  separates the carrier claim from Application-network isolation; and
- [R-024](r-024-operational-product-closure.md) fixes Authority Recovery Bundle,
  uninstall/purge, Local Grant restart, and Application Principal semantics.

Stage 6 has not advanced. Stage 7 research and documentation may be prepared in
parallel, but no Stage 7 package, command, dependency, platform privilege, or
maintained Implementation is authorized by this record.

The active team is one Product Owner and Codex. H3 can exercise threshold and
multi-builder mechanics with visibly project-controlled test identities. It
cannot claim independent custodians, builders, package distributors, operators,
auditors, or security reviewers. Those remain Horizon 4 gates.

## Hypotheses

- **H1:** Stage 7 is feasible as seven ordered vertical slices behind six deep
  Modules: Release Decision, Update Transaction, Install Lifecycle, Authority
  Custody, Application Broker, and Application Isolation. Each Module has a
  narrow Interface, and Ubuntu/Windows differences stay in real platform
  Adapters.
- **H2:** one platform-specific installer/updater/runtime per operating system is
  necessary, accepting duplicated release, state, rollback, and authorization
  behavior.
- **H0:** the required result needs an always-online vendor account, broader
  privilege than an explicit installer/broker, custom cryptography, unsafe
  mutable-state migration, unverifiable process ownership, or a false
  Application-privacy claim.

## Evaluation criteria

### Exact user outcome

From a package obtained through any source, an Endpoint Owner can install an
unprivileged default Endpoint on one frozen Ubuntu LTS `x86-64` or Windows 11
`x86-64` image; start or remain offline; repair; explicitly update or roll back;
preserve or export Authority state; attach an Application through either an
honestly limited generic profile or a qualified isolated profile; and remove or
separately purge owned state.

### Trust and privacy

- Package distribution never becomes release authority.
- Development, H3 test, and future public roots and state never merge.
- An update cannot irreversibly transform Authority or monotonic security state
  before commit.
- A Local Grant is usable only through a fresh OS-enforced or launcher-brokered
  Application Principal; PID, port, desktop user, or copyable bearer alone is
  insufficient.
- The generic profile receives no malicious-sibling or Application-network
  privacy claim.
- The isolated profile must deny ordinary DNS, listeners, fetch, WebSocket,
  WebRTC, QUIC, callback/SSRF, and arbitrary socket escape for the complete
  Application/helper process tree.

### Resource and maintenance limits

- one shared behavioral contract and deterministic verifier;
- no first-party `unsafe`, cgo, custom cryptographic primitive, kernel driver,
  or permanent privileged daemon without a superseding accepted ADR;
- privilege exists only for the exact install or isolation operation that needs
  it and does not hold Authority material;
- finite metadata, artifact, staging, rollback, log, IPC, process, disk, memory,
  retry, drain, self-test, and cleanup bounds;
- one-to-one maintenance: no always-on signing ceremony, help desk, rollout
  operations team, or manual review panel in the H3 runbook.

### Decision and evidence rule

Each technology choice must record source identity, dependency and license
closure, supported OS/API floor, advisory state, privilege, rollback/failure
behavior, reproducible experiment, and removal trigger. Platform success is
conjunctive: a passing Ubuntu Adapter cannot compensate for a failing Windows
Adapter or the reverse.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [TUF specification 1.0.36](https://theupdateframework.github.io/specification/latest/)
  for role separation, thresholds, expiry, rollback/freeze protection,
  consistent snapshots, and the boundary between trusted retrieval and actual
  installation;
- [go-tuf v2](https://github.com/theupdateframework/go-tuf) and its
  [CVE-2026-24686 advisory](https://github.com/theupdateframework/go-tuf/security/advisories/GHSA-jqc5-w2xx-5vq4)
  for the maintained Go candidate and its current misuse/advisory surface;
- [SLSA provenance 1.2](https://slsa.dev/spec/v1.2/provenance) for verifiable
  build-origin semantics without inventing builder independence;
- Microsoft documentation for
  [AppContainer launch](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer),
  [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects),
  [named-pipe client process identity](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getnamedpipeclientprocessid),
  [named-pipe impersonation](https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-impersonatenamedpipeclient),
  [atomic file replacement](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew),
  and [MSIX signing](https://learn.microsoft.com/en-us/windows/msix/package/signing-package-overview);
- Linux kernel documentation for
  [seccomp filters](https://docs.kernel.org/userspace-api/seccomp_filter.html)
  and [`no_new_privs`](https://docs.kernel.org/userspace-api/no_new_privs.html),
  plus Linux man-pages for
  [Unix peer credentials](https://man7.org/linux/man-pages/man7/unix.7.html)
  and [network namespaces](https://man7.org/linux/man-pages/man7/network_namespaces.7.html);
  and
- [Debian Policy maintainer-script lifecycle](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html)
  for idempotent install/upgrade/remove failure behavior.

Repository sources accessed 2026-08-20 are the product scope, journeys,
operating model, threat model, CONTEXT glossary, H3 technical design, R-002,
R-023, R-024, R-031, ADR-0006, and ADR-0007.

### Experiments

R-049 through R-054 own the precommitted experiments. Disposable code belongs
under `experiments/<question-id>-<slug>/`; packages, caches, VMs, signing keys,
generated installers, artifacts, captures, and evidence remain outside Git.

Every experiment begins from immutable test roots and a frozen host image,
executes positive, negative, crash, restart, tamper, pressure, and cleanup cases,
and ends in independently recomputed `pass|fail|invalid`. A spike cannot become
maintained code or a package-map fact by copying it.

### Failure scenarios

- replayed, expired, frozen, mixed, wrong-platform, wrong-network, oversized, or
  digest-mismatched release metadata/artifact;
- timestamp/snapshot duty attempting to authorize executable bytes;
- crash or power-loss equivalent before and after every update state transition;
- rollback to revoked/incompatible code or rollback of security watermarks;
- disk pressure, incomplete drain, failed self-test, and forward/rollback start
  failure;
- corrupted Vault, stale Recovery Bundle, wrong environment, missing
  reconciliation, and attempted signing while `authority locked`;
- hostile same-user sibling, bearer theft/replay, PID reuse, pipe/socket
  substitution, process restart, child-process escape, and grant revocation;
- DNS/listener/fetch/callback/WebSocket/WebRTC/QUIC/arbitrary-socket escape from
  both client and publisher process trees; and
- failed repair, uninstall with non-empty Vault, destructive purge, partial
  cleanup, and secret/evidence leakage.

## Findings

- **Sourced fact:** TUF authenticates trusted target retrieval but explicitly
  does not define package format or install the obtained file. Installation and
  rollback therefore need their own Ardents Module and evidence.
- **Sourced fact:** TUF metadata roles, version/expiry checks, target hashes and
  sizes, and consistent snapshots match the already accepted release-authority
  contract. TUF does not hide update lookup or download metadata.
- **Sourced fact:** `go-tuf/v2` supplies the current Go client workflow, but the
  legacy implementation is deprecated and a 2026 path-traversal advisory
  affected versions through `v2.4.0`; any selection must pin a patched version,
  exclude unneeded multi-repository input, and test cache confinement.
- **Sourced fact:** Windows AppContainer and Job Objects can constrain an
  Application token and process tree. Job breakaway flags and nested jobs affect
  coverage. The June 2026 `CreateProcessInSandbox` Interface remains explicitly
  experimental and cannot be a maintained Stage 7 foundation without a future
  stability decision.
- **Sourced fact:** a Windows named pipe can expose client PID/token information,
  but failed impersonation leaves the server in its own security context. The
  broker must fail closed; PID or a pipe name alone cannot become a principal.
- **Sourced fact:** Linux seccomp filters and `no_new_privs` inherit across
  `fork`/`clone`/`execve`, while kernel documentation explicitly says seccomp is
  not a complete sandbox. Network, filesystem, process, IPC, and cleanup
  controls must be evaluated together.
- **Sourced fact:** OS package signatures and stores impose platform trust and
  delivery rules. They are additional evidence and cannot replace Ardents
  release roots. Debian maintainer scripts may run at install/upgrade/remove and
  must be idempotent; they therefore must not improvise Authority migration.
- **Inference:** one stable platform-installed bootstrap plus immutable
  versioned payload directories and an atomically replaced activation record
  gives the smallest shared rollback seam. Package-specific install mechanics
  remain thin Adapters, while release verification and state migration remain
  identical across platforms.
- **Inference:** the Application Broker and Application Isolation Modules must
  remain separate. A principal can be authenticated without constraining its
  network, and network confinement can exist without authorizing an Ardents
  operation.
- **Assumption:** at least one Ubuntu and one Windows isolation candidate can
  cover a complete hostile helper process tree without privilege broader than
  the declared launcher. R-052 must falsify this before maintained selection.
- **Assumption:** a TUF-compatible Go dependency can meet source, dependency,
  advisory, conformance, size, and one-to-one maintenance limits. R-049 must
  measure this; first-party reimplementation is not the default fallback.

## Options

### O1 — Shared deep Modules with platform Adapters

One Release Decision Module verifies metadata and persisted release watermarks.
One Update Transaction Module owns stage/drain/activate/self-test/commit/rollback.
One Install Lifecycle Module owns only platform integration and owned path
creation/removal. Authority Custody, Application Broker, and Application
Isolation remain separate Modules. Ubuntu and Windows provide real Adapters at
the platform seams.

This maximizes behavioral locality, keeps privilege narrow, and lets the same
manifest and verifier exercise both platforms. It requires deliberate platform
experiments before the Adapters are selected.

### O2 — Independent Ubuntu and Windows product stacks

Rejected. Release, rollback, state migration, grant semantics, error taxonomy,
and evidence would diverge. The duplicated security behavior is beyond the
one-to-one maintenance model and makes cross-platform results incomparable.

### O3 — One privileged installer/updater/broker/custody daemon

Rejected. Deleting the process would spread unrelated privilege and authority
to every caller, which demonstrates a shallow and dangerous Interface. A
compromise could join package installation, code activation, local authorization,
network isolation, and root-key custody.

### O0 — Do not start Stage 7 after Stage 6

Required if R-049 through R-054 cannot select maintained candidates without
custom cryptography, first-party kernel code, broad permanent privilege, unsafe
rollback, unverifiable process ownership, or a false privacy claim.

## Recommendation

Choose O1 as the bounded Stage 7 architecture and decision order. Accept the
linked brief, lifecycle specification, development plan, readiness checklist,
evidence contract, and decision proposals as the complete preparation set. The
[readiness checklist](../../development/stage-7-readiness-checklist.md) is the
sole normative coding-start gate. During the remainder of Stage 6, only
documentation and disposable R-049–R-054 research are authorized.

Confidence is medium. The strongest counterargument is that Windows and Ubuntu
may need materially different installation and isolation lifecycles. The shared
Interfaces remain justified only if the experiments show platform differences
can stay behind narrow Adapters without hiding weaker guarantees.

## Disposition

- State: `review`; awaiting Product Owner walkthrough and explicit acceptance.
- Proposed scope decision: O1.
- Prepared development documents:
  [Stage 7 brief](../../development/horizon-3-stage-7-brief.md),
  [lifecycle specification](../../development/stage-7-lifecycle-spec.md),
  [development plan](../../development/stage-7-development-plan.md),
  [readiness checklist](../../development/stage-7-readiness-checklist.md), and
  [evidence contract](../../development/stage-7-platform-evidence.md).
- Consequential decision proposals, promoted to ADRs only after evidence and
  explicit Product Owner acceptance:
  [release activation](../../development/stage-7-versioned-release-activation-proposal.md)
  and
  [Application principals](../../development/stage-7-launcher-bound-application-principals-proposal.md).
- R-049–R-054 remain required research, not hidden implementation subtasks.
- `CONTEXT.md` requires no change: every product term used here is already
  canonical; implementation terms remain outside the glossary.
- No package, command, dependency, or maintained implementation is authorized.
