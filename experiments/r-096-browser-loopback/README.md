# R-096 — Existing-browser loopback fixture

## Question

Can one selected browser render a bounded Publisher fixture through a separate
one-Service loopback Adapter origin while exact route policy and CSP prevent
ambient Adapter access, inline script, redirect, and external resource fetches?

## Hypothesis

The fixture creates a publisher-only loopback server, a distinct Adapter
listener on `127.0.0.1` with an ephemeral port and fresh 128-bit path
capability, and a third loopback HTTP sentinel addressed through the
out-of-origin hostname `localhost`. The Adapter maps only GET/HEAD for one page,
stylesheet, and image to the exact publisher listener, bounds every response,
strips publisher headers, and adds a static-only response policy. A current
browser renders those same-Service resources while CSP prevents inline script
and the sentinel image fetch. No administrative route, arbitrary URL, proxy
setting, DNS suffix, or fallback is exposed.

## Predeclared browser oracle

The caller creates fresh absolute readiness and event-log paths outside the
repository. The fixture refuses occupied paths, writes one private JSON
readiness record, and appends bounded JSON request observations for Adapter,
Publisher, and the external-request sentinel. The browser is given only the
Adapter URL; the Publisher URL and event log never enter page content. Adapter
observations include the temporary User-Agent needed to identify the tested
browser profile.

The passing slice requires:

1. the actual browser/version, final URL/origin, DOM title, `isSecureContext`,
   computed same-Service CSS, same-Service image completion, absent inline
   marker, absent external image, cookie state, console CSP result, and browser
   network requests are captured;
2. the Adapter observes one exact page plus declared same-Service resources,
   and the Publisher observes the corresponding translated paths; no request
   reaches an administrative, arbitrary-target, proxy, or external-fetch route;
3. the page response has the exact restrictive CSP, `no-store`, `no-referrer`,
   and `nosniff` policy, exposes no `Set-Cookie`, and never redirects;
4. a wrong path capability and a disallowed method/path return an explicit
   local rejection without contacting Publisher;
5. after the Adapter process stops, reloading its exact URL yields a browser
   connection failure rather than DNS, search, public HTTP, or another target;
   and
6. all fixture processes join and the caller removes only its exact readiness,
   event, stdout, stderr, and build files.

The result is falsified by an external request in browser instrumentation, an
inline script marker, publisher contact for a rejected route, a redirect or
ordinary-URL forwarding path, a non-loopback listener, a stable/reused path
capability, or successful access after process stop. Browser-generated
requests not declared above must be recorded and classified, not ignored.

### Navigation counterexample

Fetch-oriented CSP directives do not by themselves establish a generic
navigation allowlist. A second predeclared probe adds an immediate Publisher
`meta refresh` and an external link to the local out-of-origin sentinel. Run it
once under the base policy and once with the header-delivered CSP directive
`sandbox allow-same-origin`. The base policy is falsified if the sentinel is
reached. The sandbox refinement passes this narrow case only if the page and
same-Service resources still render, neither automatic refresh nor a deliberate
link click reaches the sentinel, and the browser remains on the Adapter origin.
This probe still does not make arbitrary Publisher HTML safe or private.

## Run

```sh
go run main.go -ready /absolute/path/to/ready.json -events /absolute/path/to/events.jsonl

go run main.go -navigation-probe -csp-sandbox=false -ready /absolute/path/to/base-ready.json -events /absolute/path/to/base-events.jsonl

go run main.go -navigation-probe -ready /absolute/path/to/sandbox-ready.json -events /absolute/path/to/sandbox-events.jsonl
```

The program writes exactly one loopback URL plus non-secret profile facts to the
readiness file, then serves until stopped. It creates no Authority, opens no
public listener, and does not contact external hosts. The browser test inspects
browser-visible DOM/console state and the separate server event log. The
external image points to the local out-of-origin sentinel; CSP must block it
before any HTTP request reaches that listener.

## Result — 2026-08-24

The initial exact-route Windows fixture binary had SHA-256
`d08ce4f617bfd6c50173befe78897ba921039fab2f36f47ac71071df3f5e3ada`.
Two starts emitted different 128-bit path values. In the final run, the Codex
in-app browser sent a Chrome `151.0.0.0` Windows User-Agent and rendered the
page, CSS (`rgb(1, 2, 3)`), and 8-pixel SVG. The inline marker remained false,
the out-of-origin image had `naturalWidth=0`, and the sentinel received zero
requests. The browser-side Adapter log contained only the page, stylesheet,
and SVG; the SVG appeared twice because it was both the image and favicon.

The browser-facing 200 response had the exact declared CSP, `no-store`,
`no-referrer`, `nosniff`, and `Cross-Origin-Opener-Policy: same-origin`; the
Publisher's deliberately supplied `Set-Cookie` and `Location` headers did not
cross the Adapter. A wrong capability and query-bearing path returned 404,
POST returned 405, and an ordinary proxy-form `example.com` request returned
404. These four requests caused no Publisher or sentinel request. After the
exact process stopped, reloading the same URL produced
`ERR_CONNECTION_REFUSED`. Both exact temporary roots were removed with zero
residue.

The browser evaluation surface did not expose `isSecureContext`, readable
cookie state, console CSP messages, or DNS events. Zero sentinel HTTP requests
is not proof that no DNS lookup was attempted. The result therefore supports
the presentation mechanism but does not qualify a participant browser or close
R-096.

The navigation counterexample then used binary SHA-256
`2bba0edbd6f1d5bbc666613e9b9f6c1e6c0e76ac187530d4c6bef2525c661748`.
Under the fetch-only base CSP, `meta refresh` navigated to the sentinel, which
received the SVG and favicon requests. With
`sandbox allow-same-origin`, the same page, CSS, and SVG rendered, automatic
refresh and a deliberate external-link click left the Adapter URL unchanged,
and the sentinel remained at zero requests. Route rejection still produced no
new Publisher request. The source was then changed to make the sandbox profile
the default; that build had SHA-256
`aa6a793745b1e78766f2463f314b74593b36e4eae823d7eca0db4921e568a6f5`
and returned the selected CSP in a final HTTP check.

This does not qualify arbitrary HTML or every browser navigation surface. The
Windows fixture was stopped by exact checked process termination, so graceful
signal/join behavior remains unmeasured.

## Evidence boundary

This is not an Ardents Service Connection, Target Link, Endpoint authorization,
DNS observation, malicious same-user boundary, or privacy proof. The Publisher
server is a local simulator, not H4-2. The random path value is visible to the
browser and authorizes only this disposable static origin; it is not a Local
Grant. The temporary User-Agent log is local experiment evidence, not a remote
Publisher field. The slice tests the proposed H4-3 presentation boundary in one
browser engine. A maintained Adapter must receive an exact selected Target and
opaque connection handle from the Endpoint rather than accepting a URL or
choosing a transport.

## Disposition

Disposable research fixture for a decided contract. The loopback origin and
static sandbox profile are promoted to H4-3; retain only until its unique
browser-origin counterexample/evidence enters source history, then supersede it
with maintained carrier-backed qualification.
