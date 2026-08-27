# H4-4 Mozilla-signed XPI qualification

## Question

Did Mozilla return the exact H4-4 Browser Entry XPI selected for the alpha,
with the reviewed ID, permissions, host scope, and current COSE signing
surface?

## Hypothesis

The unlisted AMO artifact for `alpha-browser-entry@ardents.network` version
`0.1.0` is the exact signed XPI recorded in R-117. It has the selected
Firefox-only scope; it is not a participant-install or browser-traffic proof.

## Run

On Windows, provide the downloaded signed artifact explicitly:

```powershell
make qualification-h4-4-signed-xpi ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'
```

The qualifier requires an absolute regular file. Its missing or changed input
is an invalid selected environment and fails; it never substitutes an unsigned
build, a download URL, or a different signed version.

## Evidence and disposition

The test pins SHA-256
`d88e8ecba84cda82a7b2354d1f445e19b9d092f3f3d068868d1173ef29eaa2a2`, checks
the fixed ID, version, permissions, `.ard` host scope, and COSE metadata.
Firefox performs the actual cryptographic signature acceptance when the user
installs the XPI. The result is recorded in R-117.

This is only the signed-artifact gate. A real enrollment-v4 release bundle and
Windows/Ubuntu install, replace, remove, fresh-profile, failure, and runtime
qualification remain open.
