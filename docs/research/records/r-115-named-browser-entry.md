---
id: R-115
title: Named browser entry without localhost or port
status: closed
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-26
---

# R-115 — How can an existing browser open an authenticated Service Name without exposing `localhost` or a port, leaking the name to ordinary DNS/DoH, or turning Ardents into a browser-wide proxy?

## Decision this unlocks

Select H4-4's participant-facing **Name Service and Browser Entry**: the one
generic path from a visible Service Name such as `site.ard` to the already
authenticated, selected Publisher Service. It must work equally for every
Publisher application and must not add a special web-site or CMS mechanism.

## Current contract

- The Product Owner does not accept `localhost` or an ephemeral port as the
  normal visible browser address. The intended experience is a named origin,
  with `site.ard` the illustrative form.
- H4-4's private resolver authenticates Name-to-Target material. It is not an
  ordinary DNS resolver, public zone, registrar, or browser proxy. The Browser
  Entry must not change that authority boundary.
- The browser must use a local Endpoint only for the Ardents name zone; ordinary
  browsing remains its ordinary path. It must not choose a Target, a Publisher
  socket, or an arbitrary Internet URL.
- H4-3's selected payload-neutral contract means name handling cannot select a
  static-only, CMS-specific, or content-aware service profile.
- The IANA root-zone list contained no `ARD` TLD when inspected on 2026-08-25,
  but `.ard` is not an IETF/IANA special-use suffix. It therefore cannot be
  treated as permanently collision-free or automatically understood by normal
  browsers.

## Hypotheses

- **H1:** a user-consented, zone-scoped local resolution and Browser Entry can
  route only authenticated Ardents names to the local Endpoint, while a
  separately selected origin/trust mechanism gives the browser a visible
  no-port name.
- **H2:** a project-controlled publicly registrable suffix plus public Web PKI
  can make the browser origin ordinary, provided its local name routing has an
  explicit no-leak and coexistence contract.
- **H0:** no ordinary-browser arrangement satisfies the no-port, no-hidden
  global interception, no-localhost, and supportability constraints. Retain
  explicit Link handoff and do not claim participant-ready named browsing.

## Evaluation criteria

- A typed or followed named origin reaches only the exact Target verified by
  H4-4; expiry, withdrawal, conflict, Endpoint stop, and unavailable Service
  fail locally without DNS, search, HTTP, or destination fallback.
- The visual address contains neither `localhost` nor a non-default port.
- The generic integration neither sends the Ardents suffix to ordinary DNS/DoH
  nor redirects a non-Ardents name to the Endpoint.
- Browser traffic uses no content-specific routing, rewriting, or application
  plug-in. The same mechanism supports the H4-3B generic Service connection.
- The selected HTTP/HTTPS trust model has no certificate warning, silently
  installed local CA, or unrecorded paid certificate requirement. Its
  certificate-transparency and user-visible-name consequences are stated.
- Installation, update, collision, coexistence with a local service already on
  80/443, removal, crash recovery, and Windows/Ubuntu privilege requirements
  are explicit and supportable by the actual project team.

## Evidence plan

### Primary sources

