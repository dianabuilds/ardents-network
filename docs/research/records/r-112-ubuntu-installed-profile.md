---
id: R-112
title: Ubuntu Installed Endpoint profile
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-112 — Ubuntu Installed Endpoint profile

## Decision this unlocks

Select the first H4-1D Ubuntu `.deb` delivery shape and its smallest supported
install, upgrade, repair, and uninstall contract. It must not select Windows,
a package repository, an automatic updater, a system-wide runtime, or a Public
Beta supply-chain claim.

## Current contract

H4-1A is the selected Ubuntu `x86-64` Portable alpha journey. H4-1B owns an
explicit unprivileged local replacement with Release authorization; H4-1C keeps
Authority Vault and floors outside program bytes. An Installed profile wraps
the same authenticated Endpoint rather than introducing another runtime.

The initial alpha distribution channel may be GitHub Releases. Package-store
signatures are additional transport evidence, not the Ardents Release root.
The normal Endpoint must remain a per-user `systemd --user` process without
linger; package installation may require the ordinary explicit package action
but may not create a root daemon or silently enable a user unit.

## Hypotheses

- **H1:** A directly downloaded Ubuntu `.deb`, externally checked before
  `dpkg` execution and then Release-verified by the installed Endpoint, can
  install only program-owned bytes under a package path while retaining all
  per-user state through upgrade, repair, and uninstall.
- **H2:** A signed APT repository is required even for the first Installed
  alpha, and therefore selects a new repository key/cadence/operator surface.
- **H0:** Neither option can meet H4-1 without privileged runtime ownership or
  an unsafe removal path; keep Portable as the sole Ubuntu profile.

## Evaluation criteria

- Package contents contain no Authority, Local Grant, Instance Key, Release
  floor, cache, runtime socket, or user-specific enrollment input.
- Install/upgrade/uninstall never start, enable, linger, or administer a
  system or user Endpoint service implicitly.
- Package removal affects only package-owned program bytes; Authority Vault,
  authority watermarks, and release floors survive.
- The first executed package path has independently delivered digest/provenance
  evidence, and successor program bytes still require Release authorization.
- The selected shape is maintainable by the current one-person project team
  without a hosted package repository or paid signing service.

## Evidence plan

Read current Debian Policy and `dpkg` maintainer-script documentation, Ubuntu
package-install and removal behavior, and systemd unit installation guidance.
The first H4-1D qualification is a reproducible Linux package-lifecycle process
fixture: build two distinct real package artifacts, install, re-enroll, remove,
and purge them with `dpkg --root` and a test-owned package database, then run
each installed direct executable as an unprivileged UID with distinct program
and XDG state roots. Record exact package files, Vault/floor state, and every
maintainer-script action. A native Desktop installation and user-session
walkthrough remain a later release-promotion gate; they are not silently
substituted by this process evidence.

## Findings

- **Sourced fact:** Debian Policy defines `preinst`, `postinst`, `prerm`, and
  `postrm` as package-manager-executed scripts for install, upgrade, and
  removal; scripts must be idempotent, and failure can leave a package
  half-configured or half-installed. [Debian Policy §6](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html)
  (accessed 2026-08-25).
- **Sourced fact:** package removal removes package files before `postrm`; a
  purge also removes conffiles and then calls `postrm purge`. A package that
  has neither conffiles nor `postrm` is automatically purged on removal.
  [Debian Policy §6.8](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html)
  (accessed 2026-08-25).
- **Sourced fact:** Debian packages must not install new files beneath the
  merged-/usr alias paths such as `/bin/*` or `/sbin/*`; `/usr/bin` is the
  ordinary binary location. [Debian Policy §10.1](https://www.debian.org/doc/debian-policy/ch-files.html)
  (accessed 2026-08-25).
- **Inference:** the smallest H4-1D package should have no maintainer scripts,
  conffiles, system unit, user unit, or home/XDG payload. It can therefore
  neither enable/stop a service nor erase a Vault/floor through package
  lifecycle. It should own only an immutable versioned program directory under
  `/usr/lib/ardents/` plus a non-secret `/usr/bin/ardents` launcher.
- **Inference:** a direct `.deb` checksum alone cannot replace H4-1A's
  independently pinned first-execution enrollment and Release Decision. The
  package needs a bounded package-enrollment representation that binds the
  installed executable and its static Release inputs before H4-1D can be
  called functional; copying the Portable bundle verbatim into `/usr` without
  a new verifier would be an unexamined second artifact shape.
- **Measurement:** the maintained H4-1D process fixture built a real Linux
  `ardents` executable into the direct `.deb`, used `dpkg --root` with a
  test-owned package database to install, upgrade to a distinct v2 program and
  TUF/Release metadata set, remove, and purge it, and ran each installed direct
  executable as UID 1000. Each package places its static enrollment under a
  versioned child of `/usr/share/ardents/enrollment/`, so dpkg's retained old
  package files cannot enter the exact inventory for v2. The participant's
  explicit v2 enrollment established the new Release-authorized program record.
  The verifier accepted only root-owned non-writable
  `/usr/lib/ardents/ardents` plus root-owned readable static enrollment files;
  every tested package lifecycle step retained the external XDG Vault root and
  Release floor, while removal removed the package program. The fixture does
  not qualify a Desktop user session, package provenance, independent release
  control, or a public release.

## Failure scenarios

- A package or maintainer script starts a privileged or lingering service.
- Reinstall or upgrade overwrites user state, roots, or floors.
- `remove`/`purge` erases Authority or permits a stale Bundle to reactivate it.
- A repository or package signature is mistaken for Release authorization.
- The package manager leaves old executable bytes runnable without the
  H4-1B/current-program binding.

## Options

1. Direct GitHub-distributed `.deb`, no repository, explicit package action.
2. Signed APT repository and normal package-manager updates.
3. Keep Portable only until observed participant friction justifies Installed.

## Recommendation and disposition

Choose option 1 for the first alpha profile: a direct GitHub-distributed Ubuntu
`amd64` `.deb`, no APT repository and no maintainer scripts. This selects only
the package lifecycle shape, not a new Release trust root: an external package
digest/provenance check remains transport/bootstrap evidence, while explicit
package enrollment hands the installed exact bytes to Release Decision before
readiness. The maintained process fixture has exercised the selected lifecycle
and its state-separation boundary. The strongest argument against this choice
is that it adds a package-specific first-enrollment form and does not provide
automatic updates; native Desktop package qualification, repository policy,
package signing, repair across interrupted host versions, Windows packages, and
all public-release supply-chain claims remain out of scope.
