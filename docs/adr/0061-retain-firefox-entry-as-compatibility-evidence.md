---
status: accepted
date: 2026-08-30
---

# ADR-0061 — Retain the Firefox entry only as compatibility evidence

## Context

ADR-0045 selected a signed unlisted Firefox XPI and per-user native-host
installation as the functional-alpha Browser Entry. R-115 later observed a
clean Firefox resolver attempt for `.ard` before the proxy route. That
falsifies the required no-DNS/DoH-leak behavior and means the profile cannot be
the participant entry for a private Ardents name.

The network product is headless. Browser extensions, native hosts, desktop UI,
and other application integrations are optional clients of its explicit local
application boundary; they are not prerequisites for operating or qualifying
the network core.

## Decision

Supersede ADR-0045's participant-delivery selection. Retain the existing signed
XPI provenance, exact-name proxy restrictions, native-host authentication, and
owned-only installer/remover as optional compatibility and regression evidence.
Do not place those artifacts in the headless network candidate or count their
qualification as network-product qualification.

Any renewed Browser Entry work starts with a new decision covering system or
browser resolution, DNS/DoH leakage, default-port ownership, and HTTP/HTTPS
trust. It may consume the headless Endpoint application interface, but it must
not move Firefox-specific policy into the network core.

## Consequences

- R-117 is closed rather than awaiting two-platform participant qualification.
- Existing compatibility code and exact historical evidence may remain while
  product/application separation is performed; their presence is not a current
  product claim.
- Headless network build, operation, release, and audit gates cannot require a
  browser, XPI, native host, desktop session, DNS change, or CA installation.
- Removing or redesigning the compatibility path remains a later bounded slice;
  this ADR does not select its replacement.

## Compliance

[R-115](../research/records/r-115-named-browser-entry.md) owns the falsifying
resolver evidence. [R-117](../research/records/r-117-firefox-browser-entry-delivery.md)
retains the delivery provenance. The current boundary and remediation order are
recorded in
[network-application-separation.md](../product/network-application-separation.md).