- [IANA root-zone TLD list](https://data.iana.org/TLD/tlds-alpha-by-domain.txt),
  accessed 2026-08-25.
- [RFC 6761](https://www.rfc-editor.org/rfc/rfc6761.html), accessed
  2026-08-25: special-use suffixes require an IETF specification and define
  implementation behavior; `.localhost` is loopback-specific.
- [RFC 7686](https://www.rfc-editor.org/info/rfc7686/), accessed 2026-08-25:
  a `.onion`-aware application must resolve directly or use a proxy, while
  non-aware software should not send a DNS query.
- [Firefox DNS-over-HTTPS source documentation](https://firefox-source-docs.mozilla.org/networking/dns/dns-over-https-trr.html),
  accessed 2026-08-25: DoH may use a resolver separate from platform DNS, and
  excluded domains are a browser-specific control.
- [Firefox `proxy.onRequest`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/proxy/onRequest),
  [ProxyInfo](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/proxy/ProxyInfo),
  and the [proxy API](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/proxy),
  accessed 2026-08-26.
- [Firefox HTTP logging documentation](https://firefox-source-docs.mozilla.org/networking/http/logging.html),
  accessed 2026-08-26: `nsHostResolver` logging records host-resolution work
  and Firefox supports `MOZ_LOG`/`MOZ_LOG_FILE` for a fresh-process trace.
- [Firefox native manifests](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_manifests),
  [native messaging](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_messaging),
  and [`webRequest.onAuthRequired`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/webRequest/onAuthRequired),
  accessed 2026-08-26.
- [Firefox `runtime.onInstalled`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/runtime/onInstalled)
  and [`tabs.create`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/tabs/create),
  accessed 2026-08-26.
- [Windows Packet Monitor command reference](https://learn.microsoft.com/en-us/windows-server/networking/technologies/pktmon/pktmon-syntax),
  accessed 2026-08-26: the in-box tool can filter by UDP/TCP port and records
  packet captures; a port-53-only capture can test plaintext DNS but cannot by
  itself prove the absence of encrypted DoH.
- The maintained H4-3A Reference connection and the selected H4-3B contract,
  inspected 2026-08-26: the current alpha proxy accepts only a route already
  authenticated for one live C-2 connection; H4-3B, not a name mechanism,
  owns application-transparent bidirectional Publisher-Service traffic.
- [Let's Encrypt DNS-01 documentation](https://letsencrypt.org/docs/challenge-types/),
  accessed 2026-08-25, and its
  [Certificate Policy](https://letsencrypt.org/documents/isrg-cp-v3.1/): a
  free public certificate is possible only for a controlled public FQDN;
  wildcard issuance uses DNS-01 and public CAs do not issue Internal Names.

### Experiment

For each candidate, use a disposable Ubuntu and Windows environment with a
clean browser profile. Record resolver queries, DoH behavior, visible URL,
TLS result, listener ownership, ordinary-domain control request, Endpoint
stop/withdrawal, and removal/restoration. A pass must exercise a dynamic
generic H4-3B fixture, not only the static Reference Site.

### Failure scenarios

- `site.ard` collides with a later public delegation or a local network name.
- The browser's DoH path leaks the name, bypasses the Endpoint, or reaches an
  ordinary public target.
- A local listener takes port 80/443 without explicit ownership, blocks another
  user service, persists after Endpoint removal, or requires an undisclosed
  elevation/trust-store change.
- Certificate issuance, a local CA, or an HTTP downgrade makes the visible
  origin misleading.
- An address lookup or rendered content chooses a different Target or turns the
  Browser Entry into a generic proxy.

## Findings

- **Sourced fact:** `.onion` is not merely a name record; Tor-aware software
  handles it directly or through SOCKS. A naive normal-browser DNS lookup is
  explicitly the wrong model.
- **Sourced fact:** `.localhost` receives special loopback treatment, which
  explains why the H4-4A compatibility fixture can work without a DNS server.
  That property does not apply to `.ard`.
- **Sourced fact:** Firefox may resolve a name through DoH rather than the OS
  resolver; a local resolver or hosts entry alone is therefore not a complete
  supported-browser design.
- **Inference:** a Name Service is necessary but insufficient. A visible
  no-port name additionally needs a browser-resolution path, a default-port
  listener, and, for HTTPS, a trust model.
- **Inference:** a private `.ard` suffix cannot be selected as a durable
  ordinary-browser scheme without an explicit collision and browser-handling
  decision. It is an experience target, not a current protocol allocation.
- **Sourced fact:** Firefox's `proxy.onRequest` runs before `webRequest`,
  requires `proxy` plus only the matching host permissions, and can return a
  local proxy followed by `null` to stop browser-defined proxy fallback. A
  listener filtered only to `.ard` therefore need not alter the choice for an
  ordinary Internet URL. The API is Firefox-specific: Chromium exposes a
  materially different proxy-settings API.
- **Falsification measurement (2026-08-26):** a fresh Firefox 154 profile
  with the maintained fixed-ID add-on and Endpoint-owned HTTP proxy wrote
  `NameLookup`, `NativeLookup`, and `getaddrinfo` records for both
  `reference.ard` and `unavailable.ard` under `nsHostResolver` logging. An
  `example.com` control produced the same resolver-family records. The local
  fallback proxy received no `.ard` request, but that proves only proxy
  selection—not absence of name resolution. This is Firefox-internal evidence,
  not a packet capture and not a claim about every on-wire packet. It rejects
  the HTTP `proxy.onRequest` Browser Entry as a way to meet R-115's no-ordinary
  DNS/DoH criterion.
- **Negative exploratory observation (2026-08-26):** a disposable SOCKS
  `proxyDNS: true` variation did not receive a SOCKS request before the same
  Firefox resolver records appeared. It selects no alternative and its source
  has been removed; a future candidate needs its own reproducible evidence.
- **Sourced fact:** an extension may use per-user native messaging to obtain
  the current loopback endpoint, but the native-host manifest must restrict an
  exact extension ID. Firefox finds that manifest through a per-user registry
  key on Windows and a per-user directory on Linux. This is integration that
  the installed Endpoint can own; it is not a DNS setting or browser fork.
- **Inference:** a narrow Firefox add-on can make an `http://name.ard/`
  alpha origin reachable without binding local port 80/443 or modifying OS DNS.
  It does not solve `https://name.ard/`: an HTTP proxy receives a TLS CONNECT
  for that form and the browser still needs a certificate trusted for `name.ard`.
  The alpha candidate must intercept and fail HTTPS locally rather than leak or
  fall back, until a later TLS/origin design is selected.
- **Current-code fact:** the maintained alpha proxy has one route only after
  an Endpoint authenticates one exact binding and opens its C-2 Reference
  connection; it rejects all other `.ard` hosts and is withdrawn with that
  connection. It is therefore not an address-bar resolver or a generic
  Publisher-Service bridge.
- **Implementation fact:** the maintained `browserentry` module owns the
  alpha Firefox native-host contract. An Endpoint with an explicit per-user
  Browser Entry state path creates two separate random values with its loopback
  `AlphaProxy`: a probe capability and a proxy credential. It publishes the
  current port and both values only while an exact authenticated route is live,
  and clears the state before the route/proxy stops. The native host accepts
  only bounded port and proxy-authentication messages; either reprobes the
  exact loopback proxy with the probe capability. Only the latter returns the
  current port and separate proxy password. It cannot read or return a Service
  Name, Target, route, Service credential, or browser URL. Thus a stale state
  file cannot mistake an unrelated loopback service that merely returns `204`
  on a recycled port for the Endpoint proxy, nor receive the proxy password.
  This is not an isolation claim against another process that can read the same
  per-user state file.
- **Implementation fact:** under [ADR-0044](../../adr/0044-revalidate-browser-entry-proxy-authentication.md),
  the fixed-ID extension has `proxy`, `nativeMessaging`, `webRequest`, and
  `webRequestBlocking` permissions, all web-request listeners limited to `.ard`.
  The local proxy first gives normal requests `407 Basic`. The extension calls
  the native host again, requires its reproved port to equal the loopback
  challenger, and answers at most once. The proxy strips that authentication
  header before it reaches the selected local Publisher presentation. Its
  unavailable-host result remains an unavailable loopback proxy, not a
  DNS/direct/browser-proxy choice.
- **Measurement:** `make qualification-h4-4a-firefox` passed on 2026-08-26
  with Firefox 154 after this maintained implementation. The fresh temporary
  profile loaded the exact alpha HTTP origin through the fixed-ID add-on,
  native host, fresh `407` revalidation, and Endpoint proxy; its ordinary
  fallback control remained separate and no `.ard` request reached that
  fallback. The R-117 fixture's mismatched-port negative control remains the
  evidence that it declines rather than leaks a credential on a changed
  challenger. `web-ext@10 lint` reported zero errors, notices, and warnings.
- **Limitation:** this narrows a port-rebinding handoff race; it cannot make
  the browser/native-host/proxy handoff atomic or isolate hostile same-user
  processes and browser-profile controllers.
- **Inference:** a signed Firefox integration would improve the presentation
  of a pre-opened alpha session, but cannot independently make arbitrary
  WordPress or other dynamic Services usable. That needs the selected H4-3B
  payload-transparent connection first; adding content-aware adaptation here
  would violate the Product Owner's no-special-mechanism requirement.

## Options

1. **Zone-scoped Browser Entry under a project-controlled public domain.**
   Use a visible subdomain such as `name.<project-domain>` with a free
   DNS-01-issued public wildcard certificate, plus an explicit, zone-only local
   resolution/listener design. It avoids a made-up top-level suffix, but makes
   public-domain and certificate-transparency facts part of the user claim.
2. **Private `.ard` suffix with local resolver and local trust.** It matches
   the short name but needs browser/OS handling, collision ownership, a
   default-port listener, and a local HTTPS-trust decision. It cannot rely on
   public Web PKI today.
3. **Firefox-first zone-scoped add-on with native messaging. Rejected for the
   no-leak requirement.** A narrowly
   permitted Firefox add-on intercepts only `http(s)://*.ard/`, asks a
   per-user native host only for the running Endpoint's ephemeral loopback
   proxy port, then returns that proxy followed by a terminal `null` failover
   entry. The alpha experiment has no proxy credential: Firefox 154 did not
   deliver the documented proxy-authorization header, and broad request
   interception is rejected. It leaves all non-Ardents URL routing to Firefox
   and any user-selected proxy. It avoids system resolver installation and
   port 80/443, but adds add-on signing, update, native-host installation,
   Firefox-specific support, and a separately selected HTTPS design.
4. **Retain the former local-origin presentation only.** Rejected as the
   normal participant address by the Product Owner; keep solely as H4-4A
   evidence.

## Experiment result — PAC routing analogue

On 2026-08-26, a clean temporary Firefox 154 profile on the Windows
workstation loaded a local PAC rule for `*.ard`. It opened
`http://reference.ard/` through an ephemeral `127.0.0.1:62372` HTTP proxy and
that proxy captured `GET http://reference.ard/ HTTP/1.1` with
`Host: reference.ard`; Firefox's screenshot rendered the proxy's synthetic
response. No listener used port 80 or 443, and the profile did not modify the
user's Firefox profile, OS resolver, hosts file, certificate store, or system
proxy. The reproducible disposable script and its limitations are in
[`experiments/r-115-firefox-zone-proxy`](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-115-firefox-zone-proxy/).

This is a useful falsification result: Firefox does not need a successful
public DNS answer to make the named HTTP request when its routing policy sends
the name to loopback first. It is deliberately **not** a no-DNS/DoH-leak proof:
the experiment captured no packets. It also did not install an add-on, use
native messaging, authenticate the local proxy, preserve an existing user
proxy, exercise unavailable-proxy fallback, or make an HTTPS request.

The Manifest V3 Firefox add-on fixture passes `web-ext@10` lint with zero
warnings. Its first temporary-install attempt was inconclusive because
`web-ext --start-url` opens a URL while Firefox starts, before the temporary
add-on is installed. The fixture now uses `runtime.onInstalled` to open its
test-only probe only after registering `proxy.onRequest`; Firefox documents
that event for temporary `web-ext run` installations and documents
`tabs.create` for exactly that controlled test action.

On 2026-08-26, Firefox 154 under a fresh temporary profile temporarily
installed that fixture and captured `GET http://reference.ard/ HTTP/1.1` at
its assigned `127.0.0.1:58453` proxy. The fixture served only
`reference.ard`; its `unavailable.ard` request and its HTTPS
`CONNECT reference.ard:443` received a local synthetic `502`. The fixture
profile also had a separate temporary HTTP fallback proxy at
`127.0.0.1:58454`: it received the deliberate ordinary
`http://ordinary.invalid/` probe but no `.ard` request, including the HTTPS
probe. Thus this execution proves the selected `proxy.onRequest` filter
executed for the alpha zone and its `[{localProxy}, null]` chain did not fall
back to that browser-defined proxy. It used no listener on 80 or 443, no daily
Firefox profile, and no system DNS, proxy, certificate, hosts, route, or VPN
change. The reproducible launcher is
[`run-addon.ps1`](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-115-firefox-zone-proxy/run-addon.ps1).

This remains bounded evidence. It does **not** prove an unavailable `.ard`
request did not issue a DNS/DoH packet or attempt another direct network path;
it proves only that it did not use the configured browser fallback proxy. It
also does not supply local proxy credentials, signed add-on release
provenance, successful HTTPS/trust, or participant lifecycle.

The same fixture then enabled `nativeMessaging` and replaced its direct port
constant with `runtime.sendNativeMessage`. Its temporary host accepted one
framed `loopback-proxy-port` request and returned only its assigned loopback
port; its native manifest allowed only `r115-fixture@ardents.invalid`. On
Windows it was registered under one exact temporary
`HKCU\Software\Mozilla\NativeMessagingHosts\org.ardents.r115.fixture` key,
which the launcher verified absent after cleanup. On 2026-08-26 Firefox 154
used this route to send the same HTTP request, unavailable HTTP request, and
HTTPS `CONNECT` to loopback port `64711`, while the separately configured
fallback proxy at `64712` still received only ordinary traffic. This proves
the documented Firefox per-user manifest/allow-list handoff on this Windows
profile. It does not prove a maintained native-host protocol, its secrecy,
crash/restart behavior, Linux location, or installer/removal ownership.

**Plain-DNS observation preparation:** the Windows qualification host has
`PktMon.exe`, and its documented filter syntax can constrain a capture to UDP
and TCP port 53 before the fresh-profile run. On 2026-08-26 the current test
session was denied access to the PktMon driver while merely listing filters, so
no packet capture was started and no DNS inference is recorded. A later run
needs an explicitly authorized capture-capable Windows session, a new temporary
evidence directory, a plain-DNS port-53 baseline/control, capture cleanup, and
separate treatment of DoH. An empty port-53 capture would establish only that
plain DNS was not observed in that time window; it would not establish no-DoH
or no other browser egress.

**Maintained alpha-path qualification:** on 2026-08-26, a fresh temporary
Firefox 154 profile loaded the maintained fixed-ID add-on and obtained the
ephemeral port of an Endpoint-owned `reference.AlphaProxy` through the
maintained Go native host and its current state/probe. The proxy's `407`
challenge then caused a fresh proof before the extension received its separate
one-process credential. It loaded `http://reference.ard/` through that proxy
and the registered local Reference
origin served its declared document, stylesheet, and SVG. The same run proved
that the profile's ordinary URL used its separate fallback proxy, while no
`.ard` HTTP or HTTPS request reached that fallback; the temporary HKCU native
manifest was removed afterwards. The proxy had been given exactly the one
`reference.ard` route by the Endpoint; it was neither a default-port listener
nor an Internet proxy. The Windows-only qualification is
`TestFirefoxBrowserLoadsAlphaOriginThroughEndpointProxyQualification`, invoked
by `make qualification-h4-4a-firefox`. This is evidence that the maintained
alpha proxy can supply the visible no-port HTTP origin; it remains neither a
signed participant extension nor a DNS/DoH, HTTPS, generic-application,
installation, update, or removal proof.

**Dynamic C-2 Browser Entry qualification:** on 2026-08-26 the same temporary
Firefox 154 profile and maintained native host drove the H4-3B process
fixture's selected `http://reference.ard/` connection. After the User Endpoint
authenticated that exact alpha binding and published its current proxy port,
probe capability, and separate proxy credential, Firefox received a loopback
`407`, reproved the current port through the native host, and only then loaded
a normal Publisher document, submitted its HTML form, followed the redirect and
cookies, consumed the chunked response, and navigated to the Publisher's close
URL. The separate Publisher process
accepted that unchanged HTTP/1.1 sequence and wrote the browser-specific proof;
the C-2 User then required the Endpoint to remove its Browser Entry state at
Service withdrawal. The fixture's Go HTTP client is disabled in this leg.
This proves one browser-driven dynamic protocol shape through the selected
Service, not WordPress compatibility, multi-connection/concurrent request
support, arbitrary applications, or an installed participant workflow.

**Rejected alpha refinement:** the fixture also tested the documented
`ProxyInfo.proxyAuthorizationHeader` property with a fresh native-host value.
Firefox 154 sent neither that header on ordinary HTTP proxy requests nor on
the HTTPS `CONNECT`; the loopback proxy failed closed with `407`. ADR-0044
therefore selected the experimentally proven narrow alternative rather than a
broad interception: `.ard`-only `webRequest.onAuthRequired` obtains a fresh
native proof, matches its port to the loopback challenger, and receives only a
separate local proxy credential. It remains no same-user isolation boundary.

## Recommendation

Close the Firefox HTTP-proxy candidate as a functional compatibility trace,
not a participant Name Service. Its no-leak criterion is falsified. If named
browsing is reopened, begin a new decision record for an Endpoint-owned,
zone-scoped system resolution path on each supported OS, including the default
port listener, collision/removal, Firefox DoH behavior, and separately chosen
HTTP/HTTPS trust contract. Do not revive this add-on merely because it renders
the desired URL.

**Confidence:** high that a generic Browser Entry is required and that the
current Firefox proxy candidate cannot make the required no-leak claim; low on
a final cross-browser HTTPS origin until the platform design is selected.

## Disposition

Closed for the current H4-4 alpha slice. The current alpha corpus and
private-resolution work remain valid. A provisional maintained trace maps
one authenticated alpha binding to `http://<name>.ard/` through an
Endpoint-owned exact-name loopback HTTP proxy. It rejects unregistered names
and HTTPS `CONNECT` locally, and does not select a Target or permit an upstream
destination. The exact Mozilla-signed XPI has now been explicitly installed in
one clean Firefox Release profile and passed the dynamic C-2 route. This is not
a participant-ready journey until an enrolled release binds the participant's
components and install/replacement/removal, direct DNS/DoH observation, and
ordinary-browser coexistence are qualified. The resolver trace now establishes
that the Firefox HTTP-proxy route does not meet that requirement, so it is
retained only as signed-XPI and dynamic-C-2 compatibility evidence—not a
participant-ready Browser Entry. The temporary Firefox/native-host fixture and
signed-profile run do not themselves resolve those remaining product decisions.
They also do not remove the
H4-3B gate: a final Browser Entry may route a name only after the generic
connection can carry the selected Publisher Service without static-site, CMS,
or content-rewriting special cases. [ADR-0045](../../adr/0045-firefox-first-unlisted-browser-entry-delivery.md)
now selects a fixed Firefox-only profile: Mozilla-signed unlisted XPI,
enrollment-v4 host/XPI provenance, normal Endpoint state configuration, and an
owned-only per-user native-manifest installer/remover. The repository contains
the selected Mozilla-signed XPI provenance and clean-profile route evidence,
but no enrolled participant release or HTTPS trust model. It merely presents a
pre-opened exact alpha session at
`http://<name>.ard/`; its C-2 qualification now also carries one ordinary
dynamic HTTP browser flow without rewriting, but it does not make `.ard`
ordinary DNS.
