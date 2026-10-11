---
id: R-114
title: Application-transparent existing-browser Service connection
status: decided; qualification-linked
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-27
---

# R-114 — How can one selected existing-browser origin carry application-transparent Publisher-Service traffic without becoming an arbitrary Internet proxy or claiming browser isolation?

## Decision this unlocks

Select the H4-3B connection boundary that lets an ordinary Publisher expose an
ordinary web Service through Ardents without a CMS plug-in, static export,
content rewriting, or application-aware protocol feature.

## Current contract

- The Product Owner selected application-payload neutrality on 2026-08-25:
  Ardents carries a selected Publisher Service's bytes and does not classify
  the Service as WordPress, a static site, an API, or another special case.
- H4-3A's explicit loopback Reference fixture proves a narrow request boundary;
  its static CSP and method/resource allowlist are fixture behavior, not a
  product content rule.
- H4-4 names one authenticated Service Target. They must not select a
  content profile. Its final browser-visible address remains unselected.
- The Adapter must not become a browser-wide proxy, public Internet exit,
  Service Authority, or H4-7 browser-isolation claim.

## Hypotheses

- **H1:** a connection-scoped, bidirectional origin bridge can preserve normal
  same-Service HTTP semantics without examining application content, while the
  Endpoint selects the single Service before the browser participates.
- **H2:** standard browser semantics require content-aware adaptation or a CMS
  integration; if so, reject that adaptation rather than making it a hidden
  Ardents application mechanism.
- **H0:** no bounded existing-browser bridge meets the authorization and
  operational contract; retain H4-3A only and do not claim generic web access.

## Evaluation criteria

- An unmodified dynamic fixture performs a state-changing request, cookie
  round-trip, redirect, and streamed response through one selected Service.
- The bridge carries application bytes without an HTTP method allowlist,
  response rewriting, CMS awareness, static export, or content policy.
- A browser request cannot select another Ardents Target, ordinary Internet
  URL, Publisher socket, or Endpoint-administration operation.
- Resource limits, cancellation, failure, and withdrawal are explicit
  connection behavior rather than content classification.
- The profile makes no zero-latency, browser-isolation, external-request,
  anonymity, or location-privacy claim.

## Evidence plan

### Primary sources

- H4-3/H4-4 product contracts, R-096, R-105 through R-107, and the maintained
  Endpoint/Reference fixture source (accessed 2026-08-25).
