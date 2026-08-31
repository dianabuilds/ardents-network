---
id: R-096
title: Existing-browser Adapter contract for the usable alpha
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-096 — Which bounded local Browser Adapter contract lets an existing browser open an exact Ardents Service Link or Target Link without claiming public DNS, Web PKI, global proxying, or browser-level privacy?

## Decision this unlocks

Select, defer, or reject H4-3A's first browser handoff: destination syntax,
local attachment, authorization, request/response boundary, user-visible
failures, and removal/reversal. It may authorize one Reference web Service
path, not a general browsing stack.

## Current contract

- H4-3 requires the complete Publisher-to-existing-browser loop, initially for
  HTML and same-Service resources.
- H4-1 forbids implicit browser, proxy, DNS, route, VPN, or system integration.
- H4-4 Service Links are explicit Ardents forms; they are not Internet hostnames.
- Generic browser compatibility earns no Application-level location-privacy
  claim; H4-7 owns any protected mode.
- **Product decision (2026-08-24):** H4-3A uses an explicit
  Endpoint-to-loopback handoff for one already-selected Target. A browser
  extension, custom URI scheme, custom CA, public certificate, per-user/global
  proxy, DNS integration, and bundled browser are not H4-3A dependencies.

## Hypotheses

- **H1:** an explicit per-user local Adapter can accept only an exact Ardents
  destination, open one authenticated Service Connection, and present bounded
  HTTP semantics to the existing browser without changing ordinary browsing.
- **H2:** a custom-scheme handoff plus a local loopback origin can be usable,
  but requires a separately chosen origin/certificate/cache policy.
- **H0:** no existing-browser path is supportable without global interception,
  a false Web PKI claim, or browser maintenance beyond the project capacity.

## Evaluation criteria

- exact publish/share/open journey and all destination/failure states;
- Local Grant and endpoint authority boundary; no Service Authority exposure;
- browser origin, certificate, cache, redirects, cookies, scripts, same-Service
  resource, DNS, WebRTC, callback, and external-resource behavior;
- explicit setup/removal and coexistence with ordinary Internet browsing;
- no global proxy/DNS/VPN, bundled browser, public CA, or privacy upgrade; and
- implementation and update burden compatible with the one-to-one team.

## Evidence plan

### Primary sources

- Official browser documentation for custom protocol handling, loopback/origin
  behavior, proxy configuration, certificates, and external-request controls.
- Current Ardents H4-1, H4-3, H4-7, product, and threat contracts.

### Experiment

After a live H4-2 profile exists, run a disposable two-Endpoint Reference Site
experiment with one precise browser/version and bounded HTML/resource corpus.
Capture destination handoff, adapter authorization, HTTP requests, failure
states, attempted external/DNS/WebRTC paths, cache behavior, and complete
removal. It makes no privacy claim.

### Failure scenarios

- Adapter accepts an ordinary URL or forwards arbitrary Internet traffic.
- A page redirects or loads external content; browser DNS/WebRTC/callback leaks.
- Another same-user process reaches the loopback adapter without a grant.
- Adapter restart, endpoint failure, certificate/origin error, or stale name
  silently reaches another destination.

## Findings

