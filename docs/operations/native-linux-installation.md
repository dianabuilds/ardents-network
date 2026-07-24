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

The release archive contains `ardentsd`, `ardentsctl`,
`scripts/install/linux.sh`, and `systemd/ardentsd.service`.

## First Installation

Run from the extracted release directory:

```sh
sudo ./scripts/install/linux.sh install \
  --node-name node-1 \
  --transport-port 61001
```

Add `--bootstrap-peer <multiaddr>` when joining an existing network endpoint.
The installer creates the locked `ardents` service account, initializes the
Node once, installs the systemd unit, enables it, and starts it. It never
enables Docker workload execution implicitly.

Installed paths are stable:

| Path | Ownership and purpose |
| --- | --- |
| `/usr/local/bin/ardentsd` | root-owned daemon executable |
| `/usr/local/bin/ardentsctl` | root-owned Operator client |
| `/etc/ardents/operator.json` | private canonical `ardents.config/v1` document |
| `/var/lib/ardents` | persistent Node identity, databases, and retained data |
| `/var/lib/ardents/secrets` | protected Operator socket, one-use Bootstrap Ticket, and protected-state keys |
| `/var/lib/ardents-applications` | `ardents-apps`-scoped Application Unix socket |
| `/var/lib/ardents-authority` | root-only local bootstrap authority material |
| `/etc/systemd/system/ardentsd.service` | hardened native service definition |

The local authority is bootstrap material for the self-contained first-Node
path. It is not a replacement for externally governed realm capability
issuance. Back it up separately and do not expose it to Applications.

## First Operator And Routine Access

The install output contains the public Node Principal and the protected path of
its short-lived one-use Bootstrap Ticket. It does not print the Ticket or
create a permanent Operator credential.

Create the prospective Operator's offline root and routine device bundles,
then enroll through the permission-protected Unix socket:

```sh
ardentsctl identity principal create
ardentsctl identity device create --valid-for 720h

sudo ardentsctl \
  --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> \
  identity enroll \
  --root-signer-file "$HOME/.config/ardents/identity/principal-root-v1.json" \
  --device-signer-file "$HOME/.config/ardents/identity/device-v1.json" \
  --bootstrap-ticket-file /var/lib/ardents/secrets/operator-bootstrap-ticket
```

Successful enrollment atomically consumes the Ticket. The provisioning client
deletes its file; if cleanup reports a failure, remove that exact consumed file
before continuing. Do not keep the root signer on the Node. Issue the exact
operational grants needed by the Operator, then use only the device signer for
routine calls:

```sh
sudo ardentsctl \
  --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> \
  --signer-file "$HOME/.config/ardents/identity/device-v1.json" \
  node status
```

Remote administration uses OpenSSH stream-local forwarding directly to the
protected socket:

```sh
ardentsctl --ssh ops@node-1.example \
  --ssh-operator-socket /var/lib/ardents/secrets/control.sock \
  --principal p1_<node> node status
```

The daemon does not expose an Operator HTTP endpoint. SSH host verification and
authentication remain OpenSSH responsibilities.

## Application Enrollment

Local Applications connect only to the separate
`/var/lib/ardents-applications/application.sock`. The installer creates the
`ardents-apps` system group and protects this setgid directory as `0750`. Add
only the service identity that must connect, then restart it so supplementary
groups are refreshed:

```sh
sudo usermod --append --groups ardents-apps hello-service
```

Each Application installation generates its own Principal and root/device
material. An Operator issues a ten-minute, one-use Application Enrollment
Ticket for that exact Principal and exact initial Application actions. The
embedding Application consumes it through the SDK's typed enrollment signer;
the Ticket is not a normal Credential or Session and is never placed on argv or
printed. Operator and Application Sessions have distinct schemes and are
cross-surface-invalid. Knowledge of a CID never establishes ownership or read
authority.

## Backup, Upgrade, Rollback, And Restore

A backup is a stopped-Node consistency group: the canonical configuration,
`ardents.db`, `identity-access.db`, Node/Waku identity, protected state,
Application runtime state, and local authority material are archived together.
No transaction spans the two databases. Stop the service first; the command
refuses a live Node and writes a checksum-bearing sidecar manifest:

```sh
sudo systemctl stop ardentsd
sudo ./scripts/install/linux.sh backup \
  --output /var/backups/ardents/node-1-$(date -u +%Y%m%dT%H%M%SZ).tar.gz
sudo systemctl start ardentsd
```

Backup destinations are restricted to the root-owned
`/var/backups/ardents` tree.

To upgrade, extract the new verified release and run:

```sh
sudo ./scripts/install/linux.sh upgrade
```

The installer requires matching daemon/CLI build identities, stops an active
Node, creates a consistency-group backup, retains the prior binary pair, and
starts the candidate. Success requires the `/readyz` endpoint from the
installed operator configuration's effective loopback
`observability.listen_address` to report composite readiness for the retained
Node Principal and the exact candidate build fingerprint. The installer never
substitutes the default `127.0.0.1:9090` when another address is configured, so
an unrelated healthy service on the default port cannot accept a candidate.
This endpoint is a read-only health boundary, not an Operator credential. A
failed candidate is stopped and the previous healthy pair is restored
automatically; the command still exits non-zero.

One prior binary pair is retained for explicit rollback:

```sh
sudo ./scripts/install/linux.sh rollback
```

Rollback changes binaries only. Restore persisted data only when release notes
require it. Restore verifies the archive, allowed paths, and continuity hashes;
it refuses a non-empty target and leaves the service stopped for inspection:

```sh
sudo systemctl stop ardentsd
# Move the retained current config/state aside; do not delete it yet.
sudo ./scripts/install/linux.sh restore \
  --archive /var/backups/ardents/node-1-20260721T000000Z.tar.gz
sudo systemctl start ardentsd
sudo ardentsctl --addr unix:///var/lib/ardents/secrets/control.sock \
  --principal p1_<node> \
  --signer-file "$HOME/.config/ardents/identity/device-v1.json" node status
```

Daemon restart deliberately invalidates all Sessions. The CLI authenticates
again with its device Credential; durable grants and revocations survive.

## Uninstall

```sh
sudo ./scripts/install/linux.sh uninstall
```

Uninstall stops and disables the service and removes the unit and executables.
It retains `/etc/ardents`, `/var/lib/ardents`, the service account, identity,
authority material, device/grant state, and content. State erasure is a
separate operator-controlled action.

Use `install` only for first installation or same-build repair and `upgrade`
for release changes. `--no-start` and `--root` exist for packaging and isolated
acceptance, not normal production installation.
