# Public initial installation capture

Run the explicit driver on each separately admitted Ubuntu 24.04 x86-64,
systemd 255, cgroup-v2 host after preparing the actual public Release, Network,
Source and Custody prerequisites. It installs the fixed resources selected by
the request and starts the actual installed unit. Use dedicated authorized hosts.

```sh
sudo sh tests/qualification/protected-installation/run-ubuntu.sh \
  /absolute/pinned/ardents independently-verified-sha256 \
  /absolute/request.json /absolute/installation-root /absolute/new-evidence
```

The program digest must come from independent artifact verification. The request
must contain genuine current authority inputs; this driver creates no keys,
permissions, test responses or acceptance shortcuts. The product commands own
complete request validation. A missing selected prerequisite is an invalid
environment, never a passing skip.

The new private evidence directory must be outside both the checkout and the
installation root. It retains each stdout, stderr and exit code, plus the
verified program digest and capture scope before the first command. Set
`ARDENTS_INSTALLATION_SOURCE_REVISION` to the program's full source commit when
known; otherwise the record explicitly says `unknown`, never infers identity
from the checkout used to run the driver.
Any failure stops this attempt without retry, reset-failed, cleanup or root/floor
deletion. Explicit recovery is a separate operation with fresh authority.
Captured diagnostics may contain local paths; keep them private.

This captures public provision, installation-check and real system-manager
start. It does **not** establish participant readiness, upgrade/recovery fault
coverage, the two-Endpoint Service journey, namespace/seccomp/cgroup confinement,
joined empty scopes, or restart acceptance. A successful systemctl start for a
Type=exec unit is only manager-start evidence. Docker cannot substitute for the
admitted host. No actual admitted-host execution is claimed by adding this driver.
