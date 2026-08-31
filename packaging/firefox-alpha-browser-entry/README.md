# Firefox alpha Browser Entry source

This directory is the fixed-ID Firefox extension source and native-host
manifest template for the alpha Browser Entry. The extension routes only
`http(s)://*.ard/` through its native host. Its only permissions are
`proxy`, `nativeMessaging`, `webRequest`, and `webRequestBlocking`; both web
request listeners are constrained to the same `.ard` host matches. It has no
`tabs`, DNS, proxy-settings, broad-host, or browser-launch permission.

The manifest starts the separately built `ardents-browser-entry` binary without
arguments. It reads its fixed per-user state file, probes the exact current loopback
`AlphaProxy` with a random probe capability, and returns only its port. The
proxy responds to ordinary alpha requests with `407 Basic`; before the add-on
answers that challenge, it calls the native host again. The host reproves the
current proxy, returns an independent random proxy password only when its port
still equals the challenger, and the add-on answers once. An inactive, stale,
malformed, unprobed, or mismatched state makes the extension cancel/fail through
a known unavailable loopback proxy; it returns no ordinary DNS, direct, or
browser-configured proxy route for the alpha suffix. This describes the
extension's returned proxy policy only: a clean Firefox 154 trace still made
native resolver calls for `.ard` names before the HTTP proxy path. The artifact
is therefore retained as compatibility evidence, not a no-DNS/DoH Browser
Entry. The probe capability is not the proxy password, and
`Proxy-Authorization` is stripped before the local Publisher origin receives
the request.

The extension has no proxy listener for non-`.ard` URLs. Ordinary Firefox tabs
therefore use the browser's existing networking behavior. This is deliberate:
the extension is the local Ardents resolver hook for the `.ard` suffix only,
not a general browser networking mode.

This is source, not an installable participant release. A supported release
still requires Mozilla signing of this exact fixed-ID XPI, manifest-pinned
binary and XPI provenance, a per-user native-manifest installer/remover for
Windows and Ubuntu, normal Endpoint configuration of that state path,
update/recovery behaviour, and clean-profile DNS/DoH and
failure qualification. It intentionally provides HTTP alpha only. HTTPS
reaches the exact local proxy as CONNECT and fails locally until a separate
address/trust decision is accepted; no CA, certificate, port 80/443 listener,
or system resolver change is part of this artifact.

## Signing input

`build-xpi.ps1 -OutputPath <absolute external path>.xpi` packages only the
reviewed `manifest.json` and `background.js` into a signing-ready unsigned XPI.
It refuses an output under the repository or an existing output path, verifies
the fixed add-on ID and archive surface, and prints the XPI SHA-256. It neither
contacts Mozilla nor claims a signature. Submit that exact output to Mozilla's
unlisted signing channel, retain the returned signed XPI and its digest as
release material, then bind the signed bytes together with the native host and
Endpoint artifact in the enrollment-v4 release manifest.
