---
status: accepted
date: 2026-11-23
---

# ADR-0131 — Distribute Authority Custody separately from Endpoint

## Context

ADR-0130 includes the offline Authority Custody program in every Endpoint
bundle. Enrollment checks its bytes; the Endpoint never launches it. Service
and Admission Authorities require an offline operator with a separate Vault.
The Product Owner accepted separate delivery in
[#563](https://github.com/dianabuilds/ardents-network/issues/563#issuecomment-7098533235).

## Decision

The maintained Endpoint descriptor is `ardents-closed-alpha-enrollment-v6`.
It has the v5 descriptor fields and omits Custody from the exact inventory.
Its artifact is the canonical platform-named Endpoint program and its trusted
root is a canonical versioned root metadata file. The protected group retains
the existing signed grammar and inventory under ADR-0119. The Endpoint archive
and Debian payload refuse additional files. Enrollment checks the independent
manifest pin and actual executable and rejects v5 in the v6 command. The
headless archive without the protected group carries no protected-launch
authorization.

Distribute `ardents-custody` as a separate, versioned release artifact for the
offline Authority operator. Before first execution, the operator checks the
exact platform executable against a SHA-256 digest and release identity from
the authenticated release owner through a channel independent of the download.
The artifact contains only the program. Custody retains its own encrypted Vault,
ceremonies, purposes and floors. An Endpoint enrollment pin grants no Custody
provenance or signing privilege.

Preserve a v5 predecessor as its exact installed generation. For an installed
transition, use the retained Release history and the authenticated, cleanly
stopped original unit, worker sockets and process tree. Stage a complete v6
successor in a distinct immutable root under strictly newer authenticated
Release authorization. Reuse the original mutable roots by identity. Activate
only the complete successor after installed checks. A mixed inventory, lost
predecessor proof, or stale authorization refuses. A failed transition retains
the predecessor and all committed floors; recovery uses authorized forward
generations and cannot lower those floors. The initial enrollment pin cannot
authorize this transition.

This supersedes ADR-0130's inventory and descriptor decision for new builds.
Its retired Control and ACA1 entries stay retired. The migration requires
separate native evidence of installed predecessor continuity, causal refusal
and successful v6 startup.
