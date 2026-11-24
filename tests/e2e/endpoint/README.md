# Endpoint command end to end

This package runs the real `ardents-next enrollment verify` command
against an independently pinned v5 bundle. It checks that the exact running
Endpoint artifact is accepted and that a changed manifest fails before parsing.
The Endpoint archive filename remains `ardents-<platform>`.

The per-user Portable Endpoint lifecycle process evidence formerly here
(`endpoint enroll`/`enroll-installed` readiness and release floors, the
fixed-user-unit replacement and journal-bound rollback cases, the installed
`.deb` re-enrollment case, and the live two-Endpoint alpha-control agreement
cases) is retired with that command surface (ADR-0125, #617 S3.5). The
enrollment-pinned ACA1 inspection regressions formerly here retired with the
reader itself (ADR-0127, #647). The former Control executable and its
retirement-only corpus selectors have been removed. The canonical protected text Endpoint
launch is the platform-led `ardents-next` system-unit composition; its
installed qualification is a separate selected gate. The shared ephemeral
Release-metadata fixtures survive for the Ubuntu deb package payload/retention
case, which pins an exact artifact offline.

The bundle, TUF keys, and metadata are test-only and ephemeral. This is process
evidence for the post-execution Enrollment Pin handoff; it is not an external
first-execution verifier, alpha provenance, a participant-contact proof, a
network source, or a qualification of a released artifact.

The headless profile passes the exact prebuilt Endpoint artifact through
`ARDENTS_E2E_COMMAND`. Ordinary local runs build it from the current checkout.

The portable Enrollment process test verifies the exact compiled program and
refuses a wrong pin, changed manifest and substituted metadata. The portable
CI jobs also run this case on macOS and Windows.
