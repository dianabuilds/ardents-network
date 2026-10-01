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

## Installation

Build the worker from the candidate commit with CGO_ENABLED=0, GOOS=linux and
GOARCH=amd64. Copy that binary and this fixed source inventory to the selected
Ubuntu host, then run install.py as root with the prepared binary path.

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
still requires at least 10/40 Mbit/s over the actual measured interval. The
runner never rounds a slow result up by replacing elapsed time with 600.
Every active stream still requires at least 500 Kbit/s and at most two seconds
without progress. Canary exchanges remain outside useful active-stream bytes.

Local runner JSONL ends with a record count and checksum after every participant
has returned cleanup. Pair verification rejects missing or failed terminal
records, changed observations, duplicate participants and records after the
terminal. This checksum detects incomplete/mixed files; it is not an independent
attestation or a substitute for carrier and hosting measurements.

### Install the Endpoint runner

Prepare both static Linux amd64 commands from the same candidate. As root, run
python3 install_endpoint.py /absolute/path/ardents-qualification /absolute/path/local-plan.json,
then the worker installer above. The Endpoint installer creates the dedicated
unprivileged account when absent and installs the fixed ardents-endpoint.service
without starting or enabling an automatic workload. It refuses to overwrite a
different Endpoint service, any unit with drop-ins, or an active qualification.

The plan must point to already provisioned current State, Service Instance,
permission handover paths and the shared real hosting period. Installation
does not generate replacement authority or reset those durable roots. Prepare
their ownership for ardents-endpoint; the installer does not recursively
change arbitrary directories named by a plan.

An explicit systemctl start ardents-endpoint.service starts one bounded
attempt. Save its systemd InvocationID, result and complete journal for that
invocation, including stderr and unsuccessful termination. Preserve each
attempt before installing another plan. The unit bounds runtime to 21 minutes
and shutdown to 30 seconds; a killed process or absent runner terminal fails
acceptance. Worker cgroups remain tied to this exact Endpoint service.

### NET-32 short projection

A plan with `Mode` equal to `net32-idle` contains one normal Reader and starts
no Service Connection or qualification worker. The installed Endpoint observes
ten idle minutes, preserves one-second whole-owner and provider-interface
samples, reconciles them with the shared hosting ledger, and reports an explicit
upper 24-hour projection against 1,000,000,000 aggregate bytes. This is a short
projection for issue #60; it is never represented as a 24-hour observed run.