---
id: R-050
title: Which Ubuntu and Windows install and atomic-update Adapters meet the Stage 7 proposal?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-050 — Stage 7 install and update Adapters

## Decision this unlocks

Select the supported Ubuntu/Windows package, stable bootstrap, immutable payload
layout, atomic activation, crash durability, repair, uninstall, and purge
Adapters for S7.2–S7.4. Freeze exact platform images, filesystems/volumes,
privilege, tools, paths, ACL/modes, bounds, and cleanup.

## Current contract

R-048, the lifecycle specification, and the
[release-activation decision proposal](../../development/stage-7-versioned-release-activation-proposal.md)
require one shared
release/update behavior, thin platform install Adapters, immutable versioned
payloads, an atomically replaced activation record, copy-on-write mutable state,
safe authenticated rollback only, Authority/floor separation, and unprivileged
runtime. OS package trust is additional delivery evidence, not Ardents authority.

## Hypotheses

- **H1:** a small stable platform package/bootstrap plus immutable versioned
  payload directories and one atomic activation file works on both supported
  filesystems behind narrow Adapters.
- **H2:** fully OS-package-managed payload updates are required per platform,
  while a shared Release Decision still constrains accepted package identity.
- **H0:** safe crash-consistent activation/rollback requires in-place mutation,
  broad permanent privilege, hidden OS trust, or unequal platform guarantees.

## Evaluation criteria

- ordinary signed install/repair/remove on frozen Ubuntu LTS and Windows 11
  `x86-64` images;
- explicit package build/source/tool/license identity and reproducibility inputs;
- unprivileged runtime and exact elevated install operations;
- same-volume atomic activation and documented durability primitive;
- open-executable, reboot/power-loss equivalent, antivirus/file-lock, permission,
  path-length, link/reparse, cross-volume, and full-disk behavior;
- immutable payload verification, bounded retained versions and rollback reserve;
- idempotent repair/uninstall after partial platform operation;
- Vault/floor preservation and non-empty-Vault uninstall block/export;
- complete path/registry/service/startup/package/rule/process residue inventory;
  and
- one shared outcome taxonomy with no hidden weaker platform success.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [Microsoft MSIX signing](https://learn.microsoft.com/en-us/windows/msix/package/signing-package-overview);
- [Microsoft ReplaceFileW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew);
- [Debian Policy package lifecycle and idempotency](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html);
- the Go `os`/`filepath` platform documentation and selected OS API source after
  candidates are enumerated; and
- repository localroles/network-store atomic persistence patterns as evidence of
  experience only, not a Stage 7 selection.

### Experiment

Create `experiments/r-050-stage-7-install-update-adapters/` with separate frozen
Ubuntu and Windows host manifests. Build packages outside Git. Interrupt before
and after each platform and transaction transition; reboot/restart; fill disk;
move staging across volume; hold files open; inject unexpected links/entries;
corrupt activation/journal; repair; uninstall; purge; and compare the complete
external residue tree to the precommitted inventory.

### Failure scenarios

Wrong package/release identity; partial install; non-idempotent maintainer action;
activation across unsupported storage; lost durability; ACL/mode inheritance
change; bootstrap/payload mismatch; safe rollback absent/revoked; authority/floor
mutation; service/startup residue; uninstall with Vault; cancellation during
purge; and a platform result that cannot map to the shared Interface.

## Falsification criteria

Before the experiment, enumerate every transition/fault injection and owned
platform object. H1/H2 is falsified on a platform if any valid required A/C/D
case cannot reach the shared outcome; unauthorized bytes execute; activation
needs in-place payload mutation or cross-volume atomicity; a crash loses the
last committed activation; ordinary runtime retains elevation; repair/update/
rollback changes Vault or a monotonic floor; uninstall bypasses a non-empty
Vault rule; or a declared object/process/registration survives successful
cleanup.

The retained-version envelope is exactly current committed payload plus one
verified rollback payload and at most one staging payload; additional versions
must be rejected or removed before staging. Every precommitted interruption
point runs once on each frozen filesystem/volume profile and must recover the
expected state on the first restart. An unsupported filesystem, lock, privilege,
or durability primitive is an explicit unsupported result, not grounds to weaken
the common Interface after observing results. Failure on either required host
selects O0 unless the supported-platform contract is explicitly changed.

## Findings

- **Sourced fact:** MSIX requires a trusted package signature for installation;
  this platform trust does not express Ardents threshold release authority.
- **Sourced fact:** ReplaceFileW combines replacement operations but has specific
  attribute/ACL behavior and no supported write-through flag; durability must be
  measured rather than inferred.
- **Sourced fact:** Debian maintainer scripts execute across install/upgrade/
  remove and must be idempotent because error recovery may rerun them.
- **Inference:** platform scripts should install/repair the stable bootstrap and
  owned layout, while the common Update Transaction owns payload activation and
  never performs Authority migration inside a package script.

## Options

- **O1:** native signed package per platform for bootstrap/registration plus
  shared versioned-payload transactions.
- **O2:** OS package manager owns every payload update, with shared verified
  target identity and reduced application rollback control.
- **O0:** stop if no option preserves identical safe semantics.

## Recommendation

Test O1 first because it preserves one deep transaction Interface and limits OS
scripts. Compare O2 only if O1 cannot meet atomicity/durability/privilege on a
supported host. Do not choose exact package tools before source/license/supply
and one-to-one maintenance review. Confidence: medium.

## Disposition

- State: `open`; exact OS images, package formats, paths, APIs, filesystems,
  privileges, tools, and limits are unselected.
- Required before the proposal can be promoted to ADR-0015 and before S7.2.
- Generated packages, VMs, signing material, state, and evidence remain outside
  Git.