- IETF [RFC 9110, HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
  and [RFC 9112, HTTP/1.1](https://www.rfc-editor.org/rfc/rfc9112.html), plus
  the official Go 1.26.6 [`net/http`](https://pkg.go.dev/net/http@go1.26.6)
  documentation, accessed 2026-08-27 for redirect, message streaming,
  incomplete-response, proxy, `ReadResponse`, and `ResponseWriter` behavior.

### Experiment

Replace no production content with a disposable dynamic Publisher fixture.
Run separate Publisher and User processes through the maintained C-2 path;
capture request method/body, response headers/body, cookie, redirect,
streaming, cancellation, withdrawal, and rejection of a second destination.
Use a CMS container only as a compatibility workload after the generic fixture
passes.

### Failure scenarios

- The Adapter parses or rewrites application content to make a website work.
- Browser input causes a connection to an unintended Service or Internet host.
- A response body buffers without a declared bound, hangs after withdrawal, or
  loses a request body or response header.
- A same-user process or site content acquires Endpoint authority or a
  browser-isolation/privacy claim.

## Findings

- **Implementation fact:** `reference.TransparentServer` owns one
  connection-scoped HTTP/1.1 bridge. It receives a browser request only after
  the Endpoint authenticated one alpha Binding and Target, serializes that
  request onto that selected Service Connection, and streams the response
  back. It has no target URL, dialer, content classifier, CMS rule, static
  resource map, or Publisher administration input. HTTP Upgrade is an
  explicit unsupported protocol result, not a hidden adaptation.
- **Measurement:** the maintained separate-process C-2 tracer now runs a
  dynamic Publisher Application through `StartAlphaTransparentConnection`.
  The User sends a form `POST` with a body, header, and cookie; receives a
  `302` plus `Set-Cookie`; follows with a cookie-bearing `GET`; and reads a
  chunked response. The Publisher verifies its ordinary `reference.ard` Host
  and all those request facts. The same run rejects an unregistered `.ard`
  name and an ordinary Internet name at the local alpha proxy. It then closes
  its local Application after one final same-Service request: the authenticated
  terminal reaches the User, closes this HTTP presentation, and a subsequent
  visible-name request cannot retain a usable route or choose another
  destination. It passed locally on 2026-08-26 in 7.112 seconds.
- **Measurement (2026-08-27):** three additional separate-process cells passed
  locally. Explicit Service Administration withdrawal returned `unpublished`
  for the exact Target/generation after the active connection drained while
  the User retained `clean service connection close` (6.64s). A reset of the
  token-bound Publisher Application after a committed partial response gave
  both Endpoints `abrupt connection loss` (6.78s). A hard-stopped Publisher
  Endpoint during an active request gave the surviving User `abrupt connection
  loss` and closed only the local Publisher Application handoff (6.82s). Each
  failure left the selected name, an unregistered alpha name, and an ordinary
  Internet name unavailable through the scoped proxy. The behavior seam also
  repeats the Application-reset classification without process orchestration.
- **Measurement (2026-08-27):** the Reference behavior seam registered two
  simultaneous transparent routes with distinct Target identities. Failure of
  the requested Target returned `502` without sending any request to the
  second registered Target; an explicit request to that second name then
  returned `204`, proving that it was live but never selected as fallback.
  After process-level withdrawal, a second fresh Administration attempt
  returned `service unavailable`; the maintained publication behavior test
  independently rejects a new acquisition while `Unpublish` drains.
- **Implementation consequence:** after Publisher response headers are
  committed, Go's local HTTP server cannot emit a second failure status. It may
  terminate the local HTTP response with only the bytes already received. The
  authoritative failure is the separately classified Endpoint terminal; a
  partial application body is never evidence that the request completed.
- **Limitation:** this remains one ordered HTTP/1.1 Service Connection. It
  makes no HTTP Upgrade/WebSocket, HTTP/2, arbitrary concurrent-request,
  browser installation, external-origin, isolation, capacity, Docker/VPS,
  actual multi-host, or public participant claim. The old static Reference
  Site remains H4-3A evidence and has not become a product content policy.

## Options

1. A connection-scoped transparent bridge for one selected Service.
2. A content-aware site adapter or CMS integration.
3. Retain static Reference-only access.

Options 2 and 3 do not meet the Product Owner's selected general-Service
outcome; they remain falsification baselines, not implementation directions.

## Recommendation

Retain option 1 as the selected H4-3B implementation direction and qualify the
same exact candidate across its selected release platforms before promotion. Do not design a
WordPress-specific mechanism. A WordPress container, if later used, is only a
compatibility workload over this same bridge.

**Confidence:** high for the selected one-Service HTTP/1.1 boundary and local
failure semantics; medium for broader application compatibility. **Strongest
argument against:** a selected workload may require HTTP Upgrade, HTTP/2, or
concurrency semantics outside this bounded candidate.

## Disposition

Decided for the H4-3B connection boundary. The local behavior and
separate-process evidence now prove the selected one-Service HTTP/1.1 seam,
normal Application EOF, explicit publication withdrawal, local Application
reset, abrupt Publisher Endpoint loss, exact terminal classes, and
no-destination-fallback behavior. Remaining Docker/VPS, selected
platform/browser, actual multi-host, and release qualification belong to the
H4-3/H4-8 evidence campaign. H4-4's named Browser Entry is not a dependency of
this decision or of H4-3B completion.
