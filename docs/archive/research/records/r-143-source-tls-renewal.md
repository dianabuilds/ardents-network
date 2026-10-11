---
id: R-143
title: How can Direct-Origin Source TLS material rotate and recover after expiry without weakening CA, hostname, or pin verification?
status: decided; C0 uses controlled restart and same-key leaf renewal only
owner: Product Owner and Codex
started: 2026-09-05
---

# R-143 — Source TLS renewal and recovery

## Decision this unlocks

Choose the maintained C0 procedure for Source server/client TLS leaf renewal,
key/CA/pin rotation, and recovery after all current material has expired. The
decision must select restart or atomic reload and state the issuer and recovery
authority. It does not authorize an issuer, ACME, background renewal daemon,
hot reload, trust-map wire change, or implementation.

## Current contract

`cmd/ardents` and `cmd/ardents-node` read bounded operator certificate/key,
root PEM, and leaf-key-pin files while decoding one finite Source plan. Source
`Plan.New` validates and copies all material; `Serve` creates one TLS listener
from that immutable server state. Every new client TLS attempt verifies TLS
1.3, CA chain, server hostname, server leaf key pin, client certificate, and
server-side client key pin. R-02 provides controlled-time handshake evidence
for expiry, unknown CA, hostname, and both pins.

The current product names no Source certificate issuer, no automatic renewal
authority, and no in-process reload seam. Source plans are bounded local
operator inputs, not an ambient configuration or a public certificate
distribution protocol.

## Hypotheses

- **H1:** Explicit replacement of complete locally supplied material followed
  by controlled process restart preserves all current TLS checks and supplies a
  bounded recovery path after expiry.
- **H2:** Atomic in-process reload can preserve valid existing work without
  accepting stale/invalid material or expanding the trust boundary.
- **H3:** A product-selected online issuer can automate Source renewal safely.
- **H0:** No candidate currently meets the C0 owner/verification contract.

## Evaluation criteria

- New connections never accept expired, not-yet-valid, malformed, wrong-key,
  wrong-CA, wrong-hostname, or wrong-pin material.
- A leaf renewal retaining its key changes no pin; a key or CA rotation has an
  authenticated explicit plan and cannot fall back to an old invalid plan.
- If replacement material is invalid, an old configuration continues only while
  it remains valid; after expiry the result is explicit unavailable.
- Full-expiry recovery has an out-of-band authenticated operator input and does
  not need the expired Source TLS channel itself.
- The selected path remains operable by one Product Owner without an unselected
  issuer, unattended secret, hidden watcher, or false availability claim.

## Evidence plan

Trace CLI/Node file loading through `source.Config`, `Plan.New`, and TLS
configuration creation. Run the targeted Source TLS and configuration tests;
use controlled time and test keys. Compare same-key leaf replacement, leaf-key
pin rotation, CA rotation, malformed replacement, and full-expiry recovery.

## Options

### Controlled restart

The Product Owner obtains server/client leafs through an external process that
is not selected as an Ardents issuer. Before expiry, the owner stages one
complete bounded replacement plan and its certificate/key/root/pin inputs,
keeps the old valid process running until the new input has been independently
checked, then performs one controlled Source process restart. `Plan.New` owns
copies of the validated material and `Serve` owns one listener until
cancellation; a process restart is therefore the existing atomic configuration
seam. It makes no seamless-connection claim: the listener stops, joins bounded
handlers, and the new process accepts only new handshakes under the replacement
material.

A C0 leaf renewal retains the same Ed25519 leaf key, hostname, CA root, and
pin. The replacement leaf can then change validity/serial only: Source client
server pin and server client-pin set remain unchanged. A malformed or otherwise
invalid replacement is not started. The old process may remain only while its
own material is valid; after expiry it must return unavailable rather than use
old material.

### Atomic in-process reload

Rejected for C0. `Plan.New` copies client/server certificates, roots, and pins
into private state, and `Serve` captures one `tls.Config` for its listener.
There is no reload Interface, atomic version selection, acknowledgement,
drain/restart ordering, or error outcome. Adding a file watcher or mutating
TLS configuration would be new lifecycle and trust behavior, not a
configuration-only patch.

### Separate issuer and key/CA rotation

Rejected for C0. The current product selects no CA product, ACME account,
online issuer, certificate authority, or authenticated Source trust-map
rotation. Server leaf-key rotation changes each client `LeafKeyDigest`; client
leaf-key rotation changes the server pin set; CA rotation changes roots and
the presented chain. Those are coordinated trust changes, not a leaf renewal.
The server can list up to three client pins, but a client names exactly one
server leaf pin, so that asymmetry is not a general rotation protocol.

## Findings

- **Measurement (2026-09-05):** `go test ./internal/network/source -run
  'Test.*(TLS|Certificate|Source|Plan|Credential|Expired)' -count=1 -v` exited
  `0`. It accepts the valid mutual-TLS control and rejects expired server/client
  leafs, not-yet-valid server leaf, unknown CA, server/client pin mismatch, and
  hostname mismatch at controlled handshake time.
- **Measurement (2026-09-05):** `go test ./cmd/ardents ./cmd/ardents-node -run
  'Test.*(Source|Credential|Plan)' -count=1 -v` exited `0`. It exercises
  bounded Source-plan and Node-plan loading paths.
- **Measurement (2026-09-05):** the `ardents` and `ardents-node` loaders read
  certificate/key, root, and pin paths once into `source.Config`; `Plan.New`
  validates and copies them. `Serve` creates one listener from one server
  configuration and, on cancellation, closes that listener then waits for the
  bounded connection handlers.
- **Inference:** controlled restart is the only existing lifecycle seam that
  can atomically select complete validated Source material. It does not create
  or renew a certificate.
- **Inference:** full-expiry recovery cannot use the expired mutual-TLS path.
  The owner must receive a complete replacement plan/material through an
  independent authenticated channel, start the Source with it, and observe a
  successful ready event. Until then new connections are unavailable.

## Recommendation

Choose controlled restart with externally provisioned same-key leaf renewal
for C0. Confidence is high because this exactly matches immutable configuration
ownership and preserves all existing TLS checks. The strongest objection is
availability: renewal deliberately interrupts Source service and a missed
expiry causes explicit unavailability until the owner supplies valid material.
That limitation is visible and preferable to accepting expired material or
inventing an unreviewed issuer/reload path.

The operator procedure is:

1. before expiry, obtain a new leaf signed by the unchanged existing CA for the
   unchanged hostname and same Ed25519 leaf key;
2. stage an exact new bounded Source plan and certificate/key/root/pin inputs
   outside the running process, preserving the old input for rollback only
   while it remains valid;
3. stop the Source, wait for its listener/handlers to terminate, and start it
   with the replacement inputs; readiness is the success acknowledgement; and
4. if all prior material has expired, use the same external authenticated input
   path to recover. Do not retry renewal through expired TLS and do not use a
   stale valid-looking plan as a fallback.

Leaf-key pin rotation, client-key rotation, CA rotation, any in-process reload,
or an issuer are out of scope until separately researched and accepted.

## Disposition

Decided on 2026-09-05: C0 uses controlled restart and same-key leaf renewal;
no issuer or hot reload is selected. The decision does not claim an operational
Source profile, automatic recovery, Beta readiness, or permanent exclusion of
key/CA rotation. No ADR, implementation, wire, dependency, or current
maintained contract has changed.
