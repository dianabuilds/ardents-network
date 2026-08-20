---
id: R-056
title: Which direct-binary, desktop, and browser Adapters open Ardents Service Links without changing Internet or VPN behavior?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-056 — Stage 7 direct-binary, desktop, and browser integration

## Decision this unlocks

Decide which bounded desktop Application Adapter, if any, belongs in Stage 7 so
a User can open an explicit `ardents://` Service Link without Ardents changing
the system DNS, route table, default proxy, default browser, or active VPN
policy. Separately decide which integration is only generic compatibility and
whether a supported browser profile can enter the claim-bearing
Network-Isolated Application Boundary.

This decision freezes the install registrations and client topology that R-050
must not select by inertia. It does not select a browser engine, make a browser
part of the network core, or authorize maintained Stage 7 code.

## Current contract

The canonical shareable form is `ardents://<Service Name>` and never an HTTP or
DNS fallback. The existing Application Interface opens authenticated Service
Connections and exchanges opaque bytes. The Application Broker binds Local
Grants and Application Principals; Application Isolation separately owns a
complete process-tree network and storage boundary.

ADR-0007 and R-002 already permit generic HTTP/SOCKS/stream or browser Adapters
for compatibility, but cap them at `application-networking-unverified`. A
claim-bearing Application must have only scoped Ardents local IPC/loopback,
deny ordinary DNS/listeners/direct egress, separate origin/cache/storage by
Isolation Context, and fail every undeclared secondary destination. Product
scope excludes clearnet exit, VPN behavior, a bundled browser, and a universal
Application runtime from the network core. Stage 7 excludes transparent traffic
interception and universal browser sandboxing.

The retained candidate contract is now centralized in the
[Stage 7 Application Adapter specification](../../development/stage-7-application-adapter-spec.md).
It fixes direct `connect`/`accept`, an explicit `browse` handoff, an ephemeral
loopback HTTP Adapter, generic default-browser use, an explicit but unsupported
Stage 7 isolated-browser request, optional per-user URI registration, and exact
no-fallback/VPN non-interference. The open work is platform falsification and
joint Product Owner acceptance, not another transport topology choice. R-052
selects controlled native client/publisher Applications, rather than an
unmodified browser, for the claim-bearing Stage 7 profiles.

**Product Owner decision, accepted 2026-08-20:** O1 is the Stage 7 topology.
The exact platform executable wrapped by Installed and released directly as
Portable is a first-class Client/Application Adapter; it MUST remain fully
usable with
no browser, extension, URI registration, or mandatory SDK. The browser companion
is optional and supplies generic handoff plus a separately named isolated
request that may report unsupported; it cannot weaken or silently replace the
same Application Broker/Connection Interface seam. O4 is rejected, O3 is
deferred, and O2 remains generic compatibility only.

## Hypotheses

- **H1 — accepted topology; exact Adapter active:** a small desktop Service Link
  handler plus ephemeral loopback HTTP companion can hand an exact destination
  to the Application Broker with no extension, native host, proxy, or ambient
  network change. In an ordinary default browser it is a generic, visibly
  unqualified Adapter. R-052 may admit a separate claim-bearing browser only
  after proving all extra desktop IPC and complete helper-tree paths; the
  current Stage 7 result is explicitly unsupported.
- **H2 — explicit per-Application proxy:** a local HTTP CONNECT/SOCKS Adapter
  configured only in one Application can provide useful compatibility without
  system-wide changes, but cannot alone preserve Ardents destination/origin
  semantics or prove browser network isolation.
- **H3 — dedicated browser distribution:** a separately maintained browser
  build/profile is required for any browser-level privacy claim that an
  installed upstream browser cannot meet. It may be a later Application product
  rather than a Stage 7 dependency.
- **H0:** no browser candidate meets the Stage 7 user outcome and one-to-one
  maintenance budget; Stage 7 supports direct native Application Interface use
  and the controlled Reference Application only, with browser work explicitly
  deferred.

## Evaluation criteria

### User and Interface outcome

- Opening an exact valid `ardents://` Service Link invokes one visibly Ardents-
  scoped action and preserves the exact Service Name/Destination Binding.
- Invalid, unavailable, or failed Ardents resolution never becomes DNS, search,
  HTTP, a public proxy, or a direct socket attempt.
- The Adapter translates only declared browser/Application requests to the
  existing Connection Interface. It receives no Service Administration or
  Authority Custody access.
- Native Applications can use the same Application Interface without installing
  or embedding a browser Adapter.
- The supported platform binary implements the exact `connect`, `accept`, and
  `browse` grammar, stdio half-close, bounded terminal JSON/human result, exit
  classes, cancellation, and inherited Connection Interface resource contract
  in the Application Adapter specification. Direct raw-stream use never depends
  on a browser or URI registration.