- **Sourced fact:** the Secure Contexts specification treats `127.0.0.0/8` and
  `::1/128` as potentially trustworthy origins; the port does not affect that
  classification. [W3C Secure Contexts](https://www.w3.org/TR/secure-contexts/)
  (accessed 2026-08-24). This is a standards basis for testing a local
  loopback Adapter without a public DNS name or public certificate. It is not
  browser-version qualification and does not authenticate the remote Service.
- **Sourced fact:** the same specification explains that powerful web features
  rely on an authenticated/confidential delivery context and that a secure
  context is not a complete isolation boundary. [W3C Secure
  Contexts](https://www.w3.org/TR/secure-contexts/) (accessed 2026-08-24).
  Therefore a loopback origin cannot turn generic browser operation into the
  H4-7 protected Application claim.
- **Sourced fact:** Content Security Policy defines response-delivered controls
  for permitted fetch, script, and navigation behaviour. [W3C CSP Level
  3](https://www.w3.org/TR/CSP3/) (accessed 2026-08-24).
- **Sourced fact:** the current CSP Level 3 core gives fetch directives their
  resource-type checks and defines a header-delivered `sandbox` document
  directive. Its pre-navigation algorithm otherwise returns allowed unless a
  directive supplies the relevant check; the current navigation-directive set
  is not a generic `navigate-to` allowlist. [W3C CSP Level
  3](https://www.w3.org/TR/CSP3/) (accessed 2026-08-24). A fetch-only policy
  must therefore not be described as complete navigation containment.
- **Inference:** the smallest credible H4-3 candidate is an explicit Endpoint
  action such as `open <Target Link>` which opens one random local
  `http://127.0.0.1:<port>/...` URL in the selected existing browser. The
  Adapter binds loopback only, opens one exact Target Link through the Endpoint,
  and maps only that Service's bounded HTTP request/response set. It changes no
  browser proxy/DNS/VPN setting, browser registration, or system trust store.
- **Inference:** the initial Reference Site must prohibit executable scripts,
  WebRTC/WebSocket, form submission, external URLs, service workers, and remote
  redirects. The Adapter needs a response policy (including a restrictive CSP
  and no ambient privileged Adapter endpoints in the rendered origin) so a
  Publisher-controlled page cannot turn the loopback origin into Endpoint
  administration. This is a containment rule, not a privacy boundary.
- **Inference:** a bearer-like value in the rendered URL would be visible to
  the rendered site. If an opaque local path is needed, it may authorize only
  the already-selected Service Connection and no Endpoint administration,
  unrelated target, or persistent credential. The experiment must prove this
  route scoping rather than treat a loopback listener as same-user isolation.

- **Product selection (2026-08-24):** option 1 is accepted as H4-3A's Adapter
  direction. `open <Target Link>` means that the Endpoint first authenticates
  one Target and obtains one Service Connection, then creates a fresh
  `http://127.0.0.1:<port>/site/<opaque>/` presentation origin and opens that
  exact URL. One port/origin is scoped to one active connection; the opaque
  path is fresh, at least 128 bits, and authorizes no Target selection,
  unrelated connection, Local Grant, or Endpoint administration. Closing the
  connection withdraws the origin. This local decision does not change the
  public wire protocol, so it is promoted to the H4-3 product contract without
  a new ADR.
- **Measurement:** a separated three-listener fixture on the current Windows
  host used a Publisher simulator, a browser Adapter, and an out-of-origin
  `localhost` HTTP sentinel. The initial exact-route browser binary SHA-256 was
  `d08ce4f617bfd6c50173befe78897ba921039fab2f36f47ac71071df3f5e3ada`.
  The Adapter bound `127.0.0.1` on an ephemeral port and emitted a fresh 128-bit
  path; two independent starts produced different path values.
- **Browser measurement:** the Codex in-app browser identified itself to the
  Adapter as Chrome `151.0.0.0` on Windows. It rendered the expected title,
  heading, CSS color `rgb(1, 2, 3)`, and 8-pixel same-origin SVG. The inline
  script marker remained false, the out-of-origin image completed with
  `naturalWidth=0`, and the sentinel observed zero requests. Browser traffic at
  the Adapter consisted only of the page, stylesheet, and SVG (the SVG was
  requested separately as image and icon); the Publisher received only the
  corresponding translated `/`, `/resource.css`, and `/visual.svg` paths.
- **Response-policy measurement:** the Adapter returned the declared static CSP,
  `Cache-Control: no-store`, `Referrer-Policy: no-referrer`,
  `X-Content-Type-Options: nosniff`, and
  `Cross-Origin-Opener-Policy: same-origin`. Publisher-supplied `Set-Cookie` and
  `Location` headers were absent at the browser-facing response, and the page
  returned status 200 without a redirect.
- **Navigation counterexample:** with an immediate Publisher `meta refresh`,
  the fetch-only CSP did not contain navigation. Chrome left the Adapter origin
  for the out-of-origin sentinel, which observed `/r096-external.svg` and its
  `/favicon.ico`. This falsifies the earlier implication that the listed fetch
  directives alone prevent Publisher-driven external navigation.
- **Sandbox refinement measurement:** the comparison binary SHA-256 was
  `2bba0edbd6f1d5bbc666613e9b9f6c1e6c0e76ac187530d4c6bef2525c661748`.
  Adding header-delivered `sandbox allow-same-origin` retained the expected
  page, CSS color, 8-pixel SVG, and Adapter origin, while the same automatic
  refresh and a deliberate external-link click produced zero sentinel
  requests. The clicked navigation was rejected and the URL remained on the
  Adapter. The route rejection matrix still returned 404/405 without new
  Publisher requests. The promoted default-sandbox source then built as
  SHA-256 `aa6a793745b1e78766f2463f314b74593b36e4eae823d7eca0db4921e568a6f5`
  and returned the selected sandbox CSP by default; this last check was HTTP,
  not another browser qualification.
- **Rejection measurement:** a wrong 128-bit path and a query-bearing path
  returned 404, POST returned 405, and an ordinary proxy-form request with host
  `example.com` returned 404. None increased the Publisher request count, and
  the external sentinel remained at zero. After the exact fixture process was
  stopped, the same browser URL rendered `ERR_CONNECTION_REFUSED`; it did not
  reach another Target or a public fallback. Both exact temporary roots were
  removed with zero residue. [Experiment
  README](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-096-browser-loopback/README.md)
- **Limit of measurement:** this was the Codex in-app browser, not a selected
  participant browser/profile. Its evaluation surface did not expose
  `isSecureContext`, readable cookie state, browser console CSP messages, or a
  DNS event stream. The HTTP sentinel proves that the tested external resource
  did not become an HTTP
  fetch, not that no DNS lookup was attempted. The fixture contains no Target
  Link, Local Grant, Endpoint, H4-2 Service Connection, malicious same-user
  process, script-capable site, remote Publisher, context-menu navigation, or
  arbitrary HTML corpus. The fixture processes were stopped by exact checked
  termination on Windows; graceful signal/join behavior was not qualified. It
  therefore selects the Adapter direction and first static sandbox profile but
  does not close R-096 or H4-3A.

## Options

1. **Explicit Endpoint-to-loopback handoff — selected for H4-3A.** The Endpoint opens a scoped local
   URL for one exact Target Link in one supported browser. It has no public DNS,
   CA, browser registration, or global traffic interception.
2. **Browser extension as an explicit helper.** It may improve typing/paste UX,
   but introduces browser-store, permission, version, and review work without
   replacing the Endpoint's local authorization boundary.
3. **Custom URI scheme.** It can make a Target Link clickable, but needs OS
   protocol registration and its lifecycle/removal; that is outside the H4-1A
   no-implicit-integration baseline.
4. **Per-user browser proxy rule.** It risks widening arbitrary browser traffic
   and configuration support beyond an exact handoff.
5. **Bundled/forked browser or global interception.** Rejected by the current
   maintenance capacity unless later evidence overturns it.

## Recommendation

Implement only option 1 when H4-1/H4-2 provide the required Endpoint and
Service Connection seams. Keep the first Reference Site static and the Adapter
surface an exact method/resource allowlist with header-delivered
`sandbox allow-same-origin`; fetch directives alone are insufficient. Do not
add an extension, URI
registration, custom CA, or proxy configuration pre-emptively. The live run
must still demonstrate a supported external browser/version, Target/Local Grant
authorization, ordinary browsing unchanged, external DNS/WebRTC observation,
and Publisher withdrawal through the real H4-2 path.

**Confidence:** medium. **Strongest argument against the candidate:** a
loopback browser origin places publisher-controlled content near local Adapter
state; even a static-CSP first slice can be hard to make ergonomic without
eventually adding a browser extension or stronger browser/process isolation.

## Disposition

Decided and promoted to the H4-3 product contract. The selected pre-development
answer is the explicit fresh loopback origin and static sandboxed Reference
Site; no extension, URI registration, custom CA, proxy, or privacy claim is
selected. A purpose-named two-Endpoint, carrier-backed supported-browser run is
an implementation-linked qualification task rather than a reason to keep the
Adapter-contract decision open. Retain the temporary fixture only until its
unique counterexample and request-boundary evidence enter source history.

### Supersession note — 2026-08-25

The Product Owner retained the explicit scoped Adapter direction but rejected
the static CSP-sandbox as a restriction on Publisher content. It remains the
H4-3A Reference fixture and its recorded evidence is unchanged. It must not
define the Service or Carrier contract: Ardents carries the selected
Publisher-Service application bytes without CMS-, markup-, or content-specific
mechanisms. R-114 now owns the implementation and qualification question for a
generic bidirectional existing-browser connection. This change makes no
browser-isolation or external-request privacy claim.
