# Installed closed text command journey

`make text-command-network-check` runs the end-to-end command journey only on
a dedicated Ubuntu 24.04 x86-64 host with systemd 255 and cgroup v2. It is not
an ordinary unit or container test.

From the exact source commit, run
`ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT=/absolute/stages make text-command-network-build`
with Go 1.26.8 on Linux x86-64. The parent must be an existing directory
outside the repository. The build creates a new private stage with
`READY`, `GO-VERSION`, and `SHA256SUMS`. A stage without `READY` is incomplete;
retain its failure output rather than installing it. Record the source commit
alongside the stage and verify `SHA256SUMS` before and after copying.

Install these files from that stage as root-owned mode-0555 regular files:

- `/usr/lib/ardents/qualification/closed-text-commands.test` from
  `closed-text-commands.test`;
- `/usr/lib/ardents/qualification/commands/{ardents,ardents-custody,ardents-node,ardents-control,ardents-text}`
  from `commands/`;
- `/usr/lib/ardents/text-worker-root/ardents-text`, byte-identical to the
  pinned `worker/ardents-text` and `commands/ardents-text` candidate.

Update the root-owned `/etc/ardents/text-worker-artifact.json` so its worker
digest matches that exact candidate. Start the installed
`ardents-text-reader.socket` and `ardents-text-publisher.socket` units; both
must expose Endpoint-owned mode-0600 sockets under `/run/ardents-text/`.
The runner checks these prerequisites before creating the network topology.

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

Set `ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT` to an existing absolute, root-owned,
mode-0700 directory outside the repository to retain the complete test output
after either success or failure. The runner creates a unique mode-0600 file
there and prints its path before starting the test, so an interrupted run can
still be inspected. Without this variable, the output is printed to the caller
and the temporary file is removed. The retained file may contain private
runtime observations; keep its access and retention under local operator control.

Each of the six document/Carrier cases observes a real five-minute Descriptor
refresh. The Go test deadline is therefore 49 minutes and the independent
outer command limit is 51 minutes; a timeout is retained as evidence rather
than retried or treated as a pass.
