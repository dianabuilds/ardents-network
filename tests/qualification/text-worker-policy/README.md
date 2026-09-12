# Installed text-worker stop policy checks

Run `make text-worker-policy-check` as root in the explicitly selected Ubuntu
24.04 systemd/polkit environment after installing the verified worker rule.
This runner checks authorization only; it does not install policy, start or
stop services, issue a worker Grant, or qualify complete host confinement.

The installed root-owned rule must be present at
`/usr/share/polkit-1/rules.d/50-ardents-text.rules`. The distinct unprivileged
`ardents-endpoint` and `nobody` accounts and preinstalled `pkcheck`, `setpriv`,
`stat`, `awk`, `sleep`, `timeout`, `id`, `systemctl`, `cmp`, `uname`, and
`dirname` are required. The runner verifies Ubuntu 24.04 x86-64/systemd 255,
root:root mode 0644 without a symlink, and exact policy bytes against this
source checkout. Missing prerequisites
or an unexpected authorization result fail the run, never skip it.

The root driver creates one finite sleep subject per account, queries polkit
with that exact PID/start time/UID, and kills/joins only its own subjects. Root
is necessary because the maintained Ubuntu polkit restricts callers supplying
action details. Each subject lasts 120 seconds, exceeding the 17 five-second query bounds;
cleanup joins it before moving to the next account. Each query has no interaction. An
Endpoint subject must receive direct authorization only for exact canonical
reader/publisher instance stops. Other verbs, socket/templates, malformed
instances, integer overflow, trailing newlines and foreign services must be
refused. A foreign subject must never receive direct authorization; a distro
administrator-authentication challenge without interaction is also refusal.

Capture the complete output and the installed systemd/polkit/package and rule
inventory with the candidate's evidence. Real Endpoint-issued `systemctl stop`,
joined cgroup cleanup, malicious descendants and Principal/Grant binding remain
separate installed lifecycle tests under the confinement owner.

## Operating-system component inventory

The rule uses the Ubuntu-maintained `polkitd` JavaScript rule engine and
systemd's `org.freedesktop.systemd1.manage-units` action details. `polkitd` is
an installed runtime component; `pkcheck` is the distribution query driver.
No new Go dependency or privileged application launcher is involved.
The operator installs the security-updated Ubuntu packages as part of the
verified environment and records their actual versions with the evidence:

```sh
dpkg-query -W systemd polkitd libpolkit-agent-1-0 libpolkit-gobject-1-0 libduktape207 libglib2.0-0t64 libpam-systemd sgml-base xml-core
uname -r
systemctl --version
sha256sum /usr/share/polkit-1/rules.d/50-ardents-text.rules
```

The [Ubuntu package record](https://packages.ubuntu.com/noble/polkitd) supplies
the distribution provenance; the [polkit rule contract](https://raw.githubusercontent.com/polkit-org/polkit/124/docs/man/polkit.xml)
and [systemd 255 action details](https://raw.githubusercontent.com/systemd/systemd/v255/src/core/dbus-util.c)
supply the authorization mechanism. Accessed 2026-09-08. An installed version
or a passing permission matrix is not evidence that every host package has
passed the repository's maintenance/vulnerability acceptance rule. That
inventory and current finding disposition remain required for host acceptance.
