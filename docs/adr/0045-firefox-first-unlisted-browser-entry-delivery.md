---
status: accepted
date: 2026-08-26
---

# ADR-0045 — Deliver the alpha Browser Entry as a signed unlisted Firefox add-on

## Context

H4-4 needs a participant to use `http://<name>.ard/` in their existing browser
without making `.ard` public DNS, changing ordinary browsing, installing a
trust root, or maintaining a browser fork. Firefox native messaging provides a
per-user link from one exact add-on ID to a local native host, but neither a
source tree nor an ordinary file download proves that the XPI and host belong
to the same accepted closed-alpha release.

The Product Owner accepted a Firefox-first alpha profile and does not accept a
paid Windows OV/EV signing certificate or a paid CA as a prerequisite.

## Decision

Use one fixed-ID, Mozilla-signed **unlisted** XPI, self-distributed with the
closed-alpha release. The participant explicitly installs that XPI in Firefox;
Firefox, rather than Ardents, validates the Mozilla signature. Public AMO
listing is not part of the alpha contract.

The release uses closed-alpha enrollment descriptor v4 to separately manifest
the platform-specific `ardents-browser-entry-<platform>` native host and the
fixed `ardents-alpha-browser-entry.xpi`. Before an `install` operation writes
anything, it verifies the independently delivered enrollment pin, the enrolled
Endpoint artifact, its running native-host binary, and both companion files.
It then writes only the fixed current-user Firefox native-manifest
registration, whose `allowed_extensions` contains only the fixed add-on ID.
The Endpoint's explicit `BrowserEntryProfile: "firefox-alpha"` selects the
same fixed per-user state path used by the zero-argument host invocation.

`remove` withdraws only that owned native registration. A repair or replacement
may replace only an owned manifest and uses an atomic same-directory manifest
write. A pre-existing foreign registration or manifest fails rather than being
overwritten.

## Consequences

- Windows changes only the current user's exact Firefox native-messaging
  registry key. Ubuntu changes only the current user's native-manifest file.
  The profile adds no machine-wide registry entry, DNS/DoH rule, CA, global
  browser proxy, browser fork, port-80/443 listener, or paid Windows signing
  dependency.
- A stale state, disabled add-on, absent registration, malformed manifest, or
  stopped Endpoint fails closed for `.ard`; normal web traffic is outside the
  add-on's `.ard` filter.
- Mozilla XPI signing and the user-visible unlisted-XPI installation are
  external release prerequisites. A manifest-pinned source artifact is not
  proof that a concrete XPI is Mozilla-signed; that check is performed by
  Firefox at installation time and remains a release gate.
- Same-user file and browser-profile compromise are not solved or claimed as
  browser isolation. The closed-alpha verifier binds static release bytes; it
  does not grant an update, naming, corpus, or Endpoint authority.
- This delivery decision does not establish no-DNS/DoH behavior. A later clean
  Firefox 154 resolver trace recorded native resolution for `.ard` before the
  add-on's HTTP proxy route, so the profile is retained only as compatibility
  evidence and must not be presented as a participant Browser Entry.

## Compliance

[R-117](../research/records/r-117-firefox-browser-entry-delivery.md) owns the
delivery evidence and two-platform qualification. [ADR-0040](0040-bounded-alpha-name-overlay.md)
continues to bound `.ard` to the private alpha overlay, and
[ADR-0044](0044-revalidate-browser-entry-proxy-authentication.md) continues to
bound the in-browser proxy-authentication handoff.
