# H4-3 — User, Service, and web-access path

Status: **closed for the selected RC2 bounded functional-alpha profile. The
H4-3B application-transparent HTTP/1.1 path has accepted multi-host soak/fault
evidence for normal traffic, publication withdrawal, Publisher Application,
Endpoint, Carrier, and product-Node loss, with exact terminal classes and no
destination fallback. H4-4 Browser Entry is not on this epic's critical path;
capacity, availability, public deployment, and H4-7 browser claims remain open.**

## Decision

H4-3 is the first complete user-facing proof of Ardents. Its alpha outcome is
not a local API, a raw byte-stream demo, or a single controlled HTTP response:

> A Publisher uses Ardents to expose one local web Service, shares an explicit
> Ardents destination with a User, and that User opens and uses the site in an
> existing browser through the live H4-2 network.

The Publisher-to-browser loop is the definition of done. H4-1 supplies an
installed/running Endpoint profile; H4-2 supplies a live remote network path;
H4-3 proves that people can use the result without learning Route internals.

## The alpha loop

```text
Publisher's local web server
  -> Publisher Endpoint publishes one Service Instance
  -> explicit Target Link is shared out of band
  -> User starts their Endpoint and selects the link
  -> local browser Adapter opens an authenticated Service Connection
  -> existing browser renders the Publisher's site
```

Ardents is application-payload neutral. One selected local Publisher Service
may have arbitrary application behavior; HTTP is Application Data, and the
Carrier neither interprets, caches, indexes, replicates, sanitizes, nor assigns
meaning to it. The Publisher owns the site's authentication, authorization,
content, storage, and application behavior. A CMS, API, or other ordinary
Service needs no Ardents-specific content adapter, exporter, or rewriter.

The current Reference Site accepts one HTML page and same-Service static
resources only to make H4-3A's first request-boundary evidence small and
reproducible. That fixture is not a product content policy and does not narrow
what a Publisher Service may carry in the transparent-connection slice.

## Participant journeys

### Publish

1. The Publisher starts the unprivileged Endpoint profile from H4-1 and waits
   for **Publish Ready** or a classified failure.
2. The Publisher starts a local web Service. It accepts work only through the
   authorized local Ardents attachment; no ordinary public listener is implied.
3. Through separately authorized Service Administration, the Publisher binds
   the local Service Instance to a Service Target and publishes its current
   reachability/Instance facts.
4. The Publisher receives a shareable Target Link. It is an exact,
   location-independent destination, not a Node ID, IP address, DNS name, or
   discovery listing.
5. Withdrawal, local service stop, expired credentials, or unavailable Routes
   become explicit unavailable outcomes. They never pretend that an HTTP
   request was delivered.

### Open

1. The User starts the H4-1 Endpoint and waits for **Target Connect Ready** or
   a named failure.
2. The User supplies the exact Target Link to the supported Ardents browser
   integration. The link never falls back to DNS, search, a local alias, HTTP,
   or another destination.
3. The local Adapter asks the Endpoint for an authenticated Service Connection.
   H4-2 Route/Entry recovery either preserves the connection under its declared
   contract or returns an explicit result.
4. The Adapter passes the site's HTTP bytes to the existing browser. The User
   sees an exact-target authentication, unavailable Service, Route failure,
   local denial, timeout/cancellation, or close as distinct Ardents outcomes;
   it does not receive Node topology or raw route diagnostics.

## Browser Adapter boundary

The browser cannot speak the Ardents local Application Interface directly. The
Adapter is therefore a local compatibility Module between an existing browser
and the Endpoint. H4-3A selects an explicit Endpoint-to-loopback handoff, not a
browser proxy, extension, custom CA, URI registration, or browser fork:

1. the User explicitly gives one exact Target Link to the Endpoint;
2. the Endpoint authenticates that Target and creates one scoped Service
   Connection before involving the browser;
3. the Endpoint creates a new listener on `127.0.0.1` with an ephemeral port
   and a fresh, unguessable path value of at least 128 bits;
4. one listener/origin represents one active Service Connection, and the
   Endpoint opens only that exact local URL in the selected existing browser;
