---
status: superseded
date: 2026-08-20
superseded_by: R-085 generic Broker scope
---

# ADR-0016 — Bind and isolate launcher-born Application Principals

> Superseded for the maintained Stage 8 path by the generic/unqualified Broker
> contract. The selected platform profiles were never
> promoted; current code exposes only explicit generic/unqualified Broker
> admission. This record remains historical evidence for any future qualified
> isolation decision.

## Context

A desktop user, PID, path, named endpoint, loopback port, or copyable bearer
cannot distinguish hostile same-user Applications or cover their complete helper
tree. Treating generic browser/application attachment as isolated would
overstate Endpoint Location Privacy, while forcing every Application into one
runtime would unnecessarily narrow compatibility.

## Decision

The Application Broker creates every claim-bearing Application Principal before
untrusted work starts and jointly binds a fresh private inherited channel,
non-reusable root process handle, complete non-breakaway process tree, Local
Grant, Isolation Context, operations, resources, and deadline. Named/later
attachment remains generic and visibly `application-networking-unverified`.

The separate Application Isolation Module uses
`ubuntu-bwrap-native-v1`—non-setuid bubblewrap v0.11.2 inside the declared
cgroup-v2/pidfd tree—on Ubuntu, and
`windows-appcontainer-native-v1`—an ephemeral zero-network-capability
AppContainer with explicit handles and context ACLs inside a suspended
non-breakaway Job—on Windows. Both profiles must deny ordinary ingress/egress,
preserve only scoped IPC/storage, bind descendants, revoke on restart, and clean
up completely. Unsupported isolation fails explicitly without generic fallback.

ADR-0016 authorizes one Windows-only first-party `unsafe.Pointer` bridge solely
for the fixed `SetInformationJobObject` limit structure required by the selected
Go surface. It is not a general `unsafe` exception: exact Windows/Go structure
layout and size, pointer lifetime, race behavior, failure handling, Job
membership, descendant coverage, and cleanup require dedicated tests. No
pointer is retained beyond the synchronous call.

Direct binary use remains first-class in Installed and Portable. With no
external principal it has claim `none`. Generic default-browser handoff carries
no isolation claim; Stage 7 isolated-browser requests return
`isolation-unsupported`.

## Consequences

- Principal authentication and network confinement remain separate deep
  Modules and neither Interface implies the other guarantee.
- First-party kernel drivers, custom sandbox machinery, cgo, firewall mutation,
  loopback exemptions, system proxy/DNS/route/VPN changes, and silent fallback
  are excluded.
- Docker/current-Windows development evidence supports only independently
  observable facts. Native Ubuntu Desktop/kernel and pristine-Windows
  qualification remain explicit deferred gates, not passes.
- A platform failure removes its claim-bearing isolated profile or reopens the
  product contract; generic success cannot hide it.

## Compliance

- [ADR-0007](0007-separate-carrier-privacy-from-application-egress.md) remains
  authoritative for Application-level location claims.
