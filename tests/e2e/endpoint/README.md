# Endpoint command end to end

This package builds the real `ardents` command and runs it from the exact
artifact placed in a synthetic closed-alpha bundle. It proves that
`endpoint enrollment-check` accepts an independently pinned, exact local
inventory and refuses a changed manifest before parsing it. On Linux it also
creates ephemeral TUF-compatible metadata for that exact artifact and proves
`endpoint enroll` reaches a Release Decision plus Portable readiness, commits
release floors, and removes the local attachment after a participant stop.

The bundle, TUF keys, and metadata are test-only and ephemeral. This is process
evidence for the Enrollment Pin and Release Decision handoff; it is not alpha
provenance, a participant-contact proof, a network source, or a qualification
of a released artifact.

For an explicitly cross-compiled Linux qualification only, the test runner may
provide the matching command through `ARDENTS_E2E_COMMAND`; ordinary local and
CI runs leave it unset and build the command from the current checkout.
