---
status: accepted
date: 2026-09-22
---

# Retire Name operator network adapters without retiring Service Names

## Context

`ardents name resolve` and `ardents name control` still compose the old
HTTP/OHTTP private-resolution runtime even though no maintained production
Gateway or Resolver is selected. Human-facing Service Names remain a product
function over the future protected protocol, but that continuity does not
authorize the old command path. Canonical encoding, Namespace lifecycle and
proofs, and custody also have responsibilities independent of these commands.

## Decision

Retire the recognized `name resolve` and `name control` operator commands at
command dispatch. They must return a deterministic operator-readable refusal
before validating remaining arguments, decoding context, reading files,
opening State, constructing or using HTTP/OHTTP transport, writing output, or
mutating Namespace state. The refusal explains that the old network command is
retired and protected Service Name access is not yet selected; it supplies no
fallback.

Keep `name encode` and its canonical bytes. Keep Naming, Namespace
lifecycle/proofs, custody, and existing persisted evidence unchanged. The
uncomposed resolution module is retained pending a separate exact-consumer
decision; command retirement does not itself authorize its deletion.

Do not select a successor wire, Resolver/Gateway topology, authority,
governance, migration, Target-Link translation, ambient-DNS fallback, or AAI3
Name route. Existing roots, records, journals, floors, and authority material
are neither converted nor treated as successor authority.

## Consequences

- Until a protected successor is separately selected and delivered, there is
  no maintained operator command for network Name resolution or control.
- Service Names remain a product function, not a claim of current operator
  availability.
- Local canonical encoding and separately consumed Namespace/custody behavior
  remain unchanged.
- Removing command-only adapters or the now-uncomposed resolution package is
  later work and requires exact caller/compatibility evidence.

## Compliance

The [Naming owner](../technical/naming.md#operator-name-network-command-retirement)
defines the retained module and state boundary. The
[command reference](../reference/commands.md#ardents) owns the observable
operator result and distinguishes the selected transition from integrated
runtime behavior.
