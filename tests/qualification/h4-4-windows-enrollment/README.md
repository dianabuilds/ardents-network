# H4-4 Windows enrollment-v4 Browser Entry qualification

## Question

Can the actual Windows Browser Entry command authenticate one temporary
enrollment-v4 inventory that contains the current Endpoint, Browser Entry,
Control companions, and Mozilla-signed XPI, register only its owned native
manifest, and withdraw it completely?

## Run

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4-windows-enrollment/run-windows.ps1 -SignedXPI 'C:/absolute/path/to/signed.xpi'
```

The qualifier refuses to replace any existing Browser Entry registry key or
native manifest. It creates a unique temporary bundle, builds the three exact
current command artifacts there, constructs one pinned enrollment-v4 manifest,
performs installation, verifies the owned per-user native manifest, removes it,
and deletes only its own temporary bundle.

## Scope and disposition

This proves the Windows install/remove mechanics with the Mozilla-approved
0.1.0 XPI, not Firefox's explicit add-on installation, cryptographic signature
acceptance, an authorized production Release Decision, update/replacement, or
an Ubuntu result. Firefox remains the signature verifier when the participant
installs the signed XPI manually. R-117 records the resulting evidence.
