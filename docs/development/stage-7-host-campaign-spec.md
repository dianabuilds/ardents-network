# Stage 7 development-host campaign specification

Status: **accepted pre-execution specification under the Product Owner S7.0 and
environment decisions of 2026-08-20.** Ubuntu evidence runs in an Ubuntu 26.04 Docker
container and Windows evidence runs on the current Windows machine. Neither is
called a clean host. No Windows Installed package, repair, uninstall, purge, or
installer-owned registration may run until the Product Owner gives a separate
explicit command describing the authorized mutation scope.

This document maps the 91 logical A–H cells in the
[Stage 7 evidence contract](stage-7-platform-evidence.md) to the actually
available one-to-one development environment. It deliberately separates a
development result from future supported-host qualification. Docker, the
current Windows machine, a logic prototype, and candidate self-report cannot
prove a pristine-install, native Ubuntu Desktop, hard-power-loss, or independent
platform claim.

## 1. Campaign and coverage identities

The executable profile is
`ardents-h3-stage-7-development-host-campaign-v1` and consumes canonical S7E1
profile `ardents-h3-stage-7-evidence-v1`. It has two development surfaces:

- `ubuntu-26.04-docker-smoke`: a digest-pinned Ubuntu 26.04 AMD64 container on
  the current Docker engine/host kernel; and
- `windows-current-host-development`: the current Windows 11 AMD64 machine,
  inventoried before every run.

The complete reference inventory remains exactly 91 logical cells expanded to
392 qualification episodes. It is a coverage ledger, not a promise that these
development surfaces can execute every episode. Before candidate execution, a
run manifest partitions all 392 reference episodes into exactly one of:

- `scheduled`: observable on the selected development surface and authorized
  for this run;
- `authorization-pending`: technically runnable but blocked on a separate
  Product Owner command, including every Windows Installed lifecycle mutation;
  or
- `environment-deferred`: depends on a native Ubuntu Desktop/boot/filesystem/
  browser/kernel boundary, pristine Windows state, destructive power loss, or
  another fact unavailable on the two declared surfaces.

The three partitions are disjoint and exhaustive. Their counts and episode
identities are frozen before the first scheduled candidate attempt. A candidate
result cannot move an episode between partitions. Only `scheduled` episodes
receive independent `pass|fail|invalid`; pending/deferred is coverage, never a
success or `not-applicable` verdict. The campaign summary is therefore
`coverage=development-partial` unless all 392 episodes someday run on qualifying
surfaces.

Every logical cell has exactly one case ID, `stage7-<lower-case-cell>-v1`
(`A0` becomes `stage7-a0-v1`). A case may drive several probes inside an
episode; their manifest contains a contiguous zero-based `probe_ordinal`, exact
input/fault commitment, deadline, and required observation streams. The
corresponding row of `stage-7-platform-evidence.md` at the manifest-bound source
digest is authoritative. No case, probe, retry, or replacement is added after
results.

## 2. Reference expansion

The reference expansion stays stable so deferred coverage remains inspectable:

| Logical cells | Reference axes | Episodes |
|---|---|---:|
| A0–A9 | 2 platforms; Installed only | 20 |
| A10 | 2 platforms; Installed/Portable comparison | 2 |
| A11–A13 | 2 platforms; Portable only | 6 |
| B0–B14 | 2 platforms; shared Release Decision behavior | 30 |
| C0–C11 | 2 platforms; Installed only | 24 |
| D0, D2–D6 | 2 platforms × 2 Distribution Profiles | 24 |
| D1 | 4 platform pairs × 4 distribution pairs | 16 |
| E0–E14 | 2 platforms × 2 Distribution Profiles | 60 |
| F0–F11 | 2 platforms × 2 Distribution Profiles | 48 |
| G0–G3 | cell's fixed platform pair × 4 distribution pairs × 2 directions | 32 |
| G4–G6 | 4 platform pairs × 4 distribution pairs × 2 directions | 96 |
| G7 | 2 platforms × 2 Distribution Profiles | 4 |
| G8 | 2 platforms; Installed/Portable comparison | 2 |
| H0–H6 | 2 verifier platforms × 2 source-platform evidence variants | 28 |
| **Total reference inventory** | **91 logical cells** | **392 episodes** |

