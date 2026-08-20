# Stage 7 decision proposal — Bind claim-bearing Applications at launcher time

Status: **accepted on 2026-08-20 and recorded in
[ADR-0016](../adr/0016-bind-and-isolate-launcher-born-application-principals.md).**

R-051 has advanced O2 to an exact
[Application Principal specification](stage-7-application-principal-spec.md).
That candidate is accepted for Stage 7 development under ADR-0016. Development
evidence is limited to Ubuntu 26.04 Docker and the current Windows 11 machine;
unavailable native controls remain explicitly deferred rather than qualified.

## Context

R-024 and ADR-0007 require an OS-enforced or launcher-brokered Application
Principal for one complete Application/helper process tree. A desktop user,
PID, loopback port, named path, or copyable bearer cannot distinguish hostile
same-user Applications and cannot prevent direct network escape.

Accepting an arbitrary process at a shared socket and then inspecting its PID or
token is vulnerable to PID reuse, process replacement, bearer theft, inherited
handles, pipe/socket substitution, and incomplete helper coverage. Conversely,
forcing all Applications into one sandbox would turn an optional privacy profile
into a universal Application runtime and overstate compatibility.

## Proposed decision

Adopt the following architecture if the Product Owner accepts this proposal and
R-051/R-052 validate one candidate per supported platform:

1. The **Application Broker Module** creates a fresh Application Principal and
   short-lived session before launching claim-bearing work. Later attachment is
   generic only. The Broker binds the Local Grant, broker start identity,
   process-tree owner, exact local channel, Isolation Context, operations,
   resources, and deadline.
2. The **Application Isolation Module** is a separate Module. It launches the
   same principal under a named platform profile and returns only a bound local
   channel and observable containment result to the broker.
3. Claim-bearing profiles are launcher-born. The broker creates the process in
   the OS containment boundary before untrusted code runs, closes unrelated
   inherited handles, and owns the complete non-breakaway process tree through
   termination and cleanup.
4. A bearer or protocol capability is defense in depth, not a principal. It is
   valid only on the already authenticated channel and never survives broker,
   Application, or Endpoint restart.
5. The **generic attachment profile** may accept a coarser platform principal.
   It is visibly `application-networking-unverified` and receives neither a
   malicious-sibling isolation claim nor an Application-level Endpoint Location
   Privacy claim.
6. The **network-isolated profile** must prove deny-by-default ordinary network
   ingress/egress, scoped local IPC only, process-tree coverage, per-context
   storage, revocation, restart rebinding, and complete cleanup. Unsupported
   Applications fail explicitly; there is no silent generic fallback.
7. The same executable's direct-binary Adapter remains a first-class
   per-invocation path in Installed and Portable. With no external IPC peer it
   reports claim `none`; it does not invent self-authentication or require
   desktop/browser installation. An external Broker hop must separately satisfy
   the generic or launcher-bound principal contract.

## Consequences

- Authentication and network confinement remain separate deep Modules: neither
  Interface implies the other's guarantee.
- Ubuntu and Windows are real platform Adapters, not compile-time names for
  unequal claims. Each must report the same bounded outcomes and pass the same
  hostile matrix.
- The exact R-051 O2 candidate uses a private inherited channel plus pidfd and
  atomically seeded cgroup-v2 tree on Ubuntu, and inherited anonymous pipes plus
  suspended root, stable process handle/token facts, and a non-breakaway Job on
  Windows. Named sockets/pipes remain generic only. These are candidates, not
  decisions.
- Windows AppContainer and Linux network/storage confinement combinations remain
  R-052 candidates. Experimental Windows sandbox Interfaces, first-party kernel
  drivers, cgo, and custom syscall-filter implementations are not selected.
- The current `x/sys/windows` Job-limit call requires a fixed-structure
  `unsafe.Pointer` bridge. ADR-0016 must either reject the candidate, identify a
  maintained safe surface, or explicitly authorize only that minimal bridge
  with layout/lifetime/race/failure/cleanup tests. No exception is implied by
  this proposal.
- A same-user Application that cannot be launcher-bound belongs to one coarse
  local trust domain and can use only the generic profile.
- Isolation does not sanitize Application content, hide credentials or timing,
  resist endpoint compromise, or turn arbitrary code into an anonymous
  Application.

## Compliance and acceptance

- [R-048](../research/records/r-048-h3-stage-7-contract.md) records the research
  sources, alternatives, and current assumptions.
- [ADR-0007](../adr/0007-separate-carrier-privacy-from-application-egress.md) remains
  authoritative for the claim boundary.
- [Stage 7 lifecycle specification](../development/stage-7-lifecycle-spec.md)
  defines generic and isolated profile outcomes.
- [Application Principal specification](stage-7-application-principal-spec.md)
  freezes the O2 mechanism and hostile acceptance envelope without accepting it.
- The Product Owner accepted O2 and the two R-052 native profiles on 2026-08-20,
  including only the exact bounded Windows `unsafe.Pointer` bridge and its
  dedicated risk tests. Native facts unavailable on the development surfaces
  remain qualification gates rather than synthetic passes.
- R-051/R-052 select exact stable OS mechanisms and may reject this proposal if
  a platform cannot meet it; they cannot downgrade an isolated failure to a
  generic success silently.
