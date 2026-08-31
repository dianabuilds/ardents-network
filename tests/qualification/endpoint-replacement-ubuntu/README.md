# Ubuntu Endpoint replacement qualification

Run `make qualification-endpoint-replacement-ubuntu` only on a clean,
unprivileged Ubuntu Linux
account with a live `systemd --user` manager, `XDG_RUNTIME_DIR`, `linger=no`,
and `sha256sum`. A missing prerequisite is an invalid environment and fails
the selected target.

The matrix runs the exact command artifact behind the actual fixed per-user
unit. It proves a Release-authorized local v1-to-v2 replacement stops,
self-tests, and explicitly restarts that unit without changing linger or
protected state. Its recovery case proves a valid-but-failing v2 remains
stopped, then only its owner-only retained predecessor can perform a fresh
Release-authorized rollback and restart v1.

The fixture Enrollment Pin, TUF metadata, and keys are ephemeral. Passing this
target qualifies only the explicit Ubuntu foreground replacement paths. It is not
evidence for downloaders, unattended update, repair across arbitrary crashes,
package updates, Windows, or a public release.