5. the Adapter maps only origin-form browser traffic for that established
   Service Connection. It accepts no ordinary URL, proxy-form target, arbitrary
   Ardents destination, or Endpoint-administration request. The H4-3A
   Reference fixture may additionally limit methods and resources, but that is
   not a Carrier or Publisher-Service restriction; and
6. closing the Service Connection withdraws the listener and path. A reload
   then fails locally rather than reaching DNS, search, public HTTP, or another
   Service.

The random local path is defence against accidental or blind local access, not
a Service credential or a malicious-same-user isolation claim. It is visible
to the browser and may appear in browser history. It authorizes only the
already-selected presentation connection and never a Target choice, Local
Grant, Service Administration action, or Endpoint control operation.

The Adapter has three non-negotiable properties:

1. **Selective:** only an explicit Ardents destination uses it. Ordinary
   Internet browsing remains on the browser's ordinary path; H4-3 does not
   install a system-wide proxy, DNS, route, or VPN setting.
2. **Local and authorized:** it exposes only a scoped local attachment to the
   User's Endpoint. It has no Service Authority, Node role, remote
   administration, or ambient permission to proxy arbitrary destinations.
3. **Honest:** it reports compatibility-only operation unless H4-7 independently
   qualifies its complete browser/Application process tree. A browser extension
   or local proxy alone is not an Application isolation boundary.

The H4-3A browser address is deliberately local HTTP such as
`http://127.0.0.1:<port>/site/<opaque>/`; it is presentation plumbing, not the
remote Service identity. No TLS certificate, public hostname, Ardents CA, or
trust-store change is required. The Endpoint remains the trusted identity UI:
it must show the selected Target and authentication result outside
Publisher-controlled page content before opening the browser. The H4-3A
fixture adds its selected static response policy and exposes no privileged
status page in the rendered origin. A transparent Adapter must likewise expose
no privileged Endpoint surface there, but it must not transform a Publisher
response or impose a content-type-specific policy.

The maintained H4-3A Reference Site fixture accepts only GET/HEAD for one declared HTML
page and its same-Service CSS/images. Every document response adds a
header-delivered CSP with `sandbox allow-same-origin`, `script-src 'none'`,
`connect-src 'none'`, same-origin-only image/style fetches, and denial of
objects, base changes, forms, frames, workers, and framing. It also uses
`no-store`, `no-referrer`, and `nosniff`, rejects redirects, and does not pass
Publisher cookies into the local origin. The sandbox directive is required:
fetch directives alone do not bound HTML navigation. This is a deliberately
small Reference corpus, not a claim that arbitrary Publisher HTML is safe or
private, nor a limitation on a Publisher Service. A separate extension or URI
scheme may later improve copy/click UX, but it cannot replace this
authorization boundary and is not an H4-3A dependency.

## H4-3A: usable-alpha web Service

### Entry conditions

- H4-1 has a selected alpha Endpoint profile for both Publisher and User.
- H4-2 has one selected live multi-host profile capable of Target Connect and
  Publish readiness, or has retained equivalent evidence for the exact
  alpha-network configuration.
- The selected local Application Interface and Service publication path expose
  separate Connection and Service Administration authority.
- One supported browser/profile and one local Adapter mechanism are selected
  for the alpha. The Adapter mechanism is fixed above; the browser/profile is
  still selected by a live qualification run. Other browsers are unsupported
  rather than silently different.

### Done when

On separate Publisher and User endpoints, a clean User can receive an explicit
Target Link and use the selected browser to render the bounded Reference Site
through the live network. The evidence includes the exact endpoint/browser/
Adapter release, Target authentication result, first page plus declared
same-Service resource loads, readiness/failure result, and a Publisher
withdrawal/offline case.

The result is still valid if the page is ordinary HTTP Application Data inside a
Service Connection. It must not claim browser-level privacy, public naming,
content replication, offline delivery, a generic Internet proxy, or a global
HTTPS identity system.

### Current qualification evidence

- The maintained `reference-c2` process fixture runs Publisher, its token-bound
  local Reference Site Application, User, Gateway, Initiator, Introduction,
  Rendezvous, and Responder as distinct processes. It proves the exact
  Target-Link authentication result, the page and declared same-Service CSS/SVG
  loads, response policy headers, clean termination, Publisher-offline result,
  and local-Application token refusal.
