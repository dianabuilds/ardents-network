# Disposable Firefox add-on fixture

This is not a distributable Ardents extension. It has a fixed test-only add-on
ID, no ordinary UI, no native messaging, no credentials, no storage, no
content script, and only `http(s)://*.ard/*` host permission. On temporary
installation only, it opens the fixed `http://reference.ard/` probe after
registering its proxy listener, then an unavailable `.ard` probe, an HTTPS
`.ard` probe, and one ordinary `.invalid` URL. This is test instrumentation
rather than a participant action. A test launcher replaces the dummy loopback
port in a copy outside the repository.

Its sole question is whether Firefox 154 accepts the documented
`proxy.onRequest` shape `[{ localProxy }, null]` for the `.ard` zone. The
trailing `null` is the important safety property: Firefox must not use another
browser-defined proxy if the loopback listener fails. The fixture does not
measure DNS, DoH, or direct-path behavior.

Do not package, sign, install, or offer this add-on to any participant. A real
Browser Entry would need a Product Owner decision, a stable signed add-on ID,
native-message manifest allow-list, per-Endpoint loopback capability,
unavailable-Endpoint behaviour, extension release provenance, and a separate
HTTPS design.
