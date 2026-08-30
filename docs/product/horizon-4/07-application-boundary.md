# H4-7 — Optional protected application mode

Status: **accepted H4-7 direction; the maintained Application Interface and
Browser Adapter are generic and make no Application-level privacy claim.**

## Decision

H4-7 is not required for the ordinary H4-3 web path. A User may open an
Ardents Service in their existing browser through the local Browser Adapter,
while the browser continues to use the normal Internet for ordinary sites. That
is useful compatibility, but it cannot guarantee that the browser, an extension,
or a page's secondary behavior will not contact the ordinary network directly.

H4-7 introduces a separate, opt-in **protected application mode** only where
we want to claim Application-level Endpoint Location Privacy. It must constrain
the complete selected Application/helper process tree—not merely a local proxy
port or one initial request—so ordinary network egress fails by default and the
only permitted Ardents path is the scoped Application Interface.

If a platform, application, or attachment mode cannot meet that profile, it
remains available as generic/unqualified operation and the claim is absent.
There is no silent fallback that retains the protected-mode label.

**Current research outcome:** protected mode is deferred. The current H4-3
core candidate is headless and does not depend on a browser. Its optional
generic-browser compatibility trace remains unqualified, is not a selected
protected job, and no operating-system mechanism has been chosen. A sandbox,
browser extension, proxy, or firewall experiment cannot become a privacy
feature on its own; H4-7A starts only after the Product Owner names one exact
workload and platform. See
[R-099](../../research/records/r-099-protected-application-profile.md).

## Why the ordinary browser is not enough

```text
Generic browser path (H4-3)
browser -> local Ardents Adapter -> Ardents Service       [usable]
browser -> DNS / HTTP / WebRTC / extension / callback    [may bypass Ardents]

Protected application mode (H4-7)
selected application and descendants -> scoped Adapter   [allowed]
selected application and descendants -> ordinary network [denied]
```

An encrypted Ardents Service Connection does not constrain unrelated browser
networking, external resources, DNS prefetch, WebRTC, QUIC, callbacks/SSRF,
malicious same-user processes, cache/storage crossing, or a child process. H4-7
exists to make a narrow testable claim about those escape paths; it is not a
claim that arbitrary web content is safe or anonymous.

## Protected-mode contract

For one exact supported application job and platform profile, the mode must:

- bind the Local Grant and Isolation Context to a distinguishable Application
  Principal and complete process tree, not a PID, desktop account, loopback
  port, or copyable token;
- permit only the scoped local IPC/loopback attachment and exact Ardents
  destinations allowed by the grant;
- deny ordinary network listeners, inbound access, DNS, direct sockets, HTTP,
  WebSocket, WebRTC, QUIC, external resources, callbacks, and child-process
  escapes unless an explicit later profile qualifies them;
- keep origin, cache, credentials, and persistent storage from crossing
  Isolation Contexts; and
- close or deny work correctly on grant revocation, process-tree change,
  restart, crash, or cleanup failure.

The protected information, adversary, conditions, measurement, and honest
limitations must be stated for each profile. H4-7 never upgrades the route's
own bounded privacy guarantees or protects a compromised endpoint/application.

## Delivery slices

### H4-7A — choose one narrow protected job

**Goal:** choose the smallest useful claim-bearing workload before choosing an
OS mechanism.

Specify one Reference Application or controlled site workload, one platform,
its complete helper/process tree, accepted data and storage behavior, allowed
Ardents destinations, user journey, and exact privacy claim. A deterministic
single-response reference site is a better first candidate than a general
browser with arbitrary pages, scripts, extensions, and third-party resources.

**Done when:** the profile has a threat-model entry, falsification tests, an
owner for every process and storage surface, an explicit unsupported behavior,
and a credible maintenance boundary for the one-to-one project team. No OS
isolation technology is selected before this contract exists.

### H4-7B — enforce and falsify one platform profile

**Goal:** make the chosen narrow job fail closed outside its Ardents boundary.

Choose a platform mechanism only after H4-7A and record the consequential
trade-off in research and, when warranted, an ADR. Bind the local grant and
launch/attachment lifecycle to the actual process tree. Exercise allowed
Ardents access and denied ordinary-network attempts under clean start,
restart, revocation, crash, and cleanup.

The output is one supported protected profile, or a rejected experiment. It is
not a generic sandbox framework and does not imply Windows-and-Ubuntu support
at once.

### H4-7C — protected browser profile, if justified

**Goal:** decide separately whether a browser can carry the protected-mode
claim.

This begins only if the H4-3 generic browser path demonstrates enough value and
H4-7A/B show that an exact browser build, extension policy, child-process model,
origin model, cache/storage boundary, external-resource policy, and update
burden are supportable. A browser fork, system-wide proxy/DNS/VPN change, or
global Ardents CA is not presumed.

Rejecting H4-7C leaves the ordinary browser Adapter usable; it merely remains
honestly unqualified for the stronger Application-level claim.

## Evidence and promotion gates

For each platform profile, reproduce and record direct ingress/egress, DNS,
external-resource, WebRTC, callback/SSRF, child-process, malicious-sibling,
process-tree escape, storage crossing, cache crossing, restart, revocation,
crash, and cleanup attempts. The tests must use the selected release and exact
application version/configuration, not a similar program.

One bypass invalidates the profile's Application-level claim until fixed and
requalified. Generic Adapter operation or a successful controlled route test
is never substitute evidence. A protected profile still needs the applicable
H4-8 Route Qualification evidence; local isolation does not create route
privacy by itself.

## Non-goals

- Making the H4-3 Browser Adapter, an extension, a proxy setting, shared user
  account, or local port into an isolation boundary by naming it one.
- Anonymous browsing, third-party-content safety, content sanitization, or
  protection from a compromised selected application/endpoint.
- System-wide proxy, DNS, VPN, firewall, browser fork, or certificate-authority
  ownership unless a future selected profile explicitly justifies it.
- A universal cross-platform sandbox or a hidden maintenance organization.

## Open Product Owner selections

- Whether a claim-bearing protected application is valuable before Public Beta,
  or whether generic alpha compatibility is sufficient.
- The exact first narrow job and one platform for H4-7A.
- Whether any browser profile is worth the ongoing compatibility and update
  burden after the ordinary browser alpha has been observed.
