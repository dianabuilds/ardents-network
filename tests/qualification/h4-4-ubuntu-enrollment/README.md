# H4-4 Ubuntu container enrollment-v4 Browser Entry qualification

## Question

Can actual Linux Browser Entry bytes authenticate a temporary enrollment-v4
inventory containing the Mozilla-approved XPI, create exactly one per-user
Firefox native manifest, and withdraw it under an unprivileged Ubuntu process?

## Run from Windows Docker

```powershell
make qualification-h4-4-ubuntu-enrollment ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'
```

The runner cross-builds the current Linux Endpoint, Browser Adapter, Browser Entry, and Control
artifacts outside the repository, starts the already available `ubuntu:24.04`
image as UID 1000, and mounts the source, signed XPI, and artifacts read-only.
The container writes its manifest-pinned v4 inventory and `$HOME` only under a
unique `/tmp` root, then removes them before it exits.

## Scope and disposition

This is Ubuntu container evidence for the native-manifest install/remove
mechanics. It is not a clean Ubuntu desktop session, Firefox XPI installation,
Firefox cryptographic-signature acceptance, an authorized production Release
Decision, persistent participant installation, replacement, or recovery.
