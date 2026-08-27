# R-117 — Firefox narrow proxy-auth experiment

## Question

Can a temporary Firefox Manifest V3 extension, limited to `http(s)://*.ard/*`,
obtain a loopback proxy port through native messaging and revalidate that same
port through native messaging before answering its HTTP `407 Basic` challenge,
without observing or changing ordinary Internet requests?

This is a falsification experiment for the Browser Entry handoff. It is not a
participant installer, a maintained extension change, a native-host protocol,
or a same-user isolation claim.

## Hypothesis and falsification

The temporary native host first supplies the generated loopback port for the
route. Only after the synthetic proxy sends its `407 Basic` challenge does the
add-on ask that host again for the port/password pair. It supplies the
generated password only if the second native result names the exact challenger
port. The synthetic proxy must first observe a request without
`Proxy-Authorization`, then observe a repeat request with the exact generated
Basic credential before it returns a small HTTP response.

The hypothesis is falsified if Firefox does not deliver the authentication
event under the narrow `.ard` host filter, cannot call native messaging while
the challenge is blocked, does not repeat the request with the exact
credential, opens a credential prompt, or if the temporary add-on needs
`<all_urls>` or a browser proxy-settings change.

## Run

This Windows-only script creates one temporary profile and evidence directory
under the OS temporary root. It copies the fixture before injecting its native
host's port and password. It registers one exact temporary HKCU native-host
manifest and deletes that exact registration in `finally`. It changes no daily
Firefox profile, persistent registry key, DNS, proxy, certificate, hosts,
route, or VPN setting.

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\experiments\r-117-firefox-proxy-auth\run.ps1 `
  -FirefoxPath 'C:\Program Files\Mozilla Firefox\firefox.exe'
```

The script prints an evidence directory containing `requests.json`. A pass has
one unauthenticated request followed by the same request with an exact
`Proxy-Authorization: Basic ...` value. It stops only the Firefox process that
uses its temporary profile.

Pass `-RejectChallengeRevalidation` for the required negative control. The
native host then returns a different valid port only at the `407` challenge;
the original proxy must observe exactly its first unauthenticated request and
no credential-bearing retry.

## Inputs and evidence hygiene

The generated password and temporary native manifest are used only for this
disposable run; the password appears in its temporary evidence. No Endpoint
state, Release key, alpha corpus, participant profile, or production native
manifest is used. Review and remove the printed temporary evidence directory
after the measurement.

## Result and disposition

Passed on 2026-08-26 with Firefox 154. The temporary manifest was registered
only at its exact HKCU key and was absent after cleanup. In the positive run,
the temporary proxy at `127.0.0.1:57737` first received
`GET http://reference.ard/ HTTP/1.1` without proxy credentials, sent its Basic
`407` challenge, then received the same request with the exact generated Basic
credential supplied after the second native-host call. In the negative run,
the host deliberately returned a different port at the challenge: the proxy
at `127.0.0.1:57779` saw only its first unauthenticated request and no
credential-bearing retry. The fixture passed `web-ext@10 lint` with zero
errors, notices, and warnings.

This establishes only that Firefox can authenticate one ordinary `.ard` HTTP
proxy request with this new permission surface. It does not prove that the
permission is acceptable for the maintained Browser Entry, that a native host
may safely disclose the capability to the extension, HTTPS/TLS, DNS/DoH
behavior, no-fallback behavior, installation/update/removal, or a same-user
isolation boundary.
