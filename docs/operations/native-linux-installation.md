# Native Linux Installation

> Status: native lifecycle qualification candidate. CI accepts first install,
> restart, same-build reinstall, old-to-new upgrade, failed-upgrade rollback,
> explicit rollback, stopped-node backup/restore, and non-destructive uninstall.
> A published support promise still requires validation on the declared Linux
> distribution matrix and an operational observation window.

## Supported Shape

The native `v1` target is a Linux amd64 host using systemd. It runs the same
`ardentsd` artifact as the Docker image and does not require Docker, a workload
executor, or an ingress proxy. The default workload executor remains disabled.

The release archive contains:

- `ardentsd` and `ardentsctl`;
- `scripts/install/linux.sh`;
- `systemd/ardentsd.service`.

## First Installation

Run from the extracted release directory:

```sh
sudo ./scripts/install/linux.sh install \
  --node-name node-1 \
  --transport-port 61001
```

Add `--bootstrap-peer <multiaddr>` when joining an existing network endpoint.
The installer creates the locked `ardents` service account, initializes the
node once, installs the systemd unit, enables it, and starts it. It never
enables Docker workload execution implicitly.

Installed paths are stable:

| Path | Ownership and purpose |
| --- | --- |
| `/usr/local/bin/ardentsd` | root-owned daemon executable |
| `/usr/local/bin/ardentsctl` | root-owned operator client |
| `/etc/ardents/operator.json` | private active operator configuration |
| `/var/lib/ardents` | persistent node identity, databases, and retained data |
| `/var/lib/ardents/secrets` | persistent API and protected-state keys |
| `/var/lib/ardents-applications` | `ardents-apps`-scoped bootstrap Application credential and Unix socket |
| `/var/lib/ardents-authority` | root-only local bootstrap authority material |
| `/etc/systemd/system/ardentsd.service` | hardened native service definition |

The local authority is bootstrap material for the self-contained first-node
path. It is not a replacement for externally governed realm capability
issuance. Back it up separately and do not expose it to applications.

## Operator Access

Same-host access uses the private Unix socket:

```sh
sudo ardentsctl \
  --addr unix:///var/lib/ardents/secrets/control.sock \
  --legacy-token-file /var/lib/ardents/secrets/api-token \
  node status
```

Remote administration uses `ardentsctl --ssh`; the daemon API remains bound to
loopback and is not published as remote HTTP.

Local applications use the separate
`/var/lib/ardents-applications/application.sock` with the bootstrap credential in
`/var/lib/ardents-applications/application-token`, or the dedicated loopback
Application Interface on `127.0.0.1:8081`. This credential cannot call the
Operator Interface. Applications should use the Go SDK rather than generated
wire bindings directly.

The installer creates the `ardents-apps` system group and protects the shared
bootstrap boundary with a setgid `0750` directory, `0640` token, and `0660`
socket. Add only the service account that should consume the bootstrap
credential, then restart that service so its supplementary groups are refreshed:

```sh
sudo usermod --append --groups ardents-apps hello-service
```

The bootstrap Application credential is time-bounded. Renew its active local
admission before expiry with `sudo ./scripts/install/linux.sh
renew-application`. The operation atomically updates the active operator
document and restarts the service when it was running. PIA-011B includes the
separate one-use Principal enrollment protocol and protected-file Operator CLI,
but `application_interface.principal_enrollment_enabled` remains default-off
and is rejected until PIA-014 supplies owner-aware Principal Blob/content
access. PIA-012 admission alone does not make knowledge of a CID authorization
to read.
Do not bypass that readiness gate: replacement enrollment atomically disables
the exact legacy credential. PIA-017 later governs token rotation, retirement
state, deadlines, and default removal.

## Backup, Upgrade, Rollback, And Restore

A backup is a consistency group: operator configuration, both `ardents.db` and
`identity-access.db`, node identity, secrets, migration markers, and local
authority material are archived together. No transaction spans the two
databases. Stop the service first; this drains and releases the daemon-owned
identity database handle. The command refuses a live node, never opens the live
database independently, and writes a checksum-bearing sidecar manifest next to
the archive:

```sh
sudo systemctl stop ardentsd
sudo ./scripts/install/linux.sh backup \
  --output /var/backups/ardents/node-1-$(date -u +%Y%m%dT%H%M%SZ).tar.gz
sudo systemctl start ardentsd
```

Backup destinations are intentionally restricted to the root-owned
`/var/backups/ardents` tree; this prevents a privileged installer from writing
through an attacker-controlled temporary path.

To upgrade, extract the new verified release and run:

```sh
sudo ./scripts/install/linux.sh upgrade
```

The installer executes both staged roles, requires matching build identities,
stops an active node, creates a consistency-group backup, retains the previous
binary pair, and starts the candidate. Success requires the authenticated local
API to report readiness. A candidate that cannot become ready is stopped and
the previous healthy pair is restored automatically; the command still exits
non-zero so automation cannot mistake rollback for upgrade success. If the node
was stopped before the command, the candidate is started only for validation and
the original stopped state is restored afterward.

One prior binary pair is retained for an explicit operator rollback:

```sh
sudo ./scripts/install/linux.sh rollback
```

Rollback changes binaries only. Restore persisted data only when release notes
state that the newer release performed an incompatible migration. Restore
verifies the archive, its allowed paths, and continuity hashes; it refuses a
non-empty target and leaves the service stopped for operator inspection:

```sh
sudo systemctl stop ardentsd
# Move the retained current config/state aside; do not delete it yet.
sudo ./scripts/install/linux.sh restore \
  --archive /var/backups/ardents/node-1-20260721T000000Z.tar.gz
sudo systemctl start ardentsd
sudo ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --legacy-token-file /var/lib/ardents/secrets/api-token node status
```

Ordinary uninstall is deliberately non-destructive:

```sh
sudo ./scripts/install/linux.sh uninstall
```

It stops and disables the service and removes the unit and executables. It does
not remove `/etc/ardents`, `/var/lib/ardents`, the service account, identity,
authority material, credentials, or retained content. State erasure is a
separate operator-controlled action and is not provided as a convenience flag.

`install` is not the version-transition command: use it for first installation
or same-build repair, and use `upgrade` when changing releases. `--no-start`
and `--root` exist for release packaging and isolated acceptance; they are not
normal production installation options.
