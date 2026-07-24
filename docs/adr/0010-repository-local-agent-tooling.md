# ADR 0010: Repository-local agent tooling

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Engineering, Security, Repository Governance

## Context

The repository contains a security-audit skill under `.agents`, while an older
target-tree statement prohibited all tracked agent tooling. That contradiction
made a clean checkout fail its documented architecture even though the tooling
is deliberately used during security review. Silently deleting the rule or
silently retaining arbitrary `.agents` content would both leave repository
governance fail-open.

The skill is development-time review tooling. It is not linked into Ardents
binaries, copied into deployment bundles, or required by the runtime.

## Decision

Retain repository-local agent tooling only at the exact
`.agents/skills/security-audit/` prefix. The approved skill is
`security-audit`, sourced from `cloudflare/security-audit-skill`, and remains
pinned by `skills-lock.json`.

No other `.agents` path is part of the target tree. Adding or replacing a skill
requires an explicit update to this decision, the architecture acceptance
policy, its fail-closed gate, and the source lock in the same reviewed change.
Deleting the approved skill likewise requires those governance changes before
removing its files.

The architecture acceptance gate verifies the exact allowlist, the single
approved lock entry, its source metadata and hash shape, the locked entry point,
and this accepted decision. A clean checkout therefore cannot broaden, remove,
or relocate repository-local agent tooling without failing static acceptance.

## Consequences

- Security review instructions remain reproducible from a clean checkout.
- Repository-local agent tooling is a narrow governance exception, not product
  or runtime architecture.
- Arbitrary agent instructions, caches, generated reports, and additional
  skills remain forbidden under `.agents`.
- Updating the pinned skill is an explicit reviewed repository change.