- The exact three-scenario fixture passed locally and in a clean Linux Docker
  container on 2026-08-25. This is separate-process/Linux-runtime evidence; it
  is not a multi-host or independent-operation claim.
- An opt-in Windows run opened the authenticated scoped URL in explicitly
  selected Firefox 154 and observed all three Publisher resources. It is only
  compatibility evidence for that browser/version and this bounded fixture;
  it is not H4-7 isolation or location-privacy evidence.
- The same `ab78f257` source passed all three scenarios in a separate limited
  Go 1.26.6 Docker container on the accepted project Ubuntu VPS on 2026-08-25:
  Docker 29.4.1, `2 GiB`, `1 CPU`, `128` PIDs, no published ports, read-only
  source mount, exit code `0`, and `45.350s` total. The temporary container and
  archive were removed after capture. This closes the selected VPS Docker
  profile only; it is neither independent-operation, public-deployment,
  capacity, browser-privacy, nor multi-host evidence.

### Stop conditions

Stop and narrow the slice if opening the site requires a browser fork, a
machine-wide proxy/DNS/VPN change, a public CA, a hidden clearnet fetch,
unbounded local proxy access, direct publisher listener, or a special Route
that bypasses H4-2 authentication/recovery rules.

## H4-3B: application-transparent Publisher Service

**Goal:** prove that the selected Service Connection carries an ordinary
Publisher web Service without an Ardents content profile. The connection must
preserve request and response semantics bidirectionally; it must not recognize
WordPress, CMS markup, API schemas, static files, or any other application
type.

**Required behavior:** after the Endpoint has selected and authenticated one
Service Target, the Browser Adapter carries that origin's request methods,
headers, bodies, cookies, redirects, and streamed responses to and from that
one Service. Resource ceilings, timeouts, cancellation, and failure reporting
remain Route/Endpoint resource controls; they must be stated independently of
application content. No HTML rewriting, static export, CMS plug-in, content
filter, or Ardents-specific website mechanism is introduced.

**Done when:** a separate Publisher and User run an unmodified dynamic fixture
behind one selected Service through the H4-2 path. The evidence covers a
state-changing request, cookie round-trip, redirect, response body streaming,
Publisher withdrawal, and refusal to use the Adapter as an arbitrary Internet
proxy. A WordPress container may be a compatibility workload, but it is not a
protocol dependency or special integration.

**Current tracer:** the maintained C-2 fixture now has a separate dynamic
Publisher Application mode. Its alpha User uses one authenticated Binding and
one selected Service Connection to issue a form `POST`, retain a cookie over a
`302`, make a second request, and read a chunked response. Its local bridge is
HTTP/1.1 and orders concurrent browser requests on that one connection; it
rejects HTTP Upgrade. When that Publisher Application closes after its final
same-Service request, the native terminal closes the User presentation and a
further name request fails locally. The fixture also proves that neither an
unregistered `.ard` name nor an ordinary Internet name becomes another
Publisher destination. A separate explicit-withdrawal scenario consumes a
fresh Service Administration capability after the active connection drains,
checks the exact Target/generation, and reports `unpublished`; the User reports
`clean service connection close`. The bounded failure campaign then resets the
Publisher-local Application socket after a partial response and separately
hard-stops the Publisher Endpoint during an active request. Both failures give
the surviving User `abrupt connection loss`; the Application-reset case gives
the Publisher the same class. A behavior cell keeps a second, distinct Target
live and registered while the requested Target fails: the decoy receives no
request until its own name is explicitly requested. The process cells also
reject an unregistered alpha name and an Internet destination. After response headers are
committed, the local HTTP server may expose a truncated body rather than a
second HTTP status; the classified Endpoint terminal remains authoritative.

