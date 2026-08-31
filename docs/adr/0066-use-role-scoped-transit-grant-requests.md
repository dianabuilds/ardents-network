---
status: accepted
date: 2026-08-31
extends: 0062-scope-online-transit-grant-signing.md
---

# ADR-0066 — Use role-scoped Transit Grant requests

The maintained headless Publisher needs separate one-use Introduction and
Responder authorizations, but ADR-0062's private issuance request hard-codes
only Introduction. Generalize that versioned, target-free request to one exact
`TransitRole + TransitNodeID` tuple and permit only the already accepted
Introduction and Responder roles. Endpoint derives both facts from current
authenticated State and owns a separate at-most-once request/key lifecycle for
each adjacent attachment. The same purpose-scoped signer and common finite
duty budget serve both roles; fixed encrypted outcomes, Transit Grant v1,
receiving-Node checks, Route, Target, Descriptor v2, and C-2 semantics remain
unchanged, and no caller receives a Grant, key, peer, role, or Route plan.

This chooses one common grammar and budget over a separately keyed Responder
issuer because both receiving duties already validate the exact signed role
and Node against current State. Signer compromise can spend the remaining
common budget across either role, but cannot sign State or select a Route.
