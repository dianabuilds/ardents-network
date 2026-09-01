---
status: accepted
date: 2026-09-01
extends: 0061-retain-firefox-entry-as-compatibility-evidence.md
---

# ADR-0069 — Retire the active Browser implementation and qualification lanes

## Context

Track A findings A-BR-01 through A-BR-07 identified an active Browser
implementation, artifact lane, and platform qualification surface that no
longer belongs in the maintained product contract. ADR-0061 already requires
the Firefox/Endpoint material to remain as immutable compatibility evidence;
that evidence must not keep an executable Browser product surface alive.

## Decision

Remove the maintained Browser commands, Browser Modules, Browser packaging,
Browser command inventory, and Browser qualification lanes. The maintained
artifact set is exactly `ardents`, `ardents-node`, `ardents-control`, and
`ardents-custody`. The maintained product surface is headless Network plus the
neutral Application Interface v1 and its Network-owned Endpoint
implementation.

Retain `tests/compatibility/browser-endpoint-v4`, accepted Browser ADRs and
research records, and immutable audit receipts as non-executable historical
evidence. Retained evidence does not enter Go package discovery, build or
qualification inventories, ownership lanes, or current product claims.

This decision changes no Network implementation, wire or persisted identity,
transparent-origin behavior, Track B material, qualification execution, or
VPS operation. It changes only the architecture, ownership, build-profile,
artifact-representation, and documentation seams needed to make the
retirement factual.

## Consequences

- `make quick-check` and `make headless-check` cover the maintained build and
  artifact surface; there is no active Browser build, check, or qualification
  target.
- The neutral Application Interface remains a maintained seam, but no Browser
  client or presentation implementation is selected behind it.
- Historical Browser bytes remain available for compatibility review without
  being executable current code or evidence for a changed candidate.
- Any future Browser or desktop surface requires a new Product Owner decision,
  decision-relevant research, a bounded threat/resource contract, and an ADR.

## Compliance

The current package map, ownership registry, profile registry, Makefile, and
architecture tests contain only the four maintained command artifacts and the
neutral Application Interface. The retained compatibility tree remains
outside those current inventories under ADR-0061.
