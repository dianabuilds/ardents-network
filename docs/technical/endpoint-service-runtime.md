# Endpoint and Service runtime

Status: **current maintained technical contract.** This document describes the
local Endpoint, generic Broker, Service publication, and Service Connection
Modules that exist in the repository. It does not select a supported desktop
profile, a qualified Application Isolation profile, a public Service protocol,
or a complete Route/Node qualification.

## Ownership

The local runtime has separate Modules and Interfaces:

| Module | Interface responsibility | Implementation hidden from callers |
|---|---|---|
| internal/application/broker | Admit and consume one short-lived Local Grant capability for either connection or administration; revoke, drain, and close that finite session set; report generic/unqualified. | Capability generation, replay removal, expiry, commitments, session accounting, and grant invalidation. |
| internal/endpoint | Compose one role-local process and expose typed publication, withdrawal, inbound-connection, and outbound-connection operations. Run owns bounded plan loading, readiness, local listeners, signal cancellation, joining, and residue cleanup. | Local socket choreography, result-channel negotiation, Broker consumption, TLS carrier setup, publication acquisition, and Connection invocation. |
| internal/service/publication | Open, publish, acquire, unpublish, and close one exclusive Service Instance generation. | Crash-atomic public record/floor persistence, volatile Instance signer, live-reference accounting, drain, and private-material erasure. |
| internal/service/connection | Carry one logical authenticated Service Connection across fresh Route Attachments and return one terminal outcome. | Exact Instance challenge/proof, continuity MAC, ordered data/acknowledgement offsets, replay handling, recovery deadline, and attachment cleanup. |

The caller-facing Endpoint seam is role-specific: a publication request cannot
include Route or Application facts, and an outbound connection cannot supply a
publisher signer. This keeps publication ownership, local admission, Route
attachment, and logical-stream recovery out of one mutable request bag.

## Local admission

The Broker has one volatile generation. A Grant is bound to one opaque local
Principal and one of the closed surfaces connection or administration. Admit
creates a fresh one-use capability; Consume removes it before returning a
bounded receipt. Revoke, permitted finite drain, and close invalidate
unconsumed capabilities immediately. Work that already consumed a capability
is not claimed to be interrupted by Broker revocation.

The only current isolation observation is generic/unqualified. It means the
runtime deliberately makes no statement about sandboxing, hostile same-user
applications, process-tree confinement, supported host platforms, or
Application Location Privacy. A qualified platform Adapter requires separate
research and an ADR.

## Publication and connection lifecycle

    Administration Grant
      -> Endpoint.Publish
      -> Publication.Publish one higher Instance generation
      -> immutable public record + volatile signer
      -> Endpoint.Connect or Endpoint.Accept consumes a Connection Grant
      -> exact-Instance TLS challenge/proof + Service Connection v1
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome
      -> unpublish/supersede stops acquisitions, drains references, erases private material

Service Connection accepts only the closed ardents-interactive-route-v1
profile. It has no H3 reader, profile negotiation, direct fallback,
peer-selected profile, Publication private key, or Application IPC
authorization. Its parser bound of 16 KiB per Data record is an allocation
limit, not a product throughput promise.

Publication persists public proof and its non-decreasing generation floor but
never persists a live Instance private key. AcquireAt yields an opaque Lease;
the Lease can sign for its generation without exposing the signer. Withdrawal,
supersession, expiry, or close first prevent new acquisition, then wait for
bounded references before erasing private material.

## Endpoint process contract

The Endpoint v1 Application contract chooses a separate terminal-result channel
before opaque Application bytes flow. Raw-tail delivery and timing-selected
terminal results are retired. Endpoint cancellation closes only its owned
Application, result, and Route listeners, joins blocked accepts, and removes
those socket paths before it returns.

Endpoint is a composition Module, not a second durable domain owner. It owns
no Namespace, Network State, Release, Update, Custody, or Route-selection
state. Route Attachments are already authenticated opaque carriers; Namespace
and State facts arrive only in the typed inputs required for Connection
binding.

## Verification and related decisions

- Go tests for Broker, Endpoint, Publication, and Service Connection exercise
  the Module Interfaces and failure paths.
- The Endpoint recovery process test exercises readiness, cancellation, join,
  socket cleanup, publication, and opaque Application stream boundaries.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md) selects the
  native Route foundation; [ADR-0028](../adr/0028-native-service-connection-v1.md)
  selects the closed Service Connection grammar.
- [R-085](../research/records/r-085-m10-generic-broker-scope.md) limits the
  Broker to its explicit generic/unqualified contract.
