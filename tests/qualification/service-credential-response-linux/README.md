# Service Credential response publication Linux profile

Run `make qualification-service-credential-response-linux` to exercise the
real Linux public custody boundary for a failed Credential response write.

The runner cross-builds the current `ardents` and `ardents-custody` commands
and the selected E2E test for Linux `amd64`, then runs them as the invoking
non-root numeric user in one disposable `golang:1.26.6` container. The
container is network-disabled, read-only outside its temporary directory, and
capability-dropped. It needs a POSIX shell with `id`, `mktemp`, and `rm`, an
accessible Docker daemon, the already-present `golang:1.26.6` image, a local Go
toolchain, and the immutable module cache.
Missing prerequisites fail the profile as an invalid environment; the runner
never skips or pulls an image or module.

The test creates an Authority through `ardents-custody`, initializes the real
host request through `ardents`, and gives the request SHA-256 and Vault password
through a real PTY. It first establishes the deterministic encrypted successor
through one successful real issuance at a distinct public response path, so the
file-size limit reaches the public response boundary rather than earlier Vault
persistence. It then executes the same custody request at a fresh response path
under `ulimit -f 0`. It requires the real Linux file-size-limit failure (either
`SIGXFSZ` or the command's nonzero `file too large` result) and a visible
zero-byte regular response path without a response receipt.
It then repeats the byte-identical custody command without the limit, at that
same response path, and requires the public receipt, public response bytes, and
real host acceptance to recover. Every captured custody terminal transcript is
checked for absence of the password.

This is a focused public response-publication regression. It does not qualify
arbitrary power loss, storage devices, filesystem semantics outside this Linux
container, Authority recovery, or broader Endpoint durability.
