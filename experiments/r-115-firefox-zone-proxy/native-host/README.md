# Disposable Firefox native-messaging host

This Windows-only research fixture receives one framed native-messaging JSON
request on standard input and returns only its temporary loopback proxy port.
The launcher replaces its dummy value in a temporary copy, writes a temporary
manifest that permits only `r115-fixture@ardents.invalid`, registers that
manifest under one exact HKCU key, and removes only that key during cleanup.

It is neither an Ardents Endpoint command nor a distributable native host. It
has no credential, Service Name, Target, network, filesystem state, or browser
configuration authority. A maintained host would need a stable signed add-on
ID, owner-only Endpoint state/capability, process lifecycle, release
provenance, and Windows/Linux installer/removal evidence.

The fixture's binary framing was exercised directly, then through a Firefox
154 temporary add-on on 2026-08-26. The launcher confirmed that its exact HKCU
key no longer existed after cleanup. This does not promote the fixture into a
product host.
