# H4-4 signed Firefox Browser Entry qualification

Does Firefox Release accept the exact Mozilla-signed H4-4 XPI in a clean
temporary profile, use its fixed native host, and load the dynamic ordinary
Publisher application at `http://reference.ard/`?

Run on an interactive Windows desktop:

```powershell
make qualification-h4-4-signed-firefox ARDENTS_REFERENCE_C2_FIREFOX='C:/Program Files/Mozilla Firefox/firefox.exe' ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'
```

The runner first pins the selected XPI's hash and reviewed release surface.
It then starts a clean, disposable Firefox profile and opens Explorer at that
same XPI. The operator drags the XPI into Firefox, approves its explicit
installation, and opens `http://reference.ard/`. The page drives a normal
form/cookie/redirect/chunked-response/close sequence. The runner accepts the
result only if the proof completes, Firefox retained exactly that XPI in its
clean profile, and `extensions.json` identifies the fixed add-on ID and
version.

It registers only a temporary current-user native host and removes the exact
registration afterwards. It does not alter DNS, browser proxy settings, CA
stores, the ordinary Firefox profile, or a production Release root. A missing
Firefox, absent/altered XPI, user refusal, timeout, or failed native route is
an invalid selected environment, not a passing skip.
