# Alpha-control two-fresh-Endpoint qualification

Status: **active live qualifier for one selected immutable functional-alpha
candidate.** A passing run is required before the candidate may claim that two fresh
Endpoint instances observed the same concrete control inputs. It is not an
independent-control, update, capacity, availability, Windows-Endpoint, or
Public Beta result.

## Fixed topology and trust boundary

The Windows qualification host owns the exact published Linux archive and the
independently delivered Enrollment Pin. It verifies the archive, Pin, exact
manifest inventory, every manifested byte, and the expected Endpoint and
control-companion digests before upload. It then transfers only that archive
and this checked runner to one literal project VPS.

The remote runner requires Ubuntu 22.04 on `x86_64` and root only as the
short-lived orchestrator. It creates one validated
`/tmp/ardents-alpha-control-two-endpoints-<hex>` root. Both Endpoint processes run as
UID/GID 65534 with cleared groups, capabilities removed, a clean environment,
`no_new_privs`, separate byte-identical bundle copies, and distinct fresh
`HOME`, XDG config, state, cache, runtime, and temporary roots. No participant
state from an earlier run is reused.

The exact enrolled Endpoint binary in each copy must emit, in order:

1. `endpoint-lifecycle` `starting`;
2. `release-decision` with the exact `release-accepted` outcome and selected
   cohort/Release identity; and
3. `endpoint-lifecycle` `ready` with a live local socket.

While both Endpoints are ready, the exact manifested `ardents-control`
companion runs once for each Endpoint. Each invocation uses that Endpoint's
enrollment input, artifact copy, a distinct absent inspection root, and the
same operator-supplied RFC3339 decision time. A passing report has schema
`ardents-alpha-control-report-v1`; accepted catalog and all three components;
the exact `release-accepted` Release outcome; and exact catalog,
component, Release, Network, profile, generation, validity, and digest
identities. The catalog identity must equal SHA-256 of `catalog.ac1`; each
component root ID and digest must equal SHA-256 of its exact `.pub` and `.ac1`
bytes; and the Release identity must equal the selected release. Both Endpoints
must start without a Release floor and commit their own canonical Release
floor. Both fresh inspection roots must likewise start absent and commit their
own canonical Release floor. The complete byte inventories of all four Release
floors must agree. The Release outcomes must agree, each
control report must agree with its Endpoint outcome, and the two complete JSON
reports must be byte-for-byte equal. There is no cached repeat in this profile:
all four observations are first evaluations against absent state and this
selected profile requires authenticated protocol/build conditions that classify
as `release-accepted`. `no-update` remains a valid outcome for other Release
Safety states (including an unchanged committed floor or an incomplete protocol
overlap), but it fails this exact profile; any cached repeat belongs in a
separate non-denominator check.

The read-only `inspect-bundle` command is deliberately not executable
self-binding under ADR-0038 and ADR-0042. This qualifier closes that limited
procedural gap by verifying the control binary as the exact manifest companion
before either invocation. It does not turn the companion, GitHub, the VPS, or
the test runner into an Authority.

Finally the orchestrator sends SIGTERM to both exact Endpoint PIDs. Each must
emit `stopped`, exit zero, and remove its socket. A harness cleanup path still
terminates owned processes after a failure, performs a bounded post-SIGKILL
check, and rejects any remaining process or socket. Cleanup never converts the
failed attempt into a pass.

## Invocation

Run from a clean checked-out repository on Windows. Every value is mandatory;
there are no ambient candidate or host defaults.

```powershell
make qualification-alpha-control-two-endpoints `
  ALPHA_CONTROL_ARCHIVE='C:/absolute/outside-repository/candidate-linux-amd64.tar.gz' `
  ALPHA_CONTROL_ARCHIVE_SHA256='<64-lowercase-hex>' `
  ALPHA_CONTROL_MANIFEST_PIN='<64-lowercase-hex>' `
  ALPHA_CONTROL_ENDPOINT_SHA256='<64-lowercase-hex>' `
  ALPHA_CONTROL_CONTROL_SHA256='<64-lowercase-hex>' `
  ALPHA_CONTROL_COHORT='<exact-cohort>' `
  ALPHA_CONTROL_RELEASE='<exact-release>' `
  ALPHA_CONTROL_AT='<UTC-RFC3339-second>' `
  ALPHA_CONTROL_VPS='<literal-IPv4>' `
  ALPHA_CONTROL_SSH_KEY='C:/absolute/private-key' `
  ALPHA_CONTROL_VPS_USER='<exact-SSH-user>' `
  ALPHA_CONTROL_EVIDENCE='C:/absolute/outside-repository/previously-absent-directory'
```

`ALPHA_CONTROL_AT` is one UTC second such as `2026-08-29T12:00:00Z`; it must be within
the authenticated catalog, component, Release, and Network validity windows.
The SSH account must be able to orchestrate as UID 0. An absent prerequisite is
an invalid selected environment, never a skipped pass.

## Evidence and denominator

The `ALPHA_CONTROL_EVIDENCE` path must be absolute, outside the repository, and absent
at start. Those path-safety checks happen before attempt eligibility because no
safe receipt can exist without them. Once the runner creates that directory,
the attempt is eligible and every remaining preflight, candidate, environment,
harness, product, transfer, and cleanup failure is retained. It records:

- exact non-secret candidate inputs, harness revision and hashes, and the clean
  worktree fact;
- Windows and Ubuntu host envelopes and SSH host-key observation;
- archive, manifest, descriptor, inventory, Endpoint, control, copied-bundle,
  and complete canonical Release-floor inventories/digests;
- both lifecycle stdout/stderr streams, control stdout/stderr streams, process
  privilege facts, exit statuses, socket cleanup, and final remote residue;
- remote-run, evidence-transfer, and exact-root cleanup receipts; and
- a final SHA-256 inventory of the retained evidence.

Private-key bytes, environment variables, participant data, and raw state
contents are never copied into evidence. Every invocation that reaches safe
evidence-directory creation is one eligible attempt. The denominator is all
eligible attempts, including invalid environments and harness or product
failures; a later rerun is separate evidence and never erases an earlier
result. A copied evidence bundle is validated for mandatory receipts and exact
fresh outcomes before acceptance. Remote-root cleanup failure is a
qualification failure.
