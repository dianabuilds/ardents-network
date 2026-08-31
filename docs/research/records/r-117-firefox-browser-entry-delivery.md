---
id: R-117
title: Firefox Browser Entry delivery boundary
status: closed; participant delivery selection superseded
owner: Product Owner and Codex
started: 2026-08-26
reviewed: 2026-08-26
---

# R-117 — Which per-user delivery boundary can make the selected Firefox Browser Entry usable without silently changing browser or system authority?

## Decision this unlocks

This record historically selected an H4-4 Firefox-first alpha delivery profile:
exact signed XPI distribution, native-host manifest installation/removal on
Windows and Ubuntu, the stable Endpoint state-path input, update/recovery
behavior, and the limits of its user-facing claim. R-115 subsequently falsified
the required no-DNS/DoH-leak behavior. ADR-0061 therefore closes participant
delivery through this profile and retains the implementation only as optional
compatibility evidence.

## Retained compatibility contract

- The former Firefox path exposes only an already authenticated, pre-opened
  alpha session at `http://<name>.ard/`. Current H4-4A instead shares
  `ardents-alpha://<name>` through the headless Application Interface. The
  retained `.ard` trace is not public DNS, HTTPS, Web PKI, canonical Namespace,
  or an arbitrary proxy. [ADR-0040](../../adr/0040-bounded-alpha-name-overlay.md),
  [R-115](r-115-named-browser-entry.md), and
  [ADR-0061](../../adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
  are authoritative.
- The maintained add-on has a fixed ID, `proxy`, `nativeMessaging`,
  `webRequest`, and `webRequestBlocking` permissions, with every web-request
  listener limited to `.ard`. It has a terminal no-fallback proxy result for
  `.ard`, and now revalidates a loopback `407` before it returns the separate
  one-process proxy credential.
- The historical Endpoint-plan `BrowserEntryProfile: "firefox-alpha"` resolves
  to the native host's one fixed per-user state path. An explicit absolute
  `BrowserEntryStatePath` remains a bounded compatibility-test override; it is
  not a participant profile.
- The Product Owner accepted the bounded Firefox-first experiment and did not
  plan to buy a Windows OV/EV certificate for an alpha without users. The
  retained compatibility profile must not
  install a CA, edit ordinary DNS/DoH, take port 80/443, set global browser
  proxy settings, or require a browser fork.

## Hypotheses

- **H1 (historical):** a Mozilla-signed, self-distributed fixed-ID XPI plus an exact
  enrollment-v4 Endpoint/native-host/XPI binding can install a user-local
  native manifest and stable state-path profile without changing global browser
  or system network settings. **The mechanics are partially supported by
  maintained source and one concrete Mozilla-signed XPI. The participant
  hypothesis is superseded by ADR-0061, so no enrolled release-shaped
  two-platform run is pending.**
- **H2:** an AMO public listing is necessary for enough installation and update
  usability to justify accepting AMO listing/governance as an alpha dependency.
- **H3:** a Browser Entry can bind its later HTTP proxy request to the active
  Endpoint proxy with a narrow `.ard`-only `webRequest.onAuthRequired` handler,
  without granting `<all_urls>` access or leaking the separate local proxy
  credential to an unproven recycled port. **Supported for the maintained
  alpha tracer under ADR-0044; it is not an isolation or participant-delivery
  claim.**
- **H0:** neither signed XPI distribution form can meet the bounded support,
  recovery, and authority contract. Retain explicit Link handoff rather than
  claim participant-ready named browsing.

## Historical evaluation criteria

These criteria preserve the provenance of the former participant-delivery
experiment. They are not current release gates; optional compatibility
regression may reuse them only without making a participant Browser Entry
claim.

- A release Firefox user can install exactly one Mozilla-signed fixed-ID XPI;
  the native manifest allows only that ID and starts only the release-pinned
  host binary.
- Windows changes only the current user's exact
  `HKCU\\SOFTWARE\\Mozilla\\NativeMessagingHosts` key. Ubuntu changes only the
  current user's Firefox native-manifest directory. No machine-wide registry,
  `/usr`, DNS, proxy, CA, or listener ownership is added.
- Install, replace, interrupted replace, repair, and removal have explicit,
  idempotent outcomes. Removal withdraws only package-owned native-manifest
  registration and must not delete Endpoint Authority, alpha corpus, Release
  floors, the separately release-owned host binary, or generic Firefox data.
- The native manifest, host binary, and XPI have an exact Release
  decision/provenance binding; GitHub or AMO transport alone is not Endpoint,
  corpus, or Namespace authority.
- A stale native manifest or state fails closed for `.ard`; ordinary browsing
  remains outside the component. Same-user substitution and browser isolation
  are not claimed as solved.
- The profile requires neither paid Windows code signing nor a paid CA. Mozilla
  XPI signing and the participant's explicit installation remain external
  dependencies.
- If proxy authentication is selected, a `407` challenge must trigger a fresh
  native-host proof before the extension returns a credential. It must match
  the proved port and cancel every other challenge; an earlier probe alone is
  not an atomic browser-to-proxy handoff.

## Evidence plan

### Primary sources

- [Mozilla native manifests](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_manifests),
  accessed 2026-08-26: native manifests are provisioned outside extension
  installation; the manifest names the allowed extension IDs. Per-user Windows
  uses the exact HKCU key and per-user Linux uses
  `~/.mozilla/native-messaging-hosts/<name>.json`.
- [Mozilla signing and distribution overview](https://extensionworkshop.com/documentation/publish/signing-and-distribution-overview/),
  accessed 2026-08-26: Release/Beta Firefox requires Mozilla-signed extensions;
  AMO supports both public listing and self-distributed unlisted signing.
- [Firefox `ProxyInfo`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/proxy/ProxyInfo)
  and [`webRequest.onAuthRequired`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/webRequest/onAuthRequired),
  accessed 2026-08-26: HTTP proxy authorization uses a `407` challenge and
  `onAuthRequired`; Firefox Manifest V3 continues to support the
  `webRequestBlocking` permission. `ProxyInfo.username/password` are for SOCKS,
  not HTTP proxy authorization.
- Current maintained source:
  [`packaging/firefox-alpha-browser-entry`](../../../packaging/firefox-alpha-browser-entry/),
  [`cmd/ardents-browser-entry`](../../../cmd/ardents-browser-entry/), and
  [`internal/browserentry`](../../../internal/browserentry/).

### Experiment

The concrete signed alpha XPI now exists. The historical purpose-named signed-Firefox
qualifier starts a clean temporary profile, asks the operator to install that
exact XPI explicitly, and accepts a result only after the dynamic Publisher
proof and exact XPI/profile-index checks. It is deliberately separate from the
temporary `web-ext` fixture, which cannot prove Release Firefox accepts a
signed add-on. Its temporary native registration and C-2 input are still not a
participant Release Decision. The former plan to qualify both platforms after
an enrolled v4 bundle is retired by ADR-0061. If the optional compatibility
path is deliberately regression-tested, record its exact manifest and binary
bytes and preserve the owned-only removal and protected-root checks, without
promoting the result to participant or headless-network qualification.

### Failure scenarios

- A pre-existing foreign native-manifest key or file occupies the fixed host
  name; installer must refuse to overwrite it.
- XPI/native host version mismatch, altered native-manifest path, missing binary, malformed
  state, stale/recycled proxy port, browser disabled add-on, and Endpoint stop
  all fail closed for `.ard`.
- Interrupted install/replacement/removal leaves either prior exact component
  intact or no registered host; it cannot leave a foreign target registered.
- AMO, GitHub, release metadata, or the corpus publisher is compromised;
  Release/component verification must reject altered bytes before registration.
- A local same-user attacker alters files or Firefox configuration. This does
  not become a browser-isolation claim and needs explicit honest limitation.

## Findings

- **Sourced fact:** native-manifest provisioning is external to extension
  installation. Firefox resolves per-user native messaging through exactly the
  current user's HKCU key on Windows or `~/.mozilla/native-messaging-hosts` on
  Linux; it is not a DNS or proxy-setting mechanism.
- **Sourced fact:** a native manifest's `path` starts the host process and does
  not supply command arguments. The maintained native host now accepts that
  zero-argument invocation and uses its fixed per-user state path; the explicit
  `native-host --state` form remains a qualification-only override. A released
  per-user manifest can therefore point directly to its Release-bound binary.
- **Sourced fact:** Firefox Release/Beta requires Mozilla signing. A signed
  unlisted XPI can be self-distributed; AMO public listing is not technically
  required for that channel.
- **Implementation fact:** Windows qualification registers only the exact
  temporary HKCU key and a wrapper for its temporary state override, then
  removes the registry registration. It retains the wrapper only in its
  disposable evidence directory. This proves the mechanism, not
  install/update/removal ownership or signed distribution.
- **Implementation fact:** `BrowserEntryProfile: "firefox-alpha"` now selects
  `browserentry.DefaultStatePath`; the zero-argument host reads that exact
  per-user state path. The explicit absolute plan path remains the local
  qualification override and is rejected when combined with the stable profile.
- **Implementation fact:** the selected installer verifies a canonical
  enrollment-v4 manifest before it registers anything. It authenticates the
  Endpoint primary artifact, proves the running native host is the separately
  manifested companion, requires the exact manifested XPI's fixed Firefox ID
  and current Mozilla COSE-signature metadata, and writes only an owned
  per-user native manifest. Firefox remains the cryptographic signature
  verifier when the participant installs that exact XPI. The command prints
  `manual-required` for XPI installation; it does not launch or alter Firefox.
- **Implementation fact:** the installer uses an owner-only same-directory
  temporary file and rename for manifest replacement, refuses foreign manifest
  contents or a foreign HKCU registration, and its removal leaves Endpoint,
  Authority, corpus, and Release roots untouched.
- **Implementation fact:** `packaging/firefox-alpha-browser-entry/build-xpi.ps1`
  produces an external, signing-ready unsigned XPI containing only the reviewed
  fixed-ID manifest and background script. It refuses repository output or an
  overwrite, validates the archive surface and manifest bytes, and prints the
  resulting SHA-256 for the later enrollment-v4 record. The generated XPI is
  deliberately not called signed or installable until Mozilla returns the
  unlisted signed artifact.
- **Measurement:** on 2026-08-26, the Product Owner submitted the fixed-ID
  0.1.0 signing input to Mozilla's unlisted channel. Mozilla approved the
  resulting version with zero reported validation errors or warnings. The
  Product Owner then downloaded its signed XPI; its SHA-256 is
  `d88e8ecba84cda82a7b2354d1f445e19b9d092f3f3d068868d1173ef29eaa2a2`.
  The XPI contains `META-INF/cose.manifest` and `META-INF/cose.sig`, alongside
  Mozilla's JAR-signature files. Its `background.js` is byte-identical to the
  maintained source; parsed JSON for `manifest.json` matches both the source
  and uploaded signing input. This records an AMO-approved artifact and its
  provenance facts, not a Firefox installation or cryptographic verification
  claim by Ardents.
- **Measurement:** the purpose-named Windows enrollment-v4 qualifier then
  passed with that exact signed XPI. It built current Endpoint, Browser Entry,
  and Control artifacts into a unique temporary, manifest-pinned v4 inventory,
  ran the actual enrolled Browser Entry command, observed its exact current-user
  native manifest, and observed `remove` withdraw both that manifest and its
  HKCU registration. The emitted result recorded
  `native_manifest=installed-and-withdrawn` and `manual-required` XPI
  installation. The qualifier refuses a pre-existing registration and removes
  only its own temporary bundle. Its Windows host companion has the required
  `.exe` suffix. This is install/remove mechanics evidence; its local
  qualification control bytes are not a production Release Decision and it
  does not install the XPI into Firefox.
- **Measurement:** the matching H4-4 Ubuntu container qualifier passed with
  that same signed XPI and current cross-built Linux Endpoint, Browser Entry,
  and Control artifacts. It ran as UID 1000 in the locally available
  `ubuntu:24.04` image, made its manifest-pinned v4 bundle and `$HOME` only
  under a unique `/tmp` root, observed the exact per-user Firefox native
  manifest, then observed `remove` withdraw it before Docker cleanup. Its
  emitted result likewise recorded `native_manifest=installed-and-withdrawn`
  and `manual-required`. This is unprivileged Ubuntu container mechanics
  evidence, not a clean Ubuntu desktop/Firefox, authorized Release Decision,
  persistent participant installation, replacement, or recovery claim.
- **Measurement:** on 2026-08-26, the signed-Firefox qualifier passed on the
  Product Owner's Windows Firefox Release host with the exact approved XPI.
  The Product Owner explicitly installed it only into the runner's fresh
  temporary profile and opened `http://reference.ard/`. The C-2 browser
  workload then completed in 34.110 seconds. Before that workload began the
  qualifier pinned the XPI digest and reviewed surface; after it completed the
  runner required Firefox's retained profile XPI to have that same digest and
  its profile index to name `alpha-browser-entry@ardents.network` version
  `0.1.0`. This is evidence that Firefox accepted the selected signed artifact
  and that the signed add-on/native-host path carried the dynamic named HTTP
  flow. The runner then removed its temporary profile and owned registration.
  It is not a participant Release Decision, persistent installation,
  replacement, recovery, or Ubuntu desktop evidence.
- **Implementation fact:** the separate signed-Firefox qualifier never uses
  `web-ext` or a modified add-on. It opens only a fresh temporary Firefox
  profile and exposes the exact Mozilla-signed XPI for the operator's explicit
  installation. A successful run additionally requires Firefox's retained
  profile XPI to hash to the selected input and its profile index to name the
  fixed add-on ID and version before the C-2 browser workload proves the
  native-host route. It is not a production Release Decision or participant
  installation claim.
- **Measurement:** the isolated
  [`r-117-firefox-proxy-auth`](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-117-firefox-proxy-auth/)
  fixture passed on 2026-08-26 with Firefox 154. Its fixed temporary
  `.ard`-only Manifest V3 add-on used `proxy`, `nativeMessaging`,
  `webRequest`, and `webRequestBlocking`. It first obtained port `57737` from
  a temporary exact-ID native host; after the corresponding loopback proxy
  issued a `407 Basic` challenge, Firefox invoked that host again, matched its
  returned port to the challenger, and repeated
  `GET http://reference.ard/ HTTP/1.1` with the exact generated Basic
  credential. The fixture registered only its exact temporary HKCU native
  manifest and a post-run check found it absent. `web-ext@10 lint` reported
  zero errors, notices, and warnings. Its required negative control returned a
  different valid port only at the challenge: the original `57779` proxy saw
  only the first unauthenticated request and no credential-bearing retry.
- **Inference:** direct HTTP proxy authentication is technically available
  without `<all_urls>`, but the maintained extension cannot simply disclose
  its liveness capability after an old port proof: a stale/recycled port could
  issue the `407` and receive it. A candidate must ask the native host again at
  the challenge, require the current proved port to match the challenger, and
  then respond at most once. This narrows the handoff race but does not create
  a same-user isolation claim.

## Options

1. **Mozilla-signed unlisted XPI; self-distributed; per-user native manifest.**
   **Accepted by ADR-0045.**
   Fits the closed alpha and avoids a public AMO listing requirement. Mozilla
   remains the XPI signing dependency, while Ardents Release verification must
   bind the XPI, host binary, and manifest source. Requires explicit
   participant installation and a Mozilla developer account; no paid Windows
   code-signing certificate is introduced by this option.
2. **Public AMO listing; per-user native manifest.** Offers Firefox's familiar
   discovery/update UX but adds listing policy, review, public visibility, and
   AMO account governance. It does not remove the native-host or Endpoint
   configuration work.
3. **Unsigned XPI or temporary profile as delivery.** Rejected: the normal
   Firefox channel does not accept it, and qualification tooling is not a
   participant installation path.
4. **Native-manifest installer before stable Endpoint profile.** Rejected:
   current install would point at no assured state writer and would create an
   unsupported operational promise.
5. **Narrow revalidated HTTP proxy authentication.** **Accepted for the alpha
   tracer.** The fixed add-on adds `webRequest` and `webRequestBlocking`, both
   filtered to `.ard`, and on a loopback `407` revalidates the current
   host/port through native messaging before answering exactly one Basic
   challenge with a separate one-process credential. It binds the browser
   request more strongly than an unauthenticated proxy, but expands the signed
   add-on permission surface. ADR-0044 records the remaining same-user and
   port-rebinding limits; it does not select participant delivery.

## Recommendation

Historical recommendation: the Product Owner selected the narrowest Firefox
profile, a Mozilla-signed unlisted XPI self-distributed with a manifest-pinned
release plus exact per-user native manifest registration. ADR-0045 recorded
that decision. R-115 later falsified the participant Browser Entry hypothesis;
[ADR-0061](../../adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
supersedes the delivery selection. Do not run participant release qualification
for this profile. Any renewed Browser Entry starts with a new resolution and
HTTP/HTTPS trust decision.

**Confidence:** high that unlisted Mozilla signing is the narrowest Firefox
delivery channel; high that an installer before a stable Endpoint state profile
would be misleading. **Strongest argument against:** an unlisted XPI still
depends on Mozilla signing/account processes and gives users a less familiar
install/update journey than AMO.

## Disposition

Closed as participant-delivery research. ADR-0044 qualified the narrow
`.ard`-filtered browser permission/authentication slice; ADR-0045 selected and
implemented the bounded source-level native-manifest lifecycle and its v4
provenance model. Mozilla signed the initial fixed-ID XPI, but R-115's clean
Firefox resolver trace falsified the required no-leak behavior. ADR-0061 keeps
these mechanics only as optional compatibility evidence, outside the headless
network product and outside candidate qualification. R-113 remains the owner
of corpus distribution; name release/reclaim remains control research.
