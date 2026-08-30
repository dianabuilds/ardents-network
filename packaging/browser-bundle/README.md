# Browser companion command bundle

This boundary packages the real host-named `ardents-browser` Adapter and
`ardents-browser-entry` native-host command bytes produced by `browser-build`.
`make browser-check` builds the commands, creates the archive twice, verifies
determinism, unpacks it, and byte-compares both executable inputs.

The archive is build-artifact evidence only. It is not enrollment-v4, does not
contain an XPI or a Network Endpoint, and makes no signed-browser, platform, or
release-qualification claim. Enrollment-v4 remains the independently pinned
inventory that binds these Browser companions and the exact signed XPI.
