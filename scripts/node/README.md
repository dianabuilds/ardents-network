# Native development baseline

`make node-development-check` executes one predeclared series: three iterations
of the complete Forwarding, Issuer, Descriptor, Introduction and JOIN profiles,
in that order. Every original assertion, race binary, timeout, durable sync,
authority check and negative case in each profile remains mandatory. The
ninety-minute series bound grants no longer runtime/protocol deadline. Failed
cases are retained, never retried, skipped or converted into ordinary success.

This is a development recipe for an actual Linux cgroup-v2/systemd255 manager,
native ext4 temporary storage, reviewed Python3.12 with isolated standard-library
imports (`-I -S`) and
the five profiles' existing tools. The tool's dependency/support scope belongs to
the [dependency owner](../../docs/development/dependencies.md).
The measured stand is Ubuntu24.04, systemd255.4 and WSL2 kernel6.6.87.2 with
12 logical CPUs. WSL/host contention can invalidate or fail an attempt; these
finite receipts establish neither native-host superiority nor platform support,
availability, capacity, installed Node or Source qualification. Do not disable
host protection, move durable state to tmpfs or change the original envelopes.

Build canonical `ardents-next` with the root Makefile's canonical flags and both
`node_process_native` command/resource test binaries with `-race`, using the
selected toolchain/dependency owner. Source snapshots, caches and artifacts stay
outside Git. Generate a JSON manifest `{"files":{"relative/path":"sha256",...}}`
covering **every** source input to those builds, including these scripts, the
Makefile and profile registry. Preserve the manifest/artifact SHA-256 values
independently before transferring them to the native stand. Set the existing
`ARDENTS_NODE_NATIVE_COMMAND`, `ARDENTS_NODE_NATIVE_TEST`,
`ARDENTS_NODE_RESOURCE_TEST` and their `_SHA256` pins, plus
`ARDENTS_NODE_NATIVE_SOURCE_MANIFEST` and `ARDENTS_NODE_NATIVE_SOURCE_SHA256`.
The runner verifies those bytes before and after every complete profile.

Choose one existing private external directory as
`ARDENTS_DEVELOPMENT_LOCK_PARENT`, shared by **all our jobs**, including across
Windows and WSL (use its corresponding `/mnt/c/...` spelling in WSL). Run
compilers, scanners, ordinary quick/full gates and direct native checks through:

```sh
python3 -I -S scripts/node/run-development.py exclusive "$ARDENTS_DEVELOPMENT_LOCK_PARENT" -- make quick-check
python3 -I -S scripts/node/run-development.py exclusive "$ARDENTS_DEVELOPMENT_LOCK_PARENT" -- make check
# Use this same wrapper around the actual artifact build command as well.
```

Do not wrap `node-development-check` again: it acquires the same lock itself.
Set `ARDENTS_NODE_DEVELOPMENT_OUTPUT` to a fresh empty private directory outside
source, then invoke `make node-development-check`. Its real non-test
`node inspect-process` preflight runs as the sole process of a transient service
with the exact original CPU/RAM/PID/FD/Go envelope; the driver stays outside.
Missing/positive refusal evidence fails preflight. The runner records the full
finite schedule before any role runs, checks actual manager/process-start,
kernel, storage and cgroup prerequisites every half second, then checks source
and artifacts again after the original child joins. It keeps the original
kernel/manager/storage identity for the whole series. Continuing prerequisite
loss is invalid evidence, not a passing skip; the child retains physical cleanup
before the wrapper returns its lock. No supplied fact grants runtime admission.
Each local manager/storage query has a five-second driver bound; preflight's
original ten-second service has a fifteen-second driver wait. An unavailable
physical-completion observation retains the lock. The first continuing validity
loss is saved with its elapsed observation context and cannot be overwritten by
a later loss. Source/artifact loss records failure and leaves unrun cases visible.

The atomic lock is coordination among these cooperating jobs, not host security
or a claim to control unrelated workloads. Contention fails immediately. There
is no stale-lock takeover: after an interrupted driver, prove its original jobs,
services and callbacks joined before removing only that owned empty lock.
Do not stop foreign workloads. Required test actors retain their own finite
manager bounds and cleanup. Keep every failed receipt and any unrun scheduled
case visible. A corrected candidate needs a new source identity and separate
explicit series; it cannot erase the preceding series.

Each profile includes separate real pressure, unavailable evidence, original
caller and native placement controls. The generic native guard additionally
introduces an actual foreign child into its measured leaf, joins it, and checks
that restored placement cannot revive the failed owner. These are successful
refusal controls, never successful Application operations. Node first-failure
output extends its original single JSON report through a string-valued
`first_failure` field containing compact JSON. It contains only fixed local
phase/cause, elapsed/deadline and original numeric resource facts; independent
physical cleanup owners remain in the original outcome. No raw error, Target,
private path, key, capsule or cross-role correlation identity is emitted.

`make source-process-check` is a separate Direct-Origin Source workload. It uses
the same canonical command and command/resource race artifact pin variables,
`node_process_native` compilation tag, external lock and native prerequisites.
It is not included in the five-Node baseline series. Declare its finite schedule
and verify complete source/config/artifact and actual stand facts before/after;
retain every attempt, restore all owned changes and prove physical joins.