- A command remains a thin Adapter at the Application Interface seam. It does
  not duplicate destination resolution, release trust, Local Grant, retry,
  isolation, or Route behavior inside command parsing/presentation code.
- Unsupported browser/application behavior is explicit; no installation or
  runtime step silently changes the default browser.

### Internet, VPN, and failure boundary

- Ardents does not install a TUN/TAP device, become the system VPN, edit the
  route table or system DNS, or set a system-wide/default-browser proxy.
- In the generic profile, ordinary web traffic continues through the browser's
  existing network stack and current OS/VPN policy. Only an explicit Ardents
  action reaches the local Adapter.
- The Endpoint's Carrier Channels use ordinary unprivileged OS networking and
  therefore remain subject to the active VPN, firewall, proxy, and kill-switch
  policy. A blocked Carrier is `unavailable`; Ardents does not bypass it or add a
  direct fallback.
- Any future claim-bearing browser profile would have to give the separate
  complete browser/helper tree no ordinary network path and only scoped Ardents
  local IPC/context storage. Stage 7 selects no such profile; the User's browser
  processes and VPN remain untouched.
- A browser, extension, helper, or local page cannot use loopback discovery,
  DNS rebinding, proxy error fallback, WebRTC/STUN, external subresources,
  downloads/helpers, or inherited handles to escape the declared profile.

### Security, maintenance, and lifecycle

- Browser origin, cookies, cache, storage, history, downloads, helpers, and
  secondary destinations have exact Isolation Context rules; `ardents://` is not
  silently mapped onto a shared public-web origin.
- URI-handler objects, executable paths, loopback listeners, temporary browser
  profiles, permissions, update compatibility, registration conflicts, repair,
  uninstall, and residue are enumerated on the manifest-bound Windows and
  Ubuntu development surfaces, with unavailable desktop/native facts deferred.
- The retained Stage 7 candidate installs no extension or native-messaging host
  and changes no proxy/PAC or browser policy. Appearance of any such object is a
  contract violation, not an unrecorded implementation detail.
- Installed and Portable forms exercise identical direct-binary and browser
  Application outcomes. Portable default use creates no system registration;
  any optional per-user registration is an explicit, reversible R-056 action,
  not a Portable installer/lifecycle contract.
- The browser/extension update authority cannot authorize Ardents executable
  bytes, and the Ardents installer cannot silently install an extension into an
  existing browser profile.
- The generic path uses the current default browser without claiming support
  for a particular engine. The isolated browser request starts no browser or
  listener in Stage 7 and returns `isolation-unsupported` without fallback.
