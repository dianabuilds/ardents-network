---
status: accepted
date: 2026-09-22
---

# Retire old Node starts while preserving owned shutdown

## Context

The maintained candidate has separately authorized closed Route duties, but
old Node roles, the old Source selector, Transit issuer, and dedicated
Rendezvous Contributor can still accept new work. Removing all Contributor
control together with those starts could strand an active owned installation
or force unsupported manual host mutation. Conversely, existing interrupted-
update recovery can Start or Restart old executable bytes even before a
diagnose, drain, withdraw, or remove action.

Existing old roots, keys, floors, installation records, and historical profile
identity are also retained evidence. Their presence is not authority for a new
start or an automatic migration.

## Decision

Retire new old Node-role, Source-profile, Transit-issuer, and Contributor
apply/restart starts. Each accepting adapter must refuse its recognized old
selection before the first root, key, listener, supervisor, or Network effect,
with no fallback to a closed path.

Keep the selected closed Node duties, closed Source profile, and closed issuer
initialize/serve unchanged. Keep Contributor diagnose, drain, withdraw, and
confirmed remove for an exactly authenticated owned installation, but no such
operation or its recovery may Start, Restart, Enable, or complete an update by
running old bytes. Ambiguous state fails explicitly and retains evidence.

Historical profile recognition may authenticate existing pinned evidence for
retirement; it cannot authorize new execution and does not rewrite persisted
identity. Do not inherit a duty, regenerate keys, reset roots or floors,
convert wire/state, or adopt foreign files.

## Consequences

- A verified owner can still observe, stop, disable, and remove only the
  managed installation without keeping old execution authority alive.
- An inactive old installation is never started merely to drain or diagnose
  it. Removal retains exact deployment confirmation and inactive, disabled,
  withdrawn preconditions.
- Old engines and compatibility readers are removed only in later bounded
  slices after accepting callers and remaining consumers are proved absent.
- A crash-interrupted update may require an explicit operator-visible failure;
  automatic availability is subordinate to the no-new-old-start boundary.

## Compliance

The [Network/Node retirement contract](../technical/network-route-node.md#old-start-retirement)
owns the exact selector and effect boundary. The
[Contributor runbook](../reference/rendezvous-contributor.md#selected-retirement-transition)
owns the retained no-start operator actions and residue boundary.