The exact cross-built H4-3B command and fixture bytes now pass all four dynamic
terminal cases in a read-only, network-isolated Linux Docker cell at `1` vCPU,
`1 GiB`, and `128` PIDs. That runner also verifies cached and two-fresh-root
H4-6A control observations before it reports the C-2 results. This is local
Docker evidence, not yet the full release promotion: the same exact candidate
still needs the declared VPS repetition, selected platform/browser, and actual
multi-host qualification. The optional H4-4 named Browser Entry remains
compatibility evidence and is not an H4-3B completion dependency.

**Alpha HTTP/1.1 limits:** the transparent origin accepts at most `16 KiB` of
request head and `1 MiB` of request body, with a `1 s` header-read timeout and
`5 s` idle timeout. A known oversized request receives `413` before it can
reach the selected Publisher Service; a streaming request that crosses the
same body boundary closes the selected connection and reports `413` while no
Browser response headers are committed. Publisher response head is likewise
limited to `16 KiB`; exceeding it closes the selected connection and produces
`502` before Browser headers are written. A known response body above `1 MiB`
does the same. A chunked response is streamed, never buffered as a whole, but
once it exceeds `1 MiB` the visible response terminates after the accepted
prefix; it cannot be rewritten with a second status after headers are
committed. These are alpha availability/compatibility limits, not a general web
hosting profile or a throughput claim.

**Boundary:** this slice makes no browser-isolation, external-origin,
anonymity, or zero-latency claim. A page's ordinary external requests remain
ordinary browser traffic unless a later, separately qualified application
profile says otherwise.

## What follows H4-3A

| Later slice | Purpose |
|---|---|
| Named web destination | H4-4 binds a canonical Service Name to the same Target path; Target Links remain complete until then. |
| Browser ergonomics | Add explicit URI handling, extension/launcher UX, bookmarks, status, and selected browser support without changing the Carrier contract. |
| Protected browser mode | H4-7 qualifies a specific browser/Application process tree against DNS, WebRTC, external-resource, storage, and same-user escape paths before any Application-level location claim. |
| Application-transparent web Service | H4-3B proves ordinary bidirectional Publisher-Service traffic without content-specific Ardents mechanisms. |

### Open follow-up: a user-facing browser origin

The H4-3A loopback URL is an Adapter handoff, not the desired user-facing
address scheme. Before a later alpha slice, evaluate a browser presentation
where a user opens a Service through a stable-looking origin instead of
`http://127.0.0.1:<port>/site/<opaque-target>/`. The desired candidates are a
named form such as `https://site.ard/` and, only if a named destination is not
ready, an opaque HTTPS form such as `https://<opaque-target>/`.

This is an explicit research and design question, not an H4-3A completion
condition or a decision to use either form. In particular:

- an HTTPS origin needs a browser-accepted identity and must not depend on a
  certificate warning, self-signed certificate, silently installed local CA,
  or an unexamined trust-store change;
- `site.ard` needs an explicit binding and browser-resolution model, so it
  intersects H4-4 naming rather than silently becoming public DNS, a
  machine-wide resolver change, proxy, or VPN; and
- any presentation must preserve the exact Target and State-authorized path,
  keep the Adapter scoped to that Service, and not grant arbitrary origin or
  URL authority.

The next proposal must state the user-visible name, resolution mechanism,
browser trust mechanism, failure behavior, and the H4-4/H4-7 boundaries before
one of these forms is selected.

## Non-goals

- A bundled browser, browser fork, public DNS suffix, clearnet exit, generic
  anonymous Internet proxy, content safety engine, site replication, or public
  Service directory.
- Automatic browser privacy: arbitrary external resources, callbacks, DNS,
  WebSocket/WebRTC, QUIC, direct sockets, browser fingerprinting, credentials,
  and content behavior remain outside the generic Adapter claim.
- Permissionless naming, leases, recovery, governance, or search; these belong
  to H4-4.

## Current technical inputs

- [Product journeys](../journeys.md) defines the generic Application and
  controlled Named Unlisted Site journeys.
- [Endpoint and Service runtime](../../technical/endpoint-service-runtime.md)
  owns the current Endpoint and Service boundary.
- [Operating model](../operating-model.md) defines the local Application
  Interface, generic Adapter limitation, and later Network-Isolated Application
  Boundary.
- [H4 scope](../scope.md) owns this epic's public-alpha and Public Beta claim
  boundary.
