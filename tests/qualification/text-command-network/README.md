# Installed closed text command journey

`make text-command-network-check` runs the end-to-end command journey only on
a dedicated Ubuntu 24.04 x86-64 host with systemd 255 and cgroup v2. It is not
an ordinary unit or container test.

Build the candidate outside the repository with Go 1.26.8 and install these
root-owned mode-0555 regular files:

- `/usr/lib/ardents/qualification/closed-text-commands.test` from
  `go test -c -tags text_worker_installed -trimpath -buildvcs=false -o OUTPUT ./tests/e2e/node`;
- `/usr/lib/ardents/qualification/commands/{ardents,ardents-custody,ardents-node,ardents-control,ardents-text}`;
- `/usr/lib/ardents/text-worker-root/ardents-text`, byte-identical to the
  pinned `ardents-text` command candidate.

Install a fresh root-owned mode-0644 `/run/systemd/system/ardents-endpoint.service`
with no drop-ins. The root test binary itself temporarily replaces that unit
with the unprivileged Endpoint command, then restores it. Provide independent
SHA-256 values for the test binary, unit, five commands and worker through the
`ARDENTS_TEXT_COMMAND_*_SHA256` variables.

The runner rejects changed, symlinked, non-root-owned or missing inputs. It
requires exactly one root result and each empty, 64 KiB and 4 MiB subtest on
TCP/TLS and QUIC. Preserve the complete invocation journal and artifact/host
inventories outside Git. This is functional journey evidence, not whole-host,
privacy, hostile-network or p95 qualification.

Each of the six document/Carrier cases observes a real five-minute Descriptor
refresh. The Go test deadline is therefore 49 minutes and the independent
outer command limit is 51 minutes; a timeout is retained as evidence rather
than retried or treated as a pass.
