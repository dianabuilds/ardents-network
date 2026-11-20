# Node process tests

The ordinary process profile builds the actual command and verifies the
retired Node plan, Issuer, Source and Endpoint headless selectors. Existing
and absent plans produce the same exact refusal. The fixtures verify that
the commands preserve supplied bytes and create no runtime roots.

Maintained positive Native Node, signed Network preparation, and Source
distribution and shutdown evidence live in the selected successor profiles
listed in [testing](../../../docs/development/testing.md). Those profiles use
the actual `ardents-next` role lifecycle and joined completion. Historical
two-Node and closed-issuer receipts remain bound to their original artifacts.

Run the process checks with `make e2e` or `go test ./tests/e2e/node`.

## Installed ordinary text commands

`TestInstalledClosedTextCommandsThroughNodeProcesses` is selected only by the
Linux `text_worker_installed` build tag. Run its prebuilt test binary as root
outside the Endpoint service on the dedicated preinstalled Ubuntu host, with
`ARDENTS_TEXT_COMMAND_QUALIFICATION=1` and `ARDENTS_E2E_COMMAND_ROOT` pointing to
independently hashed, root-owned product commands from the same candidate.
The ordinary worker and its manifest must match that candidate. Missing setup
is an invalid environment; the profile has no fixture fallback.

The test provisions canonical State and all sixteen real Node commands. It starts
each Node in a separate systemd scope and verifies that its READY process is
the sole member of that scope. Short-lived orchestrator commands therefore do
not enter a Node's required pressure sample. Scope cleanup must establish an
inactive unit and an absent or unpopulated cgroup. The topology is checked for
process exit or emitted loss of READY before and after the participant journey;
an unused failed Node cannot be hidden by successful text commands. These are
fixture placement and observed lifecycle checks, not a continuous-duty or
whole-host qualification.
The test creates and accepts a Service Instance through Custody and Instance commands, then runs
the ordinary `ardents endpoint headless` as the unprivileged Endpoint service's
MainPID. Its Source plan uses the two live Sources and retains actual exposure
history in the Endpoint-owned local-role root. The fixture shares a Source client
identity with its Node setup; this is not evidence of independent participants.
The orchestrator prepares private Instance/token directories and launches text
commands through user-owned Bash pipelines with pipefail so their pollable stdio
can be reopened by that same user and command failures remain failures.
It checks permission-request commitments from that exact invocation
against the request files and uses real Custody responses. The ordinary text
commands publish and read a 64 KiB document through this network on both Carriers.
The test observes the real resolution Store through bounded reads of its atomic
public record. It verifies signatures, unchanged publication, a next revision and
fresh slot/key after elapsed refresh, then reads again after a further 65 seconds.
The observation allows command completion through 330 seconds after the first
Descriptor start; it does not extend runtime validity or independently prove
the exact ACK time, old recipient erasure or the overlap cutoff. Run the two
Carrier cases with a twenty-minute test timeout; each Endpoint unit has a
ten-minute watchdog. No clocks or scheduler fields are advanced.
An explicit request through the real Administration socket withdraws publication;
the ordinary Link command must then refuse while the same Endpoint invocation
remains active and no Reader or Publisher worker unit remains active or stopping.
The process collector continuously drains periodic resource samples; only
lifecycle and non-periodic events enter its bounded event queue. This avoids
stalling Nodes while the test waits for publication refresh, and does not
provide resource-measurement evidence.
This uses one Endpoint for both local roles, not two independent Endpoint hosts.
Compilation alone does not prove this installed journey works.

Preserve the prior ordinary worker, manifest and existing temporary Endpoint
unit before a run. The test requires an inactive service without drop-ins and
restores its prior temporary unit during cleanup. An external runner must also
restore those files after test failure or timeout and verify their hashes,
MainPID zero and absence of live workers. Retain the candidate hashes, invocation
journal and terminal result outside Git. This cell does not qualify continuous Node readiness or full recovery.
