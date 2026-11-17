# Endpoint command end to end

This package builds the real `ardents` and `ardents-control` commands and runs
them against exact artifacts placed in synthetic closed-alpha bundles. It proves
that `endpoint enrollment-check` accepts an independently pinned, exact local
inventory and refuses a changed manifest before parsing it, including the
historical persisted-input form. On Linux it also creates ephemeral
TUF-compatible Release metadata for an exact artifact and proves the
enrollment-pinned `ardents-control inspect-bundle` reader reaches an accepted
Release Decision and cached restart observation, and that the retired
`accept-alpha-corpus` intake returns its stable retirement refusal before a
control root exists while the seeded corpus floor bytes remain identical.

The per-user Portable Endpoint lifecycle process evidence formerly here
(`endpoint enroll`/`enroll-installed` readiness and release floors, the
fixed-user-unit replacement and journal-bound rollback cases, the installed
`.deb` re-enrollment case, and the live two-Endpoint alpha-control agreement
cases) is retired with that command surface (ADR-0125, #617 S3.5). The canonical
protected text Endpoint launch is the platform-led `ardents-next` system-unit
composition; its installed qualification is a separate selected gate. The
shared ephemeral Release-metadata fixtures survive only for the read-only
inspection regressions above.

The bundle, TUF keys, and metadata are test-only and ephemeral. This is process
evidence for the post-execution Enrollment Pin and Release Decision handoff; it
is not an external first-execution verifier, alpha provenance, a
participant-contact proof, a network source, or a qualification of a released
artifact.

For an explicitly cross-compiled Linux qualification only, the test runner may
provide the matching command through `ARDENTS_E2E_COMMAND`; ordinary local and
CI runs leave it unset and build the command from the current checkout.
