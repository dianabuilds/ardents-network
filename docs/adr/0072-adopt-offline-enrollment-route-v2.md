---
status: accepted
date: 2026-09-05
extends: ADR-0071
partially-supersedes: ADR-0025; ADR-0026; ADR-0027
research: R-145
---

# ADR-0072 — Adopt offline-enrollment Route/Entry v2 for C0

## Context

ADR-0071 selects recipient-bound offline enrollment, but the selected
`ardents-interactive-route-v1` profile has a bearer Entry Invite and Binding.
It cannot prove possession of an enrolled recipient key. Its Source TLS path
also verifies certificates against the process wall clock, while C0 needs a
separate challenge-bound time witness before first Source contact. Finally,
the four existing commands have no owner for an offline enrollment approval
journal and exact sealed response.

## Decision

C0 replaces Route/Entry v1 with one State-selected
`ardents-interactive-route-v2` profile. Its new canonical Entry Invite and
EntryBinding commit to an enrolled Entry-recipient public key and finite
generation; the Endpoint proves possession on the completed TLS channel before
the Initiator allocates an attachment. V1 Invite/Binding bytes, magic, ALPN,
or peer-selected negotiation are never accepted, translated, or used as a
fallback in the v2 journey. The selected TCP/TLS and QUIC Carrier set is
unchanged.

Add an offline-only `ardents-enrollment` command artifact. It owns one
exclusive finite enrollment approval/response journal and a distinct offline
time-witness key root. The enrollment and time-witness keys are separate and
neither is a State, Route, Entry, Source, Release, Target, Transit, or Service
Credential authority. The command commits only an independently approved exact
request digest and immutable sealed response; it never holds an Entry signer,
Source CA, Endpoint private key, Service Authority, or automatic password
input.

State owns time-confidence verification, interval arithmetic, durable replay
floor, and restart refusal. A witness response must bind a fresh Endpoint
challenge and bounded response interval; delivery is valid only in the same
boot-local monotonic request window. State supplies Source its bounded
verification-time port for TLS certificate validation. Source has no
system-clock fallback and remains responsible for CA, hostname, leaf-pin, and
expiry checks. Restart requires a fresh witness request and response.

The enrolled artifact inventory carries the fifth command and its fixed public
time-witness trust key through the authenticated enrollment path. Entry and
Source issue/apply only their own independently authorized receipts; the
enrollment command may reconcile those exact receipts but cannot manufacture
them. The Endpoint derives private roots and runtime facts from an accepted
response rather than accepting an operator-authored runtime JSON plan.

## Consequences

- C0 gains one additional offline artifact/command role and a new closed
  Route/Entry profile. Existing v1 persisted or wire evidence is historical
  provenance, not a supported C0 migration input.
- The implementation must define bounded canonical request, receipt, witness,
  Invite, Binding, and proof grammars; preserve all existing TLS and State
  gates; and prove copied response, wrong key, stale witness, delayed receipt,
  restart, v1 downgrade, and crash/reopen refusal before allocation.
- No standing enrollment or time service, public registration, anonymous
  bootstrap, automatic Source renewal, automatic Service Credential issuance,
  or availability claim follows from this decision.
- The C0 operator journey changes only after the new artifact, command,
  package ownership, documentation, and maintained command/process tests are
  implemented together.