The expansion order is table order, logical cell numeric order, platform order
`ubuntu,windows`, platform-pair order `uu,uw,wu,ww`, profile order `i,p`,
distribution-pair order `ii,ip,pi,pp`, Application Data direction
`client-to-publisher,publisher-to-client`, and verifier platform
`ubuntu,windows`. Unsupported isolated-browser requests remain real F11 probes
where the surface can observe their side effects; they are never converted to a
generic success.

## 3. Ubuntu 26.04 Docker surface

The manifest freezes the Ubuntu image repository/digest, OCI configuration,
container runtime and engine, host kernel/virtualization identity, architecture,
filesystem/mount/network mode, capabilities, seccomp/AppArmor policy, cgroup
view, resolver/routes, package inventory, resource limits, and controlled peer.
The image is acquired before manifest publication and no package/tool is updated
after candidate results.

This surface may provide development evidence for:

- Linux compilation and direct Portable execution;
- Release Decision, canonical format/state, bounded resource, and verifier
  behavior not dependent on native boot/desktop semantics;
- disposable `.deb` construction/install/reinstall/remove script behavior
  inside the container, explicitly labelled `container-package-smoke`;
- cross-process/protocol/Application Data behavior visible through the declared
  container and peer observers; and
- Ubuntu-side logic vectors for Authority Custody and cross-platform Bundle
  compatibility.

It does **not** qualify:

- Ubuntu Desktop, a real default browser/URI desktop handler, Snap/session/D-Bus
  integration, system boot, login, systemd ownership, or host uninstall residue;
- ext4 device-cache durability, reboot recovery, inode/disk exhaustion of a
  dedicated host volume, or acknowledged-write hard-power interruption;
- host-kernel user-namespace/cgroup/pidfd/bubblewrap confinement when the
  relevant fact is virtualized, filtered, shared with the Docker host, or
  unavailable to the observer; or
- absence of host packets, routes, listeners, helpers, files, or policy changes
  that a container-scoped observer cannot see.

A privileged container, host mount, host network, new capability, Docker daemon
configuration change, or host package installation is not inferred from this
campaign. It needs separate explicit authorization. An unavailable observation
becomes `environment-deferred` before execution or `invalid` if it unexpectedly
fails during a scheduled episode.

## 4. Current Windows machine surface

Before each run, a read-only preflight records exact Windows edition/build/UBR,
updates, firmware/hardware, NTFS volume identity, installed software and Ardents
objects, current browser/protocol handlers, AppContainer/Job/firewall/loopback
inventory, routes, proxy/VPN adapters and policy, locale/time zone, observer
versions, and existing path/Registry collisions. The baseline is evidence of
the current state, not proof that the machine is pristine.

Without a new Product Owner command, allowed preparation is limited to
documentation, source/build review, disposable logic tests, read-only inventory,
and non-installer artifacts that the current task separately authorizes. In
particular, do not run or simulate:

- MSI install, reinstall, repair, upgrade, uninstall, purge, rollback, or
  installer custom actions;
- installer-owned filesystem, Registry, service, startup, URI, browser, or
  package registration; or
- reboot, forced power interruption, firewall/proxy/DNS/route/VPN mutation, or
  destructive storage fault injection.

When the Product Owner later commands Windows installation, the command must
identify the artifact and permitted lifecycle cells. That authorization is
recorded verbatim/digested in the run manifest before the first mutation and
does not automatically authorize a newer artifact, broader cell set, purge,
reboot, network-policy change, or power cut. The campaign captures pre/post
inventories and performs only the named operations.

The current machine can provide useful Windows API, Portable, browser handoff,
principal, resource, and package-lifecycle development observations. It cannot
prove collision-free first install, pristine-state defaults, complete residue
relative to an unknown historical baseline, VM snapshot recovery, or
acknowledged-write hard-power durability. Those episode slots remain visibly
deferred rather than weakening their predicates.

## 5. Observer profile and controls

Observers start before a scheduled candidate and write only external raw roots.
The candidate/runner receives no observer-control, evidence-index, or verdict
handle. Candidate logs remain diagnostic. Missing, late, dropped, unparseable,
or identity-mismatched observation is `invalid`, never observed absence.

