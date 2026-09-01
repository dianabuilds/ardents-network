---
status: accepted
date: 2026-08-30
extends: 0062-scope-online-transit-grant-signing.md
---

# ADR-0063 — Bootstrap each Transit Grant issuer from an owner-only root

## Context

ADR-0062 separates online Transit Grant signing from Network State authority,
but the maintained issuer still receives its purpose signer and permitted
Initiator as caller-supplied runtime configuration. It also creates its OHTTP
material only after current State already contains the resulting public
profile. That cycle has no supported operational owner and prevents an
artifact-native participant from acquiring a Grant without a fixture.

## Decision

An explicit local `ardents-node` initialization operation creates one immutable
Transit Grant issuer generation in a new owner-only issuer root. The root owns
the generated purpose-scoped Grant signer and OHTTP private material. The
operation atomically retains and returns only one Node-signed public issuer
profile; reopening the completed root returns byte-identical public profile
bytes and never returns either private key. An interruption before publication
leaves no claimable generation, while an interruption after publication is
recovered by reopening that same generation.

The public profile declares exactly one permitted Initiator Node identity and
public key in addition to ADR-0062's issuer, signer, OHTTP, and terminal-bound
facts. Network State binds that Initiator by authenticating the exact opaque
profile bytes under the sole selected `transit-issuance` duty. The online
runtime additionally proves that the declared Initiator is the one current
State candidate assigned to the `initiator` domain. It never selects from plan
order and never accepts every current Initiator.

The supported issuer runtime receives only the issuer-root location, normal
State acquisition ownership, listener credentials, and finite local resource
bounds. It may serve only when current authenticated State selects the same
issuer Node, contains the byte-identical public profile, and retains the exact
profile-declared Initiator assignment. The runtime plan cannot supply or
replace the Grant signer, OHTTP key, public profile, or permitted Initiator.
Network State and Epoch private keys never enter initialization or runtime.

The first accepted State duty irreversibly binds the root's durable finite
budget and idempotency ledger to that exact State digest, Epoch, issuer, signer,
and terminal bound. A State successor, authenticated withdrawal, expiry, or
profile/Initiator mismatch stops new issuance and terminates the old duty. A
replacement or rotation is an explicit initialization into a distinct empty
issuer root followed by a new State ceremony; an existing root is never
repurposed or replenished.

[ADR-0068](0068-bind-transit-issuer-roots-to-state-generation.md) further
requires independent canonical State-generation continuity and the v2
owner-root format. It rejects every v1 root without migration or mutation.

## Consequences

- Bootstrap is a deliberate two-phase owner operation: initialize and publish
  the public profile, then start only after State authenticates it.
- Compromise of the online root exposes only that finite issuer generation. It
  neither exposes nor grants use of a Network State root key.
- The chosen Initiator and issuer can correlate requests, withhold results, or
  exhaust the shared duty budget. This decision adds no anonymity or public
  availability claim.
- The issuer profile grammar advances, but Transit Grant v1, Credential Relay,
  Route, Target, Service, and Application Interface semantics remain unchanged.

## Compliance

[R-130](../research/records/r-130-transit-issuer-bootstrap.md) records the
Product Owner decision and rejected alternatives. ADR-0062 continues to own
signer scope, fixed encrypted outcomes, and Endpoint at-most-once lifecycle.
ADR-0053 continues to own the separate Network State authority root.
[R-135](../research/records/r-135-transit-issuer-generation-continuity.md)
records the follow-up continuity decision.
