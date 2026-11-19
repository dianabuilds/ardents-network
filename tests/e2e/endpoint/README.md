# Endpoint command end to end

This package builds the real `ardents` command and runs it against exact
artifacts placed in synthetic closed-alpha bundles. It proves that the
four-argument `endpoint enrollment-check <bundle-root> <manifest-sha256>`
accepts an independently pinned, exact local inventory through the successor
enrollment verifier with a byte-identical diagnostic output contract, and
refuses a changed manifest before parsing it. The historical persisted
`ardents-alpha-enrollment-input-v1` document form returns its typed retirement
refusal before reading the document or touching the bundle (ADR-0127, #647).

The per-user Portable Endpoint lifecycle process evidence formerly here
(`endpoint enroll`/`enroll-installed` readiness and release floors, the
fixed-user-unit replacement and journal-bound rollback cases, the installed
`.deb` re-enrollment case, and the live two-Endpoint alpha-control agreement
cases) is retired with that command surface (ADR-0125, #617 S3.5). The
enrollment-pinned ACA1 inspection regressions formerly here retired with the
reader itself (ADR-0127, #647); the retired `accept-alpha-corpus` and
`inspect-alpha-corpus` before-effect refusals remain covered by the
`cmd/ardents-control` command tests. The canonical protected text Endpoint
launch is the platform-led `ardents-next` system-unit composition; its
installed qualification is a separate selected gate. The shared ephemeral
Release-metadata fixtures survive for the Ubuntu deb package payload/retention
case, which pins an exact artifact offline.

The bundle, TUF keys, and metadata are test-only and ephemeral. This is process
evidence for the post-execution Enrollment Pin handoff; it is not an external
first-execution verifier, alpha provenance, a participant-contact proof, a
network source, or a qualification of a released artifact.

For an explicitly cross-compiled Linux qualification only, the test runner may
provide the matching command through `ARDENTS_E2E_COMMAND`; ordinary local and
CI runs leave it unset and build the command from the current checkout.