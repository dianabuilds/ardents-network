# Ubuntu portable Endpoint qualification

Run `make qualification-endpoint-portable-ubuntu` only on a clean,
unprivileged Ubuntu Linux
account with a live `systemd --user` manager, `XDG_RUNTIME_DIR`, `linger=no`,
and `sha256sum`. The target is intentionally separate from ordinary process
tests: absence of any prerequisite is an invalid environment and fails.

The selected matrix builds the actual `cmd/ardents` command, places that exact
byte sequence into a synthetic closed-alpha bundle, verifies the pinned manifest
through host `sha256sum` before executable permission is granted, renders the
actual per-user unit, then proves start/readiness, stop, restart, state/floor
retention, program-byte deletion without Vault-root deletion, and unchanged
linger. It also rejects root execution and any unit containing `User=`.

The fixture Pin, metadata, and keys are ephemeral test data. Passing this target
is portable Endpoint profile evidence only; it is not a public artifact, participant
contact, independent build, Windows, automatic replacement, repair, recovery,
or public-release qualification.
