# Endpoint command end to end

This package builds the real `ardents` command and runs it from the exact
artifact placed in a synthetic closed-alpha bundle. It proves that
`endpoint enrollment-check` accepts an independently pinned, exact local
inventory and refuses a changed manifest before parsing it. On Linux it also
creates ephemeral TUF-compatible metadata for that exact artifact and proves
`endpoint enroll` reaches a Release Decision plus Portable readiness, commits
release floors, and removes the local attachment after a participant stop.
The H4-1B case then proves an explicit local replacement bundle with newly
accepted metadata stops only the fixed user-unit name, atomically runs the
candidate's no-network self-test, commits its successor binding, and permits a
later ordinary start. Its `systemctl` process is a narrow fixture that records
the exact arguments; it is not native `systemd --user` qualification. A
separate H4-1B recovery case makes the activated candidate fail, then proves
that only the retained journal-bound predecessor program can invoke an
explicit rollback with fresh successor Release metadata for those exact bytes.
The H4-1D Linux package case builds two direct `.deb` files with versioned
static enrollment roots. It proves install, explicit re-enrollment after a
distinct v2 package and Release metadata set, removal, and purge retain Vault
and Release-floor state outside package ownership.

The H4-4A control case additionally creates an exact v3 enrolled bundle with
an independent corpus root. It invokes `ardents-control accept-alpha-corpus`
for one ACA2/corpus, accepts a successor serial, rejects an attempted rollback,
and verifies that the persistent floor still resolves the successor Target.

The bundle, TUF keys, and metadata are test-only and ephemeral. This is process
evidence for the post-execution Enrollment Pin and Release Decision handoff; it
is not an external first-execution verifier, alpha provenance, a
participant-contact proof, a network source, or a qualification of a released
artifact.

For an explicitly cross-compiled Linux qualification only, the test runner may
provide the matching command through `ARDENTS_E2E_COMMAND`; ordinary local and
CI runs leave it unset and build the command from the current checkout.
