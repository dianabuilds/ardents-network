---
id: R-030
title: What is the first honest H3 real multi-node Route tracer?
status: decided
owner: Product Owner
started: 2026-08-13
reviewed: 2026-08-13
---

# R-030 — Real Multi-Node Route

## Decision this unlocks

Authorize and freeze Horizon 3 Stage 2 under the exact
[implementation brief](../../development/horizon-3-stage-2-brief.md): one endpoint-chosen, real
multi-process Route built from current authenticated eligible Node material,
carrying an unpredictable canary without a direct or shortened fallback.

This record also resolves the promotion-gate conflict left by R-029. The
Product Owner explicitly decided that the current local Stage 1 `short` pass
and a green `make check` establish development readiness for Stage 2. They do
not establish official Stage 1 qualification.

## Current contract

ADRs 0004 and 0005 require authenticated control roots, endpoint-local
selection, distinct identity/family/Role Domain positions, bounded exposure,
and fail-closed exclusions. R-001 fixes the narrow role-local knowledge claim;
R-004 retains the split multi-position candidate shape without selecting a
production implementation. R-029 provides authenticated Network State and real
Node lifecycle mechanics, while its role probe is not a Route hop.

The local Stage 1 `short` result is `pass`: 643 one-second external samples,
quiescence pass, cleanup pass, evidence digest
`a214806c7725f14fc99999739e8ae27c6f8c528ff9268e22fcd2be02f3c9f0c4`.
Repository unit, architecture, race, staticcheck, and govulncheck gates pass.
No official Ubuntu Stage 1 result set exists. The official Ubuntu `short`, a
current `churn-2h`, and an independent `unattended-24h` remain mandatory before
the final integrated H3 verdict or any stronger external, security, privacy, or
release claim. Earlier two-hour output from a superseded harness is diagnostic
only.

## Hypotheses

- **H1:** a Client can select four distinct eligible Role Domain positions from
  one authenticated Candidate View, create a real process-to-process Route, and
  deliver a fresh 32-byte canary exactly while no ordinary Node receives both
  Endpoint and Publisher binding or plaintext canary bytes.
- **H2:** the tracer needs a narrower multi-position topology or a different
  replaceable carrier Adapter while preserving the same Route contract.
- **H0:** the outcome requires shared full-route state, direct/short fallback,
  unbounded resources, or a premature production foundation.

## Frozen tracer contract

### Observable outcome

The receiving Publisher reports the exact fresh canary length, SHA-256 digest,
and bytes sent by the Client. The Client derives every selected Node from a
single current, non-conflicting authenticated Network State generation. A
selected result is valid only when all four positions completed.

### Controlled process topology

| Process | Responsibility | May know |
|---|---|---|
| E / Client | obtain Network State, enforce exclusions, select the complete Route, generate the canary | complete selected Route and Publisher test credential |
| NI | initiator position | E connection and the selected Introduction neighbor |
| NX | separate Introduction-domain position for this tracer | NI and the selected Rendezvous neighbor |
| NR | Rendezvous position | NX and the selected Responder neighbor |
| NS | Responder position | NR and the Publisher attachment |
| P / Publisher | receive the end-to-end protected canary | NS attachment and canary; never the complete Route |
| H / verifier | launch/observe the bounded test and recompute evidence | complete synthetic fixture and observations; management only, never a data-path proxy |

NI, NX, NR, NS, P, and E are separate OS processes with distinct keys,
listeners, cancellation, and evidence. This controlled same-host development
topology proves process separation, not host/operator independence.

### Selection and trust boundaries

Each Candidate record is Node-signed, declares laboratory capability `2`, and
is included in a threshold-authenticated `h3-route-tracer-v1` current Candidate
View. Stage 1 `h3-role-probe-v1` records are incompatible and fail closed. The
Client selects exactly one candidate assigned to
each of `initiator`, `introduction`, `rendezvous`, and `responder`. Selected
Node identities, signing keys, endpoints, and declared families are all
distinct. Direct-Origin Source identities/families and caller-supplied excluded
identities/families/domains are ineligible. Missing capacity in any position
fails; no position is removed, reused, or substituted silently.

The first adapter uses TLS 1.3 over literal TCP addresses with exact Ed25519
public-key pins taken from authenticated state. The Client telescopes one
authenticated layer at a time. Each Node receives only its own duty and the
next selected identity; inner setup and canary bytes remain opaque to earlier
positions. A final independent TLS layer protects Client–Publisher bytes from
all Nodes. DNS, proxy environment, ambient-network discovery, session tickets,
early data, and alternate dialing are disabled. This is a replaceable
laboratory Carrier Adapter and encoding, not Ardents transport selection.

