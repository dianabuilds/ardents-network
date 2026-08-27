# H4-4A Firefox qualification

This qualification copies the maintained fixed-ID Firefox add-on source into a
temporary `web-ext` profile, builds the maintained Go native host into a
temporary file, wraps it only to supply a temporary state path, and registers
one temporary exact HKCU native-manifest key. Firefox requests the visible
`http://reference.ard/` address. Only that request causes Endpoint to resolve
the name from its accepted alpha corpus and open the exact selected Service;
the resulting route loads the declared document, stylesheet, and SVG. The
runner also proves an ordinary URL uses the profile's separate fallback proxy
and no `.ard` HTTP/HTTPS request reaches it. The temporary install is
qualification instrumentation, not a participant installation or supported
Browser Entry.

It does not configure the user's Firefox profile, DNS, system proxy, VPN, or
CA/trust stores. It does not qualify a final participant-visible HTTPS address,
DNS/DoH non-leakage, generic applications, signing, updates, or
installation/removal. The qualification then starts the eleven-process C-2
dynamic Service fixture. Its temporary Firefox tab loads an ordinary Publisher
document, submits a normal form, follows its redirect and cookies, consumes a
chunked response, and navigates to close; no Go HTTP client performs that
workload. The Endpoint must remove the Browser Entry state before the selected
Service is withdrawn. A final ordinary C-2 process run deliberately clears the
Firefox environment variable and retains the fixture-client compatibility leg.

Run `make qualification-h4-4a-firefox` from Windows. It requires the absolute
path to Firefox in `ARDENTS_REFERENCE_C2_FIREFOX`; a missing executable or any
test failure is an invalid qualification environment, never a skipped pass.
The runner executes the alpha demand-resolution proof and then the full C-2
process fixture with the same selected executable.
