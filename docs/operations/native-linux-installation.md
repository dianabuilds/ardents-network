# Native Linux Installation

> Status: qualification candidate. Install, startup, restart, same-build
> reinstall, and non-destructive uninstall are accepted. Production support
> still requires old-to-new upgrade, rollback, and backup/restore evidence.

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
  --token-file /var/lib/ardents/secrets/api-token \
  node status
```

Remote administration uses `ardentsctl --ssh`; the daemon API remains bound to
loopback and is not published as remote HTTP.

## Upgrade And Uninstall

To upgrade, extract the new verified release and run:

```sh
sudo ./scripts/install/linux.sh install
```

The installer executes both staged roles, requires matching build identities,
then transactionally replaces the executables with rollback on replacement
failure. It keeps the existing
configuration, identity, secrets, and data, reloads systemd, and restarts an
already active service. Create a consistency-group backup before an upgrade
whose release notes mention persisted-format changes.

Ordinary uninstall is deliberately non-destructive:

```sh
sudo ./scripts/install/linux.sh uninstall
```

It stops and disables the service and removes the unit and executables. It does
not remove `/etc/ardents`, `/var/lib/ardents`, the service account, identity,
authority material, credentials, or retained content. State erasure is a
separate operator-controlled action and is not provided as a convenience flag.

`--no-start` and `--root` exist for release packaging and isolated acceptance;
they are not normal production installation options.
