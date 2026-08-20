---
status: accepted
date: 2026-08-20
---

# ADR-0015 — Separate release decision from versioned local activation

## Context

An OS package signature, mirror, installer, or self-updating executable must not
become Ardents release authority. In-place payload or schema mutation would also
make interruption recovery and rollback depend on platform-specific repair
behavior. Installed and Portable must expose the same runtime product without
creating a second Portable lifecycle stack.

## Decision

One bounded Release Decision Module verifies the accepted TUF-compatible H3
metadata profile, exact artifact identity, platform/environment binding, and
non-decreasing release floors. Distribution provides bytes only; package
signatures are additional channel evidence and cannot authorize Ardents code.

One Update Transaction Module owns immutable versioned payload directories,
atomic activation records, stop-new-work, drain, self-test, commit, safe
rollback, and `repair-required`. Mutable schema migration is copy-on-write until
commit. Authority Vaults, roots, Network Epoch and Namespace state, credentials,
Grants, Endpoint identity, and every monotonic watermark stay outside the
payload and rollback trees.

Thin Install Lifecycle Adapters own Ubuntu `.deb` and Windows WiX v7 MSI package
objects, the stable bootstrap, declared registration, repair, uninstall, and
explicit purge. The Portable Profile is the same authenticated platform
executable plus only unavoidable declared companions; it has no installer,
bootstrap, elevation, implicit registration, or lifecycle Adapter. Portable
replacement is stopped, authenticated, and rechecked before execution.

## Consequences

- Installed and Portable share runtime capabilities, Interfaces, resource
  bounds, protected-state compatibility, and claim ceilings; only managed
  lifecycle convenience differs.
- A failed update may activate only a retained authenticated, compatible,
  non-revoked payload. Otherwise networking stops while bounded repair and
  Authority export remain available.
- The stable bootstrap and activation record are small high-risk artifacts with
  dedicated interruption, durability, ACL/mode, residue, and recovery tests.
- This ADR selects architecture and package formats, not a public release
  authority or a complete native-host qualification result.
- It does not authorize a Windows installation. Windows lifecycle mutations
  still require the Product Owner's separate artifact- and operation-specific
  command.

## Compliance

- [R-049](../research/records/r-049-stage-7-release-verifier.md) selects the
  bounded release-verification client profile.
- [R-050](../research/records/r-050-stage-7-install-update-adapters.md) records
  the platform and Portable evidence and explicit qualification deferrals.
- The [lifecycle specification](../development/stage-7-lifecycle-spec.md) is
  normative for state ownership and transition behavior.