| Domain | Ubuntu Docker development source | Current Windows development source |
|---|---|---|
| process/tree/identity | container-visible `/proc`, cgroup view, Docker runtime facts and controlled peer | in-box WPR/ETW process profile plus documented process token, AppContainer SID, Job queries |
| filesystem/package/registry | container mount/package snapshots and package-manager logs | WPR File/Registry I/O, MSI verbose log only when install is authorized, NTFS/Registry/package snapshots |
| owner/mode/ACL | container-visible `stat`/ACL/mount/link facts | owner/DACL/inheritance/reparse/file-ID snapshots |
| handle/FD/IPC | container-visible `/proc/<pid>/fd`/`fdinfo` | WPR Handle Usage plus process snapshots |
| listener/packet/DNS | container and controlled-peer socket/packet/resolver observations when capabilities permit | NetTCPIP endpoint snapshots, resolver state, `pktmon`, and controlled peer |
| route/proxy/VPN | container namespace plus Docker-network and host-baseline facts | NetTCPIP/adapter/route, WinHTTP/user proxy, VPN/kill-switch/firewall inventories |
| resource/deadline | container/cgroup limits and monotonic clock | WPR resource profiles, Job accounting, and monotonic clock |
| transaction/durability | process interruption and container-volume state only | flush/replace/process interruption; reboot/power-loss deferred unless separately authorized and safely provisioned |
| cleanup/survivors | union of container-visible sources plus Docker object inventory | union of Windows sources plus final owned-object inventory |

Each scheduled observer domain runs five positive/negative control pairs before
and five after its dependent attempts. An unavailable control defers that domain
before execution; a missed control during execution invalidates its scheduled
episodes. Container evidence cannot be promoted to a host fact by combining it
with candidate self-report.

Windows ETL is retained raw and converted with the manifest-pinned in-box
`tracerpt`; WPR profile exports and hashes are captured before controls.
Sysinternals tools are not required. Ubuntu observer packages are campaign-only,
not Ardents runtime dependencies; exact versions, rules, filters, buffers, and
drop counters remain manifest inputs.

## 6. Execution and authorization order

1. **Read-only preflight:** inventory Docker/current Windows without installing
   Ardents or changing system/network policy.
2. **Coverage freeze:** partition all 392 reference episodes into scheduled,
   authorization-pending, and environment-deferred with reasons.
3. **Supply freeze:** bind source, toolchain, OCI image, candidate, observer,
   parser, peer, and external-root identities.
4. **Non-install development run:** execute only scheduled, authorized,
   observable container/Portable/logic/protocol cells.
5. **Windows authorization stop:** if any Windows Installed episode is pending,
   stop and wait for the Product Owner's separate command; do not treat waiting
   as a candidate failure or success.
6. **Authorized Windows mutation run:** publish a new run manifest containing
   the exact authorization and execute only its named operations.
7. **Admission/verdict:** independently reduce scheduled evidence to
   `pass|fail|invalid` and publish the coverage ledger beside, never inside, the
   verdict authority.
8. **Cleanup:** remove only declared owned disposable artifacts; preserve
   protected state and unrelated current-machine/Docker state.

A later run has a fresh run ID and cannot replace a retained failure/invalid.
Development evidence can support implementation decisions only under its stated
coverage. It cannot be cited as a complete 392-episode host qualification or as
H4/public release evidence.

## 7. Stage gate and future qualification

The Product Owner may accept the reduced development environment as sufficient
to start or continue Stage 7 maintained implementation after Stage 6 and the
other readiness/ADR gates. That is an explicit risk/coverage acceptance, not a
synthetic `pass` for deferred cells.

Native Ubuntu Desktop lifecycle/browser/isolation, pristine Windows install and
residue, hard-power durability, and any other deferred episode remain future
qualification gates. Under the actual one-to-one team model they are recorded
as unscheduled future requirements until the Product Owner supplies the needed
surface; documentation must not pretend such hosts or operators are currently
available.

Generated packages, keys, containers, VM/storage images, captures, ETL/pcap/
audit logs, private fixtures, state, and evidence remain outside Git.
