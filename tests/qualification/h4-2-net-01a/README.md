# H4-2 NET-01A reference-host campaign preflight

This directory prepares the **future** decision-bearing R-092 campaign; it is
not a capacity test and has no pass result for H4-2.  The real campaign still
needs a maintained native pressure profile, a frozen workload matrix, five
ten-minute sustained runs for every decision-bearing cell, and all the fault
cells defined in [R-092](../../../docs/research/records/r-092-native-node-operating-profile.md).

Run this only on the separately provisioned NET-01A host, never in Docker and
never on the project VPS.  The host must be native Ubuntu LTS `x86-64`, present
exactly two online CPUs, have a 2 GiB provider allocation, use cgroup v2, and
have a separately measured symmetric 100 Mbit/s link.  Linux reserves part of
physical RAM before exposing `MemTotal`; the runner therefore permits
1,900,000–2,200,000 KiB and retains both that raw observation and the
operator's provider declaration.  A stronger host is rejected; a value inside
this band does not by itself prove a provider allocation.

Before running, use a second host to record the two directions of the selected
link measurement, including commands, timestamps, peer identity, and raw
output.  Save that immutable transcript outside the repository.  The Product
Owner checks that it supports the declared symmetric 100 Mbit/s link; the shell
script only requires a non-empty regular file and copies its exact bytes into
the evidence directory.  Prepare a second, non-empty host-declaration file
outside the repository with the provider's 2 GiB / 2 vCPU plan, instance ID,
region, provisioning timestamp, and the person who made that declaration.  The
script copies it unchanged; it does not treat an arbitrary text file as a
provider attestation.

From the repository root on the NET-01A host:

```sh
export ARDENTS_NET_01A_EVIDENCE_DIR=/var/tmp/ardents-net-01a-20260826
export ARDENTS_NET_01A_LINK_EVIDENCE=/var/tmp/net-01a-link.txt
export ARDENTS_NET_01A_HOST_DECLARATION=/var/tmp/net-01a-host.txt
make prepare-h4-2-net-01a
```

`ARDENTS_NET_01A_EVIDENCE_DIR` must be a new absolute path outside the
repository.  The preflight writes only there, with `0700` permissions, and
does not create a cgroup, start Docker, build a product binary, alter system
settings, or contact a peer.  It retains a timestamp, `/etc/os-release`,
kernel, CPU/memory observations, cgroup facts, source commit and dirty state,
Go toolchain facts, and the supplied link and host-declaration transcripts. An
invalid host leaves an `outcome.txt` explaining its failure, so failed attempts
remain evidence. A successful preflight also writes SHA-256 values for the
captured host and supplied input transcripts.

A successful preflight says only that the host is eligible to begin the later
campaign.  It does not select a native Node capacity, validate a cgroup limit,
or close R-092 or H4-2.
