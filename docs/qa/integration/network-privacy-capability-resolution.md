# Scenario NPI-001

## Title

Private capability resolution and opaque selector interoperability.

## Layer

Integration.

## Domains

Identity, Policy authority boundary, and Network Foundation / Messaging.

## Purpose

Prove that capability material is locally protected and authorized while two
independent nodes still derive the same private Waku selector and envelope key,
and that revocation plus fresh-secret rotation excludes old material.

## Preconditions

- both nodes trust the same configured issuer Ed25519 key;
- both receive the same signed, recipient-bound channel grant;
- each node has a different local capability-store master key.

## Steps And Assertions

1. Import the signed grant into both encrypted capability stores.
2. Assert local capability references differ across nodes.
3. Resolve the grant for the bound subject, scope, permission, and time.
4. Assert both nodes derive the same opaque Waku content topic and envelope key.
5. Assert the selector contains no subject or scope text.
6. Apply a signed revocation and assert use is denied with the stable revoked
   reason.
7. Import a new generation with a fresh channel secret and assert selector and
   envelope key differ from the revoked generation.

## Failure Meaning

Failure means selector interoperability, local non-correlation, issuer trust,
revocation, or key rotation is not safe enough for encrypted Waku envelopes.
