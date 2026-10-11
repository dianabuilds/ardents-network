---
id: R-142
title: Can Service Credential renewal be automated without giving a compromised Publisher indefinite Target authority?
status: decided; C0 retains explicit interactive issuance and no automatic renewal
owner: Product Owner and Codex
started: 2026-09-05
---

# R-142 — Service Credential renewal

## Decision this unlocks

Determine whether the maintained C0 Publisher may obtain one or more future
Service Credentials without a new interactive Authority Custody ceremony. The
result must select one bounded renewal owner and recovery contract, or retain
manual issuance as the explicit C0 behavior. It does not authorize a timer,
Credential wire change, Authority password automation, online issuer, or
implementation.

## Current contract

Custody alone opens the encrypted Service Authority and issues one exact,
host-generated public Instance request. It advances the Authority generation
and `NotAfter` watermarks durably before returning the response. One Credential
lasts at most 24 hours and a request must end within the current 48-hour
horizon. The request's `NotBefore` may not precede the durable prior
`NotAfter`, so a pre-issued successor does not make it current early.

R-141 decided that C0 retains this expiry-gated successor restriction. A
consumed root never revives after publication; normal restart, host loss, and
compromise are not silently repaired by credential renewal. Existing TLS,
signature, expiry, replay/spend, and generation-floor checks remain required.
The terminal secret input is explicitly local and may not come from arguments,
environment, configuration, or a background process.

## Hypotheses

- **H1:** A finite, Authority-signed pre-authorized renewal capability can
  authorize only exact future Instance requests within a bounded generation and
  time horizon, survive interruption idempotently, and end without an
  Authority password on the Publisher.
- **H2:** A finite preset schedule of Authority-signed future Credentials can
  give the same bounded autonomy without a host-held renewal signer.
- **H3:** A separately online issuer can safely renew under a more restricted
  key than the Service Authority.
- **H0:** None of the options preserves the current single-owner custody,
  non-overlap, rollback, and host-compromise limits; C0 must retain one manual
  Authority issuance per next Credential.

## Evaluation criteria

- A compromised or copied Publisher cannot obtain an unbounded number of
  generations, extend a terminal horizon, alter Target/Network/capabilities,
  or authorize another host outside the exact delegated policy.
- Every renewal is monotonic, durable, idempotent for one request identity,
  and fails closed on rollback, time uncertainty, expiry, revocation, loss of
  network, or ambiguous response.
- A positive case advances to one future non-overlapping Credential without a
  second operator action; an exhausted/revoked case stops explicitly.
- The scheme distinguishes issuance from currentness: R-141's expiry boundary
  still prevents early Publication replacement.
- One Product Owner can operate the selected procedure without a hidden
  always-on operator, exporting Authority material, or storing the Authority
  password for background use.

## Evidence plan

### Primary sources

- Current Custody owner, `internal/custody/service_credential.go`, terminal
  secret input, command reference, private-reachability owner, and R-141,
  inspected 2026-09-05.
- Any candidate delegated-credential or online-issuer specification must be
  evaluated from its primary source before selection.

### Repository measurement

Trace the real request/digest/terminal-unlock/response/host-accept path,
including every durable write and idempotent retry. Exercise controlled-time
fixtures for ordinary issuance, pre-issued successor, exhausted authority,
clock rollback, missing response, duplicated response, and interruption
between each write. Diagnostics remain outside the repository and use test
keys only.

### Falsification cases

- an old or copied host renews after its finite right is spent or revoked;
- a renewal changes Target, Network, Capability, instance key, or generation
  outside its exact grant;
- a clock rollback, response replay, or interrupted write revives a prior
  right;
- an ordinary restart is misreported as recovery before the predecessor expiry;
- the proposed workflow requires an Authority password in a file, environment,
  argument, unattended configuration, or log.

## Options

### Bounded preset schedule

Custody can already issue a future Credential whose `NotAfter` is within 48
hours and whose `NotBefore` follows the preceding terminal bound. An owner may
therefore deliberately prepare one future host key/request and obtain its
response during an interactive ceremony. This is not automatic renewal: the
next host root, its activation after the predecessor expiry, and any further
issuance still require explicit owner action. It cannot make the successor
current early under R-141.

### Finite host-held renewal capability

No current Credential, Request, response, Vault state, or Endpoint Interface
contains a subordinate renewal right or an issuer key. Adding one would require
a new signed policy, a bounded delegated signer or Authority operation,
generation/time/Target/key/capability constraints, durable spend/revocation and
rollback state, and a host recovery model. A finite counter alone is not a
proof against a copied host right. This is a new authority and wire decision,
not a timer around the existing issuance code.

### Separately online issuer

There is no selected online issuer or Authority responder. R-141 established
that a nonce-bound Authority currentness path already lacks a permitted
topology, transport, discovery, and online-custody contract. An online renewal
issuer would require the same unselected infrastructure plus a separate issuer
trust boundary; it is not a harmless Custody adapter.

### No automatic renewal

Keep one explicit terminal-unlock issuance ceremony for each new Credential.
The operator may consciously pre-issue one bounded future Credential under the
existing 24-hour/48-hour limits, but C0 makes no autonomous renewal, restart,
or availability claim from that preparation.

## Findings

- **Measurement (2026-09-05):** `go test ./internal/custody -run
  'Test.*(Service|Credential|Successor)' -count=1 -v` exited `0`. It proves
  exact-request idempotent retry, rejects stale-source issuance of a different
  successor, and refuses over-24-hour or beyond-48-hour requests before
  password input.
- **Measurement (2026-09-05):** `go test ./cmd/ardents-custody -run
  'Test.*(IssueServiceCredential|ServiceCredential)' -count=1 -v` exited `0`.
  It proves a substituted request is rejected before password input or Vault
  mutation.
- **Measurement (2026-09-05):** every `OperationIssueServiceCredential` reads
  an encrypted source record, asks `SecretInput` for the unlock password, and
  receives an independently transferred exact request digest. The command's
  concrete `terminalSecretInput` requires an interactive terminal and forbids
  argv, environment, configuration, and shared stdin.
- **Inference:** the stored deterministic successor record makes an interrupted
  *same* issuance retry safe; it is not a reusable authorization for a new
  Instance request or next generation.
- **Inference:** a pre-issued next Credential can reduce one future custody
  ceremony but cannot provide continuous service under R-141, because it is
  not current before its non-overlapping `NotBefore` and no selected runtime
  owns automatic root/process transition at that boundary.

## Recommendation

Choose no automatic Service Credential renewal for C0. Confidence is high:
the current bounded manual ceremony is deliberately exact and idempotent, while
each autonomous alternative introduces a new issuer, secret, topology, or wire
contract that is neither selected nor safe to infer. The strongest objection is
operational: a single Product Owner must perform an issuance ceremony at least
once per prepared next Credential. That cost is explicit; it is preferable to
giving a compromised Publisher indefinite Target authority or automating the
Authority password.

This decision does not forbid a future product from selecting a finite renewal
right. It requires that future work to start as one separate question and
choose the right's issuer, exact policy, single owner, durable idempotence,
revocation, time/rollback, partition, loss, and operator procedures before an
ADR or implementation issue exists.

## Disposition

Decided on 2026-09-05: C0 retains explicit interactive issuance and no automatic
renewal. R-141 remains the expiry-gated currentness limitation; this decision
does not turn either limit into a permanent product requirement. No ADR,
implementation issue, code, wire, dependency, or current maintained contract
has changed.
