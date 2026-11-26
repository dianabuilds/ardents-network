---
status: accepted
date: 2026-11-26
---

# Group product domains under internal/domain for human readability

## Context

The Product Owner requires the project to be human-readable first: agents write
the code, but people read it. `internal/` is a developer's first map of the
system. At roughly 106 packages the top level mixes product domains (`route`,
`publication`, `network`) with technical mechanisms (`pathgrammar`,
`canonicaldigest`, `durablefile`) and platform adapters, so a reader must know
in advance which directories name Ardents concepts and which name technical
operations.

A package's place in the tree follows the contract it answers for, not the
number of domains that import it. A domain owns the rules and lifecycle of a
specific Ardents concept; a shared mechanism serves many domains and owns no
product authority. `internal/connection` is a domain (the Service Connection
bounded context retains the identity of one authenticated logical stream, with
explicit exclusions); `internal/processresource` keeps its fail-closed pressure
admission and workload profiles rather than hiding them under a platform label.

## Decision

Adopt a target tree that separates domains from technical groups by name:

- `internal/domain/<name>` holds the product domains: `network`, `route`,
  `publication`, `endpoint`, `admission`, `node`, `custody`, `enrollment`,
  `release`, `hosting`, `reachability`, `execution`, `connection`. Opening
  `internal/domain/` shows every Ardents domain in one place.
- Technical groups keep purpose names at `internal/` top level: `platform/`
  (OS adapters, including `processresource`), `diagnostics/`, `architecture/`
  (engineering checks), `application/` (application-layer contracts), and the
  self-documenting standalone mechanisms `durablefile`, `pathgrammar`,
  `canonicaldigest`.

The path distinguishes `domain/connection` (Service Connection protocol) from
`application/connection` (the local application Interface). A generic
`internal/mechanism/` grouping is rejected: its membership criterion diverges
(pure validation versus file I/O, locks and DACL) and the name predicts neither
behaviour nor permitted dependencies.

This ADR selects the target structure only. The directory migration is a
separate, incrementally scheduled program: one domain per bounded change,
smallest first, each reconciling import paths, the architecture gates, the
deadcode allowlist and the package map. Until a domain moves, the package map
records its class (domain, supporting mechanism, platform adapter, application
contract, engineering check) so the inventory is readable without waiting for
the migration.

## Consequences

Navigation improves: domains are visible as a group and technical packages are
self-named. The migration rewrites about 2114 import sites across 827 files and
updates path-keyed architecture checks; it is mechanical but large, so it is
never applied as one big-bang change. Domain authority, wire bytes, persisted
identities and the Route migration boundary are unchanged; only package paths
and their registrations move. Existing accepted persisted or wire identities
that embed an old path remain compatibility obligations until a researched
migration retires them.