- The generic path displays `application-networking-unverified`. Only a future
  exact isolated profile accepted after new R-052-equivalent research may
  display the stronger result; Firefox identity or preferences cannot do so.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- Microsoft [custom URI activation](https://learn.microsoft.com/en-us/windows/apps/develop/launch/handle-uri-activation),
  [Application registration](https://learn.microsoft.com/en-us/windows/win32/shell/app-registration),
  and [protocol-association ownership](https://learn.microsoft.com/en-us/windows/compatibility/file-type-and-protocol-associations-model);
- freedesktop.org Desktop Entry
  [`Exec` argument expansion](https://specifications.freedesktop.org/desktop-entry/latest/exec-variables.html),
  [recognized keys](https://specifications.freedesktop.org/desktop-entry/latest/recognized-keys.html),
  and [MIME default Applications](https://specifications.freedesktop.org/mime-apps/latest/default.html);
- Mozilla Firefox
  [command-line parameters](https://firefox-source-docs.mozilla.org/browser/CommandLineParameters.html),
  [profile behavior](https://firefox-source-docs.mozilla.org/toolkit/profile/),
  [ESR channel/update policy](https://firefox-admin-docs.mozilla.org/guides/firefox-channels/),
  and [Firefox ESR security advisories](https://www.mozilla.org/en-US/security/known-vulnerabilities/firefox-esr/);
- W3C [Secure Contexts](https://w3c.github.io/webappsec-secure-contexts/)
  loopback-origin treatment and RFC 8252
  [loopback redirect discussion](https://www.rfc-editor.org/rfc/rfc8252.html),
  used only as evidence for numeric loopback/random-port binding and hostile
  local-process risk, not as proof of Ardents isolation;
- Chrome [Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging)
  and [proxy API](https://developer.chrome.com/docs/extensions/reference/api/proxy)
  documentation;
- Mozilla [Native Messaging](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_messaging)
  and [proxy API](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/proxy)
  documentation;
- Tor Project guidance on
  [using Tor with other browsers](https://support.torproject.org/tor-browser/security/using-tor-with-other-browsers/)
  and [Tor Browser's browser-specific protections](https://support.torproject.org/tor-browser/getting-started/about-tor-browser/);
- repository product scope, journeys, operating model, functional map,
  `CONTEXT.md`, R-002, R-048, R-050–R-052, and ADR-0007.

### Experiment

The disposable
[R-056 state model](../../../experiments/r-056-stage-7-desktop-browser-integration/README.md)
was run first. It models both Distribution Profiles, direct and OS entry,
generic versus explicitly unsupported isolated mode, browser availability,
permitted VPN, and blocked Carrier states. It performs no OS mutation or network activity and
cannot count as platform qualification.

The remaining experiment uses the exact manifest and observer protocol in the
[Stage 7 host-campaign specification](../../development/stage-7-host-campaign-spec.md)
on clean patched Windows 11 and Ubuntu desktop images with their frozen default
browsers. It exercises a
synthetic `ardents://site.example` destination through controlled HTTP content
that attempts external subresources, redirects, DNS, loopback probing,
WebSocket, WebRTC/STUN, downloads, helper launch, and cross-context storage.

First run the exact packaged and raw-Portable executable digest through the
same direct-binary destination/result/byte-stream cells with no browser,
extension, URI registration, or SDK. Browser setup or failure MUST NOT alter
those results. Portable remains the raw executable contract; any optional
per-user desktop registration is a separate explicit experiment action.

Run ordinary no-VPN, full-tunnel VPN, split-tunnel VPN, kill-switch, blocked
Carrier, browser/Endpoint restart, browser update, repair, removal,
moved-Portable-path, and registration-conflict cells. Exercise exact stdio,
result/exit, URI quoting/injection, numeric-loopback, random-port, capability,
origin, header/method/bounds, and cleanup rules from the Adapter specification.
Host-side network, DNS, process, registry/desktop-entry, filesystem, and cleanup
observers are authoritative. No real browsing data, account, Service,
Authority, or VPN credential enters evidence.

### Failure scenarios

System proxy/DNS/route changed; VPN bypass; any failure becomes direct; ordinary
browser profile is mislabeled isolated; Ardents session reuses a public-web or
other-context origin/storage; external subresource or helper escapes; a hostile
same-user process attaches; URI handler argument injection; an unsupported
isolated request launches anything or becomes generic; browser absence bricks
direct-binary use;
installer silently changes defaults; removal leaves handler/listener/profile/
proxy state; an extension or native host appears; or Ardents begins shipping a
browser and creates an unaccepted security-update obligation.

## Findings

- **Sourced fact:** Windows and freedesktop desktop environments support
  registering an Application for a URI/MIME association; default-handler choice
  remains an operating-system/user decision and requires lifecycle ownership.
  A freedesktop `%u` field expands as one URL argument; Windows activation still
  requires platform-specific quoting and adversarial argument tests.
- **Sourced fact:** Firefox documents separate `--profile`, `--no-remote`, and
  `--new-instance` launch controls. Mozilla ships ESR as an annually based line
  with intervening security updates, so an isolated claim has to bind one exact
  current build and be requalified after drift; an indefinitely pinned browser
  would be a security failure.
- **Sourced fact:** standards treat numeric loopback origins as potentially
  trustworthy in browser secure-context evaluation, not as isolation from
  other local processes. Random loopback ports and IP literals reduce ambient
  collision/name-resolution risk but do not authenticate a same-user client.
- **Sourced fact:** Chrome and Firefox Native Messaging connect an installed
  extension to a separately installed native host through a declared manifest
  and extension allow-list. Browser installation does not install or manage the
  native Application.
- **Sourced fact:** browser proxy APIs alter browser proxy policy and include
  direct/fallback behavior that must be controlled explicitly; a proxy setting
  is not proof that every browser/helper network path is confined.
- **Sourced fact:** Tor Project warns that merely routing an ordinary browser
  through Tor does not supply Tor Browser's DNS/WebRTC, state, and fingerprinting
  protections. Tor Browser is a modified Firefox ESR product, not just a proxy
  toggle.
- **Inference:** an existing ordinary browser plus a companion can be a useful
  generic Ardents entry point, but cannot earn the claim-bearing profile merely
  because its main request entered Ardents.
- **Inference:** Stage 7 does not need an extension, native host, or proxy. A
  same-executable `browse` command can expose one ephemeral reverse HTTP Adapter
  on a numeric random loopback port. In generic mode its same-user and direct-
  browser networking limitations remain explicit. A loopback token, Firefox
  preferences, or R-051 tree ownership alone cannot prove isolation.
- **Inference:** R-052 rejects the Firefox experiment candidate for Stage 7.
  Windows unpackaged loopback requires an administrative exemption that is not
  private to this tree; Ubuntu visible Firefox requires desktop/display IPC
  outside the current Ardents-only allow-list.
- **Inference:** a claim-bearing browser mode needs a separately launched and
  identified browser/helper process tree plus isolated origin/storage. The
  User's unrelated browser process remains outside that boundary and keeps its
  normal Internet/VPN behavior.
- **Inference:** a TUN/VPN or transparent system proxy is the wrong Stage 7 seam:
  it competes with existing routing policy, expands privilege and traffic scope,
  hides explicit Service Link semantics, and creates forbidden fallback risk.
- **Product Owner decision:** direct binary use is first-class and browser-
  independent in both relatively equivalent Distribution Profiles; browser
  integration is an optional Adapter, not the Client definition.
- **Measured prototype result:** the disposable state model preserved direct
  `connect`/`accept` across Installed/Portable and browser/registration absence;
  required explicit registration for OS handoff but not direct `browse`; kept a
  permitting VPN unchanged; returned route unavailable for blocked Carrier;
  and returned `isolation-unsupported` for every isolated-browser request without
  generic/direct fallback. This supports logical coherence only and leaves all
  platform, browser, origin, side-effect, and isolation falsifiers open.

## Options

### O1 — Binary-first plus explicit companion and honest profile separation

Accepted as the product topology. The Installed package wraps the same supported
binary released directly as Portable. The OS may open `ardents://` with a small
optional per-user desktop registration; direct `ardents browse` needs none. The
binary validates the exact link and exposes one bounded ephemeral reverse HTTP
Adapter to the existing default browser with an honest generic label. An
isolated request remains a separate no-fallback operation but is unsupported in
Stage 7. Ordinary web/VPN configuration is never edited.

No extension, native-messaging host, proxy, browser bundle, loopback exemption,
or desktop-sandbox IPC profile is selected. This keeps browser integration
functional without converting incomplete browser confinement into a claim.

### O2 — Explicit per-Application HTTP CONNECT/SOCKS compatibility Adapter

Retain only as a generic compatibility candidate. It may help existing native
software, provided the User configures that one Application and direct fallback
is disabled. It cannot be the canonical Service Link handler or claim-bearing
browser profile without the O1 isolation and origin work.

### O3 — Tor-Browser-style dedicated Ardents browser distribution

Defer unless O1 cannot support a required browser outcome. A dedicated browser
can control configuration and user separation, but makes browser patch intake,
builds, signing, extension policy, vulnerability response, and cross-platform
distribution a second product. The current team cannot silently assume that
maintenance organization. Reusing Tor Browser with another network also does
not inherit Tor Browser anonymity claims.

### O4 — System VPN/TUN, global proxy, DNS, or transparent interception

Rejected for Stage 7. This would change unrelated connectivity, can conflict
with an active VPN/kill switch, requires broader privilege, and collapses
explicit Ardents destination handling into ambient IP/web traffic.

### O0 — No browser Adapter in Stage 7

Required if O1 cannot satisfy lifecycle, origin, browser-version, isolation, and
one-to-one maintenance gates. Direct native Application Interface use and the
controlled Reference Application remain possible, but the documentation must
say browser support is deferred rather than implying it through installation.

## Recommendation

Take the exact O1 generic-browser and unsupported-isolated contract in the
Application Adapter specification to the current-Windows/Ubuntu-Docker
development experiment. Retain O2 only as a later generic compatibility comparison, reject
O4, and defer O3. R-050 may model only the explicit per-user URI objects frozen
here; it must not add an extension, native host, browser payload, loopback
exemption, proxy, route, DNS, or VPN registration.

Confidence is high in the direct-binary and no-system-network-change contract,
medium in generic browser handoff, and high that the current Firefox shapes do
not meet the stronger Stage 7 boundary. Browser integration and native
claim-bearing Applications remain available without an isolated-browser claim.

## Disposition

- State: `decided`; the Product Owner accepted O1, binary-first use, two
  Distribution Profiles, O2/O3/O4 dispositions, the no-system-network-change
  boundary, and the exact Adapter contract on 2026-08-20. Extension, native host,
  proxy, and bundled-browser approaches are explicitly absent. The direct
  command/stream, generic handoff, unsupported isolated result, origin, and
  optional registration contracts remain scheduled implementation evidence.
- The scheduled current-Windows/Ubuntu-Docker subset must record the observable
  direct-binary/browser behavior, parity, VPN non-interference, unsupported-
  isolated behavior, and cleanup without a falsifier. The Product Owner
  accepted the explicit Ubuntu Desktop/native deferrals as limitations, not
  passes. Windows URI registration and all install-associated behavior stay
  `authorization-pending` until a separate Product Owner command. Final joint
  development evidence still must not infer registration success from the
  logic prototype.
- R-051/R-052 remain authoritative for principal and process-tree isolation;
  this record cannot weaken or duplicate them.
- A consequential dedicated-browser or system-network integration decision
  requires an explicit scope change and accepted ADR before implementation.
- Maintained direct-binary/generic-browser Adapter work is authorized only in
  its owning slice under package-map rules. No extension, browser artifact, or
  Windows registration is authorized.
