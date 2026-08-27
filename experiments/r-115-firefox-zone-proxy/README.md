# R-115 — Firefox zone-proxy browser-entry spike

## Question

Can an unmodified Firefox, in an isolated temporary profile, open the
no-port URL `http://reference.ard/` through an ephemeral loopback HTTP proxy
without binding port 80/443 or depending on a successful public DNS answer?

This is evidence for the Firefox-only candidate in
[R-115](../../docs/research/records/r-115-named-browser-entry.md). It is not
an Endpoint, add-on, name resolver, HTTPS design, or participant instruction.

## Hypothesis and falsification

The temporary profile's PAC rule compares the browser-supplied host text and
returns only `PROXY 127.0.0.1:<ephemeral-port>` for `*.ard`; it returns
`DIRECT` for every other host. If Firefox sends an HTTP proxy request with the
absolute `http://reference.ard/` URI to the loopback listener, the hypothesis
survives this narrow check. It is falsified if Firefox cannot load the local
PAC, opens a direct connection instead, requires port 80/443, or does not send
the expected proxy request.

The spike intentionally does **not** claim that no DNS or DoH packet was sent:
it has no packet capture. R-115's later clean-profile qualification must record
DNS/DoH traffic directly, extension installation/removal, pre-existing browser
proxy coexistence, an unavailable Endpoint, a no-fallback failure, and the
native-messaging handoff.

## Run

Run on a Windows workstation with Firefox installed. The script creates a
temporary Firefox profile and a one-request loopback proxy; it neither changes
the user's Firefox profile nor edits OS DNS, proxy, certificate, hosts, route,
or VPN settings. It leaves screenshots and the captured request under a fresh
temporary evidence directory, printed at the end.

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\experiments\r-115-firefox-zone-proxy\run.ps1 `
  -FirefoxPath 'C:\Program Files\Mozilla Firefox\firefox.exe'
```

The expected `request.txt` starts with:

```text
GET http://reference.ard/ HTTP/1.1
```

and `screenshot.png` renders the synthetic response. The selected listener
port appears in `result.json` and must not be 80 or 443.

### Extension-API follow-up

`addon/` is a source-only fixture for the next measurement. It uses Firefox's
`proxy.onRequest` rather than a PAC rule and ends the local proxy chain with
`null`, so the browser cannot fall back to another proxy or direct route. It
must be copied to an external temporary directory, have its dummy port replaced
with a fresh loopback listener port, and be launched through the Mozilla
`web-ext run` temporary-add-on workflow. Mozilla documents that this workflow
uses a temporary profile and add-on; it is not a participant installation.

`run-addon.ps1` is the reproducible success-path launcher. It copies the
fixture and creates its base profile below one fresh OS temporary directory,
uses the fixture's temporary-install event to open `http://reference.ard/`
only after the proxy listener has registered, and records `web-ext` output plus
the proxy request. The owned temporary Firefox profile also has a distinct
synthetic fallback proxy: `unavailable.ard` and `https://reference.ard/` must
not reach it, while `ordinary.invalid` must. The temporary local proxy must
observe the HTTPS `CONNECT reference.ard:443` and deliberately return `502`,
because this fixture has no HTTPS trust model. This measures the terminal
`null` failover only; it does not observe DNS or DoH packets. It opens no daily Firefox profile. Its cleanup identifies Firefox
only by that temporary profile path; it must never kill all `firefox.exe`
processes.

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\experiments\r-115-firefox-zone-proxy\run-addon.ps1 `
  -FirefoxPath 'C:\Program Files\Mozilla Firefox\firefox.exe'
```

Append `-UseNativeHost` to exercise the disposable native-messaging variant.
It writes the exact temporary HKCU key documented by Firefox, permits only the
fixed fixture add-on ID, and verifies deletion in `finally`. It is not an
installer mechanism.

`-ExternalLoopbackProxyPort <port>` is for the maintained Windows
qualification only. In that mode this script does not start a synthetic alpha
proxy and cannot itself attest alpha requests: its caller must own and observe
the listener. The fixture still supplies the temporary Firefox profile,
add-on/native host, and ordinary-URL fallback control.

The success criterion is the same absolute request as the PAC analogue, plus
the recorded local 502 and no-fallback cases. A real Endpoint-stop and DNS/DoH
packet-capture measurement remain required before this can support a Browser
Entry decision.

The fixture's Manifest V3 source passed `web-ext@10` lint with warnings treated
as errors on 2026-08-26. Its first `web-ext run` attempt was inconclusive:
`--start-url` opens before temporary installation, so the test URL could fail
before the add-on registered its listener. `run-addon.ps1` instead opens the
fixture-only probes from `runtime.onInstalled`. On 2026-08-26 that launcher
passed with Firefox 154: it captured `GET http://reference.ard/ HTTP/1.1` and
`CONNECT reference.ard:443` at the assigned loopback proxy. The proxy returned
local `502` for the unavailable and HTTPS probes; a distinct configured
fallback proxy saw the ordinary `.invalid` request but no `.ard` request. This
proves narrow proxy routing and no-browser-proxy-fallback, including HTTPS
failure handling. Its direct-port variant is not DNS/DoH, native-messaging,
HTTPS-success/trust, or participant lifecycle evidence.

The `-UseNativeHost` variant also passed on 2026-08-26. Firefox temporarily
obtained the assigned loopback port through a framed native message, with the
native manifest allow-listed to only the fixture ID; the launcher confirmed its
temporary HKCU manifest key had been removed after closing the owned Firefox
profile. This is Windows experiment evidence, not a maintained host protocol
or installer proof.

## Inputs and evidence hygiene

The input is only the local Firefox binary and synthetic response. No private
keys, alpha corpus, participant profile, packet capture, external DNS log, or
real Service content is used. The script creates all runtime files below the
OS temporary directory, not this repository. Review and delete the printed
evidence directory manually when it is no longer needed.

## Result and disposition

Result: passed on 2026-08-26 with Firefox 154 on the Windows workstation. The
temporary PAC sent `GET http://reference.ard/ HTTP/1.1` with
`Host: reference.ard` to an ephemeral loopback proxy on port 62372, and the
headless screenshot rendered its synthetic response. This proves that Firefox
can make that no-port named HTTP request without a successful public DNS answer
or local port 80/443. It does not prove that a DNS/DoH query was absent.

Retain the spike until its result has been copied into R-115. Delete it once an
accepted Browser Entry implementation and its maintained qualification replace
this disposable measurement.

## Limitations

PAC is a lower-level browser-routing analogue, not the planned signed Firefox
add-on. The result cannot prove the add-on permissions, native-messaging
binding, extension update trust, DNS/DoH non-leakage, HTTPS, generic
application traffic, Windows installation, or Linux behavior.
