# Ubuntu `.deb` packaging

This builder packages the executable built from `cmd/ardents-next` and static enrollment files for
Ubuntu `amd64`. The `make package-e2e` profile checks the installed package
contents and verifies that a user-owned state marker survives upgrade, removal,
and purge.

`make package-ubuntu-deb` runs `build.sh` with these environment variables:

- `ARDENTS_PACKAGE_PROGRAM`: the exact Linux `amd64` executable built from `cmd/ardents-next`;
- `ARDENTS_PACKAGE_STATIC_ROOT`: the independently pinned static enrollment
  directory containing `SHA256SUMS`, `RELEASE`, the Release root, and metadata.
  Supply the executable separately through `ARDENTS_PACKAGE_PROGRAM`.
- `ARDENTS_PACKAGE_VERSION`: the Debian package version, also used in the
  installed enrollment path; and
- `ARDENTS_PACKAGE_OUTPUT`: an absolute path outside the repository for the
  generated `.deb`.

The builder uses the repository Go toolchain and the reviewed Enrollment
implementation to check the exact staged v6 inventory and executable digest
before producing the package.

The package contains only:

```text
/usr/lib/ardents/ardents
/usr/share/ardents/enrollment/<package-version>/<static enrollment entries>
/usr/bin/ardents
```

The package manager records those payload files as root-owned. The executable
and launcher have mode `0755`. Static enrollment files have mode `0644`.
Package-owned files and directories disallow group and world writes.

The launcher executes `/usr/lib/ardents/ardents` directly. User data, Authority
and Release state, and service units live outside the package's payload.

Verify a distributed `.deb` against its independently delivered digest before
running `dpkg -i`. Endpoint startup follows the
[platform delivery procedure](../../docs/reference/commands.md#protected-endpoint-platform-delivery), which
uses the installed `ardents` command and performs its own artifact and Release checks.
