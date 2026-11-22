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
must refuse a missing or substituted manifest before a local Grant can exist.

This inventory is not an installed-host qualification receipt.

## Installation

Build the worker from the candidate commit using the
[text-worker qualification procedure](../../tests/qualification/text-worker-lifecycle/README.md).
Copy the static Linux-amd64 binary and this fixed source inventory to the
selected Ubuntu host, then run `install.py` as root with the prepared binary path.

The installer refuses dynamically linked binaries, non-root-controlled paths,
unexpected root contents and active qualification workers. It stops only the
qualification sockets before replacing files, writes the canonical six-file
manifest last, reloads systemd and enables those sockets. An interrupted install
requires rerunning installation; Endpoint refuses a mismatched manifest.

The resulting manifest and printed hashes identify installation, not successful
qualification. Normal text-worker artifacts are owned by their separate installer.

### Offered load and reported throughput

The fixed scheduler offers 10.1 Mbit/s per Reader and 40.4 Mbit/s at the
Publisher for 600 seconds: 47,343,750 useful bytes per active stream. The
one-percent offered margin accommodates start and EOF propagation; acceptance
still requires at least 10/40 Mbit/s over the actual measured interval. Throughput is measured using the actual elapsed time.
Every active stream still requires at least 500 Kbit/s and at most two seconds
without progress. Canary exchanges remain outside useful active-stream bytes.
