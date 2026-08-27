# R-101 — Endpoint state and local attachment harness

## Question

Can an ordinary-user candidate keep replaceable artifact bytes physically
separate from all seven Endpoint state classes, and use one filesystem Unix
socket for generic local attachment on the declared Windows 11 / Ubuntu LTS
platforms?

## Hypothesis

On each platform where Go reports Unix-socket support, the harness creates the
seven state classes under a separately owned state root; rejects a duplicate
owner and duplicate listener; completes a local request/reply; records whether
listener closure leaves a stale socket; exercises only the narrow cleanup path;
and proves that deleting a synthetic artifact directory does not delete the
state root. A successful result does **not** establish same-user adversary
isolation, path ACL correctness, a release lifecycle, or the future maintained
Endpoint implementation.

## Run

Use an ordinary user with the repository's selected Go toolchain.

On Windows PowerShell:

```powershell
pwsh -NoProfile -File .\run-windows.ps1
```

On Ubuntu LTS:

```sh
bash run-ubuntu.sh
```

Both wrappers select one fixed, experiment-specific directory under the OS
temporary location, refuse to reuse it, and remove only that exact directory
after the Go harness exits. They do not use Ardents identity material or change
any real Endpoint state.

## Evidence

Record the operating-system/build, Go version, all `key=value` output, exit
status, and whether cleanup succeeded in R-101. A failure with an unsupported
Unix-socket capability is evidence for the defined H4-1A failure path; it must
not silently change to a loopback listener.

The local Ubuntu 24.04 WSL measurement used a Linux/amd64 binary cross-compiled
by the selected Windows Go toolchain because that guest has no Go installation.
It ran the same `-root` argument directly and then verified/remediated the
fixed temporary root explicitly; a normal `run-ubuntu.sh` run performs this
cleanup through its trap.

## Abrupt-termination follow-up

The harness also accepts `-hold-ready <marker>`. It creates the state root,
exclusive fixture owner file, and Unix listener; writes `ready` to the marker;
then waits without a graceful close. A platform wrapper must terminate that
child, inspect the fixed root for the socket and owner file, and remove only
the verified experiment root. The expected research result is not a particular
socket-cleanup value: it is evidence whether the current fixture ownership
scheme can distinguish a live owner from stale durable residue. The fixture's
`O_EXCL` owner file is intentionally **not** a proposed Endpoint lock.

For Ubuntu, first produce a Linux/amd64 harness binary with the selected Go
toolchain, then run:

```sh
bash run-ubuntu-crash.sh /path/to/r-101-linux-harness
```

The wrapper refuses reused paths, kills only its child harness, prints the
post-kill observations, and deletes only `/tmp/a-r101c`, its ready marker, and
its log.

## Disposition

Disposable experiment for a decided question. Its composition is promoted to
H4-1; retain only until the unique evidence enters source history, then
supersede it with purpose-owned Endpoint behavior and qualification tests. It
is not maintained application code.
