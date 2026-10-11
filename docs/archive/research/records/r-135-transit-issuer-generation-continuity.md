---
id: R-135
title: Does a Transit Grant issuer root need an independent State-generation continuity binding?
status: decided; promoted to ADR-0068
owner: Product Owner and Codex
started: 2026-09-01
reviewed: 2026-09-01
---

# R-135 — Transit issuer State-generation continuity

## Decision this unlocks

Decide whether an owner-only Transit Grant issuer root may reopen or issue when
authenticated State changes only its canonical generation, and whether the
existing v1 root has a supported migration path.

## Current contract

ADR-0062 scopes online Grant signing away from State authority. ADR-0063 binds
one root to one first State duty, exact issuer profile, Initiator, budget, and
terminal bound. Transit Grant, Profile, Request, and Outcome grammar are
already accepted contracts and are outside this question.

## Hypotheses

- **H1:** Treating canonical State generation as an independent root-scope
  discriminator prevents an otherwise indistinguishable successor from using
  the old finite ledger.
- **H2:** Digest, Epoch, and profile equality alone establish safe continuity.
- **H0:** Neither binding has a fail-closed replacement/recovery lifecycle.

## Evaluation criteria

- A generation-only successor is unavailable before any ledger lookup,
  withdrawal, or reservation.
- The exact same generation reopens without changing the public profile or
  ledger, while every different generation fails before a Handler is exposed.
- A canonical generation is exactly lowercase 64-hex; malformed values fail
  closed.
- v1 marker-only, unbound, and bound roots retain byte-identical contents when
  rejected, including no lock or staging entry.
- The decision adds no protected-data disclosure, wire field, dependency,
  operator, or recovery owner.

## Evidence plan

Primary sources inspected 2026-09-01: ADR-0062, ADR-0063,
`internal/route/credential/{contract,issuer,root_issuer,issuer_root_store}.go`,
and `internal/node/transit_issuer_duty.go`. The reproducible measurement is
`go test ./internal/route/credential ./internal/node`: it opens/reopens a root,
changes only State generation through the public issuer Handler, checks the
durable root bytes, and exercises marker-only/unbound/bound v1 refusal.

Failure scenarios are a State successor preserving digest/Epoch/profile,
malformed generation, v1-root substitution before or after lease acquisition,
and a root copied from another ceremony.

## Findings

- **Measurement:** prior scope equality did not include State generation, so a
  generation-only successor reached the live issuer scope.
- **Sourced fact:** Node already receives State Generation independently from
  digest and Epoch in its authenticated duty view.
- **Inference:** using generation as a continuity discriminator is smaller
  than changing a protected wire grammar and makes successor handling explicit.
- **Measurement:** v1 roots can be classified before lease acquisition from
  their marker or JSON version, so no migration mutation is needed.

## Recommendation

Choose H1: persist and compare canonical State generation at every issuer
scope boundary; make v2 a hard root-format boundary with no v1 migration.
Confidence is high. The strongest contrary argument is operational replacement
cost for a v1 root, but an implicit migration would mutate owner-only material
without a selected recovery authority.

## Disposition

The Product Owner accepted H1 on 2026-09-01. ADR-0068 owns the consequential
continuity and root-format decision. No experiment directory or new dependency
is required; the retained credential/node behavior tests are the evidence.
