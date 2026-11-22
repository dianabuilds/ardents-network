# Remote development stands

Use `scripts/standctl/standctl` from the Ardents checkout you want to test. The
client uses Python 3.9 or later, Git, and OpenSSH. Each stand has a single SSH
account shared by the pool, a local Docker socket, the Docker Compose plugin,
and Python with `tarfile.data_filter`. Stands run the Ardents diagnostic image
as described in the [diagnostics guide](../diagnostics/README.md).

## Share one inventory across checkouts

The default inventory is `${XDG_CONFIG_HOME:-$HOME/.config}/standctl/hosts.json`.
Set `STANDCTL_CONFIG` to an absolute path to share an alternate inventory
across directories. An explicit `--config` before the subcommand takes
precedence. Copy the
[example](hosts.example.json) to the user configuration directory for a new
setup. Keep real addresses, SSH identities and host key files outside the
repository. Relative `identity` and `known_hosts` paths are resolved next to
the actual inventory file. Verify the server host keys and give the shared
stand account the public key before its first SSH use.

From any fresh checkout on a machine with pool access:

```sh
scripts/standctl/standctl doctor
scripts/standctl/standctl run --owner smoke -- go version
```

An operator can deploy the matching host handler with
`scripts/standctl/standctl install <stand>`. The handler is addressed by its
content hash, so checkouts that use the same handler share the existing pool.
Deploy a new handler on each affected stand before using a checkout that
changes it. Versions share the same per-host lease state.

## Select and identify a run

`run` takes the first free stand meeting the selected Compose service
requirements. The host checks those requirements again under the lease lock.
`doctor` shows readiness and lease state separately. Use `--host` to pin a
stand and `--min-cpus` or `--min-memory-gib` for stricter admission. The sample
inventory assigns small hosts a two-CPU cap for `runner` and `online` and
requires four usable CPUs for `gate`.

`run` and `acquire` report the inventory, checkout and exact Git commit on
stderr. By default the caller's current directory selects the Git worktree
and `HEAD` selects the revision. The client archives the entire commit from
the repository root, including when invoked from a subdirectory. The archive
contains committed files. Set `--repo` to choose a checkout from another
location, and `--ref` to select a commit, branch or tag in that checkout.

```sh
/absolute/ardents/scripts/standctl/standctl run --repo /absolute/ardents-other --ref HEAD --owner qa -- go version
scripts/standctl/standctl run --service online -- make quick-check
scripts/standctl/standctl run --service gate -- ardents-diagnostics run -raw -out /evidence/check -timeout 2h -- make check
```

## Lease lifecycle

`run` displays the container result, removes resources labelled with its
Compose project and releases the lease after checking cleanup. For an
interactive sequence, use `acquire`, then `up`, `ps`, `exec`, `logs` and
`release` with the returned lease reference. `up` expects a long-lived
service. Agents provide distinct labels with `--owner` or `STANDCTL_OWNER`.

A disconnected SSH client can leave the remote command running. Keep the
lease reference from stderr and use `status` and `logs <lease>` to inspect it.
Use `release <lease>` after a joined operation. For `recovery-required`,
confirm that the previous worker and its Docker operations have stopped
before using `recover <lease> --checked`. Run logs and evidence are retained
under the stand account's `.local/share/standctl/results/<id>` directory.

The Docker pool is for trusted development work. Its readiness checks cover
the Docker environment and the configured resources. Qualification follows
the project's [testing profiles](../../docs/development/testing.md).