### Finite bounds

- exactly four Route positions and one canary operation per tracer attempt;
- `32` unpredictable canary bytes from `crypto/rand`, maximum data frame
  `64 KiB`, maximum control frame `4 KiB`;
- literal endpoints only; one dial per selected leg and no retry/fallback;
- `5 s` per setup/handshake, `15 s` total operation, `5 s` drain/shutdown;
- at most `8` accepted connections and `2` copy goroutines per process;
- no persistent queue, resumption state, DNS cache, temporary credential file,
  or live Route restoration after restart;
- all listeners, connections, timers, goroutines, and child processes are owned
  by the attempt and terminal after cancellation.

### Evidence and result

The bounded evidence schema records manifest/source/build identity, authenticated
generation/Epoch/View digest, exclusions, selected role/Node/family commitments,
per-process PID and role-local peer observations, TLS pin decisions, canary
length/digest comparison, deadlines, terminal process status, listener/socket
cleanup, and an evidence digest. Raw private keys and reusable credentials are
never retained.

- `pass`: the frozen attempt is valid and every selection, process, role-local
  knowledge, byte-integrity, negative, cancellation, and cleanup conjunct passes;
- `fail`: the harness and evidence are valid but candidate behavior violates a
  frozen conjunct;
- `invalid`: manifest, fixture, process observation, or evidence integrity is
  insufficient to judge the candidate.

Evidence truncation, mutation, missing process observation, or digest mismatch
is `invalid`; candidate wrong bytes, wrong identity/domain/family, direct/short
path, forbidden fallback, or leaked full binding is `fail`.

## Evidence plan

### Primary sources

Accessed 2026-08-13: RFC 8446 (TLS 1.3); Go `crypto/tls`, `crypto/ed25519`,
`crypto/rand`, and `crypto/sha256` documentation; accepted R-001, R-004, R-023,
R-027, R-028, and R-029 records. Standard-library use avoids a new runtime
dependency. TLS supplies reviewed primitives; the project implements framing
and bounded orchestration, not cryptographic primitives.

### Experiment and mandatory cases

Freeze one deterministic Candidate View fixture before candidate behavior, then
run real child processes. Required cases are: positive exact canary; wrong Node
identity; same/wrong family and wrong Role Domain; direct and shortened Route
rejection; malformed, partial, oversized, and slow frames; cancellation during
setup and transfer; bounded shutdown and quiescence; evidence truncation,
mutation, and missing-observer `invalid` cases. H recomputes selection and the
terminal result without importing a candidate validator.

### Falsification criteria

Redesign or stop if any result needs a direct Client→Publisher connection,
fewer than four distinct positions, Node reuse, DNS/proxy/ambient fallback,
full-route disclosure to a Node, Node-visible plaintext canary, unbounded
goroutines/connections/queues/timers, hidden H data forwarding, H2 lab imports,
custom cryptography, or a selected production transport/wire/deployment
foundation. A pass is impossible when any mandatory evidence is absent.

## Non-goals and honest limitations

This tracer does not implement the Stage 3 Service Connection/Application
Interface, Service Target or Instance binding, reconnect/same-connection
recovery, Bridge, Service Names, installer/update, public admission, production
capacity, Windows, cross-host performance, anonymity, decentralization,
operator independence, or resistance to a host/global observer. The Publisher
test credential is a fixture, not a Service Instance Credential. Four processes
on one host are not four independent operators.

## Recommendation

Choose H1 and implement the smallest real slice behind existing Network State
and a new cohesive Route Module. Keep the carrier private and replaceable. Do
not change or delete Stage 1 tests. Confidence is medium: the strongest risk is
that nested laboratory TLS may expose a false replacement seam; the stop test
is whether Route selection and role-local contracts remain independent of its
addresses and framing.

## Disposition

- Explicitly accepted by the Product Owner on 2026-08-13; state `decided`.
- Stage 2 development is authorized; Stage 1 official qualification is not
  declared complete.
- Deferred gates: official Ubuntu `short`, current `churn-2h`, and independent
  `unattended-24h`, all before the final integrated H3 verdict or stronger claim.
- No ADR: the decision is a bounded, replaceable tracer and creates no accepted
  production technology lock-in.
- Generated keys, captures, process files, and evidence stay outside Git.
