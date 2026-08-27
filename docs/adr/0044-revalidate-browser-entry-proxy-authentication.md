---
status: accepted
date: 2026-08-26
---

# ADR-0044 — Revalidate Browser Entry proxy authentication

## Context

The Firefox Browser Entry first selected a live local alpha proxy by asking its
native host for a loopback port. That proves the proxy only at the instant of
the native-host probe. A later browser connection can race withdrawal or a
recycled port, so an extension must not disclose a reusable local secret merely
because of that earlier proof.

R-117's Firefox 154 experiment proved that a Manifest V3 add-on can receive a
loopback HTTP proxy `407 Basic` challenge, ask its exact native host again,
match the returned port to the challenger, and answer under `.ard`-only request
filters. The Product Owner accepted that narrower handoff and its additional
Firefox permission surface for the alpha tracer.

## Decision

Each Browser Entry publisher generates two distinct process-local random
values: a probe capability and a proxy credential. It atomically publishes both
with the current unprivileged loopback port in a distinct state-schema version.
The native host accepts only a
bounded port operation and a bounded proxy-authentication operation. Either
operation first proves the current proxy by its probe capability; the latter
then returns that same port and the hexadecimal form of only the separate proxy
credential.

An Endpoint-created Browser Entry AlphaProxy answers its fixed liveness probe
before proxy authentication. Every other request requires the current Basic
credential. It removes `Proxy-Authorization` before forwarding to the selected
local Reference origin.

The fixed Firefox add-on retains its exact `http(s)://*.ard/*` host scope. It
uses `proxy`, `nativeMessaging`, `webRequest`, and `webRequestBlocking`. On a
matching `407` from `127.0.0.1`, it calls the native host again, requires the
proved port to equal the challenger port, and supplies at most one credential
answer for that browser request. It cancels every unmatched, malformed, or
second challenge.

## Consequences

- A stale state file or unrelated loopback process that only answers `204`
  cannot receive the Browser Entry credential.
- The signed add-on's permission surface expands, but no `<all_urls>`, DNS,
  DoH, global proxy, CA, browser-fork, target-selection, or Service credential
  authority is introduced.
- This narrows a port-rebinding handoff race; it does not provide isolation
  from another process that can read the same per-user state or control the
  same browser profile.
- This remains an HTTP-alpha code tracer. It neither selects a participant
  installer/XPI signing path nor enables HTTPS, public DNS, or arbitrary web
  applications.

## Compliance

- [R-115](../research/records/r-115-named-browser-entry.md) owns the address
  and browser-boundary question; [R-117](../research/records/r-117-firefox-browser-entry-delivery.md)
  records the Firefox experiment and delivery gate.
- Browser Entry/native-host and alpha-proxy behavior tests prove fresh liveness
  before disclosure, the Basic `407` gate, and exact registered-name routing.
- `make qualification-h4-4a-firefox` exercises the maintained extension,
  native host, and Endpoint proxy in a fresh temporary Firefox profile.
