---
status: accepted; browser-presentation portion superseded by ADR-0045
date: 2026-08-25
---

# ADR-0040 — Keep named alpha outside the canonical Namespace

**Supersession note:** the finite alpha-corpus and non-Namespace decisions in
this ADR remain accepted. Its former explicit-loopback browser presentation
was superseded by ADR-0045's Firefox-only no-port `.ard` path; it remains only
historical compatibility evidence.

## Context

H4-3 proves an exact Target-Link-to-browser loop, while canonical Namespace
resolution requires a threshold-attested current Epoch materialization. A
project-selected alpha list cannot truthfully enter that verifier. The first
usable named journey must also work in an unmodified ordinary browser without a
public CA, browser-wide resolver/proxy setting, or trust-store change.

## Decision

For H4-4A only, use the explicit `ardents-alpha://<canonical-name>` Alpha
Service Link and a finite authority-signed Alpha Name Corpus. The corpus has
one declared cohort, nonzero serial, finite validity, at most eight bindings,
and an explicit empty withdrawn state. A later serial may replace it; a client
must fail closed on a stale or same-serial-conflicting corpus during one live
resolution session. Restart-persistent corpus floors remain a promotion gate,
not an unearned claim.

An alpha binding is not a current canonical Namespace binding and never becomes
an `ardents://` Service Link, DNS query, HTTP redirect, or Target-Link fallback.
The original ADR permitted a local compatibility presentation after Endpoint
authentication. That presentation is superseded:
the maintained Alpha Browser path now exposes only the ADR-0045 `.ard` address
through its scoped loopback proxy, and keeps its actual listener origin
internal. Neither presentation is public DNS, HTTPS, a Web PKI identity, or an
Application privacy boundary.

Private alpha lookup retains the selected OHTTP Relay/Gateway role split but
uses an alpha-specific message domain and corpus authority. It cannot reuse or
weaken the canonical Namespace verifier.

## Consequences

- A named alpha can test whether names improve the user journey without a
  hidden registrar or false public-name claim.
- Alpha corpus authority must be disclosed and its expiry/withdrawal visible;
  missing or invalid evidence has no destination result.
- H4-4B/C still need their canonical binding/current-proof/control evidence.
- `https://*.ard` remains a separate browser and public-domain decision.

## Compliance

- [R-097](../research/records/r-097-named-alpha-private-resolution.md) owns
  the bounded alpha experiment and its role/failure evidence.
- [ADR-0014](0014-private-naming-ohttp.md) owns the selected OHTTP dependency
  closure and its limits.
