# Fixed NET-14 stream-worker artifact

This directory is the source inventory for the separately pinned
qualification-only worker. Installation creates the root-owned immutable
`/usr/lib/ardents/network-stream-worker-root/ardents-stream-qualification`,
the two matching socket/service pairs and the stop-only policy rule. Its
manifest is `/etc/ardents/network-stream-worker-artifact.json`; it hashes
exactly those six installed files.

The text worker root and `/etc/ardents/text-worker-artifact.json` never list
this executable. This artifact has no user command, address, destination,
authority, executable selector or profile argument. The Endpoint launch owner
is added only with its real qualification runner and must refuse a missing or
substituted manifest before a local Grant can exist.

This inventory is not an installed-host qualification receipt.
