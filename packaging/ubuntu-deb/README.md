# Ubuntu `.deb` packaging

This is the selected first H4-1D package source for Ubuntu `amd64`. It builds
one directly distributed `.deb`; it does not define an APT repository, a
package signing key, a maintainer script, a system service, a user service, or
an updater.

`build.sh` takes three already authenticated release-workflow inputs:

- `ARDENTS_PACKAGE_PROGRAM`: the exact Linux `amd64` `ardents` executable;
- `ARDENTS_PACKAGE_STATIC_ROOT`: the independently pinned static enrollment
  directory, containing `SHA256SUMS`, `RELEASE`, the Release root, and metadata
  but **not** the executable; and
- `ARDENTS_PACKAGE_OUTPUT`: an absolute path outside the repository for the
  generated `.deb`.

The package contains only:

```text
/usr/lib/ardents/ardents
/usr/share/ardents/enrollment/<package-version>/<static enrollment entries>
/usr/bin/ardents
```

The package manager records those payload files as root-owned. The executable
and launcher are `0755`; static enrollment files are `0644` and every package
payload path rejects group/world write. This lets the unprivileged participant
read authenticated static facts while refusing a user-writable substitute.

The launcher executes the direct `/usr/lib/ardents/ardents` file so
`endpoint enroll-installed` can reject a substituted program rather than
accepting a symlink. The package deliberately contains no XDG, Vault, floor,
cache, runtime, enrollment-input, or unit path. `dpkg` remove/purge can thus
remove only package-owned bytes; Authority and Release state are outside the
package lifecycle.

The participant still verifies the `.deb` through independently delivered
package digest/provenance before `dpkg -i`, creates their owner-only package
enrollment input outside `/usr` naming the exact versioned static directory,
and explicitly renders/enables a user unit:

```sh
/usr/bin/ardents endpoint installed-user-unit /absolute/path/to/package-enrollment.json \
  > ~/.config/systemd/user/ardents-endpoint.service
```

The installed executable then verifies the pinned static manifest, exact
package-owned artifact, and Release Decision before Endpoint readiness. A
package checksum or any future package-store signature is transport/bootstrap
evidence, not Release authorization.